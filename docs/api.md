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
| GET | /api/v1/search | Public post search by text and exact author username |
| GET | /api/v1/posts | Public feed; optionally filter by author username |
| POST | /api/v1/posts | 201 with an owned text or image post |
| GET | /api/v1/posts/{id} | Public post detail |
| PATCH | /api/v1/posts/{id} | 200 with an edited post; author only |
| DELETE | /api/v1/posts/{id} | 204 after deleting an owned post |
| PUT | /api/v1/posts/{id}/like | 200 with like count and current user state |
| DELETE | /api/v1/posts/{id}/like | 200 after removing the current user's like |
| GET | /api/v1/posts/{id}/comments | Public paginated comments |
| POST | /api/v1/posts/{id}/comments | 201 with a new comment; signed-in users only |
| PATCH | /api/v1/comments/{id} | 200 with an edited comment; comment author only |
| DELETE | /api/v1/comments/{id} | 204; comment author or post author only |
| GET | /api/v1/users/{username} | Public author profile, excluding email |
| PATCH | /api/v1/users/me | Update the current profile and return the account |
| POST | /api/v1/media | 201 with the uploaded image metadata |
| GET | /api/v1/media/generation | Whether image generation is configured (`enabled`) |
| POST | /api/v1/media/generation | 201 with generated image metadata; signed-in users only |
| GET | /api/v1/media/{id} | Image bytes; unassigned uploads require their owner |
| DELETE | /api/v1/media/{id} | 204 after discarding an owned, unused upload |

```json
{"name":"MediaShare","status":"ready"}
```

Responses include `X-Request-ID`. The frontend calls `/api/v1` through Vite's proxy. Only search requires Elasticsearch; readiness checks PostgreSQL.

Authentication uses the `mediashare_session` HttpOnly cookie. Browser requests use the same-origin Vite proxy. Sessions last 24 hours; expired sessions require sign-in again. Account responses use `Cache-Control: no-store`. Tokens stay out of localStorage, sessionStorage, and JSON responses.

Register with `username`, `email`, and `password`; log in with `email` and `password`. Emails are trimmed and lowercased. Usernames are trimmed but keep their letter case; profile paths and author filters match it exactly. Both return `id`, `email`, `username`, `display_name`, `bio`, `avatar_url`, and `created_at`.

Missing, forged, expired, or revoked sessions return 401. Duplicate accounts return 409, invalid bodies return 400, cross-site mutations return 403, and excessive authentication attempts return 429.

## Profile updates

`PATCH /users/me` accepts any of `display_name`, `bio`, and `avatar_id`, and requires at least one field. Omitted fields stay unchanged. Display names and bios are trimmed, must be strings without NUL, and use the limits below. Identity fields such as username and email cannot be edited here.

Upload an image through `POST /media`, then send its ID as `avatar_id` to save it as an avatar. Send `"avatar_id": null` to remove the avatar. The image must belong to the current account; owned post images can also be used. Replaced images are discarded only when neither a post nor a profile uses them.

Success returns the same account fields as `GET /auth/me`, with no new session cookie. Profile changes are public through author and post responses; email remains private.

## Likes and comments

Post responses include `like_count`, `comment_count`, and `liked_by_me`. Guests receive `liked_by_me: false`; a valid session adds that viewer's state. Public post reads still work with an expired session. Responses use `Cache-Control: no-store`.

`PUT /posts/{id}/like` and `DELETE /posts/{id}/like` need no body and return `like_count` and `liked_by_me`. Both are idempotent. Signed-in users may also like their own posts.

Comments accept `{"content":"A comment"}`. Text is trimmed and must contain 1-500 Unicode characters without NUL. Only the comment author can edit it. The comment author or post author can delete it. Comment edits use the last saved value; no edit-version field is required.

Comment lists use the same page parameters and response envelope as posts, ordered by `created_at DESC, id DESC`. Creating a comment returns 201 with its data and a `Location` header. Deleting a post removes its likes and comments. Interactions do not change a post's `updated_at`.

