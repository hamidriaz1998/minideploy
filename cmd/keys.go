package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "List all API keys",
	Long:  `List all API keys registered with the daemon, showing their scope, app, label, and creation date.`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		apiKey, _ := cmd.Flags().GetString("api-key")

		if host == "" {
			if cfg, err := client.LoadConfig(".deploy.yml"); err == nil {
				host = cfg.Server.Host
			}
		}
		if host == "" {
			shared.Fatal("use --host to specify the daemon host (or add server.host to .deploy.yml)")
		}

		if apiKey == "" {
			apiKey = client.GetAdminKey(host)
		}
		if apiKey == "" {
			apiKey = os.Getenv("MINIDEPLOY_API_KEY")
		}
		if apiKey == "" {
			shared.Fatal("no admin API key found for %s", host)
		}

		cfg := resolveServerConfig(host, port, apiKey)
		apiClient := client.NewAPIClient(cfg.Server.Host, cfg.Server.APIPort, cfg.Server.APIKey)

		keys, err := apiClient.ListKeys()
		if err != nil {
			shared.Fatal("%v", err)
		}

		if len(keys) == 0 {
			shared.Info("No API keys found")
			return
		}

		fmt.Printf("%-4s %-8s %-16s %-20s %-12s\n", "ID", "Scope", "App", "Label", "Created")
		fmt.Println("---- -------- ---------------- -------------------- ------------")
		for _, k := range keys {
			app := k.AppName
			if app == "" {
				app = "(all)"
			}
			label := k.Label
			if label == "" {
				label = "-"
			}
			fmt.Printf("%-4d %-8s %-16s %-20s %-12s\n", k.ID, k.Scope, app, label, k.CreatedAt.Format("2006-01-02"))
		}
	},
}

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.Flags().StringP("host", "H", "", "daemon host (default: from .deploy.yml)")
	keysCmd.Flags().IntP("port", "p", 8443, "daemon port")
	keysCmd.Flags().StringP("api-key", "k", "", "admin API key for authentication")
}
