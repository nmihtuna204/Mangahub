# Comprehensive test run: Go unit + end-to-end tests, a quick check of each
# server, the CLI, and (when all four servers are up) the live Go tests.
# Prints a real summary and exits 1 if anything failed.
#
# Start the servers first (one terminal each):
#   go run ./cmd/api-server ; go run ./cmd/tcp-server ; go run ./cmd/udp-server ; go run ./cmd/grpc-server

$results = New-Object System.Collections.ArrayList
function Record($name, $status, $detail = "") {
    $null = $results.Add([pscustomobject]@{ Test = $name; Status = $status; Detail = $detail })
    $color = @{ PASS = "Green"; FAIL = "Red"; SKIP = "Yellow" }[$status]
    Write-Host "[$status] $name $detail" -ForegroundColor $color
}
function Section($title) {
    Write-Host ""
    Write-Host "=====================================" -ForegroundColor Yellow
    Write-Host $title -ForegroundColor Yellow
    Write-Host "=====================================" -ForegroundColor Yellow
}

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host " MANGAHUB: FULL TEST RUN" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan

# Prerequisites
Section "Checking servers..."
$tcpPorts = @{ "HTTP API" = 8080; "TCP Sync" = 9090; "gRPC Service" = 9092 }
$up = @{}
foreach ($name in $tcpPorts.Keys) {
    $c = New-Object System.Net.Sockets.TcpClient
    try { $c.Connect("localhost", $tcpPorts[$name]); $up[$name] = $true } catch { $up[$name] = $false } finally { $c.Dispose() }
}
# UDP is connectionless: ask the server itself
$udpClient = New-Object System.Net.Sockets.UdpClient
$udpEndpoint = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Parse("127.0.0.1"), 9091)
try {
    $msg = [System.Text.Encoding]::ASCII.GetBytes("REGISTER")
    $udpClient.Send($msg, $msg.Length, $udpEndpoint) | Out-Null
    $udpClient.Client.ReceiveTimeout = 2000
    $from = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any, 0)
    $up["UDP Notifier"] = [System.Text.Encoding]::ASCII.GetString($udpClient.Receive([ref]$from)) -eq "REGISTERED"
    $msg = [System.Text.Encoding]::ASCII.GetBytes("UNREGISTER")
    $udpClient.Send($msg, $msg.Length, $udpEndpoint) | Out-Null
}
catch { $up["UDP Notifier"] = $false }
$udpClient.Close()

foreach ($name in @("HTTP API", "TCP Sync", "UDP Notifier", "gRPC Service")) {
    if ($up[$name]) { Write-Host "[OK] $name is running" -ForegroundColor Green }
    else { Write-Host "[WARN] $name is not running" -ForegroundColor Yellow }
}
$running = @($up.Values | Where-Object { $_ }).Count
Write-Host "Servers running: $running/4" -ForegroundColor Cyan

# Test 1: Go unit tests (no servers needed)
Section "Test 1: Go unit tests (internal/..., pkg/...)"
$out = & go test ./internal/... ./pkg/... 2>&1
$code = $LASTEXITCODE
$out | Where-Object { $_ -match '^(ok|FAIL|---|panic)' } | ForEach-Object { Write-Host "  $_" }
$okCount = @($out | Where-Object { $_ -match '^ok' }).Count
if ($code -eq 0) { Record "Go unit tests" PASS "($okCount packages)" }
else { Record "Go unit tests" FAIL "(see output above)" }

# Test 2: HTTP API
Section "Test 2: HTTP API endpoints"
if ($up["HTTP API"]) {
    try {
        $health = Invoke-RestMethod -Uri "http://localhost:8080/health" -TimeoutSec 5
        if ($health.status -eq "healthy") { Record "GET /health" PASS } else { Record "GET /health" FAIL "(status $($health.status))" }
    }
    catch { Record "GET /health" FAIL $_.Exception.Message }
    try {
        $manga = Invoke-RestMethod -Uri "http://localhost:8080/manga?limit=5" -TimeoutSec 5
        if ($manga.success -and @($manga.data.data).Count -gt 0) { Record "GET /manga" PASS "($($manga.data.total) manga)" }
        else { Record "GET /manga" FAIL "(no manga returned)" }
    }
    catch { Record "GET /manga" FAIL $_.Exception.Message }
}
else { Record "HTTP API" SKIP "(server not running)" }

