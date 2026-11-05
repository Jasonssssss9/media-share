//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Jasonssssss9/media-share/backend/internal/config"
	"github.com/Jasonssssss9/media-share/backend/internal/posts"
	"github.com/Jasonssssss9/media-share/backend/internal/search"
)

func TestSearchPosts(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, alice := postAccount(t, accounts, "Alice")
	_, token, err := accounts.Register(context.Background(), "alice", "reader@example.test", "test-only password")
	if err != nil {
		t.Fatal(err)
	}
	reader := &http.Cookie{Name: "mediashare_session", Value: token}
	client := testSearchClient(t)
	generator := testGenerator(func(context.Context, string) ([]byte, error) {
		return imageFixture(t, "jpeg"), nil
	})
	handler, _, _ := testRoutesWithSearch(t, accounts, pool, generator, client)
	uploaded := imageUpload(t, handler, alice, imageFixture(t, "png"))
	expectStatus(t, uploaded, 201)
	generated := authRequest(t, handler, "POST", "/media/generation", jsonBody(t, map[string]string{"prompt": "A quiet lake"}), alice)
	expectStatus(t, generated, 201)
	ids := []string{uploadedImage(t, uploaded).ID, uploadedImage(t, generated).ID}
	created := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]any{
		"content": "Quiet lake photography", "media_ids": ids,
	}), alice)
	expectStatus(t, created, 201)
	post := responsePost(t, created)
	second := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]string{"content": "Quiet lake sketch"}), reader)
	expectStatus(t, second, 201)
	readerPost := responsePost(t, second)

	for _, test := range []struct {
		query string
		ids   []string
	}{
		{"q=QUIET+lake", []string{post.ID, readerPost.ID}},
		{"q=lake&author=Alice", []string{post.ID}},
		{"q=lake&author=alice", []string{readerPost.ID}},
		{"q=lake&author=ALICE", []string{}},
		{"q=quiet+mountain", []string{}},
		{"author=Alice", []string{post.ID}},
	} {
		page := searchPage(t, handler, test.query, nil)
		if page.Total != int64(len(test.ids)) || len(page.Items) != len(test.ids) {
			t.Fatalf("%s: unexpected matches: %+v", test.query, page)
		}
		found := map[string]bool{}
		for _, item := range page.Items {
			found[item.ID] = true
		}
		for _, id := range test.ids {
			if !found[id] {
				t.Fatalf("%s: missing post %s", test.query, id)
			}
		}
	}
	result := searchPage(t, handler, "q=photography", nil).Items[0]
	assertImageOrder(t, result, ids)
	for _, image := range result.Images {
		expectStatus(t, authRequest(t, handler, "GET", "/media/"+image.ID, "", nil), 200)
	}
	expectStatus(t, authRequest(t, handler, "PUT", "/posts/"+post.ID+"/like", "", reader), 200)
	expectStatus(t, authRequest(t, handler, "POST", "/posts/"+post.ID+"/comments", `{"content":"Beautiful lake"}`, reader), 201)
	expectStatus(t, authRequest(t, handler, "PATCH", "/users/me",
		jsonBody(t, map[string]any{"display_name": "Lake Photographer", "avatar_id": ids[0]}), alice), 200)
	for _, test := range []struct {
		cookie *http.Cookie
		liked  bool
	}{{nil, false}, {reader, true}, {&http.Cookie{Name: "mediashare_session", Value: "expired"}, false}} {
		item := searchPage(t, handler, "q=photography", test.cookie).Items[0]
		if item.LikeCount != 1 || item.CommentCount != 1 || item.LikedByMe != test.liked ||
			item.Author.DisplayName != "Lake Photographer" || (item.Author.AvatarURL == nil || *item.Author.AvatarURL != "/api/v1/media/"+ids[0]) {
			t.Fatal("search did not read current profile and interaction data from PostgreSQL")
		}
	}

	edited := authRequest(t, handler, "PATCH", "/posts/"+post.ID, editBody(t, post, "Mountain trail photography"), alice)
	expectStatus(t, edited, 200)
	if page := searchPage(t, handler, "q=lake&author=Alice", nil); page.Total != 0 || len(page.Items) != 0 {
		t.Fatal("old text remained searchable after editing")
	}
	if page := searchPage(t, handler, "q=mountain", nil); len(page.Items) != 1 || page.Items[0].Content != "Mountain trail photography" {
		t.Fatal("edited text was not immediately searchable")
	}
	expectStatus(t, authRequest(t, handler, "DELETE", "/posts/"+strings.ToUpper(post.ID), "", alice), 204)
	if page := searchPage(t, handler, "q=mountain", nil); page.Total != 0 || len(page.Items) != 0 {
		t.Fatal("deleting with an uppercase UUID left an indexed post")
	}

	imageOnly := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]any{
		"content": "", "media_ids": []string{ids[0]},
	}), alice)
	expectStatus(t, imageOnly, 201)
	if page := searchPage(t, handler, "author=Alice", nil); len(page.Items) != 1 || page.Items[0].Content != "" || len(page.Items[0].Images) != 1 {
		t.Fatal("author search omitted an image-only post")
	}
}

