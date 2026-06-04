package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var initAppCmd = &cobra.Command{
	Use:   "init-app",
	Short: "Initialize app directories and register with the daemon",
	Long: `Create the directory structure for an app on the server and
register it with the daemon so it can be deployed.

1. SSH into the server to create /opt/<app>/upload and releases/
2. Call the daemon API to register the app

Run this after 'minideploy init-server' for each app you want to manage.
The daemon SSH tunnel is managed automatically.`,
	Run: func(cmd *cobra.Command, args []string) {
		appName, _ := cmd.Flags().GetString("app-name")
		deployPath, _ := cmd.Flags().GetString("deploy-path")
		host, _ := cmd.Flags().GetString("host")
		sshUser, _ := cmd.Flags().GetString("ssh-user")
		apiPort, _ := cmd.Flags().GetInt("api-port")
		apiKey, _ := cmd.Flags().GetString("api-key")

		if sshUser == "" {
			sshUser = "root"
		}

		// Auto-pull from .deploy.yml if present
		if cfg, err := client.LoadConfig(".deploy.yml"); err == nil {
			if appName == "" && cfg.AppName != "" {
				appName = cfg.AppName
			}
			if deployPath == "" && cfg.DeployPath != "" {
				deployPath = cfg.DeployPath
			} else if deployPath == "" && appName != "" {
				deployPath = "/opt/" + appName
			}
			if host == "" && cfg.Server.Host != "" {
				host = cfg.Server.Host
			}
			if sshUser == "root" && cfg.Server.SSHUser != "" {
				sshUser = cfg.Server.SSHUser
			}
			if apiKey == "" && cfg.Server.APIKey != "" {
				apiKey = cfg.Server.APIKey
			}
		}

		if appName == "" {
			shared.Fatal("--app-name is required (or add app_name to .deploy.yml)")
		}
		if deployPath == "" {
			deployPath = "/opt/" + appName
		}
		if host == "" {
			shared.Fatal("--host is required (or add server.host to .deploy.yml)")
		}

		// Resolve API key from flag, global config, or env
		if apiKey == "" {
			apiKey = client.GetAdminKey()
		}
		if apiKey == "" {
			apiKey = os.Getenv("MINIDEPLOY_API_KEY")
		}

		// SSH: create app directory structure
		cmds := []string{
			fmt.Sprintf("sudo mkdir -p %s/upload %s/releases", deployPath, deployPath),
			fmt.Sprintf("sudo chown -R minideploy:minideploy %s", deployPath),
			fmt.Sprintf("sudo chmod 1777 %s/upload", deployPath),
		}

		shared.Info("creating app directories on %s...", host)
		for _, cmdStr := range cmds {
			shared.Debug("running: %s", cmdStr)
			ssh := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host), cmdStr)
			ssh.Stdout = os.Stdout
			ssh.Stderr = os.Stderr
			if err := ssh.Run(); err != nil {
				shared.Fatal("command failed: %s: %v", cmdStr, err)
			}
		}

		// Tunnel + API call: register the app with the daemon
		var apiClient *client.APIClient
		if client.NeedsTunnel(host) && !client.IsPortOpen("127.0.0.1", apiPort) {
			shared.Debug("no daemon reachable on localhost:%d, starting ssh tunnel", apiPort)
			apiClient = client.NewAPIClient("127.0.0.1", apiPort, apiKey)
			tunnel, err := client.StartTunnel(host, sshUser, apiPort, apiPort)
			if err != nil {
				shared.Fatal("ssh tunnel: %v", err)
			}
			defer tunnel.Close()
			shared.Debug("ssh tunnel established to %s", host)
		} else if client.NeedsTunnel(host) {
			shared.Debug("tunnel or local daemon already running on 127.0.0.1:%d", apiPort)
			apiClient = client.NewAPIClient("127.0.0.1", apiPort, apiKey)
		} else {
			apiClient = client.NewAPIClient(host, apiPort, apiKey)
		}

		shared.Info("registering app %q with daemon...", appName)
		if _, err := apiClient.InitApp(shared.InitAppRequest{
			AppName:    appName,
			DeployPath: deployPath,
		}); err != nil {
			shared.Fatal("register app: %v", err)
		}

		fmt.Println()
		fmt.Println("═══════════════════════════════════════════")
		fmt.Printf("  App %q initialized!\n", appName)
		fmt.Println()
		fmt.Printf("  Deploy path: %s\n", deployPath)
		fmt.Println()
		fmt.Println("  Run 'minideploy deploy' to deploy your app.")
		fmt.Println("═══════════════════════════════════════════")
	},
}

func init() {
	rootCmd.AddCommand(initAppCmd)
	initAppCmd.Flags().String("app-name", "", "App name (default: from .deploy.yml)")
	initAppCmd.Flags().String("deploy-path", "", "Deploy path on server (default: /opt/<app-name>)")
	initAppCmd.Flags().String("host", "", "Daemon host (default: from .deploy.yml)")
	initAppCmd.Flags().String("ssh-user", "root", "SSH user for directory creation and tunnel")
	initAppCmd.Flags().Int("api-port", 8443, "Daemon API port")
	initAppCmd.Flags().String("api-key", "", "API key (default: from global config or MINIDEPLOY_API_KEY env)")
}
