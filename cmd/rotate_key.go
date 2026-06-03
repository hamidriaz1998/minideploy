package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var rotateKeyCmd = &cobra.Command{
	Use:   "rotate-key",
	Short: "Generate a new API key for the daemon",
	Long: `Generate a new API key and register it with the daemon.

By default, old keys remain valid so you can update CI/CD pipelines
and other team members at your own pace.

Use --revoke-old to immediately invalidate all previous keys.

Requires the current API key for authentication (from config/env).`,
	Run: func(cmd *cobra.Command, args []string) {
		revokeOld, _ := cmd.Flags().GetBool("revoke-old")

		configPath, _ := cmd.Flags().GetString("config")
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		apiKey, _ := cmd.Flags().GetString("api-key")

		cfg := resolveServerConfig(host, port, apiKey)

		if cfg.Server.APIKey == "" && configPath == "" {
			configPath = tryFindConfig()
		}
		if configPath != "" && cfg.Server.APIKey == "" {
			loaded, err := client.LoadConfig(configPath)
			if err == nil {
				cfg.Server.APIKey = loaded.Server.APIKey
				if cfg.Server.Host == "127.0.0.1" && loaded.Server.Host != "" {
					cfg.Server.Host = loaded.Server.Host
				}
				if cfg.Server.APIPort == 8443 && loaded.Server.APIPort != 0 {
					cfg.Server.APIPort = loaded.Server.APIPort
				}
			}
		}

		if cfg.Server.APIKey == "" {
			cfg.Server.APIKey = os.Getenv("MINIDEPLOY_API_KEY")
		}
		if cfg.Server.APIKey == "" {
			fmt.Fprintln(os.Stderr, "error: no API key configured for authentication")
			os.Exit(1)
		}

		apiClient := client.NewAPIClient(cfg.Server.Host, cfg.Server.APIPort, cfg.Server.APIKey)

		resp, err := apiClient.RotateKey(shared.RotateKeyRequest{
			RevokeOld: revokeOld,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		fmt.Printf("New API key: %s\n", resp.NewKey)
		fmt.Printf("Active keys: %d\n", resp.KeysCount)
		if revokeOld {
			fmt.Println("Old keys have been revoked.")
		} else {
			fmt.Println("Old keys remain valid. Use --revoke-old to invalidate them.")
		}
	},
}

func tryFindConfig() string {
	path, err := client.FindConfig()
	if err != nil {
		return ""
	}
	return path
}

func init() {
	rootCmd.AddCommand(rotateKeyCmd)
	rotateKeyCmd.Flags().Bool("revoke-old", false, "Invalidate all previous keys")
	rotateKeyCmd.Flags().StringP("config", "c", "", "path to .deploy.yml")
	rotateKeyCmd.Flags().StringP("host", "H", "127.0.0.1", "daemon host")
	rotateKeyCmd.Flags().IntP("port", "p", 8443, "daemon port")
	rotateKeyCmd.Flags().StringP("api-key", "k", "", "current API key for authentication")
}
