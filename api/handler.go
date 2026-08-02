package api

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/fmotalleb/go-tools/log"
	"go.uber.org/zap"

	"github.com/fmotalleb/hermes/pubsub"
	"github.com/fmotalleb/hermes/request"
	"github.com/fmotalleb/hermes/web"
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

// notFoundEntity logs the underlying error once (at warn level) and returns a
// 404 response, using the handler's request-scoped logger.
func notFoundEntity(logger *zap.Logger, logMessage, entityName, value string, err error) (any, error) {
	logger.Warn(logMessage, zap.String(entityName, value), zap.Error(err))
	return nil, entityNotFoundError{Name: entityName, Value: value}
}

// bindRequest decodes the request body into v, logging a debug line on failure
// so malformed client payloads are visible without being treated as server
// errors.
func bindRequest(ctx *web.Context, logger *zap.Logger, v any) error {
	if err := ctx.Bind(v); err != nil {
		logger.Debug("failed to bind request body", zap.Error(err))
		return err
	}
	return nil
}

// reject logs a client-side validation failure at debug level and returns the
// error unchanged so the handler's response behavior is preserved.
func reject(logger *zap.Logger, err error) error {
	logger.Debug("invalid request", zap.Error(err))
	return err
}

func (h *handler) getZones(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	p := request.PaginatorOf(ctx)
	zones, err := h.repo.getZones(apiCtx, p.Limit, p.Offset)
	if err != nil {
		logger.Error("failed to get zones", zap.Error(err))
		return nil, err
	}
	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   zones,
	}, nil
}

func (h *handler) getZone(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	zone := ctx.PathParam("zone")
	r, err := h.repo.getZone(apiCtx, zone)
	if err != nil {
		return notFoundEntity(logger, "failed to get zone", "zone_id", zone, err)
	}
	return r, nil
}

func (h *handler) createZone(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	var req zoneRequest
	if err := bindRequest(ctx, logger, &req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	if req.Name == "" {
		return nil, reject(logger, errors.New("zone name is required"))
	}

	zone, err := h.repo.createZone(apiCtx, req)
	if err != nil {
		logger.Error("failed to create zone", zap.Error(err))
		return nil, normalizeCreateError(err)
	}
	logger.Debug("zone created", zap.String("zone_id", zone.ID))
	return zone, nil
}

func (h *handler) updateZone(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	var req zoneRequest
	if err := bindRequest(ctx, logger, &req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	if req.Name == "" {
		return nil, reject(logger, errors.New("zone name is required"))
	}

	zone, err := h.repo.updateZone(apiCtx, ctx.PathParam("zone"), req)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundEntity(logger, "failed to update zone", "zone_id", ctx.PathParam("zone"), err)
		}
		logger.Error("failed to update zone", zap.String("zone_id", ctx.PathParam("zone")), zap.Error(err))
		return nil, normalizeCreateError(err)
	}
	logger.Debug("zone updated", zap.String("zone_id", zone.ID))
	return zone, nil
}

func (h *handler) deleteZone(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	zoneID := ctx.PathParam("zone")
	value, err := h.repo.deleteZone(apiCtx, zoneID)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundEntity(logger, "failed to delete zone", "zone_id", zoneID, err)
	}
	if err != nil {
		logger.Error("failed to delete zone", zap.String("zone_id", zoneID), zap.Error(err))
		return nil, err
	}
	logger.Debug("zone deleted", zap.String("zone_id", zoneID))
	return value, nil
}

func (h *handler) getForwardZones(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	p := request.PaginatorOf(ctx)

	zones, err := h.repo.getForwardZones(apiCtx, p.Limit, p.Offset)
	if err != nil {
		logger.Error("failed to get forward zones", zap.Error(err))
		return nil, err
	}

	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   zones,
	}, nil
}

func (h *handler) getForwardZone(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	id := ctx.PathParam("id")
	r, err := h.repo.getForwardZone(apiCtx, id)
	if err != nil {
		return notFoundEntity(logger, "failed to get forward zone", "forward_zone_id", id, err)
	}
	return r, nil
}

