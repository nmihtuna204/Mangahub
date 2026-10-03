package rating

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"mangahub/internal/testutil"
	"mangahub/pkg/models"
)

func setup(t *testing.T) (Service, *sql.DB) {
	t.Helper()
	db := testutil.NewDB(t)
	testutil.AddUser(t, db.DB, "u1")
	testutil.AddUser(t, db.DB, "u2")
	testutil.AddManga(t, db.DB, "m1", "Alpha")
	testutil.AddManga(t, db.DB, "m2", "Bravo")
	return NewService(NewRepository(db.DB)), db.DB
}

func wantStatus(t *testing.T, err error, code int) {
	t.Helper()
	var appErr *models.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode != code {
		t.Errorf("got %v, want %d", err, code)
	}
}

// Regression: the handler reported every failure as 500.
func TestRate_ErrorCodes(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()

	_, err := svc.Rate(ctx, "u1", "m1", models.CreateRatingRequest{Rating: 11})
	wantStatus(t, err, 400)
	_, err = svc.Rate(ctx, "u1", "m1", models.CreateRatingRequest{Rating: 0})
	wantStatus(t, err, 400)
	_, err = svc.Rate(ctx, "u1", "missing", models.CreateRatingRequest{Rating: 7})
	wantStatus(t, err, 404)
	_, err = svc.GetMangaRatings(ctx, "missing", 20, 0)
	wantStatus(t, err, 404)
	wantStatus(t, svc.DeleteRating(ctx, "u1", "m1"), 404)
}

func TestRate_CreateUpdateSummary(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()

	r, err := svc.Rate(ctx, "u1", "m1", models.CreateRatingRequest{Rating: 8, ReviewText: "great", IsSpoiler: true})
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsSpoiler || r.ReviewText != "great" {
		t.Errorf("rating = %+v, want is_spoiler and review saved (is_spoiler used to be dropped)", r)
	}

	r2, err := svc.Rate(ctx, "u1", "m1", models.CreateRatingRequest{Rating: 6})
	if err != nil {
		t.Fatal(err)
	}
	if r2.ID != r.ID || r2.Rating != 6 || r2.IsSpoiler {
		t.Errorf("re-rating = %+v, want same row updated to 6, spoiler cleared", r2)
	}

	if _, err := svc.Rate(ctx, "u2", "m1", models.CreateRatingRequest{Rating: 10}); err != nil {
		t.Fatal(err)
	}
	resp, err := svc.GetMangaRatings(ctx, "m1", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	s := resp.Summary
	if s.RatingCount != 2 || s.AverageRating != 8 || s.RatingDistribution[5] != 1 || s.RatingDistribution[9] != 1 {
		t.Errorf("summary = %+v, want 2 ratings averaging 8 (one 6, one 10)", s)
	}
	if len(resp.Ratings) != 2 {
		t.Errorf("got %d ratings, want 2", len(resp.Ratings))
	}
}

// Rating activity is owned by triggers: one feed entry per rating, refreshed
// when the score changes and removed when the rating is deleted.
func TestRatingActivityTriggers(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()

	feed := func() (count int, rating float64) {
		t.Helper()
		db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(rating), 0) FROM activity_feed
			WHERE user_id = 'u1' AND manga_id = 'm1' AND activity_type = 'rating'`).Scan(&count, &rating)
		return
	}

	svc.Rate(ctx, "u1", "m1", models.CreateRatingRequest{Rating: 8})
	if n, r := feed(); n != 1 || r != 8 {
		t.Errorf("after rating: %d entries (rating %v), want 1 (8)", n, r)
	}
	svc.Rate(ctx, "u1", "m1", models.CreateRatingRequest{Rating: 5})
	if n, r := feed(); n != 1 || r != 5 {
		t.Errorf("after re-rating: %d entries (rating %v), want 1 (5)", n, r)
	}
	svc.DeleteRating(ctx, "u1", "m1")
	if n, _ := feed(); n != 0 {
		t.Errorf("after delete: %d entries, want 0", n)
	}
}
