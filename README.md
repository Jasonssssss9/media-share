# SocialAI v2

A full-stack social platform being rebuilt with React, TypeScript, Go, PostgreSQL, and Elasticsearch.

The planned application supports authenticated user workflows, posts, media uploads, and searchable content. AI features will be implemented in a later application phase, with local development designed to work without a paid API key.

**Current status:** local development environment and infrastructure only. The frontend, backend, database schema, and AI workflows have not been implemented yet.

## Development stack

| Component | Selected version | Runtime |
| --- | --- | --- |
| Ubuntu | 24.04 LTS | WSL 2 |
| Node.js / npm | 24.22.2 / 11.19.1 | WSL, managed by nvm |
| Go | 1.27.3 | WSL, user-local installation |
| PostgreSQL | 18.6 | Docker Compose |
| Elasticsearch | 9.5.6 | Docker Compose |
| Docker | Docker Desktop with Ubuntu integration | Windows host / WSL CLI |

Node and Go versions are pinned in `.nvmrc` and `.go-version`. Container versions are pinned in `compose.yaml`. Application packages and their lockfiles will be added when application development starts.

## Local infrastructure

Run these commands inside Ubuntu WSL. The working directory for this installation is `/home/jason/projects/socialai-v2`.

Prerequisites: Docker Desktop is running with Ubuntu WSL integration enabled; the pinned Node and Go toolchains are installed. See [environment setup](docs/environment.md) for the full procedure.

```bash
cd /home/jason/projects/socialai-v2
nvm use
go version
```

Create local configuration once. This generates independent random credentials and does not overwrite an existing `.env`:

```bash
python3 - <<'PY'
from pathlib import Path
import os
import secrets

target = Path(".env")
values = {
    "POSTGRES_PASSWORD": secrets.token_hex(24),
    "ELASTIC_PASSWORD": secrets.token_hex(24),
    "JWT_SECRET": secrets.token_hex(48),
}
template = Path(".env.example").read_text()
for key, value in values.items():
    template = template.replace(f"{key}=\n", f"{key}={value}\n")
fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, "w") as stream:
    stream.write(template)
Path("var/media").mkdir(parents=True, exist_ok=True)
PY
```

`.env` has already been created on the configured development machine. Skip the creation step there.

```bash
docker compose config --quiet
docker compose pull --quiet
docker compose up -d --wait --wait-timeout 240
docker compose ps
```

| Service | Local address | Authentication |
| --- | --- | --- |
| PostgreSQL | `127.0.0.1:5432` | `POSTGRES_USER`, `POSTGRES_DB`, `POSTGRES_PASSWORD` in `.env` |
| Elasticsearch | `http://127.0.0.1:9200` | User `elastic`, password `ELASTIC_PASSWORD` in `.env` |

Both ports bind to loopback. Data is stored in project-specific Docker named volumes. Elasticsearch uses password authentication with HTTP for local development. Its local configuration disables mmap, so startup does not require host kernel tuning.

```bash
docker compose stop             # Stop services; preserve containers and data
docker compose up -d --wait     # Start services again
docker compose down            # Remove project containers/network; preserve data volumes
```

Adding `--volumes` to `down` deletes the database and search data. It is not part of normal startup or troubleshooting.

## Documentation

- [Environment setup and daily use](docs/environment.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Environment verification](docs/environment-verification.md)

This rebuild targets local development first. Cloud deployment, application behavior, and AI integration are outside this environment milestone.
