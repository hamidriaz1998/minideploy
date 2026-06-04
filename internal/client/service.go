package client

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hamid/minideploy/internal/shared"
)

func EnsureServiceTemplate(cfg *Config) error {
	artifact := cfg.Artifacts[0]

	execPath := filepath.Join(cfg.DeployPath, "current", artifact)

	content := fmt.Sprintf(`[Unit]
Description=%%i - %s
After=network.target

[Service]
Type=simple
User=minideploy
Group=deploy
ExecStart=%s
Restart=always
RestartSec=5
WorkingDirectory=%s/current
EnvironmentFile=%s/.env.%%i

[Install]
WantedBy=multi-user.target
`, cfg.AppName, execPath, cfg.DeployPath, cfg.DeployPath)

	templatePath := fmt.Sprintf("/etc/systemd/system/%s@.service", cfg.AppName)

	shared.Info("ensuring systemd template %s...", templatePath)

	check := exec.Command("ssh", fmt.Sprintf("%s@%s", cfg.Server.SSHUser, cfg.Server.Host), fmt.Sprintf("test -f %s", templatePath))
	if check.Run() == nil {
		shared.Debug("template already exists, skipping")
		return nil
	}

	writeCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", cfg.Server.SSHUser, cfg.Server.Host),
		fmt.Sprintf("sudo tee %s > /dev/null", templatePath))
	stdin, _ := writeCmd.StdinPipe()
	writeCmd.Start()
	stdin.Write([]byte(content))
	stdin.Close()
	if err := writeCmd.Wait(); err != nil {
		return fmt.Errorf("write service template: %w", err)
	}

	reload := exec.Command("ssh", fmt.Sprintf("%s@%s", cfg.Server.SSHUser, cfg.Server.Host), "sudo systemctl daemon-reload")
	reload.Stdout = nil
	reload.Stderr = nil
	if err := reload.Run(); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}

	return nil
}

func EnsureEnvFiles(cfg *Config) error {
	shared.Info("ensuring instance env files...")

	globalEnv := cfg.Env
	if globalEnv == nil {
		globalEnv = make(map[string]string)
	}

	for _, inst := range cfg.Instances {
		envPath := fmt.Sprintf("%s/.env.%s", cfg.DeployPath, inst.ID)

		var lines []string
		for k, v := range globalEnv {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
		}
		for k, v := range inst.Env {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
		}

		content := strings.Join(lines, "\n") + "\n"

		check := exec.Command("ssh", fmt.Sprintf("%s@%s", cfg.Server.SSHUser, cfg.Server.Host),
			fmt.Sprintf("test -f %s", envPath))
		if check.Run() == nil {
			shared.Debug("env file %s already exists, skipping", envPath)
			continue
		}

		shared.Debug("writing %s...", envPath)
		writeCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", cfg.Server.SSHUser, cfg.Server.Host),
			fmt.Sprintf("sudo tee %s > /dev/null", envPath))
		stdin, _ := writeCmd.StdinPipe()
		writeCmd.Start()
		stdin.Write([]byte(content))
		stdin.Close()
		if err := writeCmd.Wait(); err != nil {
			return fmt.Errorf("write env file %s: %w", envPath, err)
		}
	}

	return nil
}
