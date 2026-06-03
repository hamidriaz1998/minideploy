package daemon

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"modernc.org/sqlite"

	"github.com/hamid/minideploy/internal/shared"
)

func openDB(stateDir string) (*sql.DB, error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	dbPath := filepath.Join(stateDir, "minideploy.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := migrateSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	if err := migrateFromJSON(stateDir, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate from json: %w", err)
	}

	return db, nil
}

func migrateSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS api_keys (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		key_hash   TEXT NOT NULL,
		scope      TEXT NOT NULL DEFAULT 'global',
		app_name   TEXT DEFAULT NULL,
		label      TEXT DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS apps (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		name         TEXT NOT NULL UNIQUE,
		service_type TEXT NOT NULL,
		service_name TEXT NOT NULL,
		deploy_path  TEXT NOT NULL,
		created_at   TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS instances (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		app_id      INTEGER NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
		instance_id TEXT NOT NULL,
		port        INTEGER NOT NULL,
		env_json    TEXT NOT NULL DEFAULT '{}'
	);

	CREATE TABLE IF NOT EXISTS releases (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		app_id      INTEGER NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		is_current  INTEGER NOT NULL DEFAULT 0
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	_, err := db.Exec("ALTER TABLE api_keys ADD COLUMN scope TEXT NOT NULL DEFAULT 'global'")
	if err != nil {
		// column already exists — ignore
	}
	_, err = db.Exec("ALTER TABLE api_keys ADD COLUMN app_name TEXT DEFAULT NULL")
	if err != nil {
	}
	_, err = db.Exec("ALTER TABLE api_keys ADD COLUMN label TEXT DEFAULT ''")
	if err != nil {
	}
	return nil
}

func migrateFromJSON(stateDir string, db *sql.DB) error {
	jsonPath := filepath.Join(stateDir, "state.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("read state.json: %w", err)
	}

	var old shared.DaemonState
	if err := json.Unmarshal(data, &old); err != nil {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES ('daemon_version', ?)", old.DaemonVersion); err != nil {
		return err
	}

	for _, k := range old.APIKeys {
		scope := k.Scope
		if scope == "" {
			scope = "global"
		}
		if _, err := tx.Exec(
			"INSERT INTO api_keys (key_hash, scope, app_name, label, created_at) VALUES (?, ?, ?, ?, ?)",
			k.KeyHash, scope, k.AppName, k.Label, k.CreatedAt.Format(time.RFC3339),
		); err != nil {
			return err
		}
	}

	for _, app := range old.Apps {
		res, err := tx.Exec(
			"INSERT INTO apps (name, service_type, service_name, deploy_path, created_at) VALUES (?, ?, ?, ?, ?)",
			app.Name, app.ServiceType, app.ServiceName, app.DeployPath, app.CreatedAt.Format(time.RFC3339),
		)
		if err != nil {
			return err
		}

		appID, _ := res.LastInsertId()

		for _, inst := range app.Instances {
			envBytes, _ := json.Marshal(inst.Env)
			if _, err := tx.Exec(
				"INSERT INTO instances (app_id, instance_id, port, env_json) VALUES (?, ?, ?, ?)",
				appID, inst.ID, inst.Port, string(envBytes),
			); err != nil {
				return err
			}
		}

		for _, rel := range app.Releases {
			isCurrent := 0
			if rel.Name == app.CurrentRelease {
				isCurrent = 1
			}
			if _, err := tx.Exec(
				"INSERT INTO releases (app_id, name, created_at, is_current) VALUES (?, ?, ?, ?)",
				appID, rel.Name, rel.CreatedAt.Format(time.RFC3339), isCurrent,
			); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	os.Rename(jsonPath, jsonPath+".migrated")
	return nil
}

type dbQueries struct {
	db *sql.DB
}

func newDBQueries(db *sql.DB) *dbQueries {
	return &dbQueries{db: db}
}

func (q *dbQueries) GetMeta(key string) (string, error) {
	var val string
	err := q.db.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (q *dbQueries) SetMeta(key, value string) error {
	_, err := q.db.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)", key, value)
	return err
}

func (q *dbQueries) GetApp(name string) (*shared.AppState, bool, error) {
	row := q.db.QueryRow("SELECT id, name, service_type, service_name, deploy_path, created_at FROM apps WHERE name = ?", name)
	var app shared.AppState
	var id int64
	var createdAt string
	if err := row.Scan(&id, &app.Name, &app.ServiceType, &app.ServiceName, &app.DeployPath, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	app.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	instRows, err := q.db.Query("SELECT instance_id, port, env_json FROM instances WHERE app_id = ?", id)
	if err != nil {
		return nil, false, err
	}
	defer instRows.Close()

	for instRows.Next() {
		var inst shared.Instance
		var envStr string
		if err := instRows.Scan(&inst.ID, &inst.Port, &envStr); err != nil {
			return nil, false, err
		}
		json.Unmarshal([]byte(envStr), &inst.Env)
		if inst.Env == nil {
			inst.Env = make(map[string]string)
		}
		app.Instances = append(app.Instances, inst)
	}

	relRows, err := q.db.Query("SELECT name, created_at, is_current FROM releases WHERE app_id = ? ORDER BY created_at", id)
	if err != nil {
		return nil, false, err
	}
	defer relRows.Close()

	for relRows.Next() {
		var rel shared.Release
		var relCreated string
		var isCurrent int
		if err := relRows.Scan(&rel.Name, &relCreated, &isCurrent); err != nil {
			return nil, false, err
		}
		rel.CreatedAt, _ = time.Parse(time.RFC3339, relCreated)
		rel.IsCurrent = isCurrent == 1
		if rel.IsCurrent {
			app.CurrentRelease = rel.Name
		}
		app.Releases = append(app.Releases, rel)
	}

	return &app, true, nil
}

func (q *dbQueries) ListApps() ([]shared.AppSummary, error) {
	rows, err := q.db.Query(`
		SELECT a.name, a.service_type,
			COALESCE((SELECT r.name FROM releases r WHERE r.app_id = a.id AND r.is_current = 1 LIMIT 1), '') AS current_release,
			(SELECT COUNT(*) FROM instances i WHERE i.app_id = a.id) AS instance_count
		FROM apps a
		ORDER BY a.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []shared.AppSummary
	for rows.Next() {
		var app shared.AppSummary
		if err := rows.Scan(&app.Name, &app.ServiceType, &app.CurrentRelease, &app.InstancesCount); err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func (q *dbQueries) RegisterApp(req *shared.DeployRequest) (*shared.AppState, error) {
	existing, found, err := q.GetApp(req.AppName)
	if err != nil {
		return nil, err
	}

	tx, err := q.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if found {
		if req.ServiceType != "" {
			existing.ServiceType = req.ServiceType
		}
		if req.ServiceName != "" {
			existing.ServiceName = req.ServiceName
		}
		if req.DeployPath != "" {
			existing.DeployPath = req.DeployPath
		}

		if _, err := tx.Exec(
			"UPDATE apps SET service_type = ?, service_name = ?, deploy_path = ? WHERE name = ?",
			existing.ServiceType, existing.ServiceName, existing.DeployPath, req.AppName,
		); err != nil {
			return nil, err
		}

		if req.Instances != nil {
			var appID int64
			tx.QueryRow("SELECT id FROM apps WHERE name = ?", req.AppName).Scan(&appID)
			tx.Exec("DELETE FROM instances WHERE app_id = ?", appID)
			for _, inst := range req.Instances {
				if err := insertInstance(tx, appID, inst); err != nil {
					return nil, err
				}
			}
			existing.Instances = req.Instances
		}

		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}

	res, err := tx.Exec(
		"INSERT INTO apps (name, service_type, service_name, deploy_path) VALUES (?, ?, ?, ?)",
		req.AppName, req.ServiceType, req.ServiceName, req.DeployPath,
	)
	if err != nil {
		return nil, err
	}

	appID, _ := res.LastInsertId()

	for _, inst := range req.Instances {
		if err := insertInstance(tx, appID, inst); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
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
	return app, nil
}

func insertInstance(tx *sql.Tx, appID int64, inst shared.Instance) error {
	envBytes, _ := json.Marshal(inst.Env)
	if inst.Env == nil {
		envBytes = []byte("{}")
	}
	_, err := tx.Exec(
		"INSERT INTO instances (app_id, instance_id, port, env_json) VALUES (?, ?, ?, ?)",
		appID, inst.ID, inst.Port, string(envBytes),
	)
	return err
}

func (q *dbQueries) AddRelease(appName string, rel shared.Release) error {
	tx, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var appID int64
	if err := tx.QueryRow("SELECT id FROM apps WHERE name = ?", appName).Scan(&appID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("app %q not found", appName)
		}
		return err
	}

	tx.Exec("UPDATE releases SET is_current = 0 WHERE app_id = ? AND is_current = 1", appID)

	_, err = tx.Exec(
		"INSERT INTO releases (app_id, name, created_at, is_current) VALUES (?, ?, ?, 1)",
		appID, rel.Name, rel.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (q *dbQueries) SetCurrentRelease(appName, releaseName string) error {
	tx, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var appID int64
	if err := tx.QueryRow("SELECT id FROM apps WHERE name = ?", appName).Scan(&appID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("app %q not found", appName)
		}
		return err
	}

	var count int
	tx.QueryRow("SELECT COUNT(*) FROM releases WHERE app_id = ? AND name = ?", appID, releaseName).Scan(&count)
	if count == 0 {
		return fmt.Errorf("release %q not found for app %q", releaseName, appName)
	}

	tx.Exec("UPDATE releases SET is_current = 0 WHERE app_id = ?", appID)
	tx.Exec("UPDATE releases SET is_current = 1 WHERE app_id = ? AND name = ?", appID, releaseName)

	return tx.Commit()
}

func (q *dbQueries) RemoveApp(appName string) error {
	res, err := q.db.Exec("DELETE FROM apps WHERE name = ?", appName)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("app %q not found", appName)
	}
	return nil
}

func (q *dbQueries) GetAPIKeys() ([]shared.APIKeyEntry, error) {
	rows, err := q.db.Query("SELECT id, key_hash, scope, COALESCE(app_name,''), COALESCE(label,''), created_at FROM api_keys ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []shared.APIKeyEntry
	for rows.Next() {
		var k shared.APIKeyEntry
		var created string
		if err := rows.Scan(&k.ID, &k.KeyHash, &k.Scope, &k.AppName, &k.Label, &created); err != nil {
			return nil, err
		}
		k.CreatedAt, _ = time.Parse(time.RFC3339, created)
		keys = append(keys, k)
	}
	return keys, nil
}

func (q *dbQueries) AddAPIKey(hash, scope, appName, label string) error {
	created := time.Now().Format(time.RFC3339)
	_, err := q.db.Exec(
		"INSERT INTO api_keys (key_hash, scope, app_name, label, created_at) VALUES (?, ?, ?, ?, ?)",
		hash, scope, appName, label, created,
	)
	return err
}

func (q *dbQueries) DeleteAPIKey(id int) error {
	res, err := q.db.Exec("DELETE FROM api_keys WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("key id %d not found", id)
	}
	return nil
}

func (q *dbQueries) RotateKey(hash string, revokeOld bool) (int, error) {
	tx, err := q.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO api_keys (key_hash, scope, label, created_at) VALUES (?, 'global', 'rotated', ?)",
		hash, time.Now().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}

	if revokeOld {
		var latestID int64
		err = tx.QueryRow("SELECT MAX(id) FROM api_keys").Scan(&latestID)
		if err != nil {
			return 0, err
		}
		_, err = tx.Exec("DELETE FROM api_keys WHERE id < ?", latestID)
		if err != nil {
			return 0, err
		}
	}

	var count int
	tx.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&count)

	return count, tx.Commit()
}

func (q *dbQueries) Close() error {
	return q.db.Close()
}

var _ = sqlite.Driver{}
