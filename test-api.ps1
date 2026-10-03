# Phase 2 API Test Script
# Exercises the main REST endpoints. Safe to re-run (an existing test user is
# reused). Exits 1 if any check fails.
$baseUrl = "http://localhost:8080"
$failures = 0

function Pass($msg) { Write-Host "OK - $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "FAIL - $msg" -ForegroundColor Red; $script:failures++ }
function StatusOf($err) {
    if ($err.Exception.Response) { return [int]$err.Exception.Response.StatusCode }
    return 0
}

Write-Host "`n=== Phase 2 API Tests ===" -ForegroundColor Cyan
Write-Host "Base URL: $baseUrl`n" -ForegroundColor Gray

# Test 1: Register new user
Write-Host "Test 1: Register new user" -ForegroundColor Yellow
$registerBody = @{
    username = "testuser2"
    email    = "test2@example.com"
    password = "testpass123"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/auth/register" -Method Post -Body $registerBody -ContentType "application/json"
    Pass "User registered successfully"
    Write-Host "  Username: $($response.data.username)" -ForegroundColor Gray
}
catch {
    if ((StatusOf $_) -eq 409) {
        Pass "User already exists (registered by an earlier run): 409 Conflict"
    }
    else {
        Fail "Registration failed: $($_.Exception.Message)"
    }
}

# Test 2: Login
Write-Host "`nTest 2: Login" -ForegroundColor Yellow
$loginBody = @{
    username = "testuser2"
    password = "testpass123"
} | ConvertTo-Json

try {
    $loginResponse = Invoke-RestMethod -Uri "$baseUrl/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
    $token = $loginResponse.data.token
    Pass "Login successful"
    Write-Host "  Token: $($token.Substring(0, 20))..." -ForegroundColor Gray
    Write-Host "  User: $($loginResponse.data.user.username)" -ForegroundColor Gray
}
catch {
    Fail "Login failed: $($_.Exception.Message)"
    exit 1
}

# Test 3: List all manga
Write-Host "`nTest 3: List all manga" -ForegroundColor Yellow
$firstMangaId = $null
try {
    $mangaList = Invoke-RestMethod -Uri "$baseUrl/manga?limit=10" -Method Get
    if ($mangaList.data.data.Count -gt 0) {
        Pass "Manga list retrieved"
        Write-Host "  Total: $($mangaList.data.total)" -ForegroundColor Gray
        Write-Host "  Returned: $($mangaList.data.data.Count)" -ForegroundColor Gray
        Write-Host "  First manga: $($mangaList.data.data[0].title)" -ForegroundColor Gray
        $firstMangaId = $mangaList.data.data[0].id
    }
    else {
        Fail "Manga list is empty"
    }
}
catch {
    Fail "List manga failed: $($_.Exception.Message)"
}

# Test 4: Get specific manga
if ($firstMangaId) {
    Write-Host "`nTest 4: Get manga by ID" -ForegroundColor Yellow
    try {
        $manga = Invoke-RestMethod -Uri "$baseUrl/manga/$firstMangaId" -Method Get
        if ($manga.data.id -eq $firstMangaId) {
            Pass "Manga details retrieved"
            Write-Host "  Title: $($manga.data.title)" -ForegroundColor Gray
            Write-Host "  Author: $($manga.data.author)" -ForegroundColor Gray
            Write-Host "  Status: $($manga.data.status)" -ForegroundColor Gray
        }
        else {
            Fail "GET /manga/$firstMangaId returned another manga"
        }
    }
    catch {
        Fail "Get manga failed: $($_.Exception.Message)"
    }
}

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type"  = "application/json"
}

