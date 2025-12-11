# How to Run MangaHub

This guide will help you run all functionality of the MangaHub project.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Running All Services](#running-all-services)
- [Testing All Features](#testing-all-features)
- [Using the CLI Tool](#using-the-cli-tool)
- [Manual Testing](#manual-testing)
- [Docker Setup](#docker-setup)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Software

1. **Go** (version 1.21 or higher)
   ```powershell
   go version
   ```

2. **Git**
   ```powershell
   git --version
   ```

3. **(Optional) Docker & Docker Compose** - for containerized deployment
   ```powershell
   docker --version
   docker-compose --version
   ```

### Clone the Repository

```powershell
git clone https://github.com/nmihtuna204/Mangahub.git
cd Mangahub
```

### Install Dependencies

```powershell
go mod download
go mod tidy
```

---

## Quick Start

### Option 1: Run Everything with PowerShell Script

The easiest way to test all features:

```powershell
.\test-all.ps1
```

This script will:
- Check if all servers are running
- Run comprehensive tests for all protocols
- Display results with colored output

### Option 2: Run All Servers Manually

Open **5 separate PowerShell terminals** and run:

**Terminal 1 - HTTP API Server (+ WebSocket)**
```powershell
go run ./cmd/api-server/main.go
```

**Terminal 2 - TCP Sync Server**
```powershell
go run ./cmd/tcp-server/main.go
```

**Terminal 3 - UDP Notification Server**
```powershell
go run ./cmd/udp-server/main.go
```

**Terminal 4 - gRPC Service**
```powershell
go run ./cmd/grpc-server/main.go
```

**Terminal 5 - CLI Tool**
```powershell
go run ./cmd/cli/main.go
```

---

## Running All Services

### Start All Servers

#### Method 1: Using Make (if you have Make installed)

```powershell
make all
```

#### Method 2: Build and Run Each Server

**Build all executables:**
```powershell
# Create bin directory
New-Item -ItemType Directory -Force -Path bin

# Build all servers
go build -o bin/api-server.exe ./cmd/api-server
go build -o bin/tcp-server.exe ./cmd/tcp-server
go build -o bin/udp-server.exe ./cmd/udp-server
go build -o bin/grpc-server.exe ./cmd/grpc-server
go build -o bin/cli.exe ./cmd/cli
```

**Run the servers:**
```powershell
# Terminal 1
.\bin\api-server.exe

# Terminal 2
.\bin\tcp-server.exe

# Terminal 3
.\bin\udp-server.exe

# Terminal 4
.\bin\grpc-server.exe
```

### Verify Services Are Running

Check if all ports are listening:

```powershell
# Check HTTP API
netstat -an | Select-String "8080"

# Check TCP Sync
netstat -an | Select-String "9090"

# Check UDP Notifier
netstat -an | Select-String "9091"

# Check gRPC Service
netstat -an | Select-String "9092"
```

Or use the health check script:
```powershell
# Quick health check
curl http://localhost:8080/health
```

---

## Testing All Features

### 1. Run Complete Test Suite

```powershell
.\test-all.ps1
```

### 2. Test Individual Protocols

#### Test HTTP REST API
```powershell
.\test-api.ps1
```

Features tested:
- User registration
- User login (JWT authentication)
- Manga search and retrieval
- Progress tracking (CRUD operations)
- Protected endpoints

#### Test TCP Progress Sync
```powershell
.\test-tcp.ps1
```

Features tested:
- TCP client connection
- Real-time progress synchronization
- Broadcasting to multiple clients
- Concurrent connection handling

#### Test UDP Notifications
```powershell
.\test-udp-simple.ps1
```

Features tested:
- UDP notification delivery
- Subscription management
- Push notifications for manga updates

#### Test WebSocket Chat
```powershell
.\test-websocket.ps1
```

Features tested:
- WebSocket connection establishment
- Real-time message broadcasting
- Multiple users in chat rooms
- Connection management

#### Test gRPC Service
```powershell
.\test-grpc.ps1
```

Features tested:
- gRPC client-server communication
- Manga search via gRPC
- Manga details retrieval
- Internal service communication

### 3. Run Integration Tests

```powershell
go test -v ./test/integration_test.go
```

### 4. Run Unit Tests

```powershell
# Test all packages
go test -v ./...

# Test specific package
go test -v ./internal/auth
go test -v ./internal/manga
go test -v ./internal/progress
```

---

## Using the CLI Tool

### Build the CLI

```powershell
go build -o bin/mangahub.exe ./cmd/cli
```

### CLI Commands

#### 1. Register a New User
```powershell
.\bin\mangahub.exe auth register
```

#### 2. Login
```powershell
.\bin\mangahub.exe auth login
```

#### 3. Search for Manga
```powershell
.\bin\mangahub.exe manga search
# Enter search term when prompted
```

#### 4. View Your Library
```powershell
.\bin\mangahub.exe library list
```

#### 5. Add Manga to Library
```powershell
.\bin\mangahub.exe library add
# Enter manga ID when prompted
```

#### 6. Update Reading Progress
```powershell
.\bin\mangahub.exe progress update
# Enter manga ID and chapter when prompted
```

#### 7. Show Configuration
```powershell
.\bin\mangahub.exe config show
```

### CLI Interactive Mode

Simply run the CLI without arguments for interactive mode:
```powershell
.\bin\mangahub.exe
```

Then follow the menu prompts to navigate through features.

---

## Manual Testing

### Test HTTP API with cURL

#### Register User
```powershell
curl -X POST http://localhost:8080/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{\"username\":\"testuser\",\"email\":\"test@example.com\",\"password\":\"password123\"}'
```

#### Login
```powershell
curl -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{\"email\":\"test@example.com\",\"password\":\"password123\"}'
```

#### Search Manga
```powershell
curl -X GET "http://localhost:8080/api/v1/manga/search?q=naruto" `
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

#### Get Reading Progress
```powershell
curl -X GET http://localhost:8080/api/v1/progress `
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Test TCP with PowerShell Script

```powershell
.\tcp-client.ps1
```

### Test UDP with PowerShell Script

```powershell
.\udp-client.ps1
```

### Test WebSocket

Use the provided test script or a WebSocket client:
```powershell
# Using test script
.\test-websocket.ps1

# Or use a browser-based WebSocket client
# Connect to: ws://localhost:8080/ws/chat?user_id=123&manga_id=456
```

---

## Docker Setup

### Build and Run with Docker Compose

#### Start All Services
```powershell
docker-compose up -d
```

#### View Logs
```powershell
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f api-server
docker-compose logs -f tcp-server
docker-compose logs -f udp-server
docker-compose logs -f grpc-server
```

#### Stop All Services
```powershell
docker-compose down
```

#### Rebuild Services
```powershell
docker-compose up -d --build
```

### Build Individual Docker Images

```powershell
# API Server
docker build -t mangahub-api -f Dockerfile .

# TCP Server
docker build -t mangahub-tcp -f docker/Dockerfile.tcp .

# UDP Server
docker build -t mangahub-udp -f docker/Dockerfile.udp .

# gRPC Server
docker build -t mangahub-grpc -f docker/Dockerfile.grpc .
```

### Run Individual Containers

```powershell
# API Server
docker run -d -p 8080:8080 --name mangahub-api mangahub-api

# TCP Server
docker run -d -p 9090:9090 --name mangahub-tcp mangahub-tcp

# UDP Server
docker run -d -p 9091:9091/udp --name mangahub-udp mangahub-udp

# gRPC Server
docker run -d -p 9092:9092 --name mangahub-grpc mangahub-grpc
```

---

## Troubleshooting

### Common Issues

#### 1. Port Already in Use

**Error:** `bind: address already in use`

**Solution:**
```powershell
# Find process using the port (e.g., 8080)
netstat -ano | findstr :8080

# Kill the process
taskkill /PID <PID> /F
```

#### 2. Database File Locked

**Error:** `database is locked`

**Solution:**
```powershell
# Stop all services
# Delete the database file
Remove-Item -Force data/mangahub.db

# Restart services (database will be recreated)
```

#### 3. Module Not Found

**Error:** `cannot find package`

**Solution:**
```powershell
go mod download
go mod tidy
```

#### 4. Permission Denied

**Error:** `permission denied`

**Solution:** Run PowerShell as Administrator

#### 5. gRPC Connection Failed

**Error:** `could not connect to gRPC server`

**Solution:**
```powershell
# Ensure gRPC server is running
go run ./cmd/grpc-server/main.go

# Check if port 9092 is open
netstat -an | Select-String "9092"
```

#### 6. WebSocket Connection Failed

**Error:** `websocket: bad handshake`

**Solution:**
- Ensure API server is running
- Check WebSocket URL format: `ws://localhost:8080/ws/chat?user_id=123&manga_id=456`
- Verify parameters are provided

### Logs and Debugging

#### Enable Debug Logging

Set environment variable:
```powershell
$env:LOG_LEVEL="debug"
go run ./cmd/api-server/main.go
```

#### Check Server Logs

Each server outputs logs to stdout. Check the terminal where you started the server.

#### Database Inspection

```powershell
# Install sqlite3 if not already installed
# Then inspect the database
sqlite3 data/mangahub.db

# List tables
.tables

# Query users
SELECT * FROM users;

# Query manga
SELECT * FROM manga LIMIT 10;

# Query progress
SELECT * FROM progress;

# Exit
.quit
```

---

## Performance Testing

### Load Testing

Run the load test script:
```powershell
.\test\load_test.sh
```

Or manually:
```powershell
# Install Apache Bench (ab) or use similar tool
# Test API endpoint
ab -n 1000 -c 10 http://localhost:8080/api/v1/manga/1
```

### Monitor Resource Usage

```powershell
# Monitor CPU and Memory
Get-Process | Where-Object {$_.ProcessName -like "*server*"} | Select-Object ProcessName, CPU, WS
```

---

## Summary of Ports

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| HTTP API + WebSocket | 8080 | HTTP/WS | REST API and WebSocket chat |
| TCP Sync Server | 9090 | TCP | Progress synchronization |
| UDP Notifier | 9091 | UDP | Push notifications |
| gRPC Service | 9092 | gRPC | Internal service communication |

---

## Next Steps

1. **Start all servers** in separate terminals
2. **Run the test suite**: `.\test-all.ps1`
3. **Try the CLI tool**: `.\bin\mangahub.exe`
4. **Test individual protocols** with provided test scripts
5. **Check the demo guide**: See `demo/DEMO.md` for presentation workflow

---

## Additional Resources

- **API Documentation**: See `docs/` directory
- **Demo Guide**: `demo/DEMO.md`
- **Phase Summaries**: `PHASE*_SUMMARY.md` files
- **Docker Guide**: `DOCKER.md`
- **Deployment Guide**: `DEPLOYMENT.md`
- **Known Issues**: `KNOWN_ISSUES.md`

---

## Quick Reference Commands

```powershell
# Start all servers (5 terminals needed)
go run ./cmd/api-server/main.go    # Terminal 1
go run ./cmd/tcp-server/main.go    # Terminal 2
go run ./cmd/udp-server/main.go    # Terminal 3
go run ./cmd/grpc-server/main.go   # Terminal 4

# Run complete test suite
.\test-all.ps1

# Build CLI
go build -o bin/mangahub.exe ./cmd/cli

# Run CLI
.\bin\mangahub.exe

# Docker
docker-compose up -d
docker-compose logs -f
docker-compose down

# Unit tests
go test -v ./...

# Integration tests
go test -v ./test/integration_test.go
```

---

**Happy Testing! 🚀**

For questions or issues, please check `KNOWN_ISSUES.md` or create a GitHub issue.
