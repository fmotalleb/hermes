package queries

import (
	"database/sql"
	"encoding/json"

	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/models"
)

const getForwardZonesQuery = `
SELECT
  id,
  name,
  COALESCE(addresses, '[]'::jsonb) AS addresses,
  (
    SELECT COUNT(*)
    FROM zones z
    WHERE z.forward_policy = 'custom'
      AND z.forward_zone_id = forward_zones.id
  ) AS zone_count,
  created_at,
  updated_at
FROM forward_zones
ORDER BY name
LIMIT $1 OFFSET $2;`

const getForwardZoneQuery = `
SELECT
  id,
  name,
  COALESCE(addresses, '[]'::jsonb) AS addresses,
  (
    SELECT COUNT(*)
    FROM zones z
    WHERE z.forward_policy = 'custom'
      AND z.forward_zone_id = forward_zones.id
  ) AS zone_count,
  created_at,
  updated_at
FROM forward_zones
WHERE id = $1;`

const createForwardZoneQuery = `
INSERT INTO forward_zones (name, addresses)
VALUES ($1, COALESCE($2::jsonb, '[]'::jsonb))
RETURNING
  id,
  name,
  addresses,
  (
    SELECT COUNT(*)
    FROM zones z
    WHERE z.forward_policy = 'custom'
      AND z.forward_zone_id = forward_zones.id
  ) AS zone_count,
  created_at,
  updated_at;`

const updateForwardZoneQuery = `
UPDATE forward_zones
SET
  name = $1,
  addresses = COALESCE($2::jsonb, addresses)
WHERE id = $3
RETURNING
  id,
  name,
  addresses,
  (
    SELECT COUNT(*)
    FROM zones z
    WHERE z.forward_policy = 'custom'
      AND z.forward_zone_id = forward_zones.id
  ) AS zone_count,
  created_at,
  updated_at;`

const deleteForwardZoneQuery = `DELETE FROM forward_zones WHERE id = $1;`
const detachForwardZoneFromZonesQuery = `
UPDATE zones
SET
  forward_policy = 'none',
  forward_zone_id = NULL
WHERE forward_policy = 'custom' AND forward_zone_id = $1;`

func scanForwardZone(row scanner) (models.ForwardZone, error) {
	var zone models.ForwardZone
	var addresses []byte
	if err := row.Scan(
		&zone.ID,
		&zone.Name,
		&addresses,
		&zone.ZoneCount,
		&zone.CreatedAt,
		&zone.UpdatedAt,
	); err != nil {
		return models.ForwardZone{}, err
	}

	if len(addresses) > 0 {
		if err := json.Unmarshal(addresses, &zone.Addresses); err != nil {
			return models.ForwardZone{}, err
		}
	} else {
		zone.Addresses = make([]models.ForwardAddress, 0)
	}

	return zone, nil
}

func GetForwardZones(ctx *gofr.Context, limit, offset uint32) ([]models.ForwardZone, error) {
	rows, err := ctx.SQL.QueryContext(ctx, getForwardZonesQuery, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	zones := make([]models.ForwardZone, 0)
	for rows.Next() {
		zone, err := scanForwardZone(rows)
		if err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}

	return zones, rows.Err()
}

func GetForwardZone(ctx *gofr.Context, id string) (models.ForwardZone, error) {
	row := ctx.SQL.QueryRowContext(ctx, getForwardZoneQuery, id)
	return scanForwardZone(row)
}

func CreateForwardZone(ctx *gofr.Context, name string, addresses []models.ForwardAddress) (models.ForwardZone, error) {
	buf, err := json.Marshal(addresses)
	if err != nil {
		return models.ForwardZone{}, err
	}
	row := ctx.SQL.QueryRowContext(ctx, createForwardZoneQuery, name, buf)
	return scanForwardZone(row)
}

func UpdateForwardZone(ctx *gofr.Context, id, name string, addresses []models.ForwardAddress) (models.ForwardZone, error) {
	buf, err := json.Marshal(addresses)
	if err != nil {
		return models.ForwardZone{}, err
	}
	row := ctx.SQL.QueryRowContext(ctx, updateForwardZoneQuery, name, buf, id)
	return scanForwardZone(row)
}

func DeleteForwardZone(ctx *gofr.Context, id string) error {
	result, err := ctx.SQL.ExecContext(ctx, deleteForwardZoneQuery, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func DetachForwardZoneFromZones(ctx *gofr.Context, id string) error {
	_, err := ctx.SQL.ExecContext(ctx, detachForwardZoneFromZonesQuery, id)
	return err
}
