//go:build integration

package integration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Jasonssssss9/media-share/backend/internal/api"
	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"github.com/Jasonssssss9/media-share/backend/internal/config"
	"github.com/Jasonssssss9/media-share/backend/internal/imagegen"
	"github.com/Jasonssssss9/media-share/backend/internal/media"
	"github.com/Jasonssssss9/media-share/backend/internal/posts"
	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

const authTestSecret = "test-only-auth-signing-key-with-32-characters"

func TestAuthenticationFlow(t *testing.T) {
	db, service, pool := authDatabase(t)
	handler := testRoutes(t, service, pool)
	password := "a test password"
	registered := authRequest(t, handler, "POST", "/auth/register",
		jsonBody(t, map[string]string{"username": "alice", "email": " Alice@Example.test ", "password": password}), nil)
	expectStatus(t, registered, http.StatusCreated)
	user := responseUser(t, registered)
	if user.Email != "alice@example.test" || user.Username != "alice" {
		t.Fatalf("unexpected account: %+v", user)
	}
	if strings.Contains(registered.Body.String(), "password_hash") || strings.Contains(registered.Body.String(), "session_version") {
		t.Fatal("private account fields were exposed")
	}
	cookie := registered.Result().Cookies()[0]
	if cookie.Name != "mediashare_session" || !cookie.HttpOnly || cookie.Secure ||
		cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/api" || cookie.MaxAge != 86400 {
		t.Fatalf("unexpected session cookie attributes")
	}
	var hash string
	if err := db.QueryRow("SELECT password_hash FROM users WHERE id = $1", user.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil || !match || hash == password {
		t.Fatal("password was not safely hashed")
	}

	current := authRequest(t, handler, "GET", "/auth/me", "", cookie)
	expectStatus(t, current, http.StatusOK)
	if responseUser(t, current).ID != user.ID {
		t.Fatal("cookie did not restore the registered account")
	}

	otherLogin := authRequest(t, handler, "POST", "/auth/login",
		jsonBody(t, map[string]string{"email": "ALICE@EXAMPLE.TEST", "password": password}), nil)
	expectStatus(t, otherLogin, http.StatusOK)
	otherCookie := otherLogin.Result().Cookies()[0]

	wrong := authRequest(t, handler, "POST", "/auth/login",
		jsonBody(t, map[string]string{"email": user.Email, "password": "wrong password"}), nil)
	expectStatus(t, wrong, http.StatusUnauthorized)

	tampered := *cookie
	parts := strings.Split(tampered.Value, ".")
	if parts[2][0] == 'A' {
		parts[2] = "B" + parts[2][1:]
	} else {
		parts[2] = "A" + parts[2][1:]
	}
	tampered.Value = strings.Join(parts, ".")
	expectStatus(t, authRequest(t, handler, "GET", "/auth/me", "", &tampered), http.StatusUnauthorized)

	expired := *cookie
	expired.Value, err = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "mediashare", "aud": "mediashare-web", "sub": user.ID, "sv": 0,
		"exp": time.Now().Add(-time.Minute).Unix(),
	}).SignedString([]byte(authTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, authRequest(t, handler, "GET", "/auth/me", "", &expired), http.StatusUnauthorized)

	loggedOut := authRequest(t, handler, "POST", "/auth/logout", "", cookie)
	expectStatus(t, loggedOut, http.StatusNoContent)
	if loggedOut.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatal("logout did not clear the browser cookie")
	}
	expectStatus(t, authRequest(t, handler, "GET", "/auth/me", "", cookie), http.StatusUnauthorized)
	expectStatus(t, authRequest(t, handler, "GET", "/auth/me", "", otherCookie), http.StatusUnauthorized)
	expectStatus(t, authRequest(t, handler, "POST", "/auth/logout", "", nil), http.StatusNoContent)

	loginAgain := authRequest(t, handler, "POST", "/auth/login",
		jsonBody(t, map[string]string{"email": user.Email, "password": password}), nil)
	expectStatus(t, loginAgain, http.StatusOK)
	expectStatus(t, authRequest(t, handler, "GET", "/auth/me", "", loginAgain.Result().Cookies()[0]), http.StatusOK)
	for name, response := range map[string]*httptest.ResponseRecorder{
		"register": registered, "login": otherLogin, "current user": current,
		"wrong password": wrong, "logout": loggedOut,
	} {
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s response can be cached", name)
		}
	}
}

