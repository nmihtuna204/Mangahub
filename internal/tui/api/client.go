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

// doRequest performs an HTTP request. Idempotent methods (GET/PUT/DELETE) are
// retried on network errors and 5xx responses; POST is sent once so a retry
// can never create a duplicate comment or rating. The caller must close the
// returned response body (parseResponse and send do).
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	attempts := 1
	if method == http.MethodGet || method == http.MethodPut || method == http.MethodDelete {
		attempts = DefaultRetries
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(RetryDelay * time.Duration(i)):
			}
		}

		var reqBody io.Reader
		if payload != nil {
			reqBody = bytes.NewReader(payload) // fresh reader per attempt
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if token := c.GetToken(); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= 500 && i < attempts-1 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("request failed after %d attempt(s): %w", attempts, lastErr)
}

// send performs a request whose response body is not needed and turns
// 4xx/5xx responses into errors.
func (c *Client) send(ctx context.Context, method, endpoint string, body interface{}) error {
	resp, err := c.doRequest(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return apiError(resp.StatusCode, data)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// readBody reads a response body, turning 4xx/5xx responses into errors.
func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, apiError(resp.StatusCode, data)
	}
	return data, nil
}

// apiError extracts the API's error message from a failed response.
func apiError(status int, body []byte) error {
	var errResp models.APIResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil {
		return fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}
	return fmt.Errorf("HTTP %d: %s", status, string(body))
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
		return nil, apiError(resp.StatusCode, body)
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

	// Registration returns the new profile but no token; log in to get one
	if _, err := readBody(resp); err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}
	return c.Login(ctx, username, password)
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
	err := c.send(ctx, "POST", "/auth/logout", nil)
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
	setPage(params, page, pageSize)

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

// setPage converts a 1-based page number into the API's limit/offset parameters
func setPage(params url.Values, page, pageSize int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	params.Set("limit", fmt.Sprintf("%d", pageSize))
	params.Set("offset", fmt.Sprintf("%d", (page-1)*pageSize))
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
	params.Set("genre", genre) // matches the genre's name or slug
	setPage(params, page, pageSize)

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
	err := c.send(ctx, "POST", "/users/library", map[string]interface{}{
		"manga_id":        mangaID,
		"status":          "plan_to_read",
		"current_chapter": 0,
	})
	c.cache.Delete("library") // Invalidate cache
	return err
}

// RemoveFromLibrary removes a manga from user's library
func (c *Client) RemoveFromLibrary(ctx context.Context, mangaID string) error {
	err := c.send(ctx, "DELETE", "/users/library/"+mangaID, nil)
	c.cache.Delete("library") // Invalidate cache
	return err
}

// UpdateProgress updates reading progress with chapter, status, and favorite flag
// A nil isFavorite leaves the favorite flag unchanged.
func (c *Client) UpdateProgress(ctx context.Context, mangaID string, chapter int, status string, isFavorite *bool) error {
	payload := map[string]interface{}{
		"manga_id":        mangaID,
		"current_chapter": chapter,
	}
	if status != "" {
		payload["status"] = status
	}
	if isFavorite != nil {
		payload["is_favorite"] = *isFavorite
	}

	err := c.send(ctx, "PUT", "/users/progress", payload)
	c.cache.Delete("library") // Invalidate cache
	return err
}

// =====================================
// RATINGS API
// =====================================

// RatingSummaryResponse from ratings API
type RatingSummaryResponse struct {
	Success bool                        `json:"success"`
	Data    models.MangaRatingsResponse `json:"data"`
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

	summary := &result.Data.Summary
	c.cache.Set(cacheKey, summary, CacheDuration)
	return summary, nil
}

// SubmitRating submits/updates a rating
func (c *Client) SubmitRating(ctx context.Context, mangaID string, rating int, review string) error {
	err := c.send(ctx, "POST", "/manga/"+mangaID+"/ratings", map[string]interface{}{
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
	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

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

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

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

	err := c.send(ctx, "POST", "/manga/"+mangaID+"/comments", payload)
	return err
}

// LikeComment likes a comment
func (c *Client) LikeComment(ctx context.Context, commentID string) error {
	err := c.send(ctx, "POST", "/comments/"+commentID+"/like", nil)
	return err
}

// UnlikeComment unlikes a comment
func (c *Client) UnlikeComment(ctx context.Context, commentID string) error {
	err := c.send(ctx, "DELETE", "/comments/"+commentID+"/like", nil)
	return err
}

// =====================================
// CHAT HISTORY API
// =====================================

// ChatHistoryMessage is a saved chat message from GET /rooms/:room_id/messages
type ChatHistoryMessage struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// GetRoomMessages returns the most recent saved messages of a chat room, oldest first
func (c *Client) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]ChatHistoryMessage, error) {
	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", limit))
	resp, err := c.doRequest(ctx, "GET", "/rooms/"+url.PathEscape(roomID)+"/messages?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	type historyResponse struct {
		Data struct {
			Messages []ChatHistoryMessage `json:"messages"`
		} `json:"data"`
	}
	result, err := parseResponse[historyResponse](resp)
	if err != nil {
		return nil, err
	}
	return result.Data.Messages, nil
}

// =====================================
// HEALTH CHECK
// =====================================

// HealthCheck verifies server connectivity
func (c *Client) HealthCheck(ctx context.Context) bool {
	return c.send(ctx, "GET", "/health", nil) == nil
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

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

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
	err := c.send(ctx, "PUT", "/users/progress", map[string]interface{}{
		"manga_id": mangaID,
		"status":   status,
	})
	c.cache.Delete("library") // Invalidate cache
	return err
}

// UpdateLibraryProgress updates both status and chapter progress
func (c *Client) UpdateLibraryProgress(ctx context.Context, mangaID string, status string, chapter int) error {
	err := c.send(ctx, "PUT", "/users/progress", map[string]interface{}{
		"manga_id":        mangaID,
		"status":          status,
		"current_chapter": chapter,
	})
	c.cache.Delete("library") // Invalidate cache
	return err
}

// ToggleFavorite toggles favorite status for a manga
func (c *Client) ToggleFavorite(ctx context.Context, mangaID string, isFavorite bool) error {
	err := c.send(ctx, "PUT", "/users/progress", map[string]interface{}{
		"manga_id":    mangaID,
		"is_favorite": isFavorite,
	})
	c.cache.Delete("library") // Invalidate cache
	return err
}
