//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Jasonssssss9/media-share/backend/internal/posts"
)

func TestLikesAndViewerState(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, alice := postAccount(t, accounts, "alice")
	_, bob := postAccount(t, accounts, "bob")
	handler := testRoutes(t, accounts, pool)
	created := authRequest(t, handler, "POST", "/posts", `{"content":"A post with interactions."}`, alice)
	expectStatus(t, created, 201)
	post := responsePost(t, created)
	path := "/posts/" + post.ID
	for _, method := range []string{"PUT", "DELETE"} {
		expectStatus(t, authRequest(t, handler, method, path+"/like", "", nil), 401)
		expectStatus(t, authRequest(t, handler, method, "/posts/not-a-uuid/like", "", bob), 400)
		expectStatus(t, authRequest(t, handler, method, "/posts/"+fixturePostID(99)+"/like", "", bob), 404)
	}
	for range 2 {
		response := authRequest(t, handler, "PUT", path+"/like", "", bob)
		expectStatus(t, response, 200)
		state := responseLike(t, response)
		if state.LikeCount != 1 || !state.LikedByMe {
			t.Fatal("repeated likes changed the count")
		}
	}
	expectStatus(t, authRequest(t, handler, "PUT", path+"/like", "", alice), 200)
	for _, viewer := range []*http.Cookie{nil, alice, bob} {
		response := authRequest(t, handler, "GET", path, "", viewer)
		expectStatus(t, response, 200)
		got := responsePost(t, response)
		if got.LikeCount != 2 || got.LikedByMe != (viewer != nil) || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("post like state did not match its viewer")
		}
		page := postPage(t, authRequest(t, handler, "GET", "/posts?author=alice", "", viewer))
		if len(page.Items) != 1 || page.Items[0].LikedByMe != (viewer != nil) || page.Items[0].LikeCount != 2 {
			t.Fatal("feed like state did not match its viewer")
		}
	}
	forged := &http.Cookie{Name: "mediashare_session", Value: "invalid-token"}
	if got := responsePost(t, authRequest(t, handler, "GET", path, "", forged)); got.LikedByMe || got.LikeCount != 2 {
		t.Fatal("an invalid public session was not treated as a guest")
	}
	for range 2 {
		response := authRequest(t, handler, "DELETE", path+"/like", "", bob)
		expectStatus(t, response, 200)
		if state := responseLike(t, response); state.LikedByMe || state.LikeCount != 1 {
			t.Fatal("repeated unlike requests changed another user's like")
		}
	}
	expectStatus(t, authRequest(t, handler, "POST", path+"/comments", `{"content":"A comment."}`, bob), 201)
	updated := authRequest(t, handler, "PATCH", path, editBody(t, post, "Edited with interactions."), alice)
	expectStatus(t, updated, 200)
	current := responsePost(t, updated)
	if current.LikeCount != 1 || current.CommentCount != 1 || !current.LikedByMe {
		t.Fatal("editing a post lost its interaction state")
	}
}

func TestConcurrentLikesStayUnique(t *testing.T) {
	db, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "alice")
	handler := testRoutes(t, accounts, pool)
	post := responsePost(t, authRequest(t, handler, "POST", "/posts", `{"content":"Concurrent likes."}`, cookie))
	results := make(chan *httptest.ResponseRecorder, 6)
	for range 6 {
		go func() { results <- authRequest(t, handler, "PUT", "/posts/"+post.ID+"/like", "", cookie) }()
	}
	for range 6 {
		response := <-results
		expectStatus(t, response, 200)
		if state := responseLike(t, response); state.LikeCount != 1 || !state.LikedByMe {
			t.Fatal("concurrent duplicate likes were counted twice")
		}
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM likes WHERE post_id = $1", post.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unexpected stored like count: %d (%v)", count, err)
	}
	for range 6 {
		go func() { results <- authRequest(t, handler, "DELETE", "/posts/"+post.ID+"/like", "", cookie) }()
	}
	for range 6 {
		response := <-results
		expectStatus(t, response, 200)
		if state := responseLike(t, response); state.LikeCount != 0 || state.LikedByMe {
			t.Fatal("concurrent unlike requests produced an incorrect count")
		}
	}
}

