// Package statistics - Reading Statistics Repository
// Handles database operations for reading statistics and chapter history
package statistics

import (
	"database/sql"
	"fmt"
	"time"

	"mangahub/pkg/database"
	"mangahub/pkg/models"

	"github.com/google/uuid"
)

// Repository handles statistics database operations
type Repository struct {
	db *database.DB
}

// NewRepository creates a new statistics repository
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// RecordChapterRead records a chapter read event
func (r *Repository) RecordChapterRead(history *models.ChapterHistory) error {
	if history.ID == "" {
		history.ID = uuid.New().String()
	}
	if history.ReadAt.IsZero() {
		history.ReadAt = time.Now()
	}
	history.CreatedAt = time.Now()

	query := `
		INSERT INTO chapter_history (id, user_id, manga_id, chapter_number, pages_read, time_minutes, read_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(query,
		history.ID,
		history.UserID,
		history.MangaID,
		history.ChapterNumber,
		history.PagesRead,
		history.TimeMinutes,
		history.ReadAt,
		history.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to record chapter read: %w", err)
	}

	// Update daily stats
	return r.updateDailyStats(history.UserID, history.ReadAt, 1, history.PagesRead, history.TimeMinutes)
}

// updateDailyStats updates or creates daily statistics
func (r *Repository) updateDailyStats(userID string, date time.Time, chapters, pages, minutes int) error {
	dateStr := date.Format("2006-01-02")

	// Try to update existing record
	result, err := r.db.Exec(`
		UPDATE daily_stats 
		SET chapters_read = chapters_read + ?,
		    pages_read = pages_read + ?,
		    time_minutes = time_minutes + ?,
		    manga_count = (
		        SELECT COUNT(DISTINCT manga_id) 
		        FROM chapter_history 
		        WHERE user_id = ? AND date(read_at) = ?
		    ),
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND date = ?`,
		chapters, pages, minutes, userID, dateStr, userID, dateStr)

	if err != nil {
		return fmt.Errorf("failed to update daily stats: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Insert new record
		_, err = r.db.Exec(`
			INSERT INTO daily_stats (id, user_id, date, chapters_read, pages_read, time_minutes, manga_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			uuid.New().String(), userID, dateStr, chapters, pages, minutes)
		if err != nil {
			return fmt.Errorf("failed to insert daily stats: %w", err)
		}
	}

	return nil
}

