# Database and migrations

PostgreSQL is the source of truth. Image files live outside the database; Elasticsearch holds a derived search index.

## Tables

| Table | Purpose |
| --- | --- |
| `users` | Accounts, password hashes, profiles, avatars, and session versions |
| `posts` | Post authors, text, and timestamps |
| `media` | Image ownership, file metadata, and optional post position |
| `likes` | Unique user/post pairs |
| `comments` | Comment text, authors, posts, and timestamps |

Emails are unique regardless of case; usernames are case-sensitive. UUIDs and timestamps have database defaults. Foreign keys link related rows, and indexes support feeds, comments, and ownership lookups.

Post writes validate ownership, require text or an image, and save text and image links in one transaction. Edits check the original `updated_at`. Likes and comments do not change that timestamp.

Images have unique positions 0-3 within a post; reordering can defer that constraint within a transaction. An owned image may also serve as an avatar. Deleting a post removes its likes and comments and detaches its media. The API then discards unused image records and files, preserving images still used as avatars. Deleting a media row clears its avatar reference.

See the [API reference](api.md) for field limits and the [SQL migrations](../backend/migrations) for the full schema.

## Migrations

Run from the repository root with PostgreSQL running:

```bash
make migrate-up      # Apply pending migrations
make migrate-status  # Show migration status
make migrate-down    # Roll back the latest migration
```

Goose records applied migrations in `goose_db_version`; each migration runs in a transaction. Repeating `make migrate-up` preserves existing data. Rollback can remove database data; it does not remove image files.

Migration 1 creates the tables, migration 2 adds session revocation, and migration 3 permits uppercase usernames. Migration 3 can roll back only when every username meets the earlier lowercase-only rule; it never renames accounts.

Add new numbered files under `backend/migrations` with `-- +goose Up` and `-- +goose Down` sections. Keep applied migrations unchanged. See [testing](testing.md) for schema and API checks.
