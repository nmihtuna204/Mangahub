# Phase 7: Integration Test - All 5 Protocols Working Together
#
# Subscribes to the TCP sync server, the UDP notifier and the manga's
# WebSocket chat room, makes ONE progress update over HTTP, and checks that
# the protocol bridge delivered it on each of them. (This script used to
# print "TCP/UDP/WebSocket/gRPC triggered" without checking anything.)
# The gRPC leg is an audit call whose only visible effect is an AUDIT line in
# the gRPC server's log. Exits 1 if any check fails.

$baseUrl = "http://localhost:8080"
$failures = 0

function Pass($msg) { Write-Host "[PASS] $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "[FAIL] $msg" -ForegroundColor Red; $script:failures++ }

Write-Host "=== PHASE 7: INTEGRATION & CROSS-PROTOCOL TEST ===" -ForegroundColor Cyan
Write-Host ""

# Step 1: Login
Write-Host "Step 1: Login as reader1..." -ForegroundColor Yellow
try {
    $loginBody = @{ username = 'reader1'; password = 'password123' } | ConvertTo-Json
    $loginResp = Invoke-RestMethod -Uri "$baseUrl/auth/login" -Method POST -ContentType "application/json" -Body $loginBody
    $token = $loginResp.data.token
    $userId = $loginResp.data.user.id
    Pass "Logged in, token $($token.Substring(0,20))..."
}
catch {
    Fail "Login failed: $($_.Exception.Message)"
    Write-Host "Make sure the API server is running (it seeds reader1 / password123)" -ForegroundColor Yellow
    exit 1
}
$headers = @{ "Authorization" = "Bearer $token"; "Content-Type" = "application/json" }

# Pick the manga and the next chapter
$searchResp = Invoke-RestMethod -Uri "$baseUrl/manga?q=one+piece&limit=1" -Method GET
$mangaId = $searchResp.data.data[0].id
$title = $searchResp.data.data[0].title
$library = Invoke-RestMethod -Uri "$baseUrl/users/library" -Headers $headers
$current = @($library.data | Where-Object { $_.manga_id -eq $mangaId })
$chapter = 1
if ($current.Count -gt 0) { $chapter = [int]$current[0].current_chapter + 1 }
Write-Host "  Manga: $title ($mangaId), next chapter: $chapter" -ForegroundColor Gray
Write-Host ""

# Step 2: Subscribe on every protocol
Write-Host "Step 2: Subscribing to TCP, UDP and WebSocket..." -ForegroundColor Yellow

# TCP sync client
$tcp = $null
try {
    $tcp = New-Object System.Net.Sockets.TcpClient
    $tcp.Connect("localhost", 9090)
    $tcp.GetStream().ReadTimeout = 5000
    $tcpReader = New-Object System.IO.StreamReader($tcp.GetStream())
    Pass "TCP sync client connected"
}
catch { Fail "TCP sync server not reachable: $($_.Exception.Message)"; $tcp = $null }

# UDP subscriber
$udp = New-Object System.Net.Sockets.UdpClient
$udpServer = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Parse("127.0.0.1"), 9091)
$udpOk = $false
try {
    $reg = [System.Text.Encoding]::ASCII.GetBytes("REGISTER")
    $udp.Send($reg, $reg.Length, $udpServer) | Out-Null
    $udp.Client.ReceiveTimeout = 2000
    $from = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any, 0)
    $udpOk = [System.Text.Encoding]::UTF8.GetString($udp.Receive([ref]$from)) -eq "REGISTERED"
}
catch { }
if ($udpOk) { Pass "UDP subscriber registered" } else { Fail "UDP notifier did not confirm REGISTER" }

# WebSocket member of the manga's room
$ws = New-Object System.Net.WebSockets.ClientWebSocket
$ws.Options.SetRequestHeader("Authorization", "Bearer $token")
try { $ws.ConnectAsync([Uri]"ws://localhost:8080/ws/chat?room_id=manga_$mangaId", [Threading.CancellationToken]::None).Wait() } catch { }
if ($ws.State -eq 'Open') { Pass "WebSocket joined room manga_$mangaId" } else { Fail "WebSocket connect failed (state $($ws.State))" }

