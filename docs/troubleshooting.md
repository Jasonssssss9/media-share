# Troubleshooting

## Wrong Node or Go version

Run `nvm use` from the repository root and compare `node --version` and `go version` with the [environment notes](environment.md). Load `~/.nvm/nvm.sh` first when using nvm in a noninteractive shell.

## Docker or the API is unavailable

Start Docker Desktop, enable Ubuntu's WSL integration, and run `make services`. Check `docker version` and `docker compose ps`.

Start `make dev-api` and check `curl http://localhost:8080/readyz`. A 503 response means PostgreSQL is unavailable; check the container and database settings. For a port conflict, stop the process using the port or change the corresponding port in `.env`, then restart the affected service.

## Search is unavailable or outdated

Check that Elasticsearch is healthy with `docker compose ps`. Verify `ELASTIC_PASSWORD`, `ELASTICSEARCH_PORT`, and `ELASTICSEARCH_INDEX` in `.env`. Other features can run while search is unavailable.

To repair missing updates, stop the Go API, keep both Docker services running, and run:

```bash
make search-reindex
```

Restart the API and retry the search. This replaces only the configured index using current PostgreSQL posts. Rerun it if rebuilding was interrupted. Author filters must match the username's exact letter case.

## Image generation is unavailable or fails

Set `OPENAI_API_KEY` in `.env`, restart the API, and reload the editor. The key needs image API access and credits; `OPENAI_IMAGE_MODEL` must be available to that account.

A 429 response can mean an active generation, the local cooldown, or provider limits. A 504 means the 150-second request budget expired. Failed requests are not automatically retried. Use the API log and request ID to investigate failures; keep keys and private log details out of commits.

## A service rejects a changed password

Service passwords are stored when their data volumes are initialized. Editing `.env` alone does not change an existing password. Update the stored service password and configuration together; keep the existing data volumes.
