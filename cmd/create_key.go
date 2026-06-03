package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var createKeyCmd = &cobra.Command{
	Use:   "create-key",
	Short: "Create a new API key",
	Long: `Create a new API key with optional scope (global or app) and label.

Global keys have full access. App-scoped keys can only deploy and view
status/logs for a single app.`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		apiKey, _ := cmd.Flags().GetString("api-key")
		scope, _ := cmd.Flags().GetString("scope")
		appName, _ := cmd.Flags().GetString("app-name")
		label, _ := cmd.Flags().GetString("label")

		if apiKey == "" {
			apiKey = client.GetAdminKey()
		}
		if apiKey == "" {
			apiKey = os.Getenv("MINIDEPLOY_API_KEY")
		}
		if apiKey == "" {
			fmt.Fprintln(os.Stderr, "error: no admin API key found (set --api-key, MINIDEPLOY_API_KEY, or admin_key in config)")
			os.Exit(1)
		}

		cfg := resolveServerConfig(host, port, apiKey)
		apiClient := client.NewAPIClient(cfg.Server.Host, cfg.Server.APIPort, cfg.Server.APIKey)

		req := shared.CreateKeyRequest{
			Scope:   scope,
			AppName: appName,
			Label:   label,
		}

		resp, err := apiClient.CreateKey(req)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		fmt.Printf("Key created (id=%d)\n", resp.ID)
		fmt.Printf("Scope:   %s\n", resp.Scope)
		if resp.AppName != "" {
			fmt.Printf("App:     %s\n", resp.AppName)
		}
		if resp.Label != "" {
			fmt.Printf("Label:   %s\n", resp.Label)
		}
		fmt.Printf("API Key: %s\n", resp.RawKey)
	},
}

func init() {
	rootCmd.AddCommand(createKeyCmd)
	createKeyCmd.Flags().StringP("host", "H", "127.0.0.1", "daemon host")
	createKeyCmd.Flags().IntP("port", "p", 8443, "daemon port")
	createKeyCmd.Flags().StringP("api-key", "k", "", "admin API key for authentication")
	createKeyCmd.Flags().String("scope", "app", "key scope (global or app)")
	createKeyCmd.Flags().StringP("app-name", "a", "", "app name (required for app-scoped keys)")
	createKeyCmd.Flags().StringP("label", "l", "", "optional label for the key")
}
