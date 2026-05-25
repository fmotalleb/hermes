package cache

import (
	"context"
	"errors"
	"time"

	"gofr.dev/pkg/gofr/container"
)

type redisCache struct {
	db      container.Redis
	baseKey string
}

func NewRedisCache(db container.Redis, baseKey string) Cache {
	return &redisCache{
		db,
		baseKey,
	}
}

func (r *redisCache) key(k string) string {
	return r.baseKey + ":" + k
}

func (r *redisCache) GetBytes(c context.Context, key string) ([]byte, error) {
	stat := r.db.Get(c, r.key(key))
	if err := stat.Err(); err != nil {
		return nil, err
	}
	return stat.Bytes()
}

func (r *redisCache) GetUint64(c context.Context, key string) (uint64, error) {
	stat := r.db.Get(c, r.key(key))
	if err := stat.Err(); err != nil {
		return 0, err
	}
	return stat.Uint64()
}

func (r *redisCache) Set(c context.Context, key string, value any, ttl time.Duration) error {
	stat := r.db.Set(c, r.key(key), value, ttl)
	if err := stat.Err(); err != nil {
		return err
	}
	return nil
}

func (r *redisCache) Clear(c context.Context) error {
	if r.baseKey == "" {
		return errors.New("empty redis cache baseKey")
	}

	var cursor uint64

	pattern := r.baseKey + ":*"

	for {
		keys, nextCursor, err := r.db.Scan(c, cursor, pattern, 1000).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := r.db.Unlink(c, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = nextCursor

		if cursor == 0 {
			break
		}
	}

	return nil
}
