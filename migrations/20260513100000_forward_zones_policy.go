package migrations

import "gofr.dev/pkg/gofr/migration"

const createForwardZonesTable = `
CREATE TABLE IF NOT EXISTS forward_zones (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name TEXT NOT NULL UNIQUE,
	address TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

const addForwardPolicyColumns = `
ALTER TABLE zones ADD COLUMN IF NOT EXISTS forward_mode TEXT NOT NULL DEFAULT 'default';
ALTER TABLE zones ADD COLUMN IF NOT EXISTS forward_zone_id UUID REFERENCES forward_zones(id) ON DELETE SET NULL;
ALTER TABLE inbound_settings ADD COLUMN IF NOT EXISTS fallback_forward_zone_id UUID REFERENCES forward_zones(id) ON DELETE SET NULL;`

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
			if _, err := d.SQL.Exec(addForwardModeCheck); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(addForwardZonesUpdatedAtTrigger); err != nil {
				return err
			}
			return nil
		},
	}
}

