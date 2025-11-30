# MangaHub - Net-Centric Programming Project

A comprehensive manga tracking system demonstrating all five network communication protocols: **HTTP, TCP, UDP, WebSocket, and gRPC**.

## 📚 Overview

MangaHub is a real-time manga synchronization platform built with Go, showcasing practical implementation of network programming concepts through an integrated multi-protocol architecture.

### Core Features

- **User Management**: Registration, authentication (JWT), profile management
- **Manga Database**: Search, browse, detailed information
- **Reading Progress Tracking**: Track current chapter, ratings, status
- **Real-time Synchronization**: TCP broadcast to connected clients
- **Chapter Notifications**: UDP push notifications to subscribers
- **Community Chat**: WebSocket real-time discussions
- **Internal Services**: gRPC for inter-service communication
- **CLI Tool**: Command-line interface for all operations

---

## ✅ All Phases Complete (10/10)

- ✅ **Phase 1**: Foundation & Database
- ✅ **Phase 2**: HTTP REST API & Authentication
- ✅ **Phase 3**: TCP Progress Sync Server
- ✅ **Phase 4**: UDP Notification System
- ✅ **Phase 5**: WebSocket Chat System
- ✅ **Phase 6**: gRPC Internal Service
- ✅ **Phase 7**: Protocol Integration & Cross-Communication
- ✅ **Phase 8**: CLI Tool
- ✅ **Phase 9**: Testing & Bug Fixes
- ✅ **Phase 10**: Documentation & Demo Prep

## 🏗️ System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     CLIENT LAYER                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  Web Browser │  │  CLI Tool    │  │ Mobile App   │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
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
           │   Protocol Bridge   │ (Integration point)
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
           │  (~/.mangahub/      │
           │   data.db)          │
           └─────────────────────┘
```

---

## 🚀 Quick Start

### Prerequisites

- Go 1.19 or later
- SQLite 3.x
- Git

### Installation

```bash
git clone https://github.com/nmihtuna204/Mangahub.git
cd Mangahub/mangahub
go mod tidy
```

### Start All Services

```bash
# Terminal 1: HTTP API Server
go run cmd/api-server/main.go

# Terminal 2: TCP Sync Server
go run cmd/tcp-server/main.go

# Terminal 3: UDP Notifier
go run cmd/udp-server/main.go

# Terminal 4: gRPC Service
go run cmd/grpc-server/main.go
```

### Build CLI Tool

```bash
go build -o bin/mangahub ./cmd/cli
./bin/mangahub --help
```

---

## 📋 API Documentation

### Authentication

**Register**
```http
POST /auth/register
Content-Type: application/json

{
  "username": "reader1",
  "email": "reader@example.com",
  "password": "secure123"
}
```

**Login**
```http
POST /auth/login
Content-Type: application/json

{
  "username": "reader1",
  "password": "secure123"
}

Response:
{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs..."
  }
}
```

### Manga Operations

**Search Manga**
```http
GET /manga?q=one+piece&limit=10&offset=0
```

**Get Manga Details**
```http
GET /manga/one-piece
```

### Library Management

**Add to Library**
```http
POST /users/library
Authorization: Bearer {token}
Content-Type: application/json

{
  "manga_id": "one-piece",
  "current_chapter": 0,
  "status": "reading"
}
```

**Get User Library**
```http
GET /users/library
Authorization: Bearer {token}
```

**Update Reading Progress** ⭐ *Triggers all 5 protocols!*
```http
PUT /users/progress
Authorization: Bearer {token}
Content-Type: application/json

{
  "manga_id": "one-piece",
  "current_chapter": 100,
  "status": "reading",
  "rating": 9
}
```

### WebSocket Chat

**Connect to Chat Room**
```javascript
WebSocket ws://localhost:8080/ws/chat?room_id=one-piece
Authorization: Bearer {token}

Send messages:
{
  "message": "This manga is amazing!"
}

Receive broadcasts:
{
  "user_id": "user123",
  "username": "reader1",
  "message": "This manga is amazing!",
  "timestamp": 1700000000,
  "type": "message"
}
```

---

## 🔄 Protocol Integration Demo

When user updates progress via HTTP:

1. **HTTP** - REST API receives update request
2. **🔌 Bridge** - Triggered on progress update
3. **TCP** - Broadcast to sync clients: `{"user_id":"...", "manga_id":"...", "chapter":100}`
4. **UDP** - Send notification: `{"type":"chapter_release", "message":"New progress update"}`
5. **WebSocket** - Notify chat room members in real-time
6. **gRPC** - Log to audit service via RPC call

**Result:** Single API call triggers all 5 protocols!

---

## 📊 Database Schema

### Users Table
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Manga Table
```sql
CREATE TABLE manga (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    author TEXT,
    artist TEXT,
    status TEXT,
    genres TEXT,
    total_chapters INTEGER,
    rating REAL,
    year INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Reading Progress Table
```sql
CREATE TABLE reading_progress (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    manga_id TEXT NOT NULL,
    current_chapter INTEGER,
    status TEXT,
    rating INTEGER,
    last_read_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, manga_id),
    FOREIGN KEY(user_id) REFERENCES users(id),
    FOREIGN KEY(manga_id) REFERENCES manga(id)
);
```

---

## 🧪 Testing

```bash
# Run unit tests
go test -v ./internal/auth

# Run all tests
go test -v ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## 🎓 Learning Outcomes

This project demonstrates:

- **Network Protocols**: Practical implementation of HTTP, TCP, UDP, WebSocket, and gRPC
- **Concurrency**: Goroutines, channels, and synchronization patterns
- **Database Design**: SQLite schema design and query optimization
- **API Design**: RESTful principles and error handling
- **Real-time Communication**: Broadcasting and event-driven architecture
- **CLI Development**: Cobra framework for command-line tools
- **Testing**: Unit and integration testing strategies

---

## 📝 License

MIT License - See LICENSE file for details

---

## 👨‍💻 Authors

- Your Name
- Collaborators

---

## 📞 Support

For issues, please open a GitHub issue or contact the development team.

