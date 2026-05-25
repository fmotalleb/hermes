package cache

import (
	"context"
	"time"
)

type Cache interface {
	GetBytes(context.Context, string) ([]byte, error)
	Set(context.Context, string, any, time.Duration) error
	Clear(context.Context) error
	Delete(context.Context, string) error
	DeleteSelector(context.Context, func(string) bool) error
	DeletePattern(ctx context.Context, pattern string) error
}
