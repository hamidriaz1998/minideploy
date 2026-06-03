package daemon

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hamid/minideploy/internal/shared"
)

type ProcessManager interface {
	Restart(ctx context.Context, serviceName string, instanceID string) error
	Start(ctx context.Context, serviceName string, instanceID string) error
	Stop(ctx context.Context, serviceName string, instanceID string) error
	Status(ctx context.Context, serviceName string, instanceID string) (*shared.ProcessStatus, error)
	Logs(ctx context.Context, serviceName string, instanceID string, lines int) (string, error)
}

func NewProcessManager(serviceType string) (ProcessManager, error) {
	switch serviceType {
	case "systemd":
		return &SystemdManager{}, nil
	case "pm2":
		return &PM2Manager{}, nil
	default:
		return nil, fmt.Errorf("unsupported service type: %s", serviceType)
	}
}

type SystemdManager struct{}

func serviceUnit(serviceName, instanceID string) string {
	return strings.ReplaceAll(serviceName, "%i", instanceID)
}

func (m *SystemdManager) systemctl(ctx context.Context, action, unit string) error {
	cmd := exec.CommandContext(ctx, "sudo", "systemctl", action, unit)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s %s: %s: %w", action, unit, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (m *SystemdManager) Restart(ctx context.Context, serviceName, instanceID string) error {
	unit := serviceUnit(serviceName, instanceID)
	return m.systemctl(ctx, "restart", unit)
}

func (m *SystemdManager) Start(ctx context.Context, serviceName, instanceID string) error {
	unit := serviceUnit(serviceName, instanceID)
	return m.systemctl(ctx, "start", unit)
}

func (m *SystemdManager) Stop(ctx context.Context, serviceName, instanceID string) error {
	unit := serviceUnit(serviceName, instanceID)
	return m.systemctl(ctx, "stop", unit)
}

func (m *SystemdManager) Status(ctx context.Context, serviceName, instanceID string) (*shared.ProcessStatus, error) {
	unit := serviceUnit(serviceName, instanceID)
	cmd := exec.CommandContext(ctx, "sudo", "systemctl", "show", unit, "--property=ActiveState,PID,ActiveEnterTimestamp")

	out, err := cmd.Output()
	if err != nil {
		return &shared.ProcessStatus{Running: false}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	ps := &shared.ProcessStatus{}

	for _, line := range lines {
		if strings.HasPrefix(line, "ActiveState=") {
			val := strings.TrimPrefix(line, "ActiveState=")
			ps.Running = val == "active"
		}
		if strings.HasPrefix(line, "PID=") {
			val := strings.TrimPrefix(line, "PID=")
			ps.PID, _ = strconv.Atoi(strings.TrimSpace(val))
		}
		if strings.HasPrefix(line, "ActiveEnterTimestamp=") {
			val := strings.TrimPrefix(line, "ActiveEnterTimestamp=")
			if val != "" {
				if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", val); err == nil {
					ps.Uptime = time.Since(t).Round(time.Second).String()
				}
			}
		}
	}

	return ps, nil
}

func (m *SystemdManager) Logs(ctx context.Context, serviceName, instanceID string, lines int) (string, error) {
	unit := serviceUnit(serviceName, instanceID)
	cmd := exec.CommandContext(ctx, "sudo", "journalctl", "-u", unit, "-n", fmt.Sprintf("%d", lines), "--no-pager")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("journalctl: %w", err)
	}
	return string(out), nil
}

type PM2Manager struct{}

func (m *PM2Manager) pm2(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "sudo", append([]string{"pm2"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pm2 %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return stdout.String(), nil
}

func (m *PM2Manager) Restart(ctx context.Context, serviceName, instanceID string) error {
	name := pm2Name(serviceName, instanceID)
	_, err := m.pm2(ctx, "restart", name)
	return err
}

func (m *PM2Manager) Start(ctx context.Context, serviceName, instanceID string) error {
	name := pm2Name(serviceName, instanceID)
	_, err := m.pm2(ctx, "start", name)
	return err
}

func (m *PM2Manager) Stop(ctx context.Context, serviceName, instanceID string) error {
	name := pm2Name(serviceName, instanceID)
	_, err := m.pm2(ctx, "stop", name)
	return err
}

func (m *PM2Manager) Status(ctx context.Context, serviceName, instanceID string) (*shared.ProcessStatus, error) {
	name := pm2Name(serviceName, instanceID)
	out, err := m.pm2(ctx, "jlist")
	if err != nil {
		return &shared.ProcessStatus{Running: false}, nil
	}

	if strings.Contains(out, fmt.Sprintf(`"name":"%s"`, name)) {
		return &shared.ProcessStatus{
			Running: true,
		}, nil
	}
	return &shared.ProcessStatus{Running: false}, nil
}

func (m *PM2Manager) Logs(ctx context.Context, serviceName, instanceID string, lines int) (string, error) {
	name := pm2Name(serviceName, instanceID)
	return m.pm2(ctx, "logs", name, "--lines", fmt.Sprintf("%d", lines), "--nostream")
}

func pm2Name(serviceName, instanceID string) string {
	return strings.ReplaceAll(serviceName, "%i", instanceID)
}
