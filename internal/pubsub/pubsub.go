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
}

func newBus(publisher message.Publisher, subscriber message.Subscriber, closers ...interface{ Close() error }) Bus {
	return &watermillBus{
		publisher:  publisher,
		subscriber: subscriber,
		closers:    closers,
	}
}

func NewGoChannel(logger watermill.LoggerAdapter) Bus {
	if logger == nil {
		logger = watermill.NopLogger{}
	}
	channel := gochannel.NewGoChannel(gochannel.Config{}, logger)
	return newBus(channel, channel, channel)
}

func NewRedisStream(client redis.UniversalClient, consumerGroup string, logger watermill.LoggerAdapter) (Bus, error) {
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
		FanOutOldestId: "$", // Discard older events
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

	return newBus(publisher, subscriber, publisher, subscriber), nil
}

func NewKafka(brokers []string, consumerGroup string, logger watermill.LoggerAdapter) (Bus, error) {
	if len(brokers) == 0 {
		return nil, errors.New("missing kafka brokers")
	}
	if logger == nil {
		logger = watermill.NopLogger{}
	}

	publisher, err := kafka.NewPublisher(
		kafka.PublisherConfig{
			Brokers: brokers,
		},
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

	return newBus(publisher, subscriber, publisher, subscriber), nil
}

func NewPostgres(db *sql.DB, consumerGroup string, logger watermill.LoggerAdapter) (Bus, error) {
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

	return newBus(publisher, subscriber, publisher, subscriber), nil
}

func NewRabbitMQ(uri, consumerGroup string, logger watermill.LoggerAdapter) (Bus, error) {
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

	return newBus(publisher, subscriber, publisher, subscriber), nil
}

func (b *watermillBus) Publish(ctx context.Context, topic string, payload []byte) error {
	if b == nil || b.publisher == nil {
		return errors.New("pubsub is not configured")
	}

	msg := message.NewMessage(watermill.NewUUID(), payload)
	msg.SetContext(ctx)

	return b.publisher.Publish(topic, msg)
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
			if err := handler(msg.Context(), msg.Payload); err != nil {
				msg.Nack()
				return fmt.Errorf("handle pubsub message: %w", err)
			}
			msg.Ack()
		}
	}
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
