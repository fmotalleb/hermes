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
	'forward',
	'raw'
);`

			const createHijacksTable = "\nCREATE TABLE IF NOT EXISTS hijacks (\n\tid UUID PRIMARY KEY DEFAULT uuidv7(),\n\n\tname TEXT NOT NULL,\n\tvalue TEXT NOT NULL,\n\n\trecord_type dns_record_type NOT NULL,\n\n\tpolicy hijack_policy NOT NULL,\n\n\tforward_policy NOT NULL\n\t\tDEFAULT 'default'::forward_policy,\n\n\tforward_zone_id UUID\n\t\tREFERENCES forward_zones(id)\n\t\tON DELETE SET NULL,\n\n\tttl uint32 NOT NULL DEFAULT 300,\n\n\tcreated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),\n\tupdated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),\n\n\tCONSTRAINT chk_hijack_proxy_requires_zone\n\tCHECK (\n\t\tpolicy <> 'forward'\n\t\tOR forward_zone_id IS NOT NULL\n\t),\n\n\tCONSTRAINT uq_hijacks_name_record_type\n\tUNIQUE (name, record_type)"

			const createHijackIndexes = `
CREATE INDEX IF NOT EXISTS idx_hijacks_zone_id
	ON hijacks(forward_zone_id);

CREATE INDEX IF NOT EXISTS idx_hijacks_name_type
	ON hijacks(name, record_type);

CREATE INDEX IF NOT EXISTS idx_hijacks_name_trgm
	ON hijacks USING GIN (name gin_trgm_ops);
`

			const createHijackTrigger = `
DROP TRIGGER IF EXISTS trg_hijacks_set_updated_at ON hijacks;
CREATE TRIGGER trg_hijacks_set_updated_at
BEFORE UPDATE ON hijacks
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();`

			for _, stmt := range []string{createHijackPolicyType, createHijacksTable, createHijackIndexes, createHijackTrigger} {
				if _, err := db.ExecContext(ctx, stmt); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
