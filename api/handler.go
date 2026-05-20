package api

import (
	"errors"
	"fmt"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http"

	"github.com/fmotalleb/hermes/models"
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

func notFoundEntity(ctx *gofr.Context, logMessage, entityName, value string, err error) (any, error) {
	ctx.Errorf("%s: %v", logMessage, err)
	return nil, http.ErrorEntityNotFound{
		Name:  entityName,
		Value: value,
	}
}

func (h *handler) getZones(ctx *gofr.Context) (any, error) {
	p := request.PaginatorOf(ctx)
	var err error
	var zones []models.ZoneData
	if zones, err = h.repo.getZones(ctx, p.Limit, p.Offset); err != nil {
		return nil, err
	}
	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   zones,
	}, nil
}

func (h *handler) getZone(ctx *gofr.Context) (any, error) {
	zone := ctx.PathParam("zone")
	if r, err := h.repo.getZone(ctx, zone); err != nil {
		return notFoundEntity(ctx, "failed to get zone", "zone_id", zone, err)
	} else {
		return r, nil
	}
}

func (h *handler) createZone(ctx *gofr.Context) (any, error) {
	var req zoneRequest
	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	if req.Name == "" {
		return nil, errors.New("zone name is required")
	}

	zone, err := h.repo.createZone(ctx, req)
	if err != nil {
		return nil, normalizeCreateError(err)
	}

	return zone, nil
}

func (h *handler) updateZone(ctx *gofr.Context) (any, error) {
	var req zoneRequest
	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	if req.Name == "" {
		return nil, errors.New("zone name is required")
	}

	zone, err := h.repo.updateZone(ctx, ctx.PathParam("zone"), req)
	if err != nil {
		return nil, normalizeCreateError(err)
	}

	return zone, nil
}

func (h *handler) deleteZone(ctx *gofr.Context) (any, error) {
	return h.repo.deleteZone(ctx, ctx.PathParam("zone"))
}

func (h *handler) getForwardZones(ctx *gofr.Context) (any, error) {
	p := request.PaginatorOf(ctx)

	zones, err := h.repo.getForwardZones(ctx, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   zones,
	}, nil
}

func (h *handler) getForwardZone(ctx *gofr.Context) (any, error) {
	id := ctx.PathParam("id")
	if r, err := h.repo.getForwardZone(ctx, id); err != nil {
		return notFoundEntity(ctx, "failed to get forward zone", "forward_zone_id", id, err)
	} else {
		return r, nil
	}
}

func (h *handler) createForwardZone(ctx *gofr.Context) (any, error) {
	var req forwardZoneRequest
	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	if req.Name == "" {
		return nil, errors.New("forward zone name is required")
	}
	if len(req.Addresses) == 0 {
		return nil, errors.New("forward zone addresses are required")
	}

	zone, err := h.repo.createForwardZone(ctx, req)
	if err != nil {
		return nil, normalizeCreateError(err)
	}

	return zone, nil
}

func (h *handler) updateForwardZone(ctx *gofr.Context) (any, error) {
	var req forwardZoneRequest
	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	if req.Name == "" {
		return nil, errors.New("forward zone name is required")
	}
	if len(req.Addresses) == 0 {
		return nil, errors.New("forward zone addresses are required")
	}

	zone, err := h.repo.updateForwardZone(ctx, ctx.PathParam("id"), req)
	if err != nil {
		return nil, normalizeCreateError(err)
	}

	return zone, nil
}

func (h *handler) deleteForwardZone(ctx *gofr.Context) (any, error) {
	return h.repo.deleteForwardZone(ctx, ctx.PathParam("id"))
}

func (h *handler) getZoneRecords(ctx *gofr.Context) (any, error) {
	p := request.PaginatorOf(ctx)

	records, err := h.repo.getRecords(ctx, ctx.PathParam("zone"), p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   records,
	}, nil
}

func (h *handler) getRecord(ctx *gofr.Context) (any, error) {
	zoneID := ctx.PathParam("zone")
	id := ctx.PathParam("id")
	if r, err := h.repo.getRecord(ctx, zoneID, id); err != nil {
		return notFoundEntity(ctx, "failed to get record", "record_id", id, err)
	} else {
		return r, nil
	}
}

func (h *handler) createRecord(ctx *gofr.Context) (any, error) {
	var req recordRequest
	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	req.Value = normalizeName(req.Value)
	if req.Name == "" {
		return nil, errors.New("record name is required")
	}
	if req.Value == "" {
		return nil, errors.New("record value is required")
	}
	if !validRecordType(req.Type) {
		return nil, fmt.Errorf("%w: %s", errInvalidRecordType, req.Type)
	}

	record, err := h.repo.createRecord(ctx, ctx.PathParam("zone"), req)
	if err != nil {
		return nil, normalizeCreateError(err)
	}

	return record, nil
}

func (h *handler) updateRecord(ctx *gofr.Context) (any, error) {
	var req recordRequest
	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	req.Value = normalizeName(req.Value)
	if req.Name == "" {
		return nil, errors.New("record name is required")
	}
	if req.Value == "" {
		return nil, errors.New("record value is required")
	}
	if !validRecordType(req.Type) {
		return nil, fmt.Errorf("%w: %s", errInvalidRecordType, req.Type)
	}

	record, err := h.repo.updateRecord(ctx, ctx.PathParam("zone"), ctx.PathParam("id"), req)
	if err != nil {
		return nil, normalizeCreateError(err)
	}

	return record, nil
}

func (h *handler) deleteRecord(ctx *gofr.Context) (any, error) {
	return h.repo.deleteRecord(ctx, ctx.PathParam("zone"), ctx.PathParam("id"))
}
