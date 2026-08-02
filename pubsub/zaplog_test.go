package pubsub

import (
	"errors"
	"strings"
	"testing"

	"github.com/ThreeDotsLabs/watermill"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type captureWriter struct{ buf strings.Builder }

func (w *captureWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (*captureWriter) Sync() error                   { return nil }

func newTestAdapter() (watermill.LoggerAdapter, *captureWriter) {
	var w captureWriter
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(&w),
		zapcore.DebugLevel,
	))
	return NewZapLoggerAdapter(logger), &w
}

func TestZapLoggerAdapterRoutesToZap(t *testing.T) {
	adapter, w := newTestAdapter()

	adapter.Info("hello", watermill.LogFields{"key": "value"})
	adapter = adapter.With(watermill.LogFields{"scope": "test"})
	adapter.Error("boom", errors.New("kaput"), watermill.LogFields{"id": "1"})
	adapter.Trace("trace msg", nil)

	out := w.buf.String()
	for _, want := range []string{"hello", "key", "value", "boom", "kaput", "scope", "test", "id", "trace msg"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected log output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestZapLoggerAdapterNilLogger(t *testing.T) {
	adapter := NewZapLoggerAdapter(nil)
	adapter.Info("silent", nil)
	adapter.Error("silent", errors.New("x"), nil)
	// No panic above is the assertion.
}

func TestZapLoggerAdapterWithPreservesFields(t *testing.T) {
	adapter, w := newTestAdapter()
	adapter = adapter.With(watermill.LogFields{"a": "1"}).With(watermill.LogFields{"b": "2"})
	adapter.Info("merged", watermill.LogFields{"c": "3"})

	out := w.buf.String()
	for _, want := range []string{"a", "1", "b", "2", "c", "3"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected merged log output to contain %q, got:\n%s", want, out)
		}
	}
}
