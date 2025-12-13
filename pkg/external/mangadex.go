// Package external - MangaDex API Client
// Integration với MangaDex API để fetch manga data
// Chức năng:
//   - Search manga
//   - Get manga details
//   - Get chapter list
//   - Get chapter pages/images
//   - Rate limiting (5 req/s as per MangaDex API limits)
//
// API Docs: https://api.mangadex.org/docs/
package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"mangahub/pkg/config"
	"mangahub/pkg/models"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewRateLimiter creates a rate limiter with specified rate (requests per second)
func NewRateLimiter(ratePerSecond int) *RateLimiter {
	return &RateLimiter{
		tokens:     float64(ratePerSecond),
		maxTokens:  float64(ratePerSecond),
		refillRate: float64(ratePerSecond),
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available or context is cancelled
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		r.mu.Lock()
		// Refill tokens based on elapsed time
		now := time.Now()
		elapsed := now.Sub(r.lastRefill).Seconds()
		r.tokens += elapsed * r.refillRate
		if r.tokens > r.maxTokens {
			r.tokens = r.maxTokens
		}
		r.lastRefill = now

		if r.tokens >= 1 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}

		// Calculate wait time for next token
		waitTime := time.Duration((1-r.tokens)/r.refillRate*1000) * time.Millisecond
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue loop to try again
		}
	}
}

// MangaDexClient provides methods to interact with MangaDex API
type MangaDexClient struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *RateLimiter
}

// NewMangaDexClient creates a new MangaDex API client
func NewMangaDexClient(cfg *config.MangaDexConfig) *MangaDexClient {
	return &MangaDexClient{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		rateLimiter: NewRateLimiter(cfg.RateLimit),
	}
}

// MangaDexSearchResponse represents the search API response
type MangaDexSearchResponse struct {
	Result   string          `json:"result"`
	Response string          `json:"response"`
	Data     []MangaDexManga `json:"data"`
	Limit    int             `json:"limit"`
	Offset   int             `json:"offset"`
	Total    int             `json:"total"`
}

// MangaDexMangaResponse represents a single manga response
type MangaDexMangaResponse struct {
	Result   string        `json:"result"`
	Response string        `json:"response"`
	Data     MangaDexManga `json:"data"`
}

// MangaDexManga represents a manga from MangaDex
type MangaDexManga struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Attributes    MangaDexAttributes     `json:"attributes"`
	Relationships []MangaDexRelationship `json:"relationships"`
}

// MangaDexAttributes contains manga attributes
type MangaDexAttributes struct {
	Title                  map[string]string   `json:"title"`
	AltTitles              []map[string]string `json:"altTitles"`
	Description            map[string]string   `json:"description"`
	IsLocked               bool                `json:"isLocked"`
	Links                  map[string]string   `json:"links"`
	OriginalLanguage       string              `json:"originalLanguage"`
	LastVolume             string              `json:"lastVolume"`
	LastChapter            string              `json:"lastChapter"`
	PublicationDemographic string              `json:"publicationDemographic"`
	Status                 string              `json:"status"`
	Year                   int                 `json:"year"`
	ContentRating          string              `json:"contentRating"`
	Tags                   []MangaDexTag       `json:"tags"`
	State                  string              `json:"state"`
	CreatedAt              string              `json:"createdAt"`
	UpdatedAt              string              `json:"updatedAt"`
}

// MangaDexTag represents a tag
type MangaDexTag struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name map[string]string `json:"name"`
	} `json:"attributes"`
}

// MangaDexRelationship represents a relationship (author, artist, cover)
type MangaDexRelationship struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// MangaDexChapterResponse represents chapter list response
type MangaDexChapterResponse struct {
	Result   string            `json:"result"`
	Response string            `json:"response"`
	Data     []MangaDexChapter `json:"data"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
	Total    int               `json:"total"`
}

// MangaDexChapter represents a chapter
type MangaDexChapter struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Volume             string `json:"volume"`
		Chapter            string `json:"chapter"`
		Title              string `json:"title"`
		TranslatedLanguage string `json:"translatedLanguage"`
		ExternalURL        string `json:"externalUrl"`
		PublishAt          string `json:"publishAt"`
		Pages              int    `json:"pages"`
	} `json:"attributes"`
}

// SearchManga searches for manga on MangaDex
func (c *MangaDexClient) SearchManga(ctx context.Context, query string, limit, offset int) (*MangaDexSearchResponse, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter cancelled: %w", err)
	}

	params := url.Values{}
	params.Set("title", query)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("offset", fmt.Sprintf("%d", offset))
	params.Set("includes[]", "cover_art")
	params.Set("includes[]", "author")
	params.Set("order[relevance]", "desc")

	reqURL := fmt.Sprintf("%s/manga?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result MangaDexSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetManga retrieves manga details by ID
func (c *MangaDexClient) GetManga(ctx context.Context, mangaID string) (*MangaDexManga, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter cancelled: %w", err)
	}

	params := url.Values{}
	params.Set("includes[]", "cover_art")
	params.Set("includes[]", "author")
	params.Set("includes[]", "artist")

	reqURL := fmt.Sprintf("%s/manga/%s?%s", c.baseURL, mangaID, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("manga not found: %s", mangaID)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result MangaDexMangaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Data, nil
}

// GetChapterList retrieves chapters for a manga
func (c *MangaDexClient) GetChapterList(ctx context.Context, mangaID string, limit, offset int, lang string) (*MangaDexChapterResponse, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter cancelled: %w", err)
	}

	params := url.Values{}
	params.Set("manga", mangaID)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("offset", fmt.Sprintf("%d", offset))
	params.Set("order[chapter]", "asc")
	if lang != "" {
		params.Set("translatedLanguage[]", lang)
	}

	reqURL := fmt.Sprintf("%s/chapter?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result MangaDexChapterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ToExternalMangaData converts MangaDex response to internal model
func (m *MangaDexManga) ToExternalMangaData() models.ExternalMangaData {
	// Get English title, fallback to first available
	title := ""
	if en, ok := m.Attributes.Title["en"]; ok {
		title = en
	} else {
		for _, t := range m.Attributes.Title {
			title = t
			break
		}
	}

	// Get English description
	description := ""
	if en, ok := m.Attributes.Description["en"]; ok {
		description = en
	}

	// Extract genres from tags
	var genres []string
	for _, tag := range m.Attributes.Tags {
		if name, ok := tag.Attributes.Name["en"]; ok {
			genres = append(genres, name)
		}
	}

	// Find cover art
	coverURL := ""
	for _, rel := range m.Relationships {
		if rel.Type == "cover_art" {
			if fileName, ok := rel.Attributes["fileName"].(string); ok {
				coverURL = fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s", m.ID, fileName)
			}
		}
	}

	// Extract authors
	var authors []string
	for _, rel := range m.Relationships {
		if rel.Type == "author" || rel.Type == "artist" {
			if name, ok := rel.Attributes["name"].(string); ok {
				authors = append(authors, name)
			}
		}
	}

	return models.ExternalMangaData{
		Source:      "mangadex",
		ExternalID:  m.ID,
		Title:       title,
		Description: description,
		CoverURL:    coverURL,
		Status:      m.Attributes.Status,
		Genres:      genres,
		Year:        m.Attributes.Year,
		Authors:     authors,
		FetchedAt:   time.Now(),
	}
}

// GetCoverURL builds the cover image URL
func GetCoverURL(mangaID, coverFileName string, size string) string {
	// size: 256, 512, or empty for original
	if size != "" {
		return fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s.%s.jpg", mangaID, coverFileName, size)
	}
	return fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s", mangaID, coverFileName)
}
