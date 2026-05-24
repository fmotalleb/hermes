package cache

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

var ErrCacheMiss = errors.New("cache miss")

type memoryCache struct {
	mu    sync.RWMutex
	items map[string]entry

	stopGC   chan struct{}
	gcTicker *time.Ticker
}

type entry struct {
	value     any
	expires   time.Time
	hasExpiry bool
}

type MemCacheOption struct {
	GCTickInterval time.Duration
}

func NewMemoryCache(ctx context.Context, opts ...MemCacheOption) *memoryCache {
	gcTickInterval := time.Minute
	for _, opt := range opts {
		if opt.GCTickInterval > 0 {
			gcTickInterval = opt.GCTickInterval
		}
	}
	m := &memoryCache{
		items:    make(map[string]entry),
		stopGC:   make(chan struct{}),
		gcTicker: time.NewTicker(gcTickInterval),
	}
	go m.gcLoop()
	go func() {
		<-ctx.Done()
		m.Close()
	}()
	return m
}

func (m *memoryCache) Close() {
	close(m.stopGC)
	m.gcTicker.Stop()
}

func (m *memoryCache) gcLoop() {
	for {
		select {
		case <-m.gcTicker.C:
			m.gc()

		case <-m.stopGC:
			return
		}
	}
}

func (m *memoryCache) gc() {
	now := time.Now()

	m.mu.Lock()

	for key, item := range m.items {
		if item.hasExpiry && now.After(item.expires) {
			delete(m.items, key)
		}
	}

	m.mu.Unlock()
}

func (m *memoryCache) GetBytes(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	item, ok := m.items[key]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrCacheMiss
	}

	if item.hasExpiry && time.Now().After(item.expires) {
		m.mu.Lock()
		delete(m.items, key)
		m.mu.Unlock()

		return nil, ErrCacheMiss
	}

	switch v := item.value.(type) {
	case []byte:
		return v, nil

	case string:
		return []byte(v), nil

	case uint64:
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, v)
		return buf, nil

	default:
		return nil, errors.New("value is not bytes-compatible")
	}
}

func (m *memoryCache) GetUint64(_ context.Context, key string) (uint64, error) {
	m.mu.RLock()
	item, ok := m.items[key]
	m.mu.RUnlock()

	if !ok {
		return 0, ErrCacheMiss
	}

	if item.hasExpiry && time.Now().After(item.expires) {
		m.mu.Lock()
		delete(m.items, key)
		m.mu.Unlock()

		return 0, ErrCacheMiss
	}

	switch v := item.value.(type) {
	case uint64:
		return v, nil

	case int:
		return uint64(v), nil

	case int64:
		return uint64(v), nil

	case []byte:
		if len(v) != 8 {
			return 0, errors.New("invalid uint64 byte length")
		}

		return binary.BigEndian.Uint64(v), nil

	default:
		return 0, errors.New("value is not uint64-compatible")
	}
}

func (m *memoryCache) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	item := entry{
		value: value,
	}

	if ttl > 0 {
		item.hasExpiry = true
		item.expires = time.Now().Add(ttl)
	}

	m.mu.Lock()
	m.items[key] = item
	m.mu.Unlock()

	return nil
}
