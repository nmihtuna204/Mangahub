package progress

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"mangahub/internal/manga"
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

	// Verify the manga exists so a bad ID returns 404 instead of a raw
	// foreign-key constraint failure (500).
	var exists int
	if err := r.db.QueryRowContext(ctx, "SELECT 1 FROM manga WHERE id = ?", req.MangaID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return nil, models.NewAppError(models.ErrCodeNotFound, "manga not found", 404, models.ErrMangaNotFound)
		}
		return nil, fmt.Errorf("check manga: %w", err)
	}

	// Update first; if the entry doesn't exist yet, insert it. ON CONFLICT
	// covers a concurrent insert of the same (user, manga), after which we
	// apply this request as an update instead.
	updated, err := r.update(ctx, userID, req, now)
	if err != nil {
		return nil, err
	}
	if !updated {
		inserted, err := r.insert(ctx, userID, req, now)
		if err != nil {
			return nil, err
		}
		if !inserted {
			if _, err := r.update(ctx, userID, req, now); err != nil {
				return nil, err
			}
		}
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, manga_id, current_chapter, status,
		       is_favorite, started_at, completed_at,
		       last_read_at, created_at, updated_at
		FROM reading_progress WHERE user_id = ? AND manga_id = ?`, userID, req.MangaID)

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

// update applies only the fields present in req to an existing entry.
// SQLite evaluates every SET expression against the row's old values, so
// COALESCE(?, col) means "the new value if given, else the stored one".
func (r *repository) update(ctx context.Context, userID string, req models.UpdateProgressRequest, now time.Time) (bool, error) {
	chapter, status, fav := nullable(req.CurrentChapter), nullable(req.Status), nullable(req.IsFavorite)
	res, err := r.db.ExecContext(ctx, `
		UPDATE reading_progress SET
			current_chapter = COALESCE(?, current_chapter),
			status          = COALESCE(?, status),
			is_favorite     = COALESCE(?, is_favorite),
			started_at      = CASE
			                    WHEN started_at IS NULL
			                     AND (COALESCE(?, status) IN ('reading', 'completed') OR COALESCE(?, current_chapter) > 0)
			                    THEN ? ELSE started_at END,
			completed_at    = CASE WHEN COALESCE(?, status) = 'completed' THEN COALESCE(completed_at, ?) ELSE NULL END,
			last_read_at    = CASE WHEN ? IS NULL THEN last_read_at ELSE ? END,
			updated_at      = ?
		WHERE user_id = ? AND manga_id = ?`,
		chapter, status, fav,
		status, chapter, now,
		status, now,
		chapter, now,
		now,
		userID, req.MangaID,
	)
	if err != nil {
		return false, fmt.Errorf("update progress: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("update progress: %w", err)
	}
	return n > 0, nil
}

// insert creates a new library entry, defaulting omitted fields.
// It reports false if an entry for (user, manga) already exists.
func (r *repository) insert(ctx context.Context, userID string, req models.UpdateProgressRequest, now time.Time) (bool, error) {
	chapter := 0
	if req.CurrentChapter != nil {
		chapter = *req.CurrentChapter
	}
	status := "plan_to_read"
	if chapter > 0 {
		status = "reading"
	}
	if req.Status != nil {
		status = *req.Status
	}
	fav := req.IsFavorite != nil && *req.IsFavorite

	var startedAt, completedAt interface{}
	if status == "reading" || status == "completed" || chapter > 0 {
		startedAt = now
	}
	if status == "completed" {
		completedAt = now
	}

	res, err := r.db.ExecContext(ctx, `
		INSERT INTO reading_progress
		(id, user_id, manga_id, current_chapter, status, is_favorite,
		 started_at, completed_at, last_read_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, manga_id) DO NOTHING`,
		uuid.New().String(), userID, req.MangaID, chapter, status, fav,
		startedAt, completedAt, now, now, now,
	)
	if err != nil {
		return false, fmt.Errorf("insert progress: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("insert progress: %w", err)
	}
	return n > 0, nil
}

// nullable turns an optional field into a SQL argument (NULL when absent).
func nullable[T any](v *T) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func (r *repository) ListByUser(ctx context.Context, userID string) ([]models.ProgressWithManga, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			r.id, r.user_id, r.manga_id, r.current_chapter, r.status,
			r.is_favorite, r.started_at, r.completed_at,
			r.last_read_at, r.created_at, r.updated_at,
			m.id, m.title, COALESCE(m.author, ''), COALESCE(m.artist, ''), COALESCE(m.description, ''),
			COALESCE(m.cover_url, ''),
			m.status, m.type, m.total_chapters, m.average_rating, m.rating_count, COALESCE(m.year, 0),
			m.created_at, m.updated_at
		FROM reading_progress r
		JOIN manga m ON r.manga_id = m.id
		WHERE r.user_id = ?
		ORDER BY r.last_read_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list progress: %w", err)
	}
	defer rows.Close()

	result := []models.ProgressWithManga{}
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
		result = append(result, models.ProgressWithManga{
			ReadingProgress: p,
			Manga:           m,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate progress: %w", err)
	}
	rows.Close() // release the connection before the genre query

	mangaList := make([]models.Manga, len(result))
	for i := range result {
		mangaList[i] = result[i].Manga
	}
	if err := manga.AttachGenres(ctx, r.db, mangaList); err != nil {
		return nil, err
	}
	for i := range result {
		result[i].Manga.Genres = mangaList[i].Genres
	}
	return result, nil
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
