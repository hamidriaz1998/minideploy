package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [release]",
	Short: "Rollback to a previous release",
	Long:  `Rollback the symlink to the specified release (or previous release if none specified) and restart services.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		if configPath == "" {
			var err error
			configPath, err = client.FindConfig()
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
		}

		cfg, err := client.LoadConfig(configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		if cfg.Server.APIKey == "" {
			fmt.Fprintln(os.Stderr, "error: no API key configured")
			os.Exit(1)
		}

		releaseName := ""
		if len(args) > 0 {
			releaseName = args[0]
		}

		apiClient := client.NewAPIClient(cfg.Server.Host, cfg.Server.APIPort, cfg.Server.APIKey)
		resp, err := apiClient.Rollback(shared.RollbackRequest{
			AppName:     cfg.AppName,
			ReleaseName: releaseName,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		fmt.Printf("[rollback] rolled back to release %s\n", resp.Release)
		fmt.Printf("[rollback] instances restarted: %v\n", resp.Instances)
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
	rollbackCmd.Flags().StringP("config", "c", "", "path to .deploy.yml")
}
