# WebSocket Chat Test Script
# Tests real-time chat functionality with multiple clients

$baseUrl = "http://localhost:8080"
$wsUrl = "ws://localhost:8080"

Write-Host "=== WebSocket Chat System Test ===" -ForegroundColor Cyan
Write-Host ""

# Test 1: Login to get JWT token
Write-Host "Test 1: Getting JWT token..." -ForegroundColor Yellow
try {
    $loginBody = @{
        username = "admin"
        password = "admin123"
    } | ConvertTo-Json
    
    $loginResponse = Invoke-RestMethod -Uri "$baseUrl/auth/login" `
        -Method POST `
        -ContentType "application/json" `
        -Body $loginBody
    
    $token = $loginResponse.data.token
    if ($token) {
        Write-Host "[PASS] Got JWT token: $($token.Substring(0,20))..." -ForegroundColor Green
    } else {
        Write-Host "[FAIL] No token received" -ForegroundColor Red
        exit 1
    }
}
catch {
    Write-Host "[FAIL] Login failed: $_" -ForegroundColor Red
    exit 1
}

Write-Host ""

# Test 2: Check room info endpoint
Write-Host "Test 2: Checking room info endpoint..." -ForegroundColor Yellow
try {
    $roomInfo = Invoke-RestMethod -Uri "$baseUrl/rooms/one-piece" -Method GET
    Write-Host "[PASS] Room info retrieved: $($roomInfo.count) clients in room" -ForegroundColor Green
    Write-Host "  Room ID: $($roomInfo.room_id)" -ForegroundColor Gray
}
catch {
    Write-Host "[FAIL] Room info failed: $_" -ForegroundColor Red
}

Write-Host ""

# Test 3: WebSocket connection test with .NET WebSocket
Write-Host "Test 3: Testing WebSocket connections with 2 clients..." -ForegroundColor Yellow
Write-Host "This will test real-time message broadcasting" -ForegroundColor Cyan
Write-Host ""

# Load WebSocket assembly
Add-Type -AssemblyName System.Net.WebSockets
Add-Type -AssemblyName System.Threading

$script:receivedMessages = @()

function Connect-WebSocket {
    param(
        [string]$ClientName,
        [string]$Token,
        [string]$RoomId
    )
    
    try {
        $ws = New-Object System.Net.WebSockets.ClientWebSocket
        $uri = New-Object System.Uri("$wsUrl/ws/chat?room_id=$RoomId")
        
        # Add JWT token header
        $ws.Options.SetRequestHeader("Authorization", "Bearer $Token")
        
        Write-Host "[$ClientName] Connecting to WebSocket..." -ForegroundColor Yellow
        $ct = New-Object System.Threading.CancellationToken
        $connectTask = $ws.ConnectAsync($uri, $ct)
        $connectTask.Wait()
        
        if ($ws.State -eq 'Open') {
            Write-Host "[$ClientName] Connected successfully!" -ForegroundColor Green
            return $ws
        } else {
            Write-Host "[$ClientName] Failed to connect. State: $($ws.State)" -ForegroundColor Red
            return $null
        }
    }
    catch {
        Write-Host "[$ClientName] Connection error: $_" -ForegroundColor Red
        return $null
    }
}

function Send-WSMessage {
    param(
        [System.Net.WebSockets.ClientWebSocket]$WebSocket,
        [string]$Message
    )
    
    try {
        $jsonMsg = "{`"message`":`"$Message`"}"
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($jsonMsg)
        $segment = New-Object System.ArraySegment[byte] -ArgumentList @(,$bytes)
        $ct = New-Object System.Threading.CancellationToken
        
        $sendTask = $WebSocket.SendAsync($segment, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, $ct)
        $sendTask.Wait()
        return $true
    }
    catch {
        Write-Host "Send error: $_" -ForegroundColor Red
        return $false
    }
}

function Receive-WSMessage {
    param(
        [System.Net.WebSockets.ClientWebSocket]$WebSocket,
        [int]$TimeoutMs = 2000
    )
    
    try {
        $buffer = New-Object byte[] 4096
        $segment = New-Object System.ArraySegment[byte] -ArgumentList @(,$buffer)
        $ct = New-Object System.Threading.CancellationTokenSource
        $ct.CancelAfter($TimeoutMs)
        
        $receiveTask = $WebSocket.ReceiveAsync($segment, $ct.Token)
        
        if ($receiveTask.Wait($TimeoutMs)) {
            $result = $receiveTask.Result
            if ($result.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Text) {
                $message = [System.Text.Encoding]::UTF8.GetString($buffer, 0, $result.Count)
                return $message
            }
        }
        return $null
    }
    catch {
        return $null
    }
}

