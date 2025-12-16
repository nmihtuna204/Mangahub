package models

import (
	"time"
)

// ReadingStats represents comprehensive reading statistics for a user
type ReadingStats struct {
	UserID            string         `json:"user_id" db:"user_id"`
	TotalMangaRead    int            `json:"total_manga_read"`
	TotalChaptersRead int            `json:"total_chapters_read"`
	TotalPagesRead    int            `json:"total_pages_read"`
	AverageRating     float64        `json:"average_rating"`
	ReadingStreak     int            `json:"reading_streak_days"`
	LongestStreak     int            `json:"longest_streak_days"`
	DailyAverage      float64        `json:"daily_average_chapters"`
	TotalTimeMinutes  int            `json:"total_time_minutes"`
	MostReadGenre     string         `json:"most_read_genre"`
	FavoriteAuthor    string         `json:"favorite_author"`
	FastestSeries     *SeriesRecord  `json:"fastest_series,omitempty"`
	SlowestSeries     *SeriesRecord  `json:"slowest_series,omitempty"`
	GenreDistribution []GenreCount   `json:"genre_distribution"`
	MonthlyStats      []MonthlyStats `json:"monthly_stats"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// SeriesRecord represents a manga reading record (fastest/slowest completion)
type SeriesRecord struct {
	MangaID  string `json:"manga_id"`
	Title    string `json:"title"`
	Days     int    `json:"days"`
	Chapters int    `json:"chapters"`
}

// GenreCount represents genre reading distribution
type GenreCount struct {
	Genre      string  `json:"genre"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// MonthlyStats represents reading stats for a specific month
type MonthlyStats struct {
	Year          int     `json:"year"`
	Month         int     `json:"month"`
	ChaptersRead  int     `json:"chapters_read"`
	MangaStarted  int     `json:"manga_started"`
	MangaFinished int     `json:"manga_finished"`
	ReadingDays   int     `json:"reading_days"`
	AveragePerDay float64 `json:"average_per_day"`
}

// DailyStats represents daily reading activity
type DailyStats struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	Date         time.Time `json:"date" db:"date"`
	ChaptersRead int       `json:"chapters_read" db:"chapters_read"`
	PagesRead    int       `json:"pages_read" db:"pages_read"`
	TimeMinutes  int       `json:"time_minutes" db:"time_minutes"`
	MangaCount   int       `json:"manga_count" db:"manga_count"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// ChapterHistory records individual chapter reads for detailed tracking
type ChapterHistory struct {
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"user_id" db:"user_id"`
	MangaID       string    `json:"manga_id" db:"manga_id"`
	ChapterNumber int       `json:"chapter_number" db:"chapter_number"`
	PagesRead     int       `json:"pages_read" db:"pages_read"`
	TimeMinutes   int       `json:"time_minutes" db:"time_minutes"`
	ReadAt        time.Time `json:"read_at" db:"read_at"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// ReadingHeatmap represents reading activity for heatmap visualization
type ReadingHeatmap struct {
	Date         string `json:"date"`
	ChaptersRead int    `json:"chapters_read"`
	Level        int    `json:"level"` // 0-4 intensity level
}

// StatsOverview provides a quick summary for dashboard display
type StatsOverview struct {
	TotalManga     int     `json:"total_manga"`
	TotalChapters  int     `json:"total_chapters"`
	CurrentStreak  int     `json:"current_streak"`
	ThisWeekCount  int     `json:"this_week_count"`
	ThisMonthCount int     `json:"this_month_count"`
	AverageRating  float64 `json:"average_rating"`
}

// RecordChapterRequest is used to record a chapter read
type RecordChapterRequest struct {
	MangaID       string `json:"manga_id" validate:"required"`
	ChapterNumber int    `json:"chapter_number" validate:"required,min=0"`
	PagesRead     int    `json:"pages_read,omitempty"`
	TimeMinutes   int    `json:"time_minutes,omitempty"`
}
