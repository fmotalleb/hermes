package queries

import (
	"context"
	"database/sql"

	"github.com/fmotalleb/hermes/models"
)

const getRecordsQuery = `
SELECT
  id,
  zone_id,
  name,
  type,
  value,
  ttl,
  priority,
  created_at,
  updated_at
FROM records
WHERE zone_id = $1
ORDER BY name, type, value
LIMIT $2 OFFSET $3;`

const getRecordQuery = `
SELECT
  id,
  zone_id,
  name,
  type,
  value,
  ttl,
  priority,
  created_at,
  updated_at
FROM records
WHERE zone_id = $1 AND id = $2;`

const createRecordQuery = `
INSERT INTO records (zone_id, name, type, value, ttl, priority)
VALUES ($1, $2, $3, $4, COALESCE($5, 300), COALESCE($6, 0))
RETURNING
  id,
  zone_id,
  name,
  type,
  value,
  ttl,
  priority,
  created_at,
  updated_at;`

const updateRecordQuery = `
UPDATE records
SET
  name = $1,
  type = $2,
  value = $3,
  ttl = COALESCE($4, ttl),
  priority = COALESCE($5, priority)
WHERE zone_id = $6 AND id = $7
RETURNING
  id,
  zone_id,
  name,
  type,
  value,
  ttl,
  priority,
  created_at,
  updated_at;`

const deleteRecordQuery = `DELETE FROM records WHERE zone_id = $1 AND id = $2;`

func scanRecord(row scanner) (models.DNSRecord, error) {
	var record models.DNSRecord
	if err := row.Scan(
		&record.ID,
		&record.ZoneID,
		&record.Name,
		&record.Type,
		&record.Value,
		&record.TTL,
		&record.Priority,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return models.DNSRecord{}, err
	}

	return record, nil
}

func GetRecords(ctx context.Context, db DB, zoneID string, limit, offset uint32) ([]models.DNSRecord, error) {
	rows, err := db.QueryContext(ctx, getRecordsQuery, zoneID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]models.DNSRecord, 0)
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

func GetRecord(ctx context.Context, db DB, zoneID, id string) (models.DNSRecord, error) {
	row := db.QueryRowContext(ctx, getRecordQuery, zoneID, id)
	return scanRecord(row)
}

func CreateRecord(ctx context.Context, db DB, zoneID, name string, typ models.DNSRecordType, value string, ttl, priority *uint32) (models.DNSRecord, error) {
	row := db.QueryRowContext(
		ctx,
		createRecordQuery,
		zoneID,
		name,
		typ,
		value,
		ttl,
		priority,
	)

	return scanRecord(row)
}

func UpdateRecord(ctx context.Context, db DB, zoneID, id, name string, typ models.DNSRecordType, value string, ttl, priority *uint32) (models.DNSRecord, error) {
	row := db.QueryRowContext(
		ctx,
		updateRecordQuery,
		name,
		typ,
		value,
		ttl,
		priority,
		zoneID,
		id,
	)

	return scanRecord(row)
}

func DeleteRecord(ctx context.Context, db DB, zoneID, id string) error {
	result, err := db.ExecContext(ctx, deleteRecordQuery, zoneID, id)
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
