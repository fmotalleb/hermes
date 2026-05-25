package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/dns"
	"github.com/fmotalleb/hermes/models"
	"github.com/fmotalleb/hermes/queries"
)

type repository struct{}

func newRepository() *repository {
	return new(repository)
}

const (
	zonesCacheTTL        = 15 * time.Second
	zonesCacheVersionKey = "zones:list:version"
)

func zonesCacheKey(version uint64, limit, offset uint32) string {
	return fmt.Sprintf(
		"zones:list:%d:%d:%d",
		version,
		limit,
		offset,
	)
}

func (r *repository) zonesCacheVersion(ctx *gofr.Context) uint64 {
	version, err := ctx.Redis.Get(ctx, zonesCacheVersionKey).Uint64()
	if err != nil {
		return 0
	}

	return version
}

func (r *repository) invalidateZonesCache(ctx *gofr.Context) {
	if ctx.Redis == nil {
		return
	}
	_, _ = ctx.Redis.Incr(ctx, zonesCacheVersionKey).Result()
}

func (r *repository) invalidateDNSCache(ctx *gofr.Context) {
	ctx.GetPublisher().Publish(ctx, dns.DNSCacheInvalidTopic, []byte{})
}

func (r *repository) getZones(
	ctx *gofr.Context,
	limit,
	offset uint32,
) ([]models.ZoneData, error) {
	key := zonesCacheKey(r.zonesCacheVersion(ctx), limit, offset)

	// Try cache first
	cached, err := ctx.Redis.Get(ctx, key).Result()
	if err == nil {
		var zones []models.ZoneData

		if err := json.Unmarshal(
			[]byte(cached),
			&zones,
		); err == nil {
			return zones, nil
		}
	}

	// Fallback to database
	zones, err := queries.GetZones(
		ctx,
		limit,
		offset,
	)
	if err != nil {
		return nil, err
	}

	// Store in cache
	buf, err := json.Marshal(zones)
	if err == nil {
		_ = ctx.Redis.Set(
			ctx,
			key,
			buf,
			zonesCacheTTL,
		).Err()
	}

	return zones, nil
}

func (r *repository) getZone(ctx *gofr.Context, id string) (models.ZoneData, error) {
	return queries.GetZone(ctx, id)
}

