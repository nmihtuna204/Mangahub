// Package api - MangaHub TUI API Client
// Shared HTTP client layer cho TUI
// Chức năng:
//   - Singleton HTTP client với timeout
//   - Automatic JWT token injection
//   - Typed responses using pkg/models
//   - Retry logic for transient failures
//   - In-memory cache layer
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"mangahub/pkg/models"

	"github.com/spf13/viper"
)

// =====================================
// CLIENT CONFIGURATION
// =====================================

const (
	DefaultTimeout    = 30 * time.Second
	DefaultRetries    = 3
	RetryDelay        = 500 * time.Millisecond
	CacheDuration     = 5 * time.Minute
	DashboardCacheTTL = 30 * time.Second
	TrendingCacheTTL  = 10 * time.Minute
	LibraryCacheTTL   = 1 * time.Minute
)

// =====================================
// CLIENT STRUCT
// =====================================

// Client is the shared HTTP client for TUI
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	cache      *Cache
	mu         sync.RWMutex
}

// singleton instance
var (
	instance *Client
	once     sync.Once
)

// GetClient returns the singleton API client
// Trả về singleton instance của API client
func GetClient() *Client {
	once.Do(func() {
		instance = NewClient()
	})
	return instance
}

// InitClient initializes the API client with a custom base URL
// Called from cmd/tui/main.go
func InitClient(baseURL string) {
	once.Do(func() {
		instance = &Client{
			httpClient: &http.Client{
				Timeout: DefaultTimeout,
			},
			baseURL: baseURL,
			token:   viper.GetString("user.token"),
			cache:   NewCache(),
		}
	})
}

// NewClient creates a new API client
func NewClient() *Client {
	host := viper.GetString("server.host")
	if host == "" {
		host = "localhost"
	}
	port := viper.GetInt("server.http_port")
	if port == 0 {
		port = 8080
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		token:   viper.GetString("user.token"),
		cache:   NewCache(),
	}
}

// SetToken updates the authentication token
func (c *Client) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
	viper.Set("user.token", token)
}

// GetToken returns the current authentication token
func (c *Client) GetToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

// GetBaseURL returns the base API URL
func (c *Client) GetBaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL
}

// IsAuthenticated checks if user is logged in
func (c *Client) IsAuthenticated() bool {
	return c.GetToken() != ""
}

// ClearToken removes the authentication token (logout)
func (c *Client) ClearToken() {
	c.SetToken("")
}

// =====================================
// HTTP REQUEST METHODS
// =====================================

// doRequest performs an HTTP request with retry logic
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	fullURL := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add auth token if available
	token := c.GetToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Retry logic
	var resp *http.Response
	var lastErr error
	for i := 0; i < DefaultRetries; i++ {
		resp, lastErr = c.httpClient.Do(req)
		if lastErr == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		if i < DefaultRetries-1 {
			time.Sleep(RetryDelay * time.Duration(i+1))
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", DefaultRetries, lastErr)
	}
	return resp, nil
}

// parseResponse parses JSON response into target struct
func parseResponse[T any](resp *http.Response) (*T, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for API error response
	if resp.StatusCode >= 400 {
		var errResp models.APIResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// =====================================
// AUTH API
// =====================================

// LoginRequest for authentication
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse from auth API
type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Token string       `json:"token"`
		User  *models.User `json:"user"`
	} `json:"data"`
}

// Login authenticates user and stores token
func (c *Client) Login(ctx context.Context, username, password string) (*models.User, error) {
	resp, err := c.doRequest(ctx, "POST", "/auth/login", LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		return nil, err
	}

	result, err := parseResponse[LoginResponse](resp)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("login failed: %s", result.Message)
	}

	c.SetToken(result.Data.Token)
	return result.Data.User, nil
}

// RegisterRequest for user registration
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register creates a new user account
func (c *Client) Register(ctx context.Context, username, email, password string) (*models.User, error) {
	resp, err := c.doRequest(ctx, "POST", "/auth/register", RegisterRequest{
		Username: username,
		Email:    email,
		Password: password,
	})
	if err != nil {
		return nil, err
	}

	result, err := parseResponse[LoginResponse](resp)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("registration failed: %s", result.Message)
	}

	c.SetToken(result.Data.Token)
	return result.Data.User, nil
}

// GetCurrentUser retrieves the logged-in user's profile
func (c *Client) GetCurrentUser(ctx context.Context) (*models.User, error) {
	resp, err := c.doRequest(ctx, "GET", "/auth/me", nil)
	if err != nil {
		return nil, err
	}

	type UserResponse struct {
		Success bool         `json:"success"`
		Data    *models.User `json:"data"`
	}

	result, err := parseResponse[UserResponse](resp)
	if err != nil {
		return nil, err
	}

	return result.Data, nil
}

