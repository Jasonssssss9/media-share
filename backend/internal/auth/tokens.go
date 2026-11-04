package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const SessionDuration = 24 * time.Hour

type sessionClaims struct {
	SessionVersion int `json:"sv"`
	jwt.RegisteredClaims
}

func (s *Service) session(a account) (User, string, error) {
	now := time.Now()
	claims := sessionClaims{
		SessionVersion: a.sessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "mediashare",
			Audience:  jwt.ClaimStrings{"mediashare-web"},
			Subject:   a.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(SessionDuration)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	return a.User, token, err
}

func (s *Service) parseToken(raw string) (*sessionClaims, error) {
	claims := &sessionClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer("mediashare"),
		jwt.WithAudience("mediashare-web"),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return nil, ErrInvalidSession
	}
	var id pgtype.UUID
	if err := id.Scan(claims.Subject); err != nil || !id.Valid || claims.SessionVersion < 0 {
		return nil, ErrInvalidSession
	}
	return claims, nil
}
