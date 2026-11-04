package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"github.com/Jasonssssss9/media-share/backend/internal/imagegen"
	"github.com/Jasonssssss9/media-share/backend/internal/media"
	"golang.org/x/time/rate"
)

type ImageGenerator interface {
	Enabled() bool
	Generate(context.Context, string) ([]byte, error)
}

type generationHandler struct {
	generator ImageGenerator
	media     *media.Service
	logger    *slog.Logger
	limiter   *rate.Limiter
	inFlight  sync.Mutex
}

func RegisterImageGeneration(mux *http.ServeMux, generator ImageGenerator, images *media.Service, accounts *auth.Service, origin string, logger *slog.Logger) {
	h := &generationHandler{
		generator: generator, media: images, logger: logger,
		limiter: rate.NewLimiter(rate.Every(30*time.Second), 1),
	}
	mux.HandleFunc("GET /api/v1/media/generation", h.status)
	mux.HandleFunc("POST /api/v1/media/generation", newAuthHandler(accounts, origin, logger).requireUser(h.generate))
}

func (h *generationHandler) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": h.generator.Enabled()})
}

func (h *generationHandler) generate(w http.ResponseWriter, r *http.Request, user auth.User) {
	if !h.generator.Enabled() {
		h.fail(w, imagegen.ErrUnavailable)
		return
	}
	var input struct {
		Prompt string `json:"prompt"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" || utf8.RuneCountInString(prompt) > imagegen.MaxPromptLength || strings.ContainsRune(prompt, '\x00') {
		writeError(w, http.StatusBadRequest, "invalid_input", "Enter a prompt between 1 and 1,000 characters.")
		return
	}
	if !h.inFlight.TryLock() {
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusTooManyRequests, "generation_busy", "An image is already being generated. Please wait.")
		return
	}
	defer h.inFlight.Unlock()
	if !h.limiter.Allow() {
		w.Header().Set("Retry-After", "30")
		writeError(w, http.StatusTooManyRequests, "generation_limited", "Wait 30 seconds between image generation attempts.")
		return
	}
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(imagegen.Timeout + 10*time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), imagegen.Timeout)
	defer cancel()
	data, err := h.generator.Generate(ctx, prompt)
	if err != nil {
		h.fail(w, err)
		return
	}
	result, err := h.media.Upload(ctx, user.ID, bytes.NewReader(data))
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Location", result.URL)
	writeJSON(w, http.StatusCreated, result)
}

func (h *generationHandler) fail(w http.ResponseWriter, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	h.logger.Error("image generation failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
	var upstream *imagegen.APIError
	switch {
	case errors.Is(err, imagegen.ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, "generation_unavailable", "Image generation is not enabled on this server.")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "generation_timeout", "Image generation timed out. Please try again.")
	case errors.Is(err, imagegen.ErrResponse), errors.Is(err, media.ErrInvalid), errors.Is(err, media.ErrSize):
		writeError(w, http.StatusBadGateway, "generation_failed", "The image service did not return a usable image. Please try again.")
	case errors.As(err, &upstream):
		switch {
		case upstream.Code == "moderation_blocked" || upstream.Code == "content_policy_violation" || upstream.Type == "image_generation_user_error":
			writeError(w, http.StatusBadRequest, "invalid_prompt", "This prompt could not be used. Please change it and try again.")
		case upstream.Status == http.StatusTooManyRequests:
			w.Header().Set("Retry-After", "30")
			writeError(w, http.StatusTooManyRequests, "generation_limited", "The image service is busy or has reached its quota. Please try again later.")
		case upstream.Status == http.StatusUnauthorized || upstream.Status == http.StatusForbidden:
			writeError(w, http.StatusServiceUnavailable, "generation_unavailable", "Image generation is unavailable. Please try again later.")
		default:
			writeError(w, http.StatusBadGateway, "generation_failed", "The image service could not generate an image. Please try again.")
		}
	default:
		writeError(w, http.StatusServiceUnavailable, "generation_unavailable", "Image generation is unavailable. Please try again later.")
	}
}
