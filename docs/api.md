# API contract

Application routes use `/api/v1`. JSON fields use snake_case.

## Implemented

| Method | Path | Result |
| --- | --- | --- |
| GET | /healthz | 200 when the API process is running |
| GET | /readyz | 200 when PostgreSQL responds; otherwise 503 |
| GET | /api/v1/status | 200 with application name and readiness; otherwise 503 |
| POST | /api/v1/auth/register | 201 with the account and session cookie |
| POST | /api/v1/auth/login | 200 with the account and session cookie |
| POST | /api/v1/auth/logout | 204; clear the cookie and revoke existing account sessions |
| GET | /api/v1/auth/me | 200 with the current account; 401 for an invalid session |
| GET | /api/v1/posts | Public feed; optionally filter by author username |
| POST | /api/v1/posts | 201 with a text post owned by the current account |
| GET | /api/v1/posts/{id} | Public post detail |
| PATCH | /api/v1/posts/{id} | 200 with an edited post; author only |
| DELETE | /api/v1/posts/{id} | 204 after deleting an owned post |
| GET | /api/v1/users/{username} | Public author profile, excluding email |

```json
{"name":"MediaShare","status":"ready"}
```

Responses include `X-Request-ID`. The frontend calls `/api/v1` through Vite's proxy. Elasticsearch is not required for the current endpoints.

Authentication uses the `mediashare_session` HttpOnly cookie. Browser requests use the same-origin Vite proxy. Sessions last 24 hours; expired sessions require sign-in again. Account responses use `Cache-Control: no-store`. Tokens stay out of localStorage, sessionStorage, and JSON responses.

Register with `username`, `email`, and `password`; log in with `email` and `password`. Emails are trimmed and lowercased. Both return `id`, `email`, `username`, `display_name`, `bio`, `avatar_url`, and `created_at`.

Missing, forged, expired, or revoked sessions return 401. Duplicate accounts return 409, invalid bodies return 400, cross-site mutations return 403, and excessive authentication attempts return 429.

## Planned business routes

These routes are contracts for later milestones, not working endpoints yet.

| Method | Path after /api/v1 | Purpose |
| --- | --- | --- |
| PATCH | /users/me | Update display_name, bio, or avatar_id |
| POST | /media | Upload one image as multipart field file |
| GET | /media/{id} | Read an accessible image |
| DELETE | /media/{id} | Discard an owned, unassigned image |
| PUT | /posts/{id}/like | Like a post idempotently |
| DELETE | /posts/{id}/like | Remove a like idempotently |
| GET, POST | /posts/{id}/comments | List or create comments |
| PATCH, DELETE | /comments/{id} | Edit or remove a comment with the required ownership |
| GET | /search | Search posts using q and optional author username |

## Request and response shapes

| Request | JSON body |
| --- | --- |
| Register | `username`, `email`, `password` |
| Login | `email`, `password` |
| Update profile | Any of `display_name`, `bio`, `avatar_id` |
| Create a post | `content` |
| Edit a post | `content` and the original `updated_at` |
| Create or edit a comment | `content` |

Public users have `id`, `username`, `display_name`, `bio`, and `avatar_url`. Auth responses also include the current user's `email`.

Text posts return `id`, `author` (a public user), `content`, `created_at`, and `updated_at`. An edit must include the original `updated_at`; a stale edit returns 409 without overwriting newer content.

Uploads will add `media_ids` to post requests and `images` to responses. Images will have `id`, `url`, `width`, and `height`. Interactions will add `like_count`, `comment_count`, and `liked_by_me`. Comments will contain `id`, `author`, `content`, and timestamps.

Creation returns 201 with the new resource. Reads and updates return 200 with the resource. Deletion and logout return 204. Like and unlike return `like_count` and `liked_by_me`. IDs are UUIDs; timestamps are UTC RFC 3339 strings.

## Validation and pagination

- Username: 3-24 lowercase letters, digits, or underscores.
- Display name: 1-60 characters; bio: up to 280 characters.
- Password: 8-128 Unicode characters.
- Post content: 1-2,000 Unicode characters; whitespace-only posts are rejected.
- Comment: 1-500 characters.
- Images: JPEG, PNG, or WebP; at most four per post and 5 MiB each.
- Page: at least 1. Page size defaults to 20 and is capped at 50.
- Lists return `items`, `page`, `page_size`, and `total`.

Posts are ordered by `created_at DESC, id DESC`. Page numbers use LIMIT/OFFSET; adding or deleting posts can shift later pages.

The server is authoritative for validation and ownership. Public profile responses exclude email addresses and password hashes.

## Application errors

```json
{
  "error": {
    "code": "service_unavailable",
    "message": "MediaShare is temporarily unavailable.",
    "request_id": "..."
  }
}
```

Use 400 for invalid input, 401 for missing or expired authentication, 403 for insufficient ownership, 404 for missing content, 409 for duplicates or edit conflicts, and 503 for a temporarily unavailable dependency. Unknown routes and unsupported methods currently use Go's standard 404/405 responses.
