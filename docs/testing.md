# Testing

Run from the repository root:

```bash
make services
make check
make test-db
```

`make check` runs Go and HTTP client tests, go vet, frontend lint, and both builds. `make test-db` covers migrations, accounts, profiles, images, post permissions, interactions, pagination, concurrent changes, search synchronization, and rebuilding.

Integration tests use real PostgreSQL and Elasticsearch with temporary schemas, image directories, and search indices. They clean up their fixtures without changing normal application data. Generation tests use a fake provider and make no paid requests.

For backend concurrency checks:

```bash
set -a
source .env
set +a
(cd backend && go test -race -tags=integration ./...)
```

## Manual checks

Start the API and frontend as described in the [README](../README.md#run-locally).

| Check | Expected result |
| --- | --- |
| Register, sign in, reload, and sign out | The session survives reload; sign-out revokes it |
| Save a profile and avatar | Account, profile, post, and comment authors show saved data |
| Upload and generate images; publish, edit, and delete posts | Text and ordered images stay consistent across the feed, details, profile, and search |
| Use two accounts for likes and comments | Counts update and ownership rules apply |
| Search by text and author | `Alice` and `alice` are distinct filters |
| Leave during login, saving, or deletion | Completion updates data without redirecting the current page |
| Use desktop and narrow screens | Navigation stays accessible without horizontal scrolling |

With the API running, check `curl http://localhost:8080/healthz` and `curl http://localhost:8080/readyz`. Search recovery steps are in [troubleshooting](troubleshooting.md#search-is-unavailable-or-outdated); current limits are in [architecture](design.md#scope-and-limits).
