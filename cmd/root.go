package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
)

var rootCmd = &cobra.Command{
	Use:   "minideploy",
	Short: "Server deployment manager",
	Long: `minideploy builds your project, uploads artifacts via rsync,
and deploys them through a server-side daemon with zero-downtime symlink swaps.

Supports systemd and pm2 process managers.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func resolveServerConfig(host string, port int, apiKey string) *client.Config {
	cfg := &client.Config{}
	cfg.Server.Host = host
	cfg.Server.APIPort = port
	cfg.Server.APIKey = apiKey

	if cfg.Server.APIPort == 0 {
		cfg.Server.APIPort = 8443
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	if cfg.Server.APIKey == "" {
		cfg.Server.APIKey = os.Getenv("MINIDEPLOY_API_KEY")
	}

	return cfg
}
