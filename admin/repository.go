package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	view "github.com/fmotalleb/helios/templates"
	"github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr"
)

const (
	cacheKeyZones         = "dns_admin:zones:v1"
	cacheTTL              = 30 * time.Second
	topicDNSRecordChanges = "dns-record-changes"
	defaultTTL            = 300
	forwardModeDefault    = "default"
	forwardModeNone       = "none"
	forwardModeCustom     = "custom"
)

var (
	errZoneNotFound   = errors.New("zone not found")
	errZoneExists     = errors.New("zone already exists")
	errRecordNotFound = errors.New("record not found")
	hostnameRegex     = regexp.MustCompile(`^(?i)([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)*[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])\.?$`)
)

type repository struct{}

func newRepository() *repository {
	return &repository{}
}

func (r *repository) listZones(ctx *gofr.Context) ([]view.ZoneData, error) {
	if ctx.Redis != nil {
		cached, err := ctx.Redis.Get(ctx, cacheKeyZones).Result()
		if err == nil {
			var zones []view.ZoneData
			if unmarshalErr := json.Unmarshal([]byte(cached), &zones); unmarshalErr == nil {
				return zones, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			ctx.Logger.Warnf("redis cache get failed: %v", err)
		}
	}

	fallbackID, err := r.getFallbackForwardZoneID(ctx)
	if err != nil {
		return nil, err
	}

	const query = `
SELECT z.name, COALESCE(z.forward_mode, 'default'), COALESCE(z.forward_zone_id::text, ''), COALESCE(fz.name || ' (' || fz.address || ')', ''), COALESCE(z.cache_ttl, 300), r.id::text, COALESCE(r.name, ''), COALESCE(r.type, ''), COALESCE(r.value, ''), COALESCE(r.ttl, 0), COALESCE(r.priority, 0)
FROM zones z
LEFT JOIN forward_zones fz ON fz.id = z.forward_zone_id
LEFT JOIN records r ON r.zone_id = z.id
ORDER BY z.name ASC, r.id ASC`

	rows, err := ctx.SQL.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	zones := make([]view.ZoneData, 0)
	byName := make(map[string]int)

	for rows.Next() {
		var (
			zoneName       string
			forwardMode    string
			forwardZoneID  string
			forwardLabelDB string
			cacheTTL       int
			recordID       sql.NullString
			recName        string
			recType        string
			recValue       string
			recTTL         int
			priority       int
		)
		if scanErr := rows.Scan(&zoneName, &forwardMode, &forwardZoneID, &forwardLabelDB, &cacheTTL, &recordID, &recName, &recType, &recValue, &recTTL, &priority); scanErr != nil {
			return nil, scanErr
		}

		idx, ok := byName[zoneName]
		if !ok {
			forwardLabel := "None"
			switch forwardMode {
			case forwardModeDefault:
				if fallbackID == "" {
					forwardLabel = "Default fallback: None"
				} else {
					forwardLabel = "Default fallback"
				}
			case forwardModeCustom:
				if forwardLabelDB != "" {
					forwardLabel = forwardLabelDB
				}
			}

			zones = append(zones, view.ZoneData{
				Name:          zoneName,
				ForwardMode:   forwardMode,
				ForwardZoneID: forwardZoneID,
				ForwardLabel:  forwardLabel,
				CacheTTL:      cacheTTL,
				Records:       make([]view.DNSRecord, 0),
			})
			idx = len(zones) - 1
			byName[zoneName] = idx
		}

		if recordID.Valid && recordID.String != "" {
			zones[idx].Records = append(zones[idx].Records, view.DNSRecord{
				ID:       recordID.String,
				Name:     recName,
				Type:     recType,
				Value:    recValue,
				TTL:      recTTL,
				Priority: priority,
			})
		}
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	r.cacheZones(ctx, zones)

	return zones, nil
}

func (r *repository) createZone(ctx *gofr.Context, name, forwardMode, forwardZoneID string, cacheTTL int) error {
	zone := normalizeZone(name)
	if zone == "" {
		return errors.New("zone is required")
	}
	forwardMode = normalizeForwardMode(forwardMode)
	forwardZoneID = strings.TrimSpace(forwardZoneID)
	if cacheTTL < 0 {
		return errors.New("cache ttl must be non-negative")
	}
	if forwardMode == forwardModeCustom && forwardZoneID == "" {
		return errors.New("forward zone is required for custom mode")
	}
	if forwardMode != forwardModeCustom {
		forwardZoneID = ""
	}

	const query = `INSERT INTO zones (name, forward_mode, forward_zone_id, cache_ttl) VALUES ($1, $2, NULLIF($3, ''), $4)`
	if _, err := ctx.SQL.ExecContext(ctx, query, zone, forwardMode, forwardZoneID, cacheTTL); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			return errZoneExists
		}
		return err
	}

	r.invalidateAndPublish(ctx, `{"event":"zone.created","zone":"`+zone+`"}`)

	return nil
}

