package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/dns"
	"github.com/fmotalleb/hermes/models"
	"github.com/fmotalleb/hermes/pubsub"
	"github.com/fmotalleb/hermes/queries"
	"github.com/fmotalleb/hermes/registry"
)

type repository struct {
	db       queries.DB
	cache    cache.Cache
	pubsub   pubsub.Bus
	registry *registry.RegistryConnection
}

func newRepository(db queries.DB, cache cache.Cache, pubsubBus pubsub.Bus, registry *registry.RegistryConnection) *repository {
	return &repository{
		db:       db,
		cache:    cache,
		pubsub:   pubsubBus,
		registry: registry,
	}
}

const (
	zonesCacheTTL        = 15 * time.Second
	zonesCacheVersionKey = "zones:list:version"
)

func zonesCacheKey(version uint64, limit, offset uint32) string {
	return fmt.Sprintf("zones:list:%d:%d:%d", version, limit, offset)
}

func (r *repository) zonesCacheVersion(ctx context.Context) uint64 {
	if r.cache == nil {
		return 0
	}

	data, err := r.cache.GetBytes(ctx, zonesCacheVersionKey)
	if err != nil {
		return 0
	}

	var version uint64
	if err := json.Unmarshal(data, &version); err != nil {
		return 0
	}

	return version
}

func (r *repository) invalidateZonesCache(ctx context.Context) {
	if r.cache == nil {
		return
	}

	version := r.zonesCacheVersion(ctx) + 1
	buf, err := json.Marshal(version)
	if err != nil {
		return
	}
	_ = r.cache.Set(ctx, zonesCacheVersionKey, buf, 24*time.Hour)
}

func (r *repository) invalidateDNSCache(ctx context.Context) {
	if r.pubsub == nil {
		return
	}
	_ = r.pubsub.Publish(ctx, dns.DNSCacheInvalidTopic, []byte{})
}

func (r *repository) getZones(ctx context.Context, limit, offset uint32) ([]models.ZoneData, error) {
	if r.cache != nil {
		key := zonesCacheKey(r.zonesCacheVersion(ctx), limit, offset)
		if cached, err := r.cache.GetBytes(ctx, key); err == nil {
			var zones []models.ZoneData
			if err := json.Unmarshal(cached, &zones); err == nil {
				return zones, nil
			}
		}
	}

	zones, err := queries.GetZones(ctx, r.db, limit, offset)
	if err != nil {
		return nil, err
	}

	if r.cache != nil {
		key := zonesCacheKey(r.zonesCacheVersion(ctx), limit, offset)
		if buf, err := json.Marshal(zones); err == nil {
			_ = r.cache.Set(ctx, key, buf, zonesCacheTTL)
		}
	}

	return zones, nil
}

func (r *repository) getZone(ctx context.Context, id string) (models.ZoneData, error) {
	return queries.GetZone(ctx, r.db, id)
}

