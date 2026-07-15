package queries

import (
	"context"
	"database/sql"

	"github.com/fmotalleb/hermes/models"
)

const getHijacksQuery = `
SELECT
  h.id,
  h.name,
  h.value,
  h.record_type,
  h.policy,
  h.forward_policy,
  COALESCE(h.forward_zone_id::text, '') AS forward_zone_id,
  h.ttl,
  h.created_at,
  h.updated_at
FROM hijacks h
ORDER BY h.name
LIMIT $1 OFFSET $2;`

const getHijackQuery = `
SELECT
  h.id,
  h.name,
  h.value,
  h.record_type,
  h.policy,
  h.forward_policy,
  COALESCE(h.forward_zone_id::text, '') AS forward_zone_id,
  h.ttl,
  h.created_at,
  h.updated_at
FROM hijacks h
WHERE h.id = $1;`

const createHijackQuery = `
INSERT INTO hijacks (
  name,
  value,
  record_type,
  policy,
  forward_policy,
  forward_zone_id,
  ttl
)
VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  CASE
    WHEN $4 = 'forward'::hijack_policy
    THEN NULLIF($6::text, '')::uuid
    ELSE NULL
  END,
  COALESCE($7, 300)
)
RETURNING
  id,
  name,
  value,
  record_type,
  policy,
  forward_policy,
  COALESCE(forward_zone_id::text, '') AS forward_zone_id,
  ttl,
  created_at,
  updated_at;`

const updateHijackQuery = `
UPDATE hijacks
SET
  name = $1,
  value = $2,
  record_type = $3,
  policy = $4,
  forward_policy = $5,
  forward_zone_id = CASE
    WHEN $4 = 'forward'::hijack_policy
    THEN NULLIF($6::text, '')::uuid
    ELSE NULL
  END,
  ttl = COALESCE($7, ttl),
  updated_at = NOW()
WHERE id = $8
RETURNING
  id,
  name,
  value,
  record_type,
  policy,
  forward_policy,
  COALESCE(forward_zone_id::text, '') AS forward_zone_id,
  ttl,
  created_at,
  updated_at;`

const deleteHijackQuery = `DELETE FROM hijacks WHERE id = $1;`

const searchHijackQuery = `
SELECT
  id,
  name,
  value,
  record_type,
  policy,
  forward_policy,
  COALESCE(forward_zone_id::text, '') AS forward_zone_id,
  ttl,
  created_at,
  updated_at
FROM hijacks
WHERE name ILIKE '%' || $1 || '%'
ORDER BY char_length(name) DESC
LIMIT 50;
`

const hijackLookupQuery = `
SELECT
  id,
  name,
  value,
  record_type,
  policy,
  forward_policy,
  COALESCE(forward_zone_id::text, '') AS forward_zone_id,
  ttl,
  created_at,
  updated_at
FROM hijacks
WHERE
  (name = $1 AND record_type = $2)
  OR (name ILIKE '%' || '.' || $1 AND record_type = $2)
ORDER BY
  (name = $1) DESC,
  char_length(name) DESC
LIMIT 1;
`

func scanHijack(row scanner) (models.HijackRecord, error) {
	var h models.HijackRecord

	if err := row.Scan(
		&h.ID,
		&h.Name,
		&h.Value,
		&h.Type,
		&h.HijackPolicy,
		&h.ForwardPolicy,
		&h.ForwardZoneID,
		&h.TTL,
		&h.CreatedAt,
		&h.UpdatedAt,
	); err != nil {
		return models.HijackRecord{}, err
	}

	return h, nil
}

// GetHijacks returns a paginated list of hijack rules ordered by name.
func GetHijacks(ctx context.Context, db DB, limit, offset uint32) ([]models.HijackRecord, error) {
	rows, err := db.QueryContext(ctx, getHijacksQuery, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.HijackRecord, 0)

	for rows.Next() {
		var h models.HijackRecord
		if err := rows.Scan(
			&h.ID,
			&h.Name,
			&h.Value,
			&h.Type,
			&h.HijackPolicy,
			&h.ForwardPolicy,
			&h.ForwardZoneID,
			&h.TTL,
			&h.CreatedAt,
			&h.UpdatedAt,
		); err != nil {
			return nil, err
		}

		out = append(out, h)
	}

	return out, rows.Err()
}

// GetHijack returns a single hijack rule by ID.
func GetHijack(ctx context.Context, db DB, id string) (models.HijackRecord, error) {
	row := db.QueryRowContext(ctx, getHijackQuery, id)
	return scanHijack(row)
}

// CreateHijack inserts a new hijack rule.
func CreateHijack(
	ctx context.Context,
	db DB,
	name string,
	value string,
	recordType models.DNSRecordType,
	policy models.HijackPolicy,
	forwardPolicy models.ForwardPolicy,
	forwardZoneID *string,
	ttl *uint32,
) (models.HijackRecord, error) {
	row := db.QueryRowContext(
		ctx,
		createHijackQuery,
		name,
		value,
		recordType,
		policy,
		forwardPolicy,
		forwardZoneID,
		ttl,
	)

	return scanHijack(row)
}

// UpdateHijack modifies an existing hijack rule.
func UpdateHijack(
	ctx context.Context,
	db DB,
	id string,
	name string,
	value string,
	recordType models.DNSRecordType,
	policy models.HijackPolicy,
	forwardPolicy models.ForwardPolicy,
	forwardZoneID *string,
	ttl *uint32,
) (models.HijackRecord, error) {
	row := db.QueryRowContext(
		ctx,
		updateHijackQuery,
		name,
		value,
		recordType,
		policy,
		forwardPolicy,
		forwardZoneID,
		ttl,
		id,
	)

	return scanHijack(row)
}

// DeleteHijack removes a hijack rule by ID. Returns sql.ErrNoRows if not found.
func DeleteHijack(ctx context.Context, db DB, id string) error {
	res, err := db.ExecContext(ctx, deleteHijackQuery, id)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// SearchHijack finds hijack rules whose name contains the given search string (case-insensitive).
func SearchHijack(ctx context.Context, db DB, name string) ([]models.HijackRecord, error) {
	rows, err := db.QueryContext(ctx, searchHijackQuery, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.HijackRecord, 0)

	for rows.Next() {
		var h models.HijackRecord

		if err := rows.Scan(
			&h.ID,
			&h.Name,
			&h.Value,
			&h.Type,
			&h.HijackPolicy,
			&h.ForwardPolicy,
			&h.ForwardZoneID,
			&h.TTL,
			&h.CreatedAt,
			&h.UpdatedAt,
		); err != nil {
			return nil, err
		}

		out = append(out, h)
	}

	return out, rows.Err()
}

// HijackLookup finds the most specific hijack rule matching the given name and record type.
func HijackLookup(ctx context.Context, db DB, name string, recordType models.DNSRecordType) (models.HijackRecord, error) {
	row := db.QueryRowContext(ctx, hijackLookupQuery, name, recordType)
	return scanHijack(row)
}