func TestCommentPermissionsAndCounts(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, alice := postAccount(t, accounts, "alice")
	_, bob := postAccount(t, accounts, "bob")
	_, carol := postAccount(t, accounts, "carol")
	handler := testRoutes(t, accounts, pool)
	post := responsePost(t, authRequest(t, handler, "POST", "/posts", `{"content":"A comment thread."}`, alice))
	path := "/posts/" + post.ID
	expectStatus(t, authRequest(t, handler, "POST", path+"/comments", `{"content":"Guest"}`, nil), 401)
	empty := responseComments(t, authRequest(t, handler, "GET", path+"/comments", "", nil))
	if empty.Total != 0 || empty.Items == nil || empty.PageSize != 20 {
		t.Fatal("empty comment list did not follow the pagination contract")
	}
	created := authRequest(t, handler, "POST", path+"/comments", `{"content":"  First comment.\nA second line.  "}`, bob)
	expectStatus(t, created, 201)
	comment := responseComment(t, created)
	if comment.Content != "First comment.\nA second line." || comment.Author.Username != "bob" ||
		comment.PostID != post.ID || created.Header().Get("Location") != "/api/v1/comments/"+comment.ID {
		t.Fatal("created comment data is incorrect")
	}
	commentPath := "/comments/" + comment.ID
	expectStatus(t, authRequest(t, handler, "PATCH", commentPath, `{"content":"Guest"}`, nil), 401)
	expectStatus(t, authRequest(t, handler, "DELETE", commentPath, "", nil), 401)
	for _, cookie := range []*http.Cookie{alice, carol} {
		expectStatus(t, authRequest(t, handler, "PATCH", commentPath, `{"content":"Not allowed"}`, cookie), 403)
	}
	expectStatus(t, authRequest(t, handler, "DELETE", commentPath, "", carol), 403)
	edited := authRequest(t, handler, "PATCH", commentPath, `{"content":"Edited comment."}`, bob)
	expectStatus(t, edited, 200)
	current := responseComment(t, edited)
	if current.Content != "Edited comment." || !current.CreatedAt.Equal(comment.CreatedAt) || !current.UpdatedAt.After(comment.UpdatedAt) {
		t.Fatal("comment edit did not preserve creation time and update its content")
	}
	list := authRequest(t, handler, "GET", path+"/comments", "", nil)
	expectStatus(t, list, 200)
	if strings.Contains(list.Body.String(), "email") || strings.Contains(list.Body.String(), "password_hash") ||
		responseComments(t, list).Items[0].Content != "Edited comment." {
		t.Fatal("public comments exposed private data or stale content")
	}
	if got := responsePost(t, authRequest(t, handler, "GET", path, "", nil)); got.CommentCount != 1 || !got.UpdatedAt.Equal(post.UpdatedAt) {
		t.Fatal("comment changes altered post edit time or lost their count")
	}
	expectStatus(t, authRequest(t, handler, "DELETE", commentPath, "", alice), 204)
	expectStatus(t, authRequest(t, handler, "PATCH", commentPath, `{"content":"Deleted"}`, bob), 404)
	expectStatus(t, authRequest(t, handler, "DELETE", commentPath, "", bob), 404)
	second := authRequest(t, handler, "POST", path+"/comments", `{"content":"Author can delete this."}`, bob)
	expectStatus(t, second, 201)
	expectStatus(t, authRequest(t, handler, "DELETE", "/comments/"+responseComment(t, second).ID, "", bob), 204)
	if got := responsePost(t, authRequest(t, handler, "GET", path, "", nil)); got.CommentCount != 0 {
		t.Fatal("deleting comments did not update the post count")
	}
}

