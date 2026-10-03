// Package importer - Data Import Pipeline for External APIs to SQLite
// Converts ExternalMangaData to Manga model and imports into database
// Features:
//   - Convert MangaDex/Jikan data to local Manga model
//   - Upsert to avoid duplicates (update if exists)
//   - Track external IDs for cross-referencing
//   - Batch import support
//   - Preview before import
package importer

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"mangahub/pkg/cache"
	"mangahub/pkg/models"

	"github.com/google/uuid"
)

// Importer handles data import from external APIs to SQLite
type Importer struct {
	db          *sql.DB
	cache       *cache.RedisCache
	useCache    bool
	dryRun      bool
	importStats ImportStats
}

// ImportStats tracks import statistics
type ImportStats struct {
	Total       int `json:"total"`
	Inserted    int `json:"inserted"`
	Updated     int `json:"updated"`
	Skipped     int `json:"skipped"`
	Failed      int `json:"failed"`
	CacheHits   int `json:"cache_hits"`
	CacheMisses int `json:"cache_misses"`
}

// NewImporter creates a new importer instance
func NewImporter(db *sql.DB, cacheClient *cache.RedisCache) *Importer {
	return &Importer{
		db:       db,
		cache:    cacheClient,
		useCache: cacheClient != nil,
		dryRun:   false,
	}
}

// SetDryRun enables/disables dry run mode (preview only)
func (i *Importer) SetDryRun(dryRun bool) {
	i.dryRun = dryRun
}

// GetStats returns import statistics
func (i *Importer) GetStats() ImportStats {
	return i.importStats
}

// ResetStats resets import statistics
func (i *Importer) ResetStats() {
	i.importStats = ImportStats{}
}

// ConvertToManga converts ExternalMangaData to Manga model
// Handles field mapping and nullable fields
// Note: Genres are stored separately in manga_genres table (normalized)
// Note: Ratings are stored separately in manga_ratings table, average_rating auto-calculated by triggers
func ConvertToManga(ext models.ExternalMangaData) models.Manga {
	now := time.Now()

	// Get first author or empty
	author := ""
	if len(ext.Authors) > 0 {
		author = ext.Authors[0]
	}

	// Determine manga type from source or default
	mangaType := "manga"
	if ext.Source == models.SourceJikan {
		mangaType = "manga" // Jikan is MAL, typically Japanese manga
	}

	// Normalize status
	status := normalizeStatus(ext.Status)

	return models.Manga{
		ID:            uuid.New().String(),
		Title:         ext.Title,
		Author:        author,
		Artist:        "", // External APIs often don't distinguish author/artist
		Description:   truncateDescription(ext.Description, 2000),
		CoverURL:      ext.CoverURL,
		Status:        status,
		Type:          mangaType,
		Genres:        []models.Genre{}, // Populated separately via manga_genres table
		TotalChapters: ext.ChapterCount,
		AverageRating: 0, // Auto-calculated via triggers
		RatingCount:   0, // Auto-calculated via triggers
		Year:          ext.Year,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// normalizeStatus converts various status formats to our standard
func normalizeStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "ongoing", "publishing", "releasing", "current":
		return "ongoing"
	case "completed", "finished", "complete":
		return "completed"
	case "hiatus", "on hiatus":
		return "hiatus"
	case "cancelled", "canceled", "discontinued":
		return "cancelled"
	default:
		// Anything else ("upcoming", "not yet published", "") must still satisfy
		// the manga.status CHECK constraint, or the insert fails
		return "ongoing"
	}
}

// truncateDescription limits description length
func truncateDescription(desc string, maxLen int) string {
	if len(desc) <= maxLen {
		return desc
	}
	return desc[:maxLen-3] + "..."
}

