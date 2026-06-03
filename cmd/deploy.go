package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build, upload, and deploy the project",
	Long: `Runs the full deployment pipeline:
1. Execute build steps from .deploy.yml
2. Verify artifacts exist
3. Rsync artifacts to server upload directory
4. Trigger the daemon to snapshot and deploy`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		releaseName, _ := cmd.Flags().GetString("release")

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

		fmt.Println("[deploy] starting deployment for", cfg.AppName)

		if err := client.RunBuildSteps(cfg.Build); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		if err := client.VerifyArtifacts(cfg.Artifacts); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		if err := client.RunRsync(client.RsyncConfig{
			SSHUser:   cfg.Server.SSHUser,
			Host:      cfg.Server.Host,
			DeployDir: cfg.DeployPath,
			Artifacts: cfg.Artifacts,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		if cfg.Server.APIKey == "" {
			fmt.Fprintln(os.Stderr, "error: no API key configured (set server.api_key, MINIDEPLOY_API_KEY env, or .env)")
			os.Exit(1)
		}

		host := cfg.Server.Host
		if client.NeedsTunnel(host) {
			apiClient := client.NewAPIClient("127.0.0.1", cfg.Server.APIPort, cfg.Server.APIKey)
			tunnel, err := client.StartTunnel(host, cfg.Server.SSHUser, cfg.Server.APIPort, cfg.Server.APIPort)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error: ssh tunnel:", err)
				os.Exit(1)
			}
			defer tunnel.Close()

			fmt.Println("[deploy] ssh tunnel established")
			doDeploy(apiClient, cfg, releaseName)
		} else {
			apiClient := client.NewAPIClient(host, cfg.Server.APIPort, cfg.Server.APIKey)
			doDeploy(apiClient, cfg, releaseName)
		}
	},
}

func doDeploy(apiClient *client.APIClient, cfg *client.Config, releaseName string) {
	req := shared.DeployRequest{
		AppName:      cfg.AppName,
		ServiceType:  cfg.ServiceType,
		ServiceName:  cfg.ServiceName,
		Instances:    cfg.Instances,
		DeployPath:   cfg.DeployPath,
		KeepReleases: cfg.KeepReleases,
		HealthCheck:  cfg.HealthCheck,
	}
	if releaseName != "" {
		req.ReleaseName = releaseName
	}

	resp, err := apiClient.Deploy(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("[deploy] release %s deployed successfully\n", resp.Release)
	fmt.Printf("[deploy] instances restarted: %v\n", resp.Instances)
	if resp.RolledBack {
		fmt.Printf("[deploy] WARNING: health check failed, rolled back to %s\n", resp.RolledBackTo)
	}
	if len(resp.HealthResults) > 0 {
		for _, hr := range resp.HealthResults {
			status := "✓"
			if !hr.Passed {
				status = "✗"
			}
			extra := ""
			if hr.Error != "" {
				extra = " (" + hr.Error + ")"
			}
			fmt.Printf("[deploy]   health %s port %d %s%s\n", status, hr.Port, hr.Instance, extra)
		}
	}
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringP("config", "c", "", "path to .deploy.yml")
	deployCmd.Flags().StringP("release", "r", "", "custom release name (default: auto-generated)")
}
