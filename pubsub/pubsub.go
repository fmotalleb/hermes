package pubsub

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/ThreeDotsLabs/watermill"
	amqp "github.com/ThreeDotsLabs/watermill-amqp/v3/pkg/amqp"
	"github.com/ThreeDotsLabs/watermill-kafka/v3/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	sqlpub "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "pubsub"
)

type Handler func(context.Context, []byte) error

type Bus interface {
	Publish(context.Context, string, []byte) error
	Subscribe(context.Context, string, Handler) error
	Close() error
}

type watermillBus struct {
	publisher  message.Publisher
	subscriber message.Subscriber
	closers    []interface{ Close() error }
	once       sync.Once
	closeErr   error
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

type BusOption func(*watermillBus)

func WithTracer(tracer trace.Tracer) BusOption {
	return func(b *watermillBus) { b.tracer = tracer }
}

func WithPropagator(propagator propagation.TextMapPropagator) BusOption {
	return func(b *watermillBus) { b.propagator = propagator }
}

func newBus(publisher message.Publisher, subscriber message.Subscriber, closers []interface{ Close() error }, opts ...BusOption) Bus {
	b := &watermillBus{
		publisher:  publisher,
		subscriber: subscriber,
		closers:    closers,
		tracer:     otel.Tracer(tracerName),
		propagator: otel.GetTextMapPropagator(),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func NewGoChannel(logger watermill.LoggerAdapter, opts ...BusOption) Bus {
	if logger == nil {
		logger = watermill.NopLogger{}
	}
	channel := gochannel.NewGoChannel(gochannel.Config{}, logger)
	return newBus(channel, channel, []interface{ Close() error }{channel}, opts...)
}

func NewRedisStream(client redis.UniversalClient, consumerGroup string, logger watermill.LoggerAdapter, opts ...BusOption) (Bus, error) {
	if client == nil {
		return nil, errors.New("redis client is required")
	}
	if logger == nil {
		logger = watermill.NopLogger{}
	}

	publisher, err := redisstream.NewPublisher(
		redisstream.PublisherConfig{Client: client},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("create redisstream publisher: %w", err)
	}

	subscriberConfig := redisstream.SubscriberConfig{
		Client:         client,
		FanOutOldestId: "$",
		OldestId:       "$",
	}
	if consumerGroup != "" {
		subscriberConfig.ConsumerGroup = consumerGroup
	}
	subscriber, err := redisstream.NewSubscriber(subscriberConfig, logger)
	if err != nil {
		_ = publisher.Close()
		return nil, fmt.Errorf("create redisstream subscriber: %w", err)
	}

	return newBus(publisher, subscriber, []interface{ Close() error }{publisher, subscriber}, opts...), nil
}

func NewKafka(brokers []string, consumerGroup string, logger watermill.LoggerAdapter, opts ...BusOption) (Bus, error) {
	if len(brokers) == 0 {
		return nil, errors.New("missing kafka brokers")
	}
	if logger == nil {
		logger = watermill.NopLogger{}
	}

	publisher, err := kafka.NewPublisher(
		kafka.PublisherConfig{Brokers: brokers},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka publisher: %w", err)
	}

	subscriber, err := kafka.NewSubscriber(
		kafka.SubscriberConfig{
			Brokers:       brokers,
			ConsumerGroup: consumerGroup,
		},
		logger,
	)
	if err != nil {
		_ = publisher.Close()
		return nil, fmt.Errorf("create kafka subscriber: %w", err)
	}

	return newBus(publisher, subscriber, []interface{ Close() error }{publisher, subscriber}, opts...), nil
}

func NewPostgres(db *sql.DB, consumerGroup string, logger watermill.LoggerAdapter, opts ...BusOption) (Bus, error) {
	if db == nil {
		return nil, errors.New("postgres database is required")
	}
	if logger == nil {
		logger = watermill.NopLogger{}
	}
	if consumerGroup == "" {
		consumerGroup = "hermes"
	}

	publisher, err := sqlpub.NewPublisher(
		sqlpub.BeginnerFromStdSQL(db),
		sqlpub.PublisherConfig{
			SchemaAdapter: sqlpub.DefaultPostgreSQLSchema{},
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("create postgres publisher: %w", err)
	}

	subscriber, err := sqlpub.NewSubscriber(
		sqlpub.BeginnerFromStdSQL(db),
		sqlpub.SubscriberConfig{
			ConsumerGroup:    consumerGroup,
			SchemaAdapter:    sqlpub.DefaultPostgreSQLSchema{},
			OffsetsAdapter:   sqlpub.DefaultPostgreSQLOffsetsAdapter{},
			InitializeSchema: true,
		},
		logger,
	)
	if err != nil {
		_ = publisher.Close()
		return nil, fmt.Errorf("create postgres subscriber: %w", err)
	}

	return newBus(publisher, subscriber, []interface{ Close() error }{publisher, subscriber}, opts...), nil
}

func NewRabbitMQ(uri, consumerGroup string, logger watermill.LoggerAdapter, opts ...BusOption) (Bus, error) {
	if strings.TrimSpace(uri) == "" {
		return nil, errors.New("missing rabbitmq uri")
	}
	if logger == nil {
		logger = watermill.NopLogger{}
	}
	if consumerGroup == "" {
		consumerGroup = "hermes"
	}
	config := amqp.NewDurablePubSubConfig(
		uri,
		amqp.GenerateQueueNameTopicNameWithSuffix(consumerGroup),
	)

	publisher, err := amqp.NewPublisher(config, logger)
	if err != nil {
		return nil, fmt.Errorf("create rabbitmq publisher: %w", err)
	}

	subscriber, err := amqp.NewSubscriber(config, logger)
	if err != nil {
		_ = publisher.Close()
		return nil, fmt.Errorf("create rabbitmq subscriber: %w", err)
	}

	return newBus(publisher, subscriber, []interface{ Close() error }{publisher, subscriber}, opts...), nil
}

type messageCarrier struct {
	msg *message.Message
}

func (c messageCarrier) Get(key string) string {
	return c.msg.Metadata.Get(key)
}

func (c messageCarrier) Set(key, value string) {
	c.msg.Metadata.Set(key, value)
}

func (c messageCarrier) Keys() []string {
	keys := make([]string, 0, len(c.msg.Metadata))
	for k := range c.msg.Metadata {
		keys = append(keys, k)
	}
	return keys
}

func (b *watermillBus) Publish(ctx context.Context, topic string, payload []byte) error {
	if b == nil || b.publisher == nil {
		return errors.New("pubsub is not configured")
	}

	ctx, span := b.tracer.Start(ctx, topic+" publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			semconv.MessagingSystemKey.String("watermill"),
			semconv.MessagingDestinationName(topic),
			semconv.MessagingOperationTypePublish,
			attribute.Int("messaging.message.body.size", len(payload)),
		),
	)
	defer span.End()

	msg := message.NewMessage(watermill.NewUUID(), payload)
	msg.SetContext(ctx)

	// Inject trace context into message metadata for propagation
	b.propagator.Inject(ctx, messageCarrier{msg: msg})

	span.SetAttributes(attribute.String("messaging.message.id", msg.UUID))

	if err := b.publisher.Publish(topic, msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (b *watermillBus) Subscribe(ctx context.Context, topic string, handler Handler) error {
	if b == nil || b.subscriber == nil {
		return errors.New("pubsub is not configured")
	}

	messages, err := b.subscriber.Subscribe(ctx, topic)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-messages:
			if !ok {
				return nil
			}
			if handler == nil {
				msg.Ack()
				continue
			}

			if err := b.handleMessage(ctx, topic, msg, handler); err != nil {
				return fmt.Errorf("handle pubsub message: %w", err)
			}
		}
	}
}

func (b *watermillBus) handleMessage(ctx context.Context, topic string, msg *message.Message, handler Handler) error {
	// Extract trace context from message metadata
	msgCtx := b.propagator.Extract(msg.Context(), messageCarrier{msg: msg})

	msgCtx, span := b.tracer.Start(msgCtx, topic+" process",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			semconv.MessagingSystemKey.String("watermill"),
			semconv.MessagingDestinationName(topic),
			semconv.MessagingOperationTypeReceive,
			attribute.String("messaging.message.id", msg.UUID),
			attribute.Int("messaging.message.body.size", len(msg.Payload)),
		),
	)
	defer span.End()

	if err := handler(msgCtx, msg.Payload); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		msg.Nack()
		return err
	}
	span.SetAttributes(semconv.MessagingOperationTypeSettle)
	span.SetStatus(codes.Ok, "message processed successfully")
	msg.Ack()
	return nil
}

func (b *watermillBus) Close() error {
	if b == nil {
		return nil
	}

	b.once.Do(func() {
		for _, closer := range b.closers {
			if closer == nil {
				continue
			}
			if err := closer.Close(); err != nil {
				b.closeErr = errors.Join(b.closeErr, err)
			}
		}
	})

	return b.closeErr
}
