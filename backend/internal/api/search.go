package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"github.com/Jasonssssss9/media-share/backend/internal/search"
)

func RegisterSearch(mux *http.ServeMux, service *search.Service, accounts *auth.Service, origin string, logger *slog.Logger) {
	a := newAuthHandler(accounts, origin, logger)
	mux.HandleFunc("GET /api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		page, size, ok := requestPage(w, r)
		if !ok {
			return
		}
		viewerID, ok := a.viewerID(w, r)
		if !ok {
			return
		}
		result, err := service.Search(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("author"), viewerID, page, size)
		if err != nil {
			var input search.InputError
			if errors.As(err, &input) {
				writeError(w, http.StatusBadRequest, "invalid_input", input.Error())
			} else {
				logger.Error("search failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
				writeError(w, http.StatusServiceUnavailable, "search_unavailable", "Search is temporarily unavailable. Please try again.")
			}
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}

func (h *postHandler) syncIndex(ctx context.Context, id string) {
	if h.search == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := h.search.Sync(ctx, id); err != nil {
		h.logger.Warn("post saved but search synchronization failed; run make search-reindex", "post_id", id, "error", err)
	}
}
