# Local development environment

This guide documents the local environment for SocialAI v2. Keep the repository in the WSL Linux filesystem. The configured working directory is:

```text
/home/jason/projects/socialai-v2
```

This setup covers the toolchain, supporting services, local configuration, and repository conventions. Application code and cloud deployment are outside this milestone.

## How the services run

The frontend development server and Go backend will run in Ubuntu WSL. PostgreSQL and Elasticsearch run through Docker Compose on Docker Desktop's Linux engine. The current Compose file contains only these two supporting services.

| Component | Version | Installation or runtime |
| --- | --- | --- |
| Ubuntu | 24.04.4 LTS, WSL 2 | Existing Ubuntu distribution |
| Git | 2.43.0 | Existing Ubuntu package |
| nvm | 0.40.7 | Existing installation at `~/.nvm` |
| Node.js | 24.22.2 | nvm; pinned in `.nvmrc` |
| npm | 11.19.1 | Included with this Node.js installation |
| Go | 1.27.3 | `~/.local/share/go/go1.27.3` |
| Docker Desktop | Existing installation with Ubuntu integration enabled | Windows |
| Docker Engine | 28.3.0 | Docker Desktop Linux engine |
| Docker Compose | 2.38.1-desktop.1 | Provided by Docker Desktop |
| PostgreSQL | 18.6 | `postgres:18.6` |
| Elasticsearch | 9.5.6 | `docker.elastic.co/elasticsearch/elasticsearch:9.5.6` |

Python 3, curl, CA certificates, make, a compiler toolchain, and unzip were already available in Ubuntu. Python is used for setup and verification.

PostgreSQL and Elasticsearch do not need separate native installations. The Elasticsearch image includes Java. Local media will use `var/media`. Redis, Kibana, MinIO, the GCP SDK, cloud credentials, and a paid API key are not required for this environment.

React, TypeScript, Vite, MUI, TanStack Query, and Go application libraries will be added during application development. Their versions will be recorded in project manifests and lockfiles rather than installed globally.

## 1. WSL and basic tools

Use the existing WSL 2 Ubuntu distribution. On a new machine, follow Microsoft's WSL installation guide, then check the following tools inside Ubuntu:

```bash
cat /etc/os-release
git --version
curl --version
make --version
gcc --version
python3 --version
```

Install any missing base packages:

```bash
sudo apt-get update
sudo apt-get install -y build-essential ca-certificates curl git make unzip python3
```

These packages were already installed on the configured machine. No sudo permissions were changed, and no separate Docker daemon was installed in Ubuntu.

Keep the repository under `/home/...` for dependency installation and file watching. Use the existing editor setup to work with the WSL checkout.

## 2. Docker Desktop integration

1. Start Docker Desktop on Windows and wait for the engine to run.
2. In Settings > General, confirm that Docker uses the WSL 2 engine.
3. In Settings > Resources > WSL integration, enable Ubuntu.
4. Select Apply & restart, then verify the connection from WSL.

```bash
docker version
docker compose version
docker info --format '{{.OSType}}'
```

The last command should print `linux`. Both the client and server should appear in `docker version`. The WSL CLI connects to Docker Desktop's engine; Ubuntu does not need its own dockerd service.

Ubuntu integration was disabled when this machine was first checked. It has been enabled and verified.

## 3. Node.js with nvm

The machine already had nvm 0.40.7 and Node.js 24.20.0. Ubuntu also had a system installation of Node.js 18.19.1, and Windows had its own Node.js installation. These installations remain available.

With nvm installed, run:

```bash
export NVM_DIR="$HOME/.nvm"
. "$NVM_DIR/nvm.sh"
nvm install 24.22.2
nvm alias default 24.22.2
cd /home/jason/projects/socialai-v2
nvm use
node --version
npm --version
command -v node
npm ping
```

The project should select `v24.22.2`. The executable should be under `~/.nvm/versions/node/v24.22.2/bin/`. The existing `~/.bashrc` already loads nvm.

