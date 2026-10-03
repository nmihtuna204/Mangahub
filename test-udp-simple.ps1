# UDP Notification Server Test
# Registers a subscriber, has a second socket ask the server to broadcast a
# notification, and checks that the subscriber receives it (and stops
# receiving after UNREGISTER). Exits 1 if any check fails.
#
# Note: `udp-server -demo` also sends a sample notification every 10 s, but
# that is opt-in, so this test sends its own instead of waiting for one.

$serverPort = 9091
$failures = 0
$server = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Parse("127.0.0.1"), $serverPort)

function Pass($msg) { Write-Host "[PASS] $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "[FAIL] $msg" -ForegroundColor Red; $script:failures++ }

function Send-Text($client, $text) {
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($text)
    $client.Send($bytes, $bytes.Length, $server) | Out-Null
}

# Returns the next datagram as text, or $null after $timeoutMs
function Receive-Text($client, $timeoutMs) {
    $client.Client.ReceiveTimeout = $timeoutMs
    $from = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any, 0)
    try { return [System.Text.Encoding]::UTF8.GetString($client.Receive([ref]$from)) }
    catch [System.Net.Sockets.SocketException] { return $null }
}

# Waits for a notification whose message is $text, skipping other datagrams
# (e.g. progress updates from other users)
function Wait-Notification($client, $text, $timeoutMs) {
    $deadline = (Get-Date).AddMilliseconds($timeoutMs)
    while ((Get-Date) -lt $deadline) {
        $left = [int]($deadline - (Get-Date)).TotalMilliseconds
        if ($left -lt 1) { break }
        $data = Receive-Text $client $left
        if ($null -eq $data) { break }
        try { $n = $data | ConvertFrom-Json } catch { continue }
        if ($n.message -eq $text) { return $n }
    }
    return $null
}

function Send-Broadcast($text) {
    $payload = @{
        type      = "system"
        manga_id  = ""
        message   = $text
        timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    } | ConvertTo-Json -Compress
    $sender = New-Object System.Net.Sockets.UdpClient
    Send-Text $sender "BROADCAST $payload"
    $sender.Close()
}

Write-Host "=== UDP Notification Server Test ===" -ForegroundColor Cyan
Write-Host ""

# Test 1: Register
Write-Host "Test 1: Registering a subscriber..." -ForegroundColor Yellow
$client = New-Object System.Net.Sockets.UdpClient
try { Send-Text $client "REGISTER" } catch { }
$reply = Receive-Text $client 2000
if ($reply -eq "REGISTERED") {
    Pass "Server is running and the subscriber is registered"
}
else {
    Fail "No REGISTERED reply (got '$reply')"
    Write-Host "Start the server with: go run ./cmd/udp-server" -ForegroundColor Yellow
    exit 1
}
Write-Host ""

# Test 2: Receive a broadcast
Write-Host "Test 2: Broadcasting a notification from another socket..." -ForegroundColor Yellow
$text = "ps-udp-test-$(Get-Random)"
Send-Broadcast $text
$n = Wait-Notification $client $text 3000
if ($n) {
    Pass "Subscriber received the notification (type=$($n.type), message=$($n.message))"
}
else {
    Fail "Subscriber did not receive the broadcast within 3 s"
}
Write-Host ""

# Test 3: Unregister
Write-Host "Test 3: Unregistering..." -ForegroundColor Yellow
Send-Text $client "UNREGISTER"
$reply = $null
$deadline = (Get-Date).AddSeconds(2)
while ($reply -ne "UNREGISTERED" -and (Get-Date) -lt $deadline) {
    $reply = Receive-Text $client 2000
    if ($null -eq $reply) { break }
}
if ($reply -eq "UNREGISTERED") { Pass "Server confirmed UNREGISTER" }
else { Fail "No UNREGISTERED reply" }

$text2 = "ps-udp-after-unregister-$(Get-Random)"
Send-Broadcast $text2
if (Wait-Notification $client $text2 1000) { Fail "Still receiving notifications after UNREGISTER" }
else { Pass "No notifications after UNREGISTER" }

$client.Close()

Write-Host ""
if ($failures -gt 0) {
    Write-Host "=== UDP test: $failures check(s) FAILED ===" -ForegroundColor Red
    exit 1
}
Write-Host "=== UDP test: all checks passed ===" -ForegroundColor Green
