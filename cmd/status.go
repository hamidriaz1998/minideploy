package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon health and overview",
	Long:  `Query the minideploy daemon for server health, uptime, app count, and disk usage.`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		apiKey, _ := cmd.Flags().GetString("api-key")

		cfg := resolveServerConfig(host, port, apiKey)
		apiClient := client.NewAPIClient(cfg.Server.Host, cfg.Server.APIPort, cfg.Server.APIKey)

		status, err := apiClient.Status()
		if err != nil {
			shared.Fatal("%v", err)
		}

		fmt.Printf("Daemon:  minideploy v%s\n", status.Version)
		fmt.Printf("Uptime:  %s\n", status.Uptime)
		fmt.Printf("Apps:    %d\n", status.AppsCount)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().StringP("host", "H", "127.0.0.1", "daemon host")
	statusCmd.Flags().IntP("port", "p", 8443, "daemon port")
	statusCmd.Flags().StringP("api-key", "k", "", "API key")
}