# Test 3: TCP sync server relays an update back
Section "Test 3: TCP Sync Server"
if ($up["TCP Sync"]) {
    try {
        $tcpClient = New-Object System.Net.Sockets.TcpClient
        $tcpClient.Connect("localhost", 9090)
        $stream = $tcpClient.GetStream()
        $stream.ReadTimeout = 3000
        $reader = New-Object System.IO.StreamReader($stream)
        $user = "test-all-$(Get-Random)"
        $line = '{"user_id":"' + $user + '","manga_id":"test","chapter":1,"timestamp":' + [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() + '}'
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($line + "`n")
        $stream.Write($bytes, 0, $bytes.Length)
        $relayed = $false
        while (-not $relayed) {
            $got = $reader.ReadLine()
            if ($null -eq $got) { break }
            $relayed = $got -like "*$user*"
        }
        $tcpClient.Close()
        if ($relayed) { Record "TCP relay" PASS } else { Record "TCP relay" FAIL "(update not relayed)" }
    }
    catch { Record "TCP relay" FAIL $_.Exception.Message }
}
else { Record "TCP Sync" SKIP "(server not running)" }

# Test 4: UDP (registration was checked above)
Section "Test 4: UDP Notifier"
if ($up["UDP Notifier"]) { Record "UDP REGISTER -> REGISTERED" PASS }
else { Record "UDP Notifier" SKIP "(server not running)" }

# Test 5: gRPC
Section "Test 5: gRPC Service"
if (-not $up["gRPC Service"]) { Record "gRPC" SKIP "(server not running)" }
elseif (-not (Get-Command grpcurl -ErrorAction SilentlyContinue)) {
    Record "gRPC" SKIP "(grpcurl not installed: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest)"
}
else {
    $list = & grpcurl -plaintext localhost:9092 list 2>&1 | Out-String
    if ($list -like "*mangahub.v1.MangaService*") { Record "gRPC reflection lists MangaService" PASS }
    else { Record "gRPC reflection" FAIL $list.Trim() }
}

# Test 6: CLI, built fresh (an old bin\mangahub.exe would test old code)
Section "Test 6: CLI Tool"
& go build -o bin/mangahub.exe ./cmd/cli 2>&1 | ForEach-Object { Write-Host "  $_" }
if ($LASTEXITCODE -ne 0) { Record "CLI build" FAIL }
else {
    $version = & .\bin\mangahub.exe version 2>&1 | Out-String
    if ($version -like "*v1.0.0*") { Record "mangahub version" PASS } else { Record "mangahub version" FAIL $version.Trim() }
    & .\bin\mangahub.exe config show *> $null
    if ($LASTEXITCODE -eq 0) { Record "mangahub config show" PASS } else { Record "mangahub config show" FAIL }
}

# Test 7: End-to-end tests (in-process), plus the live tests when all four servers are up
Section "Test 7: End-to-end tests (go test ./test/...)"
if ($running -eq 4) {
    $env:MANGAHUB_LIVE = "1"
    Write-Host "All four servers are up: also running the live tests against them" -ForegroundColor Cyan
}
$out = & go test -v -count=1 ./test/... 2>&1
$code = $LASTEXITCODE
Remove-Item Env:MANGAHUB_LIVE -ErrorAction SilentlyContinue
$out | Where-Object { $_ -match '^(--- (PASS|FAIL|SKIP)|ok|FAIL)' } | ForEach-Object { Write-Host "  $_" }
$passed = @($out | Where-Object { $_ -match '^--- PASS' }).Count
$skipped = @($out | Where-Object { $_ -match '^--- SKIP' }).Count
if ($code -eq 0) { Record "End-to-end tests" PASS "($passed passed, $skipped skipped)" }
else { Record "End-to-end tests" FAIL "(see output above)" }

# Summary
Write-Host ""
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host " SUMMARY" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
$results | Format-Table -AutoSize | Out-String -Width 200 | Write-Host
$failed = @($results | Where-Object { $_.Status -eq "FAIL" }).Count
$skippedTotal = @($results | Where-Object { $_.Status -eq "SKIP" }).Count
Write-Host "Passed: $(@($results | Where-Object { $_.Status -eq 'PASS' }).Count)  Failed: $failed  Skipped: $skippedTotal"
Write-Host ""
Write-Host "More checks per protocol: .\test-api.ps1, .\test-tcp.ps1, .\test-udp-simple.ps1, .\test-websocket.ps1, .\test-grpc.ps1, .\test-integration.ps1" -ForegroundColor Gray
Write-Host "Load test: make load-test (or bash test/load_test.sh)" -ForegroundColor Gray
if ($failed -gt 0) { exit 1 }
