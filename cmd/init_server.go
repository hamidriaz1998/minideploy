package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/daemon"
)

var initServerCmd = &cobra.Command{
	Use:   "init-server",
	Short: "Bootstrap the daemon on a VPS",
	Long: `Set up the minideploy daemon on a remote server:
1. Copy the daemon binary to the server
2. SSH into the server to:
   - Create minideploy user
   - Create directory structure
   - Install the daemon binary
   - Install the daemon systemd service
   - Configure sudoers for minideploy
   - Generate API key
   - Start the daemon`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		sshUser, _ := cmd.Flags().GetString("ssh-user")
		appName, _ := cmd.Flags().GetString("app-name")
		deployPath, _ := cmd.Flags().GetString("deploy-path")
		binaryPath, _ := cmd.Flags().GetString("binary")

		if host == "" {
			fmt.Fprintln(os.Stderr, "error: --host is required")
			os.Exit(1)
		}
		if sshUser == "" {
			sshUser = "root"
		}
		if appName == "" {
			appName = "my-app"
		}
		if deployPath == "" {
			deployPath = "/opt/" + appName
		}

		if binaryPath == "" {
			var err error
			binaryPath, err = os.Executable()
			if err != nil {
				fmt.Fprintln(os.Stderr, "error: cannot determine running binary path:", err)
				fmt.Fprintln(os.Stderr, "  Use --binary /path/to/minideploy to specify the binary")
				os.Exit(1)
			}
		}
		binaryPath, _ = filepath.Abs(binaryPath)

		fmt.Printf("[init] using binary: %s\n", binaryPath)
		fmt.Println("[init] generating API key...")
		rawKey, _, err := daemon.GenerateAPIKey()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: generate key:", err)
			os.Exit(1)
		}

	stateDir := "/var/lib/minideploy"
	preCommands := []string{
		fmt.Sprintf("id -u minideploy 2>/dev/null || sudo useradd --system --no-create-home --shell /sbin/nologin minideploy"),
		fmt.Sprintf("sudo mkdir -p %s/upload %s/releases %s", deployPath, deployPath, stateDir),
		fmt.Sprintf("sudo chown -R minideploy:minideploy %s %s", deployPath, stateDir),
		fmt.Sprintf("sudo chmod 1777 %s/upload", deployPath),
	}

	fmt.Println("[init] setting up directories and user...")
	for _, cmdStr := range preCommands {
		ssh := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host), cmdStr)
		ssh.Stdout = os.Stdout
		ssh.Stderr = os.Stderr
		if err := ssh.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: command failed: %s: %v\n", cmdStr, err)
			os.Exit(1)
		}
	}

	remoteTmp := "/tmp/minideploy"
	fmt.Printf("[init] uploading binary to %s@%s:%s...\n", sshUser, host, remoteTmp)
	scp := exec.Command("scp", binaryPath, fmt.Sprintf("%s@%s:%s", sshUser, host, remoteTmp))
	scp.Stdout = os.Stdout
	scp.Stderr = os.Stderr
	if err := scp.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error: scp failed:", err)
		os.Exit(1)
	}

	installCommands := []string{
		fmt.Sprintf("sudo mv %s /usr/local/bin/minideploy", remoteTmp),
		"sudo chmod 755 /usr/local/bin/minideploy",
		"sudo chown root:root /usr/local/bin/minideploy",
	}
	fmt.Println("[init] installing binary...")
	for _, cmdStr := range installCommands {
		ssh := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host), cmdStr)
		ssh.Stdout = os.Stdout
		ssh.Stderr = os.Stderr
		if err := ssh.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: command failed: %s: %v\n", cmdStr, err)
			os.Exit(1)
		}
	}

	fmt.Println("[init] writing systemd service file...")
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

	sudoersContent := fmt.Sprintf(`# minideploy daemon - managed process commands
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl status *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl start *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl stop *
minideploy ALL=(root) NOPASSWD: /usr/bin/journalctl -u *
minideploy ALL=(root) NOPASSWD: /usr/sbin/useradd *
`)
	fmt.Println("[init] writing sudoers...")
	writeSudoers := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host),
		"sudo tee /etc/sudoers.d/minideploy > /dev/null")
	sudoersStdin, _ := writeSudoers.StdinPipe()
	writeSudoers.Stdout = os.Stdout
	writeSudoers.Stderr = os.Stderr
	writeSudoers.Start()
	sudoersStdin.Write([]byte(sudoersContent))
	sudoersStdin.Close()
	writeSudoers.Wait()

		fmt.Println("[init] enabling and starting daemon...")
		enableCmds := []string{
			"sudo chmod 440 /etc/sudoers.d/minideploy",
			"sudo systemctl daemon-reload",
			"sudo systemctl enable minideploy",
			"sudo systemctl start minideploy",
		}
		for _, cmdStr := range enableCmds {
			ssh := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host), cmdStr)
			ssh.Stdout = os.Stdout
			ssh.Stderr = os.Stderr
			if err := ssh.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %s failed: %v\n", cmdStr, err)
			}
		}

		fmt.Println("[init] saving admin key to global config...")
		globalCfg := &client.GlobalConfig{AdminKey: rawKey}
		if err := client.SaveGlobalConfig(globalCfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save admin key to config: %v\n", err)
		} else {
			fmt.Println("[init] admin key saved to ~/.config/minideploy/config.yml")
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
		fmt.Println("  Or create app-scoped keys with:")
		fmt.Println("  minideploy create-key --scope app --app-name <name>")
		fmt.Println("═══════════════════════════════════════════")
	},
}

func init() {
	rootCmd.AddCommand(initServerCmd)
	initServerCmd.Flags().String("host", "", "VPS hostname or IP (required)")
	initServerCmd.Flags().String("ssh-user", "root", "SSH user for initial setup")
	initServerCmd.Flags().String("app-name", "my-app", "Default app name")
	initServerCmd.Flags().String("deploy-path", "", "Deploy path on server (default: /opt/<app-name>)")
	initServerCmd.Flags().StringP("binary", "b", "", "Path to minideploy binary to upload (default: the running binary)")
}
