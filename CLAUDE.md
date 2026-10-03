# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

MangaHub — a Go manga-tracking platform demonstrating all five core network protocols (HTTP, TCP, UDP, WebSocket, gRPC) working together through a single "protocol bridge". Module: `mangahub`.

## Build / run / test commands

Go 1.24+. The SQLite driver (`glebarez/go-sqlite`, driver name `"sqlite"`) is pure Go, so builds normally don't need cgo — but see the CGO note below for this machine.

```bash
go mod tidy

# Run each server in its own terminal (DB auto-migrates + seeds on first start)
go run ./cmd/api-server     # HTTP REST + WebSocket chat, :8080
go run ./cmd/tcp-server     # :9090
go run ./cmd/udp-server     # :9091  (add -demo for a sample notification every 10s)
go run ./cmd/grpc-server    # :9092

# CLI / TUI
go build -o bin/mangahub ./cmd/cli
go run ./cmd/tui

# Tests (no servers needed)
go test ./...                       # unit tests + in-process end-to-end suite
go test ./internal/auth/... -run TestName   # single test
go test -race ./...                 # what CI runs (needs cgo: see the CGO note below)
make test-coverage                  # -coverpkg across packages -> coverage.html
MANGAHUB_LIVE=1 go test ./test/     # also run live tests against the 4 running servers

# Lint
go fmt ./... && go vet ./...
```

A `Makefile` wraps the common ones (`make build-cli`, `make test`, `make test-unit`, `make test-e2e`, `make test-integration`, `make test-coverage`, `make lint`). CI (`.github/workflows/ci.yml`) runs gofmt, vet, build and `go test -race` with coverage. On Windows, PowerShell E2E scripts exist at the repo root (`test-all.ps1`, `test-api.ps1`, `test-tcp.ps1`, `test-udp-simple.ps1`, `test-websocket.ps1`, `test-grpc.ps1`, `test-integration.ps1`) — these also require the servers running first.

Unit tests run against the real production schema: `internal/testutil` creates temp-file SQLite DBs with the same `Migrate()` the servers use (`NewDB`), optionally seeded (`NewSeededDB`), so schema drift shows up at test time. Use file-backed DBs, not `:memory:` (each pooled connection would get its own empty database).

The end-to-end suite (`test/stack_test.go`) starts the seeded DB, TCP, UDP and gRPC servers (via their `Serve(listener)` methods on random ports) and the full API from `internal/server` in-process. New HTTP routes belong in `internal/server/server.go` so they're covered there. `test/live_test.go` holds the old tests against real server processes and is skipped unless `MANGAHUB_LIVE=1`.

### CGO / local environment note (this machine only)
This repo is cloned into `source/` inside a non-git parent working directory. The system `gcc` on PATH is an old 32-bit MinGW toolchain that cannot build cgo. `CC`/`CXX` user env vars normally point at a working MSYS2 GCC, but if a shell predates that fix, force pure-Go builds with `CGO_ENABLED=0 go build ./...`. This doesn't matter for the app itself (pure-Go SQLite driver) but can matter for other cgo-dependent tooling.

## Architecture

### The protocol bridge is the core feature
`internal/protocols/bridge.go` is what the whole project exists to demonstrate. A single HTTP call — `PUT /users/progress` — fans out through `ProtocolBridge.BroadcastProgressUpdate`:
1. **TCP** — writes the update to the sync server, which relays it to every connected client (`internal/tcp`)
2. **UDP** — sends `BROADCAST <json>` to the standalone UDP server, which pushes a `progress_update` notification to registered subscribers (`internal/udp`)
3. **gRPC** — calls `UpdateProgress` with `x-mangahub-audit: true` metadata and the user's own JWT (`ProgressEvent.Token`, captured from the request via `auth.GetToken`); the server verifies and logs an `AUDIT` entry **without writing** (a write would race newer HTTP updates and roll the chapter back). Calls without that metadata still write. Calls go through a FIFO worker so they stay in order. `UpdateProgress` requires `authorization: Bearer <jwt>` (`internal/grpc/auth.go` interceptor) and only lets non-admins change their own progress; `GetManga`/`SearchManga` are public.
4. **WebSocket** — posts a `system` notice into the manga's chat room `manga_<mangaID>` via `websocket.Hub.NotifyRoom` (the API server hosts the hub in-process)

The bridge receives the *merged* progress row after the update (not the raw request). No leg ever fails the HTTP request; each only logs. TCP and UDP sockets are dialed lazily and redialed after errors, so the servers may start in any order or restart. When touching progress-update behavior, check whether a change needs to propagate through all four legs, not just the HTTP handler.

