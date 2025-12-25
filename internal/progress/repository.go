package progress

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"mangahub/pkg/models"

	"github.com/google/uuid"
)

type Repository interface {
	AddOrUpdate(ctx context.Context, userID string, req models.UpdateProgressRequest) (*models.ReadingProgress, error)
	ListByUser(ctx context.Context, userID string) ([]models.ProgressWithManga, error)
	Delete(ctx context.Context, userID, mangaID string) error
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
			(id, user_id, manga_id, current_chapter, status, is_favorite,
			 last_read_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, userID, req.MangaID, req.CurrentChapter, req.Status,
			req.IsFavorite, now, now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("insert progress: %w", err)
		}
		existingID = id
	} else {
		_, err = r.db.ExecContext(ctx, `
			UPDATE reading_progress
			SET current_chapter = ?, status = ?, is_favorite = ?, 
			    last_read_at = ?, updated_at = ?
			WHERE id = ?`,
			req.CurrentChapter, req.Status, req.IsFavorite, now, now, existingID,
		)
		if err != nil {
			return nil, fmt.Errorf("update progress: %w", err)
		}
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, manga_id, current_chapter, status,
		       is_favorite, started_at, completed_at,
		       last_read_at, created_at, updated_at
		FROM reading_progress WHERE id = ?`, existingID)

	var p models.ReadingProgress
	err = row.Scan(
		&p.ID, &p.UserID, &p.MangaID, &p.CurrentChapter, &p.Status,
		&p.IsFavorite, &p.StartedAt, &p.CompletedAt,
		&p.LastReadAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("select progress: %w", err)
	}
	return &p, nil
}

func (r *repository) ListByUser(ctx context.Context, userID string) ([]models.ProgressWithManga, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			r.id, r.user_id, r.manga_id, r.current_chapter, r.status,
			r.is_favorite, r.started_at, r.completed_at,
			r.last_read_at, r.created_at, r.updated_at,
			m.id, m.title, m.author, m.artist, m.description, m.cover_url,
			m.status, m.type, m.total_chapters, m.average_rating, m.rating_count, m.year,
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
			&p.ID, &p.UserID, &p.MangaID, &p.CurrentChapter, &p.Status,
			&p.IsFavorite, &p.StartedAt, &p.CompletedAt,
			&p.LastReadAt, &p.CreatedAt, &p.UpdatedAt,
			&m.ID, &m.Title, &m.Author, &m.Artist, &m.Description, &m.CoverURL,
			&m.Status, &m.Type, &m.TotalChapters, &m.AverageRating, &m.RatingCount, &m.Year,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		// Load genres for manga
		m.Genres = r.loadGenresForManga(ctx, m.ID)
		result = append(result, models.ProgressWithManga{
			ReadingProgress: p,
			Manga:           m,
		})
	}
	return result, nil
}

// loadGenresForManga loads all genres for a manga from the manga_genres junction table
func (r *repository) loadGenresForManga(ctx context.Context, mangaID string) []models.Genre {
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.slug, g.created_at
		FROM genres g
		INNER JOIN manga_genres mg ON g.id = mg.genre_id
		WHERE mg.manga_id = ?
		ORDER BY g.name`, mangaID)
	if err != nil {
		return []models.Genre{}
	}
	defer rows.Close()

	var genres []models.Genre
	for rows.Next() {
		var g models.Genre
		if err := rows.Scan(&g.ID, &g.Name, &g.Slug, &g.CreatedAt); err != nil {
			continue
		}
		genres = append(genres, g)
	}
	return genres
}

// Delete removes a manga from user's library
func (r *repository) Delete(ctx context.Context, userID, mangaID string) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM reading_progress WHERE user_id = ? AND manga_id = ?",
		userID, mangaID,
	)
	if err != nil {
		return fmt.Errorf("delete progress: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("manga not found in library")
	}
	return nil
}
