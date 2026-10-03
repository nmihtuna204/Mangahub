# WebSocket Chat Test Script
# Two users join a room, exchange messages, and the script checks that both
# see every message, that the room info and saved history are right, and
# that leaving is announced. Exits 1 if any check fails.
#
# Client -> server frames are {"content": "..."} (see docs/API.md); this
# script used to send {"message": ...}, which the server ignores.

$baseUrl = "http://localhost:8080"
$wsUrl = "ws://localhost:8080"
$room = "ps-websocket-test"
$failures = 0

function Pass($msg) { Write-Host "[PASS] $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "[FAIL] $msg" -ForegroundColor Red; $script:failures++ }

function Get-Token($username, $password) {
    $body = @{ username = $username; password = $password } | ConvertTo-Json
    return (Invoke-RestMethod -Uri "$baseUrl/auth/login" -Method POST -ContentType "application/json" -Body $body).data.token
}

function Connect-WebSocket($name, $token) {
    $ws = New-Object System.Net.WebSockets.ClientWebSocket
    $ws.Options.SetRequestHeader("Authorization", "Bearer $token")
    $uri = New-Object System.Uri("$wsUrl/ws/chat?room_id=$room")
    try {
        $ws.ConnectAsync($uri, [System.Threading.CancellationToken]::None).Wait()
    }
    catch { }
    if ($ws.State -eq 'Open') { Pass "[$name] connected"; return $ws }
    Fail "[$name] could not connect (state $($ws.State))"
    return $null
}

function Send-Chat($ws, $text) {
    $json = @{ content = $text } | ConvertTo-Json -Compress
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
    $segment = New-Object System.ArraySegment[byte] -ArgumentList @(, $bytes)
    $ws.SendAsync($segment, [System.Net.WebSockets.WebSocketMessageType]::Text, $true,
        [System.Threading.CancellationToken]::None).Wait()
}

# Reads frames until one matches (type and content), or $timeoutMs passes.
# Note: in .NET Framework a receive that times out aborts the socket, so only
# wait for messages that should arrive.
function Wait-Chat($ws, $type, $content, $timeoutMs = 3000) {
    $deadline = (Get-Date).AddMilliseconds($timeoutMs)
    while ($ws.State -eq 'Open' -and (Get-Date) -lt $deadline) {
        $buffer = New-Object byte[] 16384
        $text = ""
        do {
            $segment = New-Object System.ArraySegment[byte] -ArgumentList @(, $buffer)
            $left = [int]($deadline - (Get-Date)).TotalMilliseconds
            if ($left -lt 1) { return $null }
            $cts = New-Object System.Threading.CancellationTokenSource
            $cts.CancelAfter($left)
            try {
                $task = $ws.ReceiveAsync($segment, $cts.Token)
                $task.Wait()
                $result = $task.Result
            }
            catch { return $null }
            if ($result.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) { return $null }
            $text += [System.Text.Encoding]::UTF8.GetString($buffer, 0, $result.Count)
        } while (-not $result.EndOfMessage)

        try { $msg = $text | ConvertFrom-Json } catch { continue }
        if ($msg.type -eq $type -and ($null -eq $content -or $msg.content -eq $content)) { return $msg }
    }
    return $null
}

function Close-WebSocket($ws) {
    try {
        $ws.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "test complete",
            [System.Threading.CancellationToken]::None).Wait(2000) | Out-Null
    }
    catch { }
    $ws.Dispose()
}

Write-Host "=== WebSocket Chat System Test ===" -ForegroundColor Cyan
Write-Host ""

# Test 1: Two users log in
Write-Host "Test 1: Logging in as admin and reader1..." -ForegroundColor Yellow
try {
    $adminToken = Get-Token "admin" "admin123"
    $readerToken = Get-Token "reader1" "password123"
    Pass "Got JWT tokens for both users"
}
catch {
    Fail "Login failed: $($_.Exception.Message)"
    Write-Host "Make sure the API server is running: go run ./cmd/api-server" -ForegroundColor Yellow
    exit 1
}
Write-Host ""

