package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

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

		return dns.Serve(ctx, app,
			dns.WithListenAddr(app.Config.DNSListenAddr),
			dns.WithProtocol(dns.ProtocolUDP),
		)
	},
}

func init() {
	rootCmd.AddCommand(dnsCmd)
}
