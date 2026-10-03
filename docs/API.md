# MangaHub API Reference

The examples below are real responses from the API server with the seeded database; long values are shortened with `…`.

| Protocol | Default address | Purpose |
|---|---|---|
| HTTP REST | `http://localhost:8080` | Everything below |
| WebSocket | `ws://localhost:8080/ws/chat` | Chat rooms ([WebSocket](#websocket-chat)) |
| TCP | `localhost:9090` | Progress sync stream ([TCP](#tcp-progress-sync-9090)) |
| UDP | `localhost:9091` | Push notifications ([UDP](#udp-notifications-9091)) |
| gRPC | `localhost:9092` | `mangahub.v1.MangaService` ([gRPC](#grpc-9092)) |

- [Conventions](#conventions)
- [Health](#health)
- [Auth](#auth)
- [Manga](#manga)
- [Library & progress](#library--progress)
- [Ratings](#ratings)
- [Comments](#comments)
- [Custom lists](#custom-lists)
- [Activity feed](#activity-feed)
- [Leaderboards](#leaderboards)
- [Admin: chapter releases](#admin-chapter-releases)
- [Chat rooms](#chat-rooms)
- [WebSocket chat](#websocket-chat)
- [TCP progress sync (9090)](#tcp-progress-sync-9090)
- [UDP notifications (9091)](#udp-notifications-9091)
- [gRPC (9092)](#grpc-9092)

---

## Conventions

### Authentication

Protected endpoints need a JWT from [`POST /auth/login`](#post-authlogin), sent as a header:

```
Authorization: Bearer <token>
```

Tokens last 24 hours (`jwt.expiration`). Only `GET /ws/chat` also accepts the token as a `?token=` query parameter, because browsers and `wscat` can't set headers on a WebSocket upgrade. Every other endpoint rejects a query-string token.

### Response envelope

Most endpoints wrap their result in a standard envelope:

```json
{ "success": true, "message": "manga details", "data": { … }, "timestamp": "2026-09-30T15:03:44.61+07:00" }
```

Errors use the same envelope with an `error` object instead of `data`:

```json
{ "success": false, "error": { "code": "NOT_FOUND", "message": "manga not found" }, "timestamp": "…" }
```

A few endpoints return plain JSON without the envelope: [`GET /health`](#health), [`GET /activities`](#activity-feed) and [`GET /rooms/:room_id`](#get-roomsroom_id). Each one is marked below.

| HTTP | `error.code` | When |
|---|---|---|
| 400 | `VALIDATION_ERROR` | Field values out of range (rating 11, unknown status, negative chapter, …) |
| 400 | `BAD_REQUEST` | Body isn't valid JSON (`error.details.error` says why) |
| 401 | `UNAUTHORIZED` | Missing, malformed, expired or forged token; wrong password |
| 403 | `FORBIDDEN` | Account is disabled |
| 404 | `NOT_FOUND` | Unknown manga, comment, rating or library entry |
| 409 | `CONFLICT` | Username or email already registered |
| 429 | `RATE_LIMITED` | Too many requests from your IP; wait the number of seconds in the `Retry-After` header |
| 500 | `INTERNAL_ERROR` | Server-side failure. Details are logged, never returned |

### Rate limits

Limits are per client IP (the connection's address; `X-Forwarded-For` is ignored):

| Routes | Default | Config keys (`server.`) |
|---|---|---|
| Every route | 50 requests/s, bursts of 100 | `rate_limit`, `rate_burst` |
| `POST /auth/register` | additionally 10 per minute, bursts of 10 | `auth_rate_limit`, `auth_rate_burst` |
| `POST /auth/login` | additionally 10 **failed** logins (wrong password, 401) per minute, bursts of 10. Correct logins are never limited | same keys |

Over the limit:

```
HTTP/1.1 429 Too Many Requests
Retry-After: 6

{ "success": false, "error": { "code": "RATE_LIMITED", "message": "too many requests, please slow down" }, "timestamp": "…" }
```

Set a limit to `0` to turn it off (e.g. `rate_limit: 0` for load tests from one machine).

### Pagination

List endpoints use `limit` + `offset`, except comments and ratings, which use `page` + `page_size` / `limit`. A limit above an endpoint's maximum is capped at the maximum (manga, comments, leaderboards, activities) or replaced by the default (ratings, chat history). Empty results are `[]`, never `null`.

### CORS

Any origin may call the API: `Access-Control-Allow-Origin: *`, with methods `GET, POST, PUT, DELETE, OPTIONS` and headers `Authorization, Content-Type`. This is safe because authentication uses bearer tokens, not cookies.

---

## Health

### `GET /health`

No auth. **Not enveloped.** Returns 503 if the database is unreachable.

```json
{
  "status": "healthy",
  "server": "running",
  "database": { "connected": true, "status": "healthy", "table_count": 21, "open_connections": 1,
                "max_open_connections": 25, "ping_latency_ms": 0, "database_size_bytes": 561152 }
}
```

---

## Auth

### `POST /auth/register`

No auth. `username` 3–50 characters, `email` must be valid, `password` 8–100 characters. The response does **not** include a token; call [`/auth/login`](#post-authlogin) next.

```bash
curl -X POST localhost:8080/auth/register -H "Content-Type: application/json" \
  -d '{"username":"newuser","email":"newuser@example.com","password":"password123"}'
```

`201 Created`:

```json
{ "success": true, "message": "user registered successfully",
  "data": { "id": "fd23f9f1-…", "username": "newuser", "display_name": "newuser", "avatar_url": "",
            "created_at": "2026-09-30T15:03:44.30+07:00" } }
```

Errors: `409 CONFLICT` if the username or email is taken, `400 VALIDATION_ERROR` if a field is invalid.

### `POST /auth/login`

No auth. `username` may also be the email address.

```bash
curl -X POST localhost:8080/auth/login -H "Content-Type: application/json" \
  -d '{"username":"reader1","password":"password123"}'
```

```json
{ "success": true, "message": "login successful",
  "data": { "token": "eyJhbGciOiJIUzI1NiIs…", "expires_at": "2026-10-01T15:03:44.43+07:00",
            "user": { "id": "55c072c6-…", "username": "reader1", "display_name": "John Reader",
                      "avatar_url": "", "created_at": "…" } } }
```

Errors: `401 UNAUTHORIZED` (`"invalid credentials"`) for an unknown user or wrong password, `403 FORBIDDEN` (`"account is disabled"`) for a deactivated account.

Seeded accounts: `admin` / `admin123`, and `reader1`, `reader2`, `mangafan` with `password123`.

### `GET /auth/me`

Auth required. Returns the stored profile.

```json
{ "success": true, "message": "user profile retrieved",
  "data": { "id": "fd602e00-…", "username": "reader1", "display_name": "John Reader", "avatar_url": "",
            "role": "user", "created_at": "2026-09-30T15:06:46.18+07:00",
            "last_login_at": "2026-09-30T15:06:54.73+07:00" } }
```

### `POST /auth/refresh`

Auth required. Returns a fresh token. The user's role is preserved.

```json
{ "success": true, "message": "token refreshed", "data": { "token": "eyJhbGciOiJIUzI1NiIs…", "user_id": "55c072c6-…" } }
```

### `POST /auth/logout`

Auth required. Tokens are stateless, so this only acknowledges the logout; the client should discard its token.

```json
{ "success": true, "message": "logout successful", "data": { "message": "logged out successfully", "user_id": "55c072c6-…" } }
```

---

## Manga

### `GET /manga`

No auth. Search and list the catalog.

| Query | Meaning |
|---|---|
| `q` | Full-text search over title, author and description (FTS5). Every word must match as a word prefix (`pie` finds "One Piece"); accents are ignored (`pokemon` finds "Pokémon"). Results are ranked by relevance, with title matches before author and description matches |
| `genre` | Genre slug or display name, case-insensitive (`action`, `Slice of Life`). Repeatable: `?genre=action&genre=romance` matches either |
| `genres` | Comma-separated alternative: `?genres=action,romance` |
| `status` | `ongoing`, `completed`, `hiatus`, `cancelled` |
| `type` | `manga`, `manhwa`, `manhua`, `novel` |
| `sort_by` | `relevance` (default when `q` is set), `title` (default otherwise, A→Z), `rating`, `year`, `chapters` (newest/highest first) |
| `order` | `asc` / `desc`, overriding the default direction |
| `limit` / `offset` | Page size 1–100 (default 20) and start |

```bash
curl "localhost:8080/manga?q=one&genre=action&sort_by=rating&limit=2"
```

```json
{ "success": true, "message": "manga list",
  "data": {
    "data": [
      { "id": "08e7857c-…", "title": "Code Geass", "author": "Ichiro Okouchi", "artist": "Clamp",
        "description": "A student gains the power to command anyone…", "cover_url": "",
        "status": "completed", "type": "manga", "total_chapters": 148, "average_rating": 0,
        "rating_count": 0, "year": 2006,
        "genres": [ { "id": "0d621e22-…", "name": "Action", "slug": "action", "created_at": "…" } ],
        "created_at": "…", "updated_at": "…" }
    ],
    "total": 5, "limit": 2, "offset": 0, "has_more": true } }
```

### `GET /manga/:id`

No auth. Returns one manga object as `data`, in the same shape as the list items. Unknown ID: `404 NOT_FOUND`.

---

## Library & progress

A library entry is a user's progress row for one manga:

```json
{ "id": "a2ce53b1-…", "user_id": "55c072c6-…", "manga_id": "ff02e6b4-…", "current_chapter": 12,
  "status": "plan_to_read", "is_favorite": true, "started_at": "…", "completed_at": "…",
  "last_read_at": "…", "created_at": "…", "updated_at": "…" }
```

- `status` is one of `plan_to_read`, `reading`, `completed`, `on_hold`, `dropped`.
- `started_at` is set the first time a chapter is recorded or the status becomes `reading`/`completed`.
- `completed_at` is set on `completed` and cleared when the status changes away from it.
- `last_read_at` changes only when `current_chapter` is sent.

### `PUT /users/progress`

Auth required. **Partial update:** only the fields you send change. Omitted fields (or `"status": ""`) keep their stored value. If the manga isn't in the library yet, this creates the entry: `status` defaults to `reading` when a chapter is sent, `plan_to_read` otherwise.

| Field | Type | Rules |
|---|---|---|
| `manga_id` | string | Required |
| `current_chapter` | int | ≥ 0 |
| `status` | string | One of the statuses above |
| `is_favorite` | bool | |

```bash
curl -X PUT localhost:8080/users/progress -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" -d '{"manga_id":"ff02e6b4-…","is_favorite":true}'
```

`200` with the full updated entry as `data`. Errors: `400 VALIDATION_ERROR`, `404 NOT_FOUND` for an unknown manga.

**Each successful update fans out to every protocol:**
- a line to all [TCP](#tcp-progress-sync-9090) sync clients
- a `progress_update` [UDP](#udp-notifications-9091) notification
- a `system` message in the WebSocket room `manga_<manga_id>`
- an audit call to the [gRPC](#grpc-9092) service
- a `progress` entry in the activity feed, when a chapter was sent

The HTTP response doesn't wait for any of these, and a failure in one of them never fails the request.

### `POST /users/library`

Auth required. Adds a manga to the library. It takes the same body and follows the same rules as `PUT /users/progress`, but returns `201` and does not fan out.

### `GET /users/library`

Auth required. Returns the library as `data`, most recently read first. Each entry includes a nested `manga` object (with genres).

### `DELETE /users/library/:manga_id`

Auth required. Returns `{ "manga_id": "…", "removed": true }`. If the manga isn't in the library: `404` (`"manga not found in library"`).

---

## Ratings

### `POST /manga/:id/ratings`

Auth required. Creates or replaces your rating for this manga.

```bash
curl -X POST localhost:8080/manga/$MANGA_ID/ratings -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" -d '{"rating":9,"review_text":"Great arc","is_spoiler":false}'
```

| Field | Rules |
|---|---|
| `rating` | Integer 1–10, required |
| `review_text` | Optional, ≤ 5000 characters |
| `is_spoiler` | Optional bool |

```json
{ "success": true, "message": "rating submitted successfully",
  "data": { "id": "bf2dd12c-…", "manga_id": "ff02e6b4-…", "user_id": "55c072c6-…", "rating": 9,
            "review_text": "Great arc", "is_spoiler": false, "created_at": "…", "updated_at": "…" } }
```

Errors: `400 VALIDATION_ERROR`, `404 NOT_FOUND` for an unknown manga. `manga.average_rating`, `manga.rating_count` and the activity feed update automatically through database triggers.

### `GET /manga/:id/ratings?page=1&limit=20`

No auth. `limit` is at most 100. `rating_distribution[i]` counts the ratings of `i+1`.

```json
{ "success": true, "message": "ratings retrieved",
  "data": { "summary": { "manga_id": "ff02e6b4-…", "average_rating": 9, "rating_count": 1,
                         "rating_distribution": [0, 0, 0, 0, 0, 0, 0, 0, 1, 0] },
            "ratings": [ { "id": "bf2dd12c-…", "rating": 9, "review_text": "Great arc", "is_spoiler": false,
                           "username": "reader1", "display_name": "John Reader", … } ],
            "total": 1, "page": 1, "has_more": false } }
```

### `DELETE /manga/:id/ratings`

Auth required. Removes your rating. Returns `{ "manga_id": "…", "removed": true }`. If you haven't rated it: `404`.

---

## Comments

### `POST /manga/:id/comments`

Auth required.

| Field | Rules |
|---|---|
| `content` | Required, 1–2000 characters |
| `chapter_number` | Optional. Omit it for a comment on the whole manga |
| `is_spoiler` | Optional bool |
| `parent_id` | Optional; makes this a reply |

Threads are one level deep, so a reply to a reply is attached to the top-level comment. The parent must belong to the same manga (otherwise `400`) and must not be deleted (otherwise `404`).

`201`:

```json
{ "success": true, "message": "comment created successfully",
  "data": { "id": "0486de5b-…", "manga_id": "ff02e6b4-…", "chapter_number": 12, "user_id": "55c072c6-…",
            "content": "Loved this chapter", "is_spoiler": false, "likes_count": 0, "is_edited": false,
            "is_deleted": false, "created_at": "…", "updated_at": "…" } }
```

### `GET /manga/:id/comments?chapter=12&page=1&page_size=20`

No auth needed. Send a token to get your own `liked_by_me` values. Without `chapter`, only comments on the whole manga are returned. `page_size` is at most 50. `total_count` and `has_more` count top-level comments only; replies are nested under each one.

```json
{ "success": true, "message": "comments retrieved",
  "data": { "comments": [ { "id": "0486de5b-…", "content": "Loved this chapter", "chapter_number": 12,
                            "username": "reader1", "display_name": "John Reader", "likes_count": 0,
                            "liked_by_me": false, …,
                            "replies": [ { "id": "1de7dd2e-…", "content": "Same!", "parent_id": "0486de5b-…", … } ] } ],
            "total_count": 1, "page": 1, "page_size": 20, "has_more": false } }
```

### Editing, deleting and liking

| Endpoint | Auth | Body | Returns |
|---|---|---|---|
| `PUT /comments/:id` | Owner | `{ "content": "…", "is_spoiler": false }` | The updated comment (`is_edited: true`) |
| `DELETE /comments/:id` | Owner | – | `{ "comment_id": "…", "deleted": true }` (soft delete) |
| `POST /comments/:id/like` | Any user | – | `{ "comment_id": "…", "liked": true }`; liking twice is harmless |
| `DELETE /comments/:id/like` | Any user | – | `{ "comment_id": "…", "liked": false }` |

Editing or deleting someone else's comment, or a comment that doesn't exist, returns `404`.

---

## Custom lists

Named, ordered collections of manga ("Top 10", "Must read"). A list is **private** by default; a **public** list can be read by anyone, and adding a manga to it shows up in the activity feed (`activity_type: "list_add"`, with the list name in `comment_text`).

| Endpoint | Auth | Notes |
|---|---|---|
| `GET /lists` | Required | Your lists, private and public |
| `GET /lists?user_id=<id>` | Optional | That user's public lists (all of them if it's you) |
| `POST /lists` | Required | `{ "name": "Must read", "description": "…", "is_public": true }`. `name` is 1–100 characters, `description` ≤ 500 |
| `GET /lists/:id` | Optional | The list with its manga, in order |
| `PUT /lists/:id` | Owner | Any of `name`, `description`, `is_public`; omitted fields stay |
| `DELETE /lists/:id` | Owner | Deletes the list and its items |
| `POST /lists/:id/items` | Owner | `{ "manga_id": "…", "notes": "…" }` adds at the end; adding a manga that's already there only updates its notes |
| `DELETE /lists/:id/items/:manga_id` | Owner | |
| `PUT /lists/:id/order` | Owner | `{ "item_ids": [ … ] }` must list every item ID of the list exactly once |

A private list you don't own returns `404` (its existence isn't revealed). Changing someone else's public list returns `403`. Adding an unknown manga returns `404`.

`GET /lists/:id`:

```json
{ "success": true, "message": "list retrieved",
  "data": { "id": "4c1f…", "user_id": "55c072c6-…", "name": "Must read", "description": "", "is_public": true,
            "sort_order": 0, "item_count": 2, "created_at": "…", "updated_at": "…",
            "items": [ { "id": "9a0e…", "list_id": "4c1f…", "manga_id": "ff02e6b4-…", "notes": "re-read",
                         "sort_order": 0, "added_at": "…",
                         "manga": { "id": "ff02e6b4-…", "title": "A Place Further Than the Universe", "genres": [ … ], … } } ] } }
```

`GET /lists` returns `{ "lists": [ … ], "total": 2 }` as `data`; each list has `item_count` but no items.

---

## Activity feed

Entries are created automatically: comments and ratings by database triggers, progress by the API. A rating has a single entry, which is refreshed if the rating changes and removed if the rating is deleted.

### `GET /activities?limit=20&offset=0`

No auth. **Not enveloped.** `limit` is at most 100.

```json
{ "activities": [ { "id": "act-1de7dd2e-…", "user_id": "55c072c6-…", "username": "reader1",
                    "activity_type": "comment", "manga_id": "ff02e6b4-…",
                    "manga_title": "A Place Further Than the Universe", "comment_text": "Same!",
                    "created_at": "…" } ],
  "total": 3, "limit": 2, "offset": 0 }
```

`activity_type` is one of `comment`, `rating`, `progress` or `list_add`. `chapter_number` and `rating` appear when they apply.

### `GET /activities/user/:userID`

Auth required. Same shape, filtered to one user.

---

## Leaderboards

All are public, take `limit` (≤ 100) and `offset`, and return `{ "type", "period", "entries": [...], "updated_at" }` as `data`.

| Endpoint | Ranking |
|---|---|
| `GET /leaderboards/manga` | Average rating (manga with at least one rating) |
| `GET /leaderboards/users` | Engagement score = completed × 10 + chapters read + ratings × 5 + comments × 3 |
| `GET /leaderboards/trending?days=7` | New ratings + new readers in the last `days` (7 = weekly, 30 = monthly). If there has been no activity at all, falls back to the best-rated manga |

```json
{ "success": true, "message": "most active users",
  "data": { "type": "most_active", "period": "all_time", "updated_at": "…",
            "entries": [ { "rank": 1, "user_id": "f8fba6da-…", "username": "reader2", "display_name": "Jane Bookworm",
                           "manga_completed": 2, "chapters_read": 270, "total_ratings": 0,
                           "total_comments": 0, "score": 290 } ] } }
```

Manga entries have the fields `rank`, `manga_id`, `title`, `cover_url`, `author`, `average_rating`, `total_ratings` and `total_readers`.

---

## Admin: chapter releases

### `POST /admin/manga/:id/chapters`

Requires a token with the `admin` role (seeded account `admin` / `admin123`); others get `403`. Records that a new chapter is out and announces it to the manga's **readers**: users who have it in their library with any status except `dropped`. Readers get it two ways:

- a [UDP](#udp-notifications-9091) `chapter_release` notification, delivered only to subscribers registered with those users' tokens;
- a `system` message in the WebSocket room `manga_<id>`.

```bash
curl -X POST localhost:8080/admin/manga/$MANGA_ID/chapters -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" -d '{"chapter": 381}'
```

`201`:

```json
{ "success": true, "message": "chapter released",
  "data": { "manga_id": "eb4ae4ed-…", "title": "Berserk", "previous_latest": 380, "chapter": 381, "notified_readers": 1 } }
```

`manga.total_chapters` becomes `chapter`. Errors: `404` for an unknown manga, `409` if the chapter isn't newer than `total_chapters`, `400` if `chapter` ≤ 0.

The same release happens automatically when a newer chapter is found on MangaDex:

- **One-off sync:** `go run ./cmd/data-cli sync-chapters [--link] [--dry-run] [--manga <id>] [--limit <n>]`. `--link` looks up MangaDex IDs by title for manga that don't have one yet and saves them.
- **Periodic sync in the API server:** set `chapters.sync_interval` (e.g. `"30m"`) in the config; it is `"0s"` (off) by default. Chapters are counted in `chapters.language` (default `en`).

---

## Chat rooms

Room IDs are free-form. The TUI uses `general` and `manga_<manga_id>`; the protocol bridge posts progress notices to the latter. A room is created in the database the first time someone sends a message in it.

### `GET /rooms/:room_id`

No auth. **Not enveloped.** Returns who is connected right now.

```json
{ "room_id": "general", "clients": ["reader1", "reader2"], "count": 2 }
```

### `GET /rooms/:room_id/messages?limit=50&offset=0`

No auth. Returns saved chat history, oldest first. `limit` is at most 200. Only user messages are saved; join, leave and system notices are not.

```json
{ "success": true, "message": "chat history",
  "data": { "messages": [ { "id": "…", "room_id": "general", "user_id": "…", "username": "reader1",
                            "content": "first", "is_edited": false, "is_deleted": false,
                            "created_at": "…", "updated_at": "…" } ],
            "total": 1, "limit": 50, "offset": 0, "has_more": false } }
```

---

## WebSocket chat

```bash
wscat -c "ws://localhost:8080/ws/chat?room_id=general&token=$TOKEN"
```

`room_id` is required. Authenticate with an `Authorization: Bearer` header or with `?token=`.

**Client → server:** `{ "content": "hello" }`. Surrounding whitespace is trimmed and empty messages are ignored. Content is capped at 2000 characters; a frame larger than 8 KB closes the connection. Any `type` the client sends is ignored.

**Server → client:** every member of the room receives:

```json
{ "user_id": "…", "username": "reader1", "content": "hello", "message": "hello",
  "timestamp": 1790733468, "type": "message", "room_id": "general" }
```

| `type` | Sent when |
|---|---|
| `join` / `leave` | Someone connects to or disconnects from the room |
| `message` | A user sends a message |
| `system` | The server posts a notice, e.g. `"reader1 is now on chapter 13 of One Piece"` from a progress update |

`timestamp` is in Unix seconds. `message` duplicates `content` for older clients. The server pings every 54 s; a client that doesn't answer within 60 s is dropped.

---

## TCP progress sync (9090)

This is a stream of newline-delimited JSON. Every line a client sends is relayed to **all** connected clients, including the sender. The API server's protocol bridge is one of these clients: it sends a line for every progress update.

```json
{"user_id":"55c072c6-…","manga_id":"ff02e6b4-…","chapter":13,"timestamp":1790755425}
```

```bash
nc localhost 9090          # watch updates
mangahub debug listen      # CLI equivalent
```

Lines that aren't valid JSON are ignored, and the connection stays open.

---

## UDP notifications (9091)

Each command is one datagram of plain text:

| Send | Reply | Effect |
|---|---|---|
| `REGISTER` | `REGISTERED` | Subscribe the sending address anonymously (general broadcasts only) |
| `REGISTER <jwt>` | `REGISTERED`, or `ERROR invalid token` | Subscribe as the token's user: also receive notifications targeted at you (new chapters of manga in your library). The token is the one from `/auth/login` |
| `UNREGISTER` | `UNREGISTERED` | Unsubscribe |
| `BROADCAST <json>` | – | Deliver `<json>` (a notification) to every subscriber, or, if it has `"user_ids": [...]`, only to subscribers registered as those users. `user_ids` is removed before delivery |

Notifications are JSON datagrams:

```json
{ "type": "progress_update", "manga_id": "ff02e6b4-…",
  "message": "reader1 is now on chapter 13 of A Place Further Than the Universe",
  "timestamp": 1790755425, "manga_title": "A Place Further Than the Universe",
  "chapter": 13, "username": "reader1" }
```

A new chapter, sent only to that manga's readers:

```json
{ "type": "chapter_release", "manga_id": "eb4ae4ed-…", "message": "Chapter 386 of Berserk is out!",
  "timestamp": 1790906721, "manga_title": "Berserk", "chapter": 386 }
```

`type` is `progress_update` (sent to everyone on every progress update), `chapter_release` (a [chapter release](#admin-chapter-releases); also `mangahub debug notify` and `udp-server -demo`, which go to everyone), or `system`. `manga_title`, `chapter` and `username` are optional. Subscribe from the same socket you read from (a connected UDP socket), because notifications go to the registered address.

**Heartbeat:** a subscription expires if the server hears no `REGISTER` from that address for `udp.subscriber_ttl` (default 5 minutes). Re-send the same `REGISTER` (it's idempotent) about once a minute, and ignore the `REGISTERED` replies. The TUI and `internal/udp.Client` do this. It also re-subscribes you after a UDP server restart. An expired token in the heartbeat gets `ERROR invalid token`; register again anonymously or with a fresh token.

---

## gRPC (9092)

The service is `mangahub.v1.MangaService`, defined in `proto/manga.proto`. Server reflection is enabled:

```bash
grpcurl -plaintext localhost:9092 list
grpcurl -plaintext -d '{"manga_id":"<id>"}' localhost:9092 mangahub.v1.MangaService/GetManga
```

| RPC | Request | Notes |
|---|---|---|
| `GetManga` | `{ manga_id }` | `NOT_FOUND` for an unknown ID, `INVALID_ARGUMENT` if the ID is empty |
| `SearchManga` | `{ query, genres[], status, limit, offset }` | Same matching as `GET /manga`; `limit` is 1–100 |
| `UpdateProgress` | `{ user_id, manga_id, current_chapter, status }` | **Needs a token** (below). `user_id` may be a UUID or a username. `status` defaults to `reading`. Returns `INVALID_ARGUMENT` for a negative chapter or unknown status, and `NOT_FOUND` for an unknown user or manga |

**Authentication:** `UpdateProgress` needs the JWT from [`POST /auth/login`](#post-authlogin) as metadata `authorization: Bearer <token>`. Users can only update their own progress (`user_id` must be them); admins can update anyone's. `GetManga` and `SearchManga` are public.

```bash
TOKEN=$(curl -s -X POST localhost:8080/auth/login -H "Content-Type: application/json" \
  -d '{"username":"reader1","password":"password123"}' | jq -r .data.token)
grpcurl -plaintext -H "authorization: Bearer $TOKEN" \
  -d '{"user_id":"reader1","manga_id":"<id>","current_chapter":50,"status":"reading"}' \
  localhost:9092 mangahub.v1.MangaService/UpdateProgress
```

| Status | When |
|---|---|
| `UNAUTHENTICATED` | No `authorization` metadata, not `Bearer`, or an expired/forged token |
| `PERMISSION_DENIED` | A non-admin updating someone else's progress |

**Audit mode:** when the call carries the metadata `x-mangahub-audit: true` (as the API's protocol bridge sends it, together with the user's token), `UpdateProgress` only verifies and logs the update, then returns the stored progress **without writing**. The HTTP API has already saved the update, and writing it a second time could roll back a newer one. Calls without this metadata write as usual.

There is no TLS (`-plaintext`); keep port 9092 on a private network.
