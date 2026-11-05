# MediaShare

A student project for sharing text and image posts, built with React, Go, PostgreSQL, and Elasticsearch.

- Register and sign in with cookie-based JWT sessions.
- Create, edit, and delete posts with up to four uploaded or generated images.
- Edit your display name, bio, and avatar.
- Like posts and add comments with ownership checks.
- Search post text and filter by an exact, case-sensitive username.

## Stack

- Frontend: React, TypeScript, Vite, MUI, React Router, TanStack Query
- Backend: Go, net/http, pgx, Goose
- Services: PostgreSQL and Elasticsearch through Docker Compose
- Optional image generation: OpenAI's Image API

## Run locally

Use Ubuntu WSL with Go, Node.js through nvm, and Docker Desktop's Ubuntu integration enabled. See [environment notes](docs/environment.md) for versions and configuration.

Run commands from the repository root. On a fresh checkout, copy `.env.example` to `.env` and set `POSTGRES_PASSWORD`, `ELASTIC_PASSWORD`, and a random `JWT_SECRET` of at least 32 characters. Keep `.env` private.

```bash
nvm use
make install
make services
make migrate-up
make search-reindex
```

Start the API in one terminal:

```bash
make dev-api
```

Start the frontend in another:

```bash
nvm use
make dev-web
```

Open [localhost:5173](http://localhost:5173), create an account, and choose **New post**. Guests can browse public posts, profiles, comments, and search results.

Press Ctrl+C in both terminals to stop the app. Run `make stop` to stop the Docker services and keep their data.

## Image generation

Set `OPENAI_API_KEY` in `.env`, then restart the API and reload the editor. Set `OPENAI_IMAGE_MODEL` if your account uses a different model. In **New post** or **Edit post**, enter a prompt and choose **Generate image**, then publish the result alongside uploaded images.

The key stays on the backend. Real generation requires API access and credits; the rest of the app and automated tests work without a key.

## Search

Search matches post text and can filter by an exact author username. `Alice` and `alice` are separate usernames. Uploaded and generated images appear with their posts.

Post changes update the search index after saving to PostgreSQL. Index failures do not undo saved posts. Follow the [rebuild steps](docs/troubleshooting.md#search-is-unavailable-or-outdated) to initialize existing data or repair missed updates.

## Checks

```bash
make check
make test-db
```

`make check` runs tests, lint, and both builds. `make test-db` uses real PostgreSQL and Elasticsearch with isolated test data; both Docker services must be running. Generation tests use a fake provider. See [testing notes](docs/testing.md) for manual checks.

## Documentation

- [Architecture](docs/design.md)
- [API reference](docs/api.md)
- [Database and migrations](docs/database.md)
- [Environment and configuration](docs/environment.md)
- [Testing](docs/testing.md)
- [Troubleshooting](docs/troubleshooting.md)
