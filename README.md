# MediaShare

A student project for sharing images and posts, with a React frontend, a Go backend, and searchable content.

**Current progress:** accounts, profiles, image posts, optional image generation, likes, and comments work end to end. Search is still planned.

## Stack

- React, TypeScript, Vite, MUI, React Router, TanStack Query
- Go 1.27.3, net/http, pgx
- PostgreSQL 18.6 and Elasticsearch 9.5.6 through Docker Compose
- Node.js 24.22.2 through nvm

## Run locally

Use Ubuntu WSL with Docker Desktop's Ubuntu integration enabled.

```bash
cd /home/jason/projects/media-share
nvm use
```

For a fresh checkout, copy `.env.example` to `.env` and set `POSTGRES_PASSWORD`, `ELASTIC_PASSWORD`, and a random `JWT_SECRET` of at least 32 characters (`openssl rand -hex 32` can generate one). The configured development machine already has this file.

```bash
make install
make services
make migrate-up
```

Run the API and frontend in separate terminals from the project root:

```bash
make dev-api
```

```bash
nvm use
make dev-web
```

Open [http://localhost:5173](http://localhost:5173), create an account, and choose **New post**. Add text, up to four images, or both. Anyone can browse posts and author pages; only the author can edit or delete a post. Use **Your account** to edit your display name, bio, and avatar, or open your public profile. Sessions survive page reloads. Usernames are case-sensitive: `Alice` and `alice` can be separate accounts.

Signed-in users can like posts and leave comments on post details. Comment authors can edit their comments; comment authors and post authors can delete them. Comments show newest first.

Images are stored under `var/media` and are not committed. Each image can be a JPEG, PNG, or WebP up to 5 MiB and 25 megapixels. Avatar changes take effect when you choose **Save profile**; **Reset changes** discards unsaved edits.

## Image generation (optional)

Set `OPENAI_API_KEY` in your local `.env` then restart the API and reload the editor. `OPENAI_IMAGE_MODEL` defaults to `gpt-image-2.5-flare`. The key stays on the backend and is never sent to the browser.

In **New post** or **Edit post**, enter an image prompt and choose **Generate image**. Preview, reorder, remove, and publish generated images alongside uploads. Generation may take a couple of minutes. This local app allows one generation at a time, with 30 seconds between attempts.

Without a key, generation is shown as unavailable; accounts, posts, and uploads still work. API access and credits are required for real generation. See the [official image API reference](https://developers.openai.com/api/reference/resources/images/methods/generate).

## Checks

```bash
make check
make test-db
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

`make check` runs Go and HTTP client tests, go vet, frontend lint, and both builds. `make test-db` verifies migrations, authentication, profiles, avatars, post permissions, likes, comments, pagination, concurrent changes, uploads, generated images, and cleanup in temporary PostgreSQL schemas and temporary directories. Generation tests use a fake provider and never call the paid API. Run `make fmt` to format Go files. Press Ctrl+C in each terminal to stop the application; use `make stop` to stop supporting services and preserve their data.

## Database migrations

```bash
make migrate-up      # Apply pending migrations
make migrate-status  # Show applied and pending migrations
make migrate-down    # Roll back the latest migration
```

Rollback removes the data affected by that migration. See [database notes](docs/database.md).

## Documentation

- [Design and screen sketches](docs/design.md)
- [API contract](docs/api.md)
- [Database and migrations](docs/database.md)
- [Environment notes](docs/environment.md)
- [Troubleshooting](docs/troubleshooting.md)
