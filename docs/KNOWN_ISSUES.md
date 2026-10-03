# Known Issues and Resolutions

## Overview
This document tracks all known issues, their resolutions, and the current testing status of the MangaHub project.

## Fixed Issues

### Issue 1: TCP Client Connection Timeout
- **Status**: ✅ FIXED
- **Phase**: Phase 3 - TCP Real-time Sync
- **Description**: TCP client would hang indefinitely if server was unreachable
- **Root Cause**: Missing connection timeout and error handling in bridge
- **Fix**: 
  - Added `DialTimeout` with 5-second timeout
  - Implemented proper error handling and connection retry logic
  - Added graceful degradation when TCP unavailable
- **Files Modified**: `internal/bridge/bridge.go`
- **Commit**: Phase 7 implementation

### Issue 2: UDP Broadcast Buffer Overflow
- **Status**: ✅ FIXED
- **Phase**: Phase 4 - UDP Notifications
- **Description**: UDP messages would be dropped when too many clients registered
- **Root Cause**: Client buffer size too small (64 bytes)
- **Fix**: 
  - Increased buffer size to 256 bytes
  - Implemented buffer overflow detection
  - Added client connection limit (1000 clients max)
- **Files Modified**: `cmd/udp-server/main.go`
- **Commit**: Phase 4 implementation

### Issue 3: WebSocket Token Validation Errors
- **Status**: ✅ FIXED
- **Phase**: Phase 5 - WebSocket Chat
- **Description**: JWT validation errors during WebSocket upgrade
- **Root Cause**: Token parsing from query parameters not handling all formats
- **Fix**: 
  - Added proper query parameter parsing
  - Implemented fallback to header-based auth
  - Added detailed error messages for auth failures
- **Files Modified**: `internal/websocket/handlers.go`
- **Commit**: Phase 5 implementation

### Issue 4: gRPC Concurrent Request Handling
- **Status**: ✅ FIXED
- **Phase**: Phase 2 - gRPC Service
- **Description**: gRPC server would occasionally deadlock under high concurrent load
- **Root Cause**: Database connection pool exhaustion
- **Fix**: 
  - Increased database connection pool size
  - Added connection timeout configuration
  - Implemented proper connection release in defer statements
- **Files Modified**: `pkg/database/database.go`
- **Commit**: Phase 6 implementation

### Issue 5: Protocol Bridge Ordering
- **Status**: ✅ FIXED
- **Phase**: Phase 7 - Protocol Bridge
- **Description**: Bridge would sometimes broadcast to protocols in inconsistent order
- **Root Cause**: Goroutines executing in non-deterministic order
- **Fix**: 
  - Maintained HTTP-first priority for response
  - Added synchronization for critical broadcasts
  - Implemented error aggregation across all protocols
- **Files Modified**: `internal/bridge/bridge.go`
- **Commit**: Phase 7 implementation

### Issue 6: Database Seed Race on Concurrent Server Start (2026-07-18)
- **Status**: ✅ FIXED
- **Description**: Starting all 4 servers at once could crash one of them with `UNIQUE constraint failed: users.email` — every process passed the "already seeded?" check before any of them committed
- **Fix**: Seed now runs inside a single write transaction that first claims a one-row `seed_meta` marker (`INSERT OR IGNORE`), serializing concurrent starters; added `busy_timeout(10000)` to the connection string
- **Files Modified**: `pkg/database/seed.go`, `pkg/database/sqlite.go`

### Issue 7: NULL Column Scan Failures (2026-07-18)
- **Status**: ✅ FIXED
- **Description**: `GET /manga`, gRPC `SearchManga`/`GetManga`, and several repository queries returned 500 (`converting NULL to string is unsupported`) because nullable columns (`cover_url`, `author`, `year`, `review_text`, …) were scanned into non-pointer Go types
- **Fix**: Wrapped nullable columns in `COALESCE(...)` across manga, grpc, progress, rating, leaderboard, and customlist queries; also repaired a corrupted SQL statement in `manga.GetByID`
- **Files Modified**: `internal/manga/repository.go`, `internal/grpc/service.go`, `internal/progress/repository.go`, `internal/rating/repository.go`, `internal/leaderboard/service.go`, `internal/customlist/repository.go`

