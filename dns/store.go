package dns

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net"

	"github.com/fmotalleb/hermes/models"
	"github.com/fmotalleb/hermes/registry"
)

type store struct {
	db       *sql.DB
	logger   *slog.Logger
	registry *registry.RegistryConnection
}

type zoneRow struct {
	ID            string
	Name          string
	ForwardPolicy models.ForwardPolicy
	ForwardZoneID string
	TTL           uint32
}

type settingsRow struct {
	DefaultForwardZoneID string
}

type forwardZoneRow struct {
	ID        string
	Name      string
	Addresses []models.ForwardAddress
}

type record struct {
	Name     string
	Type     models.DNSRecordType
	Value    string
	TTL      uint32
	Priority uint32
}

type hijackRow struct {
	ID            string
	Name          string
	Value         string
	Type          models.DNSRecordType
	Policy        models.HijackPolicy
	ForwardPolicy models.ForwardPolicy
	ForwardZoneID *string
	TTL           uint32
}

func (s *store) findZone(ctx context.Context, qname string) (zoneRow, error) {
	const query = `
SELECT
  id,
  name,
  forward_policy,
  COALESCE(forward_zone_id::text, '') AS forward_zone_id,
  ttl
FROM zones
WHERE LOWER($1) = LOWER(name)
   OR LOWER($1) LIKE '%' || '.' || LOWER(name)
ORDER BY CHAR_LENGTH(name) DESC
LIMIT 1;`
	var zone zoneRow
	if err := s.db.QueryRowContext(ctx, query, qname).Scan(
		&zone.ID,
		&zone.Name,
		&zone.ForwardPolicy,
		&zone.ForwardZoneID,
		&zone.TTL,
	); err != nil {
		return zoneRow{}, err
	}

	return zone, nil
}

func (s *store) loadSettings(ctx context.Context) (settingsRow, error) {
	const query = `
SELECT
  COALESCE(default_forward_zone_id::text, '') AS default_forward_zone_id
FROM settings
WHERE id = 1;`

	var settings settingsRow
	if err := s.db.QueryRowContext(ctx, query).Scan(&settings.DefaultForwardZoneID); err != nil {
		return settingsRow{}, err
	}

	return settings, nil
}

func (s *store) findForwardZone(ctx context.Context, id string) (forwardZoneRow, error) {
	const query = `
SELECT
  id,
  name,
  COALESCE(addresses, '[]'::jsonb) AS addresses
FROM forward_zones
WHERE id = $1;`

	var zone forwardZoneRow
	var addresses []byte
	if err := s.db.QueryRowContext(ctx, query, id).Scan(
		&zone.ID,
		&zone.Name,
		&addresses,
	); err != nil {
		return forwardZoneRow{}, err
	}

	if len(addresses) > 0 {
		if err := json.Unmarshal(addresses, &zone.Addresses); err != nil {
			return forwardZoneRow{}, err
		}
	} else {
		zone.Addresses = make([]models.ForwardAddress, 0)
	}

	return zone, nil
}

func (s *store) recordsForZone(ctx context.Context, zoneID string) ([]record, error) {
	const query = `
SELECT
  name,
  type,
  value,
  ttl,
  priority
FROM records
WHERE zone_id = $1;`

	rows, err := s.db.QueryContext(ctx, query, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]record, 0)
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.Name, &r.Type, &r.Value, &r.TTL, &r.Priority); err != nil {
			return nil, err
		}
		records = append(records, r)
	}

	return records, rows.Err()
}

func (s *store) findHijacks(ctx context.Context, qtype models.DNSRecordType) ([]hijackRow, error) {
	const query = `
SELECT
  id,
  name,
  value,
  record_type,
  policy,
  COALESCE(forward_policy::text, '') AS forward_policy,
  COALESCE(forward_zone_id::text, '') AS forward_zone_id,
  ttl
FROM hijacks
WHERE record_type = $1;`

	rows, err := s.db.QueryContext(ctx, query, qtype)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hijacks := make([]hijackRow, 0)
	for rows.Next() {
		var h hijackRow
		if err := rows.Scan(
			&h.ID,
			&h.Name,
			&h.Value,
			&h.Type,
			&h.Policy,
			&h.ForwardPolicy,
			&h.ForwardZoneID,
			&h.TTL,
		); err != nil {
			return nil, err
		}
		hijacks = append(hijacks, h)
	}

	return hijacks, rows.Err()
}

func (s *store) lookupHijacks(ctx context.Context, qtype models.DNSRecordType) ([]hijackRow, bool) {
	hijacks, err := s.findHijacks(ctx, qtype)
	if err != nil {
		if isNoRows(err) {
			return nil, false
		}

		s.logger.Error("hijacks lookup failed", "error", err, "type", qtype)
		return nil, false
	}

	return hijacks, true
}

func (s *store) getProxyServices(ctx context.Context) ([]net.IP, error) {
	proxies, err := s.registry.ListKind(ctx, registry.ServiceKindProxy)
	if err != nil {
		return nil, err
	}

	ips := make([]net.IP, len(proxies))
	for i, v := range proxies {
		ips[i] = v.IP
	}
	return ips, nil
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
