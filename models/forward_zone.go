package models

import "time"

type ForwardZone struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Addresses []string  `json:"addresses"`
	ZoneCount int       `json:"zone_count,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
