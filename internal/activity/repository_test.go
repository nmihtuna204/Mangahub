package activity

import (
	"context"
	"testing"

	"mangahub/internal/testutil"
	"mangahub/pkg/models"
)

// Regression: rating and progress activities have NULL comment_text, which
// made GET /activities return 500 once anyone rated a manga.
func TestReadsActivitiesWithNullCommentText(t *testing.T) {
	db := testutil.NewDB(t)
	testutil.AddUser(t, db.DB, "u1")
	testutil.AddManga(t, db.DB, "m1", "Alpha")
	testutil.MustExec(t, db.DB, `INSERT INTO manga_ratings (id, manga_id, user_id, rating) VALUES ('r1', 'm1', 'u1', 9)`)

	svc := NewService(NewRepository(db.DB))
	ctx := context.Background()
	if err := svc.RecordChapterRead(ctx, "u1", "u1", "m1", "Alpha", 3); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordCommentAdded(ctx, "u1", "u1", "m1", "Alpha", "nice"); err != nil {
		t.Fatal(err)
	}

	var nulls int
	db.QueryRow(`SELECT COUNT(*) FROM activity_feed WHERE comment_text IS NULL`).Scan(&nulls)
	if nulls != 2 {
		t.Fatalf("expected the rating and chapter activities to have NULL comment_text, got %d", nulls)
	}

	recent, total, err := svc.GetRecentActivities(ctx, 20, 0)
	if err != nil {
		t.Fatalf("GetRecentActivities: %v", err)
	}
	if total != 3 || len(recent) != 3 {
		t.Errorf("got %d/%d activities, want 3", len(recent), total)
	}
	byType := map[string]models.Activity{}
	for _, a := range recent {
		byType[a.ActivityType] = a
	}
	if a := byType["rating"]; a.Rating == nil || *a.Rating != 9 || a.CommentText != "" {
		t.Errorf("rating activity = %+v", a)
	}
	if a := byType["progress"]; a.ChapterNumber == nil || *a.ChapterNumber != 3 {
		t.Errorf("progress activity = %+v", a)
	}
	if a := byType["comment"]; a.CommentText != "nice" {
		t.Errorf("comment activity = %+v", a)
	}

	mine, _, err := svc.GetUserActivities(ctx, "u1", 20, 0)
	if err != nil || len(mine) != 3 {
		t.Errorf("GetUserActivities = %d, %v; want 3", len(mine), err)
	}
}
