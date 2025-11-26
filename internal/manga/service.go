package manga

import (
	"context"

	"github.com/yourusername/mangahub/pkg/models"
)

type Service interface {
	List(ctx context.Context, req models.MangaSearchRequest) (*models.MangaListResponse, error)
	GetByID(ctx context.Context, id string) (*models.Manga, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context, req models.MangaSearchRequest) (*models.MangaListResponse, error) {
	manga, total, err := s.repo.List(ctx, req)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to list manga", 500, err)
	}

	hasMore := req.Offset+req.Limit < total
	return &models.MangaListResponse{
		Data:    manga,
		Total:   total,
		Limit:   req.Limit,
		Offset:  req.Offset,
		HasMore: hasMore,
	}, nil
}

func (s *service) GetByID(ctx context.Context, id string) (*models.Manga, error) {
	return s.repo.GetByID(ctx, id)
}
