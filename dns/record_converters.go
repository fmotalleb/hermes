package dns

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/fmotalleb/hermes/models"
)

func recordToRR(zoneName, qname string, r record) (dns.RR, bool) {
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

func parseTLSA(hdr dns.RR_Header, r record) (dns.RR, bool) {
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

func parseSOA(hdr dns.RR_Header, zoneName string, r record) (dns.RR, bool) {
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

func parseSRV(hdr dns.RR_Header, r record) (dns.RR, bool) {
	parts := strings.Fields(r.Value)
	if len(parts) < 3 {
		return nil, false
	}

	var (
		priority uint64
		weight   uint64
		port     uint64
		err1     error
		err2     error
		err3     error
	)

	if r.Priority != 0 {
		priority = uint64(r.Priority)
		weight, err1 = strconv.ParseUint(parts[0], 10, 16)
		port, err2 = strconv.ParseUint(parts[1], 10, 16)
		target := strings.Join(parts[2:], " ")
		if target == "" {
			err3 = strconv.ErrSyntax
		}
		if err1 != nil || err2 != nil || err3 != nil {
			return nil, false
		}

		return &dns.SRV{
			Hdr:      hdr,
			Priority: uint16(priority),
			Weight:   uint16(weight),
			Port:     uint16(port),
			Target:   fqdn(target),
		}, true
	} else {
		if len(parts) < 4 {
			return nil, false
		}
		priority, err1 = strconv.ParseUint(parts[0], 10, 16)
		weight, err2 = strconv.ParseUint(parts[1], 10, 16)
		port, err3 = strconv.ParseUint(parts[2], 10, 16)
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
}
