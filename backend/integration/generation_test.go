//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Jasonssssss9/media-share/backend/internal/imagegen"
	"github.com/Jasonssssss9/media-share/backend/internal/media"
)

type testGenerator func(context.Context, string) ([]byte, error)

func (g testGenerator) Enabled() bool { return g != nil }

func (g testGenerator) Generate(ctx context.Context, prompt string) ([]byte, error) {
	return g(ctx, prompt)
}

func TestGeneratedImagesUseThePostWorkflow(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, alice := postAccount(t, accounts, "alice")
	_, bob := postAccount(t, accounts, "bob")
	data := imageFixture(t, "jpeg")
	calls := 0
	generator := testGenerator(func(ctx context.Context, prompt string) ([]byte, error) {
		calls++
		if prompt != "A small house beside a lake" {
			t.Error("prompt was not trimmed")
		}
		return data, nil
	})
	handler, directory := testRoutesWithGenerator(t, accounts, pool, generator)
	status := authRequest(t, handler, "GET", "/media/generation", "", nil)
	expectStatus(t, status, 200)
	if status.Body.String() != "{\"enabled\":true}\n" || status.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("generation availability is incorrect")
	}
	body := `{"prompt":"  A small house beside a lake  "}`
	expectStatus(t, authRequest(t, handler, "POST", "/media/generation", body, nil), 401)
	crossSite := httptest.NewRequest("POST", "http://api.test/api/v1/media/generation", strings.NewReader(body))
	crossSite.Header.Set("Content-Type", "application/json")
	crossSite.Header.Set("Origin", "http://untrusted.test")
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	crossSite.AddCookie(alice)
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, crossSite)
	expectStatus(t, denied, 403)
	if calls != 0 {
		t.Fatal("unauthorized generation reached the provider")
	}

	generated := authRequest(t, handler, "POST", "/media/generation", body, alice)
	expectStatus(t, generated, 201)
	asset := uploadedImage(t, generated)
	if asset.Width != 8 || asset.Height != 6 || generated.Header().Get("Location") != asset.URL || calls != 1 {
		t.Fatal("incorrect generated image metadata")
	}
	assertFileCount(t, directory, 1)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+asset.ID, "", nil), 404)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+asset.ID, "", bob), 404)
	own := authRequest(t, handler, "GET", "/media/"+asset.ID, "", alice)
	expectStatus(t, own, 200)
	if !bytes.Equal(own.Body.Bytes(), data) || own.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatal("generated image bytes were not saved")
	}
	expectStatus(t, authRequest(t, handler, "POST", "/posts",
		jsonBody(t, map[string]any{"content": "", "media_ids": []string{asset.ID}}), bob), 403)

	uploaded := imageUpload(t, handler, alice, imageFixture(t, "png"))
	expectStatus(t, uploaded, 201)
	ids := []string{uploadedImage(t, uploaded).ID, asset.ID}
	created := authRequest(t, handler, "POST", "/posts",
		jsonBody(t, map[string]any{"content": "", "media_ids": ids}), alice)
	expectStatus(t, created, 201)
	post := responsePost(t, created)
	assertImageOrder(t, post, ids)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+asset.ID, "", nil), 200)
	expectStatus(t, authRequest(t, handler, "DELETE", "/posts/"+post.ID, "", alice), 204)
	assertFileCount(t, directory, 0)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+asset.ID, "", nil), 404)

	limited := authRequest(t, handler, "POST", "/media/generation", body, alice)
	expectStatus(t, limited, 429)
	if limited.Header().Get("Retry-After") == "" || calls != 1 {
		t.Fatal("generation cooldown did not prevent another provider request")
	}
}

func TestGenerationConfigurationAndValidation(t *testing.T) {
	db, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "alice")
	disabled, directory := testRoutesWithGenerator(t, accounts, pool, testGenerator(nil))
	status := authRequest(t, disabled, "GET", "/media/generation", "", nil)
	expectStatus(t, status, 200)
	if status.Body.String() != "{\"enabled\":false}\n" {
		t.Fatal("unconfigured generation was advertised as enabled")
	}
	expectStatus(t, authRequest(t, disabled, "POST", "/media/generation", `{"prompt":"A lake"}`, cookie), 503)
	assertFileCount(t, directory, 0)

	calls := 0
	generator := testGenerator(func(ctx context.Context, prompt string) ([]byte, error) {
		calls++
		if utf8.RuneCountInString(prompt) != imagegen.MaxPromptLength {
			t.Error("Unicode prompt length was counted incorrectly")
		}
		return imageFixture(t, "png"), nil
	})
	handler, _ := testRoutesWithGenerator(t, accounts, pool, generator)
	for _, body := range []string{
		"{}", `{"prompt":"   "}`, `{"prompt":null}`, `{"prompt":"a\u0000b"}`,
		`{"prompt":"A lake","n":2}`, `{"prompt":"A lake"} {}`, "{",
		jsonBody(t, map[string]string{"prompt": strings.Repeat("x", 1001)}),
	} {
		expectStatus(t, authRequest(t, handler, "POST", "/media/generation", body, cookie), 400)
	}
	if calls != 0 {
		t.Fatal("invalid input reached the provider")
	}
	valid := authRequest(t, handler, "POST", "/media/generation",
		jsonBody(t, map[string]string{"prompt": strings.Repeat("\U0001f3a8", 1000)}), cookie)
	expectStatus(t, valid, 201)
	asset := uploadedImage(t, valid)
	expectStatus(t, authRequest(t, handler, "DELETE", "/media/"+asset.ID, "", cookie), 204)
	var count int
	if err := db.QueryRow("SELECT count(*) FROM media").Scan(&count); err != nil || count != 0 {
		t.Fatalf("discarded image left metadata: %v", err)
	}
}

