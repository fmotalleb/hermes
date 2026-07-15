package dns

import (
	"strings"

	"github.com/miekg/dns"
)

func nxdomain(req *dns.Msg) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Rcode = dns.RcodeNameError
	return msg
}

func ownerName(zoneName, _, recordName string) string {
	recordName = normalizeDNSName(recordName)
	zoneName = normalizeDNSName(zoneName)

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