func (h *handler) createForwardZone(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	var req forwardZoneRequest
	if err := bindRequest(ctx, logger, &req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	if req.Name == "" {
		return nil, reject(logger, errors.New("forward zone name is required"))
	}
	if len(req.Addresses) == 0 {
		return nil, reject(logger, errors.New("forward zone addresses are required"))
	}

	zone, err := h.repo.createForwardZone(apiCtx, req)
	if err != nil {
		logger.Error("failed to create forward zone", zap.Error(err))
		return nil, normalizeCreateError(err)
	}
	logger.Debug("forward zone created", zap.String("forward_zone_id", zone.ID))
	return zone, nil
}

func (h *handler) updateForwardZone(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	var req forwardZoneRequest
	if err := bindRequest(ctx, logger, &req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	if req.Name == "" {
		return nil, reject(logger, errors.New("forward zone name is required"))
	}
	if len(req.Addresses) == 0 {
		return nil, reject(logger, errors.New("forward zone addresses are required"))
	}

	zone, err := h.repo.updateForwardZone(apiCtx, ctx.PathParam("id"), req)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundEntity(logger, "failed to update forward zone", "forward_zone_id", ctx.PathParam("id"), err)
		}
		logger.Error("failed to update forward zone", zap.String("forward_zone_id", ctx.PathParam("id")), zap.Error(err))
		return nil, normalizeCreateError(err)
	}
	logger.Debug("forward zone updated", zap.String("forward_zone_id", zone.ID))
	return zone, nil
}

func (h *handler) deleteForwardZone(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	id := ctx.PathParam("id")
	value, err := h.repo.deleteForwardZone(apiCtx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundEntity(logger, "failed to delete forward zone", "forward_zone_id", id, err)
	}
	if err != nil {
		logger.Error("failed to delete forward zone", zap.String("forward_zone_id", id), zap.Error(err))
		return nil, err
	}
	logger.Debug("forward zone deleted", zap.String("forward_zone_id", id))
	return value, nil
}

func (h *handler) getZoneRecords(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	p := request.PaginatorOf(ctx)

	records, err := h.repo.getRecords(apiCtx, ctx.PathParam("zone"), p.Limit, p.Offset)
	if err != nil {
		logger.Error("failed to get records", zap.String("zone_id", ctx.PathParam("zone")), zap.Error(err))
		return nil, err
	}

	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   records,
	}, nil
}

func (h *handler) getRecord(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	zoneID := ctx.PathParam("zone")
	id := ctx.PathParam("id")
	r, err := h.repo.getRecord(apiCtx, zoneID, id)
	if err != nil {
		return notFoundEntity(logger, "failed to get record", "record_id", id, err)
	}
	return r, nil
}

func (h *handler) createRecord(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	var req recordRequest
	if err := bindRequest(ctx, logger, &req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	req.Value = normalizeName(req.Value)
	if req.Name == "" {
		return nil, reject(logger, errors.New("record name is required"))
	}
	if req.Value == "" {
		return nil, reject(logger, errors.New("record value is required"))
	}
	if !validRecordType(req.Type) {
		return nil, reject(logger, fmt.Errorf("%w: %s", errInvalidRecordType, req.Type))
	}

	record, err := h.repo.createRecord(apiCtx, ctx.PathParam("zone"), req)
	if err != nil {
		logger.Error("failed to create record", zap.String("zone_id", ctx.PathParam("zone")), zap.Error(err))
		return nil, normalizeCreateError(err)
	}
	logger.Debug("record created", zap.String("zone_id", ctx.PathParam("zone")), zap.String("record_id", record.ID))
	return record, nil
}

func (h *handler) updateRecord(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	var req recordRequest
	if err := bindRequest(ctx, logger, &req); err != nil {
		return nil, err
	}
	req.Name = normalizeName(req.Name)
	req.Value = normalizeName(req.Value)
	if req.Name == "" {
		return nil, reject(logger, errors.New("record name is required"))
	}
	if req.Value == "" {
		return nil, reject(logger, errors.New("record value is required"))
	}
	if !validRecordType(req.Type) {
		return nil, reject(logger, fmt.Errorf("%w: %s", errInvalidRecordType, req.Type))
	}

	record, err := h.repo.updateRecord(apiCtx, ctx.PathParam("zone"), ctx.PathParam("id"), req)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundEntity(logger, "failed to update record", "record_id", ctx.PathParam("id"), err)
		}
		logger.Error("failed to update record", zap.String("record_id", ctx.PathParam("id")), zap.Error(err))
		return nil, normalizeCreateError(err)
	}
	logger.Debug("record updated", zap.String("zone_id", ctx.PathParam("zone")), zap.String("record_id", record.ID))
	return record, nil
}

func (h *handler) deleteRecord(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	id := ctx.PathParam("id")
	value, err := h.repo.deleteRecord(apiCtx, ctx.PathParam("zone"), id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundEntity(logger, "failed to delete record", "record_id", id, err)
	}
	if err != nil {
		logger.Error("failed to delete record", zap.String("record_id", id), zap.Error(err))
		return nil, err
	}
	logger.Debug("record deleted", zap.String("record_id", id))
	return value, nil
}

func (h *handler) getSettings(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	settings, err := h.repo.getSettings(apiCtx)
	if err != nil {
		logger.Error("failed to get settings", zap.Error(err))
		return nil, err
	}
	return settings, nil
}

func (h *handler) updateSettings(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	var req settingsRequest
	if err := bindRequest(ctx, logger, &req); err != nil {
		return nil, err
	}

	settings, err := h.repo.updateSettings(apiCtx, req)
	if err != nil {
		logger.Error("failed to update settings", zap.Error(err))
		return nil, err
	}
	logger.Debug("settings updated")
	return settings, nil
}

func (h *handler) publishEvent(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	if h.pubsub == nil {
		return nil, errors.New("pubsub not configured")
	}

	topic := ctx.PathParam("topic")
	payload, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		logger.Error("failed to read pubsub payload", zap.String("topic", topic), zap.Error(err))
		return nil, err
	}
	if err := h.pubsub.Publish(apiCtx, topic, payload); err != nil {
		logger.Error("failed to publish event", zap.String("topic", topic), zap.Error(err))
		return nil, err
	}
	logger.Debug("event published", zap.String("topic", topic))
	return map[string]any{
		"topic":     topic,
		"published": true,
	}, nil
}

func (h *handler) runMigrations(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	if h.migrator == nil {
		return nil, errors.New("migrator not configured")
	}

	if err := h.migrator.Run(apiCtx); err != nil {
		logger.Error("failed to run migrations", zap.Error(err))
		return nil, err
	}
	logger.Debug("migrations ran successfully")
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
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	p := request.PaginatorOf(ctx)

	hijacks, err := h.repo.getHijacks(apiCtx, p.Limit, p.Offset)
	if err != nil {
		logger.Error("failed to get hijacks", zap.Error(err))
		return nil, err
	}

	return map[string]any{
		"limit":  p.Limit,
		"offset": p.Offset,
		"data":   hijacks,
	}, nil
}

func (h *handler) getHijack(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	id := ctx.PathParam("id")

	r, err := h.repo.getHijack(apiCtx, id)
	if err != nil {
		return notFoundEntity(logger, "failed to get hijack", "hijack_id", id, err)
	}

	return r, nil
}

func (h *handler) createHijack(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	var req hijackRequest

	if err := bindRequest(ctx, logger, &req); err != nil {
		return nil, err
	}

	req.Name = normalizeName(req.Name)
	req.Value = normalizeName(req.Value)

	if err := validateHijackRequest(req); err != nil {
		return nil, reject(logger, err)
	}

	record, err := h.repo.createHijack(apiCtx, req)
	if err != nil {
		logger.Error("failed to create hijack", zap.Error(err))
		return nil, normalizeCreateError(err)
	}
	logger.Debug("hijack created", zap.String("hijack_id", record.ID))
	return record, nil
}

func (h *handler) updateHijack(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	id := ctx.PathParam("id")

	var req hijackRequest

	if err := bindRequest(ctx, logger, &req); err != nil {
		return nil, err
	}

	req.Name = normalizeName(req.Name)
	req.Value = normalizeName(req.Value)

	if err := validateHijackRequest(req); err != nil {
		return nil, reject(logger, err)
	}

	record, err := h.repo.updateHijack(apiCtx, id, req)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundEntity(logger, "failed to update hijack", "hijack_id", id, err)
		}
		logger.Error("failed to update hijack", zap.String("hijack_id", id), zap.Error(err))
		return nil, normalizeCreateError(err)
	}
	logger.Debug("hijack updated", zap.String("hijack_id", record.ID))
	return record, nil
}

func (h *handler) deleteHijack(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	id := ctx.PathParam("id")

	value, err := h.repo.deleteHijack(apiCtx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundEntity(logger, "failed to delete hijack", "hijack_id", id, err)
	}
	if err != nil {
		logger.Error("failed to delete hijack", zap.String("hijack_id", id), zap.Error(err))
		return nil, err
	}
	logger.Debug("hijack deleted", zap.String("hijack_id", id))
	return value, nil
}

func (h *handler) getServices(ctx *web.Context) (any, error) {
	apiCtx, logger := log.AsNamedChild(ctx, "api")
	kind := ctx.PathParam("kind")
	services, err := h.repo.getServices(apiCtx, kind)
	if err != nil {
		logger.Error("failed to get services", zap.String("kind", kind), zap.Error(err))
		return nil, err
	}
	return services, nil
}
