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

**Terminal A: Monitor TCP broadcasts**
```bash
nc localhost 9090
# Will show incoming progress updates
```

**Terminal B: Monitor UDP notifications**
```powershell
./test-udp-simple.ps1
# Will show chapter notifications
```

**Terminal C: Monitor WebSocket chat**
```bash
wscat -c "ws://localhost:8080/ws/chat?room_id=one-piece"
# Will show chat messages
```

**Terminal D: Make HTTP update**
```bash
mangahub progress update --manga-id one-piece --chapter 100 --rating 9
```

**Result:** All 5 protocols trigger simultaneously! 🎉

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
