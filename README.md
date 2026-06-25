# in-memory-db

A Go learning project: an in-memory TCP key-value store with TTL, plus a Postgres CRUD web test bed that uses it as a cache-aside layer. Compare read latency and cache behavior (PG-only vs KV) from the UI.

Linux only — tested on Linux, not on Windows.

Architecture and design details: [design/design-doc.md](design/design-doc.md).

## Prerequisites

| Requirement | Version / note |
|-------------|----------------|
| Go | 1.23+ |
| Docker + Compose | Postgres only |
| OS | Linux |

## Configuration

```bash
cp .env.example .env
```

Edit `POSTGRES_PASSWORD` in `.env` (must match the password in `DATABASE_URL`).

| Variable | Purpose | Default |
|----------|---------|---------|
| `DATABASE_URL` | Postgres connection string | `postgresql://postgres:…@localhost:5434/in-memory-db` |
| `KV_ADDR` | KV TCP address | `localhost:55555` |
| `API_PORT` | HTTP server port | `8080` |
| `POSTGRES_HOST_PORT` | Host port mapped by Docker Compose | `5434` |

All `go run` commands load `.env` from the repo root. Run commands from the **repo root** (or any subdirectory under it).

## Database setup

```bash
docker compose up -d
go run ./cmd/migrate
go run ./cmd/seed          # optional, ~500 rows for read benchmarks
```

`migrate` applies SQL files in `migrations/`. `seed` is optional.

## Run the application

Three processes — use three terminals:

```bash
# terminal 1 — KV server
go run ./cmd/in-memory-db

# terminal 2 — API + UI
go run ./cmd/api

# browser
http://localhost:8080
```

Shutdown with Ctrl+C in each terminal (API and KV handle SIGTERM).

## Using the test bed

- **Use KV cache** toggle — compare Postgres-only vs cache-aside on the same handlers
- **Read** row — set `count` (e.g. 1000) for batch reads; provide `id` and/or `key`
- **Metrics** panel at the bottom — latency, hits/misses, ok/fail per run

## Tests

```bash
go test ./...
```

Integration tests in `internal/api/kvclient` and `internal/api/db` need Postgres and/or the KV server running; they skip if unreachable.

## Project layout

```
cmd/in-memory-db   KV TCP server
cmd/api            HTTP API + UI
cmd/migrate        DB migrations
cmd/seed           Sample data
web/               HTMX templates + static assets
migrations/        SQL schema
```
