# gRPC Service Test Script
# Tests the gRPC MangaService with 3 RPC methods. Exits 1 if any check fails.

$grpcServer = "localhost:9092"
$failures = 0

function Pass($msg) { Write-Host "[PASS] $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "[FAIL] $msg" -ForegroundColor Red; $script:failures++ }

Write-Host "=== gRPC Service Test ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Prerequisites:" -ForegroundColor Yellow
Write-Host "  1. gRPC server must be running: go run cmd/grpc-server/main.go" -ForegroundColor Gray
Write-Host "  2. grpcurl must be installed: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest" -ForegroundColor Gray
Write-Host ""

# Check if grpcurl is available
try {
    $grpcurlPath = (Get-Command grpcurl -ErrorAction Stop).Source
    Write-Host "[OK] grpcurl found at: $grpcurlPath" -ForegroundColor Green
}
catch {
    Write-Host "[ERROR] grpcurl not found. Please install it:" -ForegroundColor Red
    Write-Host "  go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest" -ForegroundColor Yellow
    Write-Host "  Then restart your terminal to refresh PATH" -ForegroundColor Yellow
    exit 1
}

Write-Host ""

# Test 1: Check if server is running
Write-Host "Test 1: Checking gRPC server..." -ForegroundColor Yellow
try {
    $result = & grpcurl -plaintext $grpcServer list 2>&1 | Out-String
    if ($result -like "*mangahub.v1.MangaService*") {
        Write-Host "[PASS] gRPC server is running and MangaService is available" -ForegroundColor Green
    } else {
        Write-Host "[FAIL] gRPC server not responding properly" -ForegroundColor Red
        Write-Host "Output: $result" -ForegroundColor Gray
        exit 1
    }
}
catch {
    Write-Host "[FAIL] Cannot connect to gRPC server" -ForegroundColor Red
    Write-Host "Make sure server is running: go run cmd/grpc-server/main.go" -ForegroundColor Yellow
    exit 1
}

Write-Host ""

# Look up a real manga ID first (IDs are generated at seed time)
$mangaId = $null
try {
    # Note: keep the query free of spaces - PowerShell 5.1 splits args that
    # contain both embedded quotes and spaces when calling native commands.
    $lookup = & grpcurl -plaintext -d '{\"query\":\"naruto\",\"limit\":1,\"offset\":0}' $grpcServer mangahub.v1.MangaService/SearchManga 2>&1 | Out-String
    if ($lookup -match '"id":\s*"([^"]+)"') {
        $mangaId = $matches[1]
        Write-Host "Using manga ID: $mangaId" -ForegroundColor Gray
    }
}
catch { }
if (-not $mangaId) {
    Fail "Could not look up a manga ID with SearchManga"
    exit 1
}

Write-Host ""

# Test 2: GetManga
Write-Host "Test 2: Testing GetManga RPC..." -ForegroundColor Yellow
try {
    $getMangaResp = & grpcurl -plaintext -d ('{\"manga_id\":\"' + $mangaId + '\"}') $grpcServer mangahub.v1.MangaService/GetManga 2>&1 | Out-String

    if ($getMangaResp -like '*"title"*') {
        Write-Host "[PASS] GetManga RPC working" -ForegroundColor Green
        # Extract title from response
        if ($getMangaResp -match '"title":\s*"([^"]+)"') {
            Write-Host "  Found manga: $($matches[1])" -ForegroundColor Gray
        }
    } else {
        Fail "GetManga did not return expected data"
        Write-Host "Response: $getMangaResp" -ForegroundColor Gray
    }
}
catch {
    Fail "GetManga RPC error: $_"
}

Write-Host ""

# Test 3: SearchManga
Write-Host "Test 3: Testing SearchManga RPC..." -ForegroundColor Yellow
try {
    $searchResp = & grpcurl -plaintext -d '{\"query\":\"one\",\"limit\":5,\"offset\":0}' $grpcServer mangahub.v1.MangaService/SearchManga 2>&1 | Out-String

    if ($searchResp -like '*"manga"*') {
        Write-Host "[PASS] SearchManga RPC working" -ForegroundColor Green
        # Extract total from response
        if ($searchResp -match '"total":\s*(\d+)') {
            Write-Host "  Total results: $($matches[1])" -ForegroundColor Gray
        }
    } else {
        Fail "SearchManga did not return expected data"
        Write-Host "Response: $searchResp" -ForegroundColor Gray
    }
}
catch {
    Fail "SearchManga RPC error: $_"
}

Write-Host ""

# Test 4: UpdateProgress
Write-Host "Test 4: Testing UpdateProgress RPC..." -ForegroundColor Yellow
try {
    # UpdateProgress requires the caller's JWT, and only for their own progress:
    # log in as reader1 over HTTP and send the token as gRPC metadata
    $loginBody = @{ username = "reader1"; password = "password123" } | ConvertTo-Json
    $token = (Invoke-RestMethod -Method Post -Uri "http://localhost:8080/auth/login" -ContentType "application/json" -Body $loginBody).data.token

    # Using username "reader1" (will be converted to UUID by service)
    $updateJson = '{\"user_id\":\"reader1\",\"manga_id\":\"' + $mangaId + '\",\"current_chapter\":50,\"status\":\"reading\"}'

    # Without the token the server must refuse
    $anonResp = & grpcurl -plaintext -d $updateJson $grpcServer mangahub.v1.MangaService/UpdateProgress 2>&1 | Out-String
    if ($anonResp -like '*Unauthenticated*') {
        Write-Host "[PASS] UpdateProgress without a token is rejected (Unauthenticated)" -ForegroundColor Green
    } else {
        Fail "UpdateProgress without a token was not rejected"
        Write-Host "Response: $anonResp" -ForegroundColor Gray
    }

    $updateResp = & grpcurl -plaintext -H "authorization: Bearer $token" -d $updateJson $grpcServer mangahub.v1.MangaService/UpdateProgress 2>&1 | Out-String

    if ($updateResp -like '*"currentChapter"*') {
        Write-Host "[PASS] UpdateProgress RPC working" -ForegroundColor Green
        # Extract chapter from response (camelCase format)
        if ($updateResp -match '"currentChapter":\s*(\d+)') {
            Write-Host "  Updated to chapter: $($matches[1])" -ForegroundColor Gray
        }
    } else {
        Fail "UpdateProgress did not return expected data"
        Write-Host "Response: $updateResp" -ForegroundColor Gray
    }
}
catch {
    Fail "UpdateProgress RPC error: $_"
}

Write-Host ""
if ($failures -gt 0) {
    Write-Host "=== gRPC Tests: $failures check(s) FAILED ===" -ForegroundColor Red
} else {
    Write-Host "=== gRPC Tests: all checks passed ===" -ForegroundColor Green
}
Write-Host ""
Write-Host "Summary:" -ForegroundColor Cyan
Write-Host "  - GetManga: Retrieves single manga by ID" -ForegroundColor Gray
Write-Host "  - SearchManga: Searches with filters and pagination" -ForegroundColor Gray
Write-Host "  - UpdateProgress: Updates user reading progress (needs a JWT: -H 'authorization: Bearer <token>')" -ForegroundColor Gray
if ($failures -gt 0) { exit 1 }
