// Package rating - Rating Service Tests
// Unit tests cho rating service
package rating

import (
	"context"
	"database/sql"
	"testing"

	"mangahub/pkg/models"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	// Create required tables
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS manga (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			cover_url TEXT,
			author TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS manga_ratings (
			id TEXT PRIMARY KEY,
			manga_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			overall_rating REAL NOT NULL,
			story_rating REAL,
			art_rating REAL,
			character_rating REAL,
			enjoyment_rating REAL,
			review_text TEXT,
			is_spoiler BOOLEAN DEFAULT 0,
			helpful_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(manga_id, user_id),
			FOREIGN KEY (manga_id) REFERENCES manga(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
	}

	// Insert test data
	db.Exec(`INSERT INTO users (id, username, email, password_hash) VALUES ('user1', 'testuser', 'test@test.com', 'hash123')`)
	db.Exec(`INSERT INTO manga (id, title, author) VALUES ('manga1', 'Test Manga', 'Test Author')`)

	return db
}

func TestRatingRepository_CreateOrUpdate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Test creating a new rating
	req := models.CreateRatingRequest{
		OverallRating: 8,
		StoryRating:   7,
		ArtRating:     9,
		ReviewText:    "Really enjoyed reading this.",
	}

	rating, err := repo.CreateOrUpdate(ctx, "user1", "manga1", req)
	if err != nil {
		t.Fatalf("CreateOrUpdate failed: %v", err)
	}

	if rating.OverallRating != 8 {
		t.Errorf("expected overall_rating 8, got %f", rating.OverallRating)
	}
	if rating.StoryRating != 7 {
		t.Errorf("expected story_rating 7, got %f", rating.StoryRating)
	}

	// Test updating existing rating
	req.OverallRating = 9
	updatedRating, err := repo.CreateOrUpdate(ctx, "user1", "manga1", req)
	if err != nil {
		t.Fatalf("CreateOrUpdate (update) failed: %v", err)
	}

	if updatedRating.OverallRating != 9 {
		t.Errorf("expected updated overall_rating 9, got %f", updatedRating.OverallRating)
	}
}

func TestRatingRepository_GetByUserAndManga(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a rating first
	req := models.CreateRatingRequest{OverallRating: 7}
	_, err := repo.CreateOrUpdate(ctx, "user1", "manga1", req)
	if err != nil {
		t.Fatalf("CreateOrUpdate failed: %v", err)
	}

	// Test retrieving the rating
	rating, err := repo.GetByUserAndManga(ctx, "user1", "manga1")
	if err != nil {
		t.Fatalf("GetByUserAndManga failed: %v", err)
	}

	if rating == nil {
		t.Fatal("expected rating, got nil")
	}
	if rating.OverallRating != 7 {
		t.Errorf("expected overall_rating 7, got %f", rating.OverallRating)
	}

	// Test non-existent rating
	rating, err = repo.GetByUserAndManga(ctx, "user1", "nonexistent")
	if err != nil {
		t.Fatalf("GetByUserAndManga failed: %v", err)
	}
	if rating != nil {
		t.Error("expected nil for non-existent rating")
	}
}

func TestRatingRepository_GetAggregate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Add test user
	db.Exec(`INSERT INTO users (id, username, email, password_hash) VALUES ('user2', 'testuser2', 'test2@test.com', 'hash456')`)

	// Create multiple ratings
	repo.CreateOrUpdate(ctx, "user1", "manga1", models.CreateRatingRequest{OverallRating: 8})
	repo.CreateOrUpdate(ctx, "user2", "manga1", models.CreateRatingRequest{OverallRating: 10})

	// Get aggregate
	agg, err := repo.GetAggregate(ctx, "manga1")
	if err != nil {
		t.Fatalf("GetAggregate failed: %v", err)
	}

	if agg == nil {
		t.Fatal("expected aggregate, got nil")
	}
	if agg.TotalRatings != 2 {
		t.Errorf("expected 2 total ratings, got %d", agg.TotalRatings)
	}
	if agg.AverageRating != 9.0 {
		t.Errorf("expected average 9.0, got %f", agg.AverageRating)
	}
}

func TestRatingRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a rating first
	req := models.CreateRatingRequest{OverallRating: 7}
	_, err := repo.CreateOrUpdate(ctx, "user1", "manga1", req)
	if err != nil {
		t.Fatalf("CreateOrUpdate failed: %v", err)
	}

	// Delete the rating
	err = repo.Delete(ctx, "user1", "manga1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	rating, _ := repo.GetByUserAndManga(ctx, "user1", "manga1")
	if rating != nil {
		t.Error("expected rating to be deleted")
	}
}

func TestRatingService_Rate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	// Test valid rating
	req := models.CreateRatingRequest{OverallRating: 8}
	rating, err := svc.Rate(ctx, "user1", "manga1", req)
	if err != nil {
		t.Fatalf("Rate failed: %v", err)
	}
	if rating.OverallRating != 8 {
		t.Errorf("expected overall_rating 8, got %f", rating.OverallRating)
	}

	// Test invalid rating (out of range)
	req.OverallRating = 15
	_, err = svc.Rate(ctx, "user1", "manga1", req)
	if err == nil {
		t.Error("expected error for invalid rating")
	}
}
