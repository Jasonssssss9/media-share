package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"github.com/Jasonssssss9/media-share/backend/internal/posts"
)

type interactionHandler struct {
	service *posts.Service
	logger  *slog.Logger
}

func RegisterInteractions(mux *http.ServeMux, service *posts.Service, accounts *auth.Service, origin string, logger *slog.Logger) {
	h := &interactionHandler{service: service, logger: logger}
	a := newAuthHandler(accounts, origin, logger)
	mux.HandleFunc("PUT /api/v1/posts/{id}/like", a.requireUser(h.like))
	mux.HandleFunc("DELETE /api/v1/posts/{id}/like", a.requireUser(h.like))
	mux.HandleFunc("GET /api/v1/posts/{id}/comments", h.list)
	mux.HandleFunc("POST /api/v1/posts/{id}/comments", a.requireUser(h.create))
	mux.HandleFunc("PATCH /api/v1/comments/{id}", a.requireUser(h.update))
	mux.HandleFunc("DELETE /api/v1/comments/{id}", a.requireUser(h.delete))
}

func (h *interactionHandler) like(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := resourceID(w, r)
	if !ok {
		return
	}
	result, err := h.service.SetLike(r.Context(), id, user.ID, r.Method == http.MethodPut)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *interactionHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := resourceID(w, r)
	if !ok {
		return
	}
	page, pageSize, ok := requestPage(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListComments(r.Context(), id, page, pageSize)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *interactionHandler) create(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := resourceID(w, r)
	if !ok {
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	comment, err := h.service.CreateComment(r.Context(), id, user.ID, input.Content)
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/comments/"+comment.ID)
	writeJSON(w, http.StatusCreated, comment)
}

func (h *interactionHandler) update(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := resourceID(w, r)
	if !ok {
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	comment, err := h.service.UpdateComment(r.Context(), id, user.ID, input.Content)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, comment)
}

func (h *interactionHandler) delete(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := resourceID(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteComment(r.Context(), id, user.ID); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *interactionHandler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, posts.ErrCommentContent):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, posts.ErrNotFound), errors.Is(err, posts.ErrCommentMissing):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, posts.ErrCommentOwner):
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
	default:
		h.logger.Error("interaction request failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Likes and comments are unavailable. Please try again.")
	}
}
