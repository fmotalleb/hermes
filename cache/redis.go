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
		db:      db,
		baseKey: baseKey,
	}
}

func (r *redisCache) key(k string) string {
	return r.baseKey + ":" + k
}

func (r *redisCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return r.db.Get(ctx, r.key(key)).Bytes()
}

func (r *redisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return r.db.Set(ctx, r.key(key), value, ttl).Err()
}

func (r *redisCache) Delete(ctx context.Context, key string) error {
	return r.db.Del(ctx, r.key(key)).Err()
}

// DeleteSelector deletes all keys under baseKey whose suffix matches the predicate.
func (r *redisCache) DeleteSelector(ctx context.Context, selector func(string) bool) error {
	if r.baseKey == "" {
		return errors.New("empty redis cache baseKey")
	}

	return r.scanAndDelete(ctx, r.baseKey+":*", func(key string) bool {
		// Strip the baseKey prefix before passing to the selector.
		suffix := key[len(r.baseKey)+1:]
		return selector(suffix)
	})
}

// DeletePattern deletes all keys under baseKey whose suffix matches the given glob pattern.
func (r *redisCache) DeletePattern(ctx context.Context, pattern string) error {
	if r.baseKey == "" {
		return errors.New("empty redis cache baseKey")
	}

	return r.scanAndDelete(ctx, r.baseKey+":"+pattern, nil)
}

// Clear deletes all keys belonging to this cache's baseKey namespace.
func (r *redisCache) Clear(ctx context.Context) error {
	if r.baseKey == "" {
		return errors.New("empty redis cache baseKey")
	}

	return r.scanAndDelete(ctx, r.baseKey+":*", nil)
}

// scanAndDelete iterates over keys matching pattern and deletes those accepted by the
// optional filter. A nil filter accepts every key.
func (r *redisCache) scanAndDelete(ctx context.Context, pattern string, filter func(string) bool) error {
	const batchSize = 1000

	var cursor uint64

	for {
		keys, nextCursor, err := r.db.Scan(ctx, cursor, pattern, batchSize).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			toDelete := keys
			if filter != nil {
				toDelete = toDelete[:0]
				for _, k := range keys {
					if filter(k) {
						toDelete = append(toDelete, k)
					}
				}
			}

			if len(toDelete) > 0 {
				if err := r.db.Unlink(ctx, toDelete...).Err(); err != nil {
					return err
				}
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}
