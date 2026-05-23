package dns

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"

	"github.com/fmotalleb/hermes/models"
)

type store struct {
	db     container.DB
	logger logging.Logger
}

type zoneRow struct {
	ID            string
	Name          string
	ForwardPolicy string
	ForwardZoneID string
	TTL           uint32
}

type settingsRow struct {
	DefaultForwardZoneID string
}

type forwardZoneRow struct {
	ID        string
	Name      string
	Addresses []string
}

type recordRow struct {
	Name     string
	Type     models.DNSRecordType
	Value    string
	TTL      uint32
	Priority uint32
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
  COALESCE(addresses, '{}') AS addresses
FROM forward_zones
WHERE id = $1;`

	var zone forwardZoneRow
	var addresses pq.StringArray
	if err := s.db.QueryRowContext(ctx, query, id).Scan(
		&zone.ID,
		&zone.Name,
		&addresses,
	); err != nil {
		return forwardZoneRow{}, err
	}
	zone.Addresses = []string(addresses)

	return zone, nil
}

func (s *store) recordsForZone(ctx context.Context, zoneID string) ([]recordRow, error) {
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

	records := make([]recordRow, 0)
	for rows.Next() {
		var r recordRow
		if err := rows.Scan(&r.Name, &r.Type, &r.Value, &r.TTL, &r.Priority); err != nil {
			return nil, err
		}
		records = append(records, r)
	}

	return records, rows.Err()
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
