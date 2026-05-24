package api

import (
	"errors"
	"strings"

	"github.com/fmotalleb/hermes/models"
)

type forwardZoneRequest struct {
	Name      string                 `json:"name"`
	Addresses []models.ForwardAddress `json:"addresses"`
}

type zoneRequest struct {
	Name          string  `json:"name"`
	ForwardPolicy *string `json:"forward_policy,omitempty"`
	ForwardZoneID *string `json:"forward_zone_id,omitempty"`
	TTL           *uint32 `json:"ttl,omitempty"`
}

type recordRequest struct {
	Name     string               `json:"name"`
	Type     models.DNSRecordType `json:"type"`
	Value    string               `json:"value"`
	TTL      *uint32              `json:"ttl,omitempty"`
	Priority *uint32              `json:"priority,omitempty"`
}

type settingsRequest struct {
	DefaultForwardZoneID *string `json:"default_forward_zone_id,omitempty"`
}

var errInvalidRecordType = errors.New("invalid record type")

func normalizeName(name string) string {
	return strings.TrimSpace(name)
}

func validRecordType(t models.DNSRecordType) bool {
	switch t {
	case models.A, models.AAAA, models.CNAME, models.MX, models.NS, models.PTR, models.SOA,
		models.SRV, models.TXT, models.CAA, models.NAPTR, models.DNSKEY, models.DS,
		models.RRSIG, models.NSEC, models.NSEC3, models.TLS:
		return true
	default:
		return false
	}
}
