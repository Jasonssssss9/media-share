package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"github.com/Jasonssssss9/media-share/backend/internal/media"
)

type profileHandler struct {
	accounts *auth.Service
	media    *media.Service
	logger   *slog.Logger
}

func RegisterProfiles(mux *http.ServeMux, accounts *auth.Service, images *media.Service, origin string, logger *slog.Logger) {
	h := &profileHandler{accounts: accounts, media: images, logger: logger}
	mux.HandleFunc("PATCH /api/v1/users/me", newAuthHandler(accounts, origin, logger).requireUser(h.update))
}

func (h *profileHandler) update(w http.ResponseWriter, r *http.Request, user auth.User) {
	var body struct {
		DisplayName json.RawMessage `json:"display_name"`
		Bio         json.RawMessage `json:"bio"`
		AvatarID    json.RawMessage `json:"avatar_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	var input auth.ProfileUpdate
	var err error
	input.DisplayName, err = profileText(body.DisplayName)
	if err != nil {
		h.fail(w, err)
		return
	}
	input.Bio, err = profileText(body.Bio)
	if err != nil {
		h.fail(w, err)
		return
	}
	input.SetAvatar = len(body.AvatarID) > 0
	if input.SetAvatar {
		if err := json.Unmarshal(body.AvatarID, &input.AvatarID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "Use an image ID or null for your avatar.")
			return
		}
	}
	saved, removed, err := h.accounts.UpdateProfile(r.Context(), user.ID, input)
	if err != nil {
		h.fail(w, err)
		return
	}
	if removed != "" {
		if err := h.media.Delete(r.Context(), removed, user.ID); err != nil && !errors.Is(err, media.ErrInUse) && !errors.Is(err, media.ErrNotFound) {
			h.logger.Warn("avatar cleanup failed", "media_id", removed, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, saved)
}

func profileText(raw json.RawMessage) (*string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var text *string
	if err := json.Unmarshal(raw, &text); err != nil || text == nil {
		return nil, auth.InputError("Use text for your display name and bio.")
	}
	return text, nil
}

func (h *profileHandler) fail(w http.ResponseWriter, err error) {
	var input auth.InputError
	switch {
	case errors.As(err, &input):
		writeError(w, http.StatusBadRequest, "invalid_input", input.Error())
	case errors.Is(err, auth.ErrAvatarOwner):
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, auth.ErrInvalidSession):
		writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
	default:
		h.logger.Error("profile update failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Your profile could not be saved. Please try again.")
	}
}
