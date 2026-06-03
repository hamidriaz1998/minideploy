package cmd

import (
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
				shared.Fatal("%v", err)
			}
		}

		cfg, err := client.LoadConfig(configPath)
		if err != nil {
			shared.Fatal("%v", err)
		}

		if cfg.Server.APIKey == "" {
			shared.Fatal("no API key configured")
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
			shared.Fatal("%v", err)
		}

		shared.Success("rollback: rolled back to release %s", resp.Release)
		shared.Info("rollback: instances restarted: %v", resp.Instances)
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
	rollbackCmd.Flags().StringP("config", "c", "", "path to .deploy.yml")
}
