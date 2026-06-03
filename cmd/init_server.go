package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/daemon"
)

var initServerCmd = &cobra.Command{
	Use:   "init-server",
	Short: "Bootstrap the daemon on a VPS",
	Long: `Set up the minideploy daemon on a remote server:
1. Cross-compile the daemon for linux/amd64
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

		fmt.Println("[init] building daemon binary for linux/amd64...")
		buildDir := "/tmp/minideploy-build"
		os.MkdirAll(buildDir, 0755)
		binaryPath := filepath.Join(buildDir, "minideploy")

		buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
		buildCmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64")
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "error: build failed:", err)
			os.Exit(1)
		}

		fmt.Println("[init] generating API key...")
		rawKey, _, err := daemon.GenerateAPIKey()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: generate key:", err)
			os.Exit(1)
		}

		stateDir := "/var/lib/minideploy"
		commands := []string{
			fmt.Sprintf("id -u minideploy 2>/dev/null || useradd --system --no-create-home --shell /sbin/nologin minideploy"),
			fmt.Sprintf("mkdir -p %s/upload %s/releases %s", deployPath, deployPath, stateDir),
			fmt.Sprintf("chown -R minideploy:minideploy %s %s", deployPath, stateDir),
		}

		fmt.Println("[init] uploading binary...")
		scp := exec.Command("scp", binaryPath, fmt.Sprintf("%s@%s:/usr/local/bin/minideploy", sshUser, host))
		scp.Stdout = os.Stdout
		scp.Stderr = os.Stderr
		if err := scp.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "error: scp failed:", err)
			os.Exit(1)
		}

		commands = append(commands,
			"chmod 755 /usr/local/bin/minideploy",
			"chown root:root /usr/local/bin/minideploy",
		)

		fmt.Println("[init] setting up daemon on server...")
		for _, cmdStr := range commands {
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

		serviceFile := "/etc/systemd/system/minideploy.service"
		ssh := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host),
			fmt.Sprintf("cat > %s", serviceFile))
		ssh.Stdin = os.Stdin
		ssh.Stderr = os.Stderr
		ssh.Stdin = nil

		ssh = exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host),
			fmt.Sprintf("cat > %s", serviceFile))
		stdin, _ := ssh.StdinPipe()
		ssh.Stdout = os.Stdout
		ssh.Stderr = os.Stderr
		ssh.Start()
		stdin.Write([]byte(serviceContent))
		stdin.Close()
		ssh.Wait()

		sudoersContent := fmt.Sprintf(`# minideploy daemon - managed process commands
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl status *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl start *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl stop *
minideploy ALL=(root) NOPASSWD: /usr/bin/journalctl -u *
minideploy ALL=(root) NOPASSWD: /usr/sbin/useradd *
`)
		fmt.Println("[init] writing sudoers...")
		ssh2 := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host),
			fmt.Sprintf("cat > /etc/sudoers.d/minideploy"))
		stdin2, _ := ssh2.StdinPipe()
		ssh2.Stdout = os.Stdout
		ssh2.Stderr = os.Stderr
		ssh2.Start()
		stdin2.Write([]byte(sudoersContent))
		stdin2.Close()
		ssh2.Wait()

		fmt.Println("[init] enabling and starting daemon...")
		enableCmds := []string{
			"chmod 440 /etc/sudoers.d/minideploy",
			"systemctl daemon-reload",
			"systemctl enable minideploy",
			"systemctl start minideploy",
		}
		for _, cmdStr := range enableCmds {
			ssh := exec.Command("ssh", fmt.Sprintf("%s@%s", sshUser, host), cmdStr)
			ssh.Stdout = os.Stdout
			ssh.Stderr = os.Stderr
			if err := ssh.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %s failed: %v\n", cmdStr, err)
			}
		}

		fmt.Println()
		fmt.Println("═══════════════════════════════════════════")
		fmt.Println("  Daemon installed!")
		fmt.Println()
		fmt.Printf("  Host:      %s\n", host)
		fmt.Printf("  API Port:  8443\n")
		fmt.Println()
		fmt.Println("  API Key:")
		fmt.Printf("  %s\n", rawKey)
		fmt.Println()
		fmt.Println("  Add to .deploy.yml:")
		fmt.Printf("  server:\n")
		fmt.Printf("    host: %s\n", host)
		fmt.Printf("    api_port: 8443\n")
		fmt.Printf("    ssh_user: %s\n", sshUser)
		fmt.Printf("    api_key: %s\n", rawKey)
		fmt.Println("═══════════════════════════════════════════")
	},
}

func init() {
	rootCmd.AddCommand(initServerCmd)
	initServerCmd.Flags().String("host", "", "VPS hostname or IP (required)")
	initServerCmd.Flags().String("ssh-user", "root", "SSH user for initial setup")
	initServerCmd.Flags().String("app-name", "my-app", "Default app name")
	initServerCmd.Flags().String("deploy-path", "", "Deploy path on server (default: /opt/<app-name>)")
}
