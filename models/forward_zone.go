package models

import "time"

type ForwardZoneOption struct {
	ID        string
	Name      string
	Addresses []string
	ZoneCount int
	CreatedAt time.Time
	UpdatedAt time.Time
}
