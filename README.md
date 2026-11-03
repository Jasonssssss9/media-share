# MediaShare

A student project for sharing images and posts, with a React frontend, a Go backend, and searchable content.

**Current progress:** the application shell is running. The frontend calls the Go API, and the API checks a real PostgreSQL connection. Accounts, posts, uploads, and search are specified but not implemented yet.

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

For a fresh checkout, copy `.env.example` to `.env` and set `POSTGRES_PASSWORD` and `ELASTIC_PASSWORD`. The configured development machine already has this file.

```bash
make install
make services
```

Run the API and frontend in separate terminals from the project root:

```bash
make dev-api
```

```bash
nvm use
make dev-web
```

Open [http://localhost:5173](http://localhost:5173). A **Connected** badge confirms that the frontend reached the Go API and PostgreSQL.

## Checks

```bash
make check
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

`make check` runs Go tests, go vet, frontend lint, and both builds. Run `make fmt` to format Go files. Press Ctrl+C in each terminal to stop the application; use `make stop` to stop supporting services and preserve their data.

## Documentation

- [Design and screen sketches](docs/design.md)
- [API contract](docs/api.md)
- [Environment notes](docs/environment.md)
- [Troubleshooting](docs/troubleshooting.md)