func (r *repository) createZone(ctx context.Context, req zoneRequest) (models.ZoneData, error) {
	policy, forwardZoneID, err := normalizeForwardPolicyForCreate(req.ForwardPolicy, req.ForwardZoneID)
	if err != nil {
		return models.ZoneData{}, err
	}

	zone, err := queries.CreateZone(ctx, r.db, req.Name, policy, forwardZoneID, req.TTL)
	if err != nil {
		return models.ZoneData{}, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return zone, nil
}

func (r *repository) updateZone(ctx context.Context, id string, req zoneRequest) (models.ZoneData, error) {
	current, err := queries.GetZone(ctx, r.db, id)
	if err != nil {
		return models.ZoneData{}, err
	}

	policy, forwardZoneID, err := normalizeForwardPolicyForUpdate(current, req.ForwardPolicy, req.ForwardZoneID)
	if err != nil {
		return models.ZoneData{}, err
	}

	zone, err := queries.UpdateZone(ctx, r.db, id, req.Name, policy, forwardZoneID, req.TTL)
	if err != nil {
		return models.ZoneData{}, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return zone, nil
}

func (r *repository) deleteZone(ctx context.Context, id string) (any, error) {
	if err := queries.DeleteZone(ctx, r.db, id); err != nil {
		return nil, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return fmt.Sprintf("zone successfully deleted with id: %s", id), nil
}

func (r *repository) getForwardZones(ctx context.Context, limit, offset uint32) ([]models.ForwardZone, error) {
	return queries.GetForwardZones(ctx, r.db, limit, offset)
}

func (r *repository) getForwardZone(ctx context.Context, id string) (models.ForwardZone, error) {
	return queries.GetForwardZone(ctx, r.db, id)
}

func (r *repository) createForwardZone(ctx context.Context, req forwardZoneRequest) (models.ForwardZone, error) {
	zone, err := queries.CreateForwardZone(ctx, r.db, req.Name, req.Addresses)
	if err != nil {
		return models.ForwardZone{}, err
	}

	r.invalidateDNSCache(ctx)
	return zone, nil
}

func (r *repository) updateForwardZone(ctx context.Context, id string, req forwardZoneRequest) (models.ForwardZone, error) {
	zone, err := queries.UpdateForwardZone(ctx, r.db, id, req.Name, req.Addresses)
	if err != nil {
		return models.ForwardZone{}, err
	}

	r.invalidateDNSCache(ctx)
	return zone, nil
}

func (r *repository) deleteForwardZone(ctx context.Context, id string) (any, error) {
	if err := queries.DetachForwardZoneFromZones(ctx, r.db, id); err != nil {
		return nil, err
	}
	if err := queries.DeleteForwardZone(ctx, r.db, id); err != nil {
		return nil, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return fmt.Sprintf("forward zone successfully deleted with id: %s", id), nil
}

func (r *repository) getRecords(ctx context.Context, zoneID string, limit, offset uint32) ([]models.DNSRecord, error) {
	return queries.GetRecords(ctx, r.db, zoneID, limit, offset)
}

func (r *repository) getRecord(ctx context.Context, zoneID, id string) (models.DNSRecord, error) {
	return queries.GetRecord(ctx, r.db, zoneID, id)
}

func (r *repository) createRecord(ctx context.Context, zoneID string, req recordRequest) (models.DNSRecord, error) {
	record, err := queries.CreateRecord(ctx, r.db, zoneID, req.Name, req.Type, req.Value, req.TTL, req.Priority)
	if err != nil {
		return models.DNSRecord{}, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return record, nil
}

func (r *repository) updateRecord(ctx context.Context, zoneID, id string, req recordRequest) (models.DNSRecord, error) {
	record, err := queries.UpdateRecord(ctx, r.db, zoneID, id, req.Name, req.Type, req.Value, req.TTL, req.Priority)
	if err != nil {
		return models.DNSRecord{}, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return record, nil
}

func (r *repository) deleteRecord(ctx context.Context, zoneID, id string) (any, error) {
	if err := queries.DeleteRecord(ctx, r.db, zoneID, id); err != nil {
		return nil, err
	}

	r.invalidateZonesCache(ctx)
	r.invalidateDNSCache(ctx)
	return fmt.Sprintf("record successfully deleted with id: %s", id), nil
}

func (r *repository) getSettings(ctx context.Context) (models.Settings, error) {
	return queries.GetSettings(ctx, r.db)
}

func (r *repository) updateSettings(ctx context.Context, req settingsRequest) (models.Settings, error) {
	settings, err := queries.UpdateSettings(ctx, r.db, req.DefaultForwardZoneID)
	if err != nil {
		return models.Settings{}, err
	}

	r.invalidateDNSCache(ctx)
	return settings, nil
}

func normalizeForwardPolicyForCreate(policy, forwardZoneID *string) (string, *string, error) {
	if policy != nil {
		switch strings.TrimSpace(strings.ToLower(*policy)) {
		case "", models.ForwardPolicyDefault:
			return models.ForwardPolicyDefault, nil, nil
		case models.ForwardPolicyNone:
			return models.ForwardPolicyNone, nil, nil
		case models.ForwardPolicyCustom:
			if forwardZoneID == nil || strings.TrimSpace(*forwardZoneID) == "" {
				return "", nil, errors.New("forward zone id is required for custom forwarding")
			}
			id := strings.TrimSpace(*forwardZoneID)
			return models.ForwardPolicyCustom, &id, nil
		default:
			return "", nil, fmt.Errorf("invalid forward policy: %s", *policy)
		}
	}

	if forwardZoneID == nil {
		return models.ForwardPolicyDefault, nil, nil
	}

	switch strings.TrimSpace(strings.ToLower(*forwardZoneID)) {
	case "", models.ForwardPolicyDefault:
		return models.ForwardPolicyDefault, nil, nil
	case models.ForwardPolicyNone:
		return models.ForwardPolicyNone, nil, nil
	default:
		id := strings.TrimSpace(*forwardZoneID)
		return models.ForwardPolicyCustom, &id, nil
	}
}

func normalizeForwardPolicyForUpdate(current models.ZoneData, policy, forwardZoneID *string) (string, *string, error) {
	if policy == nil && forwardZoneID == nil {
		switch current.ForwardPolicy {
		case models.ForwardPolicyCustom:
			id := current.ForwardZoneID
			return models.ForwardPolicyCustom, &id, nil
		case models.ForwardPolicyDefault, "none":
			return current.ForwardPolicy, nil, nil
		default:
			return models.ForwardPolicyDefault, nil, nil
		}
	}

	return normalizeForwardPolicyForCreate(policy, forwardZoneID)
}

func (r *repository) getHijacks(ctx context.Context, limit, offset uint32) ([]models.HijackRecord, error) {
	return queries.GetHijacks(ctx, r.db, limit, offset)
}

func (r *repository) getHijack(ctx context.Context, id string) (models.HijackRecord, error) {
	return queries.GetHijack(ctx, r.db, id)
}

func (r *repository) searchHijack(ctx context.Context, name string) ([]models.HijackRecord, error) {
	return queries.SearchHijack(ctx, r.db, name)
}

func (r *repository) hijackLookup(ctx context.Context, name string, recordType models.DNSRecordType) (models.HijackRecord, error) {
	return queries.HijackLookup(ctx, r.db, name, recordType)
}

func (r *repository) createHijack(ctx context.Context, req hijackRequest) (models.HijackRecord, error) {
	record, err := queries.CreateHijack(
		ctx,
		r.db,
		req.Name,
		req.Value,
		req.Type,
		req.Policy,
		req.ForwardPolicy,
		req.ForwardZoneID,
		req.TTL,
	)
	if err != nil {
		return models.HijackRecord{}, err
	}

	r.invalidateDNSCache(ctx)
	return record, nil
}

func (r *repository) updateHijack(ctx context.Context, id string, req hijackRequest) (models.HijackRecord, error) {
	record, err := queries.UpdateHijack(
		ctx,
		r.db,
		id,
		req.Name,
		req.Value,
		req.Type,
		req.Policy,
		req.ForwardPolicy,
		req.ForwardZoneID,
		req.TTL,
	)
	if err != nil {
		return models.HijackRecord{}, err
	}

	r.invalidateDNSCache(ctx)
	return record, nil
}

func (r *repository) deleteHijack(ctx context.Context, id string) (any, error) {
	if err := queries.DeleteHijack(ctx, r.db, id); err != nil {
		return nil, err
	}

	r.invalidateDNSCache(ctx)
	return fmt.Sprintf("hijack successfully deleted with id: %s", id), nil
}

func (r *repository) getServices(ctx context.Context, kind string) (any, error) {
	return r.registry.ListKind(ctx, kind)
}
