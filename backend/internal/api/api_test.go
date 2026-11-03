package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadinessTracksDatabaseAvailability(t *testing.T) {
	available := true
	handler := NewHandler(func(context.Context) error {
		if !available {
			return errors.New("database is offline")
		}
		return nil
	})
	for _, test := range []struct {
		path      string
		available bool
		want      int
	}{
		{"/readyz", true, http.StatusOK},
		{"/readyz", false, http.StatusServiceUnavailable},
		{"/api/v1/status", false, http.StatusServiceUnavailable},
		{"/healthz", false, http.StatusOK},
	} {
		available = test.available
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != test.want {
			t.Errorf("%s (database available=%v): got %d, want %d", test.path, available, response.Code, test.want)
		}
	}
}

func TestRecoveryReturnsAnErrorWithARequestID(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := Middleware(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", response.Code)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Error("missing request ID")
	}
}
