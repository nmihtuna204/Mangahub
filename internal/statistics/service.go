// Package statistics - Reading Statistics Service
// Business logic for reading statistics and analytics
package statistics

import (
	"context"
	"time"

	"mangahub/pkg/database"
	"mangahub/pkg/models"
)

// Service provides statistics business logic
type Service struct {
	repo *Repository
	db   *database.DB
}

// NewService creates a new statistics service
func NewService(db *database.DB) *Service {
	return &Service{
		repo: NewRepository(db),
		db:   db,
	}
}

// RecordChapterRead records a chapter read event and updates stats
func (s *Service) RecordChapterRead(ctx context.Context, req *models.RecordChapterRequest, userID string) error {
	history := &models.ChapterHistory{
		UserID:        userID,
		MangaID:       req.MangaID,
		ChapterNumber: req.ChapterNumber,
		PagesRead:     req.PagesRead,
		TimeMinutes:   req.TimeMinutes,
		ReadAt:        time.Now(),
	}

	return s.repo.RecordChapterRead(history)
}

// GetStatistics returns comprehensive reading statistics for a user
func (s *Service) GetStatistics(ctx context.Context, userID string) (*models.ReadingStats, error) {
	stats := &models.ReadingStats{
		UserID:    userID,
		UpdatedAt: time.Now(),
	}

	// Get total manga read (from reading_progress)
	var totalManga int
	err := s.db.QueryRow(`
		SELECT COUNT(DISTINCT manga_id) 
		FROM reading_progress 
		WHERE user_id = ? AND status IN ('reading', 'completed')`, userID).Scan(&totalManga)
	if err == nil {
		stats.TotalMangaRead = totalManga
	}

	// Get total chapters read
	if total, err := s.repo.GetTotalChaptersRead(userID); err == nil {
		stats.TotalChaptersRead = total
	}

	// Get total pages read
	if pages, err := s.repo.GetTotalPagesRead(userID); err == nil {
		stats.TotalPagesRead = pages
	}

	// Get total time spent
	if minutes, err := s.repo.GetTotalTimeSpent(userID); err == nil {
		stats.TotalTimeMinutes = minutes
	}

	// Get average rating from reading_progress
	var avgRating float64
	err = s.db.QueryRow(`
		SELECT COALESCE(AVG(rating), 0) 
		FROM reading_progress 
		WHERE user_id = ? AND rating IS NOT NULL`, userID).Scan(&avgRating)
	if err == nil {
		stats.AverageRating = avgRating
	}

	// Get current streak
	if streak, err := s.repo.GetCurrentStreak(userID); err == nil {
		stats.ReadingStreak = streak
	}

	// Get longest streak
	if longest, err := s.repo.GetLongestStreak(userID); err == nil {
		stats.LongestStreak = longest
	}

	// Calculate daily average (last 30 days)
	var last30Days int
	err = s.db.QueryRow(`
		SELECT COUNT(*) 
		FROM chapter_history 
		WHERE user_id = ? AND read_at >= date('now', '-30 days')`, userID).Scan(&last30Days)
	if err == nil && last30Days > 0 {
		stats.DailyAverage = float64(last30Days) / 30.0
	}

	// Get genre distribution
	if genres, err := s.repo.GetGenreDistribution(userID); err == nil {
		stats.GenreDistribution = genres
		if len(genres) > 0 {
			stats.MostReadGenre = genres[0].Genre
		}
	}

	// Get monthly stats (last 12 months)
	if monthly, err := s.repo.GetMonthlyStats(userID, 12); err == nil {
		stats.MonthlyStats = monthly
	}

	// Get favorite author
	var favoriteAuthor string
	err = s.db.QueryRow(`
		SELECT m.author
		FROM chapter_history ch
		JOIN manga m ON ch.manga_id = m.id
		WHERE ch.user_id = ? AND m.author != ''
		GROUP BY m.author
		ORDER BY COUNT(*) DESC
		LIMIT 1`, userID).Scan(&favoriteAuthor)
	if err == nil {
		stats.FavoriteAuthor = favoriteAuthor
	}

	// Get fastest series completion
	fastestSeries, err := s.getFastestSeries(userID)
	if err == nil && fastestSeries != nil {
		stats.FastestSeries = fastestSeries
	}

	// Get slowest series completion
	slowestSeries, err := s.getSlowestSeries(userID)
	if err == nil && slowestSeries != nil {
		stats.SlowestSeries = slowestSeries
	}

	return stats, nil
}

