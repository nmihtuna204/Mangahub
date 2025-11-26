package progress

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/mangahub/pkg/models"
)

type Repository interface {
	AddOrUpdate(ctx context.Context, userID string, req models.UpdateProgressRequest) (*models.ReadingProgress, error)
	ListByUser(ctx context.Context, userID string) ([]models.ProgressWithManga, error)
}

type repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

func (r *repository) AddOrUpdate(ctx context.Context, userID string, req models.UpdateProgressRequest) (*models.ReadingProgress, error) {
	now := time.Now()

	var existingID string
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM reading_progress WHERE user_id = ? AND manga_id = ?",
		userID, req.MangaID,
	).Scan(&existingID)

	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("check progress: %w", err)
	}

	if err == sql.ErrNoRows {
		id := uuid.New().String()
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO reading_progress
			(id, user_id, manga_id, current_chapter, status, rating, notes, is_favorite,
			 last_read_at, created_at, updated_at, sync_version)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			id, userID, req.MangaID, req.CurrentChapter, req.Status, req.Rating, req.Notes,
			req.IsFavorite, now, now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("insert progress: %w", err)
		}
		existingID = id
	} else {
		_, err = r.db.ExecContext(ctx, `
			UPDATE reading_progress
			SET current_chapter = ?, status = ?, rating = ?, notes = ?,
			    is_favorite = ?, last_read_at = ?, updated_at = updated_at, sync_version = sync_version + 1
			WHERE id = ?`,
			req.CurrentChapter, req.Status, req.Rating, req.Notes,
			req.IsFavorite, now, existingID,
		)
		if err != nil {
			return nil, fmt.Errorf("update progress: %w", err)
		}
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, manga_id, current_chapter, total_chapters, status,
		       rating, notes, is_favorite, started_at, completed_at,
		       last_read_at, created_at, updated_at, sync_version
		FROM reading_progress WHERE id = ?`, existingID)

	var p models.ReadingProgress
	err = row.Scan(
		&p.ID, &p.UserID, &p.MangaID, &p.CurrentChapter, &p.TotalChapters, &p.Status,
		&p.Rating, &p.Notes, &p.IsFavorite, &p.StartedAt, &p.CompletedAt,
		&p.LastReadAt, &p.CreatedAt, &p.UpdatedAt, &p.SyncVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("select progress: %w", err)
	}
	return &p, nil
}

func (r *repository) ListByUser(ctx context.Context, userID string) ([]models.ProgressWithManga, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			r.id, r.user_id, r.manga_id, r.current_chapter, r.total_chapters, r.status,
			r.rating, r.notes, r.is_favorite, r.started_at, r.completed_at,
			r.last_read_at, r.created_at, r.updated_at, r.sync_version,
			m.id, m.title, m.author, m.artist, m.description, m.cover_url,
			m.status, m.type, m.genres, m.total_chapters, m.rating, m.year,
			m.created_at, m.updated_at
		FROM reading_progress r
		JOIN manga m ON r.manga_id = m.id
		WHERE r.user_id = ?
		ORDER BY r.last_read_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list progress: %w", err)
	}
	defer rows.Close()

	var result []models.ProgressWithManga
	for rows.Next() {
		var p models.ReadingProgress
		var m models.Manga
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.MangaID, &p.CurrentChapter, &p.TotalChapters, &p.Status,
			&p.Rating, &p.Notes, &p.IsFavorite, &p.StartedAt, &p.CompletedAt,
			&p.LastReadAt, &p.CreatedAt, &p.UpdatedAt, &p.SyncVersion,
			&m.ID, &m.Title, &m.Author, &m.Artist, &m.Description, &m.CoverURL,
			&m.Status, &m.Type, &m.GenresJSON, &m.TotalChapters, &m.Rating, &m.Year,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan : %w", err)
		}
		result = append(result, models.ProgressWithManga{
			ReadingProgress: p,
			Manga:           m,
		})
	}
	return result, nil
}
