package migrations

import "gofr.dev/pkg/gofr/migration"

const addUpdatedAtColumns = `
ALTER TABLE zones ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE records ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`

const createSetUpdatedAtFunction = `
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;`

const createUpdatedAtTriggers = `
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

func addUpdatedAtAndTriggers() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			if _, err := d.SQL.Exec(addUpdatedAtColumns); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createSetUpdatedAtFunction); err != nil {
				return err
			}
			if _, err := d.SQL.Exec(createUpdatedAtTriggers); err != nil {
				return err
			}

			return nil
		},
	}
}
