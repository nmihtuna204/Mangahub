// Package progress - Reading Progress Service
// Xử lý logic theo dõi tiến độ đọc truyện của user
// Chức năng:
//   - Update reading progress (chapter, status, rating)
//   - List user's manga library với progress
//   - Trigger protocol bridge khi có update
//   - Manage reading history
package progress

import (
	"context"

	"github.com/yourusername/mangahub/pkg/models"
	"github.com/yourusername/mangahub/pkg/utils"
)

type Service interface {
	Update(ctx context.Context, userID string, req models.UpdateProgressRequest) (*models.ReadingProgress, error)
	List(ctx context.Context, userID string) ([]models.ProgressWithManga, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Update(ctx context.Context, userID string, req models.UpdateProgressRequest) (*models.ReadingProgress, error) {
	if err := utils.ValidateStruct(req); err != nil {
		return nil, models.NewAppError(models.ErrCodeValidation, "invalid progress data", 400, err)
	}
	return s.repo.AddOrUpdate(ctx, userID, req)
}

func (s *service) List(ctx context.Context, userID string) ([]models.ProgressWithManga, error) {
	return s.repo.ListByUser(ctx, userID)
}
