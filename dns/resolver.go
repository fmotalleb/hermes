package dns

import (
	"path"
	"strings"

	"github.com/miekg/dns"

	"github.com/fmotalleb/hermes/models"
)

type patternScore struct {
	exact         bool
	wildcardCount int
	literalCount  int
	length        int
	pattern       string
}

type recordGroup struct {
	score   patternScore
	records []record
}

func buildAnswers(zoneName, qname string, qtype uint16, records []record) []dns.RR {
	matches := selectBestRecordSet(zoneName, qname, qtype, records)
	if len(matches) == 0 {
		return nil
	}

	if cname := firstRecordOfType(matches, models.CNAME); cname != nil {
		return convertRecords(zoneName, qname, []record{*cname})
	}

	return convertRecords(zoneName, qname, matches)
}

func selectBestRecordSet(zoneName, qname string, qtype uint16, records []record) []record {
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

func convertRecords(zoneName, qname string, records []record) []dns.RR {
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

func firstRecordOfType(records []record, typ models.DNSRecordType) *record {
	for i := range records {
		if records[i].Type == typ {
			return &records[i]
		}
	}
	return nil
}
