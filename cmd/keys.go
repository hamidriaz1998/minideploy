package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "List all API keys",
	Long:  `List all API keys registered with the daemon, showing their scope, app, label, and creation date.`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		apiKey, _ := cmd.Flags().GetString("api-key")

		if apiKey == "" {
			apiKey = client.GetAdminKey()
		}
		if apiKey == "" {
			apiKey = os.Getenv("MINIDEPLOY_API_KEY")
		}
		if apiKey == "" {
			fmt.Fprintln(os.Stderr, "error: no admin API key found")
			os.Exit(1)
		}

		cfg := resolveServerConfig(host, port, apiKey)
		apiClient := client.NewAPIClient(cfg.Server.Host, cfg.Server.APIPort, cfg.Server.APIKey)

		keys, err := apiClient.ListKeys()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		if len(keys) == 0 {
			fmt.Println("No API keys found")
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
	keysCmd.Flags().StringP("host", "H", "127.0.0.1", "daemon host")
	keysCmd.Flags().IntP("port", "p", 8443, "daemon port")
	keysCmd.Flags().StringP("api-key", "k", "", "admin API key for authentication")
}
