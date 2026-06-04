package client

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
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

	start := time.Now()
	timeout := 10 * time.Second
	for time.Since(start) < timeout {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return nil, fmt.Errorf("ssh tunnel exited immediately")
		}
		if IsPortOpen("127.0.0.1", localPort) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !IsPortOpen("127.0.0.1", localPort) {
		cmd.Process.Kill()
		return nil, fmt.Errorf("ssh tunnel did not become ready within %v", timeout)
	}

	return &Tunnel{
		cmd:    cmd,
		local:  localPort,
		host:   host,
		remote: remotePort,
	}, nil
}

func IsPortOpen(host string, port int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
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
