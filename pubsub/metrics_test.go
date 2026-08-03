package pubsub

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func counterValueByAttrs(t *testing.T, rm *metricdata.ResourceMetrics, name string, want map[string]string) int64 {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("expected Sum[int64] for %s, got %T", name, m.Data)
			}
			for _, dp := range sum.DataPoints {
				if attrsContain(dp.Attributes.ToSlice(), want) {
					return dp.Value
				}
			}
		}
	}
	return 0
}

func attrsContain(attrs []attribute.KeyValue, want map[string]string) bool {
	for k, v := range want {
		found := false
		for _, a := range attrs {
			if string(a.Key) == k && a.Value.AsString() == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestBusPublishAndProcessCounters(t *testing.T) {
	prev := otel.GetMeterProvider()
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(prev)

	bus := NewGoChannel(watermill.NopLogger{})
	defer bus.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	processed := make(chan struct{})
	go func() {
		_ = bus.Subscribe(ctx, "test-topic", func(_ context.Context, _ []byte) error {
			close(processed)
			return nil
		})
	}()

	// Wait for the subscriber to be registered before publishing; gochannel
	// drops messages published to topics with no subscribers.
	time.Sleep(100 * time.Millisecond)

	if err := bus.Publish(context.Background(), "test-topic", []byte("hi")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-processed:
	case <-time.After(2 * time.Second):
		t.Fatal("message was not processed within timeout")
	}

	rm := &metricdata.ResourceMetrics{}
	if err := reader.Collect(context.Background(), rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	if got := counterValueByAttrs(t, rm, "messaging.publish.messages", map[string]string{"messaging.destination.name": "test-topic"}); got != 1 {
		t.Errorf("expected 1 published message, got %d", got)
	}
	if got := counterValueByAttrs(t, rm, "messaging.process.messages", map[string]string{"messaging.destination.name": "test-topic", "result": "success"}); got != 1 {
		t.Errorf("expected 1 processed message, got %d", got)
	}
}

// TestBusProcessErrorCounter verifies a failing handler is counted with
// result=error.
func TestBusProcessErrorCounter(t *testing.T) {
	prev := otel.GetMeterProvider()
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(prev)

	bus := NewGoChannel(watermill.NopLogger{})
	defer bus.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	processed := make(chan struct{})
	go func() {
		_ = bus.Subscribe(ctx, "bad-topic", func(_ context.Context, _ []byte) error {
			close(processed)
			return errors.New("handler failed")
		})
	}()

	time.Sleep(100 * time.Millisecond)

	if err := bus.Publish(context.Background(), "bad-topic", []byte("hi")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-processed:
	case <-time.After(2 * time.Second):
		t.Fatal("message was not processed within timeout")
	}

	rm := &metricdata.ResourceMetrics{}
	if err := reader.Collect(context.Background(), rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	if got := counterValueByAttrs(t, rm, "messaging.process.messages", map[string]string{"messaging.destination.name": "bad-topic", "result": "error"}); got != 1 {
		t.Errorf("expected 1 failed processed message, got %d", got)
	}
}
