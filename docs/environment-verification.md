# Environment verification

Verified on November 2, 2026, at 21:57:19 PST. Some command logs use UTC and therefore show the following calendar date.

Working directory: `/home/jason/projects/socialai-v2`.

## Toolchain and connectivity

| Check | Observed result |
| --- | --- |
| WSL | Ubuntu 24.04.4 LTS, WSL 2, Linux amd64 |
| Node.js | 24.22.2; new interactive shells and `nvm use` select the WSL nvm installation |
| npm | 11.19.1; `npm ping` succeeded |
| nvm | 0.40.7; the existing installation was reused and older Node.js versions were preserved |
| Go | 1.27.3 linux/amd64; `~/.local/bin/go` selects the new SDK |
| Go SDK execution | `go test net/url -run '^TestParse$' -count=1` passed |
| Go module connectivity | `go mod download -json github.com/gorilla/mux@v1.8.1` completed with checksum verification |
| Git | 2.43.0; the repository's existing identity settings were preserved |
| GitHub | `git ls-remote origin HEAD` succeeded after the owner unlocked the existing key |
| Docker Desktop | Existing version 4.43.1; Ubuntu integration is enabled |
| Docker Engine | 28.3.0, Linux, WSL 2 backend |
| Compose | 2.38.1-desktop.1 |
| README configuration command | Executed in a temporary directory; generated independent random credentials with mode 0600 and refused to overwrite an existing .env |

The Go module download was a connectivity and checksum verification probe. It does not establish the application's final dependencies. No application `go.mod` or `go.sum` was created.

## Supporting services

| Check | PostgreSQL | Elasticsearch |
| --- | --- | --- |
| Server version | 18.6 (Debian 18.6-1.pgdg13+2) | 9.5.6 |
| Final container state | running / healthy | running / healthy |
| Published port reachable from WSL | 127.0.0.1:5432 | 127.0.0.1:9200 |
| Valid credentials | psql authenticated over TCP inside the container | HTTP Basic Auth succeeded from WSL |
| Invalid or missing credentials | An intentionally incorrect password was rejected | An unauthenticated request returned HTTP 401 |
| Read/write operations | A dedicated probe table accepted and returned a marker | A dedicated index accepted a document and returned a matching search result |
| Persistence after recreation | The container ID changed and the original marker remained | The container ID changed and the original document remained |
| Final configuration check | Authentication succeeded after Docker Desktop recovery | Search and persistence checks passed again with mmap disabled |
| Probe cleanup | The probe table was removed | Probe indices from both verification rounds were removed |

Final Elasticsearch cluster health: `green`.

The node settings API confirmed that `node.store.allow_mmap=false` was active. The container reported `vm.max_map_count=262144` after Docker Desktop restarted. The local configuration does not require increasing that value. See the [environment guide](environment.md) for the mmap tradeoff.

## Storage and resource limits

| Service | Named volume | Mount path | Memory limit | CPU limit |
| --- | --- | --- | --- | --- |
| PostgreSQL | socialai-v2_postgres-data | /var/lib/postgresql | 768 MiB | 1 |
| Elasticsearch | socialai-v2_elasticsearch-data | /usr/share/elasticsearch/data | 2 GiB | 2 |

PostgreSQL's actual PGDATA was `/var/lib/postgresql/18/docker`, matching the PostgreSQL 18 image layout.

Runtime container configuration confirmed that both published ports bind to `127.0.0.1`. The Elasticsearch JVM heap is 512 MiB.

## Image provenance

Compose pins exact version tags. These are the repository digests returned for the images downloaded during verification:

- PostgreSQL: `postgres@sha256:3a4264520e121176619f54ff867d12774bc81e95e7b84db58d63bccdb6144b81`
- Elasticsearch: `docker.elastic.co/elasticsearch/elasticsearch@sha256:79b4f1de4c15e6eca824723086a51e5934406d614d410cc11be77748cafac537`

## Local configuration protection

- The local `.env` contains independent random database, search, and signing secrets.
- Its mode is `0600`, and `git check-ignore .env` passed.
- Only the credential-free `.env.example` is included in version control.
- `var/`, `.env-reports/`, build output, and logs are ignored by Git.
- Machine-local evidence is stored in `.env-reports/infrastructure-verification.json` and is excluded from version control.
- No paid provider credentials, GCP service account, or GCS bucket was used.

## Host limitations and verification scope

The full Docker Desktop restart check reproduced a Windows AF_UNIX socket startup failure. After process inspection, the exact temporary runtime directory was preserved and recreated. Docker recovered, and the services passed their final checks. The underlying host problem has not been proven permanently resolved. See [stale runtime sockets](troubleshooting.md#stale-sockets-in-dockers-runtime-directory) for the recovery record.

Restarting Docker Desktop also overrode the active mmap kernel setting. The final local configuration removes that dependency, and the experimental sysctl changes were rolled back.

The SSH agent was unlocked and verified. A full WSL restart requires the owner to unlock the existing key again; see [SSH authentication](troubleshooting.md#github-ssh-authentication-fails).

Application behavior has not been tested. There is no frontend development server, Go API, database migration, login workflow, upload endpoint, or provider integration yet. Cloud deployment and performance testing are outside this milestone. Ports 8080 and 5173 are reserved for future application services.

## Routine checks

```bash
cd /home/jason/projects/socialai-v2
nvm use
node --version
npm --version
go version
docker version
docker compose config --quiet
docker compose up -d --wait --wait-timeout 240
docker compose ps --format '{{.Name}} {{.Health}}'
git check-ignore .env
git ls-remote origin HEAD
```

Commits are kept locally. The repository owner is responsible for pushing them.
