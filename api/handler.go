package api

import (
	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/models"
	"github.com/fmotalleb/hermes/queries"
	"github.com/fmotalleb/hermes/request"
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
	p := request.PaginatorOf(ctx)
	var err error
	var zones []models.ZoneData
	if zones, err = queries.GetZones(ctx, p.Limit, p.Offset); err != nil {
		return nil, err
	}
	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   zones,
	}, nil
}