// getFastestSeries returns the fastest completed series
func (s *Service) getFastestSeries(userID string) (*models.SeriesRecord, error) {
	query := `
		SELECT 
			rp.manga_id,
			m.title,
			CAST(julianday(rp.completed_at) - julianday(rp.started_at) AS INTEGER) as days,
			rp.current_chapter
		FROM reading_progress rp
		JOIN manga m ON rp.manga_id = m.id
		WHERE rp.user_id = ? 
			AND rp.status = 'completed' 
			AND rp.started_at IS NOT NULL 
			AND rp.completed_at IS NOT NULL
		ORDER BY days ASC
		LIMIT 1`

	var record models.SeriesRecord
	err := s.db.QueryRow(query, userID).Scan(&record.MangaID, &record.Title, &record.Days, &record.Chapters)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// getSlowestSeries returns the slowest completed series
func (s *Service) getSlowestSeries(userID string) (*models.SeriesRecord, error) {
	query := `
		SELECT 
			rp.manga_id,
			m.title,
			CAST(julianday(rp.completed_at) - julianday(rp.started_at) AS INTEGER) as days,
			rp.current_chapter
		FROM reading_progress rp
		JOIN manga m ON rp.manga_id = m.id
		WHERE rp.user_id = ? 
			AND rp.status = 'completed' 
			AND rp.started_at IS NOT NULL 
			AND rp.completed_at IS NOT NULL
		ORDER BY days DESC
		LIMIT 1`

	var record models.SeriesRecord
	err := s.db.QueryRow(query, userID).Scan(&record.MangaID, &record.Title, &record.Days, &record.Chapters)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// GetStatsOverview returns a quick stats overview for dashboard
func (s *Service) GetStatsOverview(ctx context.Context, userID string) (*models.StatsOverview, error) {
	overview := &models.StatsOverview{}

	// Total manga count
	s.db.QueryRow(`
		SELECT COUNT(DISTINCT manga_id) 
		FROM reading_progress 
		WHERE user_id = ?`, userID).Scan(&overview.TotalManga)

	// Total chapters
	if total, err := s.repo.GetTotalChaptersRead(userID); err == nil {
		overview.TotalChapters = total
	}

	// Current streak
	if streak, err := s.repo.GetCurrentStreak(userID); err == nil {
		overview.CurrentStreak = streak
	}

	// This week count
	s.db.QueryRow(`
		SELECT COUNT(*) 
		FROM chapter_history 
		WHERE user_id = ? AND read_at >= date('now', 'weekday 0', '-7 days')`, userID).Scan(&overview.ThisWeekCount)

	// This month count
	s.db.QueryRow(`
		SELECT COUNT(*) 
		FROM chapter_history 
		WHERE user_id = ? AND read_at >= date('now', 'start of month')`, userID).Scan(&overview.ThisMonthCount)

	// Average rating
	s.db.QueryRow(`
		SELECT COALESCE(AVG(rating), 0) 
		FROM reading_progress 
		WHERE user_id = ? AND rating IS NOT NULL`, userID).Scan(&overview.AverageRating)

	return overview, nil
}

// GetChapterHistory retrieves paginated chapter reading history
func (s *Service) GetChapterHistory(ctx context.Context, userID string, limit, offset int) ([]models.ChapterHistory, error) {
	return s.repo.GetChapterHistory(userID, limit, offset)
}

// GetReadingHeatmap returns heatmap data for visualization
func (s *Service) GetReadingHeatmap(ctx context.Context, userID string, days int) ([]models.ReadingHeatmap, error) {
	if days <= 0 {
		days = 365 // Default to 1 year
	}
	return s.repo.GetReadingHeatmap(userID, days)
}

// GetDailyStats returns daily stats within a date range
func (s *Service) GetDailyStats(ctx context.Context, userID string, startDate, endDate time.Time) ([]models.DailyStats, error) {
	return s.repo.GetDailyStats(userID, startDate, endDate)
}
