package models

import "time"

type HijackPolicy = string

const (
	HijackPolicyBlock   = HijackPolicy("block")
	HijackPolicyProxy   = HijackPolicy("proxy")
	HijackPolicyForward = HijackPolicy("forward")
	HijackPolicyRaw     = HijackPolicy("raw")
)

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
