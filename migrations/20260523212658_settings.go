package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func settings() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			const createSettingsTable = `
CREATE TABLE IF NOT EXISTS settings (
	id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
	default_forward_zone_id UUID REFERENCES forward_zones(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO settings (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;`

			const addZoneForwardPolicy = `
ALTER TABLE zones
ADD COLUMN IF NOT EXISTS forward_policy TEXT NOT NULL DEFAULT 'default';

UPDATE zones
SET forward_policy = CASE
	WHEN forward_zone_id IS NULL THEN 'default'
	ELSE 'custom'
END;`

			const createSettingsTrigger = `
DROP TRIGGER IF EXISTS trg_settings_set_updated_at ON settings;
CREATE TRIGGER trg_settings_set_updated_at
BEFORE UPDATE ON settings
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();`

			if _, err := d.SQL.Exec(createSettingsTable); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(addZoneForwardPolicy); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createSettingsTrigger); err != nil {
				return err
			}

			return nil
		},
	}
}
