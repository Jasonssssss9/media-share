package posts

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jasonssssss9/media-share/backend/internal/media"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("This post does not exist.")
	ErrAuthorNotFound = errors.New("This user does not exist.")
	ErrForbidden      = errors.New("You can only change your own posts.")
	ErrConflict       = errors.New("This post has changed. Reload it before saving again.")
	ErrContent        = errors.New("Write text or add an image, with no more than 2,000 text characters.")
)

type Author struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	Bio         string  `json:"bio"`
	AvatarURL   *string `json:"avatar_url"`
}

type Post struct {
	Images    []media.Image `json:"images"`
	ID        string        `json:"id"`
	Author    Author        `json:"author"`
	Content   string        `json:"content"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type Page struct {
	Items    []Post `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Total    int64  `json:"total"`
}

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

const postColumns = `p.id::text, p.content, p.created_at, p.updated_at,
	u.id::text, u.username, u.display_name, u.bio,
	CASE WHEN u.avatar_id IS NOT NULL THEN '/api/v1/media/' || u.avatar_id::text END`

func (s *Service) List(ctx context.Context, page, pageSize int, author string) (Page, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result := Page{Items: []Post{}, Page: page, PageSize: pageSize}
	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM posts p JOIN users u ON u.id = p.author_id
		WHERE ($1 = '' OR u.username = $1)`, author).Scan(&result.Total); err != nil {
		return Page{}, err
	}
	if int64(page-1) > result.Total/int64(pageSize) {
		return result, nil
	}
	rows, err := s.db.Query(ctx, `SELECT `+postColumns+`
		FROM posts p JOIN users u ON u.id = p.author_id
		WHERE ($1 = '' OR u.username = $1)
		ORDER BY p.created_at DESC, p.id DESC LIMIT $2 OFFSET $3`,
		author, pageSize, int64(page-1)*int64(pageSize))
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	for rows.Next() {
		post, err := readPost(rows)
		if err != nil {
			return Page{}, err
		}
		result.Items = append(result.Items, post)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Page{}, err
	}
	if err := loadImages(ctx, s.db, result.Items); err != nil {
		return Page{}, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, id string) (Post, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	post, err := readPost(s.db.QueryRow(ctx, `SELECT `+postColumns+`
		FROM posts p JOIN users u ON u.id = p.author_id WHERE p.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, err
	}
	items := []Post{post}
	if err := loadImages(ctx, s.db, items); err != nil {
		return Post{}, err
	}
	return items[0], nil
}

func (s *Service) GetAuthor(ctx context.Context, username string) (Author, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var author Author
	err := s.db.QueryRow(ctx, `SELECT id::text, username, display_name, bio,
		CASE WHEN avatar_id IS NOT NULL THEN '/api/v1/media/' || avatar_id::text END
		FROM users WHERE username = $1`, username).Scan(
		&author.ID, &author.Username, &author.DisplayName, &author.Bio, &author.AvatarURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return Author{}, ErrAuthorNotFound
	}
	return author, err
}

func readPost(row pgx.Row) (Post, error) {
	var post Post
	err := row.Scan(&post.ID, &post.Content, &post.CreatedAt, &post.UpdatedAt,
		&post.Author.ID, &post.Author.Username, &post.Author.DisplayName, &post.Author.Bio, &post.Author.AvatarURL)
	return post, err
}

func validateContent(content string, imageCount int) error {
	if (strings.TrimSpace(content) == "" && imageCount == 0) || utf8.RuneCountInString(content) > 2000 || strings.ContainsRune(content, 0) {
		return ErrContent
	}
	return nil
}
