# Phase 2 Manual API Tests with curl
# Run this after starting the server with: go run cmd/api-server/main.go

$baseUrl = "http://localhost:8080"

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "  Phase 2 Manual API Tests (curl)" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

# Test 1: Register
Write-Host "1. POST /auth/register - Register n ew user" -ForegroundColor Yellow
Write-Host "Command:" -ForegroundColor Gray
$registerCmd = "curl -X POST `"$baseUrl/auth/register`" -H `"Content-Type: application/json`" -d `"{\\`"username\\`":\\`"testcurl\\`",\\`"email\\`":\\`"testcurl@example.com\\`",\\`"password\\`":\\`"password123\\`"}`""
Write-Host $registerCmd -ForegroundColor DarkGray
Write-Host "`nResponse:" -ForegroundColor Gray
$registerResponse = curl -X POST "$baseUrl/auth/register" -H "Content-Type: application/json" -d '{\"username\":\"testcurl\",\"email\":\"testcurl@example.com\",\"password\":\"password123\"}' 2>$null
Write-Host $registerResponse -ForegroundColor Green
Write-Host "`n"

# Test 2: Login
Write-Host "2. POST /auth/login - Login and get JWT token" -ForegroundColor Yellow
Write-Host "Command:" -ForegroundColor Gray
$loginCmd = "curl -X POST `"$baseUrl/auth/login`" -H `"Content-Type: application/json`" -d `"{\\`"username\\`":\\`"testcurl\\`",\\`"password\\`":\\`"password123\\`"}`""
Write-Host $loginCmd -ForegroundColor DarkGray
Write-Host "`nResponse:" -ForegroundColor Gray
$loginResponse = curl -X POST "$baseUrl/auth/login" -H "Content-Type: application/json" -d '{\"username\":\"testcurl\",\"password\":\"password123\"}' 2>$null | ConvertFrom-Json
Write-Host ($loginResponse | ConvertTo-Json -Depth 5) -ForegroundColor Green

$token = $loginResponse.data.token
Write-Host "`nExtracted Token: $($token.Substring(0,30))..." -ForegroundColor Cyan
Write-Host "`n"

# Test 3: List Manga (no auth)
Write-Host "3. GET /manga?limit=5 - List manga (no auth required)" -ForegroundColor Yellow
Write-Host "Command:" -ForegroundColor Gray
$listCmd = "curl `"$baseUrl/manga?limit=5`""
Write-Host $listCmd -ForegroundColor DarkGray
Write-Host "`nResponse:" -ForegroundColor Gray
$mangaList = curl "$baseUrl/manga?limit=5" 2>$null | ConvertFrom-Json
Write-Host ($mangaList | ConvertTo-Json -Depth 5) -ForegroundColor Green

$firstMangaId = $mangaList.data.data[0].id
Write-Host "`nFirst Manga ID: $firstMangaId" -ForegroundColor Cyan
Write-Host "First Manga Title: $($mangaList.data.data[0].title)" -ForegroundColor Cyan
Write-Host "`n"

# Test 4: Get specific manga
Write-Host "4. GET /manga/:id - Get manga details (no auth)" -ForegroundColor Yellow
Write-Host "Command:" -ForegroundColor Gray
$getMangaCmd = "curl `"$baseUrl/manga/$firstMangaId`""
Write-Host $getMangaCmd -ForegroundColor DarkGray
Write-Host "`nResponse:" -ForegroundColor Gray
$manga = curl "$baseUrl/manga/$firstMangaId" 2>$null | ConvertFrom-Json
Write-Host ($manga | ConvertTo-Json -Depth 5) -ForegroundColor Green
Write-Host "`n"

