package dns

import (
	"context"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/fmotalleb/hermes/models"
)

func (h *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	ctx, cancel := context.WithCancel(h.ctx)
	defer cancel()
	ctx, span := h.tracer.Start(ctx, "dns.serve")

	defer span.End(trace.WithStackTrace(true))
	if len(r.Question) == 0 {
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeFormatError)
		_ = w.WriteMsg(msg)
		span.SetStatus(codes.Error, "no question in request")
		return
	}

	q := r.Question[0]
	span.AddEvent("lookup", trace.WithAttributes(
		attribute.String("name", q.Name),
		attribute.Int("class", int(q.Qclass)),
		attribute.Int("type", int(q.Qtype)),
	))
	resp, err := h.lookup(ctx, q.Name, q.Qtype, r)
	if err != nil {
		span.AddEvent("failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		span.SetStatus(codes.Error, "failed to lookup the domain")
		h.logger.Error("dns lookup failed", err)
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		_ = w.WriteMsg(msg)
		return
	}
	_ = w.WriteMsg(resp)
	span.SetStatus(codes.Ok, "ok")
}

func (h *handler) lookup(ctx context.Context, qname string, qtype uint16, req *dns.Msg) (*dns.Msg, error) {
	ctx, span := otel.Tracer("dns").Start(ctx, "dns.lookup")
	defer span.End()

	span.SetAttributes(
		attribute.String("query.name", qname),
		attribute.Int("query.type", int(qtype)),
	)

	qname = normalizeDNSName(qname)
	span.SetAttributes(attribute.String("query.normalized_name", qname))

	zone, err := h.store.findZone(ctx, qname)
	if err != nil {
		if isNoRows(err) {
			span.AddEvent("zone not found")
			span.SetStatus(codes.Ok, "nxdomain")
			return nxdomain(req), nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to find zone")
		return nil, err
	}
	span.SetAttributes(
		attribute.String("zone.id", zone.ID),
		attribute.String("zone.name", zone.Name),
		attribute.String("zone.forward_zone_id", zone.ForwardZoneID),
	)

	records, err := h.store.recordsForZone(ctx, zone.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to load records")
		return nil, err
	}
	span.SetAttributes(attribute.Int("zone.record_count", len(records)))

	answers := buildAnswers(zone.Name, qname, qtype, records)
	if len(answers) > 0 {
		span.AddEvent("answer selected", trace.WithAttributes(
			attribute.Int("answer_count", len(answers)),
		))
		span.SetStatus(codes.Ok, "answered from zone")
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.Authoritative = true
		msg.Answer = answers
		msg.RecursionAvailable = zone.ForwardZoneID == ""
		return msg, nil
	}

	if zone.ForwardZoneID != "" {
		span.AddEvent("forwarding query", trace.WithAttributes(
			attribute.String("forward_zone_id", zone.ForwardZoneID),
		))
		return h.forward(ctx, req, zone.ForwardZoneID)
	}

	span.AddEvent("no matching records")
	span.SetStatus(codes.Ok, "no answer")
	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Authoritative = true
	msg.RecursionAvailable = false
	return msg, nil
}

func (h *handler) forward(
	ctx context.Context,
	req *dns.Msg,
	forwardZoneID string,
) (*dns.Msg, error) {
	ctx, span := otel.Tracer("dns").Start(ctx, "dns.forward")
	defer span.End()

	span.SetAttributes(attribute.String("forward_zone_id", forwardZoneID))

	fz, err := h.store.findForwardZone(
		ctx,
		forwardZoneID,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to load forward zone")
		return nil, err
	}
	span.SetAttributes(
		attribute.String("forward_zone.name", fz.Name),
		attribute.Int("forward_zone.address_count", len(fz.Addresses)),
	)

	for i, raw := range fz.Addresses {
		network, target := parseDNSAddress(raw)
		if target == "" {
			span.AddEvent("skipped empty forward address", trace.WithAttributes(
				attribute.Int("index", i),
				attribute.String("address", raw),
			))
			continue
		}

		span.AddEvent("forward attempt", trace.WithAttributes(
			attribute.Int("index", i),
			attribute.String("network", network),
			attribute.String("target", target),
		))

		client := &dns.Client{
			Net:     network,
			Timeout: 3 * time.Second,
		}

		resp, _, err := client.ExchangeContext(
			ctx,
			req.Copy(),
			target,
		)
		if err != nil {
			span.AddEvent("forward attempt failed", trace.WithAttributes(
				attribute.Int("index", i),
				attribute.String("error", err.Error()),
			))
			continue
		}

		if resp == nil {
			span.AddEvent("forward returned empty response", trace.WithAttributes(
				attribute.Int("index", i),
			))
			continue
		}

		// Retry truncated UDP over TCP
		if resp.Truncated && network == "udp" {
			span.AddEvent("retrying truncated response over tcp", trace.WithAttributes(
				attribute.Int("index", i),
				attribute.String("target", target),
			))
			tcpClient := &dns.Client{
				Net:     "tcp",
				Timeout: 3 * time.Second,
			}

			tcpResp, _, err := tcpClient.ExchangeContext(
				ctx,
				req.Copy(),
				target,
			)
			if err == nil && tcpResp != nil {
				span.AddEvent("forward succeeded over tcp", trace.WithAttributes(
					attribute.Int("index", i),
				))
				span.SetStatus(codes.Ok, "forwarded over tcp")
				return tcpResp, nil
			}
		}

		span.AddEvent("forward succeeded", trace.WithAttributes(
			attribute.Int("index", i),
		))
		span.SetStatus(codes.Ok, "forwarded")
		return resp, nil
	}

	span.SetStatus(codes.Error, "all forwarders failed")
	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Rcode = dns.RcodeServerFailure

	return msg, nil
}

func parseDNSAddress(
	addr string,
) (network string, target string) {
	switch {
	case strings.HasPrefix(addr, "udp://"):
		return "udp",
			strings.TrimPrefix(addr, "udp://")

	case strings.HasPrefix(addr, "tcp://"):
		return "tcp",
			strings.TrimPrefix(addr, "tcp://")

	case strings.HasPrefix(addr, "tls://"):
		return "tcp-tls",
			strings.TrimPrefix(addr, "tls://")

	default:
		return "udp", addr
	}
}

func normalizeDNSAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}

	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}

	return net.JoinHostPort(addr, "53")
}

