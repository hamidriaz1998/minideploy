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

type HostConfig struct {
	AdminKey string `yaml:"admin_key"`
}

type GlobalConfig struct {
	AdminKey string
	Hosts    map[string]HostConfig
}

func (cfg *GlobalConfig) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	if cfg.AdminKey != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "admin_key"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: cfg.AdminKey},
		)
	}
	for host, hc := range cfg.Hosts {
		hcNode := &yaml.Node{Kind: yaml.MappingNode}
		hcNode.Content = append(hcNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "admin_key"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: hc.AdminKey},
		)
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: host},
			hcNode,
		)
	}
	return node, nil
}

func (cfg *GlobalConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node")
	}
	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i].Value
		valNode := value.Content[i+1]
		if key == "admin_key" {
			cfg.AdminKey = valNode.Value
			continue
		}
		var hc HostConfig
		if err := valNode.Decode(&hc); err != nil {
			return err
		}
		if cfg.Hosts == nil {
			cfg.Hosts = make(map[string]HostConfig)
		}
		cfg.Hosts[key] = hc
	}
	return nil
}

func GlobalConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yml")
}

func LoadGlobalConfig() (*GlobalConfig, error) {
	path := GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{}, nil
		}
		return nil, fmt.Errorf("read global config: %w", err)
	}

	var cfg GlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse global config: %w", err)
	}
	return &cfg, nil
}

func SaveGlobalConfig(cfg *GlobalConfig) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal global config: %w", err)
	}

	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write global config: %w", err)
	}
	return nil
}

func GetAdminKey(host string) string {
	cfg, err := LoadGlobalConfig()
	if err != nil {
		return ""
	}
	if host != "" && cfg.Hosts != nil {
		if hc, ok := cfg.Hosts[host]; ok && hc.AdminKey != "" {
			return hc.AdminKey
		}
	}
	if cfg.AdminKey != "" {
		return cfg.AdminKey
	}
	return os.Getenv("MINIDEPLOY_ADMIN_KEY")
}

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
	if key := GetAdminKey(cfg.Server.Host); key != "" {
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
	if cfg.HealthCheck.Endpoint != "" {
		if cfg.HealthCheck.Timeout <= 0 {
			return fmt.Errorf("health_check.timeout must be positive")
		}
		if cfg.HealthCheck.Retries <= 0 {
			return fmt.Errorf("health_check.retries must be positive")
		}
		if cfg.HealthCheck.WaitBetweenInstances < 0 {
			return fmt.Errorf("health_check.wait_between_instances cannot be negative")
		}
	}
	if cfg.KeepReleases < 0 {
		return fmt.Errorf("keep_releases cannot be negative")
	}
	return nil
}