// ImportOne imports a single manga entry
func (i *Importer) ImportOne(ctx context.Context, ext models.ExternalMangaData) (*models.Manga, error) {
	i.importStats.Total++

	// Convert to Manga model
	manga := ConvertToManga(ext)

	if i.dryRun {
		i.importStats.Skipped++
		return &manga, nil
	}

	// Check if manga with same title already exists
	existingID, err := i.findExistingManga(ctx, manga.Title)
	if err != nil && err != sql.ErrNoRows {
		i.importStats.Failed++
		return nil, fmt.Errorf("failed to check existing manga: %w", err)
	}

	if existingID != "" {
		// Update existing manga
		manga.ID = existingID
		if err := i.updateManga(ctx, manga); err != nil {
			i.importStats.Failed++
			return nil, fmt.Errorf("failed to update manga: %w", err)
		}
		i.importStats.Updated++
	} else {
		// Insert new manga
		if err := i.insertManga(ctx, manga); err != nil {
			i.importStats.Failed++
			return nil, fmt.Errorf("failed to insert manga: %w", err)
		}
		i.importStats.Inserted++
	}

	// Store external ID mapping
	if err := i.saveExternalMapping(ctx, manga.ID, ext); err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: failed to save external mapping: %v\n", err)
	}

	// Link genres so imported manga show up in genre browsing/filtering
	if err := i.saveGenres(ctx, manga.ID, ext.Genres); err != nil {
		fmt.Printf("Warning: failed to save genres for '%s': %v\n", manga.Title, err)
	}

	return &manga, nil
}

// ImportBatch imports multiple manga entries
func (i *Importer) ImportBatch(ctx context.Context, items []models.ExternalMangaData) ([]models.Manga, error) {
	results := make([]models.Manga, 0, len(items))

	for _, ext := range items {
		manga, err := i.ImportOne(ctx, ext)
		if err != nil {
			// Log error but continue with other items
			fmt.Printf("Import error for '%s': %v\n", ext.Title, err)
			continue
		}
		if manga != nil {
			results = append(results, *manga)
		}
	}

	return results, nil
}

// findExistingManga checks if a manga with the same title exists
func (i *Importer) findExistingManga(ctx context.Context, title string) (string, error) {
	var id string
	err := i.db.QueryRowContext(ctx,
		"SELECT id FROM manga WHERE LOWER(title) = LOWER(?) LIMIT 1",
		title,
	).Scan(&id)
	return id, err
}

