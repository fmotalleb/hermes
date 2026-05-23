package models

import "time"

type Settings struct {
	DefaultForwardZoneID string    `json:"default_forward_zone_id,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
