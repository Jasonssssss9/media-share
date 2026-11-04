//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"github.com/Jasonssssss9/media-share/backend/internal/posts"
)

func TestPostsFlow(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	alice, aliceCookie := postAccount(t, accounts, "alice")
	_, bobCookie := postAccount(t, accounts, "bob")
	handler := testRoutes(t, accounts, pool)

	empty := authRequest(t, handler, "GET", "/posts", "", nil)
	expectStatus(t, empty, 200)
	if page := postPage(t, empty); page.Items == nil || len(page.Items) != 0 || page.Total != 0 {
		t.Fatal("an empty feed should return an empty array and zero total")
	}
	expectStatus(t, authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]string{"content": "Guest post"}), nil), 401)

	content := "First line\n<script>This stays plain text.</script>"
	created := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]string{"content": content}), aliceCookie)
	expectStatus(t, created, 201)
	post := responsePost(t, created)
	if post.Author.ID != alice.ID || post.Content != content || post.CreatedAt.IsZero() || !post.CreatedAt.Equal(post.UpdatedAt) {
		t.Fatalf("unexpected created post: %+v", post)
	}
	if created.Header().Get("Location") != "/api/v1/posts/"+post.ID {
		t.Fatal("creation did not return the post location")
	}
	path := "/posts/" + post.ID
	detail := authRequest(t, handler, "GET", path, "", nil)
	expectStatus(t, detail, 200)
	if responsePost(t, detail).Content != content {
		t.Fatal("public detail lost the post content")
	}

	bobPost := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]string{"content": "Bob's post"}), bobCookie)
	expectStatus(t, bobPost, 201)
	bobID := responsePost(t, bobPost).ID
	filtered := authRequest(t, handler, "GET", "/posts?author=alice", "", nil)
	expectStatus(t, filtered, 200)
	if page := postPage(t, filtered); page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != post.ID {
		t.Fatal("author filtering included another user's post")
	}
	profile := authRequest(t, handler, "GET", "/users/alice", "", nil)
	expectStatus(t, profile, 200)
	if strings.Contains(profile.Body.String(), `"email"`) || strings.Contains(profile.Body.String(), "password_hash") {
		t.Fatal("public profile exposed private account fields")
	}
	for _, method := range []string{"PATCH", "DELETE"} {
		body := ""
		if method == "PATCH" {
			body = editBody(t, post, "Unauthorized change")
		}
		expectStatus(t, authRequest(t, handler, method, path, body, nil), 401)
		expectStatus(t, authRequest(t, handler, method, path, body, bobCookie), 403)
	}
	updated := authRequest(t, handler, "PATCH", path, editBody(t, post, "Updated content"), aliceCookie)
	expectStatus(t, updated, 200)
	changed := responsePost(t, updated)
	if changed.Content != "Updated content" || !changed.UpdatedAt.After(post.UpdatedAt) || !changed.CreatedAt.Equal(post.CreatedAt) {
		t.Fatal("edit did not preserve creation time and update the content and modification time")
	}
	stale := authRequest(t, handler, "PATCH", path, editBody(t, post, "Stale content"), aliceCookie)
	expectStatus(t, stale, 409)
	if got := responsePost(t, authRequest(t, handler, "GET", path, "", nil)); got.Content != changed.Content {
		t.Fatal("a stale edit overwrote the latest content")
	}
	expectStatus(t, authRequest(t, handler, "DELETE", path, "", aliceCookie), 204)
	expectStatus(t, authRequest(t, handler, "GET", path, "", nil), 404)
	remaining := postPage(t, authRequest(t, handler, "GET", "/posts", "", nil))
	if remaining.Total != 1 || len(remaining.Items) != 1 || remaining.Items[0].ID != bobID {
		t.Fatal("deletion did not immediately update the feed")
	}
	expectStatus(t, authRequest(t, handler, "DELETE", "/posts/"+bobID, "", bobCookie), 204)
	if err := accounts.Logout(context.Background(), aliceCookie.Value); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]string{"content": "Revoked session"}), aliceCookie), 401)
}

func TestPostValidation(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "alice")
	handler := testRoutes(t, accounts, pool)
	created := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]string{"content": "Original content"}), cookie)
	expectStatus(t, created, 201)
	post := responsePost(t, created)
	missing := "/posts/11111111-1111-4111-8111-111111111111"

	for _, test := range []struct {
		name, method, path, body string
		status                   int
	}{
		{"empty", "POST", "/posts", jsonBody(t, map[string]string{"content": ""}), 400},
		{"null_character", "POST", "/posts", jsonBody(t, map[string]string{"content": "Invalid\x00text"}), 400},
		{"whitespace", "POST", "/posts", jsonBody(t, map[string]string{"content": " \n\t\u2003"}), 400},
		{"too_long", "POST", "/posts", jsonBody(t, map[string]string{"content": strings.Repeat("🔒", 2001)}), 400},
		{"spoofed_author", "POST", "/posts", jsonBody(t, map[string]string{"content": "Hello", "author_id": post.Author.ID}), 400},
		{"missing_content", "POST", "/posts", "{}", 400},
		{"missing_edit_time", "PATCH", "/posts/" + post.ID, jsonBody(t, map[string]string{"content": "Changed"}), 400},
		{"invalid_edit_time", "PATCH", "/posts/" + post.ID, jsonBody(t, map[string]string{"content": "Changed", "updated_at": "yesterday"}), 400},
		{"empty_edit", "PATCH", "/posts/" + post.ID, editBody(t, post, "   "), 400},
		{"invalid_id_read", "GET", "/posts/not-a-uuid", "", 400},
		{"invalid_id_edit", "PATCH", "/posts/not-a-uuid", editBody(t, post, "Changed"), 400},
		{"invalid_id_delete", "DELETE", "/posts/not-a-uuid", "", 400},
		{"missing_read", "GET", missing, "", 404},
		{"missing_edit", "PATCH", missing, editBody(t, post, "Changed"), 404},
		{"missing_delete", "DELETE", missing, "", 404},
		{"missing_author", "GET", "/users/missing_user", "", 404},
		{"invalid_page", "GET", "/posts?page=abc", "", 400},
		{"zero_page", "GET", "/posts?page=0", "", 400},
		{"negative_page", "GET", "/posts?page=-1", "", 400},
		{"invalid_page_size", "GET", "/posts?page_size=1.5", "", 400},
		{"zero_page_size", "GET", "/posts?page_size=0", "", 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			expectStatus(t, authRequest(t, handler, test.method, test.path, test.body, cookie), test.status)
		})
	}
	maximum := strings.Repeat("🔒", 2000)
	valid := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]string{"content": maximum}), cookie)
	expectStatus(t, valid, 201)
	if responsePost(t, valid).Content != maximum {
		t.Fatal("valid Unicode content was truncated")
	}

	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		path, body := "/api/v1/posts/"+post.ID, ""
		if method == "POST" {
			path = "/api/v1/posts"
			body = jsonBody(t, map[string]string{"content": "Cross-site post"})
		}
		if method == "PATCH" {
			body = editBody(t, post, "Cross-site edit")
		}
		request := httptest.NewRequest(method, "http://api.test"+path, strings.NewReader(body))
		request.Header.Set("Origin", "http://untrusted.test")
		request.Header.Set("Sec-Fetch-Site", "cross-site")
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		expectStatus(t, response, 403)
	}
}

