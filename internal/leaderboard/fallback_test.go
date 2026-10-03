package leaderboard

import (
	"context"
	"testing"

	"mangahub/internal/testutil"
)

// Regression: with no recent activity, trending falls back to the best-rated
// manga, and that query referenced a nonexistent m.rating column (500).
func TestTrendingFallbackWithoutRecentActivity(t *testing.T) {
	db := testutil.NewDB(t)
	testutil.AddUser(t, db.DB, "u1")
	testutil.AddManga(t, db.DB, "m1", "Alpha")
	testutil.AddManga(t, db.DB, "m2", "Bravo")
	// An old rating: outside the 7-day window but still counted in average_rating
	testutil.MustExec(t, db.DB, `INSERT INTO manga_ratings (id, manga_id, user_id, rating, created_at)
		VALUES ('r1', 'm2', 'u1', 9, datetime('now', '-60 days'))`)

	svc := NewService(db.DB)
	ctx := context.Background()

	resp, err := svc.GetTrendingManga(ctx, 10, 0, 7)
	if err != nil {
		t.Fatalf("GetTrendingManga: %v", err)
	}
	entries := resp.Entries.([]MangaLeaderboardEntry)
	if len(entries) != 2 {
		t.Fatalf("got %d fallback entries, want 2", len(entries))
	}
	if entries[0].MangaID != "m2" || entries[0].AverageRating != 9 || entries[0].TotalRatings != 1 || entries[0].Rank != 1 {
		t.Errorf("first fallback entry = %+v, want m2 rated 9 at rank 1", entries[0])
	}

	// Past the end: an empty page, not the fallback list again
	resp, err = svc.GetTrendingManga(ctx, 10, 50, 7)
	if err != nil {
		t.Fatal(err)
	}
	if entries := resp.Entries.([]MangaLeaderboardEntry); len(entries) != 0 {
		t.Errorf("offset past the end returned %d entries", len(entries))
	}
}

func TestEmptyLeaderboardsReturnEmptyLists(t *testing.T) {
	db := testutil.NewDB(t)
	svc := NewService(db.DB)
	ctx := context.Background()

	top, err := svc.GetTopRatedManga(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if e := top.Entries.([]MangaLeaderboardEntry); e == nil || len(e) != 0 {
		t.Errorf("top rated entries = %#v, want empty non-nil slice (JSON [] not null)", e)
	}
	users, err := svc.GetMostActiveUsers(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if e := users.Entries.([]UserLeaderboardEntry); e == nil || len(e) != 0 {
		t.Errorf("user entries = %#v, want empty non-nil slice", e)
	}
}