### Issue 8: Schema/Query Column Mismatches (2026-07-18)
- **Status**: ✅ FIXED
- **Description**: Rating APIs used column `review` (schema: `review_text`); leaderboards used `overall_rating` (schema: `rating`); leaderboard user query referenced missing `users.avatar_url` — all three endpoints returned 500 in production while unit tests passed against hand-written test schemas
- **Fix**: Aligned all queries with the real schema, added `users.avatar_url` via an additive migration, and rewrote unit tests to build their in-memory DB with the production `Migrate()` so schema drift now fails tests
- **Files Modified**: `internal/rating/repository.go`, `internal/leaderboard/service.go`, `pkg/database/sqlite.go`, `internal/leaderboard/leaderboard_test.go`, `internal/comment/comment_test.go`

### Issue 9: Hand-Edited Generated Protobuf Code (2026-07-18)
- **Status**: ✅ FIXED
- **Description**: `internal/grpc/pb/manga.pb.go` had been edited by hand (extra `Rating` field, `Genre` missing from the `goTypes` registry), causing a runtime panic `type mismatch: got *pb.MangaResponse, want *pb.SearchRequest` on any gRPC call
- **Fix**: Restored the correct generated file, removed the duplicate copies in `proto/` and `internal/grpc/pb/proto/`, dropped the stray field usage
- **Files Modified**: `internal/grpc/pb/manga.pb.go`, `internal/grpc/client.go`

### Issue 10: Tests Required CGO / a C Compiler (2026-07-18)
- **Status**: ✅ FIXED
- **Description**: Comment and leaderboard tests imported `mattn/go-sqlite3` (CGO) while the app uses the pure-Go `glebarez/go-sqlite`, so `go test ./...` failed on machines without a 64-bit GCC
- **Fix**: Tests now use the same pure-Go driver as the app; `mattn/go-sqlite3` removed from `go.mod`
- **Files Modified**: `internal/comment/comment_test.go`, `internal/leaderboard/leaderboard_test.go`, `go.mod`

### Issue 11: UDP Port Inconsistency (2026-07-18)
- **Status**: ✅ FIXED
- **Description**: `configs/development.yaml` used UDP port 9095 while docs, defaults, production config, TUI, and the integration tests all used 9091
- **Fix**: Standardized everything on 9091
- **Files Modified**: `configs/development.yaml`, `cmd/test-udp/main.go`, `test-all.ps1`

### Issue 12: Nonexistent Manga Returned 500 on Progress Update (2026-07-18)
- **Status**: ✅ FIXED
- **Description**: `PUT /users/progress` with an unknown `manga_id` surfaced a raw foreign-key failure as `500 INTERNAL_ERROR`
- **Fix**: Repository now verifies the manga exists and returns a typed 404 `NOT_FOUND`
- **Files Modified**: `internal/progress/repository.go`

### Issue 13: CLI `library list` Panic & Piped Password Input (2026-07-18)
- **Status**: ✅ FIXED
- **Description**: `mangahub library list` panicked (`interface conversion: interface {} is nil`) because it expected a nested `reading_progress` object while the API returns progress fields at the top level. `auth login`/`register` silently sent an empty password when stdin was piped (scripts/CI) because `term.ReadPassword` errors were discarded
- **Fix**: Library list now parses the real response shape defensively; added a `readPassword` helper that uses no-echo terminal input when interactive and falls back to reading a line from stdin when piped
- **Files Modified**: `internal/cli/library/list.go`, `internal/cli/auth/login.go`, `internal/cli/auth/register.go`, `internal/cli/auth/password.go` (new)

