package client

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

type Tunnel struct {
	cmd    *exec.Cmd
	local  int
	host   string
	remote int
}

func StartTunnel(host string, sshUser string, remotePort int, localPort int) (*Tunnel, error) {
	target := host
	if sshUser != "" {
		target = fmt.Sprintf("%s@%s", sshUser, host)
	}

	args := []string{
		"-N",
		"-L", fmt.Sprintf("%d:127.0.0.1:%d", localPort, remotePort),
		target,
	}

	cmd := exec.Command("ssh", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ssh tunnel: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return nil, fmt.Errorf("ssh tunnel exited immediately")
	}

	return &Tunnel{
		cmd:    cmd,
		local:  localPort,
		host:   host,
		remote: remotePort,
	}, nil
}

func (t *Tunnel) Close() error {
	if t.cmd != nil && t.cmd.Process != nil {
		return t.cmd.Process.Kill()
	}
	return nil
}

func NeedsTunnel(host string) bool {
	if isIP := func(h string) bool {
		for i, c := range h {
			if i == 0 && c == '-' {
				return false
			}
			if (c < '0' || c > '9') && c != '.' {
				return false
			}
		}
		return len(h) > 0
	}(host); isIP {
		return false
	}

	_, err := net.LookupHost(host)
	return err != nil
}
