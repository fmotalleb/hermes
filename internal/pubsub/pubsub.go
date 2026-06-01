package pubsub

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Handler func(context.Context, []byte) error

type Bus struct {
	client *redis.Client
}

func New(client *redis.Client) *Bus {
	return &Bus{client: client}
}

func (b *Bus) Publish(ctx context.Context, topic string, payload []byte) error {
	if b == nil || b.client == nil {
		return errors.New("pubsub is not configured")
	}
	return b.client.Publish(ctx, topic, payload).Err()
}

func (b *Bus) Subscribe(ctx context.Context, topic string, handler Handler) error {
	if b == nil || b.client == nil {
		return errors.New("pubsub is not configured")
	}

	subscription := b.client.Subscribe(ctx, topic)
	defer subscription.Close()

	channel := subscription.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-channel:
			if !ok {
				return nil
			}
			if handler == nil {
				continue
			}
			if err := handler(ctx, []byte(msg.Payload)); err != nil {
				return fmt.Errorf("handle pubsub message: %w", err)
			}
		}
	}
}