### Issues 14–30: Full-project audit fixes (2026-09-30)
All verified live against the four running servers (47 HTTP/TCP/UDP checks, 33 TUI-package checks, gRPC via grpcurl, repo integration tests).

| # | Area | Problem | Fix |
|---|------|---------|-----|
| 14 | Bridge / UDP | API server created an in-process UDP server it never started; notifications never left the process | Bridge sends `BROADCAST <json>` to the standalone UDP server (`internal/protocols/bridge.go`) |
| 15 | Bridge / TCP | TCP leg skipped forever if the TCP server wasn't up at API start; never redialed after a restart | Lazy dial + redial on write error; drains relayed messages and detects server loss |
| 16 | Bridge / WS | No WebSocket leg existed | Bridge posts a `system` notice to room `manga_<id>` via `Hub.NotifyRoom` |
| 17 | Bridge / gRPC | Async gRPC re-write of progress landed after newer HTTP updates, briefly rolling chapters back | Bridge calls with `x-mangahub-audit` metadata → server logs, doesn't write; FIFO worker keeps order |
| 18 | DB pool | Genre lookups nested inside open `rows` loops held 2 connections/request; >25 concurrent list requests hung forever | `manga.AttachGenres` batch-loads genres after rows close (manga list, library, gRPC search) |
| 19 | Activity | `GET /activities` returned 500 once any rating existed (NULL `comment_text`) | `COALESCE` in reads; NULL stored for empty text |
| 20 | Progress | `PUT /users/progress` replaced every column: missing status → 500, chapter updates cleared favorites, TUI favorite/status actions failed or reset chapter | Partial update (pointer fields), race-free upsert, `started_at`/`completed_at` maintained |
| 21 | Progress / rating | Background goroutines used the recycled `gin.Context` → progress activity never recorded | Capture values, own context; rating activity left to triggers (+ new update/delete triggers) |
| 22 | Ratings | Every error returned 500 (incl. invalid rating, unknown manga); `is_spoiler` dropped; non-standard JSON envelope | AppError mapping (400/404), `is_spoiler` persisted, standard envelope |
| 23 | Leaderboard | Trending fallback queried nonexistent `m.rating` → 500 | Uses `average_rating`/`rating_count` |
| 24 | WebSocket | Full send buffer closed a channel that unregister closed again → panic; chat never persisted; clients could spoof `join`/`system`; 512-byte frame limit | Single close path; persistence with auto-created rooms + `GET /rooms/:id/messages`; client types forced to `message`; 8 KB frames |
| 25 | TCP server | Disconnected clients never unregistered (`wg.Wait` deadlock) | Read loop owns lifecycle |
| 26 | gRPC | Unknown-manga/validation errors returned `codes.Unknown` + raw FK errors; negative chapters accepted; genre filter ignored; UTC timestamps broke library ordering | Proper status codes, validation, shared manga repository, Go timestamps |
| 27 | Auth | Disabled accounts could log in; refresh downgraded admins to `user`; no alg pinning; WebSocket couldn't authenticate from browsers/wscat | `is_active` check (403), real role, HS256 + issuer pinned, `?token=` on `/ws/chat` only |
| 28 | Comments | `total_count`/`has_more` counted replies; `liked_by_me` always false; cross-manga and reply-to-reply parents accepted; unknown manga → 500 | Top-level counts, optional auth on GET, parent validation/flattening, 404 |
| 29 | TUI | Chat showed raw JSON (timestamp type); UDP listener bound the server's port and never registered; 4xx treated as success; POST retried; ratings always 0; registration crashed (nil user); logout didn't stop UDP/WS and allowed token-less "re-login"; interactive login never started notifications; progress modal cleared favorites | Rewrote `network/ws_client.go` (sessions) and `network/udp_listener.go`; fixed `api/client.go` transport + shapes; `logout()`/`joinChat()` helpers in `app.go` |
| 30 | CLI / importer / config / FTS | `--rating` ignored; CLI panicked on non-standard errors and reset chapter/favorite via default flags; importer mapping used nonexistent `id` column, never linked genres, failed on unknown statuses; env overrides didn't work (no key replacer); FTS triggers corrupted the index on text updates | All fixed; `MANGAHUB_CONFIG` + `TCP_HOST`-style env overrides; FTS triggers migrated + index rebuilt on existing DBs |

