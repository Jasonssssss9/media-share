//go:build integration

package integration_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Jasonssssss9/media-share/backend/internal/media"
	"github.com/Jasonssssss9/media-share/backend/internal/posts"
)

func TestUploadAndPrivateImages(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	_, alice := postAccount(t, accounts, "alice")
	_, bob := postAccount(t, accounts, "bob")
	handler, directory := testRoutesWithMedia(t, accounts, pool)
	data := imageFixture(t, "png")
	expectStatus(t, imageUpload(t, handler, nil, data), 401)
	uploaded := imageUpload(t, handler, alice, data)
	expectStatus(t, uploaded, 201)
	asset := uploadedImage(t, uploaded)
	if asset.Width != 8 || asset.Height != 6 || asset.URL != "/api/v1/media/"+asset.ID {
		t.Fatal("wrong image metadata")
	}
	assertFileCount(t, directory, 1)
	path := "/media/" + asset.ID
	expectStatus(t, authRequest(t, handler, "GET", path, "", nil), 404)
	expectStatus(t, authRequest(t, handler, "GET", path, "", bob), 404)
	own := authRequest(t, handler, "GET", path, "", alice)
	expectStatus(t, own, 200)
	if !bytes.Equal(own.Body.Bytes(), data) || own.Header().Get("Content-Type") != "image/png" ||
		own.Header().Get("X-Content-Type-Options") != "nosniff" || own.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("image bytes or response headers are wrong")
	}
	expectStatus(t, authRequest(t, handler, "DELETE", path, "", bob), 403)
	expectStatus(t, authRequest(t, handler, "DELETE", path, "", alice), 204)
	assertFileCount(t, directory, 0)
	expectStatus(t, authRequest(t, handler, "GET", path, "", alice), 404)
}

func TestImageValidation(t *testing.T) {
	db, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "alice")
	handler, directory := testRoutesWithMedia(t, accounts, pool)
	for _, format := range []string{"png", "jpeg", "webp"} {
		response := imageUpload(t, handler, cookie, imageFixture(t, format))
		expectStatus(t, response, 201)
		asset := uploadedImage(t, response)
		if asset.Width < 1 || asset.Height < 1 {
			t.Fatal("missing decoded dimensions")
		}
		expectStatus(t, authRequest(t, handler, "DELETE", "/media/"+asset.ID, "", cookie), 204)
	}
	largeDimensions := append([]byte(nil), imageFixture(t, "png")...)
	binary.BigEndian.PutUint32(largeDimensions[16:20], 10000)
	binary.BigEndian.PutUint32(largeDimensions[20:24], 10000)
	binary.BigEndian.PutUint32(largeDimensions[29:33], crc32.ChecksumIEEE(largeDimensions[12:29]))
	for _, test := range []struct {
		name   string
		data   []byte
		status int
	}{
		{"empty", nil, 400},
		{"text", []byte("not an image"), 400},
		{"svg", []byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>"), 400},
		{"truncated_png", imageFixture(t, "png")[:33], 400},
		{"oversize", make([]byte, media.MaxSize+1), 413},
		{"too_many_pixels", largeDimensions, 400},
	} {
		t.Run(test.name, func(t *testing.T) { expectStatus(t, imageUpload(t, handler, cookie, test.data), test.status) })
	}
	expectStatus(t, authRequest(t, handler, "POST", "/media", "{}", cookie), 400)
	crossSite := httptest.NewRequest("POST", "http://api.test/api/v1/media", strings.NewReader("{}"))
	crossSite.Header.Set("Origin", "http://untrusted.test")
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	crossSite.AddCookie(cookie)
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, crossSite)
	expectStatus(t, denied, 403)

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for range 2 {
		part, err := writer.CreateFormFile("file", "photo.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(imageFixture(t, "png")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "http://api.test/api/v1/media", &buffer)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", "http://frontend.test")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	expectStatus(t, response, 400)
	assertFileCount(t, directory, 0)
	var count int
	if err := db.QueryRow("SELECT count(*) FROM media").Scan(&count); err != nil || count != 0 {
		t.Fatalf("rejected uploads left metadata: %v", err)
	}
}

func TestPostImages(t *testing.T) {
	db, accounts, pool := authDatabase(t)
	_, alice := postAccount(t, accounts, "alice")
	_, bob := postAccount(t, accounts, "bob")
	handler, directory := testRoutesWithMedia(t, accounts, pool)
	var ids []string
	for range 4 {
		uploaded := imageUpload(t, handler, alice, imageFixture(t, "png"))
		expectStatus(t, uploaded, 201)
		ids = append(ids, uploadedImage(t, uploaded).ID)
	}
	created := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]any{"content": "", "media_ids": ids}), alice)
	expectStatus(t, created, 201)
	post := responsePost(t, created)
	assertImageOrder(t, post, ids)
	for _, id := range ids {
		expectStatus(t, authRequest(t, handler, "GET", "/media/"+id, "", nil), 200)
		expectStatus(t, authRequest(t, handler, "DELETE", "/media/"+id, "", alice), 409)
	}
	for _, test := range []struct {
		ids    []string
		cookie *http.Cookie
		status int
	}{
		{ids, bob, 403},
		{ids, alice, 409},
		{append(append([]string{}, ids...), ids[0]), alice, 400},
		{[]string{ids[0], strings.ToUpper(ids[0])}, alice, 400},
		{[]string{"not-a-uuid"}, alice, 400},
		{[]string{"11111111-1111-4111-8111-111111111111"}, alice, 400},
	} {
		response := authRequest(t, handler, "POST", "/posts", jsonBody(t, map[string]any{"content": "Rejected", "media_ids": test.ids}), test.cookie)
		expectStatus(t, response, test.status)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM posts").Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed attachment created a partial post: %v", err)
	}
	feed := postPage(t, authRequest(t, handler, "GET", "/posts", "", nil))
	assertImageOrder(t, feed.Items[0], ids)

	reversed := []string{ids[3], ids[2], ids[1], ids[0]}
	updated := authRequest(t, handler, "PATCH", "/posts/"+post.ID, imageEditBody(t, post, "", reversed), alice)
	expectStatus(t, updated, 200)
	current := responsePost(t, updated)
	assertImageOrder(t, current, reversed)
	expectStatus(t, authRequest(t, handler, "PATCH", "/posts/"+post.ID, imageEditBody(t, post, "Stale", []string{}), alice), 409)
	assertImageOrder(t, responsePost(t, authRequest(t, handler, "GET", "/posts/"+post.ID, "", nil)), reversed)
	assertFileCount(t, directory, 4)

	kept := reversed[:3]
	updated = authRequest(t, handler, "PATCH", "/posts/"+post.ID, imageEditBody(t, current, "", kept), alice)
	expectStatus(t, updated, 200)
	current = responsePost(t, updated)
	assertImageOrder(t, current, kept)
	assertFileCount(t, directory, 3)
	expectStatus(t, authRequest(t, handler, "GET", "/media/"+reversed[3], "", nil), 404)
	legacy := authRequest(t, handler, "PATCH", "/posts/"+post.ID, editBody(t, current, "Text edit without media_ids"), alice)
	expectStatus(t, legacy, 200)
	current = responsePost(t, legacy)
	assertImageOrder(t, current, kept)
	expectStatus(t, authRequest(t, handler, "PATCH", "/posts/"+post.ID, imageEditBody(t, current, "", []string{}), alice), 400)
	assertFileCount(t, directory, 3)
	expectStatus(t, authRequest(t, handler, "DELETE", "/posts/"+post.ID, "", alice), 204)
	assertFileCount(t, directory, 0)
	for _, id := range kept {
		expectStatus(t, authRequest(t, handler, "GET", "/media/"+id, "", nil), 404)
	}
}

