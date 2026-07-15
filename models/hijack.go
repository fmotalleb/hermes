package models

import "time"

// HijackPolicy defines how a hijacked DNS query is handled.
type HijackPolicy = string

const (
	HijackPolicyBlock   = HijackPolicy("block")   // Return a synthetic NXDOMAIN response.
	HijackPolicyProxy   = HijackPolicy("proxy")   // Proxy the query through the transparent proxy.
	HijackPolicyForward = HijackPolicy("forward") // Forward the query to the designated forward zone.
	HijackPolicyRaw     = HijackPolicy("raw")     // Return the configured raw value directly.
)

// HijackRecord defines a DNS hijack rule that intercepts queries matching a name pattern.
type HijackRecord struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Value        string        `json:"value"`
	Type         DNSRecordType `json:"record_type"`
	HijackPolicy HijackPolicy  `json:"hijack_policy"`

	ForwardPolicy ForwardPolicy `json:"forward_policy"`
	ForwardZoneID *string       `json:"forward_zone_id,omitempty"`

	TTL uint32 `json:"ttl"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
