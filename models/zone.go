package models

import "time"

type ZoneData struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	ForwardPolicy ForwardPolicy `json:"forward_policy"`
	ForwardZoneID string        `json:"forward_zone,omitempty"`
	TTL           uint32        `json:"ttl"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	RecordCount   int           `json:"record_count,omitempty"`
}
