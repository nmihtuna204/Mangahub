# TUI Feature Status

*Re-checked against the code on 2026-10-02. An earlier version of this file listed rating, comments, chat and several detail-view actions as missing; they are all implemented.*

Run with `go run ./cmd/tui` (the API, TCP, UDP and gRPC servers must be running).

## Implemented

| Area | What works | Keys / where |
|---|---|---|
| Dashboard | Trending + top-rated manga, recent activity | Home view |
| Search | Full-text search (word prefixes, ranked by relevance) with paging | `/` |
| Browse | Genre grid; each genre lists matching manga (`?genre=`) | Browse view |
| Manga detail | Info, genres, rating summary, chapter strip around your current chapter | `Enter` on any manga |
| | Add to library | `a` |
| | Read next chapter (+1, up to the known total) | `r` |
| | Rate 1–10 with an optional review (modal) | `R` |
| | Comments view: list, post, like | `C`, then `c` post / `l` like |
| | Join the manga's chat room (`manga_<id>`) | `c` |
| Library | Tabs per status (Reading, Plan, Completed, On-Hold, Dropped) with progress bars | `Tab` / `Shift+Tab` |
| | Set the current chapter in an input box (digits only, shows the total) | `u`, `Enter` save, `Esc` cancel |
| | One chapter forward / back | `+` / `-` |
| | Change status | `1`–`5` |
| | Toggle favorite | `f` |
| | Remove, after a y/n confirmation | `d` |
| Chat | WebSocket rooms (`general`, `manga_<id>`), auto-reconnect with backoff, last 50 messages loaded when you enter a room, progress/chapter notices from the server | chat key / palette |
| Notifications | Toasts for UDP notifications: everyone's progress updates, and new chapters of manga in **your** library (registered with your login token) | automatic after login |
| Activity feed | Recent progress, ratings, comments and public-list additions | Activity view |
| Auth | Login, register, logout (stops chat and notifications) | `L` / palette |
| Command palette | Jump to any view or action | `Ctrl+P` |

## Still missing

- **Comment replies and unlike:** the API supports both (`parent_id`, `DELETE /comments/:id/like`); the comments view only posts top-level comments and likes.
- **Favorite marker in the library list:** favorites can be toggled with `f`, but rows don't show a ★.
- **Browse filters:** no type (manga/manhwa/manhua), status, or sort controls. The API supports `type`, `status`, `sort_by` and `order` on `GET /manga`.
- **Custom lists:** the API exists (`/lists`, see `docs/API.md`), the TUI has no view for it yet.
- **Chapter titles:** the database stores only a chapter count per manga, so the detail view shows chapter numbers, not titles.
- **Chat presence:** there's no "who's online" list or typing indicator. `GET /rooms/:room_id` returns the connected users if a view wants them.
- **Reading beyond the known total:** `r` stops at `total_chapters`. For ongoing series that total grows when a new chapter is released (admin `POST /admin/manga/:id/chapters`, or the MangaDex sync).

## Tests

The TUI's network and API code is tested in `internal/tui/network`, `internal/tui/api` and against the full in-process system in `test/clients_test.go`. The library and chat views have unit tests in `internal/tui/views`. Rendering is not tested.
