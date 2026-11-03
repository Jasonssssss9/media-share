# API contract

Application routes use `/api/v1`. JSON fields use snake_case.

## Implemented

| Method | Path | Result |
| --- | --- | --- |
| GET | /healthz | 200 when the API process is running |
| GET | /readyz | 200 when PostgreSQL responds; otherwise 503 |
| GET | /api/v1/status | 200 with application name and readiness; otherwise 503 |

```json
{"name":"MediaShare","status":"ready"}
```

Responses include `X-Request-ID`. The frontend calls `/api/v1/status` through Vite's proxy. Elasticsearch is not required for these endpoints.

## Planned business routes

These routes are contracts for later milestones, not working endpoints yet.

| Method | Path after /api/v1 | Purpose |
| --- | --- | --- |
| POST | /auth/register | Create an account and set the auth cookie |
| POST | /auth/login | Sign in and set the auth cookie |
| POST | /auth/logout | Clear the auth cookie |
| GET | /auth/me | Read the current account |
| GET | /users/{username} | Read a public profile |
| PATCH | /users/me | Update display_name, bio, or avatar_id |
| GET | /posts | List posts; optionally filter by author username |
| POST | /posts | Create a post with content and media_ids |
| GET | /posts/{id} | Read a post |
| PATCH | /posts/{id} | Update an owned post |
| DELETE | /posts/{id} | Delete an owned post |
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
| Create or edit a post | `content` and `media_ids` in display order |
| Create or edit a comment | `content` |

Public users have `id`, `username`, `display_name`, `bio`, and `avatar_url`. Auth responses also include the current user's `email`.

Posts have `id`, `author` (a public user), `content`, `images`, `like_count`, `comment_count`, `liked_by_me`, `created_at`, and `updated_at`. Images have `id`, `url`, `width`, and `height`. Comments have `id`, `author`, `content`, and timestamps.

Creation returns 201 with the new resource. Reads and updates return 200 with the resource. Deletion and logout return 204. Like and unlike return `like_count` and `liked_by_me`. IDs are UUIDs; timestamps are UTC RFC 3339 strings.

## Validation and pagination

- Username: 3-24 lowercase letters, digits, or underscores.
- Display name: 1-60 characters; bio: up to 280 characters.
- Password: 8-128 characters.
- Post content: up to 2,000 characters. A post needs text or an image.
- Comment: 1-500 characters.
- Images: JPEG, PNG, or WebP; at most four per post and 5 MiB each.
- Page: at least 1. Page size defaults to 20 and is capped at 50.
- Lists return `items`, `page`, `page_size`, and `total`.

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

Use 400 for invalid input, 401 for missing or expired authentication, 403 for insufficient ownership, 404 for missing content, 409 for duplicates, and 503 for a temporarily unavailable dependency. Unknown routes and unsupported methods currently use Go's standard 404/405 responses.
