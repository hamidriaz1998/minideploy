package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hamid/minideploy/internal/shared"
)

type Handler struct {
	state *StateManager
}

func NewHandler(state *StateManager) *Handler {
	return &Handler{state: state}
}

func (h *Handler) HandleDeploy(w http.ResponseWriter, r *http.Request) {
	var req shared.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AppName == "" {
		writeError(w, http.StatusBadRequest, "app_name is required")
		return
	}

	if !authorizeApp(r, req.AppName) {
		writeError(w, http.StatusForbidden, "this key is not authorized for this app")
		return
	}

	if req.ReleaseName == "" {
		req.ReleaseName = GenerateReleaseName()
	}

	app, err := h.state.RegisterApp(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("register app: %v", err))
		return
	}

	if err := EnsureDeployDir(app.DeployPath); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := SnapshotRelease(app.DeployPath, req.ReleaseName); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("snapshot failed: %v", err))
		return
	}

	if req.KeepReleases > 0 {
		releases := app.Releases
		pruned, err := PruneReleases(app.DeployPath, releases, req.KeepReleases)
		if err == nil {
			for _, name := range pruned {
				h.state.DeleteRelease(app.Name, name)
			}
		}
	}

	previous, err := UpdateSymlink(app.DeployPath, req.ReleaseName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("symlink failed: %v", err))
		return
	}

	pm, err := NewProcessManager(app.ServiceType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var restarted []string
	var failed []string
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	for _, inst := range app.Instances {
		unitName := strings.ReplaceAll(app.ServiceName, "%i", inst.ID)
		if err := pm.Restart(ctx, app.ServiceName, inst.ID); err != nil {
			failed = append(failed, unitName)
			continue
		}
		restarted = append(restarted, unitName)
	}

	release := MakeRelease(req.ReleaseName, app)

	var healthResults []shared.HealthResult
	rolledBack := false
	rolledBackTo := ""

	if len(failed) == 0 && req.HealthCheck.Endpoint != "" && len(app.Instances) > 0 {
		healthResults = CheckHealth(app.Instances, req.HealthCheck)
		for _, hr := range healthResults {
			if !hr.Passed {
				rolledBack = true
				break
			}
		}
		if rolledBack {
			if previous != "" {
				UpdateSymlink(app.DeployPath, previous)
				pm, _ = NewProcessManager(app.ServiceType)
				ctx2, cancel2 := context.WithTimeout(r.Context(), 30*time.Second)
				for _, inst := range app.Instances {
					pm.Restart(ctx2, app.ServiceName, inst.ID)
				}
				cancel2()
				rolledBackTo = previous
				h.state.SetCurrentRelease(app.Name, previous)
			}
		}
	}

	if !rolledBack {
		if err := h.state.AddRelease(app.Name, release); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("persist state: %v", err))
			return
		}
	}

	resp := shared.DeployResponse{
		Release:       req.ReleaseName,
		Instances:     restarted,
		AppName:       app.Name,
		HealthResults: healthResults,
		RolledBack:    rolledBack,
		RolledBackTo:  rolledBackTo,
	}

	status := http.StatusOK
	msg := ""
	if len(failed) > 0 {
		status = http.StatusOK
		msg = fmt.Sprintf("some instances failed: %s", strings.Join(failed, ", "))
	}

	writeJSON(w, status, shared.APIEnvelope{
		Success: len(failed) == 0,
		Data:    resp,
		Error:   msg,
	})
}

func (h *Handler) HandleRollback(w http.ResponseWriter, r *http.Request) {
	var req shared.RollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AppName == "" {
		writeError(w, http.StatusBadRequest, "app_name is required")
		return
	}

	if !authorizeApp(r, req.AppName) {
		writeError(w, http.StatusForbidden, "this key is not authorized for this app")
		return
	}

	app, ok := h.state.GetApp(req.AppName)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("app %q not found", req.AppName))
		return
	}

	if req.ReleaseName == "" {
		prev, err := GetPreviousRelease(app.DeployPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("find previous release: %v", err))
			return
		}
		req.ReleaseName = prev
	}

	releaseDir := filepath.Join(app.DeployPath, "releases", req.ReleaseName)
	if _, err := filepath.Glob(releaseDir); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("release %q not found on disk", req.ReleaseName))
		return
	}

	if _, err := UpdateSymlink(app.DeployPath, req.ReleaseName); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("symlink update: %v", err))
		return
	}

	if err := h.state.SetCurrentRelease(app.Name, req.ReleaseName); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("persist state: %v", err))
		return
	}

	pm, err := NewProcessManager(app.ServiceType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var restarted []string
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	for _, inst := range app.Instances {
		unitName := strings.ReplaceAll(app.ServiceName, "%i", inst.ID)
		if err := pm.Restart(ctx, app.ServiceName, inst.ID); err != nil {
			continue
		}
		restarted = append(restarted, unitName)
	}

	writeSuccess(w, http.StatusOK, shared.RollbackResponse{
		Release:   req.ReleaseName,
		Instances: restarted,
	})
}

