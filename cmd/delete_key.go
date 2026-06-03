package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
)

var deleteKeyCmd = &cobra.Command{
	Use:   "delete-key <id>",
	Short: "Delete an API key by ID",
	Long:  `Permanently delete an API key. Use 'minideploy keys' to find the key ID.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: invalid key id")
			os.Exit(1)
		}

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

		resp, err := apiClient.DeleteKey(id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		if resp.Deleted {
			fmt.Printf("Key %d deleted\n", id)
		}
	},
}

func init() {
	rootCmd.AddCommand(deleteKeyCmd)
	deleteKeyCmd.Flags().StringP("host", "H", "127.0.0.1", "daemon host")
	deleteKeyCmd.Flags().IntP("port", "p", 8443, "daemon port")
	deleteKeyCmd.Flags().StringP("api-key", "k", "", "admin API key for authentication")
}