func TestPostPagination(t *testing.T) {
	db, accounts, pool := authDatabase(t)
	alice, _ := postAccount(t, accounts, "alice")
	bob, _ := postAccount(t, accounts, "bob")
	handler := testRoutes(t, accounts, pool)
	createdAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := 1; i <= 7; i++ {
		authorID := bob.ID
		if i%2 == 0 {
			authorID = alice.ID
		}
		mustExec(t, context.Background(), db, `
			INSERT INTO posts (id, author_id, content, created_at)
			VALUES ($1, $2, $3, $4)`, fixturePostID(i), authorID, fmt.Sprintf("Post %d", i), createdAt)
	}
	for _, test := range []struct {
		path       string
		ids        []int
		total      int64
		page, size int
	}{
		{"/posts?page_size=3", []int{7, 6, 5}, 7, 1, 3},
		{"/posts?page=2&page_size=3", []int{4, 3, 2}, 7, 2, 3},
		{"/posts?page=3&page_size=3", []int{1}, 7, 3, 3},
		{"/posts?page_size=3", []int{7, 6, 5}, 7, 1, 3},
		{"/posts?author=alice&page_size=2", []int{6, 4}, 3, 1, 2},
		{"/posts?author=alice&page_size=2&page=2", []int{2}, 3, 2, 2},
		{"/posts?author=missing_user", []int{}, 0, 1, 20},
		{"/posts?page=999", []int{}, 7, 999, 20},
		{"/posts?page=9223372036854775807", []int{}, 7, 9223372036854775807, 20},
		{"/posts?page_size=100", []int{7, 6, 5, 4, 3, 2, 1}, 7, 1, 50},
	} {
		response := authRequest(t, handler, "GET", test.path, "", nil)
		expectStatus(t, response, 200)
		page := postPage(t, response)
		if page.Total != test.total || page.Page != test.page || page.PageSize != test.size || len(page.Items) != len(test.ids) {
			t.Fatalf("%s: unexpected page: %+v", test.path, page)
		}
		for i, id := range test.ids {
			if page.Items[i].ID != fixturePostID(id) {
				t.Fatalf("%s: posts are out of order", test.path)
			}
		}
	}
}

func TestConcurrentPostEdits(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "alice")
	handler := testRoutes(t, accounts, pool)
	created := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]string{"content": "Original"}), cookie)
	expectStatus(t, created, 201)
	post := responsePost(t, created)
	results := make(chan *httptest.ResponseRecorder, 2)
	for _, content := range []string{"First edit", "Second edit"} {
		go func() {
			results <- authRequest(t, handler, "PATCH", "/posts/"+post.ID, editBody(t, post, content), cookie)
		}()
	}
	statuses := map[int]int{}
	for range 2 {
		response := <-results
		statuses[response.Code]++
		if response.Code != 200 && response.Code != 409 {
			t.Fatalf("unexpected concurrent edit: %s", response.Body.String())
		}
	}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatalf("both edits should not succeed: %v", statuses)
	}
}

func postAccount(t *testing.T, service *auth.Service, username string) (auth.User, *http.Cookie) {
	t.Helper()
	user, token, err := service.Register(context.Background(), username, username+"@example.test", "test-only password")
	if err != nil {
		t.Fatal(err)
	}
	return user, &http.Cookie{Name: "mediashare_session", Value: token}
}

func responsePost(t *testing.T, response *httptest.ResponseRecorder) posts.Post {
	t.Helper()
	if strings.Contains(response.Body.String(), `"email"`) || strings.Contains(response.Body.String(), "password_hash") {
		t.Fatal("post response exposed private account fields")
	}
	var post posts.Post
	if err := json.Unmarshal(response.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	return post
}

func postPage(t *testing.T, response *httptest.ResponseRecorder) posts.Page {
	t.Helper()
	var page posts.Page
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func editBody(t *testing.T, post posts.Post, content string) string {
	t.Helper()
	return jsonBody(t, map[string]string{"content": content, "updated_at": post.UpdatedAt.Format(time.RFC3339Nano)})
}

func fixturePostID(i int) string { return fmt.Sprintf("10000000-0000-4000-8000-%012d", i) }
