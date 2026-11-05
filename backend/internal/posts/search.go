package posts

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) GetMany(ctx context.Context, ids []string, viewerID string) ([]Post, error) {
	result := []Post{}
	if len(ids) == 0 {
		return result, nil
	}
	for _, id := range ids {
		var value pgtype.UUID
		if err := value.Scan(id); err != nil || !value.Valid {
			return nil, errors.New("The search index contains an invalid post ID.")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := s.db.Query(ctx, `SELECT `+postColumns+`
		FROM posts p JOIN users u ON u.id = p.author_id WHERE p.id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := make(map[string]Post, len(ids))
	for rows.Next() {
		post, err := readPost(rows)
		if err != nil {
			return nil, err
		}
		found[post.ID] = post
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if post, ok := found[id]; ok {
			result = append(result, post)
		}
	}
	if err := loadImages(ctx, s.db, result); err != nil {
		return nil, err
	}
	if err := loadInteractions(ctx, s.db, result, viewerID); err != nil {
		return nil, err
	}
	return result, nil
}
