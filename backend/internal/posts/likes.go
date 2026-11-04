package posts

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type LikeState struct {
	LikeCount int64 `json:"like_count"`
	LikedByMe bool  `json:"liked_by_me"`
}

func (s *Service) SetLike(ctx context.Context, postID, userID string, liked bool) (LikeState, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return LikeState{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockInteractionPost(ctx, tx, postID); err != nil {
		return LikeState{}, err
	}
	if liked {
		_, err = tx.Exec(ctx, "INSERT INTO likes (post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", postID, userID)
	} else {
		_, err = tx.Exec(ctx, "DELETE FROM likes WHERE post_id = $1 AND user_id = $2", postID, userID)
	}
	if err != nil {
		return LikeState{}, err
	}
	var result LikeState
	err = tx.QueryRow(ctx, `SELECT count(*), COALESCE(bool_or(user_id = $2::uuid), false)
		FROM likes WHERE post_id = $1`, postID, userID).Scan(&result.LikeCount, &result.LikedByMe)
	if err != nil {
		return LikeState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return LikeState{}, err
	}
	return result, nil
}

func lockInteractionPost(ctx context.Context, tx pgx.Tx, postID string) error {
	var id string
	err := tx.QueryRow(ctx, "SELECT id::text FROM posts WHERE id = $1 FOR UPDATE", postID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func loadInteractions(ctx context.Context, db queryer, items []Post, viewerID string) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	index := make(map[string]int, len(items))
	for i := range items {
		ids[i], index[items[i].ID] = items[i].ID, i
	}
	rows, err := db.Query(ctx, `SELECT p.id::text,
		(SELECT count(*) FROM likes l WHERE l.post_id = p.id),
		(SELECT count(*) FROM comments c WHERE c.post_id = p.id),
		EXISTS (SELECT 1 FROM likes l WHERE l.post_id = p.id AND l.user_id = NULLIF($2, '')::uuid)
		FROM posts p WHERE p.id = ANY($1::uuid[])`, ids, viewerID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var count, comments int64
		var liked bool
		if err := rows.Scan(&id, &count, &comments, &liked); err != nil {
			return err
		}
		i := index[id]
		items[i].LikeCount, items[i].CommentCount, items[i].LikedByMe = count, comments, liked
	}
	return rows.Err()
}
