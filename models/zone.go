package models

import "time"

type ZoneData struct {
	ID            string
	Name          string
	ForwardZoneID string
	TTL           uint32
	CreatedAt     time.Time
	UpdatedAt     time.Time
	RecordCount   int
}

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
	ID        string
	ZoneID    string
	Name      string
	Type      DNSRecordType
	Value     string
	TTL       uint32
	Priority  uint32
	CreatedAt time.Time
	UpdatedAt time.Time
}
