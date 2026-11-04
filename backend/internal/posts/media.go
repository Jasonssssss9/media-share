package posts

import (
	"context"
	"errors"

	"github.com/Jasonssssss9/media-share/backend/internal/media"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrImages     = errors.New("Choose up to four distinct uploaded images.")
	ErrImageOwner = errors.New("You can only attach your own images.")
	ErrImageInUse = errors.New("This image is already attached to another post.")
)

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func normalizeImageIDs(ids []string) ([]string, error) {
	if ids == nil {
		return nil, nil
	}
	if len(ids) > 4 {
		return nil, ErrImages
	}
	result := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		var value pgtype.UUID
		if err := value.Scan(id); err != nil || !value.Valid {
			return nil, ErrImages
		}
		id = value.String()
		if seen[id] {
			return nil, ErrImages
		}
		seen[id] = true
		result = append(result, id)
	}
	return result, nil
}

func setImages(ctx context.Context, tx pgx.Tx, postID, ownerID string, ids []string) error {
	if len(ids) > 0 {
		rows, err := tx.Query(ctx, `SELECT id::text, owner_id::text, post_id::text
			FROM media WHERE id = ANY($1::uuid[]) ORDER BY id FOR UPDATE`, ids)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			var id, actualOwner string
			var actualPost *string
			if err := rows.Scan(&id, &actualOwner, &actualPost); err != nil {
				rows.Close()
				return err
			}
			if actualOwner != ownerID {
				rows.Close()
				return ErrImageOwner
			}
			if actualPost != nil && *actualPost != postID {
				rows.Close()
				return ErrImageInUse
			}
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if count != len(ids) {
			return ErrImages
		}
	}
	if _, err := tx.Exec(ctx, "UPDATE media SET post_id = NULL WHERE post_id = $1", postID); err != nil {
		return err
	}
	for position, id := range ids {
		if _, err := tx.Exec(ctx, "UPDATE media SET post_id = $1, position = $2 WHERE id = $3", postID, position, id); err != nil {
			return err
		}
	}
	return nil
}

func loadImages(ctx context.Context, db queryer, items []Post) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	index := make(map[string]int, len(items))
	for i := range items {
		ids[i], index[items[i].ID] = items[i].ID, i
		items[i].Images = []media.Image{}
	}
	rows, err := db.Query(ctx, `SELECT id::text, post_id::text, width, height
		FROM media WHERE post_id = ANY($1::uuid[]) ORDER BY post_id, position`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var image media.Image
		var postID string
		if err := rows.Scan(&image.ID, &postID, &image.Width, &image.Height); err != nil {
			return err
		}
		image.URL = "/api/v1/media/" + image.ID
		i := index[postID]
		items[i].Images = append(items[i].Images, image)
	}
	return rows.Err()
}

func imageIDs(images []media.Image) []string {
	ids := make([]string, len(images))
	for i, image := range images {
		ids[i] = image.ID
	}
	return ids
}

func removedImages(before, after []media.Image) []string {
	kept := map[string]bool{}
	for _, image := range after {
		kept[image.ID] = true
	}
	var removed []string
	for _, image := range before {
		if !kept[image.ID] {
			removed = append(removed, image.ID)
		}
	}
	return removed
}