func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	apps := h.state.ListApps()

	disk := shared.DiskUsage{}
	// best-effort disk usage; ignore errors
	if fs := filepath.Join("/var/lib/minideploy"); true {
		_ = fs
	}

	writeSuccess(w, http.StatusOK, shared.StatusResponse{
		Version:   Version,
		Uptime:    time.Since(startTime).Round(time.Second).String(),
		StartTime: startTime,
		AppsCount: len(apps),
		DiskUsage: disk,
	})
}

func (h *Handler) HandleListApps(w http.ResponseWriter, r *http.Request) {
	apps := h.state.ListApps()
	if !isGlobalKey(r) {
		appName := getKeyAppName(r)
		filtered := make([]shared.AppSummary, 0)
		for _, a := range apps {
			if a.Name == appName {
				filtered = append(filtered, a)
			}
		}
		writeSuccess(w, http.StatusOK, filtered)
		return
	}
	writeSuccess(w, http.StatusOK, apps)
}

func (h *Handler) HandleAppDetail(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/apps/")
	name = strings.SplitN(name, "/", 2)[0]

	if !authorizeApp(r, name) {
		writeError(w, http.StatusForbidden, "this key is not authorized for this app")
		return
	}

	app, ok := h.state.GetApp(name)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("app %q not found", name))
		return
	}

	detail := shared.AppDetail{
		Name:           app.Name,
		ServiceType:    app.ServiceType,
		ServiceName:    app.ServiceName,
		DeployPath:     app.DeployPath,
		Instances:      app.Instances,
		CurrentRelease: app.CurrentRelease,
		Releases:       app.Releases,
	}
	writeSuccess(w, http.StatusOK, detail)
}

func (h *Handler) HandleAppStatus(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/apps/"), "/")
	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	appName := parts[0]

	if !authorizeApp(r, appName) {
		writeError(w, http.StatusForbidden, "this key is not authorized for this app")
		return
	}

	app, ok := h.state.GetApp(appName)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("app %q not found", appName))
		return
	}

	pm, err := NewProcessManager(app.ServiceType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ctx := r.Context()
	status := shared.AppStatus{
		AppName:        appName,
		CurrentRelease: app.CurrentRelease,
	}

	for _, inst := range app.Instances {
		ps, _ := pm.Status(ctx, app.ServiceName, inst.ID)
		status.Instances = append(status.Instances, shared.InstanceStatus{
			ID:      inst.ID,
			Port:    inst.Port,
			Running: ps != nil && ps.Running,
			PID:     func() int { if ps != nil { return ps.PID }; return 0 }(),
		})
	}

	writeSuccess(w, http.StatusOK, status)
}

func (h *Handler) HandleAppReleases(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/apps/"), "/")
	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	appName := parts[0]

	if !authorizeApp(r, appName) {
		writeError(w, http.StatusForbidden, "this key is not authorized for this app")
		return
	}

	app, ok := h.state.GetApp(appName)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("app %q not found", appName))
		return
	}

	releases := make([]shared.Release, len(app.Releases))
	for i, r := range app.Releases {
		r.IsCurrent = r.Name == app.CurrentRelease
		releases[i] = r
	}

	writeSuccess(w, http.StatusOK, releases)
}

func (h *Handler) HandleAppLogs(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/apps/"), "/")
	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	appName := parts[0]

	if !authorizeApp(r, appName) {
		writeError(w, http.StatusForbidden, "this key is not authorized for this app")
		return
	}

	app, ok := h.state.GetApp(appName)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("app %q not found", appName))
		return
	}

	pm, err := NewProcessManager(app.ServiceType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ctx := r.Context()
	var allLogs strings.Builder
	for _, inst := range app.Instances {
		logs, err := pm.Logs(ctx, app.ServiceName, inst.ID, 100)
		if err != nil {
			allLogs.WriteString(fmt.Sprintf("--- %s@%s: error ---\n", app.ServiceName, inst.ID))
			continue
		}
		allLogs.WriteString(fmt.Sprintf("--- %s@%s ---\n", app.ServiceName, inst.ID))
		allLogs.WriteString(logs)
		allLogs.WriteString("\n")
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(allLogs.String()))
}