func TestConcurrentImageAttachment(t *testing.T) {
	db, accounts, pool := authDatabase(t)
	_, cookie := postAccount(t, accounts, "alice")
	handler, directory := testRoutesWithMedia(t, accounts, pool)
	uploaded := imageUpload(t, handler, cookie, imageFixture(t, "png"))
	expectStatus(t, uploaded, 201)
	id := uploadedImage(t, uploaded).ID
	body := jsonBody(t, map[string]any{"content": "", "media_ids": []string{id}})
	results := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		go func() { results <- authRequest(t, handler, "POST", "/posts", body, cookie) }()
	}
	statuses := map[int]int{}
	for range 2 {
		response := <-results
		if response.Code != 201 && response.Code != 409 {
			t.Fatalf("unexpected attachment result: %s", response.Body.String())
		}
		statuses[response.Code]++
	}
	if statuses[201] != 1 || statuses[409] != 1 {
		t.Fatalf("one image was attached twice: %v", statuses)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM posts").Scan(&count); err != nil || count != 1 {
		t.Fatalf("a rejected attachment left a post: %v", err)
	}
	assertFileCount(t, directory, 1)
}

func TestFailedUploadRemovesFile(t *testing.T) {
	_, accounts, pool := authDatabase(t)
	user, _ := postAccount(t, accounts, "alice")
	directory := t.TempDir()
	service, err := media.NewService(pool, directory)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	if _, err := service.Upload(t.Context(), user.ID, bytes.NewReader(imageFixture(t, "png"))); err == nil {
		t.Fatal("upload should fail when metadata cannot be stored")
	}
	assertFileCount(t, directory, 0)
}

func imageUpload(t *testing.T, handler http.Handler, cookie *http.Cookie, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	part, err := writer.CreateFormFile("file", "photo.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "http://api.test/api/v1/media", &buffer)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", "http://frontend.test")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func imageFixture(t *testing.T, format string) []byte {
	t.Helper()
	if format == "webp" {
		data, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4TBUAAAAvAAAAAAfQ//73v/+BiOh/AAA=")
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	var buffer bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 8, 6))
	var err error
	if format == "jpeg" {
		err = jpeg.Encode(&buffer, picture, nil)
	} else {
		err = png.Encode(&buffer, picture)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func uploadedImage(t *testing.T, response *httptest.ResponseRecorder) media.Image {
	t.Helper()
	var image media.Image
	if err := json.Unmarshal(response.Body.Bytes(), &image); err != nil {
		t.Fatal(err)
	}
	return image
}

func assertFileCount(t *testing.T, directory string, want int) {
	t.Helper()
	files, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != want {
		t.Fatalf("stored %d files, want %d", len(files), want)
	}
}

func imageEditBody(t *testing.T, post posts.Post, content string, ids []string) string {
	t.Helper()
	return jsonBody(t, map[string]any{"content": content, "updated_at": post.UpdatedAt, "media_ids": ids})
}

func assertImageOrder(t *testing.T, post posts.Post, ids []string) {
	t.Helper()
	if len(post.Images) != len(ids) {
		t.Fatalf("post has %d images, want %d", len(post.Images), len(ids))
	}
	for i, id := range ids {
		if post.Images[i].ID != id {
			t.Fatal("image order changed")
		}
	}
}