### Issues 31–35: Found while writing the test suite (2026-09-30)

| # | Area | Problem | Fix |
|---|------|---------|-----|
| 31 | TCP server | Every client's write loop appended `'\n'` into the *same* shared broadcast buffer (data race, caught by `go test -race`) | Newline added once before broadcasting |
| 32 | TCP/UDP servers | `Stop()` read the listener/socket while `Serve()` could still be setting it on another goroutine (data race) | Guarded by a mutex |
| 33 | Chat | `GetRoom` failed on rooms without a description (NULL scanned into string); history, comments, ratings and activity feed returned same-timestamp rows in random order (Windows clock ticks are coarse and IDs are random UUIDs) | `COALESCE`; `rowid` tiebreaker on every `created_at` sort |
| 34 | Chat | Messages were saved from each sender's goroutine, so history order could differ from what the room saw live | Hub queues saves in broadcast order; one worker writes them |
| 35 | API | Leaderboard errors returned raw SQL error text to clients; no CORS headers despite docs claiming CORS support; normal client disconnects logged as ERROR | Errors logged server-side only; CORS middleware (`internal/server`); disconnect writes logged at debug |

### Features added 2026-10-02

- **Custom lists** (`/lists`, see `docs/API.md`). The package existed but was written for `net/http` and never routed; it is now on Gin with proper status codes. Adding to a public list creates a `list_add` activity.
- **Full-text search:** `GET /manga?q=` and gRPC `SearchManga` use the `manga_fts` FTS5 index. They match word prefixes, ignore accents, and rank by relevance (title > author > description). User input is quoted, so it can't inject FTS syntax.
- **New chapter notifications:** `POST /admin/manga/:id/chapters`, `data-cli sync-chapters` (MangaDex), and an optional periodic sync (`chapters.sync_interval`). A UDP `chapter_release` goes only to the manga's readers: subscribers now register with `REGISTER <jwt>`, and the UDP server routes on `user_ids`. A notice is also posted to the manga's chat room.
- **TUI:**
  - Entering a chat room shows its last 50 saved messages.
  - The library asks y/n before removing a manga, and `u` opens a chapter input; `+`/`-` step one chapter.
  - The TUI subscribes to UDP with the login token, so it receives chapter releases.

### Issues 36–41: Found while building these (2026-10-02)

| # | Area | Problem | Fix |
|---|------|---------|-----|
| 36 | Seed data | Every seeded manga got a random 8-character "MangaDex ID", so all MangaDex requests failed (`400 … must be at least 36 characters`) | The seed no longer invents IDs; a migration clears non-UUID `mangadex_id` values; `sync-chapters --link` finds the real ones by title |
| 37 | TUI library | The Plan tab filtered on status `planning`, which doesn't exist (always empty), and key `2` sent it to the API (400) | Uses `plan_to_read` |
| 38 | TUI keys | Global shortcuts (`a` activity, `c` chat, `h` dashboard, `l` library) swallowed the same keys in views: detail's `a` (add to library) and `c` (manga chat) never ran; browse/detail `h`/`l` navigation and activity `l` didn't work | Views declare their own keys (`viewKeys` in `app.go`), which win over global shortcuts |
| 39 | TUI chat | Switching rooms kept the previous room's messages on screen | `SetRoom` clears messages when the room changes |
| 40 | Custom lists | Errors were plain strings (everything looked like a 500), scan errors were swallowed, an unknown manga hit the FK (500), empty results were `null`, `EnsureDefaultLists` queried a nonexistent `is_default` column | Rewritten with `AppError` codes, validation, genres, `[]` for empty, dead code removed |
| 41 | data-cli | Opened the database without WAL/busy timeout (immediate `SQLITE_BUSY` while servers run) and without running migrations | Same pragmas as the servers; runs `Migrate()` on start |

