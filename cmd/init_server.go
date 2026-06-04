package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/daemon"
	"github.com/hamid/minideploy/internal/shared"
)

var initServerCmd = &cobra.Command{
	Use:   "init-server",
	Short: "Bootstrap the daemon on a VPS",
	Long: `Set up the minideploy daemon on a remote server:
1. Copy the daemon binary to the server
2. SSH into the server to:
   - Create minideploy user
   - Create state directory
   - Install the daemon binary
   - Install the daemon systemd service
   - Configure sudoers for minideploy
   - Generate API key
   - Start the daemon

After init-server completes, run 'minideploy init-app' for each app.`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		sshUser, _ := cmd.Flags().GetString("ssh-user")
		binaryPath, _ := cmd.Flags().GetString("binary")
		forceUpload, _ := cmd.Flags().GetBool("force")

		if sshUser == "" {
			sshUser = "root"
		}

		// Auto-pull from .deploy.yml if present
		if cfg, err := client.LoadConfig(".deploy.yml"); err == nil {
			if host == "" && cfg.Server.Host != "" {
				host = cfg.Server.Host
			}
			if sshUser == "root" && cfg.Server.SSHUser != "" {
				sshUser = cfg.Server.SSHUser
			}
			shared.Debug("auto-pulled config from .deploy.yml: host=%s, ssh_user=%s", host, sshUser)
		}

		if host == "" {
			shared.Fatal("--host is required")
		}

		// Resolve binary path
		if binaryPath == "" {
			var err error
			binaryPath, err = os.Executable()
			if err != nil {
				shared.Fatal("cannot determine running binary path: %v\n  Use --binary /path/to/minideploy to specify the binary", err)
			}
		}
		binaryPath, _ = filepath.Abs(binaryPath)
		shared.Info("using binary: %s", binaryPath)

		shared.Info("generating API key...")
		rawKey, keyHash, err := daemon.GenerateAPIKey()
		if err != nil {
			shared.Fatal("generate key: %v", err)
		}

		stateDir := "/var/lib/minideploy"
		preCommands := []string{
			"id -u minideploy 2>/dev/null || sudo useradd --system --no-create-home --shell /sbin/nologin minideploy",
			"sudo groupadd -f deploy",
			fmt.Sprintf("sudo usermod -aG deploy minideploy"),
			fmt.Sprintf("sudo usermod -aG deploy %s", sshUser),
			fmt.Sprintf("sudo mkdir -p %s", stateDir),
			fmt.Sprintf("sudo chown -R minideploy:minideploy %s", stateDir),
		}

		shared.Info("setting up daemon user and state directory...")
		for _, cmdStr := range preCommands {
			shared.Debug("running: %s", cmdStr)
			ssh := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host), cmdStr)
			ssh.Stdout = os.Stdout
			ssh.Stderr = os.Stderr
			if err := ssh.Run(); err != nil {
				shared.Fatal("command failed: %s: %v", cmdStr, err)
			}
		}

		// Check if binary already exists on remote (skip check if --force)
		shouldUpload := forceUpload
		if !shouldUpload {
			shared.Debug("checking if minideploy binary exists on remote...")
			checkBin := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host), "command -v minideploy")
			if err := checkBin.Run(); err != nil {
				shouldUpload = true
			} else {
				shared.Debug("binary already installed, skipping upload")
			}
		}

		if shouldUpload {
			remoteTmp := "/tmp/minideploy"
			shared.Info("uploading binary to %s@%s:%s...", sshUser, host, remoteTmp)
			shared.Debug("scp %s %s@%s:%s", binaryPath, sshUser, host, remoteTmp)
			scp := exec.Command("scp", binaryPath, fmt.Sprintf("%s@%s:%s", sshUser, host, remoteTmp))
			scp.Stdout = os.Stdout
			scp.Stderr = os.Stderr
			if err := scp.Run(); err != nil {
				shared.Fatal("scp failed: %v", err)
			}

			installCommands := []string{
				fmt.Sprintf("sudo mv %s /usr/local/bin/minideploy", remoteTmp),
				"sudo chmod 755 /usr/local/bin/minideploy",
				"sudo chown root:root /usr/local/bin/minideploy",
			}
			shared.Info("installing binary...")
			for _, cmdStr := range installCommands {
				shared.Debug("running: %s", cmdStr)
				ssh := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host), cmdStr)
				ssh.Stdout = os.Stdout
				ssh.Stderr = os.Stderr
				if err := ssh.Run(); err != nil {
					shared.Fatal("command failed: %s: %v", cmdStr, err)
				}
			}
		}

		shared.Info("writing systemd service file...")
		serviceContent := fmt.Sprintf(`[Unit]
Description=minideploy Daemon
After=network.target

[Service]
Type=simple
User=minideploy
Group=minideploy
ExecStart=/usr/local/bin/minideploy daemon --state-dir %s
Restart=always
RestartSec=5
StateDirectory=minideploy

[Install]
WantedBy=multi-user.target
`, stateDir)

		writeService := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host),
			"sudo tee /etc/systemd/system/minideploy.service > /dev/null")
		serviceStdin, _ := writeService.StdinPipe()
		writeService.Stdout = os.Stdout
		writeService.Stderr = os.Stderr
		writeService.Start()
		serviceStdin.Write([]byte(serviceContent))
		serviceStdin.Close()
		writeService.Wait()

		sudoersContent := `# minideploy daemon - managed process commands
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl status *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl start *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl stop *
minideploy ALL=(root) NOPASSWD: /usr/bin/journalctl -u *
minideploy ALL=(root) NOPASSWD: /usr/sbin/useradd *
`
		shared.Info("writing sudoers...")
		writeSudoers := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host),
			"sudo tee /etc/sudoers.d/minideploy > /dev/null")
		sudoersStdin, _ := writeSudoers.StdinPipe()
		writeSudoers.Stdout = os.Stdout
		writeSudoers.Stderr = os.Stderr
		writeSudoers.Start()
		sudoersStdin.Write([]byte(sudoersContent))
		sudoersStdin.Close()
		writeSudoers.Wait()

		shared.Info("seeding API key into daemon database...")
		seedCmd := fmt.Sprintf("sudo -u minideploy /usr/local/bin/minideploy daemon import-key --state-dir %s '%s'", stateDir, keyHash)
		shared.Debug("running: %s", seedCmd)
		seed := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host), seedCmd)
		seed.Stdout = os.Stdout
		seed.Stderr = os.Stderr
		if err := seed.Run(); err != nil {
			shared.Fatal("seed key failed: %v", err)
		}

		shared.Info("enabling and starting daemon...")
		enableCmds := []string{
			"sudo chmod 440 /etc/sudoers.d/minideploy",
			"sudo systemctl daemon-reload",
			"sudo systemctl enable minideploy",
			"sudo systemctl start minideploy",
		}
		for _, cmdStr := range enableCmds {
			shared.Debug("running: %s", cmdStr)
			ssh := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host), cmdStr)
			ssh.Stdout = os.Stdout
			ssh.Stderr = os.Stderr
			if err := ssh.Run(); err != nil {
				shared.Warn("%s failed: %v", cmdStr, err)
			}
		}

		shared.Info("saving admin key for host %s to global config...", host)
		globalCfg, err := client.LoadGlobalConfig()
		if err != nil {
			globalCfg = &client.GlobalConfig{}
		}
		if globalCfg.Hosts == nil {
			globalCfg.Hosts = make(map[string]client.HostConfig)
		}
		globalCfg.Hosts[host] = client.HostConfig{AdminKey: rawKey}
		if err := client.SaveGlobalConfig(globalCfg); err != nil {
			shared.Warn("could not save admin key to config: %v", err)
		} else {
			shared.Info("admin key saved to ~/.config/minideploy/config.yml")
		}

		fmt.Println()
		fmt.Println("═══════════════════════════════════════════")
		fmt.Println("  Daemon installed!")
		fmt.Println()
		fmt.Printf("  Host:      %s\n", host)
		fmt.Printf("  API Port:  8443\n")
		fmt.Println()
		fmt.Println("  Admin API Key (saved to global config):")
		fmt.Printf("  %s\n", rawKey)
		fmt.Println()
		fmt.Println("  Add to .deploy.yml:")
		fmt.Printf("  server:\n")
		fmt.Printf("    host: %s\n", host)
		fmt.Printf("    api_port: 8443\n")
		fmt.Printf("    ssh_user: %s\n", sshUser)
		fmt.Printf("    api_key: %s\n", rawKey)
		fmt.Println()
		fmt.Println("  Next, initialize your app with:")
		fmt.Println("  minideploy init-app")
		fmt.Println("═══════════════════════════════════════════")
	},
}

func init() {
	rootCmd.AddCommand(initServerCmd)
	initServerCmd.Flags().String("host", "", "VPS hostname or IP (default: from .deploy.yml)")
	initServerCmd.Flags().String("ssh-user", "root", "SSH user for initial setup")
	initServerCmd.Flags().StringP("binary", "b", "", "Path to minideploy binary to upload (default: the running binary)")
	initServerCmd.Flags().Bool("force", false, "Force binary upload even if already installed")
}
