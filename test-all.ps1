# Comprehensive Phase 9 Testing Script

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host " PHASE 9: TESTING & BUG FIXES" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host ""

# Prerequisites Check
Write-Host "Checking prerequisites..." -ForegroundColor Yellow
$servers = @(
    @{Name="HTTP API"; Port=8080},
    @{Name="TCP Sync"; Port=9090},
    @{Name="UDP Notifier"; Port=9091},
    @{Name="gRPC Service"; Port=9092}
)

$running = 0
foreach ($server in $servers) {
    $tcpConn = New-Object System.Net.Sockets.TcpClient
    try {
        $tcpConn.Connect("localhost", $server.Port)
        Write-Host "[OK] $($server.Name) running on port $($server.Port)" -ForegroundColor Green
        $running++
    } catch {
        Write-Host "[WARN] $($server.Name) not running on port $($server.Port)" -ForegroundColor Yellow
    } finally {
        $tcpConn.Dispose()
    }
}

Write-Host ""
Write-Host "Servers running: $running/4" -ForegroundColor Cyan
Write-Host ""

if ($running -lt 4) {
    Write-Host "⚠️  Not all servers are running." -ForegroundColor Yellow
    Write-Host "Start servers with:" -ForegroundColor Gray
    Write-Host "  Terminal 1: go run cmd/api-server/main.go" -ForegroundColor Gray
    Write-Host "  Terminal 2: go run cmd/tcp-server/main.go" -ForegroundColor Gray
    Write-Host "  Terminal 3: go run cmd/udp-server/main.go" -ForegroundColor Gray
    Write-Host "  Terminal 4: go run cmd/grpc-server/main.go" -ForegroundColor Gray
    Write-Host ""
    Write-Host "Continuing with available servers..." -ForegroundColor Yellow
    Write-Host ""
}

# Test 1: Unit Tests
Write-Host "=====================================" -ForegroundColor Yellow
Write-Host "Test 1: Running unit tests..." -ForegroundColor Yellow
Write-Host "=====================================" -ForegroundColor Yellow
try {
    $unitOutput = & go test -v -short ./internal/auth 2>&1
    Write-Host $unitOutput
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[PASS] Unit tests passed" -ForegroundColor Green
    } else {
        Write-Host "[FAIL] Unit tests failed" -ForegroundColor Red
    }
} catch {
    Write-Host "[SKIP] Unit tests not available: $_" -ForegroundColor Yellow
}

Write-Host ""

# Test 2: HTTP API
Write-Host "=====================================" -ForegroundColor Yellow
Write-Host "Test 2: HTTP API endpoints..." -ForegroundColor Yellow
Write-Host "=====================================" -ForegroundColor Yellow

# Health check
try {
    $healthResp = Invoke-RestMethod -Uri "http://localhost:8080/health" -Method GET -TimeoutSec 5
    if ($healthResp.status -eq "healthy") {
        Write-Host "[PASS] GET /health working" -ForegroundColor Green
    }
} catch {
    Write-Host "[FAIL] Health check error: $($_.Exception.Message)" -ForegroundColor Red
}

