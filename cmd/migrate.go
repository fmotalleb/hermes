package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fmotalleb/hermes/migrations"
	"github.com/fmotalleb/hermes/registry"
	"github.com/fmotalleb/hermes/runtime"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply database migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		app, err := runtime.New(ctx, registry.ServiceKindMigrator)
		if err != nil {
			return err
		}
		defer app.Close(context.Background())

		return migrations.Apply(app.Context(), app.DB, migrations.All()...)
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
