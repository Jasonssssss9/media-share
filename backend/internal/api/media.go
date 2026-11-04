package api

import (
	"errors"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"github.com/Jasonssssss9/media-share/backend/internal/media"
)

type mediaHandler struct {
	service *media.Service
	auth    *authHandler
	logger  *slog.Logger
}

func RegisterMedia(mux *http.ServeMux, service *media.Service, authService *auth.Service, origin string, logger *slog.Logger) {
	a := newAuthHandler(authService, origin, logger)
	h := &mediaHandler{service: service, auth: a, logger: logger}
	mux.HandleFunc("POST /api/v1/media", a.requireUser(h.upload))
	mux.HandleFunc("GET /api/v1/media/{id}", h.get)
	mux.HandleFunc("DELETE /api/v1/media/{id}", a.requireUser(h.delete))
}

func (h *mediaHandler) upload(w http.ResponseWriter, r *http.Request, user auth.User) {
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxSize+(64<<10))
	err := r.ParseMultipartForm(media.MaxSize)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || errors.Is(err, multipart.ErrMessageTooLarge) {
			h.fail(w, media.ErrSize)
		} else {
			writeError(w, http.StatusBadRequest, "invalid_input", "Upload one image in the file field.")
		}
		return
	}
	if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 || len(r.MultipartForm.Value) != 0 {
		writeError(w, http.StatusBadRequest, "invalid_input", "Upload one image in the file field.")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Choose an image to upload.")
		return
	}
	defer file.Close()
	result, err := h.service.Upload(r.Context(), user.ID, file)
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Location", result.URL)
	writeJSON(w, http.StatusCreated, result)
}

func (h *mediaHandler) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := resourceID(w, r)
	if !ok {
		return
	}
	viewerID := ""
	if token := cookieValue(r); token != "" {
		user, err := h.auth.service.CurrentUser(r.Context(), token)
		if err != nil && !errors.Is(err, auth.ErrInvalidSession) {
			h.auth.fail(w, err)
			return
		}
		if err == nil {
			viewerID = user.ID
		}
	}
	file, mimeType, err := h.service.Open(r.Context(), id, viewerID)
	if err != nil {
		h.fail(w, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (h *mediaHandler) delete(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := resourceID(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), id, user.ID); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *mediaHandler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid_image", err.Error())
	case errors.Is(err, media.ErrSize):
		writeError(w, http.StatusRequestEntityTooLarge, "image_too_large", err.Error())
	case errors.Is(err, media.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, media.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, media.ErrInUse):
		writeError(w, http.StatusConflict, "image_in_use", err.Error())
	default:
		h.logger.Error("image request failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Images are unavailable. Please try again.")
	}
}
