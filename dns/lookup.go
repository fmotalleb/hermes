package dns

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/fmotalleb/hermes/models"
	"github.com/miekg/dns"
)

func (h *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeFormatError)
		_ = w.WriteMsg(msg)
		return
	}

	q := r.Question[0]
	resp, err := h.lookup(context.Background(), q.Name, q.Qtype, r)
	if err != nil {
		h.logger.Error("dns lookup failed", err)
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		_ = w.WriteMsg(msg)
		return
	}

	_ = w.WriteMsg(resp)
}

func (h *handler) lookup(ctx context.Context, qname string, qtype uint16, req *dns.Msg) (*dns.Msg, error) {
	zone, err := h.store.findZone(ctx, qname)
	if err != nil {
		if isNoRows(err) {
			return nxdomain(req), nil
		}
		return nil, err
	}

	records, err := h.store.recordsForZone(ctx, zone.ID)
	if err != nil {
		return nil, err
	}

	answers := buildAnswers(zone.Name, qname, qtype, records)
	if len(answers) > 0 {
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.Authoritative = true
		msg.Answer = answers
		msg.RecursionAvailable = zone.ForwardZoneID == ""
		return msg, nil
	}

	if zone.ForwardZoneID != "" {
		return h.forward(ctx, req, zone.ForwardZoneID)
	}

	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Authoritative = true
	msg.RecursionAvailable = false
	return msg, nil
}

func (h *handler) forward(ctx context.Context, req *dns.Msg, forwardZoneID string) (*dns.Msg, error) {
	fz, err := h.store.findForwardZone(ctx, forwardZoneID)
	if err != nil {
		return nil, err
	}

	client := &dns.Client{Timeout: 3 * time.Second}

	for _, addr := range fz.Addresses {
		target := normalizeDNSAddress(addr)
		resp, _, err := client.ExchangeContext(ctx, req, target)
		if err == nil && resp != nil {
			return resp, nil
		}
	}

	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Rcode = dns.RcodeServerFailure
	return msg, nil
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

func buildAnswers(zoneName, qname string, qtype uint16, records []recordRow) []dns.RR {
	matches := matchRecords(zoneName, qname, qtype, records)
	if len(matches) == 0 {
		return nil
	}

	if cname := firstRecordOfType(matches, models.CNAME); cname != nil {
		if rr, ok := recordToRR(zoneName, qname, *cname); ok {
			return []dns.RR{rr}
		}
		return nil
	}

	answers := make([]dns.RR, 0, len(matches))
	for _, r := range matches {
		rr, ok := recordToRR(zoneName, qname, r)
		if !ok {
			continue
		}
		answers = append(answers, rr)
	}

	return answers
}

func matchRecords(zoneName, qname string, qtype uint16, records []recordRow) []recordRow {
	qname = normalizeDNSName(qname)
	zoneName = normalizeDNSName(zoneName)

	candidates := recordNameCandidates(zoneName, qname)
	matches := make([]recordRow, 0)

	for _, r := range records {
		name := normalizeDNSName(r.Name)
		if !candidateMatch(name, candidates) {
			continue
		}

		rtype, ok := dns.StringToType[string(r.Type)]
		if !ok {
			continue
		}

		if qtype != dns.TypeANY && rtype != qtype && r.Type != models.CNAME {
			continue
		}

		matches = append(matches, r)
	}

	return matches
}

func candidateMatch(name string, candidates map[string]struct{}) bool {
	_, ok := candidates[name]
	return ok
}

func recordNameCandidates(zoneName, qname string) map[string]struct{} {
	candidates := map[string]struct{}{
		qname:             {},
		zoneName:          {},
		"@":               {},
		"":                {},
		wildcardOf(qname): {},
	}

	if qname == zoneName {
		return candidates
	}

	if strings.HasSuffix(qname, "."+zoneName) {
		rel := strings.TrimSuffix(qname, "."+zoneName)
		candidates[rel] = struct{}{}
		candidates[wildcardOf(rel)] = struct{}{}
	}

	return candidates
}

func wildcardOf(name string) string {
	if name == "" || name == "@" {
		return name
	}
	return "*." + name
}

func firstRecordOfType(records []recordRow, typ models.DNSRecordType) *recordRow {
	for i := range records {
		if records[i].Type == typ {
			return &records[i]
		}
	}
	return nil
}

func recordToRR(zoneName, qname string, r recordRow) (dns.RR, bool) {
	owner := ownerName(zoneName, qname, r.Name)
	ttl := r.TTL
	if ttl == 0 {
		ttl = 300
	}

	hdr := dns.RR_Header{
		Name:   owner,
		Rrtype: dns.StringToType[string(r.Type)],
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
	case models.DNSKEY, models.DS, models.RRSIG, models.NSEC, models.NSEC3, models.TLS:
		return &dns.TXT{Hdr: hdr, Txt: []string{r.Value}}, true
	default:
		return &dns.TXT{Hdr: hdr, Txt: []string{r.Value}}, true
	}
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

	weight, err1 := strconv.ParseUint(parts[0], 10, 16)
	port, err2 := strconv.ParseUint(parts[1], 10, 16)
	if err1 != nil || err2 != nil {
		return nil, false
	}

	return &dns.SRV{
		Hdr:      hdr,
		Priority: uint16(r.Priority),
		Weight:   uint16(weight),
		Port:     uint16(port),
		Target:   fqdn(strings.Join(parts[2:], " ")),
	}, true
}
