// Package external - Jikan API Client (MyAnimeList Unofficial)
// Integration với Jikan API để lấy MAL data
// Chức năng:
//   - Search manga
//   - Get manga details
//   - Get recommendations
//   - Get reviews
//   - Rate limiting (3 req/s)
//
// API Docs: https://docs.api.jikan.moe/
package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"mangahub/pkg/config"
	"mangahub/pkg/models"
)

// JikanClient provides methods to interact with Jikan API
type JikanClient struct {
	baseURL    string
	httpClient *http.Client
	rateLimit  int
}

// NewJikanClient creates a new Jikan API client
func NewJikanClient(cfg *config.JikanConfig) *JikanClient {
	return &JikanClient{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		rateLimit: cfg.RateLimit,
	}
}

// JikanMangaResponse represents a single manga response
type JikanMangaResponse struct {
	Data JikanMangaData `json:"data"`
}

// JikanSearchResponse represents search results
type JikanSearchResponse struct {
	Data       []JikanMangaData `json:"data"`
	Pagination JikanPagination  `json:"pagination"`
}

type JikanPagination struct {
	LastVisiblePage int  `json:"last_visible_page"`
	HasNextPage     bool `json:"has_next_page"`
	CurrentPage     int  `json:"current_page"`
	Items           struct {
		Count   int `json:"count"`
		Total   int `json:"total"`
		PerPage int `json:"per_page"`
	} `json:"items"`
}

// JikanMangaData represents manga data from Jikan
type JikanMangaData struct {
	MalID         int            `json:"mal_id"`
	URL           string         `json:"url"`
	Title         string         `json:"title"`
	TitleEnglish  string         `json:"title_english"`
	TitleJapanese string         `json:"title_japanese"`
	Type          string         `json:"type"`
	Chapters      int            `json:"chapters"`
	Volumes       int            `json:"volumes"`
	Status        string         `json:"status"`
	Publishing    bool           `json:"publishing"`
	Score         float64        `json:"score"`
	ScoredBy      int            `json:"scored_by"`
	Rank          int            `json:"rank"`
	Popularity    int            `json:"popularity"`
	Members       int            `json:"members"`
	Favorites     int            `json:"favorites"`
	Synopsis      string         `json:"synopsis"`
	Background    string         `json:"background"`
	Authors       []JikanAuthor  `json:"authors"`
	Genres        []JikanGenre   `json:"genres"`
	Themes        []JikanGenre   `json:"themes"`
	Demographics  []JikanGenre   `json:"demographics"`
	Images        JikanImages    `json:"images"`
	Published     JikanPublished `json:"published"`
}

type JikanAuthor struct {
	MalID int    `json:"mal_id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	URL   string `json:"url"`
}

type JikanGenre struct {
	MalID int    `json:"mal_id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	URL   string `json:"url"`
}

type JikanImages struct {
	JPG struct {
		ImageURL      string `json:"image_url"`
		SmallImageURL string `json:"small_image_url"`
		LargeImageURL string `json:"large_image_url"`
	} `json:"jpg"`
	WebP struct {
		ImageURL      string `json:"image_url"`
		SmallImageURL string `json:"small_image_url"`
		LargeImageURL string `json:"large_image_url"`
	} `json:"webp"`
}

type JikanPublished struct {
	From   string `json:"from"`
	To     string `json:"to"`
	String string `json:"string"`
}

// SearchManga searches for manga on MAL via Jikan
func (c *JikanClient) SearchManga(ctx context.Context, query string, page, limit int) (*JikanSearchResponse, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("sfw", "true") // Safe for work filter

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

	var result JikanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// SearchMangaFiltered searches MAL via Jikan and returns normalized ExternalMangaData
// Only essential metadata fields are returned to align with our database model.
func (c *JikanClient) SearchMangaFiltered(ctx context.Context, query string, page, limit int) ([]models.ExternalMangaData, error) {
	res, err := c.SearchManga(ctx, query, page, limit)
	if err != nil {
		return nil, err
	}

	var items []models.ExternalMangaData
	for _, m := range res.Data {
		items = append(items, m.ToExternalMangaData())
	}
	return items, nil
}

// GetManga retrieves manga details by MAL ID
func (c *JikanClient) GetManga(ctx context.Context, malID int) (*JikanMangaData, error) {
	reqURL := fmt.Sprintf("%s/manga/%d/full", c.baseURL, malID)

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
		return nil, fmt.Errorf("manga not found: %d", malID)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result JikanMangaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Data, nil
}

// GetTopManga retrieves top manga list
func (c *JikanClient) GetTopManga(ctx context.Context, page, limit int, filter string) (*JikanSearchResponse, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("limit", fmt.Sprintf("%d", limit))
	if filter != "" {
		params.Set("filter", filter) // publishing, upcoming, bypopularity, favorite
	}

	reqURL := fmt.Sprintf("%s/top/manga?%s", c.baseURL, params.Encode())

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

	var result JikanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetRecommendations retrieves manga recommendations based on MAL ID
func (c *JikanClient) GetRecommendations(ctx context.Context, malID int) ([]JikanRecommendation, error) {
	reqURL := fmt.Sprintf("%s/manga/%d/recommendations", c.baseURL, malID)

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

	var result struct {
		Data []JikanRecommendation `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

// JikanRecommendation represents a manga recommendation
type JikanRecommendation struct {
	Entry struct {
		MalID  int         `json:"mal_id"`
		URL    string      `json:"url"`
		Title  string      `json:"title"`
		Images JikanImages `json:"images"`
	} `json:"entry"`
	Votes int `json:"votes"`
}

// ToExternalMangaData converts Jikan response to our internal model
func (m *JikanMangaData) ToExternalMangaData() models.ExternalMangaData {
	// Extract genre names
	var genres []string
	for _, g := range m.Genres {
		genres = append(genres, g.Name)
	}
	for _, t := range m.Themes {
		genres = append(genres, t.Name)
	}

	// Extract author names
	var authors []string
	for _, a := range m.Authors {
		authors = append(authors, a.Name)
	}

	// Determine year from published date
	year := 0
	if len(m.Published.From) >= 4 {
		fmt.Sscanf(m.Published.From, "%d", &year)
	}

	return models.ExternalMangaData{
		Source:       "jikan",
		ExternalID:   fmt.Sprintf("%d", m.MalID),
		Title:        m.Title,
		Description:  m.Synopsis,
		CoverURL:     m.Images.JPG.LargeImageURL,
		Status:       m.Status,
		Genres:       genres,
		Rating:       m.Score,
		Popularity:   m.Popularity,
		ChapterCount: m.Chapters,
		Year:         year,
		Authors:      authors,
		FetchedAt:    time.Now(),
	}
}
