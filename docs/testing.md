# Testing

Run from the repository root with the selected Go and Node versions:

```bash
make services
make check
make test-db
```

`make check` runs Go tests, HTTP client tests, lint, and both builds. `make test-db` checks migrations and API behavior against real PostgreSQL and Elasticsearch, using temporary schemas, directories, and search indices. It does not change normal application data. Image generation uses a fake provider without paid requests.

To check backend concurrency:

```bash
set -a
source .env
set +a
cd backend
go test -race -tags=integration ./...
```

## Manual integration checks

Start `make dev-api` and `make dev-web` in separate terminals.

| Check | Expected behavior |
| --- | --- |
| Register, sign in, reload, and sign out | The session survives reload; sign-out revokes it |
| Save a display name, bio, and uploaded avatar | Account, profile, post, and comment authors show saved data |
| Upload images, generate an image, reorder, and publish | Published images stay visible in the feed, details, profile, and search |
| Use two accounts for likes and comments | Counts update; only permitted edit and delete controls appear |
| Search, edit, and delete posts | Exact-case author filters work; saved changes reach search |
| Leave while login, saving, or deletion is pending | Completion updates data without redirecting the page left open |
| Check desktop and narrow screens | Navigation remains accessible without horizontal scrolling |

Search unavailability and index rebuilding are covered by integration tests. For manual recovery, stop the API, run `make search-reindex`, restart the API, and retry the search.

## Current limits

This project runs locally with one Go process and local image files. Sessions expire after 24 hours. Search uses page numbers within the first 10,000 matches; missed index updates require a manual rebuild. Draft cleanup is best effort, so an interrupted browser or network request can leave private, unused images.
