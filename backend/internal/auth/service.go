package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailTaken         = errors.New("This email is already registered.")
	ErrUsernameTaken      = errors.New("This username is already taken.")
	ErrInvalidCredentials = errors.New("Invalid email or password.")
	ErrInvalidSession     = errors.New("Please sign in to continue.")
	usernamePattern       = regexp.MustCompile(`^[a-z0-9_]{3,24}$`)
	passwordParams        = &argon2id.Params{Memory: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}
)

type InputError string

func (e InputError) Error() string { return string(e) }

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	AvatarURL   *string   `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type Service struct {
	db     *pgxpool.Pool
	secret []byte
}

type account struct {
	User
	passwordHash   string
	sessionVersion int
}

const accountColumns = `id::text, email, username, display_name, bio,
	CASE WHEN avatar_id IS NOT NULL THEN '/api/v1/media/' || avatar_id::text END,
	created_at, password_hash, session_version`

func NewService(db *pgxpool.Pool, secret string) (*Service, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 characters")
	}
	return &Service{db: db, secret: []byte(secret)}, nil
}

func (s *Service) Register(ctx context.Context, username, email, password string) (User, string, error) {
	username = strings.TrimSpace(username)
	email = normalizeEmail(email)
	if !usernamePattern.MatchString(username) {
		return User{}, "", InputError("Use 3-24 lowercase letters, digits, or underscores for your username.")
	}
	if err := validateCredentials(email, password); err != nil {
		return User{}, "", err
	}
	hash, err := argon2id.CreateHash(password, passwordParams)
	if err != nil {
		return User{}, "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	a, err := readAccount(s.db.QueryRow(ctx, `
		INSERT INTO users (email, username, password_hash, display_name)
		VALUES ($1, $2, $3, $2) RETURNING `+accountColumns, email, username, hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "users_email_key" {
				return User{}, "", ErrEmailTaken
			}
			if pgErr.ConstraintName == "users_username_key" {
				return User{}, "", ErrUsernameTaken
			}
		}
		return User{}, "", fmt.Errorf("create account: %w", err)
	}
	return s.session(a)
}

func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	email = normalizeEmail(email)
	if err := validateCredentials(email, password); err != nil {
		return User{}, "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	a, err := readAccount(s.db.QueryRow(ctx, "SELECT "+accountColumns+" FROM users WHERE lower(email) = $1", email))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", fmt.Errorf("read account: %w", err)
	}
	match, err := argon2id.ComparePasswordAndHash(password, a.passwordHash)
	if err != nil {
		return User{}, "", err
	}
	if !match {
		return User{}, "", ErrInvalidCredentials
	}
	return s.session(a)
}

func (s *Service) CurrentUser(ctx context.Context, token string) (User, error) {
	a, err := s.authenticate(ctx, token)
	return a.User, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	a, err := s.authenticate(ctx, token)
	if errors.Is(err, ErrInvalidSession) {
		return nil
	}
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = s.db.Exec(ctx, `
		UPDATE users SET session_version = session_version + 1
		WHERE id = $1 AND session_version = $2`, a.ID, a.sessionVersion)
	return err
}

func (s *Service) authenticate(ctx context.Context, token string) (account, error) {
	claims, err := s.parseToken(token)
	if err != nil {
		return account{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	a, err := readAccount(s.db.QueryRow(ctx, "SELECT "+accountColumns+" FROM users WHERE id = $1", claims.Subject))
	if errors.Is(err, pgx.ErrNoRows) {
		return account{}, ErrInvalidSession
	}
	if err != nil {
		return account{}, fmt.Errorf("read session account: %w", err)
	}
	if a.sessionVersion != claims.SessionVersion {
		return account{}, ErrInvalidSession
	}
	return a, nil
}

func readAccount(row pgx.Row) (account, error) {
	var a account
	err := row.Scan(&a.ID, &a.Email, &a.Username, &a.DisplayName, &a.Bio,
		&a.AvatarURL, &a.CreatedAt, &a.passwordHash, &a.sessionVersion)
	return a, err
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateCredentials(email, password string) error {
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return InputError("Enter a valid email address.")
	}
	if n := utf8.RuneCountInString(password); n < 8 || n > 128 {
		return InputError("Use a password between 8 and 128 characters.")
	}
	return nil
}
