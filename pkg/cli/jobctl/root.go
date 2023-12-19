package jobctl

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"

	"github.com/kralicky/jobserver/pkg/cli/jobctl/commands"
	"github.com/kralicky/jobserver/pkg/logger"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// rootCmd represents the base command when called without any subcommands
func BuildRootCmd() *cobra.Command {
	var logLevel string
	var address string
	var caCert string
	var clientCert string
	var clientKey string
	cmd := &cobra.Command{
		Use:          "jobctl",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.GroupID != commands.GroupIdClientCommands {
				return nil
			}
			if logLevel != "" {
				var level slog.Level
				if err := level.UnmarshalText([]byte(logLevel)); err != nil {
					return fmt.Errorf("invalid log level %q: %w", logLevel, err)
				}
				logger.LogLevel.Set(level)
			}

			caBytes, err := os.ReadFile(caCert)
			if err != nil {
				return fmt.Errorf("failed to read server CA file: %w", err)
			}
			clientCert, err := tls.LoadX509KeyPair(clientCert, clientKey)
			if err != nil {
				return fmt.Errorf("failed to load client certificate: %w", err)
			}
			serverPool := x509.NewCertPool()
			serverPool.AppendCertsFromPEM(caBytes)

			tlsConfig := &tls.Config{
				MinVersion:   tls.VersionTLS13,
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      serverPool,
			}

			cc, err := grpc.DialContext(cmd.Context(), address,
				grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
				grpc.WithDefaultCallOptions(
					grpc.MaxCallRecvMsgSize(8*1024*1024), // 8MB
				),
			)
			if err != nil {
				return fmt.Errorf("failed to dial job server: %w", err)
			}

			cmd.SetContext(jobv1.ContextWithClient(cmd.Context(), jobv1.NewJobClient(cc)))
			return nil
		},
	}

	cmd.AddGroup(&cobra.Group{
		ID:    commands.GroupIdClientCommands,
		Title: "Client Commands:",
	})

	cmd.InitDefaultCompletionCmd()
	cmd.InitDefaultHelpCmd()

	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "debug", "log level (debug, info, warn, error)")
	cmd.PersistentFlags().StringVarP(&address, "address", "a", "127.0.0.1:9097", "address of the job server")
	cmd.PersistentFlags().StringVar(&caCert, "cacert", "", "path to the server's CA certificate")
	cmd.PersistentFlags().StringVar(&clientCert, "cert", "", "path to a client certificate")
	cmd.PersistentFlags().StringVar(&clientKey, "key", "", "path to a client key")

	cmd.MarkFlagRequired("address")
	cmd.MarkFlagRequired("cacert")
	cmd.MarkFlagRequired("cert")
	cmd.MarkFlagRequired("key")

	cmd.AddCommand(
		commands.BuildJobRunCmd(),
		commands.BuildJobStopCmd(),
		commands.BuildJobStatusCmd(),
		commands.BuildJobListCmd(),
		commands.BuildJobLogsCmd(),
	)

	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ctx := context.Background()
	rootCmd := BuildRootCmd()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
