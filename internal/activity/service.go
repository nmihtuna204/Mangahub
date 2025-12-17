// Package activity - Activity Feed Service
// Business logic for recording and retrieving user activities
package activity

import (
	"context"
	"fmt"
)

// Service provides activity business logic
type Service struct {
	repo Repository
}

// NewService creates a new activity service
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// RecordChapterRead records when a user reads a chapter
func (s *Service) RecordChapterRead(ctx context.Context, userID, username, mangaID, mangaTitle string, chapterNum int) error {
	activity := &Activity{
		UserID:        userID,
		Username:      username,
		ActivityType:  "chapter_read",
		MangaID:       mangaID,
		MangaTitle:    mangaTitle,
		ChapterNumber: &chapterNum,
	}
	return s.repo.Create(ctx, activity)
}

// RecordMangaRated records when a user rates a manga
func (s *Service) RecordMangaRated(ctx context.Context, userID, username, mangaID, mangaTitle string, rating float64) error {
	activity := &Activity{
		UserID:       userID,
		Username:     username,
		ActivityType: "manga_rated",
		MangaID:      mangaID,
		MangaTitle:   mangaTitle,
		Rating:       &rating,
	}
	return s.repo.Create(ctx, activity)
}

// RecordMangaCompleted records when a user completes a manga
func (s *Service) RecordMangaCompleted(ctx context.Context, userID, username, mangaID, mangaTitle string) error {
	activity := &Activity{
		UserID:       userID,
		Username:     username,
		ActivityType: "manga_completed",
		MangaID:      mangaID,
		MangaTitle:   mangaTitle,
	}
	return s.repo.Create(ctx, activity)
}

// RecordCommentAdded records when a user adds a comment
func (s *Service) RecordCommentAdded(ctx context.Context, userID, username, mangaID, mangaTitle, commentText string) error {
	activity := &Activity{
		UserID:       userID,
		Username:     username,
		ActivityType: "comment_added",
		MangaID:      mangaID,
		MangaTitle:   mangaTitle,
		CommentText:  &commentText,
	}
	return s.repo.Create(ctx, activity)
}

// GetRecentActivities retrieves recent activities
func (s *Service) GetRecentActivities(ctx context.Context, limit, offset int) ([]Activity, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.GetRecent(ctx, limit, offset)
}

// GetUserActivities retrieves activities for a specific user
func (s *Service) GetUserActivities(ctx context.Context, userID string, limit, offset int) ([]Activity, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.GetByUser(ctx, userID, limit, offset)
}

// FormatActivityMessage returns a human-readable activity message
func FormatActivityMessage(activity Activity) string {
	switch activity.ActivityType {
	case "chapter_read":
		if activity.ChapterNumber != nil {
			return fmt.Sprintf("%s read Chapter %d of %s", activity.Username, *activity.ChapterNumber, activity.MangaTitle)
		}
		return fmt.Sprintf("%s is reading %s", activity.Username, activity.MangaTitle)
	
	case "manga_rated":
		if activity.Rating != nil {
			return fmt.Sprintf("%s rated %s %.1f/10", activity.Username, activity.MangaTitle, *activity.Rating)
		}
		return fmt.Sprintf("%s rated %s", activity.Username, activity.MangaTitle)
	
	case "manga_completed":
		return fmt.Sprintf("%s completed %s", activity.Username, activity.MangaTitle)
	
	case "comment_added":
		return fmt.Sprintf("%s commented on %s", activity.Username, activity.MangaTitle)
	
	default:
		return fmt.Sprintf("%s activity on %s", activity.Username, activity.MangaTitle)
	}
}
