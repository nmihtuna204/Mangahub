// Package leaderboard - Leaderboard Service Tests
// Unit tests cho leaderboard service
package leaderboard

import (
	"context"
	"database/sql"
	"testing"
	"time"

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
			display_name TEXT DEFAULT '',
			avatar_url TEXT,
			is_active BOOLEAN DEFAULT 1,
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
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(manga_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS reading_progress (
			user_id TEXT NOT NULL,
			manga_id TEXT NOT NULL,
			status TEXT DEFAULT 'reading',
			current_chapter INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, manga_id)
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			manga_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			content TEXT NOT NULL,
			is_deleted BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS activities (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			manga_id TEXT,
			activity_type TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
	}

	// Insert test data
	// Users
	db.Exec(`INSERT INTO users (id, username, email, password_hash, display_name, is_active) VALUES ('user1', 'activeuser', 'test@test.com', 'hash123', 'Active User', 1)`)
	db.Exec(`INSERT INTO users (id, username, email, password_hash, display_name, is_active) VALUES ('user2', 'moderateuser', 'test2@test.com', 'hash456', 'Moderate User', 1)`)
	db.Exec(`INSERT INTO users (id, username, email, password_hash, display_name, is_active) VALUES ('user3', 'lowuser', 'test3@test.com', 'hash789', 'Low User', 1)`)

	// Manga
	db.Exec(`INSERT INTO manga (id, title, author) VALUES ('manga1', 'Top Rated Manga', 'Author A')`)
	db.Exec(`INSERT INTO manga (id, title, author) VALUES ('manga2', 'Medium Rated Manga', 'Author B')`)
	db.Exec(`INSERT INTO manga (id, title, author) VALUES ('manga3', 'Low Rated Manga', 'Author C')`)

	// Ratings for manga1 (high ratings)
	db.Exec(`INSERT INTO manga_ratings (id, manga_id, user_id, overall_rating) VALUES ('r1', 'manga1', 'user1', 10)`)
	db.Exec(`INSERT INTO manga_ratings (id, manga_id, user_id, overall_rating) VALUES ('r2', 'manga1', 'user2', 9)`)
	db.Exec(`INSERT INTO manga_ratings (id, manga_id, user_id, overall_rating) VALUES ('r3', 'manga1', 'user3', 9)`)

	// Ratings for manga2 (medium ratings)
	db.Exec(`INSERT INTO manga_ratings (id, manga_id, user_id, overall_rating) VALUES ('r4', 'manga2', 'user1', 7)`)
	db.Exec(`INSERT INTO manga_ratings (id, manga_id, user_id, overall_rating) VALUES ('r5', 'manga2', 'user2', 6)`)

	// Ratings for manga3 (low ratings)
	db.Exec(`INSERT INTO manga_ratings (id, manga_id, user_id, overall_rating) VALUES ('r6', 'manga3', 'user1', 4)`)

	// Reading progress (user1 most active)
	db.Exec(`INSERT INTO reading_progress (user_id, manga_id, status, current_chapter) VALUES ('user1', 'manga1', 'reading', 50)`)
	db.Exec(`INSERT INTO reading_progress (user_id, manga_id, status, current_chapter) VALUES ('user1', 'manga2', 'completed', 100)`)
	db.Exec(`INSERT INTO reading_progress (user_id, manga_id, status, current_chapter) VALUES ('user1', 'manga3', 'reading', 25)`)
	db.Exec(`INSERT INTO reading_progress (user_id, manga_id, status, current_chapter) VALUES ('user2', 'manga1', 'reading', 30)`)

	// Comments
	db.Exec(`INSERT INTO comments (id, manga_id, user_id, content, is_deleted) VALUES ('c1', 'manga1', 'user1', 'Comment 1', 0)`)
	db.Exec(`INSERT INTO comments (id, manga_id, user_id, content, is_deleted) VALUES ('c2', 'manga1', 'user1', 'Comment 2', 0)`)
	db.Exec(`INSERT INTO comments (id, manga_id, user_id, content, is_deleted) VALUES ('c3', 'manga2', 'user2', 'Comment 3', 0)`)

	// Recent activities for trending
	now := time.Now().Format("2006-01-02 15:04:05")
	db.Exec(`INSERT INTO activities (id, user_id, manga_id, activity_type, created_at) VALUES ('a1', 'user1', 'manga1', 'read', ?)`, now)
	db.Exec(`INSERT INTO activities (id, user_id, manga_id, activity_type, created_at) VALUES ('a2', 'user2', 'manga1', 'read', ?)`, now)
	db.Exec(`INSERT INTO activities (id, user_id, manga_id, activity_type, created_at) VALUES ('a3', 'user3', 'manga1', 'rate', ?)`, now)
	db.Exec(`INSERT INTO activities (id, user_id, manga_id, activity_type, created_at) VALUES ('a4', 'user1', 'manga2', 'read', ?)`, now)

	return db
}

func TestLeaderboardService_GetTopRatedManga(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	svc := NewService(db)
	ctx := context.Background()

	response, err := svc.GetTopRatedManga(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetTopRatedManga failed: %v", err)
	}

	// Type assert to get the actual entries
	entries, ok := response.Entries.([]MangaLeaderboardEntry)
	if !ok {
		t.Fatal("expected Entries to be []MangaLeaderboardEntry")
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 manga, got %d", len(entries))
	}

	// First should be manga1 (highest rating)
	if len(entries) > 0 {
		first := entries[0]
		if first.MangaID != "manga1" {
			t.Errorf("expected first manga to be 'manga1', got '%s'", first.MangaID)
		}
		if first.Rank != 1 {
			t.Errorf("expected rank 1, got %d", first.Rank)
		}
		// Average of 10, 9, 9 = 9.33
		if first.AverageRating < 9.0 {
			t.Errorf("expected average rating >= 9.0, got %f", first.AverageRating)
		}
	}
}

func TestLeaderboardService_GetMostActiveUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	svc := NewService(db)
	ctx := context.Background()

	response, err := svc.GetMostActiveUsers(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetMostActiveUsers failed: %v", err)
	}

	entries, ok := response.Entries.([]UserLeaderboardEntry)
	if !ok {
		t.Fatal("expected Entries to be []UserLeaderboardEntry")
	}

	if len(entries) < 1 {
		t.Error("expected at least 1 user in leaderboard")
	}

	// First should be user1 (most active)
	if len(entries) > 0 {
		first := entries[0]
		if first.UserID != "user1" {
			t.Errorf("expected first user to be 'user1', got '%s'", first.UserID)
		}
	}
}