On a new machine, install nvm using the [official instructions](https://github.com/nvm-sh/nvm#installing-and-updating) before running these commands.

The default version applies to new interactive WSL shells. Run `nvm use` in the repository to select the version in `.nvmrc`. Keep Windows and WSL dependencies separate. Do not share `node_modules` across them or use `sudo npm install -g` for project packages.

## 4. Go in the user directory

The existing Go 1.25.5 installation at `/usr/local/go` remains available. The project uses a separate installation:

```text
~/.local/share/go/go1.27.3
~/.local/bin/go       -> the new SDK's bin/go
~/.local/bin/gofmt    -> the new SDK's bin/gofmt
```

The archive was downloaded from the official Go distribution site and checked against this SHA-256 checksum:

```text
go1.27.3.linux-amd64.tar.gz
65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40
```

To reproduce the installation on Linux amd64, first check that the target directory and links will not overwrite another installation. Then run:

```bash
curl --fail --location --output /tmp/go1.27.3.linux-amd64.tar.gz \
  https://go.dev/dl/go1.27.3.linux-amd64.tar.gz
printf '%s  %s\n' \
  '65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40' \
  /tmp/go1.27.3.linux-amd64.tar.gz | sha256sum --check
mkdir -p "$HOME/.local/share/go/go1.27.3" "$HOME/.local/bin"
tar -xzf /tmp/go1.27.3.linux-amd64.tar.gz \
  -C "$HOME/.local/share/go/go1.27.3" --strip-components=1
ln -sfn "$HOME/.local/share/go/go1.27.3/bin/go" "$HOME/.local/bin/go"
ln -sfn "$HOME/.local/share/go/go1.27.3/bin/gofmt" "$HOME/.local/bin/gofmt"
export PATH="$HOME/.local/bin:$PATH"
go version
```

The existing `.profile` already adds `~/.local/bin` to PATH. A marked block was also added to `.bashrc` so interactive shells select the user installation. The previous file was backed up under `~/.local/state/socialai-v2-env/backups/`.

`.go-version` records the expected version; it does not switch SDKs on its own. PATH and the user directory links select this installation. A global GOROOT override is not required.

## 5. Local configuration and credentials

Use the [one-time command in the README](../README.md#local-infrastructure) to create `.env` from `.env.example`. The configured machine already has this file.

| Variable | Purpose |
| --- | --- |
| `POSTGRES_USER` / `POSTGRES_DB` | Database user and database name; both default to socialai |
| `POSTGRES_PASSWORD` | Random database password |
| `POSTGRES_PORT` | Host port; defaults to 5432 |
| `ELASTIC_PASSWORD` | Random password for the elastic user |
| `ELASTICSEARCH_PORT` | Host port; defaults to 9200 |
| `COMPOSE_PROJECT_NAME` | Resource prefix; use socialai-v2 |

An independent random `JWT_SECRET` has been generated for the future backend. The application ports, storage settings, provider settings, and cloud fields are reserved configuration. They do not indicate that those application features have been implemented.

The local `.env` has mode `0600` and is ignored by Git. Commit only the credential-free `.env.example`. Use `docker compose config --quiet` to validate syntax; the full expanded configuration includes passwords.

Once data volumes have been initialized, changing a password in `.env` does not change the password stored by the service. Use the service's password rotation procedure and update the local configuration together.

## 6. Start and connect

```bash
cd /home/jason/projects/socialai-v2
docker compose config --quiet
docker compose pull --quiet
docker compose up -d --wait --wait-timeout 240
docker compose ps
```

Both services should report `healthy`. See the verification record for the authentication, read/write, and persistence checks.

| Service | WSL address | Named volume | Container limit |
| --- | --- | --- | --- |
| PostgreSQL | `127.0.0.1:5432` | `socialai-v2_postgres-data` | 768 MiB / 1 CPU |
| Elasticsearch | `http://127.0.0.1:9200` | `socialai-v2_elasticsearch-data` | 2 GiB / 2 CPUs |

Both published ports bind to loopback. Elasticsearch uses a single node with password authentication and HTTP for local development. Its transport port is not published.

For PostgreSQL 18, the official image's named volume is mounted at `/var/lib/postgresql`. The actual PGDATA is `/var/lib/postgresql/18/docker`. This layout is included in the persistence checks.

Elasticsearch uses a 512 MiB JVM heap and a 2 GiB container memory limit. The two containers have a combined limit of about 2.75 GiB; Docker Desktop and WSL need additional memory. This machine has about 11 GiB available to WSL. Its Windows and WSL memory allocation settings were not changed.

Local Elasticsearch sets `node.store.allow_mmap: false`. This avoids a startup dependency on changing `vm.max_map_count`. It follows Elastic's quickstart approach and has performance implications. Throughput and latency have not been benchmarked. Production workloads need a separate review of mmap, memory, and kernel settings.

On this machine, restarting Docker Desktop restored the active kernel value to `262144`, even with `1048576` saved in the Docker distribution's `/etc/sysctl.conf`. The final local configuration therefore disables mmap. The experimental sysctl changes were rolled back, and the original file was retained as a backup. The global Windows `.wslconfig` was not changed.

Use the client included in the database image to open a local session:

```bash
docker compose exec postgres sh -c 'exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

The local media directory is `var/media` and is ignored by Git. Upload endpoints will be added during application development.

## 7. Daily commands

```bash
docker compose ps
docker compose logs --tail=80 postgres elasticsearch
docker compose stop
docker compose up -d --wait
docker compose down
```

`stop` preserves containers, networks, and data. `down` removes the project's containers and network but keeps named volumes. Adding `--volumes` deletes the stored data. Avoid using that option or a global Docker prune as a routine troubleshooting step.

Other Docker projects, the existing MySQL service, Windows Node.js, and older Go installations are outside this project's management scope.

## 8. Repository workflow

Write documentation and comments in natural English. Create local commits using the repository owner's configured identity. The owner handles pushes; remote refs are changed only when explicitly requested.

## Official references

- [Microsoft: Install WSL](https://learn.microsoft.com/windows/wsl/install)
- [Docker Desktop: WSL integration](https://docs.docker.com/desktop/features/wsl/)
- [Docker: WSL filesystem guidance](https://docs.docker.com/desktop/features/wsl/best-practices/)
- [nvm installation and usage](https://github.com/nvm-sh/nvm)
- [Node.js distributions](https://nodejs.org/dist/)
- [Go downloads](https://go.dev/dl/)
- [PostgreSQL container documentation](https://github.com/docker-library/docs/blob/master/postgres/README.md)
- [Elasticsearch Docker installation](https://www.elastic.co/docs/deploy-manage/deploy/self-managed/install-elasticsearch-docker-basic)
- [Elasticsearch Docker system settings](https://www.elastic.co/docs/deploy-manage/deploy/self-managed/install-elasticsearch-docker-prod)
- [Elasticsearch quickstart mmap tradeoffs](https://www.elastic.co/docs/deploy-manage/deploy/cloud-on-k8s/virtual-memory)
