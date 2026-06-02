package api

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/fmotalleb/hermes/internal/pubsub"
	"github.com/fmotalleb/hermes/internal/web"
	"github.com/fmotalleb/hermes/request"
)

type handler struct {
	repo           *repository
	migrator       Migrator
	metricsHandler http.Handler
	pubsub         pubsub.Bus
}

func newHandler(r *repository, migrator Migrator, metrics http.Handler, bus pubsub.Bus) *handler {
	return &handler{
		repo:           r,
		migrator:       migrator,
		metricsHandler: metrics,
		pubsub:         bus,
	}
}

func notFoundEntity(ctx *web.Context, logMessage, entityName, value string, err error) (any, error) {
	_ = logMessage
	return nil, entityNotFoundError{Name: entityName, Value: value}
}

func (h *handler) getZones(ctx *web.Context) (any, error) {
	p := request.PaginatorOf(ctx)
	zones, err := h.repo.getZones(ctx, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   zones,
	}, nil
}

func (h *handler) getZone(ctx *web.Context) (any, error) {
	zone := ctx.PathParam("zone")
	r, err := h.repo.getZone(ctx, zone)
	if err != nil {
		return notFoundEntity(ctx, "failed to get zone", "zone_id", zone, err)
	}
	return r, nil
}

func (h *handler) createZone(ctx *web.Context) (any, error) {
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

func (h *handler) updateZone(ctx *web.Context) (any, error) {
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
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundEntity(ctx, "failed to update zone", "zone_id", ctx.PathParam("zone"), err)
		}
		return nil, normalizeCreateError(err)
	}

	return zone, nil
}

func (h *handler) deleteZone(ctx *web.Context) (any, error) {
	zoneID := ctx.PathParam("zone")
	value, err := h.repo.deleteZone(ctx, zoneID)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundEntity(ctx, "failed to delete zone", "zone_id", zoneID, err)
	}
	return value, err
}

func (h *handler) getForwardZones(ctx *web.Context) (any, error) {
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

func (h *handler) getForwardZone(ctx *web.Context) (any, error) {
	id := ctx.PathParam("id")
	r, err := h.repo.getForwardZone(ctx, id)
	if err != nil {
		return notFoundEntity(ctx, "failed to get forward zone", "forward_zone_id", id, err)
	}
	return r, nil
}

func (h *handler) createForwardZone(ctx *web.Context) (any, error) {
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

func (h *handler) updateForwardZone(ctx *web.Context) (any, error) {
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
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundEntity(ctx, "failed to update forward zone", "forward_zone_id", ctx.PathParam("id"), err)
		}
		return nil, normalizeCreateError(err)
	}

	return zone, nil
}

func (h *handler) deleteForwardZone(ctx *web.Context) (any, error) {
	id := ctx.PathParam("id")
	value, err := h.repo.deleteForwardZone(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundEntity(ctx, "failed to delete forward zone", "forward_zone_id", id, err)
	}
	return value, err
}

func (h *handler) getZoneRecords(ctx *web.Context) (any, error) {
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

func (h *handler) getRecord(ctx *web.Context) (any, error) {
	zoneID := ctx.PathParam("zone")
	id := ctx.PathParam("id")
	r, err := h.repo.getRecord(ctx, zoneID, id)
	if err != nil {
		return notFoundEntity(ctx, "failed to get record", "record_id", id, err)
	}
	return r, nil
}

func (h *handler) createRecord(ctx *web.Context) (any, error) {
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

func (h *handler) updateRecord(ctx *web.Context) (any, error) {
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
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundEntity(ctx, "failed to update record", "record_id", ctx.PathParam("id"), err)
		}
		return nil, normalizeCreateError(err)
	}

	return record, nil
}

func (h *handler) deleteRecord(ctx *web.Context) (any, error) {
	id := ctx.PathParam("id")
	value, err := h.repo.deleteRecord(ctx, ctx.PathParam("zone"), id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundEntity(ctx, "failed to delete record", "record_id", id, err)
	}
	return value, err
}

