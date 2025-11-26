# Phase 2 API Test Script
$baseUrl = "http://localhost:8080"

Write-Host "`n=== Phase 2 API Tests ===" -ForegroundColor Cyan
Write-Host "Base URL: $baseUrl`n" -ForegroundColor Gray

# Test 1: Register new user
Write-Host "Test 1: Register new user" -ForegroundColor Yellow
$registerBody = @{
    username = "testuser2"
    email = "test2@example.com"
    password = "testpass123"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/auth/register" -Method Post -Body $registerBody -ContentType "application/json"
    Write-Host "OK - User registered successfully" -ForegroundColor Green
    Write-Host "  Username: $($response.data.username)" -ForegroundColor Gray
}
catch {
    Write-Host "FAIL - Registration failed: $($_.Exception.Message)" -ForegroundColor Red
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
    Write-Host "OK - Login successful" -ForegroundColor Green
    Write-Host "  Token: $($token.Substring(0, 20))..." -ForegroundColor Gray
    Write-Host "  User: $($loginResponse.data.user.username)" -ForegroundColor Gray
}
catch {
    Write-Host "FAIL - Login failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test 3: List all manga
Write-Host "`nTest 3: List all manga" -ForegroundColor Yellow
try {
    $mangaList = Invoke-RestMethod -Uri "$baseUrl/manga?limit=10" -Method Get
    Write-Host "OK - Manga list retrieved" -ForegroundColor Green
    Write-Host "  Total: $($mangaList.data.total)" -ForegroundColor Gray
    Write-Host "  Returned: $($mangaList.data.data.Count)" -ForegroundColor Gray
    $firstMangaId = $null
    if ($mangaList.data.data.Count -gt 0) {
        Write-Host "  First manga: $($mangaList.data.data[0].title)" -ForegroundColor Gray
        $firstMangaId = $mangaList.data.data[0].id
    }
}
catch {
    Write-Host "FAIL - List manga failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 4: Get specific manga
if ($firstMangaId) {
    Write-Host "`nTest 4: Get manga by ID" -ForegroundColor Yellow
    try {
        $manga = Invoke-RestMethod -Uri "$baseUrl/manga/$firstMangaId" -Method Get
        Write-Host "OK - Manga details retrieved" -ForegroundColor Green
        Write-Host "  Title: $($manga.data.title)" -ForegroundColor Gray
        Write-Host "  Author: $($manga.data.author)" -ForegroundColor Gray
        Write-Host "  Status: $($manga.data.status)" -ForegroundColor Gray
    }
    catch {
        Write-Host "FAIL - Get manga failed: $($_.Exception.Message)" -ForegroundColor Red
    }
}

# Test 5: Add manga to library (protected route)
if ($firstMangaId -and $token) {
    Write-Host "`nTest 5: Add manga to library (protected)" -ForegroundColor Yellow
    $headers = @{
        "Authorization" = "Bearer $token"
        "Content-Type" = "application/json"
    }
    $progressBody = @{
        manga_id = $firstMangaId
        current_chapter = 5
        status = "reading"
        is_favorite = $true
    } | ConvertTo-Json
    
    try {
        $addResponse = Invoke-RestMethod -Uri "$baseUrl/users/library" -Method Post -Headers $headers -Body $progressBody
        Write-Host "OK - Manga added to library" -ForegroundColor Green
        Write-Host "  Current chapter: $($addResponse.data.current_chapter)" -ForegroundColor Gray
        Write-Host "  Status: $($addResponse.data.status)" -ForegroundColor Gray
    }
    catch {
        Write-Host "FAIL - Add to library failed: $($_.Exception.Message)" -ForegroundColor Red
    }
}

# Test 6: Get user library (protected route)
if ($token) {
    Write-Host "`nTest 6: Get user library (protected)" -ForegroundColor Yellow
    $headers = @{
        "Authorization" = "Bearer $token"
    }
    
    try {
        $library = Invoke-RestMethod -Uri "$baseUrl/users/library" -Method Get -Headers $headers
        Write-Host "OK - Library retrieved" -ForegroundColor Green
        Write-Host "  Items in library: $($library.data.Count)" -ForegroundColor Gray
        if ($library.data.Count -gt 0) {
            Write-Host "  First item: $($library.data[0].manga.title)" -ForegroundColor Gray
        }
    }
    catch {
        Write-Host "FAIL - Get library failed: $($_.Exception.Message)" -ForegroundColor Red
    }
}

# Test 7: Update reading progress (protected route)
if ($firstMangaId -and $token) {
    Write-Host "`nTest 7: Update reading progress (protected)" -ForegroundColor Yellow
    $headers = @{
        "Authorization" = "Bearer $token"
        "Content-Type" = "application/json"
    }
    $updateBody = @{
        manga_id = $firstMangaId
        current_chapter = 10
        status = "reading"
        rating = 9
    } | ConvertTo-Json
    
    try {
        $updateResponse = Invoke-RestMethod -Uri "$baseUrl/users/progress" -Method Put -Headers $headers -Body $updateBody
        Write-Host "OK - Progress updated" -ForegroundColor Green
        Write-Host "  Current chapter: $($updateResponse.data.current_chapter)" -ForegroundColor Gray
        Write-Host "  Rating: $($updateResponse.data.rating)" -ForegroundColor Gray
    }
    catch {
        Write-Host "FAIL - Update progress failed: $($_.Exception.Message)" -ForegroundColor Red
    }
}

# Test 8: Unauthorized access test
Write-Host "`nTest 8: Unauthorized access test" -ForegroundColor Yellow
try {
    $unauthorized = Invoke-RestMethod -Uri "$baseUrl/users/library" -Method Get
    Write-Host "FAIL - Should have been unauthorized!" -ForegroundColor Red
}
catch {
    if ($_.Exception.Response.StatusCode -eq 401) {
        Write-Host "OK - Correctly rejected unauthorized access" -ForegroundColor Green
    }
    else {
        Write-Host "FAIL - Wrong error: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host "`n=== Phase 2 Tests Complete ===" -ForegroundColor Cyan
