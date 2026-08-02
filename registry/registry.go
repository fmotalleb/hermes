// Package registry provides service discovery and health tracking via Redis.
// Each service instance registers itself with a heartbeat and can discover
// other instances of the same or different service kinds.
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/fmotalleb/go-tools/log"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ServiceKind identifies a category of service in the registry.
type ServiceKind = string

const (
	ServiceKindAPI      = ServiceKind("api")      // REST API service.
	ServiceKindProxy    = ServiceKind("proxy")    // Transparent proxy service.
	ServiceKindDNS      = ServiceKind("dns")      // DNS resolver service.
	ServiceKindMigrator = ServiceKind("migrator") // Database migration service.
)

// Entry represents a single registered service instance with its metadata and last-seen timestamp.
type Entry struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	IP       net.IP         `json:"ip_address"`
	Metadata map[string]any `json:"metadata"`
	LastSeen time.Time      `json:"lastSeen"`
}

// RegistryConnection manages a service instance's registration and heartbeat in Redis.
type RegistryConnection struct {
	redis          redis.UniversalClient
	instanceID     string
	kind           string
	metadata       map[string]any
	key            string
	ttl            time.Duration
	heartbeatEvery time.Duration

	// hbHealthy tracks whether the last heartbeat succeeded. It is only touched
	// by Start and the heartbeat goroutine, so no locking is needed. It is used
	// to log connectivity transitions (first failure, and recovery) instead of
	// logging every retry while Redis is down.
	hbHealthy bool

	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// NewRegistryConnection creates a new registry connection for the given instance.
// It does not start the heartbeat until Start is called.
func NewRegistryConnection(
	rdb redis.UniversalClient,
	instanceID string,
	kind string,
	metadata map[string]any,
) *RegistryConnection {
	return &RegistryConnection{
		redis:          rdb,
		instanceID:     instanceID,
		kind:           kind,
		metadata:       cloneMap(metadata),
		key:            redisKey(kind, instanceID),
		ttl:            10 * time.Second,
		heartbeatEvery: 5 * time.Second,
		hbHealthy:      true,
	}
}

func redisKey(kind, instanceID string) string {
	return fmt.Sprintf("registry:%s:%s", kind, instanceID)
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Start does an immediate check-in and then starts heartbeat in the background.
func (c *RegistryConnection) Start(ctx context.Context, addr net.IP) error {
	ctx, logger := log.AsNamedChild(ctx, "registry")

	if err := c.checkIn(ctx, addr); err != nil {
		return err
	}

	logger.Info("service registered",
		zap.String("kind", c.kind),
		zap.String("instance_id", c.instanceID),
		zap.String("key", c.key),
		zap.String("addr", addr.String()),
	)

	hbCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		ticker := time.NewTicker(c.heartbeatEvery)
		defer ticker.Stop()

		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				// Simple: rewrite the full payload every heartbeat.
				// If Redis is temporarily down, next tick will retry.
				_ = c.checkIn(hbCtx, addr)
			}
		}
	}()

	return nil
}

func (c *RegistryConnection) checkIn(ctx context.Context, addr net.IP) error {
	c.mu.RLock()
	entry := Entry{
		ID:       c.instanceID,
		Kind:     c.kind,
		IP:       addr,
		Metadata: cloneMap(c.metadata),
		LastSeen: time.Now(),
	}
	c.mu.RUnlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if err := c.redis.Set(ctx, c.key, data, c.ttl).Err(); err != nil {
		// Log the failure only when transitioning from healthy, so an ongoing
		// Redis outage does not spam a warning every heartbeat tick. The logger
		// is inherited from the registry child attached by Start.
		if c.hbHealthy {
			log.FromContext(ctx).Warn("registry heartbeat failed",
				zap.String("kind", c.kind),
				zap.String("instance_id", c.instanceID),
				zap.Error(err),
			)
		}
		c.hbHealthy = false
		return err
	}

	if !c.hbHealthy {
		log.FromContext(ctx).Info("registry heartbeat recovered",
			zap.String("kind", c.kind),
			zap.String("instance_id", c.instanceID),
		)
	}
	c.hbHealthy = true
	return nil
}

// Stop stops heartbeat and removes this instance from Redis.
func (c *RegistryConnection) Stop(ctx context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()

	ctx, logger := log.AsNamedChild(ctx, "registry")
	if err := c.redis.Del(ctx, c.key).Err(); err != nil {
		logger.Warn("registry deregister failed",
			zap.String("kind", c.kind),
			zap.String("instance_id", c.instanceID),
			zap.Error(err),
		)
		return err
	}

	logger.Info("service deregistered",
		zap.String("kind", c.kind),
		zap.String("instance_id", c.instanceID),
	)
	return nil
}

// GetSelf reads this instance's current data from Redis.
func (c *RegistryConnection) GetSelf(ctx context.Context) (*Entry, error) {
	return c.GetByID(ctx, c.kind, c.instanceID)
}

// GetByID reads one instance from Redis.
func (c *RegistryConnection) GetByID(ctx context.Context, kind, instanceID string) (*Entry, error) {
	val, err := c.redis.Get(ctx, redisKey(kind, instanceID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var entry Entry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// ListKind returns all currently alive instances of a kind.
// It uses SCAN, not KEYS.
func (c *RegistryConnection) ListKind(ctx context.Context, kind string) ([]Entry, error) {
	pattern := fmt.Sprintf("registry:%s:*", kind)

	var (
		cursor uint64
		keys   []string
	)

	for {
		batch, nextCursor, err := c.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	ctx, logger := log.AsNamedChild(ctx, "registry")
	if len(keys) == 0 {
		logger.Debug("registry list", zap.String("kind", kind), zap.Int("count", 0))
		return []Entry{}, nil
	}

	values, err := c.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(values))
	for _, v := range values {
		if v == nil {
			continue
		}

		s, ok := v.(string)
		if !ok {
			continue
		}

		var entry Entry
		if err := json.Unmarshal([]byte(s), &entry); err != nil {
			continue
		}

		entries = append(entries, entry)
	}

	logger.Debug("registry list", zap.String("kind", kind), zap.Int("count", len(entries)))
	return entries, nil
}

// ListSameKind returns all alive peers with the same kind as this instance.
func (c *RegistryConnection) ListSameKind(ctx context.Context) ([]Entry, error) {
	return c.ListKind(ctx, c.kind)
}

func (c *RegistryConnection) OnDelete(ctx context.Context, dbIndex int, kind ServiceKind, callback func(string)) error {
	ctx, logger := log.AsNamedChild(ctx, "registry")
	pubsub := c.redis.PSubscribe(
		ctx,
		fmt.Sprintf("__keyevent@%d__:del", dbIndex),
		fmt.Sprintf("__keyevent@%d__:expired", dbIndex),
	)
	for msg := range pubsub.Channel() {
		if strings.HasPrefix(msg.Payload, fmt.Sprintf("registry:%s:", kind)) {
			logger.Debug("registry peer removed",
				zap.String("kind", kind),
				zap.String("key", msg.Payload),
			)
			callback(msg.Payload)
		}
	}
	return nil
}
