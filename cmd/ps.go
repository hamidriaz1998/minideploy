package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List running processes managed by the daemon",
	Long:  `Query the daemon for all registered apps and their running instances.`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		apiKey, _ := cmd.Flags().GetString("api-key")

		cfg := resolveServerConfig(host, port, apiKey)
		apiClient := client.NewAPIClient(cfg.Server.Host, cfg.Server.APIPort, cfg.Server.APIKey)

		apps, err := apiClient.ListApps()
		if err != nil {
			shared.Fatal("%v", err)
		}

		if len(apps) == 0 {
			shared.Info("no apps registered")
			return
		}

		for _, app := range apps {
			status := "running"
			if !app.Running {
				status = "stopped"
			}
			fmt.Printf("%-20s %-10s %s\n", app.Name, status, app.CurrentRelease)

			if app.InstancesCount > 1 || len(cmd.Flags().Args()) > 0 {
				detail, err := apiClient.AppStatus(app.Name)
				if err == nil {
					for _, inst := range detail.Instances {
						r := "●"
						if !inst.Running {
							r = "○"
						}
						fmt.Printf("  └─ %s@%s  %s  (port %d)\n", app.Name, inst.ID, r, inst.Port)
					}
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(psCmd)
	psCmd.Flags().StringP("host", "H", "127.0.0.1", "daemon host")
	psCmd.Flags().IntP("port", "p", 8443, "daemon port")
	psCmd.Flags().StringP("api-key", "k", "", "API key")
}
