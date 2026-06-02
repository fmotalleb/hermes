package api

import (
	"errors"
	"strings"

	"github.com/fmotalleb/hermes/models"
)

type forwardZoneRequest struct {
	Name      string                  `json:"name"`
	Addresses []models.ForwardAddress `json:"addresses"`
}

type zoneRequest struct {
	Name          string                `json:"name"`
	ForwardPolicy *models.ForwardPolicy `json:"forward_policy,omitempty"`
	ForwardZoneID *string               `json:"forward_zone_id,omitempty"`
	TTL           *uint32               `json:"ttl,omitempty"`
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

var errInvalidHijackPolicy = errors.New("invalid hijack policy")

type hijackRequest struct {
	Name          string               `json:"name"`
	Value         string               `json:"value"`
	Type          models.DNSRecordType `json:"type"`
	Policy        models.HijackPolicy  `json:"policy"`
	ForwardPolicy models.ForwardPolicy `json:"forward_policy,omitempty"`
	ForwardZoneID *string              `json:"forward_zone_id,omitempty"`
	TTL           *uint32              `json:"ttl,omitempty"`
}

func validHijackPolicy(p models.HijackPolicy) bool {
	switch p {
	case models.HijackPolicyBlock,
		models.HijackPolicyProxy,
		models.HijackPolicyRaw:
		return true
	default:
		return false
	}
}

func normalizeHijackPolicy(p string) string {
	return strings.TrimSpace(strings.ToLower(p))
}

func validateHijackRequest(req hijackRequest) error {
	req.Name = normalizeName(req.Name)
	req.Value = normalizeName(req.Value)

	if req.Name == "" {
		return errors.New("hijack name is required")
	}
	if req.Value == "" {
		return errors.New("hijack value is required")
	}
	if !validRecordType(req.Type) {
		return errInvalidRecordType
	}
	if !validHijackPolicy(req.Policy) {
		return errInvalidHijackPolicy
	}

	return nil
}
