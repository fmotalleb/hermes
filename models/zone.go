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
