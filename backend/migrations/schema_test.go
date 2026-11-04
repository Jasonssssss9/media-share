//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Jasonssssss9/media-share/backend/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestSchema(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := testDatabase(t)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}

	t.Run("apply_and_repeat", func(t *testing.T) {
		if err := goose.UpContext(ctx, db, "."); err != nil {
			t.Fatal(err)
		}
		if err := goose.UpContext(ctx, db, "."); err != nil {
			t.Fatal(err)
		}
		checkVersion(t, ctx, db, 1)
		for _, table := range []string{"users", "posts", "media", "likes", "comments"} {
			var exists bool
			if err := db.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil || !exists {
				t.Fatalf("table %s missing: %v", table, err)
			}
		}
	})

	var userID, postID, mediaID, avatarID string
	t.Run("store_related_records", func(t *testing.T) {
		if err := db.QueryRowContext(ctx, `
			INSERT INTO users (email, username, password_hash, display_name)
			VALUES ('alex@example.com', 'alex', 'test-hash', 'Alex')
			RETURNING id::text`).Scan(&userID); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, "INSERT INTO posts (author_id, content) VALUES ($1, 'First post') RETURNING id::text", userID).Scan(&postID); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, `
			INSERT INTO media (owner_id, post_id, storage_key, mime_type, size, width, height)
			VALUES ($1, $2, 'photos/first.jpg', 'image/jpeg', 1024, 800, 600)
			RETURNING id::text`, userID, postID).Scan(&mediaID); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, `
			INSERT INTO media (owner_id, storage_key, mime_type, size, width, height)
			VALUES ($1, 'avatars/alex.png', 'image/png', 512, 64, 64)
			RETURNING id::text`, userID).Scan(&avatarID); err != nil {
			t.Fatal(err)
		}
		mustExec(t, ctx, db, "UPDATE users SET avatar_id = $1 WHERE id = $2", avatarID, userID)
		mustExec(t, ctx, db, "INSERT INTO likes (post_id, user_id) VALUES ($1, $2)", postID, userID)
		mustExec(t, ctx, db, "INSERT INTO comments (post_id, author_id, content) VALUES ($1, $2, 'Nice photo')", postID, userID)

		if err := goose.UpContext(ctx, db, "."); err != nil {
			t.Fatal(err)
		}

		var content string
		var createdAt time.Time
		if err := db.QueryRowContext(ctx, "SELECT content, created_at FROM posts WHERE id = $1", postID).Scan(&content, &createdAt); err != nil {
			t.Fatal(err)
		}
		if content != "First post" || createdAt.IsZero() {
			t.Fatal("post values or timestamp were not stored")
		}
	})

	if userID == "" || postID == "" || mediaID == "" || avatarID == "" {
		t.Fatal("fixtures could not be created")
	}

	t.Run("constraints", func(t *testing.T) {
		for _, test := range []struct {
			name string
			sql  string
			args []any
			code string
		}{
			{"email_ignores_case", "INSERT INTO users (email, username, password_hash, display_name) VALUES ('ALEX@example.com', 'other', 'hash', 'Other')", nil, "23505"},
			{"unique_username", "INSERT INTO users (email, username, password_hash, display_name) VALUES ('other@example.com', 'alex', 'hash', 'Other')", nil, "23505"},
			{"username_format", "INSERT INTO users (email, username, password_hash, display_name) VALUES ('other@example.com', 'Bad Name', 'hash', 'Other')", nil, "23514"},
			{"duplicate_like", "INSERT INTO likes (post_id, user_id) VALUES ($1, $2)", []any{postID, userID}, "23505"},
			{"missing_author", "INSERT INTO posts (author_id) VALUES (gen_random_uuid())", nil, "23503"},
			{"missing_post", "INSERT INTO comments (post_id, author_id, content) VALUES (gen_random_uuid(), $1, 'Hello')", []any{userID}, "23503"},
			{"empty_comment", "INSERT INTO comments (post_id, author_id, content) VALUES ($1, $2, '   ')", []any{postID, userID}, "23514"},
			{"long_comment", "INSERT INTO comments (post_id, author_id, content) VALUES ($1, $2, $3)", []any{postID, userID, strings.Repeat("x", 501)}, "22001"},
			{"long_post", "INSERT INTO posts (author_id, content) VALUES ($1, $2)", []any{userID, strings.Repeat("x", 2001)}, "22001"},
			{"large_image", "INSERT INTO media (owner_id, storage_key, mime_type, size, width, height) VALUES ($1, 'large.jpg', 'image/jpeg', 5242881, 800, 600)", []any{userID}, "23514"},
			{"unsupported_image", "INSERT INTO media (owner_id, storage_key, mime_type, size, width, height) VALUES ($1, 'file.svg', 'image/svg+xml', 50, 10, 10)", []any{userID}, "23514"},
			{"duplicate_position", "INSERT INTO media (owner_id, post_id, storage_key, mime_type, size, width, height) VALUES ($1, $2, 'duplicate.jpg', 'image/jpeg', 100, 10, 10)", []any{userID, postID}, "23505"},
			{"fifth_position", "INSERT INTO media (owner_id, post_id, storage_key, mime_type, size, width, height, position) VALUES ($1, $2, 'fifth.jpg', 'image/jpeg', 100, 10, 10, 4)", []any{userID, postID}, "23514"},
		} {
			t.Run(test.name, func(t *testing.T) {
				_, err := db.ExecContext(ctx, test.sql, test.args...)
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != test.code {
					t.Fatalf("got %v, want PostgreSQL error %s", err, test.code)
				}
			})
		}
	})

	t.Run("idempotent_like", func(t *testing.T) {
		result := mustExec(t, ctx, db, "INSERT INTO likes (post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", postID, userID)
		rows, err := result.RowsAffected()
		if err != nil || rows != 0 {
			t.Fatalf("duplicate like inserted: rows=%d, error=%v", rows, err)
		}
	})

	t.Run("reorder_images", func(t *testing.T) {
		mustExec(t, ctx, db, `
			INSERT INTO media (owner_id, post_id, storage_key, mime_type, size, width, height, position)
			VALUES ($1, $2, 'photos/second.jpg', 'image/jpeg', 1024, 800, 600, 1)`, userID, postID)
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, "SET CONSTRAINTS media_post_position_key DEFERRED"); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE media SET position = 1 - position WHERE post_id = $1", postID); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("delete_relationships", func(t *testing.T) {
		mustExec(t, ctx, db, "DELETE FROM media WHERE id = $1", avatarID)
		var avatarCleared bool
		if err := db.QueryRowContext(ctx, "SELECT avatar_id IS NULL FROM users WHERE id = $1", userID).Scan(&avatarCleared); err != nil || !avatarCleared {
			t.Fatalf("avatar reference was not cleared: %v", err)
		}

		mustExec(t, ctx, db, "DELETE FROM posts WHERE id = $1", postID)
		for _, table := range []string{"likes", "comments"} {
			var count int
			if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
				t.Fatalf("%s rows remain after post deletion: %v", table, err)
			}
		}
		var detached bool
		if err := db.QueryRowContext(ctx, "SELECT post_id IS NULL FROM media WHERE id = $1", mediaID).Scan(&detached); err != nil || !detached {
			t.Fatalf("media metadata was not retained for cleanup: %v", err)
		}
		mustExec(t, ctx, db, "DELETE FROM users WHERE id = $1", userID)
		var count int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM media").Scan(&count); err != nil || count != 0 {
			t.Fatalf("owned media rows remain after user deletion: %v", err)
		}
	})

	t.Run("rollback_and_reapply", func(t *testing.T) {
		if err := goose.DownContext(ctx, db, "."); err != nil {
			t.Fatal(err)
		}
		checkVersion(t, ctx, db, 0)
		for _, table := range []string{"users", "posts", "media", "likes", "comments"} {
			var removed bool
			if err := db.QueryRowContext(ctx, "SELECT to_regclass($1) IS NULL", table).Scan(&removed); err != nil || !removed {
				t.Fatalf("table %s remains after rollback: %v", table, err)
			}
		}
		if err := goose.UpContext(ctx, db, "."); err != nil {
			t.Fatal(err)
		}
		checkVersion(t, ctx, db, 1)
	})
}

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	admin, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	schema := fmt.Sprintf("test_migrations_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Errorf("clean up test schema: %v", err)
		}
	})

	connection, err := pgx.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	connection.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*connection)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustExec(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) sql.Result {
	t.Helper()
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func checkVersion(t *testing.T, ctx context.Context, db *sql.DB, want int64) {
	t.Helper()
	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil || version != want {
		t.Fatalf("migration version=%d, want %d: %v", version, want, err)
	}
}
