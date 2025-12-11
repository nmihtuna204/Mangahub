# Script to replace all GitHub imports with simple module name

Write-Host "Replacing all import paths..." -ForegroundColor Cyan

# Get all Go files
$goFiles = Get-ChildItem -Path . -Filter "*.go" -Recurse -File

$totalFiles = 0
$totalReplacements = 0

foreach ($file in $goFiles) {
    $content = Get-Content $file.FullName -Raw
    $originalContent = $content
    
    # Replace both yourusername and nmihtuna204
    $content = $content -replace 'github\.com/yourusername/mangahub', 'mangahub'
    $content = $content -replace 'github\.com/nmihtuna204/mangahub', 'mangahub'
    
    if ($content -ne $originalContent) {
        Set-Content -Path $file.FullName -Value $content -NoNewline
        $totalFiles++
        $changes = ([regex]::Matches($originalContent, 'github\.com/(yourusername|nmihtuna204)/mangahub')).Count
        $totalReplacements += $changes
        Write-Host "  Updated: $($file.FullName) ($changes replacements)" -ForegroundColor Green
    }
}

# Update go.mod
$goModPath = ".\go.mod"
if (Test-Path $goModPath) {
    $goModContent = Get-Content $goModPath -Raw
    $goModContent = $goModContent -replace 'module github\.com/nmihtuna204/mangahub', 'module mangahub'
    Set-Content -Path $goModPath -Value $goModContent -NoNewline
    Write-Host "  Updated: go.mod" -ForegroundColor Green
}

Write-Host "`nCompleted!" -ForegroundColor Green
Write-Host "Total files updated: $totalFiles" -ForegroundColor Yellow
Write-Host "Total replacements: $totalReplacements" -ForegroundColor Yellow

Write-Host "`nRunning 'go mod tidy'..." -ForegroundColor Cyan
go mod tidy

Write-Host "`nDone! All imports now use 'mangahub/...' instead of GitHub paths." -ForegroundColor Green