func (r *repository) deleteZone(ctx *gofr.Context, name string) error {
	zone := normalizeZone(name)

	const query = `DELETE FROM zones WHERE name = $1`
	res, err := ctx.SQL.ExecContext(ctx, query, zone)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errZoneNotFound
	}

	r.invalidateAndPublish(ctx, `{"event":"zone.deleted","zone":"`+zone+`"}`)

	return nil
}

func (r *repository) updateZoneConfig(ctx *gofr.Context, name, forwardMode, forwardZoneID string, cacheTTL int) error {
	zone := normalizeZone(name)
	forwardMode = normalizeForwardMode(forwardMode)
	forwardZoneID = strings.TrimSpace(forwardZoneID)
	if cacheTTL < 0 {
		return errors.New("cache ttl must be non-negative")
	}
	if forwardMode == forwardModeCustom && forwardZoneID == "" {
		return errors.New("forward zone is required for custom mode")
	}
	if forwardMode != forwardModeCustom {
		forwardZoneID = ""
	}

	const query = `UPDATE zones SET forward_mode = $2, forward_zone_id = NULLIF($3, ''), cache_ttl = $4 WHERE name = $1`
	res, err := ctx.SQL.ExecContext(ctx, query, zone, forwardMode, forwardZoneID, cacheTTL)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errZoneNotFound
	}
	r.invalidateAndPublish(ctx, `{"event":"zone.config.updated","zone":"`+zone+`"}`)
	return nil
}

func (r *repository) createRecord(ctx *gofr.Context, zone string, record view.DNSRecord) error {
	zone = normalizeZone(zone)
	record.Name = strings.TrimSpace(record.Name)
	record.Type = strings.ToUpper(strings.TrimSpace(record.Type))
	record.Value = strings.TrimSpace(record.Value)
	if record.TTL <= 0 {
		record.TTL = defaultTTL
	}

	if zone == "" {
		return errors.New("zone is required")
	}
	if record.Name == "" || record.Type == "" || record.Value == "" {
		return errors.New("name, type and value are required")
	}
	if err := validateRecordByType(record.Type, record.Value); err != nil {
		return err
	}

	const query = `
INSERT INTO records (zone_id, name, type, value, ttl, priority)
SELECT id, $2, $3, $4, $5, $6
FROM zones
WHERE name = $1`

	res, err := ctx.SQL.ExecContext(ctx, query, zone, record.Name, record.Type, record.Value, record.TTL, record.Priority)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errZoneNotFound
	}

	r.invalidateAndPublish(ctx, `{"event":"record.created","zone":"`+zone+`"}`)

	return nil
}

func (r *repository) deleteRecord(ctx *gofr.Context, zone, recordID string) error {
	zone = normalizeZone(zone)
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return errRecordNotFound
	}

	const query = `
DELETE FROM records r
USING zones z
WHERE r.id = $1 AND r.zone_id = z.id AND z.name = $2`

	res, err := ctx.SQL.ExecContext(ctx, query, recordID, zone)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errRecordNotFound
	}

	r.invalidateAndPublish(ctx, `{"event":"record.deleted","zone":"`+zone+`","id":"`+recordID+`"}`)

	return nil
}

func (r *repository) listForwardZones(ctx *gofr.Context) ([]view.ForwardZoneOption, error) {
	const query = `SELECT id::text, name, address FROM forward_zones ORDER BY name ASC`
	rows, err := ctx.SQL.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]view.ForwardZoneOption, 0)
	for rows.Next() {
		var opt view.ForwardZoneOption
		if scanErr := rows.Scan(&opt.ID, &opt.Name, &opt.Address); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, opt)
	}

	return out, rows.Err()
}

func (r *repository) createForwardZone(ctx *gofr.Context, name, address string) error {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	if name == "" || address == "" {
		return errors.New("forward zone name and address are required")
	}
	const query = `INSERT INTO forward_zones (name, address) VALUES ($1, $2)`
	_, err := ctx.SQL.ExecContext(ctx, query, name, address)
	return err
}

func (r *repository) deleteForwardZone(ctx *gofr.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("forward zone not found")
	}
	const query = `DELETE FROM forward_zones WHERE id = $1`
	res, err := ctx.SQL.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("forward zone not found")
	}
	return nil
}

func (r *repository) getInboundSettings(ctx *gofr.Context) (view.InboundSettings, error) {
	const query = `SELECT udp_listen_address, udp_port, tcp_listen_address, tcp_port, tls_listen_address, tls_port, tls_public_key, tls_private_key, https_listen_address, https_port, https_public_key, https_private_key FROM inbound_settings LIMIT 1`
	var in view.InboundSettings
	err := ctx.SQL.QueryRowContext(ctx, query).Scan(
		&in.UDPListenAddress, &in.UDPPort, &in.TCPListenAddress, &in.TCPPort,
		&in.TLSListenAddress, &in.TLSPort, &in.TLSPublicKey, &in.TLSPrivateKey,
		&in.HTTPSListenAddress, &in.HTTPSPort, &in.HTTPSPublicKey, &in.HTTPSPrivateKey,
	)
	if err == nil {
		return in, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return view.InboundSettings{
			UDPListenAddress: "0.0.0.0", UDPPort: 53, TCPListenAddress: "0.0.0.0", TCPPort: 53,
			TLSListenAddress: "0.0.0.0", TLSPort: 853, HTTPSListenAddress: "0.0.0.0", HTTPSPort: 443,
		}, nil
	}
	return view.InboundSettings{}, err
}

func (r *repository) getFallbackForwardZoneID(ctx *gofr.Context) (string, error) {
	const query = `SELECT COALESCE(fallback_forward_zone_id::text, '') FROM inbound_settings WHERE id = 1`
	var id string
	err := ctx.SQL.QueryRowContext(ctx, query).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *repository) updateFallbackForwardZone(ctx *gofr.Context, id string) error {
	id = strings.TrimSpace(id)
	const query = `UPDATE inbound_settings SET fallback_forward_zone_id = NULLIF($1, '') WHERE id = 1`
	_, err := ctx.SQL.ExecContext(ctx, query, id)
	return err
}

func (r *repository) updateInboundSettings(ctx *gofr.Context, in view.InboundSettings) error {
	const query = `
INSERT INTO inbound_settings (
  id, udp_listen_address, udp_port, tcp_listen_address, tcp_port, tls_listen_address, tls_port, tls_public_key, tls_private_key, https_listen_address, https_port, https_public_key, https_private_key
) VALUES (
  1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
ON CONFLICT (id) DO UPDATE SET
  udp_listen_address = EXCLUDED.udp_listen_address,
  udp_port = EXCLUDED.udp_port,
  tcp_listen_address = EXCLUDED.tcp_listen_address,
  tcp_port = EXCLUDED.tcp_port,
  tls_listen_address = EXCLUDED.tls_listen_address,
  tls_port = EXCLUDED.tls_port,
  tls_public_key = EXCLUDED.tls_public_key,
  tls_private_key = EXCLUDED.tls_private_key,
  https_listen_address = EXCLUDED.https_listen_address,
  https_port = EXCLUDED.https_port,
  https_public_key = EXCLUDED.https_public_key,
  https_private_key = EXCLUDED.https_private_key`
	_, err := ctx.SQL.ExecContext(ctx, query,
		strings.TrimSpace(in.UDPListenAddress), in.UDPPort,
		strings.TrimSpace(in.TCPListenAddress), in.TCPPort,
		strings.TrimSpace(in.TLSListenAddress), in.TLSPort,
		strings.TrimSpace(in.TLSPublicKey), strings.TrimSpace(in.TLSPrivateKey),
		strings.TrimSpace(in.HTTPSListenAddress), in.HTTPSPort,
		strings.TrimSpace(in.HTTPSPublicKey), strings.TrimSpace(in.HTTPSPrivateKey),
	)
	return err
}

func (r *repository) cacheZones(ctx *gofr.Context, zones []view.ZoneData) {
	if ctx.Redis == nil {
		return
	}

	payload, err := json.Marshal(zones)
	if err != nil {
		return
	}

	if err = ctx.Redis.Set(ctx, cacheKeyZones, payload, cacheTTL).Err(); err != nil {
		ctx.Logger.Warnf("redis cache set failed: %v", err)
	}
}

func (r *repository) invalidateAndPublish(ctx *gofr.Context, event string) {
	if ctx.Redis != nil {
		if err := ctx.Redis.Del(ctx, cacheKeyZones).Err(); err != nil {
			ctx.Logger.Warnf("redis cache delete failed: %v", err)
		}
	}

	if ctx.PubSub != nil {
		if err := ctx.PubSub.Publish(ctx, topicDNSRecordChanges, []byte(event)); err != nil {
			ctx.Logger.Warnf("pubsub publish failed: %v", err)
		}
	}
}

func normalizeZone(zone string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(zone)), ".")
}

func normalizeForwardMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case forwardModeNone:
		return forwardModeNone
	case forwardModeCustom:
		return forwardModeCustom
	default:
		return forwardModeDefault
	}
}

func validateRecordByType(recordType, value string) error {
	switch recordType {
	case "A":
		ip := net.ParseIP(value)
		if ip == nil || ip.To4() == nil {
			return errors.New("A record value must be a valid IPv4 address")
		}
	case "AAAA":
		ip := net.ParseIP(value)
		if ip == nil || ip.To4() != nil {
			return errors.New("AAAA record value must be a valid IPv6 address")
		}
	case "CNAME", "NS":
		if !isValidHostname(value) {
			return fmt.Errorf("%s record value must be a valid hostname", recordType)
		}
	case "MX":
		if !isValidHostname(value) {
			return errors.New("MX record value must be a valid mail hostname")
		}
	case "TXT":
		// TXT can contain arbitrary data; only non-empty is required and already checked.
	default:
		return fmt.Errorf("unsupported record type: %s", recordType)
	}

	return nil
}

func isValidHostname(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || len(host) > 253 {
		return false
	}

	return hostnameRegex.MatchString(host)
}
