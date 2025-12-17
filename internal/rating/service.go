// Package rating - Rating Service
// Business logic layer cho rating system
// Chức năng:
//   - Validate rating requests
//   - Coordinate between handlers and repository
//   - Build rating summaries with aggregates
package rating

import (
	"context"

	"mangahub/pkg/models"
	"mangahub/pkg/utils"
)

// Service defines business operations for ratings
type Service interface {
	// Rate creates or updates a user's rating for a manga
	Rate(ctx context.Context, userID, mangaID string, req models.CreateRatingRequest) (*models.MangaRating, error)

	// GetRatingSummary returns aggregate stats + recent ratings for a manga
	GetRatingSummary(ctx context.Context, mangaID string, currentUserID string) (*models.MangaRatingsSummary, error)

	// GetUserRating returns a user's rating for a manga
	GetUserRating(ctx context.Context, userID, mangaID string) (*models.MangaRating, error)

	// DeleteRating removes a user's rating
	DeleteRating(ctx context.Context, userID, mangaID string) error

	// GetTopRated returns top rated manga for leaderboards
	GetTopRated(ctx context.Context, limit, offset int) ([]models.RatingAggregate, error)
}

type service struct {
	repo Repository
}

// NewService creates a new rating service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// Rate creates or updates a rating after validation
func (s *service) Rate(ctx context.Context, userID, mangaID string, req models.CreateRatingRequest) (*models.MangaRating, error) {
	// Validate request using struct validation
	if err := utils.ValidateStruct(req); err != nil {
		return nil, models.NewAppError(models.ErrCodeValidation, "invalid rating data", 400, err)
	}

	// Additional validation: overall rating must be between 1-10
	if req.OverallRating < 1 || req.OverallRating > 10 {
		return nil, models.NewAppError(models.ErrCodeValidation, "overall rating must be between 1 and 10", 400, nil)
	}

	rating, err := s.repo.CreateOrUpdate(ctx, userID, mangaID, req)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to save rating", 500, err)
	}

	return rating, nil
}

// GetRatingSummary builds a complete summary of ratings for a manga
// Includes: aggregate stats, recent reviews, and current user's rating if logged in
func (s *service) GetRatingSummary(ctx context.Context, mangaID string, currentUserID string) (*models.MangaRatingsSummary, error) {
	summary := &models.MangaRatingsSummary{
		MangaID: mangaID,
	}

	// Get aggregate stats
	agg, err := s.repo.GetAggregate(ctx, mangaID)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to get rating aggregate", 500, err)
	}
	summary.Aggregate = *agg

	// Get recent ratings (limit 10)
	recent, err := s.repo.GetByManga(ctx, mangaID, 10, 0)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to get recent ratings", 500, err)
	}
	summary.RecentRatings = recent

	// Get current user's rating if logged in
	if currentUserID != "" {
		userRating, err := s.repo.GetByUserAndManga(ctx, currentUserID, mangaID)
		if err != nil {
			// Log error but don't fail - user rating is optional
			_ = err
		}
		summary.UserRating = userRating
	}

	return summary, nil
}

// GetUserRating returns a specific user's rating for a manga
func (s *service) GetUserRating(ctx context.Context, userID, mangaID string) (*models.MangaRating, error) {
	rating, err := s.repo.GetByUserAndManga(ctx, userID, mangaID)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to get user rating", 500, err)
	}
	return rating, nil
}

// DeleteRating removes a user's rating for a manga
func (s *service) DeleteRating(ctx context.Context, userID, mangaID string) error {
	err := s.repo.Delete(ctx, userID, mangaID)
	if err != nil {
		return models.NewAppError(models.ErrCodeNotFound, "rating not found", 404, err)
	}
	return nil
}

// GetTopRated returns top rated manga sorted by weighted rating
func (s *service) GetTopRated(ctx context.Context, limit, offset int) ([]models.RatingAggregate, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	ratings, err := s.repo.GetTopRatedManga(ctx, limit, offset)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to get top rated manga", 500, err)
	}
	return ratings, nil
}
