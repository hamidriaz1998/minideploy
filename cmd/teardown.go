package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var teardownCmd = &cobra.Command{
	Use:   "teardown",
	Short: "Remove minideploy and optionally its managed apps from a server",
	Long: `Completely removes minideploy from a server. Optionally removes
none, some, or all deployed apps as well.

Uses SSH to execute all removal commands. The daemon does not need to
be running for this to work.

Examples:
  # Remove everything (minideploy + all apps)
  minideploy teardown --host my-vps

  # Remove only specific apps along with minideploy
  minideploy teardown --host my-vps --apps "my-api,express-api"

  # Remove only minideploy, leave apps untouched
  minideploy teardown --host my-vps --apps ""`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		sshUser, _ := cmd.Flags().GetString("ssh-user")
		appsFlag, _ := cmd.Flags().GetString("apps")

		if sshUser == "" {
			sshUser = "root"
		}

		if cfg, err := client.LoadConfig(".deploy.yml"); err == nil {
			if host == "" && cfg.Server.Host != "" {
				host = cfg.Server.Host
			}
			if sshUser == "root" && cfg.Server.SSHUser != "" {
				sshUser = cfg.Server.SSHUser
			}
		}

		if host == "" {
			shared.Fatal("--host is required (or add server.host to .deploy.yml)")
		}

		target := fmt.Sprintf("%s@%s", sshUser, host)

		shared.Info("discovering minideploy installation on %s...", host)

		discoveredApps := sshDiscoverApps(target)
		minideployInstalled := sshRun(target, "command -v minideploy") == nil
		daemonServiceExists := sshRun(target, "ls /etc/systemd/system/minideploy.service") == nil

		if minideployInstalled {
			shared.Info("minideploy binary is installed")
		}
		if daemonServiceExists {
			shared.Info("minideploy systemd service exists")
		}
		if len(discoveredApps) > 0 {
			shared.Info("discovered apps: %s", strings.Join(discoveredApps, ", "))
		} else {
			shared.Info("no minideploy-managed apps discovered")
		}

		if !minideployInstalled && !daemonServiceExists && len(discoveredApps) == 0 {
			shared.Fatal("nothing found to teardown on %s", host)
		}

		var appsToRemove []string
		switch appsFlag {
		case "all":
			appsToRemove = discoveredApps
		case "":
			appsToRemove = nil
		default:
			for _, name := range strings.Split(appsFlag, ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					appsToRemove = append(appsToRemove, name)
				}
			}
		}

		fmt.Println()
		fmt.Println("═══════════════════════════════════════════")
		fmt.Printf("  Host:              %s\n", host)
		fmt.Printf("  SSH User:          %s\n", sshUser)
		fmt.Printf("  Remove minideploy: yes\n")
		if len(appsToRemove) > 0 {
			fmt.Printf("  Remove apps:       %d\n", len(appsToRemove))
			for _, a := range appsToRemove {
				fmt.Printf("    - %s\n", a)
			}
		} else {
			fmt.Printf("  Remove apps:       none\n")
		}
		fmt.Println("═══════════════════════════════════════════")
		fmt.Println()

		fmt.Print("Continue? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			shared.Info("teardown cancelled")
			return
		}

		for _, appName := range appsToRemove {
			shared.Info("removing app %q...", appName)
			sshRemoveApp(target, appName)
		}

		shared.Info("removing minideploy...")
		sshRemoveMinideploy(target)

		shared.Success("teardown complete for %s", host)
	},
}

func init() {
	rootCmd.AddCommand(teardownCmd)
	teardownCmd.Flags().String("host", "", "Server hostname or IP (default: from .deploy.yml)")
	teardownCmd.Flags().String("ssh-user", "root", "SSH user")
	teardownCmd.Flags().String("apps", "all", `Apps to remove. Use "all" (default), a comma-separated list, or empty string to keep apps`)
}

func sshRun(target, command string) error {
	ssh := exec.Command("ssh", target, command)
	ssh.Stdout = os.Stdout
	ssh.Stderr = os.Stderr
	return ssh.Run()
}

func sshOutput(target, command string) (string, error) {
	ssh := exec.Command("ssh", target, command)
	out, err := ssh.Output()
	return strings.TrimSpace(string(out)), err
}

func sshDiscoverApps(target string) []string {
	script := `
apps=""
if command -v sqlite3 >/dev/null 2>&1 && [ -f /var/lib/minideploy/minideploy.db ]; then
	apps=$(sudo sqlite3 /var/lib/minideploy/minideploy.db "SELECT name FROM apps" 2>/dev/null)
fi
if [ -z "$apps" ]; then
	for f in /etc/systemd/system/*@.service; do
		[ -f "$f" ] || continue
		name=$(basename "$f" @.service)
		[ -d "/opt/$name/releases" ] || [ -d "/opt/$name/upload" ] || continue
		echo "$name"
	done
else
	echo "$apps"
fi
`
	out, err := sshOutput(target, script)
	if err != nil {
		return nil
	}
	var apps []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			apps = append(apps, line)
		}
	}
	return apps
}

func sshRemoveApp(target, appName string) {
	shared.Debug("stopping and disabling %s instances...", appName)
	instances := fmt.Sprintf(`
for unit in $(sudo systemctl list-units --type=service --all --no-legend 2>/dev/null | awk '/%s@/ {print $1}'); do
	sudo systemctl stop "$unit" 2>/dev/null
	sudo systemctl disable "$unit" 2>/dev/null
done
`, appName)
	sshRun(target, instances)

	shared.Debug("removing %s systemd service template...", appName)
	sshRun(target, fmt.Sprintf("sudo rm -f /etc/systemd/system/%s@.service", appName))

	shared.Debug("removing /opt/%s directory...", appName)
	sshRun(target, fmt.Sprintf("sudo rm -rf /opt/%s", appName))

	sshRun(target, "sudo systemctl daemon-reload")
}

func sshRemoveMinideploy(target string) {
	shared.Debug("stopping and disabling minideploy daemon...")
	sshRun(target, "sudo systemctl stop minideploy 2>/dev/null || true")
	sshRun(target, "sudo systemctl disable minideploy 2>/dev/null || true")

	shared.Debug("removing minideploy systemd service...")
	sshRun(target, "sudo rm -f /etc/systemd/system/minideploy.service")

	shared.Debug("removing minideploy binary...")
	sshRun(target, "sudo rm -f /usr/local/bin/minideploy")

	shared.Debug("removing minideploy state directory...")
	sshRun(target, "sudo rm -rf /var/lib/minideploy")

	shared.Debug("removing sudoers...")
	sshRun(target, "sudo rm -f /etc/sudoers.d/minideploy")

	shared.Debug("removing minideploy user and deploy group...")
	sshRun(target, "sudo userdel minideploy 2>/dev/null || true")
	sshRun(target, "sudo groupdel deploy 2>/dev/null || true")

	shared.Debug("reloading systemd...")
	sshRun(target, "sudo systemctl daemon-reload")

	shared.Debug("teardown commands sent to %s", target)
}
