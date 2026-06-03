package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hamid/minideploy/internal/shared"
)

var DefaultStateDir = "/var/lib/minideploy"

type StateManager struct {
	mu       sync.RWMutex
	state    *shared.DaemonState
	filePath string
}

func NewStateManager(stateDir string) (*StateManager, error) {
	if stateDir == "" {
		stateDir = DefaultStateDir
	}
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	path := filepath.Join(stateDir, "state.json")
	sm := &StateManager{filePath: path}

	if data, err := os.ReadFile(path); err == nil {
		var s shared.DaemonState
		if err := json.Unmarshal(data, &s); err == nil {
			sm.state = &s
			return sm, nil
		}
	}

	sm.state = &shared.DaemonState{
		DaemonVersion: Version,
		Apps:          make(map[string]*shared.AppState),
		APIKeys:       []shared.APIKeyEntry{},
	}
	if err := sm.save(); err != nil {
		return nil, fmt.Errorf("init state: %w", err)
	}
	return sm, nil
}

func (sm *StateManager) save() error {
	sm.state.DaemonVersion = Version
	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	tmp := sm.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write state tmp: %w", err)
	}
	if err := os.Rename(tmp, sm.filePath); err != nil {
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}

func (sm *StateManager) GetApp(name string) (*shared.AppState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	app, ok := sm.state.Apps[name]
	return app, ok
}

func (sm *StateManager) ListApps() []shared.AppSummary {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var apps []shared.AppSummary
	for _, a := range sm.state.Apps {
		apps = append(apps, shared.AppSummary{
			Name:           a.Name,
			ServiceType:    a.ServiceType,
			CurrentRelease: a.CurrentRelease,
			InstancesCount: len(a.Instances),
		})
	}
	return apps
}

func (sm *StateManager) RegisterApp(req *shared.DeployRequest) *shared.AppState {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if existing, ok := sm.state.Apps[req.AppName]; ok {
		if req.ServiceType != "" {
			existing.ServiceType = req.ServiceType
		}
		if req.ServiceName != "" {
			existing.ServiceName = req.ServiceName
		}
		if req.DeployPath != "" {
			existing.DeployPath = req.DeployPath
		}
		if req.Instances != nil {
			existing.Instances = req.Instances
		}
		return existing
	}

	app := &shared.AppState{
		Name:        req.AppName,
		ServiceType: req.ServiceType,
		ServiceName: req.ServiceName,
		DeployPath:  req.DeployPath,
		Instances:   req.Instances,
		Releases:    []shared.Release{},
		CreatedAt:   time.Now(),
	}
	sm.state.Apps[req.AppName] = app
	return app
}

func (sm *StateManager) AddRelease(appName string, r shared.Release) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	app, ok := sm.state.Apps[appName]
	if !ok {
		return fmt.Errorf("app %q not found", appName)
	}

	app.CurrentRelease = r.Name
	app.Releases = append(app.Releases, r)
	return sm.save()
}

func (sm *StateManager) SetCurrentRelease(appName, releaseName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	app, ok := sm.state.Apps[appName]
	if !ok {
		return fmt.Errorf("app %q not found", appName)
	}

	found := false
	for i := range app.Releases {
		if app.Releases[i].Name == releaseName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("release %q not found for app %q", releaseName, appName)
	}

	app.CurrentRelease = releaseName
	return sm.save()
}

func (sm *StateManager) AddAPIKey(hash string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.state.APIKeys = append(sm.state.APIKeys, shared.APIKeyEntry{
		KeyHash:   hash,
		CreatedAt: time.Now(),
	})
	return sm.save()
}

func (sm *StateManager) GetAPIKeys() []shared.APIKeyEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	keys := make([]shared.APIKeyEntry, len(sm.state.APIKeys))
	copy(keys, sm.state.APIKeys)
	return keys
}
