package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/fmotalleb/hermes/dns"
	"github.com/fmotalleb/hermes/registry"
	"github.com/fmotalleb/hermes/runtime"
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Run the DNS server",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		app, err := runtime.New(ctx, registry.ServiceKindDNS, logger())
		if err != nil {
			return err
		}
		defer app.Close(context.Background())

		bus, err := newPubSubBus(app.Config, app)
		if err != nil {
			return err
		}
		defer bus.Close()

		dnsOpts := make([]dns.ServerOption, 0)
		dnsOpts = append(dnsOpts, dns.WithListenAddr(app.Config.DNSListenAddr))
		if proto, err := dns.ProtocolFromStr(app.Config.DNSProtocol); err != nil {
			return err
		} else {
			dnsOpts = append(dnsOpts, dns.WithProtocol(proto))
		}
		if app.Config.DNSTLSCertificateFile != "" && app.Config.DNSTLSPrivateKeyFile != "" {
			dnsOpts = append(
				dnsOpts,
				dns.WithTLSFiles(
					app.Config.DNSTLSCertificateFile,
					app.Config.DNSTLSPrivateKeyFile,
				),
			)
		}

		eg, ctx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			return app.StartMetricsServer(ctx, app.MetricsHandler)
		})
		eg.Go(func() error {
			return dns.Serve(app.Context(), app, bus, dnsOpts...)
		})
		return eg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(dnsCmd)
}
