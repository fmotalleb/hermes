package migrations

import "gofr.dev/pkg/gofr/migration"

const enableExtensions = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
`

const createUInt32Domain = `
CREATE DOMAIN uint32 AS bigint
CHECK (VALUE >= 0 AND VALUE <= 4294967295);`

const createForwardZonesTable = `
CREATE TABLE IF NOT EXISTS forward_zones (
	id UUID PRIMARY KEY DEFAULT uuidv7(),
	name TEXT NOT NULL UNIQUE,
	addresses TEXT[] NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

const createZonesTable = `
CREATE TABLE IF NOT EXISTS zones (
	id UUID PRIMARY KEY DEFAULT uuidv7(),
	name TEXT NOT NULL UNIQUE,
	forward_zone_id UUID REFERENCES forward_zones(id) ON DELETE SET NULL,
	ttl uint32 NOT NULL DEFAULT 300,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

const createDNSRecordType = `
CREATE TYPE dns_record_type AS ENUM (
	'A',
	'AAAA',
	'CNAME',
	'MX',
	'NS',
	'PTR',
	'SOA',
	'SRV',
	'TXT',
	'CAA',
	'NAPTR',
	'DNSKEY',
	'DS',
	'RRSIG',
	'NSEC',
	'NSEC3',
	'TLSA'
);`

const createRecordsTable = `
CREATE TABLE IF NOT EXISTS records (
	id UUID PRIMARY KEY DEFAULT uuidv7(),
	zone_id UUID NOT NULL
		REFERENCES zones(id)
		ON DELETE CASCADE,
	name TEXT NOT NULL,
	type dns_record_type NOT NULL,
	value TEXT NOT NULL,
	ttl uint32 NOT NULL DEFAULT 300,
	priority uint32 NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

const createZonesIndexes = `
CREATE INDEX IF NOT EXISTS idx_records_zone_id ON zones(name);
CREATE INDEX IF NOT EXISTS idx_records_zone_id ON zones(forward_zone_id);`

const createRecordsIndexes = `
CREATE INDEX IF NOT EXISTS idx_records_zone_id ON records(zone_id);
CREATE INDEX IF NOT EXISTS idx_records_name_type ON records(name, type);
CREATE INDEX IF NOT EXISTS idx_records_name_trgm ON records USING GIN (name gin_trgm_ops);
`

const createUniqueConstraint = `
CREATE UNIQUE INDEX IF NOT EXISTS uniq_record ON records(zone_id, name, type, value, priority);`

const createUpdatedAtTriggers = `
DROP TRIGGER IF EXISTS trg_forward_zones_set_updated_at ON forward_zones;
CREATE TRIGGER trg_forward_zones_set_updated_at
BEFORE UPDATE ON forward_zones
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_zones_set_updated_at ON zones;
CREATE TRIGGER trg_zones_set_updated_at
BEFORE UPDATE ON zones
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_records_set_updated_at ON records;
CREATE TRIGGER trg_records_set_updated_at
BEFORE UPDATE ON records
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();`

func createDNSApiSchema() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			if _, err := d.SQL.Exec(enableExtensions); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createUInt32Domain); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createDNSRecordType); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createForwardZonesTable); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createZonesTable); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createZonesIndexes); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createRecordsTable); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createRecordsIndexes); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createUniqueConstraint); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createUpdatedAtTriggers); err != nil {
				return err
			}

			return nil
		},
	}
}
