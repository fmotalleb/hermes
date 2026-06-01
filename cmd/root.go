package cmd

import (
	"log/slog"
	"os"
	"strings"

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
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func logger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(logLevel)) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func init() {
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "set log level of the application")
}
