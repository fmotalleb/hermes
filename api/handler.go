package api

import (
	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/models"
)

type handler struct {
	repo *repository
}

func newHandler(r *repository) *handler {
	return &handler{
		repo: r,
	}
}

func (h *handler) getZones(ctx *gofr.Context) (any, error) {
	p := new(models.Paginator)

	if err := ctx.Bind(p); err != nil {
		return nil, err
	}
	if err := p.Error(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, forward_zone_id, ttl, created_at, updated_at
		FROM zones
		ORDER BY name
		LIMIT $1 OFFSET $2
	`

	rows, err := ctx.SQL.QueryContext(ctx, query, p.Limit, p.Offset)
	if err != nil {
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
		)
		if err != nil {
			return nil, err
		}

		zones = append(zones, z)
	}

	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   zones,
	}, nil
}
