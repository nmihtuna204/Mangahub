# Phase 2 Manual API Tests with curl
# Run this after starting the server with: go run ./cmd/api-server
# Shows each request as a curl command, then checks the response.
# Safe to re-run. Exits 1 if any check fails.
#
# Note: uses curl.exe explicitly - in Windows PowerShell plain `curl` is an
# alias for Invoke-WebRequest and does not accept curl's flags.

$baseUrl = "http://localhost:8080"
$curl = "curl.exe"
$failures = 0

function Pass($msg) { Write-Host "[PASS] $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "[FAIL] $msg" -ForegroundColor Red; $script:failures++ }

# Runs curl and returns @{ Code = <HTTP status>; Body = <parsed JSON or $null> }
function Invoke-Curl([string[]]$curlArgs) {
    $out = & $curl -s -w "\n%{http_code}" @curlArgs   # curl expands \n itself
    $lines = @($out)
    $code = [int]$lines[-1]
    $text = ($lines[0..($lines.Count - 2)] -join "`n")
    $body = $null
    try { $body = $text | ConvertFrom-Json } catch { }
    return @{ Code = $code; Body = $body; Text = $text }
}

function Show($title, $command) {
    Write-Host $title -ForegroundColor Yellow
    Write-Host "  $command" -ForegroundColor DarkGray
}

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "  Phase 2 Manual API Tests (curl)" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

# 1: Register (the user may already exist from a previous run)
Show "1. POST /auth/register - Register new user" "curl -X POST $baseUrl/auth/register -H 'Content-Type: application/json' -d '{...}'"
$r = Invoke-Curl @("-X", "POST", "$baseUrl/auth/register", "-H", "Content-Type: application/json",
    "-d", '{\"username\":\"testcurl\",\"email\":\"testcurl@example.com\",\"password\":\"password123\"}')
if ($r.Code -eq 201) { Pass "201 Created, user id $($r.Body.data.id)" }
elseif ($r.Code -eq 409) { Pass "409 Conflict: testcurl already exists (earlier run)" }
else { Fail "register: HTTP $($r.Code) $($r.Text)" }
Write-Host ""

# 2: Login
Show "2. POST /auth/login - Login and get JWT token" "curl -X POST $baseUrl/auth/login -H 'Content-Type: application/json' -d '{...}'"
$r = Invoke-Curl @("-X", "POST", "$baseUrl/auth/login", "-H", "Content-Type: application/json",
    "-d", '{\"username\":\"testcurl\",\"password\":\"password123\"}')
$token = $r.Body.data.token
if ($r.Code -eq 200 -and $token) {
    Pass "200 OK, token $($token.Substring(0, [Math]::Min(30, $token.Length)))... (expires $($r.Body.data.expires_at))"
}
else {
    Fail "login: HTTP $($r.Code) $($r.Text)"
    Write-Host "Aborting the protected tests" -ForegroundColor Red
    exit 1
}
Write-Host ""

# 3: List manga (no auth)
Show "3. GET /manga?limit=5 - List manga (no auth required)" "curl $baseUrl/manga?limit=5"
$r = Invoke-Curl @("$baseUrl/manga?limit=5")
$firstMangaId = $null
if ($r.Code -eq 200 -and @($r.Body.data.data).Count -eq 5) {
    $firstMangaId = $r.Body.data.data[0].id
    Pass "200 OK, $($r.Body.data.total) manga in total, 5 returned; first: $($r.Body.data.data[0].title)"
}
else { Fail "list manga: HTTP $($r.Code)" }
Write-Host ""

if ($firstMangaId) {
    # 4: Get specific manga
    Show "4. GET /manga/:id - Get manga details (no auth)" "curl $baseUrl/manga/$firstMangaId"
    $r = Invoke-Curl @("$baseUrl/manga/$firstMangaId")
    if ($r.Code -eq 200 -and $r.Body.data.id -eq $firstMangaId) { Pass "200 OK: $($r.Body.data.title) by $($r.Body.data.author)" }
    else { Fail "get manga: HTTP $($r.Code)" }
    Write-Host ""

    # 5: Add to library (protected; adding again just updates the entry)
    Show "5. POST /users/library - Add manga to library (requires JWT)" "curl -X POST $baseUrl/users/library -H 'Authorization: Bearer <token>' -d '{...}'"
    $addBody = '{\"manga_id\":\"' + $firstMangaId + '\",\"current_chapter\":5,\"status\":\"reading\",\"is_favorite\":true}'
    $r = Invoke-Curl @("-X", "POST", "$baseUrl/users/library", "-H", "Authorization: Bearer $token", "-H", "Content-Type: application/json", "-d", $addBody)
    if ($r.Code -eq 201 -and $r.Body.data.current_chapter -eq 5) { Pass "201 Created: chapter 5, status $($r.Body.data.status)" }
    else { Fail "add to library: HTTP $($r.Code) $($r.Text)" }
    Write-Host ""

    # 6: Get library (protected)
    Show "6. GET /users/library - Get user's library (requires JWT)" "curl $baseUrl/users/library -H 'Authorization: Bearer <token>'"
    $r = Invoke-Curl @("$baseUrl/users/library", "-H", "Authorization: Bearer $token")
    if ($r.Code -eq 200 -and (@($r.Body.data) | Where-Object { $_.manga_id -eq $firstMangaId })) { Pass "200 OK, $(@($r.Body.data).Count) item(s), including the added manga" }
    else { Fail "get library: HTTP $($r.Code)" }
    Write-Host ""

    # 7: Update progress (protected)
    Show "7. PUT /users/progress - Update reading progress (requires JWT)" "curl -X PUT $baseUrl/users/progress -H 'Authorization: Bearer <token>' -d '{...}'"
    $updateBody = '{\"manga_id\":\"' + $firstMangaId + '\",\"current_chapter\":15,\"status\":\"reading\"}'
    $r = Invoke-Curl @("-X", "PUT", "$baseUrl/users/progress", "-H", "Authorization: Bearer $token", "-H", "Content-Type: application/json", "-d", $updateBody)
    if ($r.Code -eq 200 -and $r.Body.data.current_chapter -eq 15) { Pass "200 OK: now on chapter 15" }
    else { Fail "update progress: HTTP $($r.Code) $($r.Text)" }
    Write-Host ""
}

# 8: Unauthorized access
Show "8. GET /users/library - Test unauthorized access (no token)" "curl $baseUrl/users/library"
$r = Invoke-Curl @("$baseUrl/users/library")
if ($r.Code -eq 401 -and $r.Body.error.code -eq "UNAUTHORIZED") { Pass "401 Unauthorized: $($r.Body.error.message)" }
else { Fail "no token: HTTP $($r.Code), want 401" }
Write-Host ""

Write-Host "========================================" -ForegroundColor Cyan
if ($failures -gt 0) {
    Write-Host "  $failures check(s) FAILED" -ForegroundColor Red
    Write-Host "========================================`n" -ForegroundColor Cyan
    exit 1
}
Write-Host "  All Manual Tests Passed" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan
