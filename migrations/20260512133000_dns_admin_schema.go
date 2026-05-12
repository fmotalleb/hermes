package migrations

import "gofr.dev/pkg/gofr/migration"

const enableUUIDExtension = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;`

const createZonesTable = `
CREATE TABLE IF NOT EXISTS zones (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name TEXT NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

const createRecordsTable = `
CREATE TABLE IF NOT EXISTS records (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	zone_id UUID NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	value TEXT NOT NULL,
	ttl INTEGER NOT NULL DEFAULT 300,
	priority INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

const createRecordsIndexes = `
CREATE INDEX IF NOT EXISTS idx_records_zone_id ON records(zone_id);
CREATE INDEX IF NOT EXISTS idx_records_type ON records(type);`

func createDNSAdminSchema() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			if _, err := d.SQL.Exec(enableUUIDExtension); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createZonesTable); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createRecordsTable); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createRecordsIndexes); err != nil {
				return err
			}

			return nil
		},
	}
}
