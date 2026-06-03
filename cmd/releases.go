package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var releasesCmd = &cobra.Command{
	Use:   "releases [app-name]",
	Short: "List releases for an app",
	Long:  `Show all releases for a registered app. If no app name is given, reads from .deploy.yml.`,
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

		releases, err := apiClient.AppReleases(appName)
		if err != nil {
			shared.Fatal("%v", err)
		}

		if len(releases) == 0 {
			shared.Info("no releases for app %q", appName)
			return
		}

		for _, r := range releases {
			mark := "  "
			if r.IsCurrent {
				mark = "→ "
			}
			fmt.Printf("%s %s  %s\n", mark, r.Name, r.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	},
}

func init() {
	rootCmd.AddCommand(releasesCmd)
	releasesCmd.Flags().StringP("host", "H", "127.0.0.1", "daemon host")
	releasesCmd.Flags().IntP("port", "p", 8443, "daemon port")
	releasesCmd.Flags().StringP("api-key", "k", "", "API key")
}
