# Interactive UDP Client
$serverHost = "localhost"
$serverPort = 9091

Write-Host "UDP Notification Client" -ForegroundColor Cyan
Write-Host "Connecting to ${serverHost}:${serverPort}..." -ForegroundColor Yellow

$client = New-Object System.Net.Sockets.UdpClient
$serverEndpoint = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Parse("127.0.0.1"), $serverPort)

try {
    # Register
    $registerMsg = [System.Text.Encoding]::ASCII.GetBytes("REGISTER")
    $client.Send($registerMsg, $registerMsg.Length, $serverEndpoint) | Out-Null
    
    $client.Client.ReceiveTimeout = 2000
    $remoteEP = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any, 0)
    $response = $client.Receive([ref]$remoteEP)
    $confirmation = [System.Text.Encoding]::ASCII.GetString($response)
    
    if ($confirmation -eq "REGISTERED") {
        Write-Host "Connected! Listening for notifications..." -ForegroundColor Green
        Write-Host "Press Ctrl+C to quit`n" -ForegroundColor Gray
    }
    
    $client.Client.ReceiveTimeout = 1000
    
    while ($true) {
        try {
            $data = $client.Receive([ref]$remoteEP)
            $message = [System.Text.Encoding]::ASCII.GetString($data)
            $timestamp = Get-Date -Format "HH:mm:ss"
            Write-Host "[$timestamp] $message" -ForegroundColor Cyan
        }
        catch [System.Net.Sockets.SocketException] {
            # Timeout, continue
        }
    }
}
catch {
    Write-Host "Error: $_" -ForegroundColor Red
}
finally {
    if ($client) {
        $unregisterMsg = [System.Text.Encoding]::ASCII.GetBytes("UNREGISTER")
        $client.Send($unregisterMsg, $unregisterMsg.Length, $serverEndpoint) | Out-Null
        $client.Close()
    }
}
