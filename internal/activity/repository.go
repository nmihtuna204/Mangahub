// Package activity - Activity Feed Repository
// Handles database operations for user activity tracking
package activity

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"mangahub/pkg/models"
)

// Repository defines activity data operations
type Repository interface {
	Create(ctx context.Context, activity *models.Activity) error
	GetRecent(ctx context.Context, limit, offset int) ([]models.Activity, int, error)
	GetByUser(ctx context.Context, userID string, limit, offset int) ([]models.Activity, int, error)
}

type repository struct {
	db *sql.DB
}

// NewRepository creates a new activity repository
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

// Create inserts a new activity entry
func (r *repository) Create(ctx context.Context, activity *models.Activity) error {
	if activity.ID == "" {
		activity.ID = uuid.New().String()
	}
	if activity.CreatedAt.IsZero() {
		activity.CreatedAt = time.Now()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO activity_feed (id, user_id, username, activity_type, manga_id, manga_title, chapter_number, rating, comment_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		activity.ID, activity.UserID, activity.Username, activity.ActivityType,
		activity.MangaID, activity.MangaTitle, activity.ChapterNumber, activity.Rating,
		activity.CommentText, activity.CreatedAt,
	)
	return err
}

// GetRecent retrieves recent activities across all users
func (r *repository) GetRecent(ctx context.Context, limit, offset int) ([]models.Activity, int, error) {
	// Get total count
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM activity_feed").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count activities: %w", err)
	}

	// Get activities
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, username, activity_type, manga_id, manga_title, 
		       chapter_number, rating, comment_text, created_at
		FROM activity_feed
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query activities: %w", err)
	}
	defer rows.Close()

	var activities []models.Activity
	for rows.Next() {
		var a models.Activity
		err := rows.Scan(&a.ID, &a.UserID, &a.Username, &a.ActivityType,
			&a.MangaID, &a.MangaTitle, &a.ChapterNumber, &a.Rating,
			&a.CommentText, &a.CreatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("scan activity: %w", err)
		}
		activities = append(activities, a)
	}

	return activities, total, nil
}

// GetByUser retrieves activities for a specific user
func (r *repository) GetByUser(ctx context.Context, userID string, limit, offset int) ([]models.Activity, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM activity_feed WHERE user_id = ?", userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count user activities: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, username, activity_type, manga_id, manga_title,
		       chapter_number, rating, comment_text, created_at
		FROM activity_feed
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query user activities: %w", err)
	}
	defer rows.Close()

	var activities []models.Activity
	for rows.Next() {
		var a models.Activity
		err := rows.Scan(&a.ID, &a.UserID, &a.Username, &a.ActivityType,
			&a.MangaID, &a.MangaTitle, &a.ChapterNumber, &a.Rating,
			&a.CommentText, &a.CreatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("scan activity: %w", err)
		}
		activities = append(activities, a)
	}

	return activities, total, nil
}
