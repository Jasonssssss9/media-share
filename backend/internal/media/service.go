package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const MaxSize = 5 << 20
const MaxPixels = 25_000_000

var (
	ErrInvalid   = errors.New("Choose a valid JPEG, PNG, or WebP image up to 25 megapixels.")
	ErrSize      = errors.New("Each image must be 5 MiB or smaller.")
	ErrNotFound  = errors.New("This image does not exist or is not accessible.")
	ErrForbidden = errors.New("You can only discard your own uploads.")
	ErrInUse     = errors.New("Remove this image from its post or profile before discarding it.")
)

type Image struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Service struct {
	db        *pgxpool.Pool
	directory string
}

func NewService(db *pgxpool.Pool, directory string) (*Service, error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	return &Service{db: db, directory: directory}, nil
}

func (s *Service) Upload(ctx context.Context, ownerID string, reader io.Reader) (Image, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxSize+1))
	if err != nil {
		return Image{}, err
	}
	if len(data) > MaxSize {
		return Image{}, ErrSize
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > MaxPixels {
		return Image{}, ErrInvalid
	}
	mimeType, extension := "", ""
	switch format {
	case "jpeg":
		mimeType, extension = "image/jpeg", ".jpg"
	case "png":
		mimeType, extension = "image/png", ".png"
	case "webp":
		mimeType, extension = "image/webp", ".webp"
	default:
		return Image{}, ErrInvalid
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return Image{}, ErrInvalid
	}
	key := rand.Text() + extension
	path := filepath.Join(s.directory, key)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Image{}, err
	}
	saved := false
	defer func() {
		if !saved {
			_ = os.Remove(path)
		}
	}()
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil {
		return Image{}, err
	}
	if closeErr != nil {
		return Image{}, closeErr
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result := Image{Width: config.Width, Height: config.Height}
	err = s.db.QueryRow(ctx, `INSERT INTO media
		(owner_id, storage_key, mime_type, size, width, height)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id::text`,
		ownerID, key, mimeType, len(data), config.Width, config.Height).Scan(&result.ID)
	if err != nil {
		return Image{}, err
	}
	result.URL = "/api/v1/media/" + result.ID
	saved = true
	return result, nil
}

func (s *Service) Open(ctx context.Context, id, viewerID string) (*os.File, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var ownerID, key, mimeType string
	var public bool
	err := s.db.QueryRow(ctx, `SELECT owner_id::text, storage_key, mime_type,
		post_id IS NOT NULL OR EXISTS (SELECT 1 FROM users WHERE avatar_id = media.id)
		FROM media WHERE id = $1`, id).Scan(&ownerID, &key, &mimeType, &public)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	if !public && ownerID != viewerID {
		return nil, "", ErrNotFound
	}
	file, err := os.Open(filepath.Join(s.directory, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", ErrNotFound
	}
	return file, mimeType, err
}

func (s *Service) Delete(ctx context.Context, id, ownerID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var key string
	err := s.db.QueryRow(ctx, `DELETE FROM media WHERE id = $1 AND owner_id = $2
		AND post_id IS NULL AND NOT EXISTS (SELECT 1 FROM users WHERE avatar_id = media.id)
		RETURNING storage_key`, id, ownerID).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		var actualOwner string
		if err := s.db.QueryRow(ctx, "SELECT owner_id::text FROM media WHERE id = $1", id).Scan(&actualOwner); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if actualOwner != ownerID {
			return ErrForbidden
		}
		return ErrInUse
	}
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.directory, key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