# Test 5: Add to library (protected)
Write-Host "5. POST /users/library - Add manga to library (requires JWT)" -ForegroundColor Yellow
Write-Host "Command:" -ForegroundColor Gray
$addLibCmd = "curl -X POST `"$baseUrl/users/library`" -H `"Authorization: Bearer <token>`" -H `"Content-Type: application/json`" -d `"{\\`"manga_id\\`":\\`"$firstMangaId\\`",\\`"current_chapter\\`":5,\\`"status\\`":\\`"reading\\`",\\`"is_favorite\\`":true}`""
Write-Host $addLibCmd -ForegroundColor DarkGray
Write-Host "`nResponse:" -ForegroundColor Gray
$addLibrary = curl -X POST "$baseUrl/users/library" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "{`"manga_id`":`"$firstMangaId`",`"current_chapter`":5,`"status`":`"reading`",`"is_favorite`":true}" 2>$null | ConvertFrom-Json
Write-Host ($addLibrary | ConvertTo-Json -Depth 5) -ForegroundColor Green
Write-Host "`n"

# Test 6: Get library (protected)
Write-Host "6. GET /users/library - Get user's library (requires JWT)" -ForegroundColor Yellow
Write-Host "Command:" -ForegroundColor Gray
$getLibCmd = "curl `"$baseUrl/users/library`" -H `"Authorization: Bearer <token>`""
Write-Host $getLibCmd -ForegroundColor DarkGray
Write-Host "`nResponse:" -ForegroundColor Gray
$library = curl "$baseUrl/users/library" -H "Authorization: Bearer $token" 2>$null | ConvertFrom-Json
Write-Host ($library | ConvertTo-Json -Depth 5) -ForegroundColor Green
Write-Host "`n"

# Test 7: Update progress (protected)
Write-Host "7. PUT /users/progress - Update reading progress (requires JWT)" -ForegroundColor Yellow
Write-Host "Command:" -ForegroundColor Gray
$updateCmd = "curl -X PUT `"$baseUrl/users/progress`" -H `"Authorization: Bearer <token>`" -H `"Content-Type: application/json`" -d `"{\\`"manga_id\\`":\\`"$firstMangaId\\`",\\`"current_chapter\\`":15,\\`"status\\`":\\`"reading\\`",\\`"rating\\`":8}`""
Write-Host $updateCmd -ForegroundColor DarkGray
Write-Host "`nResponse:" -ForegroundColor Gray
$updateProgress = curl -X PUT "$baseUrl/users/progress" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "{`"manga_id`":`"$firstMangaId`",`"current_chapter`":15,`"status`":`"reading`",`"rating`":8}" 2>$null | ConvertFrom-Json
Write-Host ($updateProgress | ConvertTo-Json -Depth 5) -ForegroundColor Green
Write-Host "`n"

# Test 8: Unauthorized access
Write-Host "8. GET /users/library - Test unauthorized access (no token)" -ForegroundColor Yellow
Write-Host "Command:" -ForegroundColor Gray
$unauthCmd = "curl `"$baseUrl/users/library`""
Write-Host $unauthCmd -ForegroundColor DarkGray
Write-Host "`nResponse (should be 401):" -ForegroundColor Gray
$unauth = curl "$baseUrl/users/library" 2>$null | ConvertFrom-Json
Write-Host ($unauth | ConvertTo-Json -Depth 5) -ForegroundColor Red
Write-Host "`n"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  All Manual Tests Complete!" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

Write-Host "Summary of Endpoints Tested:" -ForegroundColor Yellow
Write-Host "  ✓ POST   /auth/register" -ForegroundColor Green
Write-Host "  ✓ POST   /auth/login" -ForegroundColor Green
Write-Host "  ✓ GET    /manga" -ForegroundColor Green
Write-Host "  ✓ GET    /manga/:id" -ForegroundColor Green
Write-Host "  ✓ POST   /users/library (protected)" -ForegroundColor Green
Write-Host "  ✓ GET    /users/library (protected)" -ForegroundColor Green
Write-Host "  ✓ PUT    /users/progress (protected)" -ForegroundColor Green
Write-Host "  ✓ Unauthorized access test" -ForegroundColor Green
Write-Host ""
