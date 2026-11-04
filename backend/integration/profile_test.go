//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Jasonssssss9/media-share/backend/internal/posts"
)

func TestProfileUpdates(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	alice, cookie := postAccount(t, accounts, "Alice")
	handler, _ := testRoutesWithMedia(t, accounts, pool)
	lower := authRequest(t, handler, "POST", "/auth/register",
		`{"username":"alice","email":"lower-alice@example.test","password":"test-only password"}`, nil)
	expectStatus(t, lower, 201)
	other := lower.Result().Cookies()[0]
	created := authRequest(t, handler, "POST", "/posts", `{"content":"A profile update test."}`, cookie)
	expectStatus(t, created, 201)
	post := responsePost(t, created)

	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me", `{"bio":"Updated"}`, nil), 401)
	crossSite := httptest.NewRequest("PATCH", "http://api.test/api/v1/users/me", strings.NewReader(`{"bio":"Updated"}`))
	crossSite.Header.Set("Content-Type", "application/json")
	crossSite.Header.Set("Origin", "http://untrusted.test")
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	crossSite.AddCookie(cookie)
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, crossSite)
	expectStatus(t, denied, 403)

	updated := authRequest(t, handler, "PATCH", "/users/me",
		`{"display_name":"  Alice Example  ","bio":"  Student project.\nPhotos and notes.  "}`, cookie)
	expectStatus(t, updated, 200)
	user := responseUser(t, updated)
	if user.ID != alice.ID || user.Username != "Alice" || user.Email != alice.Email ||
		user.DisplayName != "Alice Example" || user.Bio != "Student project.\nPhotos and notes." {
		t.Fatal("profile fields were not saved correctly")
	}
	if updated.Header().Get("Cache-Control") != "no-store" || len(updated.Result().Cookies()) != 0 ||
		strings.Contains(updated.Body.String(), "password_hash") || strings.Contains(updated.Body.String(), "session_version") {
		t.Fatal("profile response exposed private fields or changed the session")
	}
	current := authRequest(t, handler, "GET", "/auth/me", "", cookie)
	expectStatus(t, current, 200)
	if responseUser(t, current).DisplayName != user.DisplayName {
		t.Fatal("the current account did not reflect the profile update")
	}
	public := authRequest(t, handler, "GET", "/users/Alice", "", nil)
	expectStatus(t, public, 200)
	var author posts.Author
	if err := json.Unmarshal(public.Body.Bytes(), &author); err != nil || author.DisplayName != user.DisplayName ||
		strings.Contains(public.Body.String(), "email") {
		t.Fatal("public profile data is incorrect")
	}
	if got := responsePost(t, authRequest(t, handler, "GET", "/posts/"+post.ID, "", nil)); got.Author.DisplayName != user.DisplayName {
		t.Fatal("post author data did not reflect the profile update")
	}
	if got := responseUser(t, authRequest(t, handler, "GET", "/auth/me", "", other)); got.Username != "alice" || got.DisplayName != "alice" {
		t.Fatal("a case-distinct account was changed")
	}
	partial := authRequest(t, handler, "PATCH", "/users/me", `{"bio":""}`, cookie)
	expectStatus(t, partial, 200)
	if got := responseUser(t, partial); got.DisplayName != user.DisplayName || got.Bio != "" {
		t.Fatal("a partial update changed an omitted field")
	}
}

func TestProfileValidation(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "alice")
	handler, _ := testRoutesWithMedia(t, accounts, pool)
	for _, body := range []string{
		"{}", "null", `{"display_name":null,"bio":"Should not be saved"}`,
		`{"display_name":42}`, `{"display_name":"   "}`, `{"display_name":"a\u0000b"}`,
		`{"bio":null}`, `{"bio":[]}`, `{"bio":"a\u0000b"}`,
		`{"username":"other"}`, `{"email":"other@example.test"}`, `{"id":"other"}`,
		`{"avatar_id":""}`, `{"avatar_id":"not-a-uuid"}`, `{"avatar_id":true}`,
		`{"display_name":"Should not be saved","avatar_id":"11111111-1111-4111-8111-111111111111"}`,
		jsonBody(t, map[string]string{"display_name": strings.Repeat("\U0001f3a8", 61)}),
		jsonBody(t, map[string]string{"display_name": "Should not be saved", "bio": strings.Repeat("x", 281)}),
		`{"bio":"Valid"} {}`,
	} {
		response := authRequest(t, handler, "PATCH", "/users/me", body, cookie)
		expectStatus(t, response, 400)
	}
	current := responseUser(t, authRequest(t, handler, "GET", "/auth/me", "", cookie))
	if current.DisplayName != "alice" || current.Bio != "" || current.AvatarURL != nil {
		t.Fatal("an invalid update changed the profile")
	}
	allowed := authRequest(t, handler, "PATCH", "/users/me", jsonBody(t, map[string]string{
		"display_name": strings.Repeat("\U0001f3a8", 60), "bio": strings.Repeat("\U0001f331", 280),
	}), cookie)
	expectStatus(t, allowed, 200)
}

