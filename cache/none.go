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

func (_ *noneCache) GetUint64(_ context.Context, _ string) (uint64, error) {
	return 0, ErrCacheMiss
}

func (_ *noneCache) Set(_ context.Context, _ string, _ any, _ time.Duration) error {
	return nil
}

func (_ *noneCache) Clear(c context.Context) error {
	return nil
}