func TestSearchRebuildAndStaleResults(t *testing.T) {
	db, accounts, pool := authDatabase(t)
	owner, _ := postAccount(t, accounts, "Alice")
	client := testSearchClient(t)
	handler, _, service := testRoutesWithSearch(t, accounts, pool, testGenerator(nil), client)
	createdAt := time.Date(2026, 1, 1, 12, 0, 0, 123456000, time.UTC)
	for i := 1; i <= 105; i++ {
		mustExec(t, context.Background(), db, `
			INSERT INTO posts (id, author_id, content, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)`, fixturePostID(i), owner.ID, fmt.Sprintf("Forest walk %d", i), createdAt)
	}
	if page := searchPage(t, handler, "q=forest", nil); page.Total != 0 {
		t.Fatal("test index should start empty")
	}
	ctx := context.Background()
	count, err := service.Rebuild(ctx)
	if err != nil || count != 105 {
		t.Fatalf("rebuild count=%d: %v", count, err)
	}
	for _, test := range []struct {
		query string
		first int
		size  int
	}{
		{"q=forest&page_size=50", 105, 50},
		{"q=forest&page=2&page_size=50", 55, 50},
		{"q=forest&page=3&page_size=50", 5, 5},
	} {
		page := searchPage(t, handler, test.query, nil)
		if page.Total != 105 || len(page.Items) != test.size {
			t.Fatalf("unexpected page: %+v", page)
		}
		for i, item := range page.Items {
			if item.ID != fixturePostID(test.first-i) {
				t.Fatal("search order changed while hydrating PostgreSQL posts")
			}
		}
	}

	// Simulate missed synchronization after a committed database change.
	mustExec(t, ctx, db, "UPDATE posts SET content = 'Ocean waves', updated_at = updated_at + interval '1 second' WHERE id = $1", fixturePostID(105))
	mustExec(t, ctx, db, "DELETE FROM posts WHERE id = $1", fixturePostID(104))
	stale := searchPage(t, handler, "q=forest&page_size=50", nil)
	if stale.Total != 105 || len(stale.Items) != 48 {
		t.Fatal("stale or deleted hits should be skipped until synchronization is repaired")
	}
	for _, item := range stale.Items {
		if item.ID == fixturePostID(105) || item.ID == fixturePostID(104) {
			t.Fatal("a stale indexed hit leaked into search results")
		}
	}
	if err := service.Sync(ctx, fixturePostID(105)); err != nil {
		t.Fatal(err)
	}
	if err := service.Sync(ctx, fixturePostID(104)); err != nil {
		t.Fatal(err)
	}
	if page := searchPage(t, handler, "q=ocean", nil); page.Total != 1 || page.Items[0].Content != "Ocean waves" {
		t.Fatal("synchronization did not read the latest database revision")
	}
	if page := searchPage(t, handler, "q=forest", nil); page.Total != 103 {
		t.Fatal("synchronization did not remove a deleted database post")
	}
	if err := client.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, authRequest(t, handler, "GET", "/search?q=forest", "", nil), 503)
	count, err = service.Rebuild(ctx)
	if err != nil || count != 104 {
		t.Fatalf("rebuild after clearing the index count=%d: %v", count, err)
	}
	if page := searchPage(t, handler, "q=forest", nil); page.Total != 103 {
		t.Fatal("rebuilding a cleared index did not restore searchable posts")
	}
	if page := searchPage(t, handler, "q=ocean", nil); page.Total != 1 {
		t.Fatal("rebuild lost the latest text")
	}
	for _, query := range []string{
		"", "q=%20", "q=" + url.QueryEscape(strings.Repeat("x", 201)), "q=bad%00text",
		"author=Al", "author=Alice!", "author=" + strings.Repeat("a", 25),
		"q=forest&page=0", "q=forest&page_size=0", "q=forest&page=501",
		"q=forest&page=9223372036854775807",
	} {
		expectStatus(t, authRequest(t, handler, "GET", "/search?"+query, "", nil), 400)
	}
	if page := searchPage(t, handler, "q=forest&page=500", nil); page.Total != 103 || len(page.Items) != 0 {
		t.Fatal("the last allowed result window should remain valid")
	}
}