func (h *handler) getSettings(ctx *web.Context) (any, error) {
	return h.repo.getSettings(ctx)
}

func (h *handler) updateSettings(ctx *web.Context) (any, error) {
	var req settingsRequest
	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}

	settings, err := h.repo.updateSettings(ctx, req)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

func (h *handler) publishEvent(ctx *web.Context) (any, error) {
	if h.pubsub == nil {
		return nil, errors.New("pubsub not configured")
	}

	topic := ctx.PathParam("topic")
	payload, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return nil, err
	}
	if err := h.pubsub.Publish(ctx, topic, payload); err != nil {
		return nil, err
	}

	return map[string]any{
		"topic":     topic,
		"published": true,
	}, nil
}

func (h *handler) runMigrations(ctx *web.Context) (any, error) {
	if h.migrator == nil {
		return nil, errors.New("migrator not configured")
	}

	if err := h.migrator.Run(ctx); err != nil {
		return nil, err
	}

	return map[string]any{"ok": true}, nil
}

func (h *handler) metrics(ctx *web.Context) (any, error) {
	if h.metricsHandler == nil {
		return nil, errors.New("metrics handler not configured")
	}

	h.metricsHandler.ServeHTTP(ctx.ResponseWriter, ctx.Request)
	return web.Responded{}, nil
}

func (h *handler) getHijacks(ctx *web.Context) (any, error) {
	p := request.PaginatorOf(ctx)

	hijacks, err := h.repo.getHijacks(ctx, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   hijacks,
	}, nil
}

func (h *handler) getHijack(ctx *web.Context) (any, error) {
	id := ctx.PathParam("id")

	r, err := h.repo.getHijack(ctx, id)
	if err != nil {
		return notFoundEntity(ctx, "failed to get hijack", "hijack_id", id, err)
	}

	return r, nil
}

func (h *handler) createHijack(ctx *web.Context) (any, error) {
	var req hijackRequest

	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}

	req.Name = normalizeName(req.Name)
	req.Value = normalizeName(req.Value)

	if req.Name == "" {
		return nil, errors.New("hijack name is required")
	}
	if req.Value == "" {
		return nil, errors.New("hijack value is required")
	}
	if !validRecordType(req.Type) {
		return nil, fmt.Errorf("%w: %s", errInvalidRecordType, req.Type)
	}
	if !validHijackPolicy(req.Policy) {
		return nil, fmt.Errorf("invalid hijack policy: %s", req.Policy)
	}

	record, err := h.repo.createHijack(ctx, req)
	if err != nil {
		return nil, normalizeCreateError(err)
	}

	return record, nil
}

func (h *handler) updateHijack(ctx *web.Context) (any, error) {
	id := ctx.PathParam("id")

	var req hijackRequest

	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}

	req.Name = normalizeName(req.Name)
	req.Value = normalizeName(req.Value)

	if req.Name == "" {
		return nil, errors.New("hijack name is required")
	}
	if req.Value == "" {
		return nil, errors.New("hijack value is required")
	}
	if !validRecordType(req.Type) {
		return nil, fmt.Errorf("%w: %s", errInvalidRecordType, req.Type)
	}
	if !validHijackPolicy(req.Policy) {
		return nil, fmt.Errorf("invalid hijack policy: %s", req.Policy)
	}

	record, err := h.repo.updateHijack(ctx, id, req)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundEntity(ctx, "failed to update hijack", "hijack_id", id, err)
		}
		return nil, normalizeCreateError(err)
	}

	return record, nil
}

func (h *handler) deleteHijack(ctx *web.Context) (any, error) {
	id := ctx.PathParam("id")

	value, err := h.repo.deleteHijack(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundEntity(ctx, "failed to delete hijack", "hijack_id", id, err)
	}

	return value, err
}