// insertManga inserts a new manga into the database
// Note: Genres must be inserted separately via manga_genres junction table
// Note: Ratings must be inserted separately via manga_ratings table
func (i *Importer) insertManga(ctx context.Context, m models.Manga) error {
	_, err := i.db.ExecContext(ctx, `
		INSERT INTO manga (id, title, author, artist, description, cover_url, status, type, total_chapters, year, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Title, m.Author, m.Artist, m.Description, m.CoverURL, m.Status, m.Type, m.TotalChapters, m.Year, m.CreatedAt, m.UpdatedAt,
	)
	return err
}

// updateManga updates an existing manga in the database
// Note: Genres should be updated separately via manga_genres junction table
// Note: Ratings should be updated separately via manga_ratings table
func (i *Importer) updateManga(ctx context.Context, m models.Manga) error {
	_, err := i.db.ExecContext(ctx, `
		UPDATE manga SET 
			author = COALESCE(NULLIF(?, ''), author),
			description = COALESCE(NULLIF(?, ''), description),
			cover_url = COALESCE(NULLIF(?, ''), cover_url),
			status = ?,
			total_chapters = CASE WHEN ? > total_chapters THEN ? ELSE total_chapters END,
			year = COALESCE(NULLIF(?, 0), year),
			updated_at = ?
		WHERE id = ?`,
		m.Author, m.Description, m.CoverURL, m.Status,
		m.TotalChapters, m.TotalChapters,
		m.Year, m.UpdatedAt, m.ID,
	)
	return err
}

// saveExternalMapping saves the external ID mapping for cross-referencing.
// manga_external_ids has one row per manga (manga_id is the primary key), with
// a column per source, so a manga imported from several sources keeps every ID.
func (i *Importer) saveExternalMapping(ctx context.Context, mangaID string, ext models.ExternalMangaData) error {
	var mangadexID interface{}
	var malID, anilistID int
	switch ext.Source {
	case models.SourceMangaDex:
		mangadexID = sqlNullString(true, ext.ExternalID)
	case models.SourceJikan:
		fmt.Sscanf(ext.ExternalID, "%d", &malID)
	case models.SourceAniList:
		fmt.Sscanf(ext.ExternalID, "%d", &anilistID)
	}

	now := time.Now()
	_, err := i.db.ExecContext(ctx, `
		INSERT INTO manga_external_ids
			(manga_id, mangadex_id, mal_id, anilist_id, primary_source, last_synced_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(manga_id) DO UPDATE SET
			mangadex_id    = COALESCE(excluded.mangadex_id, mangadex_id),
			mal_id         = COALESCE(excluded.mal_id, mal_id),
			anilist_id     = COALESCE(excluded.anilist_id, anilist_id),
			last_synced_at = excluded.last_synced_at,
			updated_at     = excluded.updated_at`,
		mangaID, mangadexID, sqlNullInt(malID), sqlNullInt(anilistID), ext.Source, now, now, now,
	)
	return err
}

// saveGenres links the manga to its genres, creating genres that don't exist yet.
func (i *Importer) saveGenres(ctx context.Context, mangaID string, names []string) error {
	now := time.Now()
	for _, name := range names {
		name = strings.TrimSpace(name)
		slug := slugify(name)
		if slug == "" {
			continue
		}

		// genres.name and genres.slug are both UNIQUE; reuse whichever matches
		if _, err := i.db.ExecContext(ctx,
			"INSERT OR IGNORE INTO genres (id, name, slug, created_at) VALUES (?, ?, ?, ?)",
			uuid.New().String(), name, slug, now,
		); err != nil {
			return fmt.Errorf("insert genre %q: %w", name, err)
		}
		var genreID string
		if err := i.db.QueryRowContext(ctx,
			"SELECT id FROM genres WHERE slug = ? OR LOWER(name) = LOWER(?) LIMIT 1", slug, name,
		).Scan(&genreID); err != nil {
			return fmt.Errorf("find genre %q: %w", name, err)
		}

		if _, err := i.db.ExecContext(ctx,
			"INSERT OR IGNORE INTO manga_genres (id, manga_id, genre_id, created_at) VALUES (?, ?, ?, ?)",
			uuid.New().String(), mangaID, genreID, now,
		); err != nil {
			return fmt.Errorf("link genre %q: %w", name, err)
		}
	}
	return nil
}

// slugify turns a genre name into the slug format used by the seed data
// ("Slice of Life" -> "slice-of-life", "Sci-Fi" -> "sci-fi").
func slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// Helper functions for SQL null handling
func sqlNullString(condition bool, value string) interface{} {
	if condition && value != "" {
		return value
	}
	return nil
}

func sqlNullInt(value int) interface{} {
	if value > 0 {
		return value
	}
	return nil
}

// PreviewImport shows what would be imported without actually importing
func (i *Importer) PreviewImport(items []models.ExternalMangaData) []MangaPreview {
	previews := make([]MangaPreview, 0, len(items))
	for _, ext := range items {
		author := ""
		if len(ext.Authors) > 0 {
			author = ext.Authors[0]
		}
		previews = append(previews, MangaPreview{
			Title:      ext.Title,
			Author:     author,
			Status:     normalizeStatus(ext.Status),
			Rating:     ext.Rating,
			Year:       ext.Year,
			Genres:     ext.Genres,
			Chapters:   ext.ChapterCount,
			Source:     ext.Source,
			ExternalID: ext.ExternalID,
			HasCover:   ext.CoverURL != "",
			DescLength: len(ext.Description),
		})
	}
	return previews
}

// MangaPreview represents a preview of manga data before import
type MangaPreview struct {
	Title      string   `json:"title"`
	Author     string   `json:"author"`
	Status     string   `json:"status"`
	Rating     float64  `json:"rating"`
	Year       int      `json:"year"`
	Genres     []string `json:"genres"`
	Chapters   int      `json:"chapters"`
	Source     string   `json:"source"`
	ExternalID string   `json:"external_id"`
	HasCover   bool     `json:"has_cover"`
	DescLength int      `json:"desc_length"`
}
