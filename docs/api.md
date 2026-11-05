# API reference

Business routes are relative to `/api/v1`. JSON uses snake_case; IDs are UUIDs and timestamps are UTC RFC 3339 strings. JSON requests use `Content-Type: application/json`.

## Health

| Method | Path | Result |
| --- | --- | --- |
| GET | /healthz | 200 when the API is running |
| GET | /readyz | 200 when PostgreSQL responds; otherwise 503 |
| GET | /api/v1/status | Application name and database readiness; 503 if unavailable |

## Routes

| Method | Path | Result |
| --- | --- | --- |
| POST | /auth/register | 201 with the account and session cookie |
| POST | /auth/login | 200 with the account and session cookie |
| POST | /auth/logout | 204; clear the cookie and revoke account sessions |
| GET | /auth/me | Current account; requires a valid session |
| GET | /users/{username} | Public profile |
| PATCH | /users/me | Updated account; signed-in user only |
| GET | /posts | Public feed; optional `author` username filter |
| POST | /posts | 201 with a new post; signed-in user only |
| GET | /posts/{id} | Public post detail |
| PATCH | /posts/{id} | Updated post; post author only |
| DELETE | /posts/{id} | 204; post author only |
| PUT | /posts/{id}/like | Like count and viewer state; signed-in user only |
| DELETE | /posts/{id}/like | Remove the viewer's like and return its state |
| GET | /posts/{id}/comments | Public paginated comments |
| POST | /posts/{id}/comments | 201 with a comment; signed-in user only |
| PATCH | /comments/{id} | Updated comment; comment author only |
| DELETE | /comments/{id} | 204; comment author or post author only |
| POST | /media | 201 with uploaded image metadata; signed-in user only |
| GET | /media/{id} | Public image, or an unused upload belonging to the viewer |
| DELETE | /media/{id} | 204; owner only, and the image must be unused |
| GET | /media/generation | `{"enabled":true}` when a backend key is configured |
| POST | /media/generation | 201 with generated image metadata; signed-in user only |
| GET | /search | Public post search by text, author, or both |

## Accounts and profiles

Register with `username`, `email`, and `password`; sign in with `email` and `password`. Emails are trimmed and lowercased. Usernames are trimmed but preserve letter case: `Alice` and `alice` are different accounts.

Authentication uses the `mediashare_session` HttpOnly cookie for 24 hours. Tokens are not returned in JSON or stored in browser storage. Sign-out revokes the account's existing sessions. Protected routes return 401 for invalid sessions; public post reads treat them as guests.

`PATCH /users/me` accepts any of `display_name`, `bio`, and `avatar_id`, with at least one field required. Omitted fields stay unchanged. Upload an owned image first, then send its ID as `avatar_id`; use `null` to remove the avatar. Owned post images can also serve as avatars. Usernames and emails cannot be changed.

## Posts, images, and comments

Create a post with `content` and optional ordered `media_ids`. Edit it with `content`, its original `updated_at`, and optional `media_ids`. A stale edit returns 409. Omitting `media_ids` preserves images; an empty array removes them.

Upload one image as multipart field `file` to `POST /media`. File contents determine its format. Uploads stay private until attached to a post or saved as an avatar. Only the uploader can attach an image to a post, and an image cannot belong to two posts. Replaced or removed images are discarded only when no post or avatar uses them.

Like and unlike need no body, are idempotent, and return `like_count` and `liked_by_me`. Both require a session. Comments accept `{"content":"A comment"}`; only the author can edit them, while the comment author or post author can delete them. Deleting a post removes its likes and comments. Interactions do not change the post's `updated_at`.

## Search and generation

`GET /search?q=lake&author=Alice&page=1&page_size=20` accepts keywords, an exact author username, or both. All analyzed search terms must match post text. Author-only searches also include image-only posts. Results sort by relevance, creation time, then ID, all descending, within the first 10,000 matches.

Search displays current PostgreSQL data for matching index IDs. Deleted or outdated hits are skipped, so a page may contain fewer items than its indexed `total` suggests. Unavailable search returns 503 with `search_unavailable`.

`POST /media/generation` accepts `{"prompt":"A small house beside a lake"}` and returns the same metadata as an upload. Attach the image through `media_ids`. The availability endpoint checks configuration, not provider access.

Generation allows one request at a time and 30 seconds between attempts across the app, with a 150-second budget. Rejected prompts return 400; missing keys or rejected credentials return 503; provider failures return 502; timeout returns 504. Limits return 429 with `Retry-After`. Failed requests are not automatically retried.

## Response shapes

| Resource | Fields |
| --- | --- |
| Public user | `id`, `username`, `display_name`, `bio`, `avatar_url` |
| Account | Public user fields plus `email` and `created_at` |
| Post | `id`, `author`, `content`, `images`, `created_at`, `updated_at`, `like_count`, `comment_count`, `liked_by_me` |
| Image | `id`, `url`, `width`, `height` |
| Comment | `id`, `post_id`, `author`, `content`, `created_at`, `updated_at` |
| Page | `items`, `page`, `page_size`, `total` |

Post and comment authors use the public user shape. `avatar_url` is nullable. Guests receive `liked_by_me: false`; valid sessions add the viewer's state.

Reads and updates return 200 unless noted. Post, comment, and image creation also return a `Location` header. Responses include `X-Request-ID`; account, post, and search responses use `Cache-Control: no-store`.

## Validation and pagination

| Field | Limit |
| --- | --- |
| Username | 3-24 ASCII letters, digits, or underscores; case-sensitive |
| Password | 8-128 Unicode characters |
| Display name / bio | 1-60 / up to 280 Unicode characters |
| Post text | Up to 2,000 Unicode characters; a post needs nonblank text or an image |
| Comment | 1-500 Unicode characters after trimming |
| Images | JPEG, PNG, or WebP; up to four per post, 5 MiB and 25 megapixels each |
| Search keywords | Up to 200 Unicode characters; keywords or an author are required |
| Image prompt | 1-1,000 Unicode characters after trimming |

Display names, bios, post text, comments, keywords, and prompts reject NUL characters. Post and comment lists use `created_at DESC, id DESC`. `page` starts at 1; `page_size` defaults to 20 and is capped at 50. Adding or deleting content can shift later pages.

## Errors

```json
{"error":{"code":"invalid_input","message":"...","request_id":"..."}}
```

Common statuses are 400 for invalid input, 401 for authentication, 403 for ownership or origin checks, 404 for missing content, 409 for duplicates, edit conflicts, or images in use, 413 for oversized uploads, 429 for limits, and 503 for unavailable dependencies. Unexpected server errors return 500. Image provider failures use 502 or 504 as described above. Unknown routes and unsupported methods use Go's standard 404/405 responses.
