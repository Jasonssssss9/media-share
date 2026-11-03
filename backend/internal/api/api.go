package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type PingDatabase func(context.Context) error

func NewHandler(ping PingDatabase) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !databaseReady(w, r, ping) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		if !databaseReady(w, r, ping) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"name":   "MediaShare",
			"status": "ready",
		})
	})
	return mux
}

func databaseReady(w http.ResponseWriter, r *http.Request, ping PingDatabase) bool {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "MediaShare is temporarily unavailable.")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":       code,
			"message":    message,
			"request_id": w.Header().Get("X-Request-ID"),
		},
	})
}
