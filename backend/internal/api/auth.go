package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"golang.org/x/time/rate"
)

const sessionCookie = "mediashare_session"

type authHandler struct {
	service *auth.Service
	logger  *slog.Logger
	secure  bool
	limiter *rate.Limiter
}

func WithAuth(mux *http.ServeMux, service *auth.Service, origin string, logger *slog.Logger) (http.Handler, error) {
	h := &authHandler{
		service: service, logger: logger,
		secure:  strings.HasPrefix(origin, "https://"),
		limiter: rate.NewLimiter(rate.Every(time.Second), 10),
	}
	mux.HandleFunc("POST /api/v1/auth/register", h.limit(h.register))
	mux.HandleFunc("POST /api/v1/auth/login", h.limit(h.login))
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.HandleFunc("GET /api/v1/auth/me", h.me)

	protection := http.NewCrossOriginProtection()
	if err := protection.AddTrustedOrigin(origin); err != nil {
		return nil, err
	}
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusForbidden, "forbidden_origin", "This request origin is not allowed.")
	}))
	return protection.Handler(mux), nil
}

func (h *authHandler) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	user, token, err := h.service.Register(r.Context(), input.Username, input.Email, input.Password)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.setCookie(w, token)
	writeJSON(w, http.StatusCreated, user)
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	user, token, err := h.service.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.setCookie(w, token)
	writeJSON(w, http.StatusOK, user)
}

func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.CurrentUser(r.Context(), cookieValue(r))
	if err != nil {
		if errors.Is(err, auth.ErrInvalidSession) {
			h.clearCookie(w)
		}
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Logout(r.Context(), cookieValue(r)); err != nil {
		h.fail(w, err)
		return
	}
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "too_many_requests", "Too many attempts. Please try again soon.")
			return
		}
		next(w, r)
	}
}

func (h *authHandler) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/api",
		HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(auth.SessionDuration.Seconds()), Expires: time.Now().Add(auth.SessionDuration),
	})
}

func (h *authHandler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Path: "/api", MaxAge: -1, Expires: time.Unix(1, 0),
		HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode,
	})
}

func cookieValue(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func readJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Send a JSON request body.")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid request body.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid request body.")
		return false
	}
	return true
}

func (h *authHandler) fail(w http.ResponseWriter, err error) {
	var inputError auth.InputError
	switch {
	case errors.As(err, &inputError):
		writeError(w, http.StatusBadRequest, "invalid_input", inputError.Error())
	case errors.Is(err, auth.ErrEmailTaken), errors.Is(err, auth.ErrUsernameTaken):
		writeError(w, http.StatusConflict, "account_exists", err.Error())
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrInvalidSession):
		writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
	default:
		h.logger.Error("authentication request failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Authentication is unavailable. Please try again.")
	}
}
