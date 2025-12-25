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

	// GetMangaRatings returns aggregate stats + recent ratings for a manga
	GetMangaRatings(ctx context.Context, mangaID string, limit, offset int) (*models.MangaRatingsResponse, error)

	// GetUserRating returns a user's rating for a manga
	GetUserRating(ctx context.Context, userID, mangaID string) (*models.MangaRating, error)

	// DeleteRating removes a user's rating
	DeleteRating(ctx context.Context, userID, mangaID string) error
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

	// Validation is handled by struct tags in CreateRatingRequest (min=1, max=10)
	rating, err := s.repo.CreateOrUpdate(ctx, userID, mangaID, req)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to save rating", 500, err)
	}

	return rating, nil
}

// GetMangaRatings returns aggregate stats + recent ratings for a manga
func (s *service) GetMangaRatings(ctx context.Context, mangaID string, limit, offset int) (*models.MangaRatingsResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// Get summary (aggregate stats from manga table)
	summary, err := s.repo.GetSummary(ctx, mangaID)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to get rating summary", 500, err)
	}

	// Get recent ratings with user info
	ratings, err := s.repo.GetByManga(ctx, mangaID, limit, offset)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to get recent ratings", 500, err)
	}

	return &models.MangaRatingsResponse{
		Summary: *summary,
		Ratings: ratings,
		Total:   summary.RatingCount,
		Page:    offset/limit + 1,
		HasMore: offset+limit < summary.RatingCount,
	}, nil
}

// GetUserRating returns a specific user's rating for a manga
func (s *service) GetUserRating(ctx context.Context, userID, mangaID string) (*models.MangaRating, error) {
	if userID == "" || mangaID == "" {
		return nil, models.NewAppError(models.ErrCodeValidation, "user_id and manga_id are required", 400, nil)
	}

	rating, err := s.repo.GetByUserAndManga(ctx, userID, mangaID)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to get user rating", 500, err)
	}
	return rating, nil
}

// DeleteRating removes a user's rating for a manga
func (s *service) DeleteRating(ctx context.Context, userID, mangaID string) error {
	if userID == "" || mangaID == "" {
		return models.NewAppError(models.ErrCodeValidation, "user_id and manga_id are required", 400, nil)
	}

	err := s.repo.Delete(ctx, userID, mangaID)
	if err != nil {
		return models.NewAppError(models.ErrCodeNotFound, "rating not found", 404, err)
	}
	return nil
}
