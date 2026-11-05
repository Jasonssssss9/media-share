# Environment and configuration

Run Go, Node.js, and project commands in Ubuntu WSL. Docker Desktop must be running with Ubuntu enabled under **Settings > Resources > WSL integration**. PostgreSQL and Elasticsearch run in containers; they do not need native WSL installations.

## Versions

| Component | Version | Declared in |
| --- | --- | --- |
| Node.js | 24.22.2 | `.nvmrc` |
| Go | 1.27.3 | `backend/go.mod` |
| PostgreSQL | 18.6 | `compose.yaml` |
| Elasticsearch | 9.5.6 | `compose.yaml` |

Run `nvm install` once to install the selected Node version, then `nvm use` from the repository root. Frontend dependencies are pinned in `web/package-lock.json`; Go dependencies use `go.mod` and `go.sum`.

## Configuration

On a fresh checkout, copy [.env.example](../.env.example) to `.env`. Set both service passwords and a random `JWT_SECRET` of at least 32 characters; `openssl rand -hex 32` can generate one. Never commit `.env`.

| Setting | Default or purpose |
| --- | --- |
| `POSTGRES_HOST`, `POSTGRES_PORT` | `127.0.0.1`, `5432` |
| `POSTGRES_USER`, `POSTGRES_DB` | `mediashare`, `mediashare` |
| `POSTGRES_PASSWORD`, `ELASTIC_PASSWORD` | Required service passwords |
| `ELASTICSEARCH_PORT`, `ELASTICSEARCH_INDEX` | `9200`, `media-share-posts` |
| `API_PORT`, `WEB_PORT` | `8080`, `5173` |
| `JWT_SECRET` | Session signing key |
| `APP_ORIGIN` | Defaults to `http://localhost:<WEB_PORT>` |
| `MEDIA_DIR` | `../var/media`, relative to the backend working directory |
| `OPENAI_API_KEY` | Optional; enables image generation |
| `OPENAI_IMAGE_MODEL` | Uses the default in `.env.example`; must be available to your API account |

The Make commands load `.env` when starting the app or running database commands. The frontend uses Vite's API proxy and needs no separate API URL. `make dev-web` removes service passwords, the signing key, and the API key from the frontend process environment.

## Default addresses

| Service | Address |
| --- | --- |
| Frontend | http://localhost:5173 |
| Go API | http://localhost:8080 |
| PostgreSQL | 127.0.0.1:5432 |
| Elasticsearch | http://127.0.0.1:9200 |

Service ports bind to loopback. Elasticsearch uses the `elastic` account and `ELASTIC_PASSWORD`. Search availability does not affect API startup or PostgreSQL readiness checks. Restart the app after changing configuration.

## Persistent data

Compose stores database data in `media-share_postgres-data` and `media-share_elasticsearch-data`. Image files live in `var/media` when the API runs through Make. Keep the PostgreSQL data and image directory together when backing up the project; the search index can be rebuilt.

`make stop` and `docker compose down` preserve named volumes. Adding `--volumes` to `docker compose down` deletes their stored data.

Local configuration, uploads, reports, dependencies, and builds stay out of Git. See [.gitignore](../.gitignore) for project exclusions.