func nxdomain(req *dns.Msg) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Rcode = dns.RcodeNameError
	return msg
}

type patternScore struct {
	exact         bool
	wildcardCount int
	literalCount  int
	length        int
	pattern       string
}

type recordGroup struct {
	score   patternScore
	records []recordRow
}

func buildAnswers(zoneName, qname string, qtype uint16, records []recordRow) []dns.RR {
	matches := selectBestRecordSet(zoneName, qname, qtype, records)
	if len(matches) == 0 {
		return nil
	}

	if cname := firstRecordOfType(matches, models.CNAME); cname != nil {
		return convertRecords(zoneName, qname, []recordRow{*cname})
	}

	return convertRecords(zoneName, qname, matches)
}

func selectBestRecordSet(zoneName, qname string, qtype uint16, records []recordRow) []recordRow {
	zoneName = normalizeDNSName(zoneName)
	qname = normalizeDNSName(qname)

	groups := make(map[string]*recordGroup)

	for _, r := range records {
		pattern := recordPattern(zoneName, r.Name)
		ok, err := path.Match(pattern, qname)
		if err != nil || !ok {
			continue
		}

		if qtype != dns.TypeANY {
			if r.Type != models.CNAME {
				rtype, ok := dns.StringToType[string(r.Type)]
				if !ok && r.Type != models.DNSRecordType("TLSA") {
					continue
				}
				if r.Type != models.DNSRecordType("TLSA") && rtype != qtype {
					continue
				}
			}
		}

		group := groups[pattern]
		if group == nil {
			group = &recordGroup{score: specificityOf(pattern, qname)}
			groups[pattern] = group
		}
		group.records = append(group.records, r)
	}

	var best *recordGroup
	for _, group := range groups {
		if best == nil || moreSpecific(group.score, best.score) {
			best = group
		}
	}

	if best == nil {
		return nil
	}

	return best.records
}

