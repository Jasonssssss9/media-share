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

Migration files live in `backend/migrations`. Add the next numbered SQL file with `-- +goose Up` and `-- +goose Down` sections. Keep applied files unchanged. Each migration runs in a transaction. Migration 3 can be rolled back only when all usernames fit the old lowercase-only rule; rollback never renames accounts.

## Schema rules

- UUID primary keys and timestamps have database defaults.
- Emails are unique regardless of case. Usernames use 3-24 ASCII letters, digits, or underscores. Their existing unique constraint is case-sensitive, so `Alice` and `alice` can coexist.
- Foreign keys link users, posts, comments, likes, and media.
- Likes use a composite primary key to prevent duplicates.
- Text lengths match the API contract. Empty comments are rejected.
- `users.session_version` invalidates existing JWTs when the account signs out.
- Image metadata accepts JPEG, PNG, or WebP, positive dimensions, and sizes up to 5 MiB. Uploads also enforce a 25-megapixel limit.
- Attached images occupy unique positions 0-3. Reordering can defer that unique constraint within a transaction.
- Deleting a post removes its likes and comments and sets its media's `post_id` to NULL.
- Deleting a media row clears any avatar reference. Post and profile handlers discard unused image records and files after committing changes. Database rollback alone does not remove filesystem files.

Profile edits use the existing display name, bio, and avatar fields. The profile and selected image rows are locked while validating ownership and saving changes. Removing an avatar preserves its image when a post still uses it.

Feed, author-feed, comment, and ownership lookups have indexes. Post handlers require text or an image. They lock the post and selected image rows, verify ownership and availability, and save content and image positions in one transaction. The original `updated_at` guards against conflicting edits.

Likes and comments use the existing tables. Like changes and comment creation briefly lock the parent post; comment edits and deletions check ownership while holding the comment row lock. Counts are calculated when reading posts, and interactions leave the post's edit timestamp unchanged.

## Tests

`make test-db` uses the configured PostgreSQL database but creates a separate temporary schema. Tests in `backend/integration` check migrations, constraints, related-row deletion, authentication, profile editing, avatar ownership and cleanup, post permissions, pagination, concurrent edits and likes, comment permissions, post-deletion cleanup, uploads, image access, and file cleanup. The schema, temporary upload directories, and fixtures are removed afterward.

The migration runner uses [Goose](https://pressly.github.io/goose/documentation/annotations/) with the existing pgx PostgreSQL driver.
