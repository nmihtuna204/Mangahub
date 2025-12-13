// Package rating - Rating Repository
// Data access layer cho rating system
// Chức năng:
//   - CRUD operations for manga ratings
//   - Aggregate calculations (average, distribution)
//   - User rating lookup
package rating

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"mangahub/pkg/models"
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

	// GetAggregate calculates aggregate stats for a manga
	GetAggregate(ctx context.Context, mangaID string) (*models.RatingAggregate, error)

	// Delete removes a user's rating
	Delete(ctx context.Context, userID, mangaID string) error

	// GetTopRatedManga returns manga sorted by rating for leaderboards
	GetTopRatedManga(ctx context.Context, limit, offset int) ([]models.RatingAggregate, error)
}

type repository struct {
	db *sql.DB
}

// NewRepository creates a new rating repository
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

// CreateOrUpdate creates a new rating or updates existing one
// Uses UPSERT pattern with UNIQUE(manga_id, user_id) constraint
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
			(id, manga_id, user_id, overall_rating, story_rating, art_rating, 
			 character_rating, enjoyment_rating, review_text, is_spoiler, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ratingID, mangaID, userID, req.OverallRating, req.StoryRating, req.ArtRating,
			req.CharacterRating, req.EnjoymentRating, req.ReviewText, req.IsSpoiler, now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("insert rating: %w", err)
		}
	} else {
		// Update existing rating
		ratingID = existingID
		_, err = r.db.ExecContext(ctx, `
			UPDATE manga_ratings 
			SET overall_rating = ?, story_rating = ?, art_rating = ?, 
			    character_rating = ?, enjoyment_rating = ?, review_text = ?, 
			    is_spoiler = ?, updated_at = ?
			WHERE id = ?`,
			req.OverallRating, req.StoryRating, req.ArtRating,
			req.CharacterRating, req.EnjoymentRating, req.ReviewText,
			req.IsSpoiler, now, ratingID,
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
		SELECT id, manga_id, user_id, overall_rating, story_rating, art_rating,
		       character_rating, enjoyment_rating, review_text, is_spoiler,
		       helpful_count, created_at, updated_at
		FROM manga_ratings WHERE id = ?`, id,
	).Scan(
		&rating.ID, &rating.MangaID, &rating.UserID, &rating.OverallRating,
		&rating.StoryRating, &rating.ArtRating, &rating.CharacterRating,
		&rating.EnjoymentRating, &rating.ReviewText, &rating.IsSpoiler,
		&rating.HelpfulCount, &rating.CreatedAt, &rating.UpdatedAt,
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
		SELECT id, manga_id, user_id, overall_rating, story_rating, art_rating,
		       character_rating, enjoyment_rating, review_text, is_spoiler,
		       helpful_count, created_at, updated_at
		FROM manga_ratings WHERE user_id = ? AND manga_id = ?`, userID, mangaID,
	).Scan(
		&rating.ID, &rating.MangaID, &rating.UserID, &rating.OverallRating,
		&rating.StoryRating, &rating.ArtRating, &rating.CharacterRating,
		&rating.EnjoymentRating, &rating.ReviewText, &rating.IsSpoiler,
		&rating.HelpfulCount, &rating.CreatedAt, &rating.UpdatedAt,
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
		SELECT r.id, r.manga_id, r.user_id, r.overall_rating, r.story_rating, r.art_rating,
		       r.character_rating, r.enjoyment_rating, r.review_text, r.is_spoiler,
		       r.helpful_count, r.created_at, r.updated_at,
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
			&r.ID, &r.MangaID, &r.UserID, &r.OverallRating,
			&r.StoryRating, &r.ArtRating, &r.CharacterRating,
			&r.EnjoymentRating, &r.ReviewText, &r.IsSpoiler,
			&r.HelpfulCount, &r.CreatedAt, &r.UpdatedAt,
			&r.Username, &r.DisplayName,
		)
		if err != nil {
			return nil, fmt.Errorf("scan rating: %w", err)
		}
		ratings = append(ratings, r)
	}
	return ratings, nil
}

// GetAggregate calculates aggregate stats for a manga
func (r *repository) GetAggregate(ctx context.Context, mangaID string) (*models.RatingAggregate, error) {
	var agg models.RatingAggregate
	agg.MangaID = mangaID

	// Get average and count
	err := r.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(AVG(overall_rating), 0),
			COUNT(*),
			COALESCE(AVG(story_rating), 0),
			COALESCE(AVG(art_rating), 0),
			COALESCE(AVG(character_rating), 0),
			COALESCE(AVG(enjoyment_rating), 0)
		FROM manga_ratings WHERE manga_id = ?`, mangaID,
	).Scan(
		&agg.AverageRating, &agg.TotalRatings,
		&agg.AverageStory, &agg.AverageArt,
		&agg.AverageCharacter, &agg.AverageEnjoyment,
	)
	if err != nil {
		return nil, fmt.Errorf("get aggregate: %w", err)
	}

	// Calculate weighted rating using Bayesian average
	// Using minimum 10 votes and global mean of 6.0
	agg.WeightedRating = models.CalculateWeightedRating(agg.AverageRating, agg.TotalRatings, 10, 6.0)

	// Get rating distribution (count per score 1-10)
	rows, err := r.db.QueryContext(ctx, `
		SELECT CAST(overall_rating AS INTEGER) as score, COUNT(*) as cnt
		FROM manga_ratings 
		WHERE manga_id = ?
		GROUP BY CAST(overall_rating AS INTEGER)`, mangaID,
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
			agg.RatingDistribution[score-1] = count
		}
	}

	return &agg, nil
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

// GetTopRatedManga returns manga sorted by weighted rating for leaderboards
func (r *repository) GetTopRatedManga(ctx context.Context, limit, offset int) ([]models.RatingAggregate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT 
			manga_id,
			AVG(overall_rating) as avg_rating,
			COUNT(*) as total_ratings,
			AVG(story_rating) as avg_story,
			AVG(art_rating) as avg_art,
			AVG(character_rating) as avg_char,
			AVG(enjoyment_rating) as avg_enjoy
		FROM manga_ratings
		GROUP BY manga_id
		HAVING COUNT(*) >= 1
		ORDER BY avg_rating DESC, total_ratings DESC
		LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get top rated: %w", err)
	}
	defer rows.Close()

	var results []models.RatingAggregate
	for rows.Next() {
		var agg models.RatingAggregate
		err := rows.Scan(
			&agg.MangaID, &agg.AverageRating, &agg.TotalRatings,
			&agg.AverageStory, &agg.AverageArt, &agg.AverageCharacter, &agg.AverageEnjoyment,
		)
		if err != nil {
			return nil, fmt.Errorf("scan top rated: %w", err)
		}
		agg.WeightedRating = models.CalculateWeightedRating(agg.AverageRating, agg.TotalRatings, 10, 6.0)
		results = append(results, agg)
	}
	return results, nil
}
