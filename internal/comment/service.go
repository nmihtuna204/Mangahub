// Package comment - Comment Service
// Business logic layer cho comment system
// Chức năng:
//   - Validate comment requests
//   - Build comment threads with replies
//   - Coordinate likes/unlikes
//   - Handle pagination
package comment

import (
	"context"

	"mangahub/pkg/models"
	"mangahub/pkg/utils"
)

// Service defines business operations for comments
type Service interface {
	// Create creates a new comment
	Create(ctx context.Context, userID, mangaID string, req models.CreateCommentRequest) (*models.Comment, error)

	// GetComments retrieves comments for a manga with optional chapter filter
	GetComments(ctx context.Context, mangaID string, chapterNumber *int, currentUserID string, page, pageSize int) (*models.CommentListResponse, error)

	// Update updates a comment's content
	Update(ctx context.Context, id, userID string, req models.UpdateCommentRequest) (*models.Comment, error)

	// Delete soft-deletes a comment
	Delete(ctx context.Context, id, userID string) error

	// Like adds a like to a comment
	Like(ctx context.Context, commentID, userID string) error

	// Unlike removes a like from a comment
	Unlike(ctx context.Context, commentID, userID string) error
}

type service struct {
	repo Repository
}

// NewService creates a new comment service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// Create creates a new comment after validation
func (s *service) Create(ctx context.Context, userID, mangaID string, req models.CreateCommentRequest) (*models.Comment, error) {
	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		return nil, models.NewAppError(models.ErrCodeValidation, "invalid comment data", 400, err)
	}

	// Validate content length
	if len(req.Content) < 1 || len(req.Content) > 2000 {
		return nil, models.NewAppError(models.ErrCodeValidation, "comment must be 1-2000 characters", 400, nil)
	}

	// If replying, verify parent exists
	if req.ParentID != "" {
		parent, err := s.repo.GetByID(ctx, req.ParentID)
		if err != nil {
			return nil, models.NewAppError(models.ErrCodeInternal, "failed to verify parent comment", 500, err)
		}
		if parent == nil {
			return nil, models.NewAppError(models.ErrCodeNotFound, "parent comment not found", 404, nil)
		}
	}

	comment, err := s.repo.Create(ctx, userID, mangaID, req)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to create comment", 500, err)
	}

	return comment, nil
}

// GetComments retrieves comments with pagination and nested replies
func (s *service) GetComments(ctx context.Context, mangaID string, chapterNumber *int, currentUserID string, page, pageSize int) (*models.CommentListResponse, error) {
	// Default pagination values
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}

	offset := (page - 1) * pageSize

	// Get total count
	totalCount, err := s.repo.CountByManga(ctx, mangaID, chapterNumber)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to count comments", 500, err)
	}

	// Get top-level comments
	comments, err := s.repo.GetByManga(ctx, mangaID, chapterNumber, pageSize, offset)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to get comments", 500, err)
	}

	// Build response with nested replies
	var commentsWithReplies []models.CommentWithReplies
	for _, c := range comments {
		cwr := models.CommentWithReplies{
			CommentWithUser: c,
		}

		// Check if current user liked this comment
		if currentUserID != "" {
			liked, _ := s.repo.HasLiked(ctx, c.ID, currentUserID)
			cwr.LikedByMe = liked
		}

		// Get replies for this comment
		replies, err := s.repo.GetReplies(ctx, c.ID)
		if err == nil && len(replies) > 0 {
			// Check like status for replies too
			if currentUserID != "" {
				for i := range replies {
					liked, _ := s.repo.HasLiked(ctx, replies[i].ID, currentUserID)
					replies[i].LikedByMe = liked
				}
			}
			cwr.Replies = replies
		}

		commentsWithReplies = append(commentsWithReplies, cwr)
	}

	return &models.CommentListResponse{
		Comments:   commentsWithReplies,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		HasMore:    offset+len(comments) < totalCount,
	}, nil
}

// Update updates a comment's content
func (s *service) Update(ctx context.Context, id, userID string, req models.UpdateCommentRequest) (*models.Comment, error) {
	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		return nil, models.NewAppError(models.ErrCodeValidation, "invalid comment data", 400, err)
	}

	comment, err := s.repo.Update(ctx, id, userID, req)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeNotFound, "comment not found or not owned by you", 404, err)
	}

	return comment, nil
}

// Delete soft-deletes a comment
func (s *service) Delete(ctx context.Context, id, userID string) error {
	err := s.repo.Delete(ctx, id, userID)
	if err != nil {
		return models.NewAppError(models.ErrCodeNotFound, "comment not found or not owned by you", 404, err)
	}
	return nil
}

// Like adds a like to a comment
func (s *service) Like(ctx context.Context, commentID, userID string) error {
	// Verify comment exists
	comment, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return models.NewAppError(models.ErrCodeInternal, "failed to get comment", 500, err)
	}
	if comment == nil {
		return models.NewAppError(models.ErrCodeNotFound, "comment not found", 404, nil)
	}

	err = s.repo.Like(ctx, commentID, userID)
	if err != nil {
		return models.NewAppError(models.ErrCodeInternal, "failed to like comment", 500, err)
	}
	return nil
}

// Unlike removes a like from a comment
func (s *service) Unlike(ctx context.Context, commentID, userID string) error {
	err := s.repo.Unlike(ctx, commentID, userID)
	if err != nil {
		return models.NewAppError(models.ErrCodeNotFound, "like not found", 404, err)
	}
	return nil
}