### Security and operations hardening (2026-10-02)

- **gRPC authentication.** `UpdateProgress` requires `authorization: Bearer <jwt>` metadata (the token from `/auth/login`, checked by `internal/grpc.AuthInterceptor` with the same secret and issuer as the HTTP API). Users may only change their own progress; admins may change anyone's. A missing or bad token returns `UNAUTHENTICATED`; someone else's progress returns `PERMISSION_DENIED`. `GetManga` and `SearchManga` stay public, like their HTTP equivalents. The protocol bridge forwards the user's token with its audit call. `test-grpc.ps1` and `cmd/test-grpc -token` log in first.
- **Rate limiting** (`internal/ratelimit`, token bucket per client IP): 50 requests/s with bursts of 100 on every HTTP route, plus 10 per minute on `/auth/register` and 10 **failed** logins (wrong password) per minute on `/auth/login`. Over the limit returns `429 RATE_LIMITED` with `Retry-After`. The client IP comes from the TCP connection, not `X-Forwarded-For`, so a made-up header doesn't get around the limit. Configure with `server.rate_limit`, `rate_burst`, `auth_rate_limit`, `auth_rate_burst`; `0` turns a limit off.
- **UDP subscribers expire.** A subscriber that doesn't re-send `REGISTER` within `udp.subscriber_ttl` (default 5 minutes) is dropped. The TUI and `udp.Client` re-register every minute as a heartbeat, which also restores the subscription after a UDP server restart. If the login token in the heartbeat has expired, the TUI falls back to an anonymous subscription (general broadcasts only) and says so once.
- **Clean shutdown.** On Ctrl+C / SIGTERM the API server stops accepting connections, finishes in-flight requests (up to 10 s), drains the protocol bridge queues, sends every WebSocket client a `1001 going away` close frame, saves chat messages still queued, and only then closes the database. A second Ctrl+C force-quits. The gRPC server falls back from `GracefulStop` to `Stop` after 10 s.
- **staticcheck is clean** (`staticcheck ./...` reports nothing). The dead fields, functions and styles it listed were removed.

### Issues 42–44: Found during the hardening (2026-10-02)

| # | Area | Problem | Fix |
|---|------|---------|-----|
| 42 | WebSocket shutdown | `Hub.Stop` only ended the hub loop: clients saw an abnormal close (1006), chat messages still queued for saving were dropped, every client's `readPump` blocked forever sending to the stopped hub, a new connection blocked forever in `ServeWS`, and a second `Stop` panicked | `Stop` closes clients with 1001, drains the save queue (up to 5 s), and everything that sends to the hub also watches its stop channel |
| 43 | API server | A listen error (e.g. port 8080 in use) called `logger.Fatalf` from a goroutine, which exits without running the deferred cleanup | The error ends `main` normally |
| 44 | Chat | `chat.Repository.GetOrCreateMangaRoom` (never called) created rooms owned by a nonexistent user `system`, which violates the `owner_id` foreign key, with random IDs instead of the `manga_<id>` convention | Removed; manga rooms are created on first message by `EnsureRoom` |

### Issues 45–58: Test review (2026-10-02)

A review of every test (Go tests, the PowerShell scripts, the load test and the `cmd/test-*` tools), running the Go suite three times in random order with the race detector and the scripts against live servers, found that several tests could not fail, a few were flaky, and some exposed real bugs.

**Project bugs**

