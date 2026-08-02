package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/fmotalleb/hermes/migrations"
	"github.com/fmotalleb/hermes/runtime"
)

const migrateFlag = "migrate"

type preRunTask func(ctx context.Context, app *runtime.App) error

var preRunTasks = make([]preRunTask, 0)

func preRunFromArgs(cmd *cobra.Command) {
	if migrate, _ := cmd.Flags().GetBool(migrateFlag); migrate {
		registerPreRun(func(_ context.Context, app *runtime.App) error {
			return migrations.Apply(app.Context(), app.DB, migrations.All()...)
		})
	}
}

func registerPreRunFlags(_ *cobra.Command) {
	pf := rootCmd.PersistentFlags()
	pf.Bool(migrateFlag, false, "run migration before running the API or DNS server")
}

func registerPreRun(t preRunTask) {
	preRunTasks = append(preRunTasks, t)
}

func execPreRun(ctx context.Context, app *runtime.App) error {
	for _, t := range preRunTasks {
		if err := t(ctx, app); err != nil {
			return err
		}
	}
	return nil
}
