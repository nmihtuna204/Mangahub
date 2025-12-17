# MangaHub Testing Guide

Comprehensive testing documentation for all features and protocols in the MangaHub project.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Test Suite](#quick-test-suite)
- [Test Cases Overview](#test-cases-overview)
- [Manual Testing Guide](#manual-testing-guide)
- [cURL Testing Commands](#curl-testing-commands)
- [Protocol-Specific Tests](#protocol-specific-tests)
- [Integration Testing](#integration-testing)
- [Troubleshooting Tests](#troubleshooting-tests)

---

## Prerequisites

### Required Servers Running

Before running any tests, ensure all servers are started:

```powershell
# Terminal 1 - HTTP API Server
go run ./cmd/api-server/main.go

# Terminal 2 - TCP Sync Server
go run ./cmd/tcp-server/main.go

# Terminal 3 - UDP Notification Server
go run ./cmd/udp-server/main.go

# Terminal 4 - gRPC Service
go run ./cmd/grpc-server/main.go
```

### Required Tools

- **Go** (1.21+)
- **cURL** (for HTTP testing)
- **grpcurl** (for gRPC testing) - Install: `go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest`
- **PowerShell** (for test scripts)

### Default Test Credentials

The database is seeded with these test accounts:

| Username | Password | Role | Email |
|----------|----------|------|-------|
| `admin` | `admin123` | admin | admin@mangahub.com |
| `reader1` | `password123` | user | reader1@example.com |
| `reader2` | `password123` | user | reader2@example.com |
| `mangafan` | `password123` | user | fan@example.com |

---

## Quick Test Suite

### Run All Tests

```powershell
.\test-all.ps1
```

This script runs:
- Unit tests
- HTTP API tests
- TCP connection tests
- UDP notification tests
- WebSocket chat tests
- gRPC service tests
- Integration tests

---

## Test Cases Overview

### 1. HTTP REST API Test Cases

#### TC-HTTP-001: User Registration
**Objective**: Verify new user can register  
**Expected**: User created with unique username and email  
**Script**: `.\test-api.ps1` (Test 1)

#### TC-HTTP-002: User Login
**Objective**: Verify user authentication returns JWT token  
**Expected**: Valid JWT token returned  
**Script**: `.\test-api.ps1` (Test 2)

#### TC-HTTP-003: List All Manga
**Objective**: Verify manga catalog retrieval  
**Expected**: Returns paginated manga list  
**Script**: `.\test-api.ps1` (Test 3)

#### TC-HTTP-004: Get Manga by ID
**Objective**: Verify single manga details retrieval  
**Expected**: Returns complete manga information  
**Script**: `.\test-api.ps1` (Test 4)

#### TC-HTTP-005: Search Manga
**Objective**: Verify manga search functionality  
**Expected**: Returns matching manga by title/author  
**Script**: `.\test-api.ps1` (Test 6)

#### TC-HTTP-006: Add to Library (Protected)
**Objective**: Verify authenticated user can add manga to library  
**Expected**: Manga added with initial progress  
**Script**: `.\test-api.ps1` (Test 5)

#### TC-HTTP-007: Update Reading Progress (Protected)
**Objective**: Verify progress tracking update  
**Expected**: Chapter progress updated  
**Script**: `.\test-api.ps1` (Test 7)

#### TC-HTTP-008: Get User Progress (Protected)
**Objective**: Verify user's reading progress retrieval  
**Expected**: Returns all user's progress entries  
**Script**: `.\test-api.ps1` (Test 8)

#### TC-HTTP-009: Health Check
**Objective**: Verify server health endpoint  
**Expected**: Returns `{"status":"healthy"}`  
**Script**: `.\test-all.ps1` (Test 2)

---

### 2. TCP Progress Sync Test Cases

#### TC-TCP-001: Server Connection
**Objective**: Verify TCP server accepts connections  
**Expected**: Client connects successfully to port 9090  
**Script**: `.\test-tcp.ps1` (Test 1)

#### TC-TCP-002: Single Client Message
**Objective**: Verify client can send progress update  
**Expected**: Message sent without error  
**Script**: `.\test-tcp.ps1` (Test 2)

#### TC-TCP-003: Multiple Clients Broadcast
**Objective**: Verify progress broadcast to all connected clients  
**Expected**: All clients receive broadcast messages  
**Script**: `.\test-tcp.ps1` (Test 3)

#### TC-TCP-004: Concurrent Connections
**Objective**: Verify server handles multiple simultaneous connections  
**Expected**: All clients connected and receive messages  
**Script**: `.\test-tcp.ps1` (Test 3)

#### TC-TCP-005: Message Format Validation
**Objective**: Verify JSON message parsing  
**Expected**: Valid JSON accepted, invalid rejected  
**Script**: `.\test-tcp.ps1` (Test 4)

---

### 3. UDP Notification Test Cases

#### TC-UDP-001: Client Registration
**Objective**: Verify client can register for notifications  
**Expected**: Server responds with "REGISTERED"  
**Script**: `.\test-udp-simple.ps1` (Test 1)

#### TC-UDP-002: Receive Notifications
**Objective**: Verify client receives broadcast notifications  
**Expected**: Notifications received within timeout  
**Script**: `.\test-udp-simple.ps1` (Test 2)

#### TC-UDP-003: Multiple Subscribers
**Objective**: Verify broadcast to multiple clients  
**Expected**: All registered clients receive notifications  
**Script**: `.\test-udp.ps1` (Test 3)

#### TC-UDP-004: Notification Content
**Objective**: Verify notification contains correct data  
**Expected**: Notification includes manga_id, chapter, message  
**Script**: `.\test-udp-simple.ps1` (Test 2)

---

### 4. WebSocket Chat Test Cases

#### TC-WS-001: Authentication Required
**Objective**: Verify WebSocket requires valid JWT  
**Expected**: Connection rejected without valid token  
**Script**: `.\test-websocket.ps1` (Test 1)

#### TC-WS-002: Room Connection
**Objective**: Verify client can join chat room  
**Expected**: Successfully connected to room  
**Script**: `.\test-websocket.ps1` (Test 3)

#### TC-WS-003: Send Message
**Objective**: Verify client can send chat message  
**Expected**: Message sent successfully  
**Script**: `.\test-websocket.ps1` (Test 3)

#### TC-WS-004: Receive Broadcast
**Objective**: Verify messages broadcast to all room participants  
**Expected**: All clients in room receive message  
**Script**: `.\test-websocket.ps1` (Test 3)

#### TC-WS-005: Room Info
**Objective**: Verify room information endpoint  
**Expected**: Returns room participant count  
**Script**: `.\test-websocket.ps1` (Test 2)

#### TC-WS-006: Multiple Rooms
**Objective**: Verify isolation between different manga rooms  
**Expected**: Messages only broadcast within same room  
**Script**: `.\test-websocket.ps1` (Test 4)

---

### 5. gRPC Service Test Cases

#### TC-GRPC-001: Service Discovery
**Objective**: Verify gRPC service is available  
**Expected**: MangaService listed in reflection  
**Script**: `.\test-grpc.ps1` (Test 1)

#### TC-GRPC-002: GetManga RPC
**Objective**: Verify GetManga retrieves manga by ID  
**Expected**: Returns complete manga details  
**Script**: `.\test-grpc.ps1` (Test 2)

#### TC-GRPC-003: SearchManga RPC
**Objective**: Verify SearchManga with filters  
**Expected**: Returns filtered manga results  
**Script**: `.\test-grpc.ps1` (Test 3)

#### TC-GRPC-004: UpdateProgress RPC
**Objective**: Verify UpdateProgress updates reading progress  
**Expected**: Progress updated and confirmed  
**Script**: `.\test-grpc.ps1` (Test 4)

---

### 6. Integration Test Cases

#### TC-INT-001: Full User Flow
**Objective**: Complete user journey from registration to reading  
**Expected**: All operations succeed in sequence  
**Script**: `.\test-integration.ps1`

#### TC-INT-002: Cross-Protocol Sync
**Objective**: Verify TCP sync triggers after HTTP update  
**Expected**: Progress update broadcast via TCP  
**Script**: `.\test-integration.ps1`

#### TC-INT-003: Notification Trigger
**Objective**: Verify UDP notification sent on manga update  
**Expected**: Subscribers receive notification  
**Script**: `.\test-integration.ps1`

---

## Manual Testing Guide

### 1. HTTP REST API Manual Testing

#### Test: User Registration

1. Open a terminal/PowerShell
2. Run registration command:
```powershell
curl -X POST http://localhost:8080/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{"username":"manualtest","email":"manual@test.com","password":"Test1234"}'
```
3. **Expected Result**: Status 201, returns user object with ID
4. **Verify**: Username is unique (trying again should fail with 409)

#### Test: User Login

1. Run login command:
```powershell
curl -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{"username":"manualtest","password":"Test1234"}'
```
2. **Expected Result**: Status 200, returns JWT token
3. **Verify**: Copy the token for next tests

#### Test: List Manga

```powershell
curl -X GET "http://localhost:8080/api/v1/manga?limit=5&offset=0"
```
**Expected**: Returns array of 5 manga with pagination info

#### Test: Search Manga

```powershell
curl -X GET "http://localhost:8080/api/v1/manga/search?q=one+piece"
```
**Expected**: Returns manga matching search term

#### Test: Add to Library (Protected)

```powershell
# Replace YOUR_TOKEN with JWT from login
curl -X POST http://localhost:8080/api/v1/progress `
  -H "Authorization: Bearer YOUR_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"manga_id":"MANGA_ID","current_chapter":1,"status":"reading"}'
```
**Expected**: Status 201, progress created

#### Test: Update Progress (Protected)

```powershell
curl -X PUT http://localhost:8080/api/v1/progress/MANGA_ID `
  -H "Authorization: Bearer YOUR_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"current_chapter":5,"status":"reading"}'
```
**Expected**: Status 200, progress updated

#### Test: Get My Progress (Protected)

```powershell
curl -X GET http://localhost:8080/api/v1/progress `
  -H "Authorization: Bearer YOUR_TOKEN"
```
**Expected**: Returns array of user's progress entries

---

### 2. TCP Server Manual Testing

#### Test: Connect and Send Message

1. Open PowerShell
2. Run TCP client script:
```powershell
.\tcp-client.ps1
```
3. **Expected**: Connection established, messages sent/received
4. **Verify**: Check server logs for connection message

#### Test: Multiple Clients

1. Open 3 separate PowerShell windows
2. Run `.\tcp-client.ps1` in each
3. Send message from client 1
4. **Expected**: All clients receive the broadcast
5. **Verify**: All windows show received message

---

### 3. UDP Server Manual Testing

#### Test: Register and Receive Notifications

1. Run UDP client script:
```powershell
.\udp-client.ps1
```
2. **Expected**: 
   - "REGISTERED" confirmation received
   - Periodic notifications appear
3. **Verify**: Notifications contain manga updates

#### Test: Multiple Subscribers

1. Open 2 PowerShell windows
2. Run `.\udp-client.ps1` in each
3. **Expected**: Both receive same notifications
4. **Verify**: Timestamps match

---

### 4. WebSocket Manual Testing

#### Test: Join Chat Room

1. Get JWT token (login via HTTP API)
2. Use browser dev tools or WebSocket client
3. Connect to: `ws://localhost:8080/ws/chat?manga_id=one-piece`
4. Add header: `Authorization: Bearer YOUR_TOKEN`
5. **Expected**: Connection established
6. **Verify**: Check room info endpoint

#### Test: Send and Receive Messages

1. Connect 2 WebSocket clients to same room
2. Send message from client 1:
```json
{"type":"message","content":"Hello from client 1"}
```
3. **Expected**: Client 2 receives the message
4. **Verify**: Both see each other's messages

---

### 5. gRPC Manual Testing

#### Test: List Services

```powershell
grpcurl -plaintext localhost:9092 list
```
**Expected**: Shows `mangahub.v1.MangaService`

#### Test: Get Manga

```powershell
grpcurl -plaintext -d '{"manga_id":"3051a7b2-b47f-4e37-9204-231ce56b7dfb"}' localhost:9092 mangahub.v1.MangaService/GetManga
```
**Expected**: Returns manga details in JSON

#### Test: Search Manga

```powershell
grpcurl -plaintext -d '{"query":"one","limit":5}' localhost:9092 mangahub.v1.MangaService/SearchManga
```
**Expected**: Returns search results

---

## cURL Testing Commands

### Complete cURL Test Suite

#### 1. Health Check

```bash
curl -X GET http://localhost:8080/health
```

#### 2. Register User

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "curluser",
    "email": "curl@test.com",
    "password": "Secure123"
  }'
```

#### 3. Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "curluser",
    "password": "Secure123"
  }'
```

**Save the token from response for next commands**

#### 4. List All Manga

```bash
curl -X GET "http://localhost:8080/api/v1/manga?limit=10&offset=0"
```

#### 5. Get Manga by ID

```bash
curl -X GET http://localhost:8080/api/v1/manga/MANGA_ID
```

#### 6. Search Manga

```bash
curl -X GET "http://localhost:8080/api/v1/manga/search?q=naruto"
```

#### 7. Add Manga to Library (Protected)

```bash
curl -X POST http://localhost:8080/api/v1/progress \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "manga_id": "MANGA_ID",
    "current_chapter": 1,
    "total_chapters": 100,
    "status": "reading"
  }'
```

#### 8. Update Reading Progress (Protected)

```bash
curl -X PUT http://localhost:8080/api/v1/progress/MANGA_ID \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "current_chapter": 25,
    "status": "reading",
    "rating": 9
  }'
```

#### 9. Get My Progress (Protected)

```bash
curl -X GET http://localhost:8080/api/v1/progress \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

#### 10. Delete Progress (Protected)

```bash
curl -X DELETE http://localhost:8080/api/v1/progress/MANGA_ID \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

---

## Protocol-Specific Tests

### TCP Protocol Tests

Run dedicated TCP test script:

```powershell
.\test-tcp.ps1
```

**Tests covered:**
- Single client connection
- Multiple concurrent clients
- Message broadcasting
- Connection persistence
- Error handling

---

### UDP Protocol Tests

Run dedicated UDP test script:

```powershell
.\test-udp-simple.ps1
```

**Tests covered:**
- Client registration
- Notification delivery
- Multiple subscribers
- Timeout handling

---

### WebSocket Protocol Tests

Run dedicated WebSocket test script:

```powershell
.\test-websocket.ps1
```

**Tests covered:**
- Authentication
- Room joining
- Message sending
- Broadcast reception
- Multiple rooms

---

### gRPC Protocol Tests

Run dedicated gRPC test script:

```powershell
.\test-grpc.ps1
```

**Tests covered:**
- Service availability
- GetManga RPC
- SearchManga RPC
- UpdateProgress RPC

---

## Integration Testing

### Full System Integration Test

Run complete integration test:

```powershell
.\test-integration.ps1
```

**Flow:**
1. Register new user
2. Login and get token
3. Search for manga
4. Add manga to library
5. Update progress (triggers TCP sync)
6. Verify progress via gRPC
7. Check UDP notifications
8. Join WebSocket chat room

---

## Troubleshooting Tests

### Common Test Failures

#### 1. Connection Refused

**Symptom**: Cannot connect to server  
**Check**:
```powershell
netstat -an | Select-String "8080|9090|9091|9092"
```
**Fix**: Start the server for that protocol

#### 2. Authentication Failed

**Symptom**: 401 Unauthorized  
**Check**: Token validity  
**Fix**: Login again to get fresh token

#### 3. Port Already in Use

**Symptom**: Server won't start  
**Check**:
```powershell
netstat -ano | Select-String ":PORT"
```
**Fix**: Kill process or change port in config

#### 4. Database Locked

**Symptom**: Database errors  
**Fix**:
```powershell
# Stop all servers
# Delete database
Remove-Item data/mangahub.db
# Restart servers (auto-recreates with seed data)
```

---

## Test Execution Checklist

### Before Testing

- [ ] All 4 servers are running (HTTP, TCP, UDP, gRPC)
- [ ] Database is seeded with test data
- [ ] Required tools installed (curl, grpcurl)
- [ ] No port conflicts

### During Testing

- [ ] Run automated test suite first
- [ ] Verify each protocol individually
- [ ] Test with multiple concurrent clients
- [ ] Check server logs for errors
- [ ] Monitor resource usage

### After Testing

- [ ] Review test results
- [ ] Document any failures
- [ ] Check error logs
- [ ] Clean up test data if needed

---

## Test Results Expected

### Success Criteria

- **Unit Tests**: All pass (100%)
- **HTTP API**: 9/9 tests pass
- **TCP**: 5/5 tests pass
- **UDP**: 4/4 tests pass
- **WebSocket**: 6/6 tests pass
- **gRPC**: 4/4 tests pass
- **Integration**: Full flow completes successfully

### Performance Benchmarks

- API response time: < 100ms
- TCP message latency: < 50ms
- UDP notification: < 30ms
- WebSocket message: < 20ms
- gRPC call: < 80ms

---

## Additional Testing Tools

### Load Testing

```bash
# Install Apache Bench
# Test API endpoint
ab -n 1000 -c 10 http://localhost:8080/api/v1/manga

# Or use the load test script
.\test\load_test.sh
```

### Database Inspection

```powershell
sqlite3 data/mangahub.db
.tables
SELECT * FROM users;
SELECT * FROM manga LIMIT 5;
SELECT * FROM progress;
.quit
```

### Log Monitoring

```powershell
# Watch API server logs
Get-Content -Path "logs/api-server.log" -Tail 50 -Wait

# Or check stdout where server is running
```

---

## Test Coverage

### Unit Test Coverage

Run with coverage report:

```powershell
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

Open `coverage.html` in browser to view detailed coverage.

---

## Quick Reference

### Test Scripts Location

- `test-all.ps1` - Complete test suite
- `test-api.ps1` - HTTP REST API tests
- `test-tcp.ps1` - TCP sync tests
- `test-udp-simple.ps1` - UDP notification tests
- `test-websocket.ps1` - WebSocket chat tests
- `test-grpc.ps1` - gRPC service tests
- `test-integration.ps1` - Integration tests
- `test-curl.ps1` - cURL-based tests

### Default Ports

| Service | Port | Protocol |
|---------|------|----------|
| HTTP API | 8080 | HTTP |
| TCP Sync | 9090 | TCP |
| UDP Notifier | 9091 | UDP |
| gRPC Service | 9092 | gRPC |

---

**For more details, see:**
- [HOW_TO_RUN.md](HOW_TO_RUN.md) - Setup and running guide
- [README.md](README.md) - Project overview
- [KNOWN_ISSUES.md](KNOWN_ISSUES.md) - Known issues and workarounds