| # | Area | Problem | Fix |
|---|------|---------|-----|
| 45 | WebSocket shutdown | `Hub.Stop` sent its "going away" close frame while the client's own goroutines could close the connection first, so clients sometimes saw 1006 instead of 1001 (a flaky test caught it) | All three paths close through one `closeConn` (sync.Once) that sends the 1001 frame first while the hub is stopping |
| 46 | Rate limiting | Every login counted against the 10/minute auth limit, and register shared the same budget. All local clients share one IP, so the project's own test scripts run back to back (or a demo with a few logins) got `429` on correct passwords | Login counts only failed attempts (401); register has its own limit (`ratelimit.FailureMiddleware`) |
| 47 | Seed data | The sample libraries used `SELECT ... LIMIT 5` without `ORDER BY` on random UUIDs, so every fresh database got a different set of 5 manga (and a test that assumed a manga wasn't in anyone's library failed about 1 run in 20) | Ordered by title |
| 48 | TUI | The activity view showed made-up users and posts when the API failed or the feed was empty; the dashboard did the same, and matched activity types the API never sends (`manga_rated`, `chapter_read`, ...), so real entries read like "progress Naruto". `list_add` showed as "started reading" | Errors and empty feeds are shown as such; types match the API (`comment`, `rating`, `progress`, `list_add`) |
| 49 | WebSocket logs | A client closing normally (code 1000) was logged as `ERROR` | Normal closes are not logged; unexpected ones are warnings |
| 50 | `cmd/test-udp` | Sent bare JSON instead of `BROADCAST <json>`, which the server ignores, so its "notification sent" never reached anyone | Sends a proper broadcast request; `-listen 3s` waits for it and exits 1 if it doesn't arrive |
| 51 | `cmd/test-grpc` | Defaulted to a hard-coded manga ID that no seeded database has; exited 0 even when the call failed | Looks up a real manga when `-manga` is omitted, takes the user from the token, exits 1 on failure |
| 52 | `docker-compose.yml` | The obsolete `version:` key made every compose command print a warning | Removed |

**Tests that couldn't fail, or were flaky**

| # | Test | Problem | Fix |
|---|------|---------|-----|
| 53 | `test-websocket.ps1` | Sent `{"message": ...}` (the server reads `content`), so no chat message was ever sent, then printed PASS unconditionally. A receive timeout in .NET Framework also aborts the socket, which caused the "Send error" seen in its output | Sends `content`; two different users; checks join, both messages on both clients, room info, saved history and the leave notice; exits 1 on failure |
| 54 | `test-integration.ps1` | Printed "TCP / UDP / WebSocket / gRPC triggered" without checking any of them | Subscribes on TCP, UDP and the manga's chat room, makes one update and checks each one receives it |
| 55 | `test-tcp.ps1`, `test-udp-simple.ps1` | TCP counted the sender getting its own message back as "broadcast working" and used background jobs whose start-up delay made timing random; UDP waited for demo notifications that are opt-in, so it never checked receiving | TCP checks both directions between two clients; UDP sends its own broadcast and checks `UNREGISTER` stops delivery |
| 56 | All `test-*.ps1`, `test/load_test.sh` | Printed FAIL lines but always exited 0 (and `test-all.ps1` printed "✓" for every item regardless); the load test needed `ab`/`nc`, skipped silently without them and had no checks; `test-all.ps1` ran unit tests for one package and tested a stale `bin\mangahub.exe` | Every script counts failures and exits 1; `test-all.ps1` runs all Go tests, builds the CLI first and prints a real summary; the load test needs only bash and curl and checks every request |
| 57 | `test/live_test.go` | GetManga used an ID that doesn't exist and only asserted on success; SearchManga asserted `len >= 0`; UDP/TCP checks were optional; unreachable servers were skipped even with `MANGAHUB_LIVE=1` | Real assertions, plus a live version of the 4-protocol fan-out test; with `MANGAHUB_LIVE=1` an unreachable server fails |
| 58 | Go tests | The TUI-client e2e test called `t.Fatal` from a goroutine and could take a late notification for the one it wanted; the CLI e2e test failed on a second run in one process (Cobra flags stayed set); the chapter-release test could read the progress notice instead of the release notice; the rate-limit test could pass or fail depending on machine speed; `comment` and `leaderboard` tests used `:memory:` SQLite (each pooled connection gets an empty database); several waits on channels had no timeout; a soft-delete and a trending test passed without checking anything | Fixed each; the suite passes 3 times in a row in random order with `-race` |

