package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hamid/minideploy/internal/shared"
)

type Config = shared.Config

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	resolveAPIKey(&cfg)

	return &cfg, nil
}

func FindConfig() (string, error) {
	candidates := []string{".deploy.yml", "deploy.yml", ".deploy.yaml", "deploy.yaml"}
	for _, name := range candidates {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no deploy config found (looked for: %s)", strings.Join(candidates, ", "))
}

var ConfigDir = func() string {
	if d := os.Getenv("MINIDEPLOY_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "minideploy")
}

func resolveAPIKey(cfg *Config) {
	if cfg.Server.APIKey != "" {
		return
	}
	if key := os.Getenv("MINIDEPLOY_API_KEY"); key != "" {
		cfg.Server.APIKey = key
		return
	}
	if key := readEnvFile(".env"); key != "" {
		cfg.Server.APIKey = key
		return
	}
}

func readEnvFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MINIDEPLOY_API_KEY=") {
			return strings.TrimPrefix(line, "MINIDEPLOY_API_KEY=")
		}
	}
	return ""
}

func validate(cfg *Config) error {
	if cfg.AppName == "" {
		return fmt.Errorf("app_name is required")
	}
	if cfg.ServiceType == "" {
		return fmt.Errorf("service_type is required (systemd or pm2)")
	}
	if cfg.ServiceType != "systemd" && cfg.ServiceType != "pm2" {
		return fmt.Errorf("service_type must be 'systemd' or 'pm2', got %q", cfg.ServiceType)
	}
	if cfg.ServiceName == "" {
		return fmt.Errorf("service_name is required")
	}
	if cfg.DeployPath == "" {
		return fmt.Errorf("deploy_path is required")
	}
	if cfg.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}
	if cfg.Server.APIPort == 0 {
		cfg.Server.APIPort = 8443
	}
	if len(cfg.Build) == 0 {
		return fmt.Errorf("at least one build step is required")
	}
	if len(cfg.Artifacts) == 0 {
		return fmt.Errorf("at least one artifact is required")
	}
	return nil
}
