# Design

MediaShare is a small media sharing project. Keep the implementation direct: one Go API, one React application, PostgreSQL, Elasticsearch, and a local upload directory.

## Features and permissions

| Action | Guest | Signed-in user |
| --- | --- | --- |
| Read posts, profiles, and search results | Yes | Yes |
| Create a post, upload an image, like, or comment | No | Yes |
| Edit a post, profile, or comment | No | Own content only |
| Delete a comment | No | Comment author or post author |

Posts are public. Only the owner can attach an uploaded image to a post. Pending uploads are private until attached.

## Data model

The [initial migration](../backend/migrations/00001_create_tables.sql) creates these tables; migration 2 adds `users.session_version`. Run `make migrate-up` to apply both.

| Table | Main fields |
| --- | --- |
| users | id, email, username, password_hash, display_name, bio, avatar_id, created_at, session_version |
| posts | id, author_id, content, created_at, updated_at |
| media | id, owner_id, post_id, storage_key, mime_type, size, width, height, position, created_at |
| likes | user_id, post_id, created_at |
| comments | id, post_id, author_id, content, created_at, updated_at |

Email uniqueness is case-insensitive; usernames are unique and lowercase. A like has a unique `(post_id, user_id)` pair. Foreign keys link users, posts, and media. An image can remain unassigned while it waits for a post or avatar update.

Deleting a post removes its likes and comments but keeps its media unassigned for file cleanup. Deleting an avatar image clears its profile reference. See [database notes](database.md) for commands and constraints.

PostgreSQL is the source of truth. Elasticsearch stores a derived post index for keyword searches and author filtering. Files stay under `var/media`; SQL stores their metadata.

## Authentication

Passwords use Argon2id with random salts. A 24-hour HS256 JWT is stored in an HttpOnly, SameSite=Lax cookie scoped to `/api`. It is Secure when `APP_ORIGIN` uses HTTPS. The signing key comes from `JWT_SECRET`.

`GET /auth/me` restores the current user after a page reload. Expired sessions return to sign-in. Logout clears the cookie and increments `users.session_version`, invalidating all existing tokens for that account. There is no refresh-token system.

Go's cross-origin protection checks state-changing requests, with `APP_ORIGIN` as a trusted origin. Registration and login share a small in-process limit: a burst of 10 attempts, replenishing at one per second.

Development uses Vite's API proxy, so browser requests stay on one origin. The backend does not need a permissive CORS policy.

## Screens

```text
Shared layout: MediaShare | Feed | Search | Sign in
Feed:          Post cards -> pagination
Post detail:   Author -> text and images -> likes -> comments
Editor:        Content -> image previews -> Publish
Profile:       Public information -> author's posts
Search:        Keyword + optional author -> results
Sign in:       Email + password -> Submit
Register:      Username + email + password -> Submit
```

Accounts, public feeds, author pages, post details, and a text editor are implemented. Post authors can edit and delete their content. Media, interactions, and search are planned.

Text posts use page-number pagination with creation time and ID as a deterministic order. Edits compare the original `updated_at` value in the SQL update, returning a conflict if another edit won. PostgreSQL handles each change directly; search synchronization will be added with search.

## Code layout

- `backend/cmd/api`: startup, database connection, and shutdown.
- `backend/internal/api`: routes, JSON responses, request logging, and recovery.
- `backend/internal/config`: environment configuration.
- `backend/internal/auth`: account validation, password hashing, and sessions.
- `backend/internal/posts`: post storage, public author data, and ownership checks.
- `backend/cmd/migrate` and `backend/migrations`: migration command and SQL.
- `backend/integration`: isolated PostgreSQL migration, authentication, and post tests.
- `web/src/app`: layout, routing setup, and theme.
- `web/src/features`: page-level features.
- `web/src/lib`: the shared HTTP client.

Add feature packages when their code is needed. Keep data access and business logic out of HTTP handlers as those features grow.

Include the user ID in query keys for user-specific frontend data, so switching accounts does not reuse another account's cached results.
