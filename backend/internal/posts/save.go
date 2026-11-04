package posts

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) Create(ctx context.Context, authorID, content string, mediaIDs []string) (Post, error) {
	ids, err := normalizeImageIDs(mediaIDs)
	if err != nil {
		return Post{}, err
	}
	if err := validateContent(content, len(ids)); err != nil {
		return Post{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)
	post, err := readPost(tx.QueryRow(ctx, `WITH created AS (
		INSERT INTO posts (author_id, content) VALUES ($1, $2) RETURNING *
	) SELECT `+postColumns+` FROM created p JOIN users u ON u.id = p.author_id`, authorID, content))
	if err != nil {
		return Post{}, err
	}
	if err := setImages(ctx, tx, post.ID, authorID, ids); err != nil {
		return Post{}, err
	}
	items := []Post{post}
	if err := loadImages(ctx, tx, items); err != nil {
		return Post{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Post{}, err
	}
	return items[0], nil
}

func (s *Service) Update(ctx context.Context, id, authorID, content string, updatedAt time.Time, mediaIDs []string) (Post, []string, error) {
	ids, err := normalizeImageIDs(mediaIDs)
	if err != nil {
		return Post{}, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Post{}, nil, err
	}
	defer tx.Rollback(ctx)
	existing, err := lockedPost(ctx, tx, id, authorID)
	if err != nil {
		return Post{}, nil, err
	}
	if !existing.UpdatedAt.Equal(updatedAt) {
		return Post{}, nil, ErrConflict
	}
	if mediaIDs == nil {
		ids = imageIDs(existing.Images)
	}
	if err := validateContent(content, len(ids)); err != nil {
		return Post{}, nil, err
	}
	if _, err := tx.Exec(ctx, "UPDATE posts SET content = $2, updated_at = now() WHERE id = $1", id, content); err != nil {
		return Post{}, nil, err
	}
	if err := setImages(ctx, tx, id, authorID, ids); err != nil {
		return Post{}, nil, err
	}
	post, err := readPost(tx.QueryRow(ctx, `SELECT `+postColumns+`
		FROM posts p JOIN users u ON u.id = p.author_id WHERE p.id = $1`, id))
	if err != nil {
		return Post{}, nil, err
	}
	items := []Post{post}
	if err := loadImages(ctx, tx, items); err != nil {
		return Post{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Post{}, nil, err
	}
	return items[0], removedImages(existing.Images, items[0].Images), nil
}

func (s *Service) Delete(ctx context.Context, id, authorID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	post, err := lockedPost(ctx, tx, id, authorID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM posts WHERE id = $1", id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return imageIDs(post.Images), nil
}

func lockedPost(ctx context.Context, tx pgx.Tx, id, authorID string) (Post, error) {
	post, err := readPost(tx.QueryRow(ctx, `SELECT `+postColumns+`
		FROM posts p JOIN users u ON u.id = p.author_id WHERE p.id = $1 FOR UPDATE OF p`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, err
	}
	if post.Author.ID != authorID {
		return Post{}, ErrForbidden
	}
	items := []Post{post}
	if err := loadImages(ctx, tx, items); err != nil {
		return Post{}, err
	}
	return items[0], nil
}