func TestAvatarLifecycle(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, alice := postAccount(t, accounts, "alice")
	_, bob := postAccount(t, accounts, "bob")
	handler, directory := testRoutesWithMedia(t, accounts, pool)
	firstUpload := imageUpload(t, handler, alice, imageFixture(t, "jpeg"))
	expectStatus(t, firstUpload, 201)
	first := uploadedImage(t, firstUpload)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+first.ID, "", bob), 404)
	denied := authRequest(t, handler, "PATCH", "/users/me",
		jsonBody(t, map[string]string{"display_name": "Not saved", "avatar_id": first.ID}), bob)
	expectStatus(t, denied, 403)
	if responseUser(t, authRequest(t, handler, "GET", "/auth/me", "", bob)).DisplayName != "bob" {
		t.Fatal("an unauthorized avatar update partially changed the profile")
	}
	saved := authRequest(t, handler, "PATCH", "/users/me", jsonBody(t, map[string]string{"avatar_id": first.ID}), alice)
	expectStatus(t, saved, 200)
	if user := responseUser(t, saved); user.AvatarURL == nil || *user.AvatarURL != first.URL {
		t.Fatal("avatar was not linked to the account")
	}
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+first.ID, "", nil), 200)
	expectStatus(t, authRequest(t, handler, "DELETE", "/media/"+first.ID, "", alice), 409)
	assertFileCount(t, directory, 1)

	secondUpload := imageUpload(t, handler, alice, imageFixture(t, "webp"))
	expectStatus(t, secondUpload, 201)
	second := uploadedImage(t, secondUpload)
	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me",
		jsonBody(t, map[string]string{"avatar_id": second.ID}), alice), 200)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+first.ID, "", alice), 404)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+second.ID, "", nil), 200)
	assertFileCount(t, directory, 1)

	unchanged := authRequest(t, handler, "PATCH", "/users/me", `{"display_name":"Alice Updated"}`, alice)
	expectStatus(t, unchanged, 200)
	if user := responseUser(t, unchanged); user.AvatarURL == nil || *user.AvatarURL != second.URL {
		t.Fatal("omitting avatar_id did not preserve the avatar")
	}
	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me",
		jsonBody(t, map[string]string{"avatar_id": strings.ToUpper(second.ID)}), alice), 200)
	assertFileCount(t, directory, 1)
	cleared := authRequest(t, handler, "PATCH", "/users/me", `{"avatar_id":null}`, alice)
	expectStatus(t, cleared, 200)
	if responseUser(t, cleared).AvatarURL != nil {
		t.Fatal("avatar was not removed")
	}
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+second.ID, "", nil), 404)
	assertFileCount(t, directory, 0)
}

func TestAvatarsCanShareOwnedPostImages(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "alice")
	handler, directory := testRoutesWithMedia(t, accounts, pool)
	upload := imageUpload(t, handler, cookie, imageFixture(t, "png"))
	expectStatus(t, upload, 201)
	shared := uploadedImage(t, upload)
	created := authRequest(t, handler, "POST", "/posts",
		jsonBody(t, map[string]any{"content": "", "media_ids": []string{shared.ID}}), cookie)
	expectStatus(t, created, 201)
	post := responsePost(t, created)
	setShared := jsonBody(t, map[string]string{"avatar_id": shared.ID})
	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me", setShared, cookie), 200)
	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me", `{"avatar_id":null}`, cookie), 200)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+shared.ID, "", nil), 200)
	assertFileCount(t, directory, 1)
	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me", setShared, cookie), 200)

	replacement := imageUpload(t, handler, cookie, imageFixture(t, "jpeg"))
	expectStatus(t, replacement, 201)
	other := uploadedImage(t, replacement)
	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me",
		jsonBody(t, map[string]string{"avatar_id": other.ID}), cookie), 200)
	assertFileCount(t, directory, 2)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+shared.ID, "", nil), 200)
	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me", `{"avatar_id":null}`, cookie), 200)
	assertFileCount(t, directory, 1)

	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me", setShared, cookie), 200)
	expectStatus(t, authRequest(t, handler, "DELETE", "/posts/"+post.ID, "", cookie), 204)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+shared.ID, "", nil), 200)
	assertFileCount(t, directory, 1)
	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me", `{"avatar_id":null}`, cookie), 200)
	assertFileCount(t, directory, 0)
}

func TestDiscardWaitsForAvatarSave(t *testing.T) {
	db, accounts, pool := authDatabase(t)
	user, cookie := postAccount(t, accounts, "alice")
	handler, directory := testRoutesWithMedia(t, accounts, pool)
	upload := imageUpload(t, handler, cookie, imageFixture(t, "png"))
	expectStatus(t, upload, 201)
	asset := uploadedImage(t, upload)
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	var id string
	if err := tx.QueryRow(t.Context(), "SELECT id::text FROM media WHERE id = $1 FOR UPDATE", asset.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- authRequest(t, handler, "DELETE", "/media/"+asset.ID, "", cookie) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var waiting bool
		err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			WHERE $1::integer = ANY(pg_blocking_pids(pid)))`, int32(tx.Conn().PgConn().PID())).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("discard did not wait for the image lock")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := tx.Exec(t.Context(), "UPDATE users SET avatar_id = $1 WHERE id = $2", asset.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, <-done, 409)
	current := responseUser(t, authRequest(t, handler, "GET", "/auth/me", "", cookie))
	if current.AvatarURL == nil || *current.AvatarURL != asset.URL {
		t.Fatal("concurrent discard removed a newly saved avatar")
	}
	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me", `{"avatar_id":null}`, cookie), 200)
	assertFileCount(t, directory, 0)
}
