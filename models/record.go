package models

import "time"

// DNSRecordType represents a DNS resource record type (A, AAAA, CNAME, etc.).
type DNSRecordType string

const (
	A      = DNSRecordType("A")      // A record: IPv4 address
	AAAA   = DNSRecordType("AAAA")   // AAAA record: IPv6 address
	CNAME  = DNSRecordType("CNAME")  // CNAME record: canonical name alias
	MX     = DNSRecordType("MX")     // MX record: mail exchange
	NS     = DNSRecordType("NS")     // NS record: authoritative nameserver
	PTR    = DNSRecordType("PTR")    // PTR record: reverse DNS pointer
	SOA    = DNSRecordType("SOA")    // SOA record: start of authority
	SRV    = DNSRecordType("SRV")    // SRV record: service locator
	TXT    = DNSRecordType("TXT")    // TXT record: text data
	CAA    = DNSRecordType("CAA")    // CAA record: certification authority authorization
	NAPTR  = DNSRecordType("NAPTR")  // NAPTR record: naming authority pointer
	DNSKEY = DNSRecordType("DNSKEY") // DNSKEY record: DNSSEC public key
	DS     = DNSRecordType("DS")     // DS record: DNSSEC delegation signer
	RRSIG  = DNSRecordType("RRSIG")  // RRSIG record: DNSSEC signature
	NSEC   = DNSRecordType("NSEC")   // NSEC record: DNSSEC next secure
	NSEC3  = DNSRecordType("NSEC3")  // NSEC3 record: DNSSEC hashed next secure
	TLS    = DNSRecordType("TLS")    // TLSA record: TLS certificate association
)

// DNSRecord represents a single DNS resource record within a zone.
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
