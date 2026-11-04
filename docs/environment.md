# Environment notes

Work in Ubuntu WSL at `/home/jason/projects/media-share`.

| Component | Version |
| --- | --- |
| Node.js / npm | 24.22.2 / 11.19.1 |
| Go | 1.27.3 |
| PostgreSQL | 18.6 |
| Elasticsearch | 9.5.6 |

Run `nvm use` in the repository. Go is selected through `~/.local/bin/go`; its required version is declared in `backend/go.mod`. Existing Windows and WSL toolchains were preserved.

Docker Desktop must be running with Ubuntu enabled under Settings > Resources > WSL integration. No separate Docker daemon or native database installation is needed in Ubuntu.

## Configuration

Copy `.env.example` to `.env` on a new checkout, then set both database passwords and a random `JWT_SECRET` of at least 32 characters. Keep `.env` private and out of Git. The configured machine already has generated passwords.

`make dev-api` and `make dev-web` export `.env` before starting their processes. Run them from the project root.

| Service | Address |
| --- | --- |
| Frontend | http://localhost:5173 |
| Go API | http://localhost:8080 |
| PostgreSQL | 127.0.0.1:5432 |
| Elasticsearch | http://127.0.0.1:9200 |

Ports can be changed in `.env`. The Vite proxy reads `API_PORT`, so the frontend does not need a separate API URL. `APP_ORIGIN` identifies the trusted frontend origin. Leave it empty to follow `WEB_PORT`, or set it explicitly when the browser uses a different origin.

## Data

`MEDIA_DIR` defaults to `../var/media`, relative to the backend working directory. `make dev-api` runs there, so files live under the project's `var/media`. Keep that directory together with PostgreSQL data when backing up the project.

Compose uses `media-share_postgres-data` and `media-share_elasticsearch-data`. PostgreSQL 18 mounts its volume at `/var/lib/postgresql`. Elasticsearch disables mmap for local development.

The renamed project has its own volumes. Containers and volumes from the old project name were preserved; its old containers are stopped to release the same ports.

Use `make stop` for normal shutdown. `docker compose down` also keeps named volumes; adding `--volumes` deletes stored data.

## Local files

`.env`, `AGENTS.md`, `var/`, `web/node_modules/`, and `web/dist/` are not committed. Local diagnostics are kept under `var/reports`.

`.nvmrc` selects Node.js for `nvm use`. Commit dependency manifests and lockfiles (`go.mod`, `go.sum`, `package.json`, and `package-lock.json`) so installs remain reproducible.
