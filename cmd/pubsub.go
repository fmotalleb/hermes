package cmd

import (
	"fmt"
	"strings"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/fmotalleb/hermes/internal/pubsub"
	"github.com/fmotalleb/hermes/internal/runtime"
)

func newPubSubBus(cfg runtime.Config, app *runtime.App) (pubsub.Bus, error) {
	logger := watermill.NewSlogLogger(app.Logger)
	group := cfg.PubSubConsumerGroup + "-" + app.ID().String()
	switch strings.ToLower(strings.TrimSpace(cfg.PubSubBackend)) {
	case "", "gochannel", "memory", "local":
		return pubsub.NewGoChannel(logger), nil
	case "redis", "redisstream":
		return pubsub.NewRedisStream(app.Redis, group, logger)
	case "kafka":
		brokers := splitNonEmpty(cfg.PubSubBrokers)
		return pubsub.NewKafka(brokers, group, logger)
	case "postgres", "sql":
		return pubsub.NewPostgres(app.DB, group, logger)
	case "rabbitmq", "amqp":
		return pubsub.NewRabbitMQ(cfg.PubSubRabbitMQURI, group, logger)
	default:
		return nil, fmt.Errorf("unknown pubsub backend: %s", cfg.PubSubBackend)
	}
}

func splitNonEmpty(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
