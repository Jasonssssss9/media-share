# Design

MediaShare is a small media sharing project. Keep the implementation direct: one Go API, one React application, PostgreSQL, Elasticsearch, and a local upload directory.

## Features and permissions

| Action | Guest | Signed-in user |
| --- | --- | --- |
| Read posts, profiles, and search results | Yes | Yes |
| Create a post, upload or generate an image, like, or comment | No | Yes |
| Edit a post, profile, or comment | No | Own content only |
| Delete a comment | No | Comment author or post author |

Posts are public. Only the owner can attach an uploaded image to a post. Pending uploads are private until attached.

## Data model

The [initial migration](../backend/migrations/00001_create_tables.sql) creates these tables; migration 2 adds `users.session_version`, and migration 3 permits uppercase usernames. Run `make migrate-up` to apply all migrations.

| Table | Main fields |
| --- | --- |
| users | id, email, username, password_hash, display_name, bio, avatar_id, created_at, session_version |
| posts | id, author_id, content, created_at, updated_at |
| media | id, owner_id, post_id, storage_key, mime_type, size, width, height, position, created_at |
| likes | user_id, post_id, created_at |
| comments | id, post_id, author_id, content, created_at, updated_at |

Email uniqueness is case-insensitive; usernames preserve letter case and are unique with case-sensitive comparison. A like has a unique `(post_id, user_id)` pair. Foreign keys link users, posts, and media. An image can remain unassigned while it waits for a post or avatar update.

Deleting a post removes its likes and comments and detaches its images. After the transaction commits, the API discards unused image records and files. Images still used as avatars are kept. Deleting an avatar image clears its profile reference. See [database notes](database.md) for commands and constraints.

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
Editor:        Content -> upload or generate -> image previews -> Publish
Profile:       Public information -> author's posts
Account:       Display name + bio + avatar preview -> Save or Reset
Search:        Keyword + optional author -> results
Sign in:       Email + password -> Submit
Register:      Username + email + password -> Submit
```

Accounts, editable profiles and avatars, public feeds, author pages, post details, and an editor with uploads and optional image generation are implemented. Post authors can edit text, add or remove images, change their order, and delete posts. Search supports post text and exact author usernames.

Posts use page-number pagination with creation time and ID as a deterministic order. Edits check the original `updated_at` while holding a row lock. Content and ordered image links are saved in one transaction; invalid attachments roll back the whole change.

Uploads use generated filenames, decoded image formats and dimensions, a 5 MiB size limit, and a 25-megapixel limit. Draft uploads are private; the editor discards them on removal or navigation away. File cleanup follows committed database changes. Cleanup failures are logged without undoing a saved post. Remaining unassigned image records stay private.

## Profiles

The account page edits the display name, bio, and avatar. Usernames and emails stay fixed. The existing `users` fields support this feature without a new migration.

`PATCH /users/me` saves profile fields and the avatar reference in one transaction. Only owned images can become avatars, including images already used by the owner's posts. Image cleanup locks the media row and checks current references before deletion. Replaced avatars remain available when a post still uses them.

Uploads stay private until saved as avatars. Resetting or leaving the form discards unused uploads. Saving updates the current account cache and invalidates public profile and post queries.

## Likes and comments

The existing `likes` and `comments` tables support interactions without a new migration. Likes use the unique user/post pair, so repeated requests do not add duplicates. Counts are read from these tables rather than stored separately.

Anyone can read comments. Signed-in users can like posts and create comments; only comment authors can edit them. Comment authors and post authors can delete comments. Deleting a post removes both kinds of interaction through foreign keys.

The frontend shows counts on post cards and a paginated comment section on post details. New comments appear on the first page. Mutations invalidate related post and comment queries. Post query keys include the viewer's account ID because `liked_by_me` is personal; comment data stays public. Profile edits also invalidate comment authors.

## Search

The Go backend uses Elasticsearch's REST API directly. The `media-share-posts` index stores post ID, body text, exact author username, and timestamps. Text uses a full-text mapping; usernames use a keyword mapping without case normalization. Images and interaction counts stay in PostgreSQL.

After a post transaction commits, the API synchronizes that post with a three-second budget. Index writes are serialized within the API process and read the latest database revision. Successful writes wait for search visibility. Index errors are logged without changing the successful post response.

Search retrieves matching IDs, then hydrates current data from PostgreSQL in the same order. Deleted or outdated hits are skipped. Search failures return a clear 503 response while the rest of the app remains available.

`make search-reindex` replaces the configured index and reloads posts in batches. Stop the API before running it so posts do not change during the rebuild. This manual repair command keeps the local project simple; no background synchronization worker is needed. Pagination follows Elasticsearch's [10,000-result window](https://www.elastic.co/docs/reference/elasticsearch/rest-apis/paginate-search-results).

## Image generation

The Go backend calls OpenAI's Image API with a server-side key. One request produces a 1024x1024 JPEG at low quality and 80% output compression. The model is configurable; no provider SDK is needed for this single HTTP request.

Generated bytes go through the existing media service, so decoding, size limits, ownership, draft privacy, attachment ordering, and cleanup follow the same rules as uploads. No extra tables are needed.

Generation has a 150-second request budget and a longer write deadline for this endpoint. Other API endpoints keep their ordinary timeouts. One generation can run at a time, with a shared 30-second cooldown between attempts. These in-memory limits reset when the API restarts. Failed generation is not automatically retried.

Without `OPENAI_API_KEY`, the API still starts and the editor shows generation as unavailable. Automated tests use simulated provider responses.

## Code layout

- `backend/cmd/api`: startup, database connection, and shutdown.
- `backend/internal/api`: routes, JSON responses, request logging, and recovery.
- `backend/internal/config`: environment configuration.
- `backend/internal/auth`: account validation, profile updates, password hashing, and sessions.
- `backend/internal/posts`: post storage, public author data, image links, likes, and comments.
- `backend/internal/media`: local image storage, decoding, and access rules.
- `backend/internal/search`: index requests, synchronization, and search hydration.
- `backend/cmd/reindex`: rebuild the search index from PostgreSQL.
- `backend/internal/imagegen`: the image API client and response decoding.
- `backend/cmd/migrate` and `backend/migrations`: migration command and SQL.
- `backend/integration`: isolated database, image, interaction, and search tests.
- `web/src/app`: layout, routing setup, and theme.
- `web/src/features`: page-level features.
- `web/src/lib`: the shared HTTP client.

Add feature packages when their code is needed. Keep data access and business logic out of HTTP handlers as those features grow.

Include the user ID in query keys for user-specific frontend data, so switching accounts does not reuse another account's cached results.
