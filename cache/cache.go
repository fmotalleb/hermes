package cache

import (
	"context"
	"time"
)

type Cache interface {
	GetBytes(context.Context, string) ([]byte, error)
	GetUint64(context.Context, string) (uint64, error)
	Set(context.Context, string, any, time.Duration) error
	Clear(context.Context) error
}