# Test 2: The endpoint requires a token
Write-Host "Test 2: Connecting without a token..." -ForegroundColor Yellow
try {
    Invoke-WebRequest -Uri "$baseUrl/ws/chat?room_id=$room" -UseBasicParsing | Out-Null
    Fail "The chat endpoint accepted a request without a token"
}
catch {
    if ($_.Exception.Response.StatusCode -eq 401) { Pass "Rejected with 401" }
    else { Fail "Unexpected error: $($_.Exception.Message)" }
}
Write-Host ""

# Test 3: Both join the room
Write-Host "Test 3: Two clients join room '$room'..." -ForegroundColor Yellow
$ws1 = Connect-WebSocket "Client1/admin" $adminToken
if (-not $ws1) { exit 1 }
if (Wait-Chat $ws1 "join" "admin joined the chat") { Pass "[Client1] got its own join notice" }
else { Fail "[Client1] no join notice" }

$ws2 = Connect-WebSocket "Client2/reader1" $readerToken
if (-not $ws2) { exit 1 }
if (Wait-Chat $ws2 "join" "reader1 joined the chat") { Pass "[Client2] got its own join notice" }
else { Fail "[Client2] no join notice" }
if (Wait-Chat $ws1 "join" "reader1 joined the chat") { Pass "[Client1] saw reader1 join" }
else { Fail "[Client1] did not see reader1 join" }
Write-Host ""

# Test 4: Messages reach both members
Write-Host "Test 4: Exchanging messages..." -ForegroundColor Yellow
$hello1 = "Hello from Client 1! ($(Get-Random))"
Send-Chat $ws1 $hello1
foreach ($c in @(@{ Name = "Client1"; Ws = $ws1 }, @{ Name = "Client2"; Ws = $ws2 })) {
    $m = Wait-Chat $c.Ws "message" $hello1
    if ($m -and $m.username -eq "admin") { Pass "[$($c.Name)] received admin's message" }
    else { Fail "[$($c.Name)] did not receive admin's message" }
}

$hello2 = "Hello from Client 2! ($(Get-Random))"
Send-Chat $ws2 $hello2
foreach ($c in @(@{ Name = "Client1"; Ws = $ws1 }, @{ Name = "Client2"; Ws = $ws2 })) {
    $m = Wait-Chat $c.Ws "message" $hello2
    if ($m -and $m.username -eq "reader1") { Pass "[$($c.Name)] received reader1's message" }
    else { Fail "[$($c.Name)] did not receive reader1's message" }
}
Write-Host ""

# Test 5: Room info and saved history
Write-Host "Test 5: Room info and chat history..." -ForegroundColor Yellow
try {
    $info = Invoke-RestMethod -Uri "$baseUrl/rooms/$room" -Method GET
    if ($info.count -eq 2) { Pass "GET /rooms/$room lists 2 connected users ($($info.clients -join ', '))" }
    else { Fail "GET /rooms/$room count = $($info.count), want 2" }
}
catch { Fail "Room info failed: $($_.Exception.Message)" }

$saved = $false
$deadline = (Get-Date).AddSeconds(3)   # messages are saved asynchronously
while (-not $saved -and (Get-Date) -lt $deadline) {
    try {
        $history = (Invoke-RestMethod -Uri "$baseUrl/rooms/$room/messages?limit=50" -Method GET).data.messages
        $contents = @($history | ForEach-Object { $_.content })
        $saved = ($contents -contains $hello1) -and ($contents -contains $hello2)
    }
    catch { }
    if (-not $saved) { Start-Sleep -Milliseconds 200 }
}
if ($saved) { Pass "Both messages are in the saved room history" }
else { Fail "Messages missing from GET /rooms/$room/messages" }
Write-Host ""

# Test 6: Leaving is announced
Write-Host "Test 6: Client1 leaves..." -ForegroundColor Yellow
Close-WebSocket $ws1
if (Wait-Chat $ws2 "leave" "admin left the chat") { Pass "[Client2] saw admin leave" }
else { Fail "[Client2] no leave notice" }
Close-WebSocket $ws2

Write-Host ""
if ($failures -gt 0) {
    Write-Host "=== WebSocket test: $failures check(s) FAILED ===" -ForegroundColor Red
    exit 1
}
Write-Host "=== WebSocket test: all checks passed ===" -ForegroundColor Green
