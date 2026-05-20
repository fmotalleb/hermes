package queries

import (
	"database/sql"

	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/models"
)

const getForwardZonesQuery = `
SELECT
  id,
  name,
  COALESCE(addresses, '{}') AS addresses,
  (
    SELECT COUNT(*)
    FROM zones z
    WHERE z.forward_zone_id = forward_zones.id
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
  COALESCE(addresses, '{}') AS addresses,
  (
    SELECT COUNT(*)
    FROM zones z
    WHERE z.forward_zone_id = forward_zones.id
  ) AS zone_count,
  created_at,
  updated_at
FROM forward_zones
WHERE id = $1;`

const createForwardZoneQuery = `
INSERT INTO forward_zones (name, addresses)
VALUES ($1, COALESCE($2::text[], '{}'))
RETURNING
  id,
  name,
  addresses,
  (
    SELECT COUNT(*)
    FROM zones z
    WHERE z.forward_zone_id = forward_zones.id
  ) AS zone_count,
  created_at,
  updated_at;`

const updateForwardZoneQuery = `
UPDATE forward_zones
SET
  name = $1,
  addresses = COALESCE($2::text[], addresses)
WHERE id = $3
RETURNING
  id,
  name,
  addresses,
  (
    SELECT COUNT(*)
    FROM zones z
    WHERE z.forward_zone_id = forward_zones.id
  ) AS zone_count,
  created_at,
  updated_at;`

const deleteForwardZoneQuery = `DELETE FROM forward_zones WHERE id = $1;`

func scanForwardZone(row scanner) (models.ForwardZoneOption, error) {
	var zone models.ForwardZoneOption
	if err := row.Scan(
		&zone.ID,
		&zone.Name,
		&zone.Addresses,
		&zone.ZoneCount,
		&zone.CreatedAt,
		&zone.UpdatedAt,
	); err != nil {
		return models.ForwardZoneOption{}, err
	}

	return zone, nil
}

func GetForwardZones(ctx *gofr.Context, limit, offset uint32) ([]models.ForwardZoneOption, error) {
	rows, err := ctx.SQL.QueryContext(ctx, getForwardZonesQuery, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	zones := make([]models.ForwardZoneOption, 0)
	for rows.Next() {
		zone, err := scanForwardZone(rows)
		if err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}

	return zones, rows.Err()
}

func GetForwardZone(ctx *gofr.Context, id string) (models.ForwardZoneOption, error) {
	row := ctx.SQL.QueryRowContext(ctx, getForwardZoneQuery, id)
	return scanForwardZone(row)
}

func CreateForwardZone(ctx *gofr.Context, name string, addresses []string) (models.ForwardZoneOption, error) {
	row := ctx.SQL.QueryRowContext(ctx, createForwardZoneQuery, name, addresses)
	return scanForwardZone(row)
}

func UpdateForwardZone(ctx *gofr.Context, id, name string, addresses []string) (models.ForwardZoneOption, error) {
	row := ctx.SQL.QueryRowContext(ctx, updateForwardZoneQuery, name, addresses, id)
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
