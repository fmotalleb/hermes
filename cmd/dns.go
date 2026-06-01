package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/fmotalleb/hermes/dns"
	"github.com/fmotalleb/hermes/internal/runtime"
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Run the DNS server",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		app, err := runtime.New(ctx, logger())
		if err != nil {
			return err
		}
		defer app.Close(context.Background())

		bus, err := newPubSubBus(app.Config, app)
		if err != nil {
			return err
		}
		defer bus.Close()
		eg, ctx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			return app.StartMetricsServer(ctx, app.MetricsHandler)
		})
		eg.Go(func() error {
			return dns.Serve(ctx, app, bus,
				dns.WithListenAddr(app.Config.DNSListenAddr),
				dns.WithProtocol(dns.ProtocolUDP),
			)
		})
		return eg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(dnsCmd)
}
