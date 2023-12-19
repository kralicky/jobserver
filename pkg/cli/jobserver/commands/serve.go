package commands

import (
	"fmt"
	"os"

	rbacv1 "github.com/kralicky/jobserver/pkg/apis/rbac/v1"
	"github.com/kralicky/jobserver/pkg/auth"
	"github.com/kralicky/jobserver/pkg/cgroups"
	"github.com/kralicky/jobserver/pkg/jobs"
	"github.com/kralicky/jobserver/pkg/rbac"
	"github.com/kralicky/jobserver/pkg/server"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"sigs.k8s.io/yaml"
)

// ServeCmd represents the serve command
func BuildServeCmd() *cobra.Command {
	var rbacConfigFile string
	var serverConfig server.Options
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "A brief description of your command",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("server must be run as root (try 'sudo %s serve')", os.Args[0])
			}
			if rbacConfigFile != "" {
				config, err := loadRbacConfig(rbacConfigFile)
				if err != nil {
					return fmt.Errorf("failed to parse RBAC configuration: %w", err)
				}
				serverConfig.AuthMiddlewares = []auth.Middleware{
					auth.NewMiddleware(auth.NewMTLSAuthenticator()),
					rbac.NewAllowedMethodsMiddleware(config),
				}
			}
			runtimeId, err := cgroups.DetectFilesystemRuntime()
			if err != nil {
				return err
			}
			builder, ok := jobs.LookupRuntime(runtimeId)
			if !ok {
				return fmt.Errorf("no runtime found for %q", runtimeId)
			}
			rt, err := builder()
			if err != nil {
				return fmt.Errorf("failed to initialize runtime: %w", err)
			}
			srv := server.NewServer(rt, serverConfig)
			return srv.ListenAndServe(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&serverConfig.ListenAddress, "listen-address", "a", "127.0.0.1:9097", "address to listen on")
	cmd.Flags().StringVar(&rbacConfigFile, "rbac", "", "path to a configuration file containing rbac rules")
	cmd.Flags().StringVar(&serverConfig.CaCertFile, "cacert", "", "path to the CA certificate")
	cmd.Flags().StringVar(&serverConfig.CertFile, "cert", "", "path to the server certificate")
	cmd.Flags().StringVar(&serverConfig.KeyFile, "key", "", "path to the server key")
	cmd.MarkFlagRequired("cacert")
	cmd.MarkFlagRequired("cert")
	cmd.MarkFlagRequired("key")
	return cmd
}

func loadRbacConfig(path string) (*rbacv1.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read rbac configuration file: %w", err)
	}

	config := &rbacv1.Config{}

	data, err = yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("malformed rbac configuration file, expecting yaml or json: %w", err)
	}
	if err := protojson.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rbac configuration: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid rbac configuration: %w", err)
	}

	return config, nil
}