## Image generation

`GET /media/generation` reports whether a backend key is configured; it does not verify provider access. `POST /media/generation` accepts `{"prompt":"A small house beside a lake"}` and returns the same `id`, `url`, `width`, and `height` fields as an upload, plus a `Location` header. Attach the result through a post's `media_ids`.

Prompts are trimmed and must contain 1-1,000 Unicode characters without NUL. Generation requires a session and uses the same origin checks as other mutations. It allows one request at a time and 30 seconds between attempts across this local app. A 429 response includes `Retry-After`.

Missing keys or rejected provider credentials return 503, a rejected prompt returns 400, provider failures or unusable images return 502, and generation timeout returns 504. Raw provider messages are not returned. The request budget is 150 seconds, and failed requests are not automatically retried.

## Search

`GET /search?q=lake&author=Alice&page=1&page_size=20` returns the same page envelope and post fields as the feed. Supply `q`, `author`, or both. Keywords are trimmed, limited to 200 Unicode characters without NUL, and matched against post text with all terms required. The author filter is an exact, case-sensitive username; author-only searches also include image-only posts.

Results use relevance, then creation time and ID in descending order. Standard page parameters apply, within the first 10,000 matches. Invalid criteria or deeper offsets return 400; an unavailable index returns 503 with `search_unavailable`.

Elasticsearch supplies matching IDs; PostgreSQL supplies the displayed posts, images, profiles, and viewer-specific likes. Deleted posts and hits with an outdated post revision are omitted. Until a failed update is repaired, a page can contain fewer items than its indexed `total` suggests.

## Request and response shapes

| Request | JSON body |
| --- | --- |
| Register | `username`, `email`, `password` |
| Login | `email`, `password` |
| Generate an image | `prompt` |
| Update profile | Any of `display_name`, `bio`, `avatar_id` |
| Create a post | `content` and optional ordered `media_ids` |
| Edit a post | `content`, original `updated_at`, and optional ordered `media_ids` |
| Create or edit a comment | `content` |

Public users have `id`, `username`, `display_name`, `bio`, and `avatar_url`. Auth responses also include the current user's `email`.

Posts return `id`, `author` (a public user), `content`, `images`, `created_at`, `updated_at`, `like_count`, `comment_count`, and `liked_by_me`. Images contain `id`, `url`, `width`, and `height`. Omitting `media_ids` on an edit preserves images; an empty array removes them. An edit must include the original `updated_at`; a stale edit returns 409 without overwriting newer content.

Comments contain `id`, `post_id`, `author`, `content`, `created_at`, and `updated_at`. Authors use the public user shape.

Upload one image as multipart field `file`. Actual file contents determine its format. Unassigned images are private; attached post images and profile avatars are public. Only the owner can attach an image, and it cannot belong to two posts.

Creation returns 201 with the new resource. Reads and updates return 200 with the resource. Deletion and logout return 204. Like and unlike return `like_count` and `liked_by_me`. IDs are UUIDs; timestamps are UTC RFC 3339 strings.

## Validation and pagination

- Username: 3-24 ASCII letters (A-Z, a-z), digits, or underscores. Case-sensitive; `Alice` and `alice` are different usernames.
- Display name: 1-60 Unicode characters; bio: up to 280 Unicode characters.
- Password: 8-128 Unicode characters.
- Post content: up to 2,000 Unicode characters. A post needs nonblank text or at least one image.
- Comment: 1-500 Unicode characters after trimming.
- Images: JPEG, PNG, or WebP; at most four per post, 5 MiB and 25 megapixels each.
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

Use 400 for invalid input, 401 for missing or expired authentication, 403 for insufficient ownership, 404 for missing content, 409 for duplicates, edit conflicts, or an image already in use, 413 for an oversized upload, 429 for rate limits, 502 for image provider failures, 503 for a temporarily unavailable dependency, and 504 for image generation timeout. Unknown routes and unsupported methods currently use Go's standard 404/405 responses.
