# MangaHub

A real-time manga tracking platform built in **Go**, demonstrating all five core network protocols — **HTTP, TCP, UDP, WebSocket, and gRPC** — working together through a single protocol bridge.

> One `PUT /users/progress` call fans out to a TCP broadcast, a UDP push notification, a WebSocket room message, and a gRPC audit log — all in real time.

---

## ✨ Features

- **User accounts** — registration, JWT authentication, profile management
- **Manga catalog** — 100+ seeded titles, full-text search ranked by relevance (SQLite FTS5), genre filtering, pagination
- **Reading progress** — per-user library, chapter tracking, favorites, reading status
- **Ratings & reviews** — 1-10 ratings with auto-computed averages (SQL triggers)
- **Comments** — threaded replies, likes, spoiler flags, soft deletion
- **Custom lists** — private or public named lists of manga, reorderable
- **Leaderboards** — top-rated manga, most active users, weekly trending
- **Real-time sync** — TCP broadcast of progress updates to all connected clients
- **Push notifications** — UDP notifications; new chapters (found on MangaDex or released by an admin) go only to that manga's readers
- **Community chat** — WebSocket rooms per manga with saved history
- **Internal RPC** — gRPC service (with reflection) for search, details, and audit logging
- **CLI & TUI** — Cobra-based CLI plus a Bubble Tea terminal UI

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     CLIENT LAYER                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │  Web Browser │  │  CLI / TUI   │  │ Test Scripts │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
└──────────┬──────────────────────┬──────────────────┬────────────┘
           │                      │                  │
    ┌──────▼──────┐      ┌────────▼────────┐  ┌─────▼──────┐
    │ HTTP REST   │      │  WebSocket      │  │ TCP Client │
    │ :8080       │      │  :8080/ws/chat  │  │ :9090      │
    └──────┬──────┘      └────────┬────────┘  └─────┬──────┘
           │                      │                  │
           └──────────┬───────────┴──────────────────┘
                      │
           ┌──────────▼──────────┐
           │   Protocol Bridge   │  ← one HTTP call triggers all protocols
           └──────────┬──────────┘
                      │
         ┌────────────┼────────────┐
         │            │            │
    ┌────▼────┐  ┌────▼─────┐  ┌──▼────┐
    │ UDP     │  │ gRPC     │  │ TCP   │
    │ :9091   │  │ :9092    │  │ :9090 │
    └────┬────┘  └────┬─────┘  └──┬────┘
         │            │            │
         └────────────┼────────────┘
                      │
           ┌──────────▼──────────┐
           │  SQLite Database    │
           │  (pure-Go driver,   │
           │   WAL, FTS5)        │
           └─────────────────────┘
```

**Tech stack:** Go 1.25 · Gin · gorilla/websocket · gRPC + Protocol Buffers · SQLite (glebarez/go-sqlite, pure Go — no CGO) · Cobra + Viper · Bubble Tea · JWT · bcrypt · logrus · Docker

## 🚀 Quick Start

### Prerequisites

- Go 1.24+ (no C compiler needed — the SQLite driver is pure Go)
- Git

### Installation

```bash
git clone https://github.com/nmihtuna204/Mangahub.git
cd Mangahub
go mod tidy
```

### Start All Services

The database is created, migrated, and seeded automatically on first start (concurrent-start safe).

```bash
# Terminal 1: HTTP API Server (+ WebSocket chat)
go run ./cmd/api-server

# Terminal 2: TCP Sync Server
go run ./cmd/tcp-server

# Terminal 3: UDP Notifier
go run ./cmd/udp-server

# Terminal 4: gRPC Service
go run ./cmd/grpc-server
```

### Build the CLI / TUI

```bash
go build -o bin/mangahub ./cmd/cli
./bin/mangahub --help

go run ./cmd/tui   # terminal UI
```

### Default Accounts

| Username | Password | Role |
|----------|----------|------|
| `admin` | `admin123` | admin |
| `reader1` / `reader2` / `mangafan` | `password123` | user |

## 📋 API Overview

Full reference with request/response examples for every endpoint and protocol: **[docs/API.md](docs/API.md)**.

### Authentication

```http
POST /auth/register        {"username", "email", "password"}
POST /auth/login           {"username", "password"} → JWT token
GET  /auth/me              (Bearer token)
POST /auth/refresh         (Bearer token)
POST /auth/logout          (Bearer token)
```

### Manga & Library

```http
GET  /manga?q=one+piece&limit=10&offset=0     search & list
GET  /manga/:id                               details with genres
GET  /health                                  health check + DB stats

