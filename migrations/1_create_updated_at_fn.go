package migrations

import (
	"context"
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

func createUpdatedAtFn() Migration {
	return Migration{
		Version: 1,
		Name:    "create_updated_at_fn",
		Up: func(ctx context.Context, db DBTX) error {
			_, err := db.ExecContext(ctx, createSetUpdatedAtFunction)
			return err
		},
	}
}
