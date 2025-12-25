// Package rating - Rating Repository
// Data access layer cho rating system
// Chức năng:
//   - CRUD operations for manga ratings (simplified single rating 1-10)
//   - Aggregate calculations (average, distribution) from manga table (auto-calculated by triggers)
//   - User rating lookup
package rating

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"mangahub/pkg/models"

	"github.com/google/uuid"
)

// Repository defines data access operations for ratings
type Repository interface {
	// Create creates or updates a user's rating for a manga
	CreateOrUpdate(ctx context.Context, userID, mangaID string, req models.CreateRatingRequest) (*models.MangaRating, error)

	// GetByID retrieves a rating by ID
	GetByID(ctx context.Context, id string) (*models.MangaRating, error)

	// GetByUserAndManga retrieves a user's rating for a specific manga
	GetByUserAndManga(ctx context.Context, userID, mangaID string) (*models.MangaRating, error)

	// GetByManga retrieves all ratings for a manga with pagination
	GetByManga(ctx context.Context, mangaID string, limit, offset int) ([]models.RatingWithUser, error)

	// GetSummary gets rating summary for a manga from manga table (auto-calculated)
	GetSummary(ctx context.Context, mangaID string) (*models.RatingSummary, error)

	// Delete removes a user's rating
	Delete(ctx context.Context, userID, mangaID string) error
}

type repository struct {
	db *sql.DB
}

// NewRepository creates a new rating repository
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

// CreateOrUpdate creates a new rating or updates existing one
// CreateOrUpdate creates or updates a user's rating (simplified single rating 1-10)
func (r *repository) CreateOrUpdate(ctx context.Context, userID, mangaID string, req models.CreateRatingRequest) (*models.MangaRating, error) {
	now := time.Now()

	// Check if rating exists
	var existingID string
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM manga_ratings WHERE user_id = ? AND manga_id = ?",
		userID, mangaID,
	).Scan(&existingID)

	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("check existing rating: %w", err)
	}

	var ratingID string
	if err == sql.ErrNoRows {
		// Insert new rating
		ratingID = uuid.New().String()
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO manga_ratings 
			(id, manga_id, user_id, rating, review, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			ratingID, mangaID, userID, req.Rating, req.ReviewText, now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("insert rating: %w", err)
		}
	} else {
		// Update existing rating
		ratingID = existingID
		_, err = r.db.ExecContext(ctx, `
			UPDATE manga_ratings 
			SET rating = ?, review = ?, updated_at = ?
			WHERE id = ?`,
			req.Rating, req.ReviewText, now, ratingID,
		)
		if err != nil {
			return nil, fmt.Errorf("update rating: %w", err)
		}
	}

	return r.GetByID(ctx, ratingID)
}

// GetByID retrieves a rating by its ID
func (r *repository) GetByID(ctx context.Context, id string) (*models.MangaRating, error) {
	var rating models.MangaRating
	err := r.db.QueryRowContext(ctx, `
		SELECT id, manga_id, user_id, rating, review, created_at, updated_at
		FROM manga_ratings WHERE id = ?`, id,
	).Scan(
		&rating.ID, &rating.MangaID, &rating.UserID, &rating.Rating,
		&rating.ReviewText, &rating.CreatedAt, &rating.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get rating by id: %w", err)
	}
	return &rating, nil
}

// GetByUserAndManga retrieves a user's rating for a specific manga
func (r *repository) GetByUserAndManga(ctx context.Context, userID, mangaID string) (*models.MangaRating, error) {
	var rating models.MangaRating
	err := r.db.QueryRowContext(ctx, `
		SELECT id, manga_id, user_id, rating, review, created_at, updated_at
		FROM manga_ratings WHERE user_id = ? AND manga_id = ?`, userID, mangaID,
	).Scan(
		&rating.ID, &rating.MangaID, &rating.UserID, &rating.Rating,
		&rating.ReviewText, &rating.CreatedAt, &rating.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get rating by user and manga: %w", err)
	}
	return &rating, nil
}

