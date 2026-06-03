package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type ServiceKind = string

const (
	ServiceKindAPI      = ServiceKind("api")
	ServiceKindProxy    = ServiceKind("proxy")
	ServiceKindDNS      = ServiceKind("dns")
	ServiceKindMigrator = ServiceKind("migrator")
)

type Entry struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Metadata map[string]any `json:"metadata"`
	LastSeen time.Time      `json:"lastSeen"`
}

type RegistryConnection struct {
	redis          redis.UniversalClient
	instanceID     string
	kind           string
	metadata       map[string]any
	key            string
	ttl            time.Duration
	heartbeatEvery time.Duration

	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

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
func (c *RegistryConnection) Start(ctx context.Context) error {
	if err := c.checkIn(ctx); err != nil {
		return err
	}

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
				_ = c.checkIn(hbCtx)
			}
		}
	}()

	return nil
}

func (c *RegistryConnection) checkIn(ctx context.Context) error {
	c.mu.RLock()
	entry := Entry{
		ID:       c.instanceID,
		Kind:     c.kind,
		Metadata: cloneMap(c.metadata),
		LastSeen: time.Now(),
	}
	c.mu.RUnlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return c.redis.Set(ctx, c.key, data, c.ttl).Err()
}

// Stop stops heartbeat and removes this instance from Redis.
func (c *RegistryConnection) Stop(ctx context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	return c.redis.Del(ctx, c.key).Err()
}

// GetSelf reads this instance's current data from Redis.
func (c *RegistryConnection) GetSelf(ctx context.Context) (*Entry, error) {
	return c.GetByID(ctx, c.kind, c.instanceID)
}

// GetByID reads one instance from Redis.
func (c *RegistryConnection) GetByID(ctx context.Context, kind, instanceID string) (*Entry, error) {
	val, err := c.redis.Get(ctx, redisKey(kind, instanceID)).Result()
	if err == redis.Nil {
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

	if len(keys) == 0 {
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

	return entries, nil
}

// ListSameKind returns all alive peers with the same kind as this instance.
func (c *RegistryConnection) ListSameKind(ctx context.Context) ([]Entry, error) {
	return c.ListKind(ctx, c.kind)
}

func (c *RegistryConnection) OnDelete(ctx context.Context, dbIndex int, kind ServiceKind, callback func(string)) error {
	pubsub := c.redis.PSubscribe(
		ctx,
		fmt.Sprintf("__keyevent@%d__:expired", dbIndex),
	)
	for msg := range pubsub.Channel() {
		if strings.HasPrefix(msg.Payload, fmt.Sprintf("registry:%s:", kind)) {
			callback(msg.Payload)
		}
	}
	return nil
}
