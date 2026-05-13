package migrations

import "gofr.dev/pkg/gofr/migration"

func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		20260512133000: createDNSAdminSchema(),
		20260512135500: addUpdatedAtAndTriggers(),
		20260512143000: addZoneConfigAndInboundSettings(),
	}
}
