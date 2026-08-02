package cmd

import (
	"context"
	"os"

	"github.com/fmotalleb/go-tools/log"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hermes",
	Short: "Hermes DNS admin tooling",

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		preRunFromArgs(cmd)
		lvl, err := cmd.Flags().GetString("log-level")
		if err != nil || !cmd.Flags().Changed("log-level") {
			return
		}
		// The base logger is built from the environment in Execute(), before
		// flags are parsed. When -l is explicitly passed, rebuild it with the
		// requested level (preserving the rest of the env configuration) and
		// re-attach it to the context so every subcommand — and the OTLP tee
		// added in runtime.New — sees it.
		ctx, err := log.WithNewLogger(cmd.Context(), func(b *log.Builder) *log.Builder {
			return b.FromEnv().Level(lvl)
		})
		if err != nil {
			return
		}
		cmd.SetContext(ctx)
	},
}

func Execute() {
	ctx := context.Background()
	ctx, err := log.WithNewEnvLogger(ctx)
	if err != nil {
		panic(err)
	}
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "set log level of the application")
	registerPreRunFlags(rootCmd)
}
