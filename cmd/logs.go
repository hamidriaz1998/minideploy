package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
)

var logsCmd = &cobra.Command{
	Use:   "logs [app-name]",
	Short: "Tail logs for an app",
	Long:  `Fetch the most recent log entries for all instances of the specified app.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		apiKey, _ := cmd.Flags().GetString("api-key")

		appName := ""
		if len(args) > 0 {
			appName = args[0]
		}

		cfg := resolveServerConfig(host, port, apiKey)
		if appName == "" {
			appName = cfg.AppName
		}

		apiClient := client.NewAPIClient(cfg.Server.Host, cfg.Server.APIPort, cfg.Server.APIKey)

		logs, err := apiClient.AppLogs(appName)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		fmt.Print(logs)
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().StringP("host", "H", "127.0.0.1", "daemon host")
	logsCmd.Flags().IntP("port", "p", 8443, "daemon port")
	logsCmd.Flags().StringP("api-key", "k", "", "API key")
}
