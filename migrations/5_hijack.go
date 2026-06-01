package migrations

import (
	"context"
)

func hijack() Migration {
	return Migration{
		Version: 5,
		Name:    "hijack",
		Up: func(ctx context.Context, db DBTX) error {
			const createHijackPolicyType = `
CREATE TYPE hijack_policy AS ENUM (
	'block',
	'proxy',
	'raw'
);`

			const createHijacksTable = `
CREATE TABLE IF NOT EXISTS hijacks (
	id UUID PRIMARY KEY DEFAULT uuidv7(),
	name TEXT NOT NULL,
	policy hijack_policy NOT NULL,
	forward_policy TEXT NOT NULL DEFAULT 'default',
	record_type dns_record_type NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

			const createHijackTrigger = `
DROP TRIGGER IF EXISTS trg_hijacks_set_updated_at ON hijacks;
CREATE TRIGGER trg_hijacks_set_updated_at
BEFORE UPDATE ON hijacks
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();`

			for _, stmt := range []string{createHijackPolicyType, createHijacksTable, createHijackTrigger} {
				if _, err := db.ExecContext(ctx, stmt); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