// Logout clears the auth token
func (c *Client) Logout(ctx context.Context) error {
	_, err := c.doRequest(ctx, "POST", "/auth/logout", nil)
	c.ClearToken()
	return err
}

// =====================================
// MANGA API
// =====================================

// MangaListResponse from manga list API
type MangaListResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Data    []models.Manga `json:"data"`
		Total   int            `json:"total"`
		Limit   int            `json:"limit"`
		Offset  int            `json:"offset"`
		HasMore bool           `json:"has_more"`
	} `json:"data"`
}

// SearchManga searches for manga by query
func (c *Client) SearchManga(ctx context.Context, query string, page, pageSize int) ([]models.Manga, int, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("search:%s:%d:%d", query, page, pageSize)
	if cached, found := c.cache.Get(cacheKey); found {
		if result, ok := cached.(*MangaListResponse); ok {
			return result.Data.Data, result.Data.Total, nil
		}
	}

	params := url.Values{}
	if query != "" {
		params.Set("q", query)
	}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("page_size", fmt.Sprintf("%d", pageSize))

	endpoint := "/manga?" + params.Encode()
	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, 0, err
	}

	result, err := parseResponse[MangaListResponse](resp)
	if err != nil {
		return nil, 0, err
	}

	// Cache the result
	c.cache.Set(cacheKey, result, CacheDuration)

	return result.Data.Data, result.Data.Total, nil
}

// GetManga retrieves a single manga by ID
func (c *Client) GetManga(ctx context.Context, mangaID string) (*models.Manga, error) {
	cacheKey := "manga:" + mangaID
	if cached, found := c.cache.Get(cacheKey); found {
		if result, ok := cached.(*models.Manga); ok {
			return result, nil
		}
	}

	resp, err := c.doRequest(ctx, "GET", "/manga/"+mangaID, nil)
	if err != nil {
		return nil, err
	}

	type SingleMangaResponse struct {
		Success bool          `json:"success"`
		Data    *models.Manga `json:"data"`
	}

	result, err := parseResponse[SingleMangaResponse](resp)
	if err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, result.Data, CacheDuration)
	return result.Data, nil
}

// SearchMangaByGenre searches for manga by genre
func (c *Client) SearchMangaByGenre(ctx context.Context, genre string, page, pageSize int) ([]models.Manga, int, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("genre:%s:%d:%d", genre, page, pageSize)
	if cached, found := c.cache.Get(cacheKey); found {
		if result, ok := cached.(*MangaListResponse); ok {
			return result.Data.Data, result.Data.Total, nil
		}
	}

	params := url.Values{}
	params.Set("q", genre) // The API searches in genres JSON array
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("page_size", fmt.Sprintf("%d", pageSize))

	endpoint := "/manga?" + params.Encode()
	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, 0, err
	}

	result, err := parseResponse[MangaListResponse](resp)
	if err != nil {
		return nil, 0, err
	}

	// Cache the result
	c.cache.Set(cacheKey, result, CacheDuration)
	return result.Data.Data, result.Data.Total, nil
}

// =====================================
// LIBRARY API
// ===================================== ==

// LibraryEntry represents a manga in user's library
type LibraryEntry struct {
	MangaID        string       `json:"manga_id"`
	Manga          models.Manga `json:"manga"`
	Status         string       `json:"status"` // reading, plan_to_read, completed, on_hold, dropped
	CurrentChapter int          `json:"current_chapter"`
	IsFavorite     bool         `json:"is_favorite"`
	LastReadAt     time.Time    `json:"last_read_at"`
	AddedAt        time.Time    `json:"added_at"`
}

// LibraryResponse from library API
type LibraryResponse struct {
	Success bool           `json:"success"`
	Data    []LibraryEntry `json:"data"`
}

// GetLibrary retrieves user's manga library
func (c *Client) GetLibrary(ctx context.Context) ([]LibraryEntry, error) {
	cacheKey := "library"
	if cached, found := c.cache.Get(cacheKey); found {
		if result, ok := cached.([]LibraryEntry); ok {
			return result, nil
		}
	}

	resp, err := c.doRequest(ctx, "GET", "/users/library", nil)
	if err != nil {
		return nil, err
	}

	result, err := parseResponse[LibraryResponse](resp)
	if err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, result.Data, LibraryCacheTTL)
	return result.Data, nil
}