// GetByManga retrieves all ratings for a manga with user info
func (r *repository) GetByManga(ctx context.Context, mangaID string, limit, offset int) ([]models.RatingWithUser, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, r.manga_id, r.user_id, r.rating, r.review,
		       r.created_at, r.updated_at,
		       u.username, u.display_name
		FROM manga_ratings r
		JOIN users u ON r.user_id = u.id
		WHERE r.manga_id = ?
		ORDER BY r.created_at DESC
		LIMIT ? OFFSET ?`, mangaID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get ratings by manga: %w", err)
	}
	defer rows.Close()

	var ratings []models.RatingWithUser
	for rows.Next() {
		var r models.RatingWithUser
		err := rows.Scan(
			&r.ID, &r.MangaID, &r.UserID, &r.Rating, &r.ReviewText,
			&r.CreatedAt, &r.UpdatedAt,
			&r.Username, &r.DisplayName,
		)
		if err != nil {
			return nil, fmt.Errorf("scan rating: %w", err)
		}
		ratings = append(ratings, r)
	}
	return ratings, nil
}

// GetSummary gets rating summary from manga table (auto-calculated by triggers)
func (r *repository) GetSummary(ctx context.Context, mangaID string) (*models.RatingSummary, error) {
	var summary models.RatingSummary
	var ratingCount int
	summary.MangaID = mangaID

	// Get average_rating and rating_count from manga table (auto-calculated by triggers)
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(average_rating, 0), COALESCE(rating_count, 0)
		FROM manga WHERE id = ?`, mangaID,
	).Scan(&summary.AverageRating, &ratingCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("manga not found")
		}
		return nil, fmt.Errorf("get rating summary: %w", err)
	}

	summary.RatingCount = ratingCount

	// Get rating distribution (count per score 1-10)
	rows, err := r.db.QueryContext(ctx, `
		SELECT rating, COUNT(*) as cnt
		FROM manga_ratings 
		WHERE manga_id = ?
		GROUP BY rating
		ORDER BY rating`, mangaID,
	)
	if err != nil {
		return nil, fmt.Errorf("get distribution: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var score, count int
		if err := rows.Scan(&score, &count); err != nil {
			return nil, fmt.Errorf("scan distribution: %w", err)
		}
		if score >= 1 && score <= 10 {
			summary.RatingDistribution[score-1] = count
		}
	}

	return &summary, nil
}

// Delete removes a user's rating for a manga
func (r *repository) Delete(ctx context.Context, userID, mangaID string) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM manga_ratings WHERE user_id = ? AND manga_id = ?",
		userID, mangaID,
	)
	if err != nil {
		return fmt.Errorf("delete rating: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("rating not found")
	}
	return nil
}

// GetTopRatedManga returns manga sorted by rating for leaderboards
// This method is not part of the interface but can be added if needed for leaderboard features
// Currently not used as leaderboards use manga.average_rating directly
func (r *repository) GetTopRatedManga(ctx context.Context, limit, offset int) ([]models.Manga, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.id, m.title, m.author, m.artist, m.description, m.cover_url, 
		       m.status, m.type, m.total_chapters, m.average_rating, m.rating_count, 
		       m.year, m.created_at, m.updated_at
		FROM manga m
		WHERE m.rating_count > 0
		ORDER BY m.average_rating DESC, m.rating_count DESC
		LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get top rated: %w", err)
	}
	defer rows.Close()

	var results []models.Manga
	for rows.Next() {
		var manga models.Manga
		err := rows.Scan(
			&manga.ID, &manga.Title, &manga.Author, &manga.Artist, &manga.Description,
			&manga.CoverURL, &manga.Status, &manga.Type, &manga.TotalChapters,
			&manga.AverageRating, &manga.RatingCount, &manga.Year,
			&manga.CreatedAt, &manga.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan top rated: %w", err)
		}
		results = append(results, manga)
	}
	return results, nil
}
