package daemon

import (
	"database/sql"
	"fmt"

	"github.com/hamid/minideploy/internal/shared"
)

var DefaultStateDir = "/var/lib/minideploy"

type StateManager struct {
	db  *sql.DB
	q   *dbQueries
}

func NewStateManager(stateDir string) (*StateManager, error) {
	if stateDir == "" {
		stateDir = DefaultStateDir
	}

	db, err := openDB(stateDir)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	q := newDBQueries(db)

	if err := q.SetMeta("daemon_version", Version); err != nil {
		db.Close()
		return nil, fmt.Errorf("set version: %w", err)
	}

	sm := &StateManager{db: db, q: q}

	if len(sm.getAPIKeys()) == 0 {
		raw, hash, err := GenerateAPIKey()
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("generate initial api key: %w", err)
		}
		if err := sm.AddAPIKey(hash, "global", "", "initial key"); err != nil {
			db.Close()
			return nil, fmt.Errorf("store initial api key: %w", err)
		}
		fmt.Printf("!!! No API key configured. Generated one-time key:\n")
		fmt.Printf("!!!   %s\n", raw)
		fmt.Printf("!!! Set this as MINIDEPLOY_API_KEY or in .deploy.yml server.api_key\n")
	}

	return sm, nil
}

func (sm *StateManager) getAPIKeys() []shared.APIKeyEntry {
	keys, _ := sm.q.GetAPIKeys()
	return keys
}

func (sm *StateManager) GetApp(name string) (*shared.AppState, bool) {
	app, found, err := sm.q.GetApp(name)
	if err != nil || !found {
		return nil, false
	}
	return app, true
}

func (sm *StateManager) ListApps() []shared.AppSummary {
	apps, err := sm.q.ListApps()
	if err != nil {
		return []shared.AppSummary{}
	}
	return apps
}

func (sm *StateManager) RegisterApp(req *shared.DeployRequest) (*shared.AppState, error) {
	return sm.q.RegisterApp(req)
}

func (sm *StateManager) AddRelease(appName string, r shared.Release) error {
	return sm.q.AddRelease(appName, r)
}

func (sm *StateManager) DeleteRelease(appName, releaseName string) error {
	return sm.q.DeleteRelease(appName, releaseName)
}

func (sm *StateManager) SetCurrentRelease(appName, releaseName string) error {
	return sm.q.SetCurrentRelease(appName, releaseName)
}

func (sm *StateManager) RemoveApp(appName string) error {
	return sm.q.RemoveApp(appName)
}

func (sm *StateManager) AddAPIKey(hash, scope, appName, label string) error {
	return sm.q.AddAPIKey(hash, scope, appName, label)
}

func (sm *StateManager) DeleteAPIKey(id int) error {
	return sm.q.DeleteAPIKey(id)
}

func (sm *StateManager) GetKeyByID(id int) (*shared.APIKeyEntry, error) {
	keys, err := sm.q.GetAPIKeys()
	if err != nil {
		return nil, err
	}
	for _, k := range keys {
		if k.ID == id {
			return &k, nil
		}
	}
	return nil, fmt.Errorf("key id %d not found", id)
}

func (sm *StateManager) GetAPIKeys() []shared.APIKeyEntry {
	return sm.getAPIKeys()
}

func (sm *StateManager) RotateKey(hash string, revokeOld bool) (int, error) {
	return sm.q.RotateKey(hash, revokeOld)
}

func (sm *StateManager) Close() error {
	return sm.db.Close()
}

func SeedKey(stateDir, hash string) error {
	db, err := openDB(stateDir)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	q := newDBQueries(db)
	return q.AddAPIKey(hash, "global", "", "init-server")
}