// AddToLibrary adds a manga to user's library
func (c *Client) AddToLibrary(ctx context.Context, mangaID string) error {
	_, err := c.doRequest(ctx, "POST", "/users/library", map[string]interface{}{
		"manga_id":        mangaID,
		"status":          "plan_to_read",
		"current_chapter": 0,
	})
	c.cache.Delete("library") // Invalidate cache
	return err
}

// RemoveFromLibrary removes a manga from user's library
func (c *Client) RemoveFromLibrary(ctx context.Context, mangaID string) error {
	_, err := c.doRequest(ctx, "DELETE", "/users/library/"+mangaID, nil)
	c.cache.Delete("library") // Invalidate cache
	return err
}

// UpdateProgress updates reading progress with chapter, status, and favorite flag
func (c *Client) UpdateProgress(ctx context.Context, mangaID string, chapter int, status string, isFavorite bool) error {
	payload := map[string]interface{}{
		"manga_id":        mangaID,
		"current_chapter": chapter,
	}
	if status != "" {
		payload["status"] = status
	}
	payload["is_favorite"] = isFavorite

	_, err := c.doRequest(ctx, "PUT", "/users/progress", payload)
	c.cache.Delete("library") // Invalidate cache
	return err
}

// =====================================
// RATINGS API
// =====================================

// RatingSummaryResponse from ratings API
type RatingSummaryResponse struct {
	Success bool                  `json:"success"`
	Data    *models.RatingSummary `json:"data"`
}

// GetRatings retrieves rating summary for a manga
func (c *Client) GetRatings(ctx context.Context, mangaID string) (*models.RatingSummary, error) {
	cacheKey := "ratings:" + mangaID
	if cached, found := c.cache.Get(cacheKey); found {
		if result, ok := cached.(*models.RatingSummary); ok {
			return result, nil
		}
	}

	resp, err := c.doRequest(ctx, "GET", "/manga/"+mangaID+"/ratings", nil)
	if err != nil {
		return nil, err
	}

	result, err := parseResponse[RatingSummaryResponse](resp)
	if err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, result.Data, CacheDuration)
	return result.Data, nil
}

// SubmitRating submits/updates a rating
func (c *Client) SubmitRating(ctx context.Context, mangaID string, rating int, review string) error {
	_, err := c.doRequest(ctx, "POST", "/manga/"+mangaID+"/ratings", map[string]interface{}{
		"rating":      rating, // 1-10 integer scale
		"review_text": review,
	})
	c.cache.Delete("ratings:" + mangaID)
	return err
}

// =====================================
// LEADERBOARDS API
// =====================================

// LeaderboardResponse from leaderboards API
type LeaderboardResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Type    string      `json:"type"`
		Entries interface{} `json:"entries"`
	} `json:"data"`
}

// TrendingEntry represents a trending manga
type TrendingEntry struct {
	Rank          int     `json:"rank"`
	MangaID       string  `json:"manga_id"`
	Title         string  `json:"title"`
	CoverURL      string  `json:"cover_url"`
	AverageRating float64 `json:"average_rating"`
	ActivityCount int     `json:"activity_count"`
}

