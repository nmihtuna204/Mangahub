# Simple TCP Client Test
# Use this to manually test the TCP server

$serverHost = "localhost"
$serverPort = 9090

Write-Host "Connecting to TCP server at ${serverHost}:${serverPort}..." -ForegroundColor Cyan

try {
    $client = New-Object System.Net.Sockets.TcpClient
    $client.Connect($serverHost, $serverPort)
    
    if ($client.Connected) {
        Write-Host "Connected! Type JSON messages and press Enter to send." -ForegroundColor Green
        Write-Host "Example: {`"user_id`":`"u1`",`"manga_id`":`"one-piece`",`"chapter`":10,`"timestamp`":1700000000}" -ForegroundColor Yellow
        Write-Host "Press Ctrl+C to quit.`n" -ForegroundColor Gray
        
        $stream = $client.GetStream()
        $writer = New-Object System.IO.StreamWriter($stream)
        $reader = New-Object System.IO.StreamReader($stream)
        $writer.AutoFlush = $true
        
        # Start a background job to read from server
        $readJob = Start-Job -ScriptBlock {
            param($stream)
            $reader = New-Object System.IO.StreamReader($stream)
            while ($true) {
                try {
                    $line = $reader.ReadLine()
                    if ($line) {
                        Write-Output ">> Received: $line"
                    }
                }
                catch {
                    break
                }
            }
        } -ArgumentList $stream
        
        # Read from user and send to server
        while ($true) {
            $message = Read-Host "Send"
            if ($message) {
                $writer.WriteLine($message)
            }
            
            # Check for received messages
            $output = Receive-Job -Job $readJob
            if ($output) {
                $output | ForEach-Object { Write-Host $_ -ForegroundColor Green }
            }
        }
    }
}
catch {
    Write-Host "Error: $_" -ForegroundColor Red
    Write-Host "`nMake sure the TCP server is running: go run cmd/tcp-server/main.go" -ForegroundColor Yellow
}
finally {
    if ($client) { $client.Close() }
}
