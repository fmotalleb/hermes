package queries

import (
	"database/sql"

	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/models"
)

const getZonesQuery = `
SELECT
  z.id,
  z.name,
  COALESCE(z.forward_zone_id::text, '') AS forward_zone_id,
  z.ttl,
  z.created_at,
  z.updated_at,
  (
    SELECT COUNT(*)
    FROM records r
    WHERE r.zone_id = z.id
  ) AS record_count
FROM zones z
ORDER BY z.name
LIMIT $1 OFFSET $2;`

const getZoneQuery = `
SELECT
  z.id,
  z.name,
  COALESCE(z.forward_zone_id::text, '') AS forward_zone_id,
  z.ttl,
  z.created_at,
  z.updated_at,
  (
    SELECT COUNT(*)
    FROM records r
    WHERE r.zone_id = z.id
  ) AS record_count
FROM zones z
WHERE z.id = $1;`

const createZoneQuery = `
INSERT INTO zones (name, forward_zone_id, ttl)
VALUES ($1, NULLIF($2::text, '')::uuid, COALESCE($3, 300))
RETURNING
  id,
  name,
  COALESCE(forward_zone_id::text, '') AS forward_zone_id,
  ttl,
  created_at,
  updated_at,
  (
    SELECT COUNT(*)
    FROM records r
    WHERE r.zone_id = zones.id
  ) AS record_count;`

const updateZoneQuery = `
UPDATE zones
SET
  name = $1,
  forward_zone_id = CASE
    WHEN $2::text IS NULL THEN forward_zone_id
    ELSE NULLIF($2::text, '')::uuid
  END,
  ttl = COALESCE($3, ttl)
WHERE id = $4
RETURNING
  id,
  name,
  COALESCE(forward_zone_id::text, '') AS forward_zone_id,
  ttl,
  created_at,
  updated_at,
  (
    SELECT COUNT(*)
    FROM records r
    WHERE r.zone_id = zones.id
  ) AS record_count;`

const deleteZoneQuery = `DELETE FROM zones WHERE id = $1;`

func scanZone(row scanner) (models.ZoneData, error) {
	var zone models.ZoneData
	if err := row.Scan(
		&zone.ID,
		&zone.Name,
		&zone.ForwardZoneID,
		&zone.TTL,
		&zone.CreatedAt,
		&zone.UpdatedAt,
		&zone.RecordCount,
	); err != nil {
		return models.ZoneData{}, err
	}

	return zone, nil
}

func GetZones(ctx *gofr.Context, limit, offset uint32) ([]models.ZoneData, error) {
	var rows *sql.Rows
	var err error
	if rows, err = ctx.SQL.QueryContext(ctx, getZonesQuery, limit, offset); err != nil {
		return nil, err
	}
	defer rows.Close()

	zones := make([]models.ZoneData, 0)

	for rows.Next() {
		var z models.ZoneData

		err := rows.Scan(
			&z.ID,
			&z.Name,
			&z.ForwardZoneID,
			&z.TTL,
			&z.CreatedAt,
			&z.UpdatedAt,
			&z.RecordCount,
		)
		if err != nil {
			return nil, err
		}

		zones = append(zones, z)
	}
	return zones, rows.Err()
}

func GetZone(ctx *gofr.Context, id string) (models.ZoneData, error) {
	row := ctx.SQL.QueryRowContext(ctx, getZoneQuery, id)
	return scanZone(row)
}

func CreateZone(ctx *gofr.Context, name string, forwardZoneID *string, ttl *uint32) (models.ZoneData, error) {
	row := ctx.SQL.QueryRowContext(
		ctx,
		createZoneQuery,
		name,
		forwardZoneID,
		ttl,
	)
	return scanZone(row)
}

func UpdateZone(ctx *gofr.Context, id, name string, forwardZoneID *string, ttl *uint32) (models.ZoneData, error) {
	row := ctx.SQL.QueryRowContext(
		ctx,
		updateZoneQuery,
		name,
		forwardZoneID,
		ttl,
		id,
	)

	return scanZone(row)
}

func DeleteZone(ctx *gofr.Context, id string) error {
	result, err := ctx.SQL.ExecContext(ctx, deleteZoneQuery, id)
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
