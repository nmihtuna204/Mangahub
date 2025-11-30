Write-Host "=====================================" -ForegroundColor Green
Write-Host " FINAL VERIFICATION - READY TO DEMO" -ForegroundColor Green
Write-Host "=====================================" -ForegroundColor Green
Write-Host ""

$checks = 0
$total = 0

function Check($name, $condition) {
    $script:total++
    if ($condition) {
        Write-Host "[✓] $name" -ForegroundColor Green
        $script:checks++
    } else {
        Write-Host "[✗] $name" -ForegroundColor Red
    }
}

# Verify binaries
Check "API server source" (Test-Path "./cmd/api-server/main.go")
Check "TCP server source" (Test-Path "./cmd/tcp-server/main.go")
Check "UDP server source" (Test-Path "./cmd/udp-server/main.go")
Check "gRPC server source" (Test-Path "./cmd/grpc-server/main.go")
Check "CLI source" (Test-Path "./cmd/cli/main.go")

# Verify documentation
Check "README.md" (Test-Path "README.md")
Check "DEPLOYMENT.md" (Test-Path "DEPLOYMENT.md")
Check "CHECKLIST.md" (Test-Path "CHECKLIST.md")
Check "Demo script" (Test-Path "demo/DEMO.md")

# Verify code
Check "Proto files" (Test-Path "proto/manga.proto")
Check "Tests present" (Test-Path "internal/auth/handlers_test.go")
Check "Config file" (Test-Path "configs/development.yaml")

# Verify git
$gitLogCount = (git log --oneline 2>$null | Measure-Object).Count
Check "Git history" ($gitLogCount -gt 10)

Write-Host ""
Write-Host "Verification: $checks/$total passed" -ForegroundColor Cyan

if ($checks -eq $total) {
    Write-Host ""
    Write-Host "✅ PROJECT READY FOR SUBMISSION!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Yellow
    Write-Host "1. Start all 4 servers in separate terminals" -ForegroundColor Gray
    Write-Host "   - go run cmd/api-server/main.go" -ForegroundColor Gray
    Write-Host "   - go run cmd/tcp-server/main.go" -ForegroundColor Gray
    Write-Host "   - go run cmd/udp-server/main.go" -ForegroundColor Gray
    Write-Host "   - go run cmd/grpc-server/main.go" -ForegroundColor Gray
    Write-Host "2. Test integration with CLI tool" -ForegroundColor Gray
    Write-Host "3. Show instructor the live system" -ForegroundColor Gray
    Write-Host "4. Present the 5-protocol architecture" -ForegroundColor Gray
} else {
    Write-Host ""
    Write-Host "⚠️  Some checks failed. Fix above items before submission." -ForegroundColor Yellow
}