// GetDailyStats retrieves daily stats for a user within a date range
func (r *Repository) GetDailyStats(userID string, startDate, endDate time.Time) ([]models.DailyStats, error) {
	query := `
		SELECT id, user_id, date, chapters_read, pages_read, time_minutes, manga_count, created_at, updated_at
		FROM daily_stats
		WHERE user_id = ? AND date >= ? AND date <= ?
		ORDER BY date DESC`

	rows, err := r.db.Query(query, userID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("failed to query daily stats: %w", err)
	}
	defer rows.Close()

	var stats []models.DailyStats
	for rows.Next() {
		var s models.DailyStats
		err := rows.Scan(&s.ID, &s.UserID, &s.Date, &s.ChaptersRead, &s.PagesRead, &s.TimeMinutes, &s.MangaCount, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan daily stats: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// GetChapterHistory retrieves chapter reading history
func (r *Repository) GetChapterHistory(userID string, limit, offset int) ([]models.ChapterHistory, error) {
	query := `
		SELECT id, user_id, manga_id, chapter_number, pages_read, time_minutes, read_at, created_at
		FROM chapter_history
		WHERE user_id = ?
		ORDER BY read_at DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query chapter history: %w", err)
	}
	defer rows.Close()

	var history []models.ChapterHistory
	for rows.Next() {
		var h models.ChapterHistory
		err := rows.Scan(&h.ID, &h.UserID, &h.MangaID, &h.ChapterNumber, &h.PagesRead, &h.TimeMinutes, &h.ReadAt, &h.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chapter history: %w", err)
		}
		history = append(history, h)
	}

	return history, nil
}

// GetTotalChaptersRead returns total chapters read by a user
func (r *Repository) GetTotalChaptersRead(userID string) (int, error) {
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM chapter_history WHERE user_id = ?`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get total chapters: %w", err)
	}
	return total, nil
}

// GetTotalPagesRead returns total pages read by a user
func (r *Repository) GetTotalPagesRead(userID string) (int, error) {
	var total sql.NullInt64
	err := r.db.QueryRow(`SELECT SUM(pages_read) FROM chapter_history WHERE user_id = ?`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get total pages: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return int(total.Int64), nil
}

// GetTotalTimeSpent returns total reading time in minutes
func (r *Repository) GetTotalTimeSpent(userID string) (int, error) {
	var total sql.NullInt64
	err := r.db.QueryRow(`SELECT SUM(time_minutes) FROM chapter_history WHERE user_id = ?`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get total time: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return int(total.Int64), nil
}

// GetCurrentStreak returns current reading streak in days
func (r *Repository) GetCurrentStreak(userID string) (int, error) {
	// Get all dates with reading activity, ordered by date descending
	rows, err := r.db.Query(`
		SELECT DISTINCT date(read_at) as read_date
		FROM chapter_history
		WHERE user_id = ?
		ORDER BY read_date DESC`, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to query streak: %w", err)
	}
	defer rows.Close()

	streak := 0
	expectedDate := time.Now().Truncate(24 * time.Hour)

	for rows.Next() {
		var dateStr string
		if err := rows.Scan(&dateStr); err != nil {
			return 0, fmt.Errorf("failed to scan date: %w", err)
		}

		readDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// Check if this date matches expected date or previous day
		if readDate.Equal(expectedDate) || readDate.Equal(expectedDate.AddDate(0, 0, -1)) {
			streak++
			expectedDate = readDate.AddDate(0, 0, -1)
		} else if !readDate.Equal(expectedDate) {
			// If we haven't started the streak yet (today has no reading), check if yesterday has
			if streak == 0 && readDate.Equal(expectedDate.AddDate(0, 0, -1)) {
				streak++
				expectedDate = readDate.AddDate(0, 0, -1)
			} else {
				break // Streak broken
			}
		}
	}

	return streak, nil
}

// GetLongestStreak returns the longest reading streak in days
func (r *Repository) GetLongestStreak(userID string) (int, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT date(read_at) as read_date
		FROM chapter_history
		WHERE user_id = ?
		ORDER BY read_date ASC`, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to query streak: %w", err)
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var dateStr string
		if err := rows.Scan(&dateStr); err != nil {
			continue
		}
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		dates = append(dates, date)
	}

	if len(dates) == 0 {
		return 0, nil
	}

	longestStreak := 1
	currentStreak := 1

	for i := 1; i < len(dates); i++ {
		diff := dates[i].Sub(dates[i-1]).Hours() / 24
		if diff == 1 {
			currentStreak++
			if currentStreak > longestStreak {
				longestStreak = currentStreak
			}
		} else {
			currentStreak = 1
		}
	}

	return longestStreak, nil
}

// GetGenreDistribution returns genre reading distribution
func (r *Repository) GetGenreDistribution(userID string) ([]models.GenreCount, error) {
	query := `
		SELECT m.genres, COUNT(*) as count
		FROM chapter_history ch
		JOIN manga m ON ch.manga_id = m.id
		WHERE ch.user_id = ?
		GROUP BY m.genres
		ORDER BY count DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query genre distribution: %w", err)
	}
	defer rows.Close()

	genreCounts := make(map[string]int)
	total := 0

	for rows.Next() {
		var genresJSON sql.NullString
		var count int
		if err := rows.Scan(&genresJSON, &count); err != nil {
			continue
		}

		// Parse genres from JSON array stored in DB
		if genresJSON.Valid && genresJSON.String != "" {
			// Simple parsing - genres are stored as comma-separated or JSON array
			genres := parseGenres(genresJSON.String)
			for _, genre := range genres {
				genreCounts[genre] += count
				total += count
			}
		}
	}

	// Convert to slice and calculate percentages
	var distribution []models.GenreCount
	for genre, count := range genreCounts {
		percentage := 0.0
		if total > 0 {
			percentage = float64(count) / float64(total) * 100
		}
		distribution = append(distribution, models.GenreCount{
			Genre:      genre,
			Count:      count,
			Percentage: percentage,
		})
	}

	return distribution, nil
}

// parseGenres parses genre string (JSON array or comma-separated)
func parseGenres(genresStr string) []string {
	// Simple comma-separated parsing
	var genres []string
	if len(genresStr) > 0 {
		// Handle JSON-like format ["genre1","genre2"]
		cleaned := genresStr
		cleaned = replaceAll(cleaned, "[", "")
		cleaned = replaceAll(cleaned, "]", "")
		cleaned = replaceAll(cleaned, "\"", "")

		for _, g := range splitString(cleaned, ",") {
			g = trimSpace(g)
			if g != "" {
				genres = append(genres, g)
			}
		}
	}
	return genres
}

// Helper functions for string manipulation
func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}

func splitString(s, sep string) []string {
	var result []string
	current := ""
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, current)
			current = ""
			i += len(sep) - 1
		} else {
			current += string(s[i])
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

// GetMonthlyStats returns reading stats aggregated by month
func (r *Repository) GetMonthlyStats(userID string, months int) ([]models.MonthlyStats, error) {
	query := `
		SELECT 
			strftime('%Y', read_at) as year,
			strftime('%m', read_at) as month,
			COUNT(*) as chapters_read,
			COUNT(DISTINCT manga_id) as manga_count,
			COUNT(DISTINCT date(read_at)) as reading_days
		FROM chapter_history
		WHERE user_id = ? AND read_at >= date('now', '-' || ? || ' months')
		GROUP BY year, month
		ORDER BY year DESC, month DESC`

	rows, err := r.db.Query(query, userID, months)
	if err != nil {
		return nil, fmt.Errorf("failed to query monthly stats: %w", err)
	}
	defer rows.Close()

	var stats []models.MonthlyStats
	for rows.Next() {
		var s models.MonthlyStats
		if err := rows.Scan(&s.Year, &s.Month, &s.ChaptersRead, &s.MangaStarted, &s.ReadingDays); err != nil {
			continue
		}
		if s.ReadingDays > 0 {
			s.AveragePerDay = float64(s.ChaptersRead) / float64(s.ReadingDays)
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// GetReadingHeatmap returns heatmap data for the last N days
func (r *Repository) GetReadingHeatmap(userID string, days int) ([]models.ReadingHeatmap, error) {
	query := `
		SELECT 
			date(read_at) as date,
			COUNT(*) as chapters_read
		FROM chapter_history
		WHERE user_id = ? AND read_at >= date('now', '-' || ? || ' days')
		GROUP BY date(read_at)
		ORDER BY date ASC`

	rows, err := r.db.Query(query, userID, days)
	if err != nil {
		return nil, fmt.Errorf("failed to query heatmap: %w", err)
	}
	defer rows.Close()

	// Find max for level calculation
	var heatmapData []struct {
		Date  string
		Count int
	}
	maxCount := 0
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			continue
		}
		heatmapData = append(heatmapData, struct {
			Date  string
			Count int
		}{date, count})
		if count > maxCount {
			maxCount = count
		}
	}

	// Calculate levels (0-4)
	var heatmap []models.ReadingHeatmap
	for _, d := range heatmapData {
		level := 0
		if maxCount > 0 {
			ratio := float64(d.Count) / float64(maxCount)
			if ratio > 0.75 {
				level = 4
			} else if ratio > 0.5 {
				level = 3
			} else if ratio > 0.25 {
				level = 2
			} else if ratio > 0 {
				level = 1
			}
		}
		heatmap = append(heatmap, models.ReadingHeatmap{
			Date:         d.Date,
			ChaptersRead: d.Count,
			Level:        level,
		})
	}

	return heatmap, nil
}
