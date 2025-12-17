// Package models - External IDs for Multi-Source Integration
// Mapping IDs từ các nguồn external APIs (MangaDex, AniList, MAL)
// Chức năng:
//   - Lưu trữ external IDs để cross-reference
//   - Hỗ trợ multi-source aggregation
//   - Track nguồn gốc data
package models

import (
	"time"
)

// MangaExternalIDs stores external platform IDs for a manga
// Enables cross-referencing between MangaDex, AniList, MAL, etc.
type MangaExternalIDs struct {
	ID            string    `json:"id" db:"id"`
	MangaID       string    `json:"manga_id" db:"manga_id"`
	MangaDexID    string    `json:"mangadex_id,omitempty" db:"mangadex_id"`
	AniListID     int       `json:"anilist_id,omitempty" db:"anilist_id"`
	MyAnimeListID int       `json:"mal_id,omitempty" db:"mal_id"`
	KitsuID       string    `json:"kitsu_id,omitempty" db:"kitsu_id"`
	PrimarySource string    `json:"primary_source" db:"primary_source"` // mangadex, anilist, mal
	LastSyncedAt  time.Time `json:"last_synced_at" db:"last_synced_at"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// ExternalChapterMapping maps internal chapter to external chapter IDs
type ExternalChapterMapping struct {
	ID                string `json:"id" db:"id"`
	MangaID           string `json:"manga_id" db:"manga_id"`
	ChapterNumber     int    `json:"chapter_number" db:"chapter_number"`
	MangaDexChapterID string `json:"mangadex_chapter_id,omitempty" db:"mangadex_chapter_id"`
	ExternalURL       string `json:"external_url,omitempty" db:"external_url"`
}

// ExternalMangaData represents aggregated data from external sources
// Used for caching API responses
type ExternalMangaData struct {
	Source       string                 `json:"source"` // mangadex, jikan, anilist
	ExternalID   string                 `json:"external_id"`
	Title        string                 `json:"title"`
	AltTitles    []string               `json:"alt_titles,omitempty"`
	Description  string                 `json:"description"`
	CoverURL     string                 `json:"cover_url"`
	Status       string                 `json:"status"`
	Genres       []string               `json:"genres"`
	Rating       float64                `json:"rating"`
	Popularity   int                    `json:"popularity"`
	ChapterCount int                    `json:"chapter_count"`
	LastChapter  int                    `json:"last_chapter"`
	Year         int                    `json:"year"`
	Authors      []string               `json:"authors"`
	RawData      map[string]interface{} `json:"raw_data,omitempty"` // Original API response
	FetchedAt    time.Time              `json:"fetched_at"`
}

// External source constants
const (
	SourceMangaDex = "mangadex"
	SourceJikan    = "jikan"
	SourceAniList  = "anilist"
	SourceKitsu    = "kitsu"
)
