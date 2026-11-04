package posts

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var (
	ErrCommentContent = errors.New("Write a comment between 1 and 500 characters.")
	ErrCommentMissing = errors.New("This comment no longer exists.")
	ErrCommentOwner   = errors.New("You do not have permission to change this comment.")
)

type Comment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"post_id"`
	Author    Author    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CommentPage struct {
	Items    []Comment `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Total    int64     `json:"total"`
}

const commentColumns = `c.id::text, c.post_id::text, c.content, c.created_at, c.updated_at,
	u.id::text, u.username, u.display_name, u.bio,
	CASE WHEN u.avatar_id IS NOT NULL THEN '/api/v1/media/' || u.avatar_id::text END`

func (s *Service) ListComments(ctx context.Context, postID string, page, pageSize int) (CommentPage, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var exists bool
	if err := s.db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM posts WHERE id = $1)", postID).Scan(&exists); err != nil {
		return CommentPage{}, err
	}
	if !exists {
		return CommentPage{}, ErrNotFound
	}
	result := CommentPage{Items: []Comment{}, Page: page, PageSize: pageSize}
	if err := s.db.QueryRow(ctx, "SELECT count(*) FROM comments WHERE post_id = $1", postID).Scan(&result.Total); err != nil {
		return CommentPage{}, err
	}
	if int64(page-1) > result.Total/int64(pageSize) {
		return result, nil
	}
	rows, err := s.db.Query(ctx, `SELECT `+commentColumns+` FROM comments c
		JOIN users u ON u.id = c.author_id WHERE c.post_id = $1
		ORDER BY c.created_at DESC, c.id DESC LIMIT $2 OFFSET $3`,
		postID, pageSize, int64(page-1)*int64(pageSize))
	if err != nil {
		return CommentPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		comment, err := readComment(rows)
		if err != nil {
			return CommentPage{}, err
		}
		result.Items = append(result.Items, comment)
	}
	return result, rows.Err()
}

func (s *Service) CreateComment(ctx context.Context, postID, authorID, content string) (Comment, error) {
	content = strings.TrimSpace(content)
	if err := validateComment(content); err != nil {
		return Comment{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Comment{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockInteractionPost(ctx, tx, postID); err != nil {
		return Comment{}, err
	}
	comment, err := readComment(tx.QueryRow(ctx, `WITH created AS (
		INSERT INTO comments (post_id, author_id, content) VALUES ($1, $2, $3) RETURNING *
	) SELECT `+commentColumns+` FROM created c JOIN users u ON u.id = c.author_id`, postID, authorID, content))
	if err != nil {
		return Comment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Comment{}, err
	}
	return comment, nil
}

func (s *Service) UpdateComment(ctx context.Context, id, userID, content string) (Comment, error) {
	content = strings.TrimSpace(content)
	if err := validateComment(content); err != nil {
		return Comment{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Comment{}, err
	}
	defer tx.Rollback(ctx)
	comment, _, err := lockedComment(ctx, tx, id)
	if err != nil {
		return Comment{}, err
	}
	if comment.Author.ID != userID {
		return Comment{}, ErrCommentOwner
	}
	err = tx.QueryRow(ctx, "UPDATE comments SET content = $2, updated_at = clock_timestamp() WHERE id = $1 RETURNING updated_at", id, content).Scan(&comment.UpdatedAt)
	if err != nil {
		return Comment{}, err
	}
	comment.Content = content
	if err := tx.Commit(ctx); err != nil {
		return Comment{}, err
	}
	return comment, nil
}

func (s *Service) DeleteComment(ctx context.Context, id, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	comment, postOwner, err := lockedComment(ctx, tx, id)
	if err != nil {
		return err
	}
	if comment.Author.ID != userID && postOwner != userID {
		return ErrCommentOwner
	}
	if _, err := tx.Exec(ctx, "DELETE FROM comments WHERE id = $1", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lockedComment(ctx context.Context, tx pgx.Tx, id string) (Comment, string, error) {
	var comment Comment
	var postOwner string
	err := tx.QueryRow(ctx, `SELECT `+commentColumns+`, p.author_id::text
		FROM comments c JOIN users u ON u.id = c.author_id JOIN posts p ON p.id = c.post_id
		WHERE c.id = $1 FOR UPDATE OF c`, id).Scan(
		&comment.ID, &comment.PostID, &comment.Content, &comment.CreatedAt, &comment.UpdatedAt,
		&comment.Author.ID, &comment.Author.Username, &comment.Author.DisplayName, &comment.Author.Bio, &comment.Author.AvatarURL,
		&postOwner)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, "", ErrCommentMissing
	}
	return comment, postOwner, err
}

func readComment(row pgx.Row) (Comment, error) {
	var comment Comment
	err := row.Scan(&comment.ID, &comment.PostID, &comment.Content, &comment.CreatedAt, &comment.UpdatedAt,
		&comment.Author.ID, &comment.Author.Username, &comment.Author.DisplayName, &comment.Author.Bio, &comment.Author.AvatarURL)
	return comment, err
}

func validateComment(content string) error {
	if content == "" || utf8.RuneCountInString(content) > 500 || strings.ContainsRune(content, 0) {
		return ErrCommentContent
	}
	return nil
}
