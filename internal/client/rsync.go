package client

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type RsyncConfig struct {
	SSHUser   string
	Host      string
	DeployDir string
	Artifacts []string
}

func RunRsync(cfg RsyncConfig) error {
	dest := fmt.Sprintf("%s@%s:%s/upload/", cfg.SSHUser, cfg.Host, cfg.DeployDir)

	args := []string{
		"-avz", "--delete",
	}

	args = append(args, cfg.Artifacts...)
	args = append(args, dest)

	cmdStr := fmt.Sprintf("rsync %s", strings.Join(args, " "))
	fmt.Fprintf(os.Stdout, "[rsync] %s\n", cmdStr)

	cmd := exec.Command("rsync", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	return nil
}

func RsyncDestination(sshUser, host, deployPath string) string {
	return fmt.Sprintf("%s@%s:%s/upload/", sshUser, host, deployPath)
}