// GetTrending retrieves trending manga
func (c *Client) GetTrending(ctx context.Context, limit int, days int) ([]TrendingEntry, error) {
	cacheKey := fmt.Sprintf("trending:%d:%d", limit, days)
	if cached, found := c.cache.Get(cacheKey); found {
		if result, ok := cached.([]TrendingEntry); ok {
			return result, nil
		}
	}

	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("days", fmt.Sprintf("%d", days))

	resp, err := c.doRequest(ctx, "GET", "/leaderboards/trending?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	// Parse as raw JSON first
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var rawResp struct {
		Success bool `json:"success"`
		Data    struct {
			Entries []TrendingEntry `json:"entries"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, rawResp.Data.Entries, TrendingCacheTTL)
	return rawResp.Data.Entries, nil
}

// GetTopRated retrieves top rated manga
func (c *Client) GetTopRated(ctx context.Context, limit int) ([]TrendingEntry, error) {
	cacheKey := fmt.Sprintf("toprated:%d", limit)
	if cached, found := c.cache.Get(cacheKey); found {
		if result, ok := cached.([]TrendingEntry); ok {
			return result, nil
		}
	}

	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", limit))

	resp, err := c.doRequest(ctx, "GET", "/leaderboards/manga?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var rawResp struct {
		Success bool `json:"success"`
		Data    struct {
			Entries []TrendingEntry `json:"entries"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, rawResp.Data.Entries, TrendingCacheTTL)
	return rawResp.Data.Entries, nil
}

// =====================================
// COMMENTS API
// =====================================

// GetComments retrieves comments for a manga
func (c *Client) GetComments(ctx context.Context, mangaID string, page, pageSize int) (*models.CommentListResponse, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("page_size", fmt.Sprintf("%d", pageSize))

	resp, err := c.doRequest(ctx, "GET", "/manga/"+mangaID+"/comments?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	type CommentsResponse struct {
		Success bool                        `json:"success"`
		Data    *models.CommentListResponse `json:"data"`
	}

	result, err := parseResponse[CommentsResponse](resp)
	if err != nil {
		return nil, err
	}

	return result.Data, nil
}

// PostComment posts a new comment on a manga
func (c *Client) PostComment(ctx context.Context, mangaID, content string, chapterNum *int, parentID *string) error {
	payload := map[string]interface{}{
		"manga_id": mangaID,
		"content":  content,
	}
	if chapterNum != nil {
		payload["chapter_number"] = *chapterNum
	}
	if parentID != nil {
		payload["parent_id"] = *parentID
	}

	_, err := c.doRequest(ctx, "POST", "/manga/"+mangaID+"/comments", payload)
	return err
}

// LikeComment likes a comment
func (c *Client) LikeComment(ctx context.Context, commentID string) error {
	_, err := c.doRequest(ctx, "POST", "/comments/"+commentID+"/like", nil)
	return err
}

// UnlikeComment unlikes a comment
func (c *Client) UnlikeComment(ctx context.Context, commentID string) error {
	_, err := c.doRequest(ctx, "DELETE", "/comments/"+commentID+"/like", nil)
	return err
}

// =====================================
// HEALTH CHECK
// =====================================

// HealthCheck verifies server connectivity
func (c *Client) HealthCheck(ctx context.Context) bool {
	resp, err := c.doRequest(ctx, "GET", "/health", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// =====================================
// ACTIVITY FEED API
// =====================================

// ActivityEntry represents an activity from the feed
type ActivityEntry struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	ActivityType string    `json:"activity_type"`
	MangaID      string    `json:"manga_id"`
	MangaTitle   string    `json:"manga_title"`
	Rating       *float64  `json:"rating,omitempty"`
	Chapter      *int      `json:"chapter_number,omitempty"`
	CommentText  string    `json:"comment_text,omitempty"` // String, not pointer (empty if no comment)
	CreatedAt    time.Time `json:"created_at"`
}

// GetActivities retrieves recent activity feed
func (c *Client) GetActivities(ctx context.Context, limit int) ([]ActivityEntry, error) {
	cacheKey := fmt.Sprintf("activities:%d", limit)
	if cached, found := c.cache.Get(cacheKey); found {
		if result, ok := cached.([]ActivityEntry); ok {
			return result, nil
		}
	}

	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", limit))

	resp, err := c.doRequest(ctx, "GET", "/activities?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// API returns {activities: [], total, limit, offset} NOT wrapped in data
	var rawResp struct {
		Activities []ActivityEntry `json:"activities"`
		Total      int             `json:"total"`
	}
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, rawResp.Activities, DashboardCacheTTL)
	return rawResp.Activities, nil
}

// =====================================
// LIBRARY STATUS UPDATES
// =====================================

// UpdateLibraryStatus updates the reading status of a manga in library
func (c *Client) UpdateLibraryStatus(ctx context.Context, mangaID string, status string) error {
	_, err := c.doRequest(ctx, "PUT", "/users/progress", map[string]interface{}{
		"manga_id": mangaID,
		"status":   status,
	})
	c.cache.Delete("library") // Invalidate cache
	return err
}

// UpdateLibraryProgress updates both status and chapter progress
func (c *Client) UpdateLibraryProgress(ctx context.Context, mangaID string, status string, chapter int) error {
	_, err := c.doRequest(ctx, "PUT", "/users/progress", map[string]interface{}{
		"manga_id":        mangaID,
		"status":          status,
		"current_chapter": chapter,
	})
	c.cache.Delete("library") // Invalidate cache
	return err
}

// ToggleFavorite toggles favorite status for a manga
func (c *Client) ToggleFavorite(ctx context.Context, mangaID string, isFavorite bool) error {
	_, err := c.doRequest(ctx, "PUT", "/users/progress", map[string]interface{}{
		"manga_id":    mangaID,
		"is_favorite": isFavorite,
	})
	c.cache.Delete("library") // Invalidate cache
	return err
}
