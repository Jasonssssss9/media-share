package imagegen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGenerateImage(t *testing.T) {
	want := []byte("generated image bytes")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" ||
			r.Header.Get("Authorization") != "Bearer test-only-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("incorrect image API request")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["model"] != "test-model" || body["prompt"] != "A quiet lake" || body["n"] != float64(1) ||
			body["size"] != "1024x1024" || body["quality"] != "low" ||
			body["output_format"] != "jpeg" || body["output_compression"] != float64(80) {
			t.Error("incorrect image generation parameters")
		}
		if _, exists := body["response_format"]; exists {
			t.Error("legacy response_format must not be sent")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(want)}},
		})
	}))
	defer server.Close()
	client := NewClient(" test-only-key ", "test-model")
	client.endpoint = server.URL + "/v1/images/generations"
	got, err := client.Generate(t.Context(), "A quiet lake")
	if err != nil || string(got) != string(want) || calls.Load() != 1 {
		t.Fatalf("generation failed: %v", err)
	}
}

func TestGenerationWithoutAKey(t *testing.T) {
	client := NewClient("", "")
	if client.Enabled() || client.model != DefaultModel {
		t.Fatal("incorrect optional configuration")
	}
	if _, err := client.Generate(t.Context(), "A quiet lake"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unconfigured generation returned %v", err)
	}
}

func TestAPIRejectionsAreNotRetriedOrExposed(t *testing.T) {
	for _, status := range []int{400, 401, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("X-Request-ID", "provider-request-123")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"error":{"code":"test_error","type":"api_error","message":"private provider details"}}`)
			}))
			defer server.Close()
			client := NewClient("test-only-key", "")
			client.endpoint = server.URL
			_, err := client.Generate(t.Context(), "A quiet lake")
			var upstream *APIError
			if !errors.As(err, &upstream) || upstream.Status != status || upstream.Code != "test_error" ||
				upstream.RequestID != "provider-request-123" || calls.Load() != 1 {
				t.Fatalf("incorrect API error: %v", err)
			}
			if strings.Contains(err.Error(), "private provider details") || strings.Contains(err.Error(), "test-only-key") {
				t.Fatal("private details were exposed")
			}
		})
	}
}

func TestInvalidGenerationResponses(t *testing.T) {
	for name, body := range map[string]string{
		"invalid_json":      "<html>unavailable</html>",
		"missing_image":     `{"data":[]}`,
		"empty_image":       `{"data":[{"b64_json":""}]}`,
		"invalid_base64":    `{"data":[{"b64_json":"invalid!"}]}`,
		"multiple_images":   `{"data":[{"b64_json":"YQ=="},{"b64_json":"Yg=="}]}`,
		"oversize_response": strings.Repeat("x", maxResponseSize+1),
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			client := NewClient("test-only-key", "")
			client.endpoint = server.URL
			if _, err := client.Generate(t.Context(), "A quiet lake"); !errors.Is(err, ErrResponse) {
				t.Fatalf("invalid response returned %v", err)
			}
		})
	}
}

func TestGenerationHonorsCancellationAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer server.Close()
	client := NewClient("test-only-key", "")
	client.endpoint = server.URL

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.Generate(ctx, "A quiet lake"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request returned %v", err)
	}
	client.http.Timeout = 20 * time.Millisecond
	if _, err := client.Generate(t.Context(), "A quiet lake"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed-out request returned %v", err)
	}
}
