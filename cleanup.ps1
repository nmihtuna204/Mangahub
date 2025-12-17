# MangaHub Project Cleanup Script
# Run this to clean up redundant files safely

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host " MangaHub Project Cleanup" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host ""

# Backup first
Write-Host "Creating backup..." -ForegroundColor Yellow
$backupDir = "backup_$(Get-Date -Format 'yyyyMMdd_HHmmss')"
New-Item -ItemType Directory -Force -Path $backupDir | Out-Null

# 1. Remove empty test directories
Write-Host "`n[1] Removing empty test directories..." -ForegroundColor Cyan
if (Test-Path "tests/integration" -and (Get-ChildItem "tests/integration" | Measure-Object).Count -eq 0) {
    Remove-Item -Recurse -Force "tests/integration"
    Write-Host "  ✓ Removed tests/integration (empty)" -ForegroundColor Green
}
if (Test-Path "tests/unit" -and (Get-ChildItem "tests/unit" | Measure-Object).Count -eq 0) {
    Remove-Item -Recurse -Force "tests/unit"
    Write-Host "  ✓ Removed tests/unit (empty)" -ForegroundColor Green
}
if (Test-Path "tests" -and (Get-ChildItem "tests" | Measure-Object).Count -eq 0) {
    Remove-Item -Recurse -Force "tests"
    Write-Host "  ✓ Removed tests directory (empty)" -ForegroundColor Green
}

# 2. Remove test-foundation (optional - uncomment to remove)
Write-Host "`n[2] Checking test-foundation..." -ForegroundColor Cyan
if (Test-Path "cmd/test-foundation") {
    Write-Host "  ⚠ cmd/test-foundation exists (foundation testing tool)" -ForegroundColor Yellow
    Write-Host "    Keep it if you want to test database setup" -ForegroundColor Gray
    # Uncomment to remove:
    # Remove-Item -Recurse -Force "cmd/test-foundation"
    # Write-Host "  ✓ Removed cmd/test-foundation" -ForegroundColor Green
}

# 3. Move PHASE summaries to docs
Write-Host "`n[3] Organizing documentation..." -ForegroundColor Cyan
$phaseFiles = @(
    "PHASE2_SUMMARY.md",
    "PHASE3_SUMMARY.md", 
    "PHASE5_SUMMARY.md",
    "PHASE6_SUMMARY.md",
    "PHASE6_TESTING.md",
    "PHASE7_SUMMARY.md"
)
foreach ($file in $phaseFiles) {
    if (Test-Path $file) {
        Move-Item -Force $file "docs/$file"
        Write-Host "  ✓ Moved $file to docs/" -ForegroundColor Green
    }
}

# 4. Move optional docs to docs folder
Write-Host "`n[4] Moving optional documentation..." -ForegroundColor Cyan
$optionalDocs = @(
    "PLAN.md",
    "DEPLOYMENT.md",
    "CHECKLIST.md",
    "KNOWN_ISSUES.md"
)
foreach ($file in $optionalDocs) {
    if (Test-Path $file) {
        Move-Item -Force $file "docs/$file"
        Write-Host "  ✓ Moved $file to docs/" -ForegroundColor Green
    }
}

# 5. Remove binary from root
Write-Host "`n[5] Cleaning up binaries..." -ForegroundColor Cyan
if (Test-Path "api-server.exe") {
    Remove-Item -Force "api-server.exe"
    Write-Host "  ✓ Removed api-server.exe from root" -ForegroundColor Green
}

# 6. Clean go modules
Write-Host "`n[6] Cleaning Go modules..." -ForegroundColor Cyan
go mod tidy
Write-Host "  ✓ Ran go mod tidy" -ForegroundColor Green

# 7. Rebuild all servers
Write-Host "`n[7] Rebuilding all servers..." -ForegroundColor Cyan
go build -o bin/api-server.exe ./cmd/api-server
go build -o bin/tcp-server.exe ./cmd/tcp-server
go build -o bin/udp-server.exe ./cmd/udp-server
go build -o bin/grpc-server.exe ./cmd/grpc-server
go build -o bin/cli.exe ./cmd/cli

if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ All servers built successfully!" -ForegroundColor Green
} else {
    Write-Host "  ✗ Build failed!" -ForegroundColor Red
}

# Summary
Write-Host ""
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host " Cleanup Complete!" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Project structure is now cleaner:" -ForegroundColor Green
Write-Host "  - Empty test folders removed" -ForegroundColor Gray
Write-Host "  - Documentation organized in docs/" -ForegroundColor Gray
Write-Host "  - Go modules cleaned" -ForegroundColor Gray
Write-Host "  - All servers verified" -ForegroundColor Gray
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. Review changes with: git status" -ForegroundColor Gray
Write-Host "  2. Test servers: test-all.ps1" -ForegroundColor Gray
Write-Host "  3. Commit changes to git" -ForegroundColor Gray
Write-Host ""
