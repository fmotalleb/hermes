package models

import "time"

type ForwardZoneOption struct {
	ID        string
	Name      string
	Addresses []string
	CreatedAt time.Time
	UpdatedAt time.Time
}
