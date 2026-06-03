package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy [app-name]",
	Short: "Remove an app from the server",
	Long: `Completely remove an app managed by the daemon.

Soft mode (--soft): stops services and unregisters the app,
but leaves releases on disk for recovery.

Hard mode (default): stops services, removes all files from
the deploy directory, and unregisters the app.

Use --confirm to acknowledge the action.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		soft, _ := cmd.Flags().GetBool("soft")
		confirm, _ := cmd.Flags().GetBool("confirm")
		configPath, _ := cmd.Flags().GetString("config")

		appName := ""
		if len(args) > 0 {
			appName = args[0]
		}

		if !confirm {
			fmt.Fprintln(os.Stderr, "error: --confirm is required to destroy an app")
			os.Exit(1)
		}

		cfg := &client.Config{}
		if appName == "" {
			if configPath == "" {
				var err error
				configPath, err = client.FindConfig()
				if err != nil {
					fmt.Fprintln(os.Stderr, "error:", err)
					os.Exit(1)
				}
			}
			var err error
			cfg, err = client.LoadConfig(configPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
			appName = cfg.AppName
		}

		if cfg.Server.APIKey == "" {
			if key := os.Getenv("MINIDEPLOY_API_KEY"); key != "" {
				cfg.Server.APIKey = key
			}
		}
		if cfg.Server.APIKey == "" && appName != "" {
			// bare command: minideploy destroy my-app --confirm
			// try env variable
			cfg.Server.APIKey = os.Getenv("MINIDEPLOY_API_KEY")
		}
		if cfg.Server.APIKey == "" {
			fmt.Fprintln(os.Stderr, "error: no API key configured")
			os.Exit(1)
		}
		if cfg.Server.Host == "" {
			cfg.Server.Host = "127.0.0.1"
		}
		if cfg.Server.APIPort == 0 {
			cfg.Server.APIPort = 8443
		}

		mode := "hard"
		if soft {
			mode = "soft"
		}
		fmt.Printf("[destroy] %s destroying %q on %s...\n", mode, appName, cfg.Server.Host)

		apiClient := client.NewAPIClient(cfg.Server.Host, cfg.Server.APIPort, cfg.Server.APIKey)
		resp, err := apiClient.Destroy(shared.DestroyRequest{
			AppName: appName,
			Soft:    soft,
			Confirm: true,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		mode = "hard"
		if resp.Soft {
			mode = "soft"
		}
		fmt.Printf("[destroy] %s destroyed %q\n", mode, resp.AppName)
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().BoolP("soft", "s", false, "soft destroy (keep files on disk)")
	destroyCmd.Flags().BoolP("confirm", "y", false, "confirm destruction")
	destroyCmd.Flags().StringP("config", "c", "", "path to .deploy.yml")
}