POST   /users/library                          add manga (Bearer)
GET    /users/library                          my library (Bearer)
DELETE /users/library/:manga_id                remove (Bearer)
PUT    /users/progress                         update progress (Bearer) ⭐ triggers all 5 protocols
```

### Social

```http
POST/DELETE /manga/:id/ratings     rate 1-10 with review (Bearer)
GET         /manga/:id/ratings     summary + distribution
POST        /manga/:id/comments    comment / reply (Bearer)
GET         /manga/:id/comments    list comments
POST/DELETE /comments/:id/like     like / unlike (Bearer)
GET  /leaderboards/manga           top rated
GET  /leaderboards/users           most active
GET  /leaderboards/trending        weekly trending
GET  /activities                   recent activity feed
```

### WebSocket Chat

```javascript
// JWT required (header or ?token=)
ws://localhost:8080/ws/chat?room_id=general&token=<JWT>

send:    {"content": "This manga is amazing!"}
receive: {"user_id", "username", "content", "timestamp", "type", "room_id"}
```

### gRPC (with server reflection)

```bash
grpcurl -plaintext localhost:9092 list
grpcurl -plaintext -d '{"query":"naruto","limit":5}' localhost:9092 mangahub.v1.MangaService/SearchManga
grpcurl -plaintext -d '{"manga_id":"<uuid>"}'        localhost:9092 mangahub.v1.MangaService/GetManga
# UpdateProgress needs your JWT (from POST /auth/login) and only changes your own progress
grpcurl -plaintext -H "authorization: Bearer <token>" \
        -d '{"user_id":"reader1","manga_id":"<uuid>","current_chapter":50,"status":"reading"}' \
        localhost:9092 mangahub.v1.MangaService/UpdateProgress
```

## 🔄 Protocol Integration Flow

When a user updates progress via HTTP:

1. **HTTP** — REST API validates and persists the update
2. **Bridge** — protocol bridge is triggered
3. **TCP** — progress broadcast to all connected sync clients
4. **UDP** — chapter notification pushed to registered subscribers
5. **WebSocket** — chat room members notified in real time
6. **gRPC** — audit entry logged via RPC

## 📊 Database

Normalized SQLite schema (21 tables) managed by code-first migrations in [`pkg/database/sqlite.go`](pkg/database/sqlite.go):

- `users`, `manga`, `genres`, `manga_genres`, `manga_external_ids`
- `reading_progress` (unique per user+manga), `manga_ratings`, `comments`, `comment_likes`
- `chat_rooms`, `chat_room_members`, `chat_messages`
- `custom_lists`, `custom_list_items`, `activity_feed`, `seed_meta`
- `manga_fts` — FTS5 full-text index kept in sync by triggers

Highlights:

- **Triggers** keep `manga.average_rating`, comment like counts, and the activity feed up to date automatically
- **Concurrent-start-safe seeding** — a single-row `seed_meta` marker claimed inside a write transaction guarantees exactly one process seeds the DB
- **WAL mode + busy timeout** for safe multi-process access

## 🧪 Testing

```bash
# Everything: unit tests + end-to-end tests (no servers needed; the
# end-to-end suite starts all five protocols in-process on random ports)
go test ./...

make test-unit          # unit tests only (internal/, pkg/)
make test-e2e           # end-to-end suite (test/)
make test-integration   # live tests against the 4 running servers (MANGAHUB_LIVE=1)
make test-coverage      # coverage.html (~71% of backend/protocol code)
```

PowerShell end-to-end scripts (Windows):

```powershell
.\test-all.ps1           # full suite: unit + HTTP + TCP + UDP + gRPC + CLI + integration
.\test-api.ps1           # REST API flow
.\test-curl.ps1          # manual curl walkthrough
.\test-tcp.ps1           # TCP broadcast between two clients
.\test-udp-simple.ps1    # UDP register + notification receive
.\test-websocket.ps1     # two-client WebSocket chat broadcast
.\test-grpc.ps1          # all three RPCs via grpcurl
.\test-integration.ps1   # cross-protocol bridge verification
bash test/load_test.sh   # load test (bash + curl; grpcurl optional)
```

Each script checks its results and exits 1 if any check fails.

CI (`.github/workflows/ci.yml`) runs gofmt, vet, build and the whole suite with the race detector on every push.

Unit tests run against the **real production schema** — the test databases are created by the same `Migrate()` the servers use, so schema drift is caught at test time.

## 🐳 Docker

```bash
docker compose up --build
```

See [DOCKER.md](DOCKER.md) for details.

## 📚 Documentation

- [docs/API.md](docs/API.md) — API reference (REST, WebSocket, TCP, UDP, gRPC)
- [HOW_TO_RUN.md](HOW_TO_RUN.md) — step-by-step run guide
- [TEST.md](TEST.md) — full manual test catalog
- [CLI_README.md](CLI_README.md) — CLI usage
- [docs/KNOWN_ISSUES.md](docs/KNOWN_ISSUES.md) — fixed issues, open issues, test coverage and measured performance
- [docs/](docs/) — design notes and phase summaries

## 👨‍💻 Authors

- [nmihtuna204](https://github.com/nmihtuna204)
- [duythucne22](https://github.com/duythucne22)

## 📝 License

MIT
