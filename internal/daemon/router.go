package daemon

import (
	"net/http"
	"strings"
)

func NewRouter(state *StateManager) http.Handler {
	h := NewHandler(state)
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/deploy", h.HandleDeploy)
	mux.HandleFunc("/api/v1/rollback", h.HandleRollback)
	mux.HandleFunc("/api/v1/rotate-key", h.HandleRotateKey)
	mux.HandleFunc("/api/v1/status", h.HandleStatus)
	mux.HandleFunc("/api/v1/health", h.HandleHealth)
	mux.HandleFunc("/api/v1/apps", h.HandleListApps)
	mux.HandleFunc("/api/v1/apps/init", h.HandleInitApp)

	mux.HandleFunc("/api/v1/apps/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/apps/")
		switch {
		case strings.HasSuffix(path, "/destroy"):
			h.HandleDestroy(w, r)
		case strings.HasSuffix(path, "/status"):
			h.HandleAppStatus(w, r)
		case strings.HasSuffix(path, "/releases"):
			h.HandleAppReleases(w, r)
		case strings.HasSuffix(path, "/logs"):
			h.HandleAppLogs(w, r)
		case strings.Contains(path, "/"):
			writeError(w, http.StatusNotFound, "not found")
		default:
			h.HandleAppDetail(w, r)
		}
	})

	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.HandleListKeys(w, r)
		case http.MethodPost:
			h.HandleCreateKey(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/v1/keys/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.HandleDeleteKey(w, r)
	})

	// Public endpoints (no auth)
	public := http.NewServeMux()
	public.Handle("/api/v1/health", mux)

	// Wrap the main mux with auth
	authMw := authMiddleware(state)
	protected := authMw(mux)

	// Combine: public routes take priority
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" {
			public.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	})

	handler = loggingMiddleware(handler)
	handler = recoveryMiddleware(handler)

	return handler
}
