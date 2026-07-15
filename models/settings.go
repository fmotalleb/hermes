package models

import "time"

// Settings represents the global application settings, including the default forward zone.
type Settings struct {
	DefaultForwardZoneID string    `json:"default_forward_zone_id,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
