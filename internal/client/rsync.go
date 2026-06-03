package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ArtifactsTotalSize(artifacts []string) string {
	var total int64
	for _, a := range artifacts {
		filepath.Walk(a, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				total += info.Size()
			}
			return nil
		})
	}
	return humanBytes(total)
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

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
		"--no-owner", "--no-group",
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
