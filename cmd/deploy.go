package cmd

import (
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
		skipBuild, _ := cmd.Flags().GetBool("skip-build")
		skipUpload, _ := cmd.Flags().GetBool("skip-upload")

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

		shared.Info("starting deployment for %s", cfg.AppName)

		if !skipBuild {
			shared.Debug("running build steps: %v", cfg.Build)
			if err := client.RunBuildSteps(cfg.Build); err != nil {
				shared.Fatal("%v", err)
			}
			shared.Debug("verifying artifacts: %v", cfg.Artifacts)
			if err := client.VerifyArtifacts(cfg.Artifacts); err != nil {
				shared.Fatal("%v", err)
			}
		} else {
			shared.Debug("skipping build step (--skip-build)")
		}

		if !skipUpload {
			size := client.ArtifactsTotalSize(cfg.Artifacts)
			shared.Info("uploading %s of artifacts to %s@%s:%s/upload/", size, cfg.Server.SSHUser, cfg.Server.Host, cfg.DeployPath)
			if err := client.RunRsync(client.RsyncConfig{
				SSHUser:   cfg.Server.SSHUser,
				Host:      cfg.Server.Host,
				DeployDir: cfg.DeployPath,
				Artifacts: cfg.Artifacts,
			}); err != nil {
				shared.Fatal("%v", err)
			}
		} else {
			shared.Debug("skipping upload (--skip-upload), using existing files in upload/")
		}

		if cfg.Server.APIKey == "" {
			shared.Fatal("no API key configured (set server.api_key, MINIDEPLOY_API_KEY env, or .env)")
		}

		if cfg.Server.SSHUser == "" {
			cfg.Server.SSHUser = "root"
		}

		if err := client.EnsureServiceTemplate(cfg); err != nil {
			shared.Fatal("%v", err)
		}
		if err := client.EnsureEnvFiles(cfg); err != nil {
			shared.Fatal("%v", err)
		}

		host := cfg.Server.Host
		if client.NeedsTunnel(host) && !client.IsPortOpen("127.0.0.1", cfg.Server.APIPort) {
			shared.Debug("no daemon reachable on localhost:%d, starting ssh tunnel", cfg.Server.APIPort)
			apiClient := client.NewAPIClient("127.0.0.1", cfg.Server.APIPort, cfg.Server.APIKey)
			tunnel, err := client.StartTunnel(host, cfg.Server.SSHUser, cfg.Server.APIPort, cfg.Server.APIPort)
			if err != nil {
				shared.Fatal("ssh tunnel: %v", err)
			}
			defer tunnel.Close()

			shared.Debug("ssh tunnel established to %s", host)
			doDeploy(apiClient, cfg, releaseName)
		} else if client.NeedsTunnel(host) {
			shared.Debug("tunnel or local daemon already running on 127.0.0.1:%d", cfg.Server.APIPort)
			apiClient := client.NewAPIClient("127.0.0.1", cfg.Server.APIPort, cfg.Server.APIKey)
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
		shared.Debug("using custom release name: %s", releaseName)
	}

	shared.Debug("sending deploy request to daemon")
	resp, err := apiClient.Deploy(req)
	if err != nil {
		shared.Fatal("%v", err)
	}

	if len(resp.FailedInstances) > 0 {
		shared.Warn("some instances failed to restart: %v", resp.FailedInstances)
	}
	if resp.RolledBack {
		shared.Warn("health check failed, rolled back to %s", resp.RolledBackTo)
	} else if len(resp.FailedInstances) == 0 {
		shared.Success("release %s deployed successfully", resp.Release)
	}
	shared.Info("instances restarted: %v", resp.Instances)
	if len(resp.HealthResults) > 0 {
		for _, hr := range resp.HealthResults {
			if hr.Passed {
				shared.Success("health ✓ port %d %s", hr.Port, hr.Instance)
			} else {
				extra := ""
				if hr.Error != "" {
					extra = " (" + hr.Error + ")"
				}
				shared.Error("health ✗ port %d %s%s", hr.Port, hr.Instance, extra)
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringP("config", "c", "", "path to .deploy.yml")
	deployCmd.Flags().StringP("release", "r", "", "custom release name (default: auto-generated)")
	deployCmd.Flags().Bool("skip-build", false, "skip build steps and artifact verification")
	deployCmd.Flags().Bool("skip-upload", false, "skip rsync upload (deploy from existing upload/ on server)")
}
