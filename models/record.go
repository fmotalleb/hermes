package models

import "time"

type DNSRecordType string

const (
	A      = DNSRecordType("A")
	AAAA   = DNSRecordType("AAAA")
	CNAME  = DNSRecordType("CNAME")
	MX     = DNSRecordType("MX")
	NS     = DNSRecordType("NS")
	PTR    = DNSRecordType("PTR")
	SOA    = DNSRecordType("SOA")
	SRV    = DNSRecordType("SRV")
	TXT    = DNSRecordType("TXT")
	CAA    = DNSRecordType("CAA")
	NAPTR  = DNSRecordType("NAPTR")
	DNSKEY = DNSRecordType("DNSKEY")
	DS     = DNSRecordType("DS")
	RRSIG  = DNSRecordType("RRSIG")
	NSEC   = DNSRecordType("NSEC")
	NSEC3  = DNSRecordType("NSEC3")
	TLS    = DNSRecordType("TLS")
)

type DNSRecord struct {
	ID        string        `json:"id"`
	ZoneID    string        `json:"zone_id"`
	Name      string        `json:"name"`
	Type      DNSRecordType `json:"type"`
	Value     string        `json:"value"`
	TTL       uint32        `json:"ttl"`
	Priority  uint32        `json:"priority"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}