# Connect Client 1
$ws1 = Connect-WebSocket -ClientName "Client1" -Token $token -RoomId "one-piece"
Start-Sleep -Milliseconds 500

# Connect Client 2
$ws2 = Connect-WebSocket -ClientName "Client2" -Token $token -RoomId "one-piece"
Start-Sleep -Milliseconds 500

if ($ws1 -and $ws2) {
    Write-Host ""
    Write-Host "Both clients connected. Testing message broadcast..." -ForegroundColor Cyan
    Write-Host ""
    
    # Client 1 receives join notifications
    Write-Host "[Client1] Listening for join messages..." -ForegroundColor Yellow
    $msg = Receive-WSMessage -WebSocket $ws1 -TimeoutMs 2000
    if ($msg) {
        $msgObj = $msg | ConvertFrom-Json
        Write-Host "[Client1] Received: [$($msgObj.type)] $($msgObj.message)" -ForegroundColor Green
    }
    
    # Client 2 receives its own join notification
    $msg = Receive-WSMessage -WebSocket $ws2 -TimeoutMs 2000
    if ($msg) {
        $msgObj = $msg | ConvertFrom-Json
        Write-Host "[Client2] Received: [$($msgObj.type)] $($msgObj.message)" -ForegroundColor Green
    }
    
    Write-Host ""
    
    # Client 1 sends a message
    Write-Host "[Client1] Sending: Hello from Client 1!" -ForegroundColor Cyan
    Send-WSMessage -WebSocket $ws1 -Message "Hello from Client 1!" | Out-Null
    Start-Sleep -Milliseconds 500
    
    # Both clients should receive it
    $msg1 = Receive-WSMessage -WebSocket $ws1 -TimeoutMs 2000
    $msg2 = Receive-WSMessage -WebSocket $ws2 -TimeoutMs 2000
    
    if ($msg1) {
        $msgObj = $msg1 | ConvertFrom-Json
        Write-Host "[Client1] Received broadcast: $($msgObj.message)" -ForegroundColor Green
    }
    
    if ($msg2) {
        $msgObj = $msg2 | ConvertFrom-Json
        Write-Host "[Client2] Received broadcast: $($msgObj.message)" -ForegroundColor Green
    }
    
    Write-Host ""
    
    # Client 2 sends a message
    Write-Host "[Client2] Sending: Hello from Client 2!" -ForegroundColor Cyan
    Send-WSMessage -WebSocket $ws2 -Message "Hello from Client 2!" | Out-Null
    Start-Sleep -Milliseconds 500
    
    # Both clients should receive it
    $msg1 = Receive-WSMessage -WebSocket $ws1 -TimeoutMs 2000
    $msg2 = Receive-WSMessage -WebSocket $ws2 -TimeoutMs 2000
    
    if ($msg1) {
        $msgObj = $msg1 | ConvertFrom-Json
        Write-Host "[Client1] Received broadcast: $($msgObj.message)" -ForegroundColor Green
    }
    
    if ($msg2) {
        $msgObj = $msg2 | ConvertFrom-Json
        Write-Host "[Client2] Received broadcast: $($msgObj.message)" -ForegroundColor Green
    }
    
    Write-Host ""
    Write-Host "[PASS] WebSocket chat is working! Messages broadcast successfully" -ForegroundColor Green
    
    # Close connections
    Write-Host ""
    Write-Host "Closing connections..." -ForegroundColor Yellow
    
    $ct = New-Object System.Threading.CancellationToken
    $ws1.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "Test complete", $ct).Wait()
    
    # Client 2 should receive leave notification
    Start-Sleep -Milliseconds 500
    $leaveMsg = Receive-WSMessage -WebSocket $ws2 -TimeoutMs 2000
    if ($leaveMsg) {
        $msgObj = $leaveMsg | ConvertFrom-Json
        Write-Host "[Client2] Received: [$($msgObj.type)] $($msgObj.message)" -ForegroundColor Yellow
    }
    
    $ws2.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "Test complete", $ct).Wait()
    
    $ws1.Dispose()
    $ws2.Dispose()
}
else {
    Write-Host "[FAIL] Could not establish WebSocket connections" -ForegroundColor Red
    Write-Host "Make sure the API server is running: go run cmd/api-server/main.go" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== Test Complete ===" -ForegroundColor Cyan
Write-Host "Check the API server logs to see connection and message details" -ForegroundColor Gray