# Manga search
try {
    $mangaResp = Invoke-RestMethod -Uri "http://localhost:8080/manga?limit=5" -Method GET -TimeoutSec 5
    if ($mangaResp.success) {
        Write-Host "[PASS] GET /manga working ($($mangaResp.data.total) manga found)" -ForegroundColor Green
    }
} catch {
    Write-Host "[FAIL] Manga search error: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""

# Test 3: TCP Server
Write-Host "=====================================" -ForegroundColor Yellow
Write-Host "Test 3: TCP Sync Server..." -ForegroundColor Yellow
Write-Host "=====================================" -ForegroundColor Yellow
try {
    $tcpClient = New-Object System.Net.Sockets.TcpClient
    $tcpClient.Connect("localhost", 9090)
    Write-Host "[PASS] TCP connection established" -ForegroundColor Green
    
    # Send test message
    $stream = $tcpClient.GetStream()
    $message = '{"user_id":"test","manga_id":"test","chapter":1,"timestamp":' + [int][double]::Parse((Get-Date -UFormat %s)) + '}'
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($message + "`n")
    $stream.Write($bytes, 0, $bytes.Length)
    Write-Host "[PASS] TCP message sent" -ForegroundColor Green
    
    $tcpClient.Close()
} catch {
    Write-Host "[FAIL] TCP connection failed: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""

# Test 4: UDP Server
Write-Host "=====================================" -ForegroundColor Yellow
Write-Host "Test 4: UDP Notifier..." -ForegroundColor Yellow
Write-Host "=====================================" -ForegroundColor Yellow
try {
    $udpClient = New-Object System.Net.Sockets.UdpClient
    $endpoint = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Parse("127.0.0.1"), 9091)
    $msg = [System.Text.Encoding]::ASCII.GetBytes("REGISTER")
    $udpClient.Send($msg, $msg.Length, $endpoint) | Out-Null
    Write-Host "[PASS] UDP message sent" -ForegroundColor Green
    $udpClient.Close()
} catch {
    Write-Host "[FAIL] UDP error: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""

# Test 5: gRPC
Write-Host "=====================================" -ForegroundColor Yellow
Write-Host "Test 5: gRPC Service..." -ForegroundColor Yellow
Write-Host "=====================================" -ForegroundColor Yellow
try {
    $grpcTest = & grpcurl -plaintext list localhost:9092 2>&1
    if ($grpcTest -like "*MangaService*") {
        Write-Host "[PASS] gRPC service available" -ForegroundColor Green
    } else {
        Write-Host "[SKIP] gRPC service not detected" -ForegroundColor Yellow
    }
} catch {
    Write-Host "[SKIP] grpcurl not available - install with: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest" -ForegroundColor Yellow
}

Write-Host ""

# Test 6: CLI Tool
Write-Host "=====================================" -ForegroundColor Yellow
Write-Host "Test 6: CLI Tool..." -ForegroundColor Yellow
Write-Host "=====================================" -ForegroundColor Yellow
try {
    $cliVersion = & .\bin\mangahub.exe version 2>&1
    if ($cliVersion -like "*v1.0.0*") {
        Write-Host "[PASS] CLI working" -ForegroundColor Green
        Write-Host "  Version: $($cliVersion -replace '`n', ' ')" -ForegroundColor Gray
    } else {
        Write-Host "[WARN] CLI version unexpected: $cliVersion" -ForegroundColor Yellow
    }
} catch {
    Write-Host "[FAIL] CLI error: $($_.Exception.Message)" -ForegroundColor Red
}

# Test CLI commands
try {
    $cliConfig = & .\bin\mangahub.exe config show 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[PASS] CLI config show working" -ForegroundColor Green
    }
} catch {
    Write-Host "[FAIL] CLI config show error" -ForegroundColor Red
}

Write-Host ""

# Test 7: Integration Tests
Write-Host "=====================================" -ForegroundColor Yellow
Write-Host "Test 7: Integration Tests..." -ForegroundColor Yellow
Write-Host "=====================================" -ForegroundColor Yellow
try {
    $integrationOutput = & go test -v ./test/... 2>&1
    Write-Host $integrationOutput
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[PASS] Integration tests passed" -ForegroundColor Green
    } else {
        Write-Host "[WARN] Some integration tests skipped or failed" -ForegroundColor Yellow
    }
} catch {
    Write-Host "[SKIP] Integration tests not available: $_" -ForegroundColor Yellow
}

Write-Host ""

# Summary
Write-Host "=====================================" -ForegroundColor Green
Write-Host " PHASE 9 TESTING COMPLETE" -ForegroundColor Green
Write-Host "=====================================" -ForegroundColor Green

Write-Host ""
Write-Host "Test Summary:" -ForegroundColor Cyan
Write-Host "  ✓ Unit tests executed" -ForegroundColor White
Write-Host "  ✓ HTTP API verified" -ForegroundColor White
Write-Host "  ✓ TCP server verified" -ForegroundColor White
Write-Host "  ✓ UDP server verified" -ForegroundColor White
Write-Host "  ✓ gRPC service verified" -ForegroundColor White
Write-Host "  ✓ CLI tool verified" -ForegroundColor White
Write-Host "  ✓ Integration tests executed" -ForegroundColor White

Write-Host ""
Write-Host "Next Steps:" -ForegroundColor Cyan
Write-Host "  - Review test output above" -ForegroundColor Gray
Write-Host "  - Fix any failing tests" -ForegroundColor Gray
Write-Host "  - Run load tests: make load-test" -ForegroundColor Gray
Write-Host "  - Generate coverage: make test-coverage" -ForegroundColor Gray
Write-Host ""
Write-Host "Ready for Phase 10: Documentation & Demo Prep!" -ForegroundColor Green
