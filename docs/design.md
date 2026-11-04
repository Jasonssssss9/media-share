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

These tables are implemented by [the initial migration](../backend/migrations/00001_create_tables.sql). Run `make migrate-up` to create them.

| Table | Main fields |
| --- | --- |
| users | id, email, username, password_hash, display_name, bio, avatar_id, created_at |
| posts | id, author_id, content, created_at, updated_at |
| media | id, owner_id, post_id, storage_key, mime_type, size, width, height, position, created_at |
| likes | user_id, post_id, created_at |
| comments | id, post_id, author_id, content, created_at, updated_at |

Email uniqueness is case-insensitive; usernames are unique and lowercase. A like has a unique `(post_id, user_id)` pair. Foreign keys link users, posts, and media. An image can remain unassigned while it waits for a post or avatar update.

Deleting a post removes its likes and comments but keeps its media unassigned for file cleanup. Deleting an avatar image clears its profile reference. See [database notes](database.md) for commands and constraints.

PostgreSQL is the source of truth. Elasticsearch stores a derived post index for keyword searches and author filtering. Files stay under `var/media`; SQL stores their metadata.

## Authentication

Use password hashing and a signed JWT in an HttpOnly, SameSite cookie. The token expires after 24 hours; logout clears the browser cookie. State-changing browser requests must come from the configured frontend origin. There is no refresh-token system or separate session service in the initial version.

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

Only the shared layout, feed shell, and not-found page are implemented in this milestone. The feed checks service availability; it does not fetch posts yet.

## Code layout

- `backend/cmd/api`: startup, database connection, and shutdown.
- `backend/internal/api`: routes, JSON responses, request logging, and recovery.
- `backend/internal/config`: environment configuration.
- `backend/cmd/migrate` and `backend/migrations`: migration command, SQL, and database tests.
- `web/src/app`: layout, routing setup, and theme.
- `web/src/features`: page-level features.
- `web/src/lib`: the shared HTTP client.

Add feature packages when their code is needed. Keep data access and business logic out of HTTP handlers as those features grow.