### Issue 59: Docker setup (2026-10-03)

| Problem | Fix |
|---|---|
| `Dockerfile` and `docker/*` used `golang:1.21-alpine`, which can't build a module that requires Go 1.25 | `golang:1.25-alpine`; built with `CGO_ENABLED=0` (the SQLite driver is pure Go), so no gcc/sqlite packages |
| `docker/Dockerfile.tcp` was empty, so the `docker build -f docker/Dockerfile.tcp` command in HOW_TO_RUN.md failed; `docker/Docker.api` copied `data/`, which a fresh clone may not have | All four per-service Dockerfiles written the same way; they create `/app/data` instead of copying it |
| `configs/docker.yaml` used the key `http_port` (ignored) and `expiration: 86400`, which Viper reads as 86400 **nanoseconds** (every token expired at once) | `port`, `24h` |
| `scripts/build-docker.sh` was an unfinished stub; DOCKER.md pointed at `docker-compose.prod.yml` and `configs/production.yaml.example`, which don't exist, and ran `go test` inside the runtime image (no Go there) | Script builds the compose images; DOCKER.md shows the real production run and where tests run |

Verified on Docker 28 / Compose 2.34 from a clean copy of the repo: `docker compose build` and `up` work, the API is healthy, the bridge reaches the other containers by service name (gRPC audits arrive with the user's token), every `test-*.ps1` script, `test/load_test.sh` and the live Go tests pass against the containers, and `docker compose stop` shuts all four down cleanly (exit code 0) within 5 s.

## Current Issues

- **UDP `BROADCAST` and the TCP sync server are unauthenticated.** Any process that can reach port 9091 can make the server send a notification (including a targeted one), and anything that reaches port 9090 can push progress lines to sync clients. UDP subscriptions are authenticated (`REGISTER <jwt>`), sends are not. Fine on a private network.
- **Rate limits are per API process and per connection IP.** Behind a reverse proxy every client would share the proxy's IP. Deploying behind one needs `router.SetTrustedProxies(...)` set to the proxy's address in `internal/server`.
- `internal/tui/views/progress.go` is never used by the app.

## Testing Status

*Measured 2026-10-02 (first measured 2026-09-30). Earlier versions of this section listed per-protocol coverage of 72–88% that no test run had produced. At that point only three packages had tests and total coverage was a few percent.*

### How tests are organized

| Suite | Location | Needs servers? | Runs in CI |
|---|---|---|---|
| Unit tests (23 packages) | `internal/...`, `pkg/...` (`*_test.go`) | No: real schema via `database.Migrate()` on temp-file SQLite (`internal/testutil`) | Yes |
| End-to-end | `test/e2e_test.go`, `test/clients_test.go` | No: `test/stack_test.go` starts the seeded DB, TCP, UDP and gRPC servers and the full HTTP/WebSocket API (`internal/server`) in-process on random ports | Yes |
| Live | `test/live_test.go` | Yes, all four on default ports | No (`MANGAHUB_LIVE=1`, i.e. `make test-integration`) |
| Scripts | `test-*.ps1` (PowerShell), `test/load_test.sh` (bash + curl) | Yes | No. Each exits 1 when a check fails; `test-all.ps1` runs the Go suites too |

The end-to-end suite covers: one progress update reaching TCP, UDP, gRPC and the WebSocket room; 20 rapid update pairs with no rollback; 60 concurrent requests against the 25-connection pool; partial updates; ratings/activity triggers; search, genre filter, pagination, leaderboards; comment threads; auth hardening; chat persistence/history; gRPC over the wire; CORS; and the real TUI client code (HTTP, UDP listener, WebSocket) and CLI commands against the running stack.

All suites pass with the race detector (`go test -race ./...`), which CI runs on every push (`.github/workflows/ci.yml`). On 2026-10-02 the Go suite also passed three times in a row in random order (`go test -race -count=3 -shuffle=on ./...`), and every script passed twice in a row against freshly started servers.

### Coverage

`go test -coverpkg=./internal/...,./pkg/... -coverprofile=coverage.out ./...` (i.e. `make test-coverage`):

| Scope | Statements covered |
|---|---|
| Backend + protocol packages (everything except the items in the next row) | **~71%** (2766 / 3898) |
| Whole codebase, including TUI rendering (`internal/tui/views`, `styles`, `app.go`), generated protobuf code, external API clients (`pkg/external`, `pkg/cache`) and the CLI auth/config/debug/library commands | **~48%** |

Per package, counting all tests: `pkg/config` 93%, `server` 90%, `ratelimit` 90%, `cli/progress` 86%, `chapters` 85%, `websocket` 84%, `manga` 84%, `protocols` 84%, `leaderboard` 81%, `tui/network` 79%, `customlist` 78%, `auth` 78%, `grpc` 71%, `importer` 67%, `rating` 67%, `progress` 66%, `tcp` 65%, `activity` 63%, `chat` 63%, `database` 60%, `comment` 59%, `tui/api` 58%, `udp` 55%, `cli/manga` 51%, `tui/views` 22%.

## Performance (measured)

*Measured 2026-09-30, before rate limiting existed: to reproduce, set `server.rate_limit: 0` (one load generator is one IP, so the default 50 req/s limit would reject most requests). Measured on the development laptop (Windows 11, 12 logical CPUs) with the four server binaries (default dev config) on localhost, the seeded database (101 manga), `logging.level: info` written to a file, and a Go load generator on the same machine. Treat the numbers as this machine's order of magnitude, not a benchmark. Earlier figures in this file (e.g. "HTTP 4,000 req/sec", "gRPC 5,500 req/sec") had no measurement behind them.*

| Workload | Req/s | p50 ms | p95 ms | p99 ms | Errors |
|---|---|---|---|---|---|
| HTTP `GET /manga?limit=20` (20 workers, 10 s) | 1,725 | 6.9 | 38.0 | 61.6 | 0 |
| HTTP `GET /manga/:id` (20 workers) | 5,342 | 1.1 | 23.8 | 53.0 | 0 |
| HTTP `GET /manga?q=the` search (20 workers) | 1,702 | 7.3 | 37.2 | 66.3 | 0 |
| HTTP `PUT /users/progress` with 4-protocol fan-out (6 workers) | 414 | 3.2 | 57.0 | 182.3 | 0 |
| gRPC `GetManga` (20 workers) | 4,331 | 1.6 | 25.4 | 47.1 | 0 |
| gRPC `SearchManga` limit 20 (20 workers) | 1,943 | 6.4 | 33.8 | 50.5 | 0 |

- **Fan-out delivery:** during the PUT run, a TCP sync client and a UDP subscriber each received **4,152 of 4,152** updates.
- **WebSocket:** a message reached all 50 members of a room in p50 0.6 ms / p95 1.2 ms / p99 1.6 ms (200 messages).
- **Writes** are bounded by SQLite's single writer (WAL mode). Reads scale with the 25-connection pool.

## Security

- ✅ Passwords hashed with bcrypt; JWT (HS256 only, issuer checked); disabled accounts cannot log in
- ✅ Parameterized SQL everywhere; sort columns whitelisted
- ✅ Internal errors are logged, not returned to clients
- ✅ CORS: any origin, bearer-token auth only (no cookies)
- ✅ Rate limiting per client IP (429 + `Retry-After`), with a stricter limit on login/register
- ✅ gRPC `UpdateProgress` requires the user's JWT; users can only change their own progress
- ✅ UDP subscriptions tied to a user need a valid JWT; silent subscribers expire
- ⚠️ No TLS on any protocol; UDP `BROADCAST` and the TCP sync server are unauthenticated (fine for development / a private network)
- ⚠️ JWT secret is in `configs/development.yaml`; override it with the `JWT_SECRET` environment variable in real deployments
