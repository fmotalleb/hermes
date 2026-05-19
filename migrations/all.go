// This is auto-generated file using 'gofr migrate' tool. DO NOT EDIT.
package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		1: createUpdatedAtFn(),
		2: createDNSApiSchema(),
	}
}
