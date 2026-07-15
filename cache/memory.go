package cache

import (
	"context"
	"errors"
	"time"

	"github.com/gobwas/glob"
	"github.com/maypok86/otter/v2"
)

var (
	// ErrCacheMiss is returned when a requested key is not found in the cache.
	ErrCacheMiss = errors.New("cache miss")
	// ErrStoreFailed is returned when the underlying storage fails to persist a value.
	ErrStoreFailed = errors.New("failed to store value: storage exception")
	// ErrStoreUnsupportedType is returned when attempting to store a non-[]byte value.
	ErrStoreUnsupportedType = errors.New("failed to store value: unsupported type")
)

const defaultMaxSize = 100_000

type memoryCache struct {
	storage *otter.Cache[string, entry]
}

type entry struct {
	value     any
	ttl       time.Duration
	hasExpiry bool
}

// MemCacheOption configures the in-memory cache.
type MemCacheOption struct {
	// MaxSize is the maximum number of entries the cache can hold.
	MaxSize int
}

// NewMemoryCache creates an in-memory cache using an Otter concurrent cache.
// Optional MemCacheOption values can be provided to configure limits.
func NewMemoryCache(_ context.Context, opts ...MemCacheOption) Cache {
	maxSize := defaultMaxSize
	for _, opt := range opts {
		if opt.MaxSize != 0 {
			maxSize = opt.MaxSize
		}
	}
	storage := otter.Must(&otter.Options[string, entry]{
		ExpiryCalculator: otter.ExpiryAccessingFunc(func(entry otter.Entry[string, entry]) time.Duration {
			return entry.Value.ttl
		}),
		RefreshCalculator: otter.RefreshWritingFunc(func(entry otter.Entry[string, entry]) time.Duration {
			return entry.Value.ttl
		}),
		MaximumSize: maxSize,
	})

	return &memoryCache{
		storage: storage,
	}
}

func (m *memoryCache) GetBytes(_ context.Context, key string) ([]byte, error) {
	entry, ok := m.storage.GetEntry(key)
	if !ok {
		return nil, ErrCacheMiss
	}
	return entry.Value.value.([]byte), nil
}

func (m *memoryCache) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	if _, ok := value.([]byte); !ok {
		return ErrStoreUnsupportedType
	}
	_, ok := m.storage.Set(key, entry{
		hasExpiry: true,
		value:     value,
		ttl:       ttl,
	})
	if !ok {
		return ErrStoreFailed
	}
	return nil
}

func (m *memoryCache) Clear(c context.Context) error {
	m.storage.InvalidateAll()
	return nil
}

// Delete implements [Cache].
func (m *memoryCache) Delete(_ context.Context, key string) error {
	m.storage.Invalidate(key)
	return nil
}

// DeleteSelector implements [Cache].
func (m *memoryCache) DeleteSelector(_ context.Context, selector func(string) bool) error {
	for k := range m.storage.Keys() {
		if selector(k) {
			m.storage.Invalidate(k)
		}
	}
	return nil
}

// DeletePattern implements [Cache].
func (m *memoryCache) DeletePattern(ctx context.Context, pattern string) error {
	matcher, err := glob.Compile(pattern)
	if err != nil {
		return err
	}
	return m.DeleteSelector(ctx, matcher.Match)
}
