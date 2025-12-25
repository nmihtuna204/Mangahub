package manga

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"mangahub/pkg/models"
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
		conditions = append(conditions, "(title LIKE ? OR author LIKE ? OR description LIKE ?)")
		q := "%" + req.Query + "%"
		args = append(args, q, q, q)
	}
	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}
	// Note: Genre filtering should use JOIN with manga_genres table
	if len(req.Genres) > 0 {
		genrePlaceholders := strings.Repeat("?,", len(req.Genres)-1) + "?"
		conditions = append(conditions, fmt.Sprintf("id IN (SELECT manga_id FROM manga_genres mg JOIN genres g ON mg.genre_id = g.id WHERE g.slug IN (%s))", genrePlaceholders))
		for _, genre := range req.Genres {
			args = append(args, genre)
		}
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
		orderBy = "average_rating DESC"
	case "year":
		orderBy = "year DESC"
	}

	listSQL := fmt.Sprintf(`
		SELECT id, title, author, artist, description, cover_url, status, type,
		       total_chapters, average_rating, rating_count, year, created_at, updated_at
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
			&m.Status, &m.Type, &m.TotalChapters, &m.AverageRating, &m.RatingCount,
			&m.Year, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan manga: %w", err)
		}
		// Load genres for each manga
		m.Genres = r.loadGenresForManga(ctx, m.ID)
		result = append(result, m)
	}

	return result, total, nil
}

func (r *repository) GetByID(ctx context.Context, id string) (*models.Manga, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, title, author, artist, description, cover_url, status, type,
		       genres, total_chapters, rating, year, created_at, updated_at
		FROM matotal_chapters, average_rating, rating_count, year, created_at, updated_at
		FROM manga
		WHERE id = ?`, id)

	var m models.Manga
	if err := row.Scan(
		&m.ID, &m.Title, &m.Author, &m.Artist, &m.Description, &m.CoverURL,
		&m.Status, &m.Type, &m.TotalChapters, &m.AverageRating, &m.RatingCount,
		&m.Year, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, models.NewAppError(models.ErrCodeNotFound, "manga not found", 404, models.ErrMangaNotFound)
		}
		return nil, fmt.Errorf("get manga: %w", err)
	}
	// Load genres via join
	m.Genres = r.loadGenresForManga(ctx, m.ID)
	return &m, nil
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
