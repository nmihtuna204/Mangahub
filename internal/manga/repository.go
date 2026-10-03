package manga

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"

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

	from := "manga m"
	conditions := []string{"1=1"}
	args := []interface{}{}

	// Text search uses the manga_fts full-text index: whole words, prefix
	// matching, ranked by relevance
	searching := false
	if req.Query != "" {
		match := ftsQuery(req.Query)
		if match == "" {
			return []models.Manga{}, 0, nil // nothing searchable (only punctuation)
		}
		from = "manga m JOIN manga_fts ON manga_fts.rowid = m.rowid"
		conditions = append(conditions, "manga_fts MATCH ?")
		args = append(args, match)
		searching = true
	}
	if req.Status != "" {
		conditions = append(conditions, "m.status = ?")
		args = append(args, req.Status)
	}
	// Genres match by slug ("slice-of-life") or display name ("Slice of Life"), case-insensitively
	if len(req.Genres) > 0 {
		genrePlaceholders := strings.Repeat("?,", len(req.Genres)-1) + "?"
		conditions = append(conditions, fmt.Sprintf(
			"m.id IN (SELECT manga_id FROM manga_genres mg JOIN genres g ON mg.genre_id = g.id WHERE LOWER(g.slug) IN (%[1]s) OR LOWER(g.name) IN (%[1]s))",
			genrePlaceholders))
		lowered := make([]interface{}, len(req.Genres))
		for i, genre := range req.Genres {
			lowered[i] = strings.ToLower(strings.TrimSpace(genre))
		}
		args = append(args, lowered...)
		args = append(args, lowered...)
	}
	if req.Type != "" {
		conditions = append(conditions, "m.type = ?")
		args = append(args, req.Type)
	}

	where := strings.Join(conditions, " AND ")

	countSQL := "SELECT COUNT(*) FROM " + from + " WHERE " + where
	var total int
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count manga: %w", err)
	}

	orderBy := orderClause(req, searching)

	listSQL := fmt.Sprintf(`
		SELECT m.id, m.title, COALESCE(m.author, ''), COALESCE(m.artist, ''), COALESCE(m.description, ''),
		       COALESCE(m.cover_url, ''), m.status, m.type,
		       m.total_chapters, m.average_rating, m.rating_count, COALESCE(m.year, 0), m.created_at, m.updated_at
		FROM %s
		WHERE %s
		ORDER BY %s
		LIMIT ? OFFSET ?`, from, where, orderBy)

	argsWithPaging := append(args, req.Limit, req.Offset)

	rows, err := r.db.QueryContext(ctx, listSQL, argsWithPaging...)
	if err != nil {
		return nil, 0, fmt.Errorf("query manga: %w", err)
	}
	defer rows.Close()

	result := []models.Manga{}
	for rows.Next() {
		var m models.Manga
		if err := rows.Scan(
			&m.ID, &m.Title, &m.Author, &m.Artist, &m.Description, &m.CoverURL,
			&m.Status, &m.Type, &m.TotalChapters, &m.AverageRating, &m.RatingCount,
			&m.Year, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan manga: %w", err)
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate manga: %w", err)
	}
	rows.Close() // release the connection before the genre query

	if err := AttachGenres(ctx, r.db, result); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

// ftsQuery turns free text into a safe FTS5 query: every word becomes a
// quoted prefix term ("one piece" -> "one"* "piece"*), and all terms must
// match. Quoting means user input can never inject FTS5 syntax (AND, NEAR,
// column filters, unbalanced quotes). Returns "" when there are no words.
func ftsQuery(q string) string {
	words := strings.FieldsFunc(q, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	terms := make([]string, 0, len(words))
	for _, w := range words {
		terms = append(terms, `"`+w+`"*`)
	}
	return strings.Join(terms, " ")
}

// orderClause builds the ORDER BY from whitelisted values only, keeping user
// input out of the SQL text. Text searches default to relevance.
func orderClause(req models.MangaSearchRequest, searching bool) string {
	sortBy := req.SortBy
	if sortBy == "" && searching {
		sortBy = "relevance"
	}

	column, dir := "m.title", "ASC"
	switch sortBy {
	case "relevance":
		if searching {
			// bm25: lower is better. Column weights: id, title, author, description
			column, dir = "bm25(manga_fts, 0.0, 10.0, 5.0, 1.0)", "ASC"
		}
	case "rating":
		column, dir = "m.average_rating", "DESC"
	case "year":
		column, dir = "m.year", "DESC"
	case "chapters":
		column, dir = "m.total_chapters", "DESC"
	}
	switch strings.ToLower(req.Order) {
	case "asc":
		dir = "ASC"
	case "desc":
		dir = "DESC"
	}
	return column + " " + dir + ", m.title ASC"
}

func (r *repository) GetByID(ctx context.Context, id string) (*models.Manga, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, title, COALESCE(author, ''), COALESCE(artist, ''), COALESCE(description, ''),
		       COALESCE(cover_url, ''), status, type,
		       total_chapters, average_rating, rating_count, COALESCE(year, 0), created_at, updated_at
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
	one := []models.Manga{m}
	if err := AttachGenres(ctx, r.db, one); err != nil {
		return nil, err
	}
	return &one[0], nil
}

// AttachGenres fills in Genres for every manga in the slice using a single
// query. Call it only after the rows that produced the slice are closed:
// running it inside a rows loop holds two pooled connections per request,
// which deadlocks the pool under concurrent load.
func AttachGenres(ctx context.Context, db *sql.DB, list []models.Manga) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]interface{}, len(list))
	for i, m := range list {
		ids[i] = m.ID
	}

	rows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT mg.manga_id, g.id, g.name, g.slug, g.created_at
		FROM genres g
		INNER JOIN manga_genres mg ON g.id = mg.genre_id
		WHERE mg.manga_id IN (%s)
		ORDER BY g.name`, strings.Repeat("?,", len(ids)-1)+"?"), ids...)
	if err != nil {
		return fmt.Errorf("load genres: %w", err)
	}
	defer rows.Close()

	byManga := make(map[string][]models.Genre, len(list))
	for rows.Next() {
		var mangaID string
		var g models.Genre
		if err := rows.Scan(&mangaID, &g.ID, &g.Name, &g.Slug, &g.CreatedAt); err != nil {
			return fmt.Errorf("scan genre: %w", err)
		}
		byManga[mangaID] = append(byManga[mangaID], g)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate genres: %w", err)
	}

	for i := range list {
		list[i].Genres = byManga[list[i].ID]
	}
	return nil
}
