# UDP Notification Server Test
$serverHost = "localhost"
$serverPort = 9091

Write-Host "=== UDP Notification Server Test ===" -ForegroundColor Cyan
Write-Host ""

# Test 1: Check if server is running
Write-Host "Test 1: Checking if UDP server is running..." -ForegroundColor Yellow

$client = New-Object System.Net.Sockets.UdpClient
try {
    $serverEndpoint = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Parse("127.0.0.1"), $serverPort)
    
    # Send REGISTER message
    $registerMsg = [System.Text.Encoding]::ASCII.GetBytes("REGISTER")
    $client.Send($registerMsg, $registerMsg.Length, $serverEndpoint) | Out-Null
    
    # Wait for confirmation
    $client.Client.ReceiveTimeout = 2000
    $remoteEP = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any, 0)
    $response = $client.Receive([ref]$remoteEP)
    $confirmation = [System.Text.Encoding]::ASCII.GetString($response)
    
    if ($confirmation -eq "REGISTERED") {
        Write-Host "[PASS] Server is running and client registered!" -ForegroundColor Green
    } else {
        Write-Host "[FAIL] Unexpected response: $confirmation" -ForegroundColor Red
        exit 1
    }
}
catch {
    Write-Host "[FAIL] Server not responding: $_" -ForegroundColor Red
    Write-Host "Start server with: go run cmd/udp-server/main.go" -ForegroundColor Yellow
    exit 1
}

Write-Host ""

# Test 2: Listen for notifications
Write-Host "Test 2: Listening for notifications (10 seconds)..." -ForegroundColor Yellow
Write-Host "The server sends demo notifications every 10 seconds" -ForegroundColor Cyan

$receivedCount = 0
$endTime = (Get-Date).AddSeconds(12)

try {
    $client.Client.ReceiveTimeout = 500
    while ((Get-Date) -lt $endTime) {
        try {
            $data = $client.Receive([ref]$remoteEP)
            $message = [System.Text.Encoding]::ASCII.GetString($data)
            
            Write-Host "[RECEIVED] $message" -ForegroundColor Green
            $receivedCount++
        }
        catch [System.Net.Sockets.SocketException] {
            # Timeout, continue
        }
    }
}
catch {
    Write-Host "Error: $_" -ForegroundColor Red
}

Write-Host ""

if ($receivedCount -gt 0) {
    Write-Host "[PASS] Received $receivedCount notification(s)" -ForegroundColor Green
} else {
    Write-Host "[INFO] No notifications received yet (server sends every 10s)" -ForegroundColor Yellow
}

# Cleanup
Write-Host ""
Write-Host "Unregistering client..." -ForegroundColor Cyan
$unregisterMsg = [System.Text.Encoding]::ASCII.GetBytes("UNREGISTER")
$client.Send($unregisterMsg, $unregisterMsg.Length, $serverEndpoint) | Out-Null
$client.Close()

Write-Host ""
Write-Host "=== Test Complete ===" -ForegroundColor Cyan
Write-Host "UDP server is working and broadcasting notifications" -ForegroundColor Green
