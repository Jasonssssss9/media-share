package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"github.com/Jasonssssss9/media-share/backend/internal/media"
	"github.com/Jasonssssss9/media-share/backend/internal/posts"
	"github.com/jackc/pgx/v5/pgtype"
)

type postHandler struct {
	service *posts.Service
	media   *media.Service
	logger  *slog.Logger
	auth    *authHandler
}

func RegisterPosts(mux *http.ServeMux, service *posts.Service, mediaService *media.Service, authService *auth.Service, origin string, logger *slog.Logger) {
	a := newAuthHandler(authService, origin, logger)
	h := &postHandler{service: service, media: mediaService, logger: logger, auth: a}
	mux.HandleFunc("GET /api/v1/posts", h.list)
	mux.HandleFunc("GET /api/v1/posts/{id}", h.get)
	mux.HandleFunc("GET /api/v1/users/{username}", h.author)
	mux.HandleFunc("POST /api/v1/posts", a.requireUser(h.create))
	mux.HandleFunc("PATCH /api/v1/posts/{id}", a.requireUser(h.update))
	mux.HandleFunc("DELETE /api/v1/posts/{id}", a.requireUser(h.delete))
}

func (h *postHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	page, pageSize, ok := requestPage(w, r)
	if !ok {
		return
	}
	viewerID, ok := h.auth.viewerID(w, r)
	if !ok {
		return
	}
	result, err := h.service.List(r.Context(), page, pageSize, strings.TrimSpace(r.URL.Query().Get("author")), viewerID)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *postHandler) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := resourceID(w, r)
	if !ok {
		return
	}
	viewerID, ok := h.auth.viewerID(w, r)
	if !ok {
		return
	}
	post, err := h.service.Get(r.Context(), id, viewerID)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (h *postHandler) author(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	author, err := h.service.GetAuthor(r.Context(), r.PathValue("username"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, author)
}

func (h *postHandler) create(w http.ResponseWriter, r *http.Request, user auth.User) {
	var input struct {
		Content  string   `json:"content"`
		MediaIDs []string `json:"media_ids"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	post, err := h.service.Create(r.Context(), user.ID, input.Content, input.MediaIDs)
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/posts/"+post.ID)
	writeJSON(w, http.StatusCreated, post)
}

func (h *postHandler) update(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := resourceID(w, r)
	if !ok {
		return
	}
	var input struct {
		Content   *string   `json:"content"`
		UpdatedAt time.Time `json:"updated_at"`
		MediaIDs  []string  `json:"media_ids"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if input.Content == nil || input.UpdatedAt.IsZero() {
		writeError(w, http.StatusBadRequest, "invalid_input", "Include content and the post's original updated_at value.")
		return
	}
	post, removed, err := h.service.Update(r.Context(), id, user.ID, *input.Content, input.UpdatedAt, input.MediaIDs)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.cleanup(r.Context(), user.ID, removed)
	writeJSON(w, http.StatusOK, post)
}

func (h *postHandler) delete(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := resourceID(w, r)
	if !ok {
		return
	}
	removed, err := h.service.Delete(r.Context(), id, user.ID)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.cleanup(r.Context(), user.ID, removed)
	w.WriteHeader(http.StatusNoContent)
}

func (h *postHandler) cleanup(ctx context.Context, ownerID string, ids []string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, id := range ids {
		if err := h.media.Delete(ctx, id, ownerID); err != nil && !errors.Is(err, media.ErrInUse) && !errors.Is(err, media.ErrNotFound) {
			h.logger.Warn("image cleanup failed", "media_id", id, "error", err)
		}
	}
}

func (h *postHandler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, posts.ErrContent), errors.Is(err, posts.ErrImages):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, posts.ErrNotFound), errors.Is(err, posts.ErrAuthorNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, posts.ErrForbidden), errors.Is(err, posts.ErrImageOwner):
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, posts.ErrImageInUse):
		writeError(w, http.StatusConflict, "image_in_use", err.Error())
	case errors.Is(err, posts.ErrConflict):
		writeError(w, http.StatusConflict, "edit_conflict", err.Error())
	default:
		h.logger.Error("post request failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Posts are unavailable. Please try again.")
	}
}

func resourceID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	var value pgtype.UUID
	if err := value.Scan(id); err != nil || !value.Valid {
		writeError(w, http.StatusBadRequest, "invalid_input", "Use a valid resource ID.")
		return "", false
	}
	return id, true
}

func positiveInt(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 0, errors.New("invalid positive integer")
	}
	return n, nil
}

func requestPage(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	page, err := positiveInt(r.URL.Query().Get("page"), 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Page must be a positive integer.")
		return 0, 0, false
	}
	size, err := positiveInt(r.URL.Query().Get("page_size"), 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Page size must be a positive integer.")
		return 0, 0, false
	}
	return page, min(size, 50), true
}
