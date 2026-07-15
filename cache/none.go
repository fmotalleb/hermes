package cache

import (
	"context"
	"time"
)

type noneCache struct{}

// NewNoneCache returns a no-op cache that always returns ErrCacheMiss on reads
// and silently discards all writes. Useful for disabling caching entirely.
func NewNoneCache() Cache {
	return &noneCache{}
}

func (*noneCache) Close() {
}

func (*noneCache) GetBytes(_ context.Context, _ string) ([]byte, error) {
	return nil, ErrCacheMiss
}

func (*noneCache) Set(_ context.Context, _ string, _ any, _ time.Duration) error {
	return nil
}

func (*noneCache) Clear(c context.Context) error {
	return nil
}

// Delete implements [Cache].
func (*noneCache) Delete(context.Context, string) error {
	return nil
}

// DeleteSelector implements [Cache].
func (*noneCache) DeleteSelector(context.Context, func(string) bool) error {
	return nil
}

// DeletePattern implements [Cache].
func (*noneCache) DeletePattern(ctx context.Context, pattern string) error {
	return nil
}
