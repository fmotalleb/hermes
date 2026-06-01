package cmd

import (
	"fmt"
	"strings"

	"github.com/fmotalleb/hermes/internal/pubsub"
	"github.com/fmotalleb/hermes/internal/runtime"
)

func newPubSubBus(cfg runtime.Config, app *runtime.App) (pubsub.Bus, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.PubSubBackend)) {
	case "", "gochannel", "memory", "local":
		return pubsub.NewGoChannel(), nil
	case "redis", "redisstream":
		return pubsub.NewRedisStream(app.Redis, "")
	default:
		return nil, fmt.Errorf("unknown pubsub backend: %s", cfg.PubSubBackend)
	}
}