func TestSearchFailureDoesNotUndoPosts(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "Alice")
	unavailable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":{"type":"unavailable","reason":"private diagnostic"}}`))
	}))
	defer unavailable.Close()
	client, err := search.NewClient(unavailable.URL, "test-only", "test-posts")
	if err != nil {
		t.Fatal(err)
	}
	handler, _, _ := testRoutesWithSearch(t, accounts, pool, testGenerator(nil), client)
	created := authRequest(t, handler, "POST", "/posts", `{"content":"Saved without search"}`, cookie)
	expectStatus(t, created, 201)
	post := responsePost(t, created)
	edited := authRequest(t, handler, "PATCH", "/posts/"+post.ID, editBody(t, post, "Still saved without search"), cookie)
	expectStatus(t, edited, 200)
	detail := authRequest(t, handler, "GET", "/posts/"+post.ID, "", nil)
	expectStatus(t, detail, 200)
	if responsePost(t, detail).Content != "Still saved without search" {
		t.Fatal("index failure changed a committed edit")
	}
	failed := authRequest(t, handler, "GET", "/search?q=saved", "", nil)
	expectStatus(t, failed, 503)
	if !strings.Contains(failed.Body.String(), "search_unavailable") || strings.Contains(failed.Body.String(), "private diagnostic") {
		t.Fatal("search failure should return a clear message without raw upstream details")
	}
	expectStatus(t, authRequest(t, handler, "DELETE", "/posts/"+post.ID, "", cookie), 204)
	expectStatus(t, authRequest(t, handler, "GET", "/posts/"+post.ID, "", nil), 404)
}

func testSearchClient(t *testing.T) *search.Client {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	client, err := search.NewClient(cfg.ElasticsearchURL, cfg.ElasticsearchPassword, fmt.Sprintf("media-share-test-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Drop(ctx); err != nil {
			t.Errorf("clean up search index: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.EnsureIndex(ctx); err != nil {
		t.Fatalf("start Elasticsearch with make services before search integration tests: %v", err)
	}
	return client
}

func searchPage(t *testing.T, handler http.Handler, query string, cookie *http.Cookie) posts.Page {
	t.Helper()
	response := authRequest(t, handler, "GET", "/search?"+query, "", cookie)
	expectStatus(t, response, 200)
	if response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), `"email"`) {
		t.Fatal("search response exposed private account fields or allowed caching")
	}
	return postPage(t, response)
}
