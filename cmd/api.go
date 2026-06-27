package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/sync/errgroup"

	"github.com/fmotalleb/hermes/api"
	"github.com/fmotalleb/hermes/auth"
	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/migrations"
	"github.com/fmotalleb/hermes/registry"
	"github.com/fmotalleb/hermes/runtime"
	"github.com/fmotalleb/hermes/static"
	"github.com/fmotalleb/hermes/web"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Run the HTTP admin service",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		app, err := runtime.New(ctx, registry.ServiceKindAPI, logger())
		if err != nil {
			return err
		}
		defer app.Close(context.Background())

		bus, err := newPubSubBus(app.Config, app)
		if err != nil {
			return err
		}
		defer bus.Close()

		router := web.NewRouter()
		router.Use(auth.Middleware(app.Config))

		// TODO: read from env
		router.AllowCors(
			web.WithOrigins("*"),
		)
		api.Register(
			router,
			app,
			cache.NewRedisCache(app.Redis, "api"),
			bus,
			migrations.NewRunner(app.DB, app.Logger),
			app.MetricsHandler,
		)
		static.Register(router)
		if err := execPreRun(ctx, app); err != nil {
			return err
		}
		eg, ctx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			return app.StartMetricsServer(ctx, app.MetricsHandler)
		})
		eg.Go(func() error {
			return app.StartHTTPServer(ctx, otelhttp.NewHandler(router, "http"))
		})
		return eg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.Flags().Bool("migrate", false, "Run migrations when starting api server")
}
