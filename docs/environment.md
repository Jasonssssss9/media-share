# Environment notes

Work in Ubuntu WSL at `/home/jason/projects/media-share`.

| Component | Version |
| --- | --- |
| Node.js / npm | 24.22.2 / 11.19.1 |
| Go | 1.27.3 |
| PostgreSQL | 18.6 |
| Elasticsearch | 9.5.6 |

Run `nvm use` in the repository. Go is selected through `~/.local/bin/go`. Existing Windows and WSL toolchains were preserved.

Docker Desktop must be running with Ubuntu enabled under Settings > Resources > WSL integration. No separate Docker daemon or native database installation is needed in Ubuntu.

## Configuration

Copy `.env.example` to `.env` on a new checkout, then set both database passwords. Keep `.env` private and out of Git. The configured machine already has generated passwords.

`make dev-api` and `make dev-web` export `.env` before starting their processes. Run them from the project root.

| Service | Address |
| --- | --- |
| Frontend | http://localhost:5173 |
| Go API | http://localhost:8080 |
| PostgreSQL | 127.0.0.1:5432 |
| Elasticsearch | http://127.0.0.1:9200 |

Ports can be changed in `.env`. The Vite proxy reads `API_PORT`, so the frontend does not need a separate API URL.

## Data

Compose uses `media-share_postgres-data` and `media-share_elasticsearch-data`. PostgreSQL 18 mounts its volume at `/var/lib/postgresql`. Elasticsearch disables mmap for local development.

The renamed project has its own volumes. Containers and volumes from the old project name were preserved; its old containers are stopped to release the same ports.

Use `make stop` for normal shutdown. `docker compose down` also keeps named volumes; adding `--volumes` deletes stored data.
