package cache

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/container"
)

type redisCache struct {
	db container.Redis
}

func NewRedisCache(db container.Redis) Cache {
	return &redisCache{
		db,
	}
}

func (r *redisCache) GetBytes(c context.Context, key string) ([]byte, error) {
	stat := r.db.Get(c, key)
	if err := stat.Err(); err != nil {
		return nil, err
	}
	return stat.Bytes()
}

func (r *redisCache) GetUint64(c context.Context, key string) (uint64, error) {
	stat := r.db.Get(c, key)
	if err := stat.Err(); err != nil {
		return 0, err
	}
	return stat.Uint64()
}

func (r *redisCache) Set(c context.Context, key string, value any, ttl time.Duration) error {
	stat := r.db.Set(c, key, value, ttl)
	if err := stat.Err(); err != nil {
		return err
	}
	return nil
}