func TestLeaderboardService_GetTrendingManga(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	svc := NewService(db)
	ctx := context.Background()

	// Get trending for last 7 days
	response, err := svc.GetTrendingManga(ctx, 10, 0, 7)
	if err != nil {
		t.Fatalf("GetTrendingManga failed: %v", err)
	}

	entries, ok := response.Entries.([]MangaLeaderboardEntry)
	if !ok {
		t.Fatal("expected Entries to be []MangaLeaderboardEntry")
	}

	// manga1 should be trending (most activities)
	if len(entries) > 0 {
		first := entries[0]
		if first.MangaID != "manga1" {
			t.Errorf("expected first trending manga to be 'manga1', got '%s'", first.MangaID)
		}
	}
}

func TestLeaderboardService_Pagination(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	svc := NewService(db)
	ctx := context.Background()

	// Test with limit=1
	response, err := svc.GetTopRatedManga(ctx, 1, 0)
	if err != nil {
		t.Fatalf("GetTopRatedManga failed: %v", err)
	}

	entries, ok := response.Entries.([]MangaLeaderboardEntry)
	if !ok {
		t.Fatal("expected Entries to be []MangaLeaderboardEntry")
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 manga with limit=1, got %d", len(entries))
	}

	// Test offset
	response, err = svc.GetTopRatedManga(ctx, 1, 1)
	if err != nil {
		t.Fatalf("GetTopRatedManga with offset failed: %v", err)
	}

	entries, ok = response.Entries.([]MangaLeaderboardEntry)
	if !ok {
		t.Fatal("expected Entries to be []MangaLeaderboardEntry")
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 manga with offset=1, got %d", len(entries))
	}

	// Should be manga2 (second highest rated)
	if len(entries) > 0 && entries[0].MangaID != "manga2" {
		t.Errorf("expected manga2 at offset 1, got '%s'", entries[0].MangaID)
	}
}
