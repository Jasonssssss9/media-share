# Architecture

MediaShare runs as one React application and one Go API. During development, Vite forwards `/api` requests to the Go API, keeping browser requests on one origin.

| Component | Responsibility |
| --- | --- |
| React | Pages, forms, navigation, and query caching |
| Go API | Authentication, validation, permissions, and application logic |
| PostgreSQL | Accounts, posts, profiles, media metadata, likes, and comments |
| Elasticsearch | A derived index for post text and author searches |
| Local storage | Image files under `var/media` |
| OpenAI | Optional image generation requested by the Go API |

## Accounts and permissions

Passwords use Argon2id. A 24-hour HS256 JWT is stored in an HttpOnly, SameSite=Lax cookie scoped to `/api`; HTTPS origins also use Secure cookies. Sign-out increments `users.session_version` to revoke the account's existing sessions. State-changing requests use Go's cross-origin protection.

Posts, profiles, comments, and search are public. Signed-in users can create content and like posts. Only the author can edit or delete a post or edit a comment. Comment authors and post authors can delete comments.

## Posts, profiles, and images

Post text and ordered image links are saved in one transaction. Ownership checks and row locks protect changes; the original `updated_at` prevents conflicting post edits. Likes are unique per user and post, and interaction counts are calculated when reading posts.

Uploads remain private until used in a public post or avatar. Avatars must use owned images and can share an image with a post. Removed images are discarded after the database change commits, but kept if a post or profile still uses them.

## Search and generation

The search index stores post ID, body text, exact username, and timestamps. After a post change commits, index synchronization reads the latest database revision with a three-second budget. Writes are serialized within the API process and wait for search visibility. Failures are logged without changing the successful post response.

Search gets matching IDs from Elasticsearch, then reads current posts, profiles, images, and interaction data from PostgreSQL in result order. Deleted or outdated hits are skipped. The [rebuild command](troubleshooting.md#search-is-unavailable-or-outdated) restores the index from PostgreSQL.

Image generation uses a small HTTP client with a server-side key. Each request produces one 1024x1024 JPEG at low quality and 80% output compression. Generated images use the same validation, ownership, storage, and cleanup rules as uploads.

## Frontend behavior

TanStack Query caches API data. Post query keys include the viewer's account ID because like state is personal. Mutations invalidate related data, and profile changes update author information across pages.

Login, post saves, and deletion update cached data after completion but redirect only while their originating screen is still active. The header wraps when space is limited.

## Code layout

| Path | Contents |
| --- | --- |
| `backend/cmd` | API, migration, and index rebuild commands |
| `backend/internal` | HTTP handlers, configuration, authentication, posts, media, search, and generation |
| `backend/migrations` | Numbered SQL migrations |
| `backend/integration` | Database and API integration tests |
| `web/src/features` | Account, feed, post, and search pages |
| `web/src/app`, `web/src/lib` | Shared layout, theme, and HTTP client |

## Scope and limits

This is a local project with one API process and local image storage. Sessions require sign-in again after 24 hours. Search uses page numbers within the first 10,000 matches; missed updates require a manual rebuild. Draft cleanup is best effort, so interrupted requests can leave private, unused images.

Generation allows one request at a time and 30 seconds between attempts across the app, with a 150-second request budget. These in-memory limits reset when the API restarts. Failed generation is not automatically retried.
