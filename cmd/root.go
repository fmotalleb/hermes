package cmd

import (
	"context"
	"os"

	"github.com/fmotalleb/go-tools/log"
	"github.com/spf13/cobra"
)

var logLevel = "info"

var rootCmd = &cobra.Command{
	Use:   "hermes",
	Short: "Hermes DNS admin tooling",

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		lvl, err := cmd.Flags().GetString("log-level")
		if err != nil || lvl == "" {
			return
		}
		logLevel = lvl
		preRunFromArgs(cmd)
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
