// Package cache provides a generic caching interface with multiple backends:
// Redis, in-memory, and a no-op implementation. It supports key-based CRUD,
// pattern-based deletion, and TTL expiration.
package cache

import (
	"context"
	"time"
)

// Cache defines the interface for a generic key-value cache with TTL support.
// Implementations must support key-based CRUD, pattern-based deletion,
// and full cache invalidation.
type Cache interface {
	GetBytes(context.Context, string) ([]byte, error)
	Set(context.Context, string, any, time.Duration) error
	Clear(context.Context) error
	Delete(context.Context, string) error
	DeleteSelector(context.Context, func(string) bool) error
	DeletePattern(ctx context.Context, pattern string) error
}
