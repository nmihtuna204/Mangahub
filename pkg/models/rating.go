// Package models - Rating and Review System (Simplified)
// Hệ thống đánh giá manga đơn giản hóa
// Chức năng:
//   - Single rating scale (1-10)
//   - Optional review text with spoiler tags
//   - Helpful count tracking
//   - Auto-calculation of manga.average_rating via triggers
package models

import (
	"time"
)

// MangaRating represents a user's rating for a manga
type MangaRating struct {
	ID         string    `json:"id" db:"id"`
	MangaID    string    `json:"manga_id" db:"manga_id"`
	UserID     string    `json:"user_id" db:"user_id"`
	Rating     int       `json:"rating" db:"rating" validate:"required,min=1,max=10"` // 1-10 scale
	ReviewText string    `json:"review_text,omitempty" db:"review_text"`
	IsSpoiler  bool      `json:"is_spoiler" db:"is_spoiler"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// RatingWithUser includes user info for display
type RatingWithUser struct {
	MangaRating
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// RatingSummary provides aggregate rating statistics for a manga
type RatingSummary struct {
	MangaID            string  `json:"manga_id"`
	AverageRating      float64 `json:"average_rating"`      // 0.0 - 10.0
	RatingCount        int     `json:"rating_count"`        // total number of ratings
	RatingDistribution [10]int `json:"rating_distribution"` // count for each score 1-10
}

// ===== Request/Response Types for Rating API =====

// CreateRatingRequest is the payload for submitting a rating
type CreateRatingRequest struct {
	Rating     int    `json:"rating" validate:"required,min=1,max=10"`
	ReviewText string `json:"review_text,omitempty" validate:"omitempty,max=5000"`
	IsSpoiler  bool   `json:"is_spoiler"`
}

// UpdateRatingRequest is the payload for updating a rating
type UpdateRatingRequest struct {
	Rating     int    `json:"rating" validate:"required,min=1,max=10"`
	ReviewText string `json:"review_text,omitempty" validate:"omitempty,max=5000"`
	IsSpoiler  bool   `json:"is_spoiler"`
}

// MangaRatingsResponse is returned when fetching ratings for a manga
type MangaRatingsResponse struct {
	Summary RatingSummary    `json:"summary"`
	Ratings []RatingWithUser `json:"ratings"`
	Total   int              `json:"total"`
	Page    int              `json:"page"`
	HasMore bool             `json:"has_more"`
}