# Reads WebSocket frames until $match returns true for one, or $timeoutMs passes
function Wait-WS($match, $timeoutMs) {
    $deadline = (Get-Date).AddMilliseconds($timeoutMs)
    while ($ws.State -eq 'Open' -and (Get-Date) -lt $deadline) {
        $buf = New-Object byte[] 16384
        $seg = New-Object System.ArraySegment[byte] -ArgumentList @(, $buf)
        $cts = New-Object System.Threading.CancellationTokenSource
        $cts.CancelAfter([Math]::Max(1, [int]($deadline - (Get-Date)).TotalMilliseconds))
        try { $t = $ws.ReceiveAsync($seg, $cts.Token); $t.Wait(); $res = $t.Result } catch { return $null }
        try { $m = [System.Text.Encoding]::UTF8.GetString($buf, 0, $res.Count) | ConvertFrom-Json } catch { continue }
        if (& $match $m) { return $m }
    }
    return $null
}
if ($ws.State -eq 'Open') { Wait-WS { param($m) $m.type -eq "join" } 3000 | Out-Null }
Write-Host ""

# Step 3: One HTTP update
Write-Host "Step 3: PUT /users/progress (chapter $chapter)..." -ForegroundColor Yellow
try {
    $updateBody = @{ manga_id = $mangaId; current_chapter = $chapter; status = "reading" } | ConvertTo-Json
    $updateResp = Invoke-RestMethod -Uri "$baseUrl/users/progress" -Method PUT -Headers $headers -Body $updateBody
    if ($updateResp.data.current_chapter -eq $chapter) { Pass "HTTP: progress saved (chapter $chapter)" }
    else { Fail "HTTP: response chapter $($updateResp.data.current_chapter)" }
}
catch {
    Fail "HTTP update failed: $($_.Exception.Message)"
    exit 1
}
Write-Host ""

# Step 4: Did the bridge deliver it everywhere?
Write-Host "Step 4: Checking every protocol received it..." -ForegroundColor Yellow

if ($tcp) {
    $got = $false
    try {
        while (-not $got) {
            $line = $tcpReader.ReadLine()
            if ($null -eq $line) { break }
            try { $u = $line | ConvertFrom-Json } catch { continue }
            $got = ($u.manga_id -eq $mangaId -and $u.chapter -eq $chapter -and $u.user_id -eq $userId)
        }
    }
    catch { }
    if ($got) { Pass "TCP: sync clients got {manga_id, chapter $chapter}" } else { Fail "TCP: no update within 5 s" }
}

if ($udpOk) {
    $got = $false
    $udp.Client.ReceiveTimeout = 5000
    try {
        while (-not $got) {
            $data = [System.Text.Encoding]::UTF8.GetString($udp.Receive([ref]$from))
            try { $n = $data | ConvertFrom-Json } catch { continue }
            $got = ($n.type -eq "progress_update" -and $n.manga_id -eq $mangaId -and $n.chapter -eq $chapter)
        }
    }
    catch { }
    if ($got) { Pass "UDP: subscribers got a progress_update notification ('$($n.message)')" } else { Fail "UDP: no notification within 5 s" }
}

if ($ws.State -eq 'Open') {
    $notice = Wait-WS { param($m) $m.type -eq "system" -and $m.content -like "*chapter $chapter of*" } 5000
    if ($notice) { Pass "WebSocket: room got '$($notice.content)'" } else { Fail "WebSocket: no system notice within 5 s" }
}

Write-Host "[INFO] gRPC: the bridge's audit call shows up as an AUDIT line in the gRPC server log" -ForegroundColor Gray
Write-Host ""

# Step 5: The stored entry
Write-Host "Step 5: Verifying the library entry..." -ForegroundColor Yellow
try {
    $libraryResp = Invoke-RestMethod -Uri "$baseUrl/users/library" -Method GET -Headers $headers
    $entry = @($libraryResp.data | Where-Object { $_.manga_id -eq $mangaId })
    if ($entry.Count -eq 1 -and $entry[0].current_chapter -eq $chapter) {
        Pass "Library: $($entry[0].manga.title) is on chapter $chapter"
    }
    else { Fail "Library entry not updated" }
}
catch { Fail "Failed to retrieve library: $($_.Exception.Message)" }

# Cleanup
try {
    $unreg = [System.Text.Encoding]::ASCII.GetBytes("UNREGISTER")
    $udp.Send($unreg, $unreg.Length, $udpServer) | Out-Null
}
catch { }
$udp.Close()
if ($tcp) { $tcp.Close() }
if ($ws.State -eq 'Open') {
    try { $ws.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "done", [Threading.CancellationToken]::None).Wait(2000) | Out-Null } catch { }
}
$ws.Dispose()

Write-Host ""
if ($failures -gt 0) {
    Write-Host "=== INTEGRATION TEST: $failures check(s) FAILED ===" -ForegroundColor Red
    exit 1
}
Write-Host "=== INTEGRATION TEST: one HTTP update reached HTTP, TCP, UDP and WebSocket ===" -ForegroundColor Green
