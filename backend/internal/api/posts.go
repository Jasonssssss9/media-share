package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"github.com/Jasonssssss9/media-share/backend/internal/posts"
	"github.com/jackc/pgx/v5/pgtype"
)

type postHandler struct {
	service *posts.Service
	logger  *slog.Logger
}

func RegisterPosts(mux *http.ServeMux, service *posts.Service, authService *auth.Service, origin string, logger *slog.Logger) {
	h := &postHandler{service: service, logger: logger}
	a := &authHandler{service: authService, logger: logger, secure: strings.HasPrefix(origin, "https://")}
	mux.HandleFunc("GET /api/v1/posts", h.list)
	mux.HandleFunc("GET /api/v1/posts/{id}", h.get)
	mux.HandleFunc("GET /api/v1/users/{username}", h.author)
	mux.HandleFunc("POST /api/v1/posts", a.requireUser(h.create))
	mux.HandleFunc("PATCH /api/v1/posts/{id}", a.requireUser(h.update))
	mux.HandleFunc("DELETE /api/v1/posts/{id}", a.requireUser(h.delete))
}

func (h *postHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	page, err := positiveInt(r.URL.Query().Get("page"), 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Page must be a positive integer.")
		return
	}
	pageSize, err := positiveInt(r.URL.Query().Get("page_size"), 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Page size must be a positive integer.")
		return
	}
	pageSize = min(pageSize, 50)
	result, err := h.service.List(r.Context(), page, pageSize, strings.TrimSpace(r.URL.Query().Get("author")))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *postHandler) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := postID(w, r)
	if !ok {
		return
	}
	post, err := h.service.Get(r.Context(), id)
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
		Content string `json:"content"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	post, err := h.service.Create(r.Context(), user.ID, input.Content)
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/posts/"+post.ID)
	writeJSON(w, http.StatusCreated, post)
}

func (h *postHandler) update(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := postID(w, r)
	if !ok {
		return
	}
	var input struct {
		Content   string    `json:"content"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if input.UpdatedAt.IsZero() {
		writeError(w, http.StatusBadRequest, "invalid_input", "Include the post's original updated_at value.")
		return
	}
	post, err := h.service.Update(r.Context(), id, user.ID, input.Content, input.UpdatedAt)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (h *postHandler) delete(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := postID(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), id, user.ID); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *postHandler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, posts.ErrContent):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, posts.ErrNotFound), errors.Is(err, posts.ErrAuthorNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, posts.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, posts.ErrConflict):
		writeError(w, http.StatusConflict, "edit_conflict", err.Error())
	default:
		h.logger.Error("post request failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Posts are unavailable. Please try again.")
	}
}

func postID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	var value pgtype.UUID
	if err := value.Scan(id); err != nil || !value.Valid {
		writeError(w, http.StatusBadRequest, "invalid_input", "Use a valid post ID.")
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
