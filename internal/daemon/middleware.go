package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/hamid/minideploy/internal/shared"
)

type contextKey string

const (
	contextKeyScope   contextKey = "scope"
	contextKeyAppName contextKey = "app_name"
)

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				err, ok := rec.(error)
				if !ok {
					err = fmt.Errorf("%v", rec)
				}
				log.Printf("PANIC: %v", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggedResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lrw.statusCode, time.Since(start))
	})
}

type loggedResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggedResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func authMiddleware(state *StateManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				writeError(w, http.StatusUnauthorized, "invalid authorization format")
				return
			}

			keys := state.GetAPIKeys()
			entry, ok := findKeyByToken(token, keys)
			if !ok {
				writeError(w, http.StatusUnauthorized, "invalid api key")
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyScope, entry.Scope)
			ctx = context.WithValue(ctx, contextKeyAppName, entry.AppName)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func findKeyByToken(token string, keys []shared.APIKeyEntry) (*shared.APIKeyEntry, bool) {
	for _, k := range keys {
		if validateBcrypt(token, k.KeyHash) {
			return &k, true
		}
	}
	return nil, false
}

func validateBcrypt(provided, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(provided)) == nil
}

func getScope(r *http.Request) string {
	s, _ := r.Context().Value(contextKeyScope).(string)
	return s
}

func getKeyAppName(r *http.Request) string {
	a, _ := r.Context().Value(contextKeyAppName).(string)
	return a
}

func isGlobalKey(r *http.Request) bool {
	return getScope(r) == "global"
}

func authorizeApp(r *http.Request, appName string) bool {
	if isGlobalKey(r) {
		return true
	}
	return getKeyAppName(r) == appName
}

func writeJSON(w http.ResponseWriter, status int, env shared.APIEnvelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(env)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, shared.APIEnvelope{
		Success: false,
		Error:   msg,
	})
}

func writeSuccess(w http.ResponseWriter, status int, data interface{}) {
	writeJSON(w, status, shared.APIEnvelope{
		Success: true,
		Data:    data,
	})
}
