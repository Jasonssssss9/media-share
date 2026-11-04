# Database and migrations

PostgreSQL stores five business tables: `users`, `posts`, `media`, `likes`, and `comments`. Goose records migration history in `goose_db_version`.

## Commands

Run these from the project root after `make services`:

```bash
make migrate-up
make migrate-status
make test-db
```

`make migrate-down` rolls back the latest migration and deletes its affected data. Normal startup only needs `make migrate-up`; repeating it preserves existing data.

Migration files live in `backend/migrations`. Add the next numbered SQL file with `-- +goose Up` and `-- +goose Down` sections. Keep applied files unchanged. Each migration runs in a transaction.

## Schema rules

- UUID primary keys and timestamps have database defaults.
- Emails are unique regardless of case. Usernames use 3-24 lowercase letters, digits, or underscores.
- Foreign keys link users, posts, comments, likes, and media.
- Likes use a composite primary key to prevent duplicates.
- Text lengths match the API contract. Empty comments are rejected.
- Image metadata accepts JPEG, PNG, or WebP, positive dimensions, and sizes up to 5 MiB.
- Attached images occupy unique positions 0-3. Reordering can defer that unique constraint within a transaction.
- Deleting a post removes its likes and comments and sets its media's `post_id` to NULL.
- Deleting a media row clears any avatar reference. Database deletion does not remove filesystem files.

Feed, author-feed, comment, and ownership lookups have indexes. Business handlers will check ownership and require post text or an image; they will also set `updated_at` when editing content.

## Tests

`make test-db` uses the configured PostgreSQL database but creates a separate temporary schema. It checks application, repeat application with existing rows, rollback, reapplication, constraints, and related-row deletion. The schema and fixtures are removed afterward.

The migration runner uses [Goose](https://pressly.github.io/goose/documentation/annotations/) with the existing pgx PostgreSQL driver.