func TestGenerationFailuresLeaveNoImages(t *testing.T) {
	db, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "alice")
	for _, test := range []struct {
		name   string
		data   []byte
		err    error
		status int
	}{
		{"moderation", nil, &imagegen.APIError{Status: 400, Code: "moderation_blocked"}, 400},
		{"user_error", nil, &imagegen.APIError{Status: 400, Type: "image_generation_user_error"}, 400},
		{"credentials", nil, &imagegen.APIError{Status: 401}, 503},
		{"quota", nil, &imagegen.APIError{Status: 429}, 429},
		{"provider_error", nil, &imagegen.APIError{Status: 500}, 502},
		{"invalid_response", nil, imagegen.ErrResponse, 502},
		{"timeout", nil, context.DeadlineExceeded, 504},
		{"network_error", nil, errors.New("private provider details"), 503},
		{"invalid_image", []byte("not an image"), nil, 502},
		{"oversize_image", make([]byte, media.MaxSize+1), nil, 502},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, directory := testRoutesWithGenerator(t, accounts, pool,
				testGenerator(func(context.Context, string) ([]byte, error) { return test.data, test.err }))
			response := authRequest(t, handler, "POST", "/media/generation", `{"prompt":"A lake"}`, cookie)
			expectStatus(t, response, test.status)
			if strings.Contains(response.Body.String(), "private provider details") {
				t.Fatal("provider error details were exposed")
			}
			assertFileCount(t, directory, 0)
			var count int
			if err := db.QueryRow("SELECT count(*) FROM media").Scan(&count); err != nil || count != 0 {
				t.Fatalf("failed generation left metadata: %v", err)
			}
		})
	}
	t.Run("storage_failure", func(t *testing.T) {
		data := imageFixture(t, "png")
		handler, directory := testRoutesWithGenerator(t, accounts, pool,
			testGenerator(func(context.Context, string) ([]byte, error) {
				pool.Close()
				return data, nil
			}))
		expectStatus(t, authRequest(t, handler, "POST", "/media/generation", `{"prompt":"A lake"}`, cookie), 503)
		assertFileCount(t, directory, 0)
	})
}

func TestGenerationExtendsWriteDeadlineAndRejectsOverlap(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "alice")
	data := imageFixture(t, "jpeg")
	entered, release := make(chan struct{}), make(chan struct{})
	handler, _ := testRoutesWithGenerator(t, accounts, pool,
		testGenerator(func(ctx context.Context, prompt string) ([]byte, error) {
			close(entered)
			select {
			case <-release:
				return data, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}))
	server := httptest.NewUnstartedServer(handler)
	server.Config.WriteTimeout = 30 * time.Millisecond
	server.Start()
	defer server.Close()
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	client := server.Client()
	client.Timeout = 2 * time.Second
	type result struct {
		response *http.Response
		err      error
	}
	done := make(chan result, 1)
	request, err := http.NewRequest("POST", server.URL+"/api/v1/media/generation", strings.NewReader(`{"prompt":"A lake"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", "http://frontend.test")
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	go func() {
		response, err := client.Do(request)
		done <- result{response, err}
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("generation did not start")
	}
	busy := authRequest(t, handler, "POST", "/media/generation", `{"prompt":"Another lake"}`, cookie)
	expectStatus(t, busy, 429)
	if busy.Header().Get("Retry-After") == "" {
		t.Fatal("busy response is missing Retry-After")
	}
	time.Sleep(80 * time.Millisecond)
	unblock()
	got := <-done
	if got.err != nil {
		t.Fatalf("generation was interrupted by the ordinary write timeout: %v", got.err)
	}
	defer got.response.Body.Close()
	body, err := io.ReadAll(got.response.Body)
	if err != nil || got.response.StatusCode != 201 {
		t.Fatalf("generation status=%d body=%s error=%v", got.response.StatusCode, body, err)
	}
	var asset media.Image
	if err := json.Unmarshal(body, &asset); err != nil || asset.ID == "" {
		t.Fatal("generation did not return image metadata")
	}
}
