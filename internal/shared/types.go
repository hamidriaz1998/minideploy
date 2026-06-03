package shared

import "time"

type Config struct {
	AppName     string            `yaml:"app_name"`
	ServiceType string            `yaml:"service_type"`
	ServiceName string            `yaml:"service_name"`
	Instances   []Instance        `yaml:"instances"`
	DeployPath  string            `yaml:"deploy_path"`
	Build       []string          `yaml:"build"`
	Artifacts   []string          `yaml:"artifacts"`
	Server      ServerConfig      `yaml:"server"`
	Env         map[string]string `yaml:"env"`
	PreDeploy   []Hook            `yaml:"pre_deploy"`
	PostDeploy  []Hook            `yaml:"post_deploy"`
}

type Instance struct {
	ID   string            `yaml:"id"`
	Port int               `yaml:"port"`
	Env  map[string]string `yaml:"env"`
}

type ServerConfig struct {
	Host    string `yaml:"host"`
	APIPort int    `yaml:"api_port"`
	SSHUser string `yaml:"ssh_user"`
	APIKey  string `yaml:"api_key"`
}

type Hook struct {
	Cmd string `yaml:"cmd"`
}

type APIEnvelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type CreateKeyRequest struct {
	Scope   string `json:"scope"`
	AppName string `json:"app_name,omitempty"`
	Label   string `json:"label,omitempty"`
}

type CreateKeyResponse struct {
	ID      int    `json:"id"`
	RawKey  string `json:"raw_key"`
	Scope   string `json:"scope"`
	AppName string `json:"app_name,omitempty"`
	Label   string `json:"label,omitempty"`
}

type DeleteKeyRequest struct {
	ID int `json:"id"`
}

type DeleteKeyResponse struct {
	Deleted bool `json:"deleted"`
}

type KeyInfo struct {
	ID        int       `json:"id"`
	Scope     string    `json:"scope"`
	AppName   string    `json:"app_name,omitempty"`
	Label     string    `json:"label,omitempty"`
	HashHint  string    `json:"hash_hint"`
	CreatedAt time.Time `json:"created_at"`
}

type RotateKeyRequest struct {
	RevokeOld bool `json:"revoke_old"`
}

type RotateKeyResponse struct {
	NewKey    string `json:"new_key"`
	KeysCount int    `json:"keys_count"`
}

type DestroyRequest struct {
	AppName string `json:"app_name"`
	Soft    bool   `json:"soft"`
	Confirm bool   `json:"confirm"`
}

type DestroyResponse struct {
	AppName string `json:"app_name"`
	Soft    bool   `json:"soft"`
}

type DeployRequest struct {
	AppName     string     `json:"app_name"`
	ReleaseName string     `json:"release_name,omitempty"`
	ServiceType string     `json:"service_type,omitempty"`
	ServiceName string     `json:"service_name,omitempty"`
	Instances   []Instance `json:"instances,omitempty"`
	DeployPath  string     `json:"deploy_path,omitempty"`
}

type DeployResponse struct {
	Release    string   `json:"release"`
	Instances  []string `json:"instances"`
	AppName    string   `json:"app_name"`
}

type RollbackRequest struct {
	AppName     string `json:"app_name"`
	ReleaseName string `json:"release_name,omitempty"`
}

type RollbackResponse struct {
	Release   string   `json:"release"`
	Instances []string `json:"instances"`
}

type StatusResponse struct {
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
	StartTime time.Time `json:"start_time"`
	AppsCount int       `json:"apps_count"`
	DiskUsage DiskUsage `json:"disk_usage"`
}

type DiskUsage struct {
	Total     int64 `json:"total"`
	Used      int64 `json:"used"`
	Available int64 `json:"available"`
}

type AppSummary struct {
	Name            string `json:"name"`
	ServiceType     string `json:"service_type"`
	CurrentRelease  string `json:"current_release"`
	InstancesCount  int    `json:"instances_count"`
	Running         bool   `json:"running"`
}

type AppDetail struct {
	Name           string     `json:"name"`
	ServiceType    string     `json:"service_type"`
	ServiceName    string     `json:"service_name"`
	DeployPath     string     `json:"deploy_path"`
	Instances      []Instance `json:"instances"`
	CurrentRelease string     `json:"current_release"`
	Releases       []Release  `json:"releases"`
}

type Release struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	IsCurrent bool      `json:"is_current"`
}

type AppStatus struct {
	AppName        string           `json:"app_name"`
	CurrentRelease string           `json:"current_release"`
	Instances      []InstanceStatus `json:"instances"`
}

type InstanceStatus struct {
	ID      string `json:"id"`
	Port    int    `json:"port"`
	Running bool   `json:"running"`
	PID     int    `json:"pid"`
}

type ProcessStatus struct {
	Running bool
	PID     int
	Uptime  string
	Memory  string
}

type AppState struct {
	Name           string     `json:"name"`
	ServiceType    string     `json:"service_type"`
	ServiceName    string     `json:"service_name"`
	DeployPath     string     `json:"deploy_path"`
	Instances      []Instance `json:"instances"`
	CurrentRelease string     `json:"current_release"`
	Releases       []Release  `json:"releases"`
	CreatedAt      time.Time  `json:"created_at"`
}

type DaemonState struct {
	DaemonVersion string               `json:"daemon_version"`
	Apps          map[string]*AppState `json:"apps"`
	APIKeys       []APIKeyEntry        `json:"api_keys"`
}

type APIKeyEntry struct {
	ID        int       `json:"id"`
	KeyHash   string    `json:"key_hash"`
	Scope     string    `json:"scope"`
	AppName   string    `json:"app_name,omitempty"`
	Label     string    `json:"label,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
