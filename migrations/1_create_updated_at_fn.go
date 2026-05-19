package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

const createSetUpdatedAtFunction = `
CREATE OR REPLACE FUNCTION set_updated_at()
  RETURNS trigger AS $BODY$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$BODY$
  LANGUAGE plpgsql VOLATILE
  COST 100`

func createUpdatedAtFn() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			if _, err := d.SQL.Exec(createSetUpdatedAtFunction); err != nil {
				return err
			}
			return nil
		},
	}
}
