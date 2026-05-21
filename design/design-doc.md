# In-Memory DB — Design

![In-Memory DB architecture](./in-memory-db.png)

Go learning project: custom in-memory key-value database and a driver web application. Linux only.

---

## System

Four components and one shared utility, as in the diagram:

| Component | Description |
|-----------|-------------|
| **Frontend UI** | HTML served by the Backend API; HTMX for partial updates; Pico.css for styling |
| **Backend API** | Go HTTP server in this repo (`cmd/api`) |
| **Postgres DB** | Persistent relational database; local instance via Docker Compose |
| **In Memory DB** | Standalone TCP service (`cmd/in-memory-db`) |
| **Custom Logger** | Shared logging package (`internal/logger`) used by Backend API and In Memory DB |

**Driver web application:** Frontend and Backend API are one integrated app in this repository.

### External interfaces

| From | To | Message |
|------|-----|---------|
| Frontend UI | Backend API | Request |
| Backend API | Frontend UI | Response |
| Backend API | Postgres DB | Query |
| Postgres DB | Backend API | Data |
| Backend API | In Memory DB | Request |
| In Memory DB | Backend API | Response |

---

## In Memory DB

Separate process. Entry point: `cmd/in-memory-db`. Default listen: `localhost:55555`.

Internal pipeline (diagram):

| Layer | Package | Role |
|-------|---------|------|
| TCP Connection Manager | `internal/transport/tcp` | Accept connections; read/write lines; return responses |
| Command Parser | `internal/parser` | Parse text commands |
| KV Engine | `internal/kv/engine` | Execute commands |
| KV Store | `internal/kv/store` | In-memory map; optional TTL; lazy expiry on read |

Flow: Request → TCP Connection Manager → Command Parser → KV Engine ↔ KV Store → KV Engine → TCP Connection Manager → Response.

Time source for TTL: `internal/timeprovider`.

### TCP protocol

One command per line (`\n`-terminated). One response per line.

**Commands**

| Command | Syntax |
|---------|--------|
| SET | `SET "key" VALUE "value"` |
| SET (TTL) | `SET "key" VALUE "value" TTL "seconds"` |
| GET | `GET "key"` |
| DELETE | `DELETE "key"` |
| CLEAR | `CLEAR` |

Quoted strings; escapes `\"` and `\\`. Keywords (`SET`, `VALUE`, `TTL`, …) are case-insensitive. Keys and values must be quoted.

**Responses**

| Outcome | Format |
|---------|--------|
| Success | `SUCCESS: <payload>` |
| Error | `ERROR: <message>` |

---

## Backend API

Entry point: `cmd/api`.

| Package | Role |
|---------|------|
| `internal/api/server` | HTTP routes; render templates |
| `internal/api/db` | Postgres access |
| `internal/api/kvclient` | TCP client to In Memory DB |

Serves HTML templates and static assets (Pico.css). HTMX drives dynamic fragments without a separate frontend build.

Postgres holds application data the UI needs to persist. KV operations go to the In Memory DB over TCP; the API does not embed the KV engine.

---

## Local runtime

1. `docker compose up` — Postgres  
2. `go run ./cmd/in-memory-db` — In Memory DB  
3. `go run ./cmd/api` — Driver web app  

Module: `github.com/inv-hemanthb/in-memory-db` (Go 1.23).
