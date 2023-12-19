package jobserver

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/kralicky/jobserver/pkg/cli/jobserver/commands"
	"github.com/kralicky/jobserver/pkg/logger"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
func BuildRootCmd() *cobra.Command {
	var logLevel string
	rootCmd := &cobra.Command{
		Use:          "jobserver",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if logLevel != "" {
				var level slog.Level
				if err := level.UnmarshalText([]byte(logLevel)); err != nil {
					return fmt.Errorf("invalid log level %q: %w", logLevel, err)
				} else {
					logger.LogLevel.Set(level)
				}
			}
			return nil
		},
	}

	rootCmd.AddCommand(commands.BuildServeCmd())

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "debug", "log level (debug, info, warn, error)")

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := BuildRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
