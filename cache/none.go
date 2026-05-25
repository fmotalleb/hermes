package cache

import (
	"context"
	"time"
)

type noneCache struct{}

func NewNoneCache() Cache {
	return &noneCache{}
}

func (_ *noneCache) Close() {
}

func (_ *noneCache) GetBytes(_ context.Context, _ string) ([]byte, error) {
	return nil, ErrCacheMiss
}

func (_ *noneCache) Set(_ context.Context, _ string, _ any, _ time.Duration) error {
	return nil
}

func (_ *noneCache) Clear(c context.Context) error {
	return nil
}

// Delete implements [Cache].
func (_ *noneCache) Delete(context.Context, string) error {
	return nil
}

// DeleteSelector implements [Cache].
func (_ *noneCache) DeleteSelector(context.Context, func(string) bool) error {
	return nil
}

// DeletePattern implements [Cache].
func (_ *noneCache) DeletePattern(ctx context.Context, pattern string) error {
	return nil
}
