package auth

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrAvatarOwner = errors.New("Choose one of your own images for your avatar.")

type ProfileUpdate struct {
	DisplayName *string
	Bio         *string
	AvatarID    *string
	SetAvatar   bool
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, input ProfileUpdate) (User, string, error) {
	if input.DisplayName == nil && input.Bio == nil && !input.SetAvatar {
		return User{}, "", InputError("Include a display name, bio, or avatar.")
	}
	if input.DisplayName != nil {
		name := strings.TrimSpace(*input.DisplayName)
		if n := utf8.RuneCountInString(name); n < 1 || n > 60 || strings.ContainsRune(name, 0) {
			return User{}, "", InputError("Use a display name between 1 and 60 characters.")
		}
		input.DisplayName = &name
	}
	if input.Bio != nil {
		bio := strings.TrimSpace(*input.Bio)
		if utf8.RuneCountInString(bio) > 280 || strings.ContainsRune(bio, 0) {
			return User{}, "", InputError("Keep your bio within 280 characters.")
		}
		input.Bio = &bio
	}
	if input.SetAvatar && input.AvatarID != nil {
		var id pgtype.UUID
		if err := id.Scan(*input.AvatarID); err != nil || !id.Valid {
			return User{}, "", InputError("Choose a valid image for your avatar.")
		}
		normalized := id.String()
		input.AvatarID = &normalized
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, "", err
	}
	defer tx.Rollback(ctx)
	var previous *string
	err = tx.QueryRow(ctx, "SELECT avatar_id::text FROM users WHERE id = $1 FOR UPDATE", userID).Scan(&previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrInvalidSession
	}
	if err != nil {
		return User{}, "", err
	}
	if input.SetAvatar && input.AvatarID != nil {
		var ownerID string
		err := tx.QueryRow(ctx, "SELECT owner_id::text FROM media WHERE id = $1 FOR UPDATE", *input.AvatarID).Scan(&ownerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "", InputError("This avatar image is no longer available. Upload it again.")
		}
		if err != nil {
			return User{}, "", err
		}
		if ownerID != userID {
			return User{}, "", ErrAvatarOwner
		}
	}
	account, err := readAccount(tx.QueryRow(ctx, `UPDATE users SET
		display_name = COALESCE($2, display_name),
		bio = COALESCE($3, bio),
		avatar_id = CASE WHEN $4 THEN $5::uuid ELSE avatar_id END
		WHERE id = $1 RETURNING `+accountColumns,
		userID, input.DisplayName, input.Bio, input.SetAvatar, input.AvatarID))
	if err != nil {
		return User{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, "", err
	}
	removed := ""
	if input.SetAvatar && previous != nil && (input.AvatarID == nil || *previous != *input.AvatarID) {
		removed = *previous
	}
	return account.User, removed, nil
}