func specificityOf(pattern, qname string) patternScore {
	wildcards := strings.Count(pattern, "*") + strings.Count(pattern, "?") + strings.Count(pattern, "[")
	literals := len([]rune(pattern)) - wildcards
	return patternScore{
		exact:         pattern == qname,
		wildcardCount: wildcards,
		literalCount:  literals,
		length:        len([]rune(pattern)),
		pattern:       pattern,
	}
}

func moreSpecific(a, b patternScore) bool {
	if a.exact != b.exact {
		return a.exact
	}
	if a.wildcardCount != b.wildcardCount {
		return a.wildcardCount < b.wildcardCount
	}
	if a.literalCount != b.literalCount {
		return a.literalCount > b.literalCount
	}
	if a.length != b.length {
		return a.length > b.length
	}
	return a.pattern < b.pattern
}

func convertRecords(zoneName, qname string, records []recordRow) []dns.RR {
	answers := make([]dns.RR, 0, len(records))
	for _, r := range records {
		rr, ok := recordToRR(zoneName, qname, r)
		if !ok {
			continue
		}
		answers = append(answers, rr)
	}

	return answers
}

func firstRecordOfType(records []recordRow, typ models.DNSRecordType) *recordRow {
	for i := range records {
		if records[i].Type == typ {
			return &records[i]
		}
	}
	return nil
}

func recordPattern(zoneName, recordName string) string {
	recordName = normalizeDNSName(recordName)
	zoneName = normalizeDNSName(zoneName)

	switch recordName {
	case "", "@":
		return zoneName
	}

	if recordName == zoneName || strings.HasSuffix(recordName, "."+zoneName) {
		return recordName
	}

	return recordName + "." + zoneName
}

func recordToRR(zoneName, qname string, r recordRow) (dns.RR, bool) {
	owner := ownerName(zoneName, qname, r.Name)
	ttl := r.TTL
	if ttl == 0 {
		ttl = 300
	}

	rtype, ok := dns.StringToType[string(r.Type)]
	if !ok {
		if r.Type == models.DNSRecordType("TLSA") {
			rtype = dns.TypeTLSA
		} else {
			rtype = dns.TypeTXT
		}
	}

	hdr := dns.RR_Header{
		Name:   owner,
		Rrtype: rtype,
		Class:  dns.ClassINET,
		Ttl:    ttl,
	}

	switch r.Type {
	case models.A:
		ip := net.ParseIP(strings.TrimSpace(r.Value))
		if ip == nil {
			return nil, false
		}
		return &dns.A{Hdr: hdr, A: ip.To4()}, true
	case models.AAAA:
		ip := net.ParseIP(strings.TrimSpace(r.Value))
		if ip == nil {
			return nil, false
		}
		return &dns.AAAA{Hdr: hdr, AAAA: ip}, true
	case models.CNAME:
		return &dns.CNAME{Hdr: hdr, Target: fqdn(r.Value)}, true
	case models.NS:
		return &dns.NS{Hdr: hdr, Ns: fqdn(r.Value)}, true
	case models.PTR:
		return &dns.PTR{Hdr: hdr, Ptr: fqdn(r.Value)}, true
	case models.MX:
		return &dns.MX{Hdr: hdr, Preference: uint16(r.Priority), Mx: fqdn(r.Value)}, true
	case models.TXT:
		return &dns.TXT{Hdr: hdr, Txt: []string{r.Value}}, true
	case models.CAA:
		return parseCAA(hdr, r.Value)
	case models.SOA:
		return parseSOA(hdr, zoneName, r)
	case models.SRV:
		return parseSRV(hdr, r)
	case models.DNSRecordType("TLSA"):
		return parseTLSA(hdr, r)
	case models.DNSKEY, models.DS, models.RRSIG, models.NSEC, models.NSEC3, models.TLS:
		return &dns.TXT{Hdr: hdr, Txt: []string{r.Value}}, true
	default:
		return &dns.TXT{Hdr: hdr, Txt: []string{r.Value}}, true
	}
}

