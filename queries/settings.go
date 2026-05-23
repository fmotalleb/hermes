package queries

import (
	"database/sql"

	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/models"
)

const getSettingsQuery = `
SELECT
  COALESCE(default_forward_zone_id::text, '') AS default_forward_zone_id,
  created_at,
  updated_at
FROM settings
WHERE id = 1;`

const upsertSettingsQuery = `
INSERT INTO settings (id, default_forward_zone_id)
VALUES (1, NULLIF($1::text, '')::uuid)
ON CONFLICT (id) DO UPDATE
SET default_forward_zone_id = EXCLUDED.default_forward_zone_id
RETURNING
  COALESCE(default_forward_zone_id::text, '') AS default_forward_zone_id,
  created_at,
  updated_at;`

func scanSettings(row scanner) (models.Settings, error) {
	var settings models.Settings
	if err := row.Scan(
		&settings.DefaultForwardZoneID,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	); err != nil {
		return models.Settings{}, err
	}

	return settings, nil
}

func GetSettings(ctx *gofr.Context) (models.Settings, error) {
	row := ctx.SQL.QueryRowContext(ctx, getSettingsQuery)
	return scanSettings(row)
}

func UpdateSettings(ctx *gofr.Context, defaultForwardZoneID *string) (models.Settings, error) {
	row := ctx.SQL.QueryRowContext(ctx, upsertSettingsQuery, defaultForwardZoneID)
	return scanSettings(row)
}

func ensureSettingsRow(ctx *gofr.Context) error {
	_, err := ctx.SQL.ExecContext(ctx, `INSERT INTO settings (id) VALUES (1) ON CONFLICT (id) DO NOTHING;`)
	return err
}

func isSettingsMissing(err error) bool {
	return err == sql.ErrNoRows
}
