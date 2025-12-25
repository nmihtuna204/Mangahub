package models

import (
	"time"
)

// ReadingProgress represents a user's reading progress for a manga
type ReadingProgress struct {
	ID             string     `json:"id" db:"id"`
	UserID         string     `json:"user_id" db:"user_id"`
	MangaID        string     `json:"manga_id" db:"manga_id"`
	CurrentChapter int        `json:"current_chapter" db:"current_chapter"`
	Status         string     `json:"status" db:"status"` // plan_to_read, reading, completed, on_hold, dropped
	IsFavorite     bool       `json:"is_favorite" db:"is_favorite"`
	StartedAt      *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	LastReadAt     time.Time  `json:"last_read_at" db:"last_read_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// ProgressWithManga combines progress with manga details
type ProgressWithManga struct {
	ReadingProgress
	Manga Manga `json:"manga"`
}

// UpdateProgressRequest represents a progress update request
type UpdateProgressRequest struct {
	MangaID        string `json:"manga_id" validate:"required"`
	CurrentChapter int    `json:"current_chapter" validate:"min=0"`
	Status         string `json:"status" validate:"omitempty,oneof=plan_to_read reading completed on_hold dropped"`
	IsFavorite     bool   `json:"is_favorite"`
}

// LibraryStats represents user library statistics
type LibraryStats struct {
	TotalManga     int     `json:"total_manga"`
	Reading        int     `json:"reading"`
	Completed      int     `json:"completed"`
	PlanToRead     int     `json:"plan_to_read"`
	Dropped        int     `json:"dropped"`
	TotalChapters  int     `json:"total_chapters_read"`
	AverageRating  float64 `json:"average_rating"`
}
