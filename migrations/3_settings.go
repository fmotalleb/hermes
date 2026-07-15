package migrations

import (
	"context"
)

func settings() Migration {
	return Migration{
		Version: 3,
		Name:    "settings",
		Up: func(ctx context.Context, db DBTX) error {
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
			const createForwardPolicyEnum = `
CREATE TYPE forward_policy AS ENUM (
	'none',
	'default',
	'custom'
);`
			const addZoneForwardPolicy = "\nALTER TABLE zones\nADD COLUMN IF NOT EXISTS forward_policy NOT NULL DEFAULT 'default';\n\nUPDATE zones\nSET forward_policy = CASE\n    WHEN forward_zone_id IS NULL THEN 'default'::forward_policy\n    ELSE 'custom'::forward_policy\nEND;"

			const createSettingsTrigger = `
DROP TRIGGER IF EXISTS trg_settings_set_updated_at ON settings;
CREATE TRIGGER trg_settings_set_updated_at
BEFORE UPDATE ON settings
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();`

			for _, stmt := range []string{createSettingsTable, createForwardPolicyEnum, addZoneForwardPolicy, createSettingsTrigger} {
				if _, err := db.ExecContext(ctx, stmt); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
