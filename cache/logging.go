package cache

import (
	"context"
	"errors"
	"time"

	"github.com/fmotalleb/go-tools/log"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// loggingCache decorates a Cache with diagnostics. Hits and misses are logged
// at debug level; unexpected failures (Redis down, unsupported values, ...) at
// warn level for writes and debug for reads. The logger is derived from the
// context as a "cache" child (see log.AsNamedChild), so it follows the
// request-scoped logger when available.
type loggingCache struct {
	inner Cache
}

func withLogging(inner Cache) Cache {
	return &loggingCache{inner: inner}
}

// isCacheMiss reports whether err represents a missing key regardless of the
// backend (ErrCacheMiss for in-memory and no-op caches, redis.Nil for Redis).
func isCacheMiss(err error) bool {
	return errors.Is(err, ErrCacheMiss) || errors.Is(err, redis.Nil)
}

func (c *loggingCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	ctx, logger := log.AsNamedChild(ctx, "cache")
	data, err := c.inner.GetBytes(ctx, key)
	switch {
	case err == nil:
		logger.Debug("cache hit", zap.String("key", key))
	case isCacheMiss(err):
		logger.Debug("cache miss", zap.String("key", key))
	default:
		logger.Debug("cache get failed", zap.String("key", key), zap.Error(err))
	}
	return data, err
}

func (c *loggingCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	ctx, logger := log.AsNamedChild(ctx, "cache")
	if err := c.inner.Set(ctx, key, value, ttl); err != nil {
		logger.Warn("cache set failed", zap.String("key", key), zap.Error(err))
		return err
	}
	logger.Debug("cache set", zap.String("key", key), zap.Duration("ttl", ttl))
	return nil
}

func (c *loggingCache) Clear(ctx context.Context) error {
	ctx, logger := log.AsNamedChild(ctx, "cache")
	if err := c.inner.Clear(ctx); err != nil {
		logger.Warn("cache clear failed", zap.Error(err))
		return err
	}
	logger.Debug("cache cleared")
	return nil
}

func (c *loggingCache) Delete(ctx context.Context, key string) error {
	ctx, logger := log.AsNamedChild(ctx, "cache")
	if err := c.inner.Delete(ctx, key); err != nil {
		logger.Warn("cache delete failed", zap.String("key", key), zap.Error(err))
		return err
	}
	logger.Debug("cache deleted", zap.String("key", key))
	return nil
}

func (c *loggingCache) DeleteSelector(ctx context.Context, selector func(string) bool) error {
	ctx, logger := log.AsNamedChild(ctx, "cache")
	if err := c.inner.DeleteSelector(ctx, selector); err != nil {
		logger.Warn("cache selector delete failed", zap.Error(err))
		return err
	}
	logger.Debug("cache selector delete completed")
	return nil
}

func (c *loggingCache) DeletePattern(ctx context.Context, pattern string) error {
	ctx, logger := log.AsNamedChild(ctx, "cache")
	if err := c.inner.DeletePattern(ctx, pattern); err != nil {
		logger.Warn("cache pattern delete failed", zap.String("pattern", pattern), zap.Error(err))
		return err
	}
	logger.Debug("cache pattern delete completed", zap.String("pattern", pattern))
	return nil
}
