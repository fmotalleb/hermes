package migrations

import "gofr.dev/pkg/gofr/migration"

const createForwardZonesTable = `
CREATE TABLE IF NOT EXISTS forward_zones (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name TEXT NOT NULL UNIQUE,
	addresses TEXT[] NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

const addForwardPolicyColumns = `
ALTER TABLE zones ADD COLUMN IF NOT EXISTS forward_mode TEXT NOT NULL DEFAULT 'default';
ALTER TABLE zones ADD COLUMN IF NOT EXISTS forward_zone_id UUID REFERENCES forward_zones(id) ON DELETE SET NULL;
ALTER TABLE inbound_settings ADD COLUMN IF NOT EXISTS fallback_forward_zone_id UUID REFERENCES forward_zones(id) ON DELETE SET NULL;`

const migrateForwardZoneAddresses = `
ALTER TABLE forward_zones ADD COLUMN IF NOT EXISTS addresses TEXT[] NOT NULL DEFAULT '{}';
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'forward_zones'
      AND column_name = 'address'
  ) THEN
    EXECUTE '
      UPDATE forward_zones
      SET addresses = ARRAY[address]
      WHERE addresses IS NULL OR array_length(addresses, 1) IS NULL
    ';
  END IF;
END$$;`

const createInboundEntrypointsTable = `
CREATE TABLE IF NOT EXISTS inbound_entrypoints (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	type TEXT NOT NULL,
	listen_address TEXT NOT NULL,
	port INTEGER NOT NULL,
	public_key TEXT NOT NULL DEFAULT '',
	private_key TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

const addInboundEntrypointsCheck = `
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'inbound_entrypoints_type_check'
  ) THEN
    ALTER TABLE inbound_entrypoints ADD CONSTRAINT inbound_entrypoints_type_check CHECK (type IN ('UDP', 'TCP', 'TLS', 'HTTPS'));
  END IF;
END$$;`

const addInboundEntrypointsUpdatedAtTrigger = `
DROP TRIGGER IF EXISTS trg_inbound_entrypoints_set_updated_at ON inbound_entrypoints;
CREATE TRIGGER trg_inbound_entrypoints_set_updated_at
BEFORE UPDATE ON inbound_entrypoints
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();`

const addForwardModeCheck = `
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'zones_forward_mode_check'
  ) THEN
    ALTER TABLE zones ADD CONSTRAINT zones_forward_mode_check CHECK (forward_mode IN ('default', 'none', 'custom'));
  END IF;
END$$;`

const addForwardZonesUpdatedAtTrigger = `
DROP TRIGGER IF EXISTS trg_forward_zones_set_updated_at ON forward_zones;
CREATE TRIGGER trg_forward_zones_set_updated_at
BEFORE UPDATE ON forward_zones
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();`

func addForwardZonesAndPolicies() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			if _, err := d.SQL.Exec(createForwardZonesTable); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(addForwardPolicyColumns); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(migrateForwardZoneAddresses); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createInboundEntrypointsTable); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(addInboundEntrypointsCheck); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(addForwardModeCheck); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(addForwardZonesUpdatedAtTrigger); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(addInboundEntrypointsUpdatedAtTrigger); err != nil {
				return err
			}
			return nil
		},
	}
}
