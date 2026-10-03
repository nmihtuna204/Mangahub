// Package leaderboard - Leaderboard Service Tests
// Unit tests cho leaderboard service
package leaderboard

import (
	"context"
	"database/sql"
	"testing"

	"mangahub/internal/testutil"
)

// setupTestDB returns a database with the production schema and test data.
// It used ":memory:", where every pooled connection gets its own empty
// database, so a second connection would have seen no tables;
// testutil.NewDB is file-backed.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB := testutil.NewDB(t).DB

	mustExec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := sqlDB.Exec(query, args...); err != nil {
			t.Fatalf("failed to exec %q: %v", query, err)
		}
	}

	// Users
	mustExec(`INSERT INTO users (id, username, email, password_hash, display_name, is_active) VALUES ('user1', 'activeuser', 'test@test.com', 'hash123', 'Active User', 1)`)
	mustExec(`INSERT INTO users (id, username, email, password_hash, display_name, is_active) VALUES ('user2', 'moderateuser', 'test2@test.com', 'hash456', 'Moderate User', 1)`)
	mustExec(`INSERT INTO users (id, username, email, password_hash, display_name, is_active) VALUES ('user3', 'lowuser', 'test3@test.com', 'hash789', 'Low User', 1)`)

	// Manga
	mustExec(`INSERT INTO manga (id, title, author) VALUES ('manga1', 'Top Rated Manga', 'Author A')`)
	mustExec(`INSERT INTO manga (id, title, author) VALUES ('manga2', 'Medium Rated Manga', 'Author B')`)
	mustExec(`INSERT INTO manga (id, title, author) VALUES ('manga3', 'Low Rated Manga', 'Author C')`)

	// Ratings for manga1 (high ratings)
	mustExec(`INSERT INTO manga_ratings (id, manga_id, user_id, rating) VALUES ('r1', 'manga1', 'user1', 10)`)
	mustExec(`INSERT INTO manga_ratings (id, manga_id, user_id, rating) VALUES ('r2', 'manga1', 'user2', 9)`)
	mustExec(`INSERT INTO manga_ratings (id, manga_id, user_id, rating) VALUES ('r3', 'manga1', 'user3', 9)`)

	// Ratings for manga2 (medium ratings)
	mustExec(`INSERT INTO manga_ratings (id, manga_id, user_id, rating) VALUES ('r4', 'manga2', 'user1', 7)`)
	mustExec(`INSERT INTO manga_ratings (id, manga_id, user_id, rating) VALUES ('r5', 'manga2', 'user2', 6)`)

	// Ratings for manga3 (low ratings)
	mustExec(`INSERT INTO manga_ratings (id, manga_id, user_id, rating) VALUES ('r6', 'manga3', 'user1', 4)`)

	// Reading progress (user1 most active)
	mustExec(`INSERT INTO reading_progress (id, user_id, manga_id, status, current_chapter) VALUES ('p1', 'user1', 'manga1', 'reading', 50)`)
	mustExec(`INSERT INTO reading_progress (id, user_id, manga_id, status, current_chapter) VALUES ('p2', 'user1', 'manga2', 'completed', 100)`)
	mustExec(`INSERT INTO reading_progress (id, user_id, manga_id, status, current_chapter) VALUES ('p3', 'user1', 'manga3', 'reading', 25)`)
	mustExec(`INSERT INTO reading_progress (id, user_id, manga_id, status, current_chapter) VALUES ('p4', 'user2', 'manga1', 'reading', 30)`)

	// Comments
	mustExec(`INSERT INTO comments (id, manga_id, user_id, content, is_deleted) VALUES ('c1', 'manga1', 'user1', 'Comment 1', 0)`)
	mustExec(`INSERT INTO comments (id, manga_id, user_id, content, is_deleted) VALUES ('c2', 'manga1', 'user1', 'Comment 2', 0)`)
	mustExec(`INSERT INTO comments (id, manga_id, user_id, content, is_deleted) VALUES ('c3', 'manga2', 'user2', 'Comment 3', 0)`)

	return sqlDB
}

func TestLeaderboardService_GetTopRatedManga(t *testing.T) {
	db := setupTestDB(t)

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

	// manga1 should be trending (most activities). An empty list used to pass
	// this test without checking anything.
	if len(entries) == 0 {
		t.Fatal("no trending manga, want manga1 first (it has this week's ratings, progress and comments)")
	}
	if entries[0].MangaID != "manga1" {
		t.Errorf("expected first trending manga to be 'manga1', got '%s'", entries[0].MangaID)
	}
}

func TestLeaderboardService_Pagination(t *testing.T) {
	db := setupTestDB(t)

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
