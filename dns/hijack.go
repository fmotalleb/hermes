package dns

import (
	"context"
	"net"
	"path"
	"strings"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/fmotalleb/hermes/models"
)

func (h *handler) hijack(ctx context.Context, qname string, qtype uint16, req *dns.Msg) (*dns.Msg, bool) {
	hijacks, ok := h.lookupHijacks(ctx, models.DNSRecordType(dns.TypeToString[qtype]))
	if !ok {
		return nil, false
	}

	hr, ok := selectBestHijack(qname, hijacks)
	if !ok {
		return nil, false
	}

	h.m().hijacks.Add(ctx, 1, metric.WithAttributes(
		attribute.String("policy", hr.Policy),
	))

	switch hr.Policy {
	case models.HijackPolicyBlock:
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.Answer = []dns.RR{
			&dns.NXNAME{},
		}
		return msg, true
	case models.HijackPolicyRaw:
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.Answer = convertRecords(qname, qname, []record{
			{
				Name:     qname,
				Type:     hr.Type,
				Value:    hr.Value,
				TTL:      hr.TTL,
				Priority: 0,
			},
		})
		return msg, true
	case models.HijackPolicyProxy:
		proxies, err := h.getProxyServices(ctx)
		msg := new(dns.Msg)
		msg.SetReply(req)
		if err != nil {
			msg.Answer = []dns.RR{
				&dns.NXNAME{},
			}
			return msg, true
		}

		msg.Answer = convertRecords(qname, qname, convertProxiesToRecords(qname, hr, proxies))
		return msg, true
		// panic("unhandled state")
	case models.HijackPolicyForward:
		ans, _ := h.forward(ctx, req, *hr.ForwardZoneID)
		return ans, true
	}
	return nil, false
}

func convertProxiesToRecords(qname string, hr hijackRow, addrs []net.IP) []record {
	records := make([]record, 0)
	for _, v := range addrs {
		addrKind := addrKind(v)
		if addrKind == nil {
			continue
		}
		if *addrKind == hr.Type {
			records = append(records,
				record{
					Name:     qname,
					Type:     hr.Type,
					Value:    v.String(),
					TTL:      hr.TTL,
					Priority: 0,
				},
			)
		}
	}
	return records
}

func selectBestHijack(qname string, hijacks []hijackRow) (hijackRow, bool) {
	normalizedQname := normalizeDNSName(qname)

	var bestHijack *hijackRow
	var bestScore hijackPatternScore

	for i := range hijacks {
		hr := hijacks[i]
		pattern := normalizeDNSName(hr.Name)

		ok, err := path.Match(pattern, normalizedQname)
		if err != nil || !ok {
			continue
		}

		currentScore := specificityOfHijack(pattern, normalizedQname)

		if bestHijack == nil || moreSpecificHijack(currentScore, bestScore) {
			bestHijack = &hr
			bestScore = currentScore
		}
	}

	if bestHijack == nil {
		return hijackRow{}, false
	}

	return *bestHijack, true
}

type hijackPatternScore struct {
	exact         bool
	wildcardCount int
	literalCount  int
	length        int
	pattern       string
}

func specificityOfHijack(pattern, qname string) hijackPatternScore {
	wildcards := strings.Count(pattern, "*") + strings.Count(pattern, "?") + strings.Count(pattern, "[")
	literals := len([]rune(pattern)) - wildcards
	return hijackPatternScore{
		exact:         pattern == qname,
		wildcardCount: wildcards,
		literalCount:  literals,
		length:        len([]rune(pattern)),
		pattern:       pattern,
	}
}

func moreSpecificHijack(a, b hijackPatternScore) bool {
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

func addrKind(ip net.IP) *models.DNSRecordType {
	switch {
	case ip == nil:
		return nil
	case ip.To4() != nil:
		return new(models.A)
	case ip.To16() != nil:
		return new(models.AAAA)
	}
	return nil
}
