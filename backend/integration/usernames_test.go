//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestCaseSensitiveUsernames(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	handler := testRoutes(t, accounts, pool)
	password := "test-only password"
	names := []string{"Alice", "alice"}
	emails := []string{"alice-upper@example.test", "alice-lower@example.test"}
	ids := make([]string, 2)
	cookies := make([]*http.Cookie, 2)
	postIDs := make([]string, 2)

	for i, name := range names {
		response := authRequest(t, handler, "POST", "/auth/register", jsonBody(t, map[string]string{
			"username": name, "email": emails[i], "password": password,
		}), nil)
		expectStatus(t, response, 201)
		user := responseUser(t, response)
		if user.Username != name || user.DisplayName != name {
			t.Fatalf("registration changed username case: %+v", user)
		}
		ids[i] = user.ID
		cookies[i] = response.Result().Cookies()[0]
		post := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]string{"content": name + "'s post"}), cookies[i])
		expectStatus(t, post, 201)
		postIDs[i] = responsePost(t, post).ID
	}
	if ids[0] == ids[1] {
		t.Fatal("case-distinct usernames shared an account")
	}

	for i, name := range names {
		duplicate := authRequest(t, handler, "POST", "/auth/register", jsonBody(t, map[string]string{
			"username": name, "email": "duplicate-" + emails[i], "password": password,
		}), nil)
		expectStatus(t, duplicate, 409)
		login := authRequest(t, handler, "POST", "/auth/login", jsonBody(t, map[string]string{
			"email": emails[i], "password": password,
		}), nil)
		expectStatus(t, login, 200)
		if user := responseUser(t, login); user.ID != ids[i] || user.Username != name {
			t.Fatal("sign-in returned the wrong case-distinct account")
		}
		profile := authRequest(t, handler, "GET", "/users/"+name, "", nil)
		expectStatus(t, profile, 200)
		if user := responseUser(t, profile); user.ID != ids[i] || user.Username != name {
			t.Fatal("profile lookup ignored username case")
		}
		response := authRequest(t, handler, "GET", "/posts?author="+name, "", nil)
		expectStatus(t, response, 200)
		page := postPage(t, response)
		if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != postIDs[i] || page.Items[0].Author.ID != ids[i] {
			t.Fatal("author filtering mixed case-distinct accounts")
		}
	}
	expectStatus(t, authRequest(t, handler, "GET", "/users/ALICE", "", nil), 404)
	unknown := authRequest(t, handler, "GET", "/posts?author=ALICE", "", nil)
	expectStatus(t, unknown, 200)
	if page := postPage(t, unknown); page.Total != 0 || len(page.Items) != 0 {
		t.Fatal("unknown case matched an author")
	}

	detail := authRequest(t, handler, "GET", "/posts/"+postIDs[0], "", nil)
	expectStatus(t, detail, 200)
	post := responsePost(t, detail)
	expectStatus(t, authRequest(t, handler, "PATCH", "/posts/"+post.ID, editBody(t, post, "Wrong owner"), cookies[1]), 403)
	expectStatus(t, authRequest(t, handler, "DELETE", "/posts/"+post.ID, "", cookies[1]), 403)
	expectStatus(t, authRequest(t, handler, "PATCH", "/posts/"+post.ID, editBody(t, post, "Own edit"), cookies[0]), 200)
}

func TestUsernameMigrationRollback(t *testing.T) {
	db := testDatabase(t)
	ctx := t.Context()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpContext(ctx, db, "../migrations"); err != nil {
		t.Fatal(err)
	}
	mustExec(t, ctx, db, "INSERT INTO users (username, email, password_hash, display_name) VALUES ('Alice', 'upper@example.test', 'test-hash', 'Alice'), ('alice', 'lower@example.test', 'test-hash', 'alice')")
	if err := goose.DownContext(ctx, db, "../migrations"); err == nil {
		t.Fatal("rollback should reject usernames outside the old lowercase-only rule")
	}
	checkVersion(t, ctx, db, 3)
	var username string
	if err := db.QueryRowContext(ctx, "SELECT username FROM users WHERE email='upper@example.test'").Scan(&username); err != nil || username != "Alice" {
		t.Fatalf("failed rollback changed the account: %q, %v", username, err)
	}
	mustExec(t, ctx, db, "DELETE FROM users WHERE email='upper@example.test'")
	if err := goose.DownContext(ctx, db, "../migrations"); err != nil {
		t.Fatal(err)
	}
	checkVersion(t, ctx, db, 2)
	if _, err := db.ExecContext(ctx, "INSERT INTO users (username, email, password_hash, display_name) VALUES ('Alice', 'upper@example.test', 'test-hash', 'Alice')"); err == nil {
		t.Fatal("rollback did not restore the old username check")
	}
	if err := goose.UpContext(ctx, db, "../migrations"); err != nil {
		t.Fatal(err)
	}
	checkVersion(t, ctx, db, 3)
	mustExec(t, ctx, db, "INSERT INTO users (username, email, password_hash, display_name) VALUES ('Alice', 'upper@example.test', 'test-hash', 'Alice')")
}
