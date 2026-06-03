package cmd

import (
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var deleteKeyCmd = &cobra.Command{
	Use:   "delete-key <id>",
	Short: "Delete an API key by ID",
	Long:  `Permanently delete an API key. Use 'minideploy keys' to find the key ID.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			shared.Fatal("invalid key id")
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
			shared.Fatal("no admin API key found")
		}

		cfg := resolveServerConfig(host, port, apiKey)
		apiClient := client.NewAPIClient(cfg.Server.Host, cfg.Server.APIPort, cfg.Server.APIKey)

		resp, err := apiClient.DeleteKey(id)
		if err != nil {
			shared.Fatal("%v", err)
		}

		if resp.Deleted {
			shared.Success("Key %d deleted", id)
		}
	},
}

func init() {
	rootCmd.AddCommand(deleteKeyCmd)
	deleteKeyCmd.Flags().StringP("host", "H", "127.0.0.1", "daemon host")
	deleteKeyCmd.Flags().IntP("port", "p", 8443, "daemon port")
	deleteKeyCmd.Flags().StringP("api-key", "k", "", "admin API key for authentication")
}
