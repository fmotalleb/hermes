package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/fmotalleb/hermes/api"
	"github.com/fmotalleb/hermes/auth"
	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/internal/pubsub"
	"github.com/fmotalleb/hermes/internal/runtime"
	"github.com/fmotalleb/hermes/internal/web"
	"github.com/fmotalleb/hermes/migrations"
	"github.com/fmotalleb/hermes/static"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Run the HTTP admin service",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		app, err := runtime.New(ctx, logger())
		if err != nil {
			return err
		}
		defer app.Close(context.Background())

		router := web.NewRouter()
		router.Use(auth.Middleware(app.Config))

		bus := pubsub.New(app.Redis)
		api.Register(
			router,
			app.DB,
			cache.NewRedisCache(app.Redis, "hermes"),
			bus,
			migrations.NewRunner(app.DB),
			app.MetricsHandler,
		)
		static.Register(router)

		go func() {
			_ = app.StartMetricsServer(ctx, app.MetricsHandler)
		}()

		return app.StartHTTPServer(ctx, otelhttp.NewHandler(router, "http"))
	},
}

func init() {
	rootCmd.AddCommand(apiCmd)
}