func TestAuthenticationValidationAndOrigin(t *testing.T) {
	_, service, pool := authDatabase(t)
	if _, _, err := service.Register(context.Background(), "alice", "alice@example.test", "a test password"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{"duplicate_email", "/auth/register", `{"username":"bob","email":"ALICE@example.test","password":"a test password"}`, 409},
		{"duplicate_username", "/auth/register", `{"username":"alice","email":"bob@example.test","password":"a test password"}`, 409},
		{"invalid_username", "/auth/register", `{"username":"Bad Name","email":"bob@example.test","password":"a test password"}`, 400},
		{"invalid_email", "/auth/register", `{"username":"bob","email":"not-an-email","password":"a test password"}`, 400},
		{"short_password", "/auth/register", `{"username":"bob","email":"bob@example.test","password":"short"}`, 400},
		{"long_password", "/auth/register", jsonBody(t, map[string]string{"username": "bob", "email": "bob@example.test", "password": strings.Repeat("x", 129)}), 400},
		{"unknown_account", "/auth/login", `{"email":"unknown@example.test","password":"a test password"}`, 401},
		{"malformed_json", "/auth/register", "{", 400},
		{"unknown_field", "/auth/register", `{"username":"bob","email":"bob@example.test","password":"a test password","admin":true}`, 400},
		{"trailing_json", "/auth/login", `{"email":"alice@example.test","password":"a test password"} {}`, 400},
		{"large_body", "/auth/login", strings.Repeat("x", 17<<10), 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			expectStatus(t, authRequest(t, testRoutes(t, service, pool), "POST", test.path, test.body, nil), test.status)
		})
	}

	handler := testRoutes(t, service, pool)
	for _, path := range []string{"/auth/register", "/auth/login", "/auth/logout"} {
		request := httptest.NewRequest(http.MethodPost, "http://api.test/api/v1"+path, strings.NewReader("{}"))
		request.Header.Set("Origin", "http://untrusted.test")
		request.Header.Set("Sec-Fetch-Site", "cross-site")
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		expectStatus(t, response, http.StatusForbidden)
	}
	expectStatus(t, authRequest(t, handler, "GET", "/auth/me", "", nil), http.StatusUnauthorized)

	t.Run("auth_attempt_limit", func(t *testing.T) {
		handler := testRoutes(t, service, pool)
		for range 10 {
			expectStatus(t, authRequest(t, handler, "POST", "/auth/login", "{}", nil), http.StatusBadRequest)
		}
		expectStatus(t, authRequest(t, handler, "POST", "/auth/login", "{}", nil), http.StatusTooManyRequests)
	})
}

func authDatabase(t *testing.T) (*sql.DB, *auth.Service, *pgxpool.Pool) {
	t.Helper()
	db := testDatabase(t)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpContext(context.Background(), db, "../migrations"); err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := db.QueryRow("SELECT current_schema()").Scan(&schema); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	service, err := auth.NewService(pool, authTestSecret)
	if err != nil {
		t.Fatal(err)
	}
	return db, service, pool
}

func testRoutes(t *testing.T, service *auth.Service, pool *pgxpool.Pool) http.Handler {
	handler, _ := testRoutesWithMedia(t, service, pool)
	return handler
}

func testRoutesWithMedia(t *testing.T, service *auth.Service, pool *pgxpool.Pool) (http.Handler, string) {
	return testRoutesWithGenerator(t, service, pool, imagegen.NewClient("", ""))
}

func testRoutesWithGenerator(t *testing.T, service *auth.Service, pool *pgxpool.Pool, generator api.ImageGenerator) (http.Handler, string) {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	mux := api.NewHandler(pool.Ping)
	directory := t.TempDir()
	images, err := media.NewService(pool, directory)
	if err != nil {
		t.Fatal(err)
	}
	api.RegisterPosts(mux, posts.NewService(pool), images, service, "http://frontend.test", logger)
	api.RegisterMedia(mux, images, service, "http://frontend.test", logger)
	api.RegisterProfiles(mux, service, images, "http://frontend.test", logger)
	api.RegisterImageGeneration(mux, generator, images, service, "http://frontend.test", logger)
	handler, err := api.WithAuth(mux, service, "http://frontend.test", logger)
	if err != nil {
		t.Fatal(err)
	}
	return api.Middleware(logger, handler), directory
}

func authRequest(t *testing.T, handler http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://api.test/api/v1"+path, strings.NewReader(body))
	request.Header.Set("Origin", "http://frontend.test")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func expectStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status=%d, want %d: %s", response.Code, want, response.Body.String())
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("response is missing a request ID")
	}
}

func jsonBody(t *testing.T, body any) string {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func responseUser(t *testing.T, response *httptest.ResponseRecorder) auth.User {
	t.Helper()
	var user auth.User
	if err := json.Unmarshal(response.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	return user
}

func TestUnicodePasswords(t *testing.T) {
	_, service, pool := authDatabase(t)
	handler := testRoutes(t, service, pool)
	password := strings.Repeat("\U0001f512", 128)
	registered := authRequest(t, handler, "POST", "/auth/register", jsonBody(t, map[string]string{
		"username": "unicode_user", "email": "unicode@example.test", "password": password,
	}), nil)
	expectStatus(t, registered, http.StatusCreated)
	loggedIn := authRequest(t, handler, "POST", "/auth/login", jsonBody(t, map[string]string{
		"email": "unicode@example.test", "password": password,
	}), nil)
	expectStatus(t, loggedIn, http.StatusOK)

	for _, length := range []int{7, 129} {
		response := authRequest(t, handler, "POST", "/auth/register", jsonBody(t, map[string]string{
			"username": "another_user", "email": "another@example.test", "password": strings.Repeat("\U0001f512", length),
		}), nil)
		expectStatus(t, response, http.StatusBadRequest)
	}
}

func TestDatabaseFailurePreservesSession(t *testing.T) {
	_, service, pool := authDatabase(t)
	_, token, err := service.Register(context.Background(), "alice", "alice@example.test", "a test password")
	if err != nil {
		t.Fatal(err)
	}
	handler := testRoutes(t, service, pool)
	pool.Close()
	cookie := &http.Cookie{Name: "mediashare_session", Value: token}

	for _, path := range []string{"/auth/me", "/auth/logout", "/auth/login"} {
		method, body := http.MethodPost, ""
		if path == "/auth/me" {
			method = http.MethodGet
		}
		if path == "/auth/login" {
			body = jsonBody(t, map[string]string{"email": "alice@example.test", "password": "a test password"})
		}
		response := authRequest(t, handler, method, path, body, cookie)
		expectStatus(t, response, http.StatusServiceUnavailable)
		if len(response.Result().Cookies()) != 0 {
			t.Errorf("%s changed the session cookie while the database was unavailable", path)
		}
	}
}
