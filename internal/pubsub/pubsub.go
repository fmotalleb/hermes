package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
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

func NewGoChannel() Bus {
	channel := gochannel.NewGoChannel(gochannel.Config{}, watermill.NewStdLogger(false, false))
	return &watermillBus{
		publisher:  channel,
		subscriber: channel,
		closers:    []interface{ Close() error }{channel},
	}
}

func NewRedisStream(client redis.UniversalClient, consumerGroup string) (Bus, error) {
	if client == nil {
		return nil, errors.New("redis client is required")
	}

	publisher, err := redisstream.NewPublisher(
		redisstream.PublisherConfig{
			Client: client,
		},
		watermill.NewStdLogger(false, false),
	)
	if err != nil {
		return nil, fmt.Errorf("create redisstream publisher: %w", err)
	}

	subscriberConfig := redisstream.SubscriberConfig{
		Client: client,
	}
	if consumerGroup != "" {
		subscriberConfig.ConsumerGroup = consumerGroup
	}

	subscriber, err := redisstream.NewSubscriber(
		subscriberConfig,
		watermill.NewStdLogger(false, false),
	)
	if err != nil {
		_ = publisher.Close()
		return nil, fmt.Errorf("create redisstream subscriber: %w", err)
	}

	return &watermillBus{
		publisher:  publisher,
		subscriber: subscriber,
		closers:    []interface{ Close() error }{publisher, subscriber},
	}, nil
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
