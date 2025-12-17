// Package models - Rating and Review System
// Hệ thống đánh giá manga với aspect-based ratings
// Chức năng:
//   - Overall rating (1-10 scale)
//   - Aspect ratings (story, art, character, enjoyment)
//   - Reviews with spoiler tags
//   - Aggregate statistics
package models

import (
	"time"
)

// MangaRating represents a user's rating for a manga
type MangaRating struct {
	ID              string    `json:"id" db:"id"`
	MangaID         string    `json:"manga_id" db:"manga_id"`
	UserID          string    `json:"user_id" db:"user_id"`
	OverallRating   float64   `json:"overall_rating" db:"overall_rating"`     // 1.0 - 10.0
	StoryRating     float64   `json:"story_rating,omitempty" db:"story_rating"`
	ArtRating       float64   `json:"art_rating,omitempty" db:"art_rating"`
	CharacterRating float64   `json:"character_rating,omitempty" db:"character_rating"`
	EnjoymentRating float64   `json:"enjoyment_rating,omitempty" db:"enjoyment_rating"`
	ReviewText      string    `json:"review_text,omitempty" db:"review_text"`
	IsSpoiler       bool      `json:"is_spoiler" db:"is_spoiler"`
	HelpfulCount    int       `json:"helpful_count" db:"helpful_count"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// RatingAggregate stores aggregated rating stats for a manga
type RatingAggregate struct {
	MangaID           string  `json:"manga_id" db:"manga_id"`
	AverageRating     float64 `json:"average_rating" db:"average_rating"`
	WeightedRating    float64 `json:"weighted_rating" db:"weighted_rating"` // Bayesian average
	TotalRatings      int     `json:"total_ratings" db:"total_ratings"`
	RatingDistribution [10]int `json:"rating_distribution"` // Count per score 1-10
	AverageStory      float64 `json:"average_story,omitempty" db:"average_story"`
	AverageArt        float64 `json:"average_art,omitempty" db:"average_art"`
	AverageCharacter  float64 `json:"average_character,omitempty" db:"average_character"`
	AverageEnjoyment  float64 `json:"average_enjoyment,omitempty" db:"average_enjoyment"`
}

// AspectRating for individual aspects
type AspectRating struct {
	Aspect  string  `json:"aspect"`  // story, art, character, enjoyment
	Rating  float64 `json:"rating"`
	Weight  float64 `json:"weight"`  // How much this aspect contributes
}

// MangaReview represents a detailed review
type MangaReview struct {
	ID           string    `json:"id" db:"id"`
	MangaID      string    `json:"manga_id" db:"manga_id"`
	UserID       string    `json:"user_id" db:"user_id"`
	Username     string    `json:"username" db:"-"` // Joined
	Title        string    `json:"title" db:"title"`
	Content      string    `json:"content" db:"content"`
	IsSpoiler    bool      `json:"is_spoiler" db:"is_spoiler"`
	Rating       float64   `json:"rating" db:"rating"`
	HelpfulCount int       `json:"helpful_count" db:"helpful_count"`
	ReportCount  int       `json:"report_count" db:"report_count"`
	IsApproved   bool      `json:"is_approved" db:"is_approved"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// ReviewHelpful tracks which users found a review helpful
type ReviewHelpful struct {
	ID        string    `json:"id" db:"id"`
	ReviewID  string    `json:"review_id" db:"review_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	IsHelpful bool      `json:"is_helpful" db:"is_helpful"` // true = helpful, false = not helpful
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Rating aspects
const (
	AspectStory     = "story"
	AspectArt       = "art"
	AspectCharacter = "character"
	AspectEnjoyment = "enjoyment"
)

// CalculateWeightedRating uses Bayesian average
// Formula: WR = (v / (v + m)) * R + (m / (v + m)) * C
// Where: v = number of votes, m = minimum votes required, R = average rating, C = mean rating across all manga
func CalculateWeightedRating(averageRating float64, totalRatings int, minVotes int, globalMean float64) float64 {
	v := float64(totalRatings)
	m := float64(minVotes)
	R := averageRating
	C := globalMean

	if v+m == 0 {
		return 0
	}

	return (v/(v+m))*R + (m/(v+m))*C
}

// ===== Request/Response Types for Rating API =====

// CreateRatingRequest is the payload for submitting a rating
type CreateRatingRequest struct {
	OverallRating   float64 `json:"overall_rating" validate:"required,min=1,max=10"`
	StoryRating     float64 `json:"story_rating,omitempty" validate:"omitempty,min=1,max=10"`
	ArtRating       float64 `json:"art_rating,omitempty" validate:"omitempty,min=1,max=10"`
	CharacterRating float64 `json:"character_rating,omitempty" validate:"omitempty,min=1,max=10"`
	EnjoymentRating float64 `json:"enjoyment_rating,omitempty" validate:"omitempty,min=1,max=10"`
	ReviewText      string  `json:"review_text,omitempty" validate:"max=5000"`
	IsSpoiler       bool    `json:"is_spoiler"`
}

// RatingWithUser includes user info for display
type RatingWithUser struct {
	MangaRating
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// MangaRatingsSummary is returned when fetching ratings for a manga
type MangaRatingsSummary struct {
	MangaID        string           `json:"manga_id"`
	Aggregate      RatingAggregate  `json:"aggregate"`
	UserRating     *MangaRating     `json:"user_rating,omitempty"` // Current user's rating if exists
	RecentRatings  []RatingWithUser `json:"recent_ratings"`
}
