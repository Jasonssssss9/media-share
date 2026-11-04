# MediaShare

A student project for sharing images and posts, with a React frontend, a Go backend, and searchable content.

**Current progress:** accounts and text posts work end to end: public feeds, author pages, post details, publishing, editing, and deletion. Uploads, likes, comments, and search are still planned.

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

Open [http://localhost:5173](http://localhost:5173), create an account, and choose **New post**. Anyone can browse posts and author pages; only the author can edit or delete a post. **Your account** links to your posts. Sessions survive page reloads.

## Checks

```bash
make check
make test-db
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

`make check` runs Go and HTTP client tests, go vet, frontend lint, and both builds. `make test-db` verifies migrations, authentication, post permissions, pagination, and edit conflicts in temporary PostgreSQL schemas. Run `make fmt` to format Go files. Press Ctrl+C in each terminal to stop the application; use `make stop` to stop supporting services and preserve their data.

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
