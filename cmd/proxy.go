package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/fmotalleb/go-tools/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/proxy"
	"github.com/fmotalleb/hermes/registry"
	"github.com/fmotalleb/hermes/runtime"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the transparent proxy (HTTP + TLS/SNI)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		app, err := runtime.New(ctx, registry.ServiceKindProxy)
		if err != nil {
			return err
		}
		defer app.Close(context.Background())

		bus, err := newPubSubBus(app.Config, app)
		if err != nil {
			return err
		}
		defer bus.Close()

		_, logger := log.AsNamedChild(app.Context(), "proxy")
		p, err := proxy.NewProxy(
			app.Config.ProxyListenAddr,
			app.Config.ProxyServerHTTPPorts,
			app.Config.ProxyServerTLSPorts,
			app.Config.ProxyTimeout,
			app.Config.ProxyURL,
			cache.NewMemoryCache(ctx, cache.MemCacheOption{MaxSize: 10_000}),
			app.DB,
			bus,
			logger,
		)
		if err != nil {
			return err
		}

		eg, ctx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			return app.StartMetricsServer(ctx, app.MetricsHandler)
		})
		eg.Go(func() error {
			// app.Context() carries the context logger so the proxy's routers
			// can derive their named children from it.
			return p.Serve(app.Context())
		})
		return eg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(proxyCmd)
}