**Chapter releases** (`internal/chapters`) are the other notification path. `Service.Release` raises `manga.total_chapters` and calls a `Notifier` with the manga's readers (library entries not `dropped`). In the API server the notifier is the bridge (targeted UDP + a chat room notice); `data-cli` uses `udp.Broadcaster`. Triggers: `POST /admin/manga/:id/chapters` (admin role), `data-cli sync-chapters`, and the optional `chapters.sync_interval` loop. `Syncer` checks MangaDex through the `Source` interface; tests use a fake source. UDP targeting works because subscribers register with `REGISTER <jwt>` (the UDP server verifies it with `auth.VerifyToken` and the shared secret). The server delivers a notification with `user_ids` only to those users and strips the list before sending.

`PUT /users/progress` is a partial update: only fields present in the JSON (`current_chapter`, `status`, `is_favorite`) change (`models.UpdateProgressRequest` uses pointers). Handler goroutines must not touch the `gin.Context` or its request context after the handler returns — capture values and use `context.Background()`.

UDP subscribers expire after `udp.subscriber_ttl` (5m) without a `REGISTER`; clients (`internal/tui/network.UDPListener`, `internal/udp.Client`) re-send it every minute as a heartbeat. HTTP routes are rate limited per connection IP (`internal/ratelimit`, `server.rate_limit` etc.; `0` disables); `/auth/login` counts only failed (401) attempts, since every local client shares one IP. Every `test-*.ps1` script and `test/load_test.sh` must exit non-zero when a check fails. Anything that sends to `websocket.Hub` channels must also select on `hub.stop`, so shutdown (`Hub.Stop`: 1001 close frames, drain the chat save queue) never leaves goroutines blocked.

List queries must finish iterating (and close) their rows before running any other query. Load genres with `manga.AttachGenres` after the loop. A nested query inside a `rows.Next()` loop holds two pooled connections per request and deadlocked the 25-connection pool under load.

### Four independent server binaries, one shared DB
`cmd/api-server` (routes and wiring in `internal/server`), `cmd/tcp-server`, `cmd/udp-server`, `cmd/grpc-server` are separate processes (separate `go run`/ports) that all read/write the same SQLite file at `data/mangahub.db`. Concurrent-start seeding safety is handled via a `seed_meta` marker row claimed inside a write transaction (see `pkg/database/seed.go`) — do not assume single-process startup ordering.

### Per-domain vertical slices under `internal/`
Most business domains (`auth`, `manga`, `progress`, `rating`, `comment`, `customlist`, `activity`, `leaderboard`, `chat`, `chapters`) follow the same three-file pattern: `repository.go` (SQL), `service.go` (business logic), `handlers.go` (Gin HTTP handlers). Follow this pattern when adding a new domain rather than introducing a different layering.

`internal/tcp`, `internal/udp`, `internal/websocket`, `internal/grpc` hold the protocol-specific server/client/protocol code consumed by the bridge and by `cmd/*-server`.

### CLI/TUI
`internal/cli/<noun>/` (auth, config, debug, library, manga, progress) holds Cobra command groups wired together in `internal/cli/root/root.go`, consumed by `cmd/cli`. `internal/tui/` (Bubble Tea) has its own `api/`, `network/`, `styles/`, `views/` subpackages and is consumed by `cmd/tui`. Both talk to the running servers over the network rather than touching the DB directly.

### Config
`pkg/config/config.go` defines all config structs and Viper defaults; YAML files live in `configs/{development,docker,production}.yaml`. Every binary calls `Load("./configs/development.yaml")`; set `MANGAHUB_CONFIG=<path>` to load a different file. Any key can be overridden by an environment variable with dots replaced by underscores (`TCP_HOST=tcp-server`, `SERVER_PORT=8081`) — docker-compose uses this to point the API's bridge at the other containers.

### Database
Text search (`manga.Repository.List` with `q`) uses the `manga_fts` FTS5 index joined on `rowid`. Build FTS queries only through `ftsQuery`, which quotes every word so user input can't inject FTS syntax.

Normalized SQLite schema (21 tables), managed by code-first migrations in `pkg/database/sqlite.go` — there is no separate migration tool/CLI; schema changes go directly in that file. Triggers keep `manga.average_rating`, comment like counts, and `activity_feed` in sync automatically, and `manga_fts` (FTS5) is kept in sync by triggers too — don't hand-maintain these in application code. The exception is progress activity, which has no trigger; `internal/progress/handlers.go` records it. Rating and comment activity come *only* from triggers; recording them in handlers too would duplicate feed entries. `manga_fts` is an external-content table: its triggers must key rows by `manga.rowid` and remove entries with the `'delete'` command (see `fixMangaFTSTriggers`). Timestamps are written from Go (`time.Now()`), never SQLite's `datetime('now')`: that produces UTC text that sorts wrongly against the local-time strings everywhere else. WAL mode + busy timeout are enabled for safe multi-process access across the four server binaries.

## Default seeded accounts
`admin`/`admin123` (admin role), `reader1`/`reader2`/`mangafan` with `password123` (user role).