func TestCommentValidationPaginationAndCascade(t *testing.T) {
	db, accounts, pool := authDatabase(t)
	alice, cookie := postAccount(t, accounts, "alice")
	handler := testRoutes(t, accounts, pool)
	post := responsePost(t, authRequest(t, handler, "POST", "/posts", `{"content":"A paginated thread."}`, cookie))
	path := "/posts/" + post.ID
	for _, body := range []string{
		"{}", `{"content":null}`, `{"content":"   "}`, `{"content":42}`,
		`{"content":"a\u0000b"}`, `{"content":"Valid","author_id":"other"}`,
		`{"content":"Valid"} {}`, jsonBody(t, map[string]string{"content": strings.Repeat("\U0001f331", 501)}),
	} {
		expectStatus(t, authRequest(t, handler, "POST", path+"/comments", body, cookie), 400)
	}
	valid := authRequest(t, handler, "POST", path+"/comments",
		jsonBody(t, map[string]string{"content": strings.Repeat("\U0001f331", 500)}), cookie)
	expectStatus(t, valid, 201)
	expectStatus(t, authRequest(t, handler, "PATCH", "/comments/"+responseComment(t, valid).ID, `{"content":""}`, cookie), 400)
	expectStatus(t, authRequest(t, handler, "DELETE", "/comments/"+responseComment(t, valid).ID, "", cookie), 204)
	for i := 1; i <= 55; i++ {
		_, err := db.Exec(`INSERT INTO comments (id, post_id, author_id, content, created_at, updated_at)
			VALUES ($1, $2, $3, $4, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
			commentFixtureID(i), post.ID, alice.ID, fmt.Sprintf("Comment %d", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	first := responseComments(t, authRequest(t, handler, "GET", path+"/comments?page_size=5", "", nil))
	second := responseComments(t, authRequest(t, handler, "GET", path+"/comments?page=2&page_size=5", "", nil))
	if first.Total != 55 || len(first.Items) != 5 || first.Items[0].ID != commentFixtureID(55) || second.Items[0].ID != commentFixtureID(50) {
		t.Fatal("newest-first pagination was not deterministic for equal timestamps")
	}
	capped := responseComments(t, authRequest(t, handler, "GET", path+"/comments?page_size=999", "", nil))
	if capped.PageSize != 50 || len(capped.Items) != 50 {
		t.Fatal("comment page size was not capped")
	}
	far := responseComments(t, authRequest(t, handler, "GET", path+"/comments?page=9223372036854775807", "", nil))
	if far.Total != 55 || len(far.Items) != 0 {
		t.Fatal("an out-of-range page did not return an empty list")
	}
	for _, suffix := range []string{"?page=0", "?page=-1", "?page=invalid", "?page_size=0"} {
		expectStatus(t, authRequest(t, handler, "GET", path+"/comments"+suffix, "", nil), 400)
	}
	for _, test := range []struct{ method, path, body string }{
		{"PUT", path + "/like", ""},
		{"DELETE", path + "/like", ""},
		{"POST", path + "/comments", `{"content":"Cross-site"}`},
		{"PATCH", "/comments/" + commentFixtureID(1), `{"content":"Cross-site"}`},
		{"DELETE", "/comments/" + commentFixtureID(1), ""},
	} {
		request := httptest.NewRequest(test.method, "http://api.test/api/v1"+test.path, strings.NewReader(test.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "http://untrusted.test")
		request.Header.Set("Sec-Fetch-Site", "cross-site")
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		expectStatus(t, response, 403)
	}
	expectStatus(t, authRequest(t, handler, "PUT", path+"/like", "", cookie), 200)
	expectStatus(t, authRequest(t, handler, "DELETE", path, "", cookie), 204)
	for _, table := range []string{"likes", "comments"} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM "+table+" WHERE post_id = $1", post.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("post deletion left %s: %v", table, err)
		}
	}
	expectStatus(t, authRequest(t, handler, "GET", path+"/comments", "", nil), 404)
	expectStatus(t, authRequest(t, handler, "POST", path+"/comments", `{"content":"Deleted post"}`, cookie), 404)
	expectStatus(t, authRequest(t, handler, "PATCH", "/comments/"+commentFixtureID(1), `{"content":"Deleted comment"}`, cookie), 404)
}

func TestCommentChangesDuringPostDeletion(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, alice := postAccount(t, accounts, "alice")
	_, bob := postAccount(t, accounts, "bob")
	handler := testRoutes(t, accounts, pool)
	for _, method := range []string{"PATCH", "DELETE"} {
		post := responsePost(t, authRequest(t, handler, "POST", "/posts", `{"content":"Concurrent deletion."}`, alice))
		comment := responseComment(t, authRequest(t, handler, "POST", "/posts/"+post.ID+"/comments", `{"content":"A comment."}`, bob))
		done := make(chan *httptest.ResponseRecorder, 1)
		go func() { done <- authRequest(t, handler, method, "/comments/"+comment.ID, `{"content":"Edited."}`, bob) }()
		expectStatus(t, authRequest(t, handler, "DELETE", "/posts/"+post.ID, "", alice), 204)
		response := <-done
		if response.Code != 200 && response.Code != 204 && response.Code != 404 {
			t.Fatalf("concurrent comment change failed: %d %s", response.Code, response.Body.String())
		}
	}
}

func responseLike(t *testing.T, response *httptest.ResponseRecorder) posts.LikeState {
	t.Helper()
	var result posts.LikeState
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func responseComment(t *testing.T, response *httptest.ResponseRecorder) posts.Comment {
	t.Helper()
	var result posts.Comment
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func responseComments(t *testing.T, response *httptest.ResponseRecorder) posts.CommentPage {
	t.Helper()
	var result posts.CommentPage
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func commentFixtureID(i int) string { return fmt.Sprintf("40000000-0000-4000-8000-%012d", i) }
