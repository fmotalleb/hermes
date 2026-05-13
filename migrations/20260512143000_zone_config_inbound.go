package migrations

import "gofr.dev/pkg/gofr/migration"

const addZoneConfigColumns = `
ALTER TABLE zones ADD COLUMN IF NOT EXISTS forward_zone TEXT NOT NULL DEFAULT '';
ALTER TABLE zones ADD COLUMN IF NOT EXISTS cache_ttl INTEGER NOT NULL DEFAULT 300;`

const createInboundSettingsTable = `
CREATE TABLE IF NOT EXISTS inbound_settings (
	id INTEGER PRIMARY KEY,
	udp_listen_address TEXT NOT NULL DEFAULT '0.0.0.0',
	udp_port INTEGER NOT NULL DEFAULT 53,
	tcp_listen_address TEXT NOT NULL DEFAULT '0.0.0.0',
	tcp_port INTEGER NOT NULL DEFAULT 53,
	tls_listen_address TEXT NOT NULL DEFAULT '0.0.0.0',
	tls_port INTEGER NOT NULL DEFAULT 853,
	tls_public_key TEXT NOT NULL DEFAULT '',
	tls_private_key TEXT NOT NULL DEFAULT '',
	https_listen_address TEXT NOT NULL DEFAULT '0.0.0.0',
	https_port INTEGER NOT NULL DEFAULT 443,
	https_public_key TEXT NOT NULL DEFAULT '',
	https_private_key TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

const seedInboundSettings = `
INSERT INTO inbound_settings (id) VALUES (1)
ON CONFLICT (id) DO NOTHING;`

const triggerInboundUpdatedAt = `
DROP TRIGGER IF EXISTS trg_inbound_set_updated_at ON inbound_settings;
CREATE TRIGGER trg_inbound_set_updated_at
BEFORE UPDATE ON inbound_settings
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();`

func addZoneConfigAndInboundSettings() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			if _, err := d.SQL.Exec(addZoneConfigColumns); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createInboundSettingsTable); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(seedInboundSettings); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(triggerInboundUpdatedAt); err != nil {
				return err
			}
			return nil
		},
	}
}
