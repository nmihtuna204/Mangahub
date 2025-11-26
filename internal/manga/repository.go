package manga

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yourusername/mangahub/pkg/models"
)

type Repository interface {
	List(ctx context.Context, req models.MangaSearchRequest) ([]models.Manga, int, error)
	GetByID(ctx context.Context, id string) (*models.Manga, error)
}

type repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

func (r *repository) List(ctx context.Context, req models.MangaSearchRequest) ([]models.Manga, int, error) {
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	conditions := []string{"1=1"}
	args := []interface{}{}

	if req.Query != "" {
		conditions = append(conditions, "(title LIKE ? OR author LIKE ?)")
		q := "%" + req.Query + "%"
		args = append(args, q, q)
	}
	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}

	where := strings.Join(conditions, " AND ")

	countSQL := "SELECT COUNT(*) FROM manga WHERE " + where
	var total int
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count manga: %w", err)
	}

	orderBy := "title ASC"
	switch req.SortBy {
	case "rating":
		orderBy = "rating DESC"
	case "year":
		orderBy = "year DESC"
	}

	listSQL := fmt.Sprintf(`
		SELECT id, title, author, artist, description, cover_url, status, type,
		       genres, total_chapters, rating, year, created_at, updated_at
		FROM manga
		WHERE %s
		ORDER BY %s
		LIMIT ? OFFSET ?`, where, orderBy)

	argsWithPaging := append(args, req.Limit, req.Offset)

	rows, err := r.db.QueryContext(ctx, listSQL, argsWithPaging...)
	if err != nil {
		return nil, 0, fmt.Errorf("query manga: %w", err)
	}
	defer rows.Close()

	var result []models.Manga
	for rows.Next() {
		var m models.Manga
		if err := rows.Scan(
			&m.ID, &m.Title, &m.Author, &m.Artist, &m.Description, &m.CoverURL,
			&m.Status, &m.Type, &m.GenresJSON, &m.TotalChapters, &m.Rating,
			&m.Year, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan manga: %w", err)
		}
		if m.GenresJSON != "" {
			_ = json.Unmarshal([]byte(m.GenresJSON), &m.Genres)
		}
		result = append(result, m)
	}

	return result, total, nil
}

func (r *repository) GetByID(ctx context.Context, id string) (*models.Manga, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, title, author, artist, description, cover_url, status, type,
		       genres, total_chapters, rating, year, created_at, updated_at
		FROM manga
		WHERE id = ?`, id)

	var m models.Manga
	if err := row.Scan(
		&m.ID, &m.Title, &m.Author, &m.Artist, &m.Description, &m.CoverURL,
		&m.Status, &m.Type, &m.GenresJSON, &m.TotalChapters, &m.Rating,
		&m.Year, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, models.NewAppError(models.ErrCodeNotFound, "manga not found", 404, models.ErrMangaNotFound)
		}
		return nil, fmt.Errorf("get manga: %w", err)
	}
	if m.GenresJSON != "" {
		_ = json.Unmarshal([]byte(m.GenresJSON), &m.Genres)
	}
	return &m, nil
}
