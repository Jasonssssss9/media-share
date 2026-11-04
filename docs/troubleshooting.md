# Troubleshooting

## Node or Go selects an old version

Run `nvm use` in the repository. In a noninteractive shell, load `~/.nvm/nvm.sh` first. Check `command -v node` and `command -v go`; Go should resolve to `~/.local/bin/go`.

## Docker is unavailable in WSL

Start Docker Desktop and enable Ubuntu under Settings > Resources > WSL integration. Verify that `docker version` shows both Client and Server.

This machine has previously failed at startup because of an inaccessible `dockerInference` socket. Recovery preserved `%LOCALAPPDATA%\Docker\run` under a timestamped backup name and recreated the empty runtime directory after Docker had exited. Images and data volumes were preserved. Inspect current logs and process state before repeating that procedure.

## The frontend cannot connect

Start `make services`, then `make dev-api`. Check `curl http://localhost:8080/readyz`. A 503 response means the API is running but cannot reach PostgreSQL. The frontend's Retry button performs a new connection check.

A port conflict is reported rather than silently selecting another frontend port. Stop the process using the port or change `API_PORT` or `WEB_PORT` in `.env`.

## Image generation is unavailable or fails

Set `OPENAI_API_KEY` in the project root's `.env` then restart `make dev-api` and reload the editor. A configured key must have image API access and available credits. Check `OPENAI_IMAGE_MODEL` if the server logs report a provider configuration error.

A 429 response can mean the local cooldown, an active generation, or a provider quota limit. Generation can take a couple of minutes; a 504 response means its 150-second budget expired. Failed requests are not automatically retried. Check the API log's request ID for provider failures. Keep keys and private log details out of screenshots and commits.

## Password changes do not take effect

Database passwords are stored when a data volume is initialized. Changing `.env` does not rotate an existing service password. Update the service password and configuration together.

## GitHub asks for the SSH passphrase

Local commits do not need GitHub authentication. Unlock the existing key in your own WSL terminal before pushing. The repository's local `core.sshCommand` selects the previously configured agent; inspect it with `git config --get core.sshCommand`.