func parseTLSA(hdr dns.RR_Header, r recordRow) (dns.RR, bool) {
	parts := strings.Fields(r.Value)
	if len(parts) != 4 {
		return nil, false
	}

	usage, err1 := strconv.ParseUint(parts[0], 10, 8)
	selector, err2 := strconv.ParseUint(parts[1], 10, 8)
	matchingType, err3 := strconv.ParseUint(parts[2], 10, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, false
	}

	return &dns.TLSA{
		Hdr:          hdr,
		Usage:        uint8(usage),
		Selector:     uint8(selector),
		MatchingType: uint8(matchingType),
		Certificate:  parts[3],
	}, true
}

func ownerName(zoneName, qname, recordName string) string {
	recordName = normalizeDNSName(recordName)
	zoneName = normalizeDNSName(zoneName)
	qname = normalizeDNSName(qname)

	switch recordName {
	case "", "@", zoneName:
		return fqdn(zoneName)
	}

	if strings.HasSuffix(recordName, "."+zoneName) {
		return fqdn(recordName)
	}

	return fqdn(recordName + "." + zoneName)
}

func normalizeDNSName(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
}

func fqdn(name string) string {
	name = normalizeDNSName(name)
	if name == "" {
		return "."
	}
	return name + "."
}

func parseCAA(hdr dns.RR_Header, value string) (dns.RR, bool) {
	parts := strings.Fields(value)
	if len(parts) < 3 {
		return nil, false
	}

	flags, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, false
	}

	tag := parts[1]
	data := strings.Join(parts[2:], " ")
	data = strings.Trim(data, `"`)

	return &dns.CAA{Hdr: hdr, Flag: uint8(flags), Tag: tag, Value: data}, true
}

func parseSOA(hdr dns.RR_Header, zoneName string, r recordRow) (dns.RR, bool) {
	parts := strings.Fields(r.Value)
	if len(parts) >= 7 {
		serial, err1 := strconv.ParseUint(parts[2], 10, 32)
		refresh, err2 := strconv.ParseUint(parts[3], 10, 32)
		retry, err3 := strconv.ParseUint(parts[4], 10, 32)
		expire, err4 := strconv.ParseUint(parts[5], 10, 32)
		minimum, err5 := strconv.ParseUint(parts[6], 10, 32)
		if err1 == nil && err2 == nil && err3 == nil && err4 == nil && err5 == nil {
			return &dns.SOA{
				Hdr:     hdr,
				Ns:      fqdn(parts[0]),
				Mbox:    fqdn(parts[1]),
				Serial:  uint32(serial),
				Refresh: uint32(refresh),
				Retry:   uint32(retry),
				Expire:  uint32(expire),
				Minttl:  uint32(minimum),
			}, true
		}
	}

	ttl := r.TTL
	if ttl == 0 {
		ttl = 300
	}

	serial := uint32(time.Now().Unix())
	return &dns.SOA{
		Hdr:     hdr,
		Ns:      fqdn("ns1." + zoneName),
		Mbox:    fqdn("hostmaster." + zoneName),
		Serial:  serial,
		Refresh: 3600,
		Retry:   600,
		Expire:  86400,
		Minttl:  ttl,
	}, true
}

func parseSRV(hdr dns.RR_Header, r recordRow) (dns.RR, bool) {
	parts := strings.Fields(r.Value)
	if len(parts) < 3 {
		return nil, false
	}

	priority, err1 := strconv.ParseUint(parts[0], 10, 16)
	weight, err2 := strconv.ParseUint(parts[1], 10, 16)
	port, err3 := strconv.ParseUint(parts[2], 10, 16)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, false
	}

	target := ""
	if len(parts) > 3 {
		target = strings.Join(parts[3:], " ")
	}

	return &dns.SRV{
		Hdr:      hdr,
		Priority: uint16(priority),
		Weight:   uint16(weight),
		Port:     uint16(port),
		Target:   fqdn(target),
	}, true
}
