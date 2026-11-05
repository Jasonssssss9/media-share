package search

import (
	"context"
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

func TestIndexMappingAndSearchRequest(t *testing.T) {
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 123000, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "elastic" || password != "test-only-password" {
			t.Error("search requests did not use server-side credentials")
		}
		if r.URL.Path == "/test-posts" {
			var body struct {
				Mappings struct {
					Properties map[string]struct {
						Type string `json:"type"`
					} `json:"properties"`
				} `json:"mappings"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Mappings.Properties["author"].Type != "keyword" || body.Mappings.Properties["content"].Type != "text" {
				t.Error("author and content mappings are incorrect")
			}
			w.WriteHeader(400)
			_, _ = io.WriteString(w, `{"error":{"type":"resource_already_exists_exception"}}`)
			return
		}
		if r.URL.Path != "/test-posts/_search" || r.Method != "POST" {
			t.Error("incorrect search endpoint")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		encoded, _ := json.Marshal(body["query"])
		if !strings.Contains(string(encoded), `"author":"Alice"`) || !strings.Contains(string(encoded), `"operator":"and"`) ||
			body["from"] != float64(20) || body["size"] != float64(20) || body["track_total_hits"] != true ||
			r.URL.Query().Get("allow_partial_search_results") != "false" {
			t.Error("search did not preserve exact author filtering and pagination")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"hits": map[string]any{
			"total": map[string]int{"value": 1},
			"hits":  []any{map[string]any{"_id": "post-id", "_source": map[string]any{"updated_at": stamp}}},
		}})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "test-only-password", "test-posts")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureIndex(t.Context()); err != nil {
		t.Fatal(err)
	}
	result, err := client.Find(t.Context(), "river walk", "Alice", 2, 20)
	if err != nil || result.Total != 1 || len(result.Hits) != 1 || !result.Hits[0].UpdatedAt.Equal(stamp) {
		t.Fatalf("unexpected search response: %v", err)
	}
}

func TestBulkChecksItemFailuresAndNewlines(t *testing.T) {
	var failed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		lines := strings.Split(string(data), "\n")
		if r.Header.Get("Content-Type") != "application/x-ndjson" || len(lines) != 3 || lines[2] != "" {
			t.Error("bulk requests must use newline-delimited JSON with a final newline")
		}
		var document Document
		if err := json.Unmarshal([]byte(lines[1]), &document); err != nil || document.Content != "First line.\nSecond line." {
			t.Error("bulk requests did not preserve multiline post text")
		}
		if failed.Load() {
			_, _ = io.WriteString(w, `{"errors":true,"items":[{"index":{"status":400}}]}`)
		} else {
			_, _ = io.WriteString(w, `{"errors":false,"items":[{"index":{"status":201}}]}`)
		}
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "", "test-posts")
	documents := []Document{{ID: "post-id", Content: "First line.\nSecond line."}}
	if err := client.Bulk(t.Context(), documents); err != nil {
		t.Fatal(err)
	}
	failed.Store(true)
	if err := client.Bulk(t.Context(), documents); !errors.Is(err, ErrResponse) {
		t.Fatalf("partial bulk failure was accepted: %v", err)
	}
}

func TestSearchFailuresAndCancellation(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"timed_out":true,"hits":{"total":{"value":1},"hits":[]}}`,
		`{"_shards":{"failed":1},"hits":{"total":{"value":1},"hits":[]}}`,
		`{"hits":{"total":{"value":1}}}`,
		"<html>unavailable</html>",
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, body)
		}))
		client, _ := NewClient(server.URL, "", "test-posts")
		_, err := client.Find(t.Context(), "river", "", 1, 20)
		server.Close()
		if !errors.Is(err, ErrResponse) {
			t.Fatalf("incomplete search was accepted: %v", err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		_, _ = io.WriteString(w, `{"error":{"type":"unavailable_shards_exception","reason":"private details"}}`)
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "test-only-password", "test-posts")
	_, err := client.Find(t.Context(), "river", "", 1, 20)
	var upstream *APIError
	if !errors.As(err, &upstream) || upstream.Status != 503 || strings.Contains(err.Error(), "private details") {
		t.Fatalf("incorrect search error: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.Find(ctx, "river", "", 1, 20); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled search returned %v", err)
	}
}

func TestIndexNamesCannotSelectMultipleIndices(t *testing.T) {
	for _, name := range []string{"", "*", "_all", ".security", "posts,other", "posts/other", "Posts"} {
		if _, err := NewClient("http://127.0.0.1:9200", "", name); err == nil {
			t.Fatalf("unsafe index name accepted: %q", name)
		}
	}
}

func TestSyncLockHonorsCancellation(t *testing.T) {
	service := NewService(nil, nil)
	if err := service.lock(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer service.unlock()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := service.lock(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("a canceled sync waited for the index lock: %v", err)
	}
}
