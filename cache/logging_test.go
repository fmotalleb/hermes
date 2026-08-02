package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fmotalleb/go-tools/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func testObservedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	return zap.New(core), logs
}

func TestMemoryCacheLogsHitAndMiss(t *testing.T) {
	logger, logs := testObservedLogger()
	ctx := log.WithLogger(context.Background(), logger)
	c := NewMemoryCache(ctx)

	if _, err := c.GetBytes(ctx, "nope"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}
	if got := len(logs.FilterMessage("cache miss").All()); got != 1 {
		t.Fatalf("expected 1 cache miss log, got %d", got)
	}

	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	data, err := c.GetBytes(ctx, "k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(data) != "v" {
		t.Fatalf("expected value v, got %q", data)
	}
	if got := len(logs.FilterMessage("cache hit").All()); got != 1 {
		t.Fatalf("expected 1 cache hit log, got %d", got)
	}
}

func TestMemoryCacheLogsSetFailure(t *testing.T) {
	logger, logs := testObservedLogger()
	ctx := log.WithLogger(context.Background(), logger)
	c := NewMemoryCache(ctx)

	// The memory cache only accepts []byte values; storing anything else must
	// fail and be logged at warn level.
	if err := c.Set(ctx, "k", "not-bytes", time.Minute); !errors.Is(err, ErrStoreUnsupportedType) {
		t.Fatalf("expected ErrStoreUnsupportedType, got %v", err)
	}
	if got := len(logs.FilterMessage("cache set failed").All()); got != 1 {
		t.Fatalf("expected 1 cache set failed log, got %d", got)
	}
	if got := logs.FilterMessage("cache set failed").All()[0].Level; got != zapcore.WarnLevel {
		t.Fatalf("expected warn level, got %v", got)
	}
}

func TestNoneCacheSkipsLogging(t *testing.T) {
	logger, logs := testObservedLogger()
	ctx := log.WithLogger(context.Background(), logger)
	c := NewNoneCache()

	if _, err := c.GetBytes(ctx, "k"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}
	if got := len(logs.All()); got != 0 {
		t.Fatalf("expected no logs from none cache, got %d", got)
	}
}
