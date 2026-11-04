package api

import (
	"crypto/rand"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

func Middleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := rand.Text()
		w.Header().Set("X-Request-ID", requestID)
		response := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		defer func() {
			logger.Info("request",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", response.status,
				"duration", time.Since(start),
			)
		}()
		defer func() {
			if recovered := recover(); recovered != nil {
				if recovered == http.ErrAbortHandler {
					panic(recovered)
				}
				logger.Error("request panicked", "request_id", requestID, "panic", recovered, "stack", string(debug.Stack()))
				if response.wroteHeader {
					panic(http.ErrAbortHandler)
				}
				writeError(response, http.StatusInternalServerError, "internal_error", "Something went wrong.")
			}
		}()

		r.Body = http.MaxBytesReader(response, r.Body, 25<<20)
		next.ServeHTTP(response, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
