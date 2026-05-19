package queries

import (
	"database/sql"

	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/models"
)

const getZonesQuery = `
SELECT
  z.id,
  z.name,
  z.forward_zone_id,
  z.ttl,
  z.created_at,
  z.updated_at,
  (
    SELECT COUNT(*)
    FROM records r
    WHERE r.zone_id = z.id
  ) AS record_count
FROM zones z
ORDER BY z.name
LIMIT $1 OFFSET $2;`

func GetZones(ctx *gofr.Context, limit, offset uint32) ([]models.ZoneData, error) {
	var rows *sql.Rows
	var err error
	if rows, err = ctx.SQL.QueryContext(ctx, getZonesQuery, limit, offset); err != nil {
		return nil, err
	}
	defer rows.Close()

	zones := make([]models.ZoneData, 0)

	for rows.Next() {
		var z models.ZoneData

		err := rows.Scan(
			&z.ID,
			&z.Name,
			&z.ForwardZoneID,
			&z.TTL,
			&z.CreatedAt,
			&z.UpdatedAt,
			&z.RecordCount,
		)
		if err != nil {
			return nil, err
		}

		zones = append(zones, z)
	}
	return zones, nil
}
