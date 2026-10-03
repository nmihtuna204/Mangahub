# MangaHub Live Demo

## Demo Flow (15 minutes)

### 1. Authentication (2 min)

```bash
# CLI login
mangahub auth login --username admin

# Verify token
mangahub config show
```

### 2. Manga Discovery (2 min)

```bash
# Search manga
mangahub manga search "one piece"

# View details
curl http://localhost:8080/manga/one-piece
```

### 3. Library Management (2 min)

```bash
# Add to library
mangahub library add --manga-id one-piece --status reading

# View library
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/users/library
```

### 4. **MAIN DEMO: Protocol Integration** (5 min)

Manga IDs are UUIDs. Pick one first (e.g. One Piece) and use it below as `$MANGA_ID`:
```bash
mangahub manga search "One Piece" --limit 1   # copy the ID line
```

**Terminal A: Monitor TCP broadcasts**
```bash
nc localhost 9090
# Will show incoming progress updates
```

**Terminal B: Monitor UDP notifications**
```powershell
./test-udp-simple.ps1
# Will show a "progress_update" notification for every progress change
# (start the UDP server with -demo to also get a sample notification every 10s)
```

**Terminal C: Monitor WebSocket chat**
```bash
# Browsers/wscat can't send headers on a WebSocket upgrade, so pass the JWT as ?token=
wscat -c "ws://localhost:8080/ws/chat?room_id=manga_$MANGA_ID&token=$TOKEN"
# Will show chat messages, plus a system notice for each progress update on this manga
```

**Terminal D: Make HTTP update**
```bash
mangahub progress update --manga-id $MANGA_ID --chapter 100 --rating 9
```

**Result:** All 5 protocols trigger simultaneously! 🎉 (the gRPC server logs an `AUDIT progress` line)

### 4b. **New chapter → only its readers are notified**

Terminal B above registered anonymously, so it only sees general broadcasts. A logged-in TUI (or `mangahub debug listen`) registers with its token and also gets chapter releases for manga in **its** library:

```bash
# reader1 has $MANGA_ID in their library (step 4); release its next chapter as admin
ADMIN_TOKEN=$(curl -s -X POST localhost:8080/auth/login -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r .data.token)
curl -X POST localhost:8080/admin/manga/$MANGA_ID/chapters -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" -d '{"chapter": 1101}'   # any number above the current total
```

reader1's TUI shows "📖 … Chapter 1101 released!"; other users see nothing, and the manga's chat room gets a notice.

Same thing from real data: `go run ./cmd/data-cli sync-chapters --link` looks every manga up on MangaDex and releases any newer chapter (add `--dry-run` to preview).

### 5. CLI Capabilities (2 min)

```bash
mangahub --help
mangahub auth --help
mangahub manga --help
mangahub progress --help
```

---

## Key Talking Points

1. **Multi-Protocol Architecture**: Single update triggers 5 different protocols
2. **Real-time Synchronization**: TCP ensures all clients stay in sync
3. **Push Notifications**: UDP delivers chapter releases without polling
4. **Community Features**: WebSocket enables live chat discussions
5. **Internal Services**: gRPC provides efficient inter-service communication
6. **CLI Integration**: Desktop users can interact without web browser
7. **Scalability**: Handles concurrent connections across all protocols

---

## Success Criteria

✅ User can register and login
✅ User can search and add manga
✅ Update progress via HTTP
✅ TCP broadcasts reach all connected clients
✅ UDP notifications received by subscribers
✅ WebSocket chat messages appear in real-time
✅ gRPC service responds to queries
✅ CLI tool works for all operations