func (h *Handler) HandleDestroy(w http.ResponseWriter, r *http.Request) {
	appName := extractAppName(r.URL.Path, "/api/v1/apps/", "/destroy")

	if appName == "" {
		writeError(w, http.StatusBadRequest, "app name not found in path")
		return
	}

	if !isGlobalKey(r) {
		writeError(w, http.StatusForbidden, "only global keys can destroy apps")
		return
	}

	app, ok := h.state.GetApp(appName)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("app %q not found", appName))
		return
	}

	var req struct {
		Confirm bool `json:"confirm"`
		Soft    bool `json:"soft"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !req.Confirm {
		writeError(w, http.StatusBadRequest, "confirmation required: set confirm=true")
		return
	}

	pm, err := NewProcessManager(app.ServiceType)
	if err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		for _, inst := range app.Instances {
			_ = pm.Stop(ctx, app.ServiceName, inst.ID)
		}
		cancel()
	}

	if !req.Soft {
		if err := os.RemoveAll(app.DeployPath); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("remove deploy path: %v", err))
			return
		}
	}

	if err := h.state.RemoveApp(appName); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("remove from state: %v", err))
		return
	}

	mode := "hard"
	if req.Soft {
		mode = "soft"
	}

	writeJSON(w, http.StatusOK, shared.APIEnvelope{
		Success: true,
		Data: shared.DestroyResponse{
			AppName: appName,
			Soft:    req.Soft,
		},
		Error: "",
	})
	_ = mode
}

func (h *Handler) HandleRotateKey(w http.ResponseWriter, r *http.Request) {
	if !isGlobalKey(r) {
		writeError(w, http.StatusForbidden, "only global keys can rotate keys")
		return
	}

	var req shared.RotateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	raw, hash, err := GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("generate key: %v", err))
		return
	}

	count, err := h.state.RotateKey(hash, req.RevokeOld)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("rotate key: %v", err))
		return
	}

	writeSuccess(w, http.StatusOK, shared.RotateKeyResponse{
		NewKey:    raw,
		KeysCount: count,
	})
}

func (h *Handler) HandleCreateKey(w http.ResponseWriter, r *http.Request) {
	if !isGlobalKey(r) {
		writeError(w, http.StatusForbidden, "only global keys can create new keys")
		return
	}

	var req shared.CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Scope == "" {
		req.Scope = "app"
	}
	if req.Scope != "global" && req.Scope != "app" {
		writeError(w, http.StatusBadRequest, "scope must be 'global' or 'app'")
		return
	}
	if req.Scope == "app" && req.AppName == "" {
		writeError(w, http.StatusBadRequest, "app_name is required for app-scoped keys")
		return
	}

	raw, hash, err := GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("generate key: %v", err))
		return
	}

	if err := h.state.AddAPIKey(hash, req.Scope, req.AppName, req.Label); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("store key: %v", err))
		return
	}

	keys := h.state.GetAPIKeys()
	var newKey *shared.APIKeyEntry
	for i := range keys {
		if keys[i].KeyHash == hash {
			newKey = &keys[i]
			break
		}
	}

	resp := shared.CreateKeyResponse{
		RawKey:  raw,
		Scope:   req.Scope,
		AppName: req.AppName,
		Label:   req.Label,
	}
	if newKey != nil {
		resp.ID = newKey.ID
	}

	writeSuccess(w, http.StatusCreated, resp)
}

func (h *Handler) HandleDeleteKey(w http.ResponseWriter, r *http.Request) {
	if !isGlobalKey(r) {
		writeError(w, http.StatusForbidden, "only global keys can delete keys")
		return
	}

	var req shared.DeleteKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID <= 0 {
		writeError(w, http.StatusBadRequest, "valid key id is required")
		return
	}

	if err := h.state.DeleteAPIKey(req.ID); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("delete key: %v", err))
		return
	}

	writeSuccess(w, http.StatusOK, shared.DeleteKeyResponse{Deleted: true})
}

func (h *Handler) HandleListKeys(w http.ResponseWriter, r *http.Request) {
	if !isGlobalKey(r) {
		writeError(w, http.StatusForbidden, "only global keys can list keys")
		return
	}

	keys := h.state.GetAPIKeys()
	infos := make([]shared.KeyInfo, len(keys))
	for i, k := range keys {
		hint := ""
		if len(k.KeyHash) > 12 {
			hint = k.KeyHash[:12] + "..."
		}
		infos[i] = shared.KeyInfo{
			ID:        k.ID,
			Scope:     k.Scope,
			AppName:   k.AppName,
			Label:     k.Label,
			HashHint:  hint,
			CreatedAt: k.CreatedAt,
		}
	}
	writeSuccess(w, http.StatusOK, infos)
}

func extractAppName(path, prefix, suffix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	idx := strings.Index(trimmed, suffix)
	if idx < 0 {
		return ""
	}
	return trimmed[:idx]
}

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, shared.APIEnvelope{
		Success: true,
		Data:    map[string]string{"status": "ok"},
	})
}

var startTime = time.Now()
