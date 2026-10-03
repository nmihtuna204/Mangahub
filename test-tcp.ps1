# TCP Server Test Script
# Tests the TCP Progress Sync Server: every progress line a client sends is
# relayed to all connected clients (the sender included).
# Exits 1 if any check fails.

$serverHost = "localhost"
$serverPort = 9090
$failures = 0
$runId = Get-Random

function Pass($msg) { Write-Host "[PASS] $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "[FAIL] $msg" -ForegroundColor Red; $script:failures++ }

function Connect-Client {
    $client = New-Object System.Net.Sockets.TcpClient
    $client.Connect($serverHost, $serverPort)
    $stream = $client.GetStream()
    $stream.ReadTimeout = 3000
    $writer = New-Object System.IO.StreamWriter($stream)
    $writer.AutoFlush = $true
    $reader = New-Object System.IO.StreamReader($stream)
    return @{ Client = $client; Writer = $writer; Reader = $reader }
}

function Send-Update($conn, $userId, $chapter) {
    $ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    $line = '{"user_id":"' + $userId + '","manga_id":"one-piece","chapter":' + $chapter + ',"timestamp":' + $ts + '}'
    Write-Host "  sending: $line" -ForegroundColor Gray
    $conn.Writer.WriteLine($line)
}

# Reads lines until the update from $userId arrives or a read times out.
# Other processes (e.g. the API's protocol bridge) may send updates too.
function Wait-Update($conn, $userId, $chapter) {
    try {
        while ($true) {
            $line = $conn.Reader.ReadLine()
            if ($null -eq $line) { return $false }   # connection closed
            try { $u = $line | ConvertFrom-Json } catch { continue }
            if ($u.user_id -eq $userId -and $u.chapter -eq $chapter) { return $true }
        }
    }
    catch { return $false }   # read timeout
}

Write-Host "=== TCP Progress Sync Server Test ===" -ForegroundColor Cyan
Write-Host ""

# Test 1: Server reachable
Write-Host "Test 1: Connecting to the TCP server on port $serverPort..." -ForegroundColor Yellow
try {
    $a = Connect-Client
    Pass "Server is running"
}
catch {
    Fail "Server is not running: $($_.Exception.Message)"
    Write-Host "Start it with: go run ./cmd/tcp-server" -ForegroundColor Yellow
    exit 1
}
Write-Host ""

# Test 2: The sender gets its own update back
Write-Host "Test 2: Single client sends an update..." -ForegroundColor Yellow
$userA = "ps-tcp-a-$runId"
Send-Update $a $userA 10
if (Wait-Update $a $userA 10) { Pass "The server relayed the update back to the sender" }
else { Fail "The update was not relayed within 3 s" }
Write-Host ""

# Test 3: Two clients see each other's updates
Write-Host "Test 3: Broadcast between two clients..." -ForegroundColor Yellow
$b = Connect-Client
Start-Sleep -Milliseconds 300   # let the server register client B
$userB = "ps-tcp-b-$runId"

Send-Update $a $userA 25
if (Wait-Update $b $userA 25) { Pass "Client B received client A's update" }
else { Fail "Client B did not receive client A's update" }

Send-Update $b $userB 50
if (Wait-Update $a $userB 50) { Pass "Client A received client B's update" }
else { Fail "Client A did not receive client B's update" }
Write-Host ""

# Test 4: Invalid input doesn't kill the connection
Write-Host "Test 4: Invalid JSON is ignored..." -ForegroundColor Yellow
$a.Writer.WriteLine("this is not json")
Send-Update $a $userA 26
if (Wait-Update $a $userA 26) { Pass "The connection survived a bad line" }
else { Fail "No relay after a bad line (connection dropped?)" }

foreach ($c in @($a, $b)) { $c.Client.Close() }

Write-Host ""
if ($failures -gt 0) {
    Write-Host "=== TCP test: $failures check(s) FAILED ===" -ForegroundColor Red
    exit 1
}
Write-Host "=== TCP test: all checks passed ===" -ForegroundColor Green