# Test 5: Add manga to library (protected route; adding again just updates)
if ($firstMangaId) {
    Write-Host "`nTest 5: Add manga to library (protected)" -ForegroundColor Yellow
    $progressBody = @{
        manga_id        = $firstMangaId
        current_chapter = 5
        status          = "reading"
        is_favorite     = $true
    } | ConvertTo-Json

    try {
        $addResponse = Invoke-RestMethod -Uri "$baseUrl/users/library" -Method Post -Headers $headers -Body $progressBody
        if ($addResponse.data.current_chapter -eq 5 -and $addResponse.data.status -eq "reading") {
            Pass "Manga added to library"
            Write-Host "  Current chapter: $($addResponse.data.current_chapter)" -ForegroundColor Gray
            Write-Host "  Status: $($addResponse.data.status)" -ForegroundColor Gray
        }
        else {
            Fail "Library entry = chapter $($addResponse.data.current_chapter), status $($addResponse.data.status)"
        }
    }
    catch {
        Fail "Add to library failed: $($_.Exception.Message)"
    }
}

# Test 6: Get user library (protected route)
Write-Host "`nTest 6: Get user library (protected)" -ForegroundColor Yellow
try {
    $library = Invoke-RestMethod -Uri "$baseUrl/users/library" -Method Get -Headers @{ "Authorization" = "Bearer $token" }
    $entry = $library.data | Where-Object { $_.manga_id -eq $firstMangaId }
    if ($entry) {
        Pass "Library retrieved and contains the added manga"
        Write-Host "  Items in library: $(@($library.data).Count)" -ForegroundColor Gray
        Write-Host "  Entry: $($entry.manga.title), chapter $($entry.current_chapter)" -ForegroundColor Gray
    }
    else {
        Fail "The added manga is not in the library"
    }
}
catch {
    Fail "Get library failed: $($_.Exception.Message)"
}

# Test 7: Update reading progress (protected route; triggers the protocol bridge)
if ($firstMangaId) {
    Write-Host "`nTest 7: Update reading progress (protected)" -ForegroundColor Yellow
    $updateBody = @{
        manga_id        = $firstMangaId
        current_chapter = 10
        status          = "reading"
    } | ConvertTo-Json

    try {
        $updateResponse = Invoke-RestMethod -Uri "$baseUrl/users/progress" -Method Put -Headers $headers -Body $updateBody
        if ($updateResponse.data.current_chapter -eq 10 -and $updateResponse.data.is_favorite) {
            Pass "Progress updated (and the favorite flag was kept)"
            Write-Host "  Current chapter: $($updateResponse.data.current_chapter)" -ForegroundColor Gray
            Write-Host "  Status: $($updateResponse.data.status)" -ForegroundColor Gray
        }
        else {
            Fail "After update: chapter $($updateResponse.data.current_chapter), favorite $($updateResponse.data.is_favorite)"
        }
    }
    catch {
        Fail "Update progress failed: $($_.Exception.Message)"
    }

    # Ratings have their own endpoint (a "rating" field in a progress update is ignored)
    Write-Host "`nTest 7b: Rate the manga" -ForegroundColor Yellow
    try {
        $rateBody = @{ rating = 9; review_text = "test-api.ps1" } | ConvertTo-Json
        $rated = Invoke-RestMethod -Uri "$baseUrl/manga/$firstMangaId/ratings" -Method Post -Headers $headers -Body $rateBody
        if ($rated.data.rating -eq 9) { Pass "Rating saved: $($rated.data.rating)/10" }
        else { Fail "Rating response: $($rated | ConvertTo-Json -Compress)" }
    }
    catch {
        Fail "Rating failed: $($_.Exception.Message)"
    }
}

# Test 8: Unauthorized access test
Write-Host "`nTest 8: Unauthorized access test" -ForegroundColor Yellow
try {
    Invoke-RestMethod -Uri "$baseUrl/users/library" -Method Get | Out-Null
    Fail "Should have been unauthorized!"
}
catch {
    if ((StatusOf $_) -eq 401) {
        Pass "Correctly rejected unauthorized access"
    }
    else {
        Fail "Wrong error: $($_.Exception.Message)"
    }
}

Write-Host ""
if ($failures -gt 0) {
    Write-Host "=== Phase 2 Tests: $failures check(s) FAILED ===" -ForegroundColor Red
    exit 1
}
Write-Host "=== Phase 2 Tests: all checks passed ===" -ForegroundColor Cyan