func (r *repository) createZone(ctx *gofr.Context, req zoneRequest) (models.ZoneData, error) {
	policy, forwardZoneID, err := normalizeForwardPolicyForCreate(req.ForwardPolicy, req.ForwardZoneID)
	if err != nil {
		return models.ZoneData{}, err
	}

	zone, err := queries.CreateZone(ctx, req.Name, policy, forwardZoneID, req.TTL)
	if err != nil {
		return models.ZoneData{}, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return zone, nil
}

func (r *repository) updateZone(ctx *gofr.Context, id string, req zoneRequest) (models.ZoneData, error) {
	current, err := queries.GetZone(ctx, id)
	if err != nil {
		return models.ZoneData{}, err
	}

	policy, forwardZoneID, err := normalizeForwardPolicyForUpdate(current, req.ForwardPolicy, req.ForwardZoneID)
	if err != nil {
		return models.ZoneData{}, err
	}

	zone, err := queries.UpdateZone(ctx, id, req.Name, policy, forwardZoneID, req.TTL)
	if err != nil {
		return models.ZoneData{}, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return zone, nil
}

func (r *repository) deleteZone(ctx *gofr.Context, id string) (any, error) {
	if err := queries.DeleteZone(ctx, id); err != nil {
		return nil, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return fmt.Sprintf("zone successfully deleted with id: %s", id), nil
}

func (r *repository) getForwardZones(ctx *gofr.Context, limit, offset uint32) ([]models.ForwardZone, error) {
	return queries.GetForwardZones(ctx, limit, offset)
}

func (r *repository) getForwardZone(ctx *gofr.Context, id string) (models.ForwardZone, error) {
	return queries.GetForwardZone(ctx, id)
}

func (r *repository) createForwardZone(ctx *gofr.Context, req forwardZoneRequest) (models.ForwardZone, error) {
	zone, err := queries.CreateForwardZone(ctx, req.Name, req.Addresses)
	if err != nil {
		return models.ForwardZone{}, err
	}

	r.invalidateDNSCache(ctx)
	return zone, nil
}

func (r *repository) updateForwardZone(ctx *gofr.Context, id string, req forwardZoneRequest) (models.ForwardZone, error) {
	zone, err := queries.UpdateForwardZone(ctx, id, req.Name, req.Addresses)
	if err != nil {
		return models.ForwardZone{}, err
	}

	r.invalidateDNSCache(ctx)
	return zone, nil
}

func (r *repository) deleteForwardZone(ctx *gofr.Context, id string) (any, error) {
	if err := queries.DetachForwardZoneFromZones(ctx, id); err != nil {
		return nil, err
	}
	if err := queries.DeleteForwardZone(ctx, id); err != nil {
		return nil, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return fmt.Sprintf("forward zone successfully deleted with id: %s", id), nil
}

func (r *repository) getRecords(ctx *gofr.Context, zoneID string, limit, offset uint32) ([]models.DNSRecord, error) {
	return queries.GetRecords(ctx, zoneID, limit, offset)
}

func (r *repository) getRecord(ctx *gofr.Context, zoneID, id string) (models.DNSRecord, error) {
	return queries.GetRecord(ctx, zoneID, id)
}

func (r *repository) createRecord(ctx *gofr.Context, zoneID string, req recordRequest) (models.DNSRecord, error) {
	zone, err := queries.CreateRecord(ctx, zoneID, req.Name, req.Type, req.Value, req.TTL, req.Priority)
	if err != nil {
		return models.DNSRecord{}, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return zone, nil
}

func (r *repository) updateRecord(ctx *gofr.Context, zoneID, id string, req recordRequest) (models.DNSRecord, error) {
	record, err := queries.UpdateRecord(ctx, zoneID, id, req.Name, req.Type, req.Value, req.TTL, req.Priority)
	if err != nil {
		return models.DNSRecord{}, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return record, nil
}

func (r *repository) deleteRecord(ctx *gofr.Context, zoneID, id string) (any, error) {
	if err := queries.DeleteRecord(ctx, zoneID, id); err != nil {
		return nil, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return fmt.Sprintf("record successfully deleted with id: %s", id), nil
}

func (r *repository) getSettings(ctx *gofr.Context) (models.Settings, error) {
	return queries.GetSettings(ctx)
}

func (r *repository) updateSettings(ctx *gofr.Context, req settingsRequest) (models.Settings, error) {
	settings, err := queries.UpdateSettings(ctx, req.DefaultForwardZoneID)
	if err != nil {
		return models.Settings{}, err
	}

	r.invalidateDNSCache(ctx)
	return settings, nil
}

func normalizeForwardPolicyForCreate(policy, forwardZoneID *string) (string, *string, error) {
	if policy != nil {
		switch strings.TrimSpace(strings.ToLower(*policy)) {
		case "", "default":
			return "default", nil, nil
		case "none":
			return "none", nil, nil
		case "custom":
			if forwardZoneID == nil || strings.TrimSpace(*forwardZoneID) == "" {
				return "", nil, errors.New("forward zone id is required for custom forwarding")
			}
			id := strings.TrimSpace(*forwardZoneID)
			return "custom", &id, nil
		default:
			return "", nil, fmt.Errorf("invalid forward policy: %s", *policy)
		}
	}

	if forwardZoneID == nil {
		return "default", nil, nil
	}

	switch strings.TrimSpace(strings.ToLower(*forwardZoneID)) {
	case "", "default":
		return "default", nil, nil
	case "none":
		return "none", nil, nil
	default:
		id := strings.TrimSpace(*forwardZoneID)
		return "custom", &id, nil
	}
}

func normalizeForwardPolicyForUpdate(current models.ZoneData, policy, forwardZoneID *string) (string, *string, error) {
	if policy == nil && forwardZoneID == nil {
		switch current.ForwardPolicy {
		case "custom":
			id := current.ForwardZoneID
			return "custom", &id, nil
		case "default", "none":
			return current.ForwardPolicy, nil, nil
		default:
			return "default", nil, nil
		}
	}

	return normalizeForwardPolicyForCreate(policy, forwardZoneID)
}
