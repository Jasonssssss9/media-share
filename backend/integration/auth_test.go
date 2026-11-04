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
	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

const authTestSecret = "test-only-auth-signing-key-with-32-characters"

func TestAuthenticationFlow(t *testing.T) {
	db, service, pool := authDatabase(t)
	handler := authRoutes(t, service, pool)
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
			expectStatus(t, authRequest(t, authRoutes(t, service, pool), "POST", test.path, test.body, nil), test.status)
		})
	}

	handler := authRoutes(t, service, pool)
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
		handler := authRoutes(t, service, pool)
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

func authRoutes(t *testing.T, service *auth.Service, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler, err := api.WithAuth(api.NewHandler(pool.Ping), service, "http://frontend.test", logger)
	if err != nil {
		t.Fatal(err)
	}
	return api.Middleware(logger, handler)
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

func jsonBody(t *testing.T, body map[string]string) string {
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
