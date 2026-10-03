package comment

import (
	"context"
	"errors"
	"testing"

	"mangahub/internal/testutil"
	"mangahub/pkg/models"
)

func setupService(t *testing.T) Service {
	t.Helper()
	db := testutil.NewDB(t)
	testutil.AddUser(t, db.DB, "u1")
	testutil.AddUser(t, db.DB, "u2")
	testutil.AddManga(t, db.DB, "m1", "Alpha")
	testutil.AddManga(t, db.DB, "m2", "Bravo")
	return NewService(NewRepository(db.DB))
}

func mustCreate(t *testing.T, svc Service, user, manga, content, parent string) *models.Comment {
	t.Helper()
	c, err := svc.Create(context.Background(), user, manga, models.CreateCommentRequest{Content: content, ParentID: parent})
	if err != nil {
		t.Fatalf("Create(%q): %v", content, err)
	}
	return c
}

func appStatus(err error) int {
	var appErr *models.AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode
	}
	return 0
}

// Regression: total_count counted replies too, so has_more stayed true on the last page.
func TestGetComments_CountsTopLevelOnly(t *testing.T) {
	svc := setupService(t)
	top := mustCreate(t, svc, "u1", "m1", "top", "")
	mustCreate(t, svc, "u2", "m1", "reply 1", top.ID)
	mustCreate(t, svc, "u2", "m1", "reply 2", top.ID)

	resp, err := svc.GetComments(context.Background(), "m1", nil, "", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalCount != 1 || resp.HasMore || len(resp.Comments) != 1 || len(resp.Comments[0].Replies) != 2 {
		t.Errorf("got total %d, has_more %v, %d top-level with %d replies; want 1, false, 1, 2",
			resp.TotalCount, resp.HasMore, len(resp.Comments), len(resp.Comments[0].Replies))
	}
}

func TestGetComments_LikedByMe(t *testing.T) {
	svc := setupService(t)
	top := mustCreate(t, svc, "u1", "m1", "top", "")
	if err := svc.Like(context.Background(), top.ID, "u2"); err != nil {
		t.Fatal(err)
	}

	liker, _ := svc.GetComments(context.Background(), "m1", nil, "u2", 1, 20)
	other, _ := svc.GetComments(context.Background(), "m1", nil, "u1", 1, 20)
	anon, _ := svc.GetComments(context.Background(), "m1", nil, "", 1, 20)
	if !liker.Comments[0].LikedByMe || other.Comments[0].LikedByMe || anon.Comments[0].LikedByMe {
		t.Errorf("liked_by_me: liker %v, other %v, anonymous %v; want true, false, false",
			liker.Comments[0].LikedByMe, other.Comments[0].LikedByMe, anon.Comments[0].LikedByMe)
	}
	if liker.Comments[0].LikesCount != 1 {
		t.Errorf("likes_count = %d, want 1", liker.Comments[0].LikesCount)
	}
}

func TestCreate_ParentRules(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()
	top := mustCreate(t, svc, "u1", "m1", "top", "")
	reply := mustCreate(t, svc, "u2", "m1", "reply", top.ID)

	// Threads are one level deep: a reply to a reply joins the top-level thread
	nested := mustCreate(t, svc, "u1", "m1", "reply to reply", reply.ID)
	if nested.ParentID == nil || *nested.ParentID != top.ID {
		t.Errorf("reply-to-reply parent = %v, want %s", nested.ParentID, top.ID)
	}

	_, err := svc.Create(ctx, "u1", "m2", models.CreateCommentRequest{Content: "x", ParentID: top.ID})
	if appStatus(err) != 400 {
		t.Errorf("parent on another manga: got %v, want 400", err)
	}
	_, err = svc.Create(ctx, "u1", "m1", models.CreateCommentRequest{Content: "x", ParentID: "missing"})
	if appStatus(err) != 404 {
		t.Errorf("missing parent: got %v, want 404", err)
	}

	if err := svc.Delete(ctx, top.ID, "u1"); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Create(ctx, "u2", "m1", models.CreateCommentRequest{Content: "x", ParentID: top.ID})
	if appStatus(err) != 404 {
		t.Errorf("reply to deleted comment: got %v, want 404", err)
	}
	if appStatus(svc.Like(ctx, top.ID, "u2")) != 404 {
		t.Errorf("liking a deleted comment should be 404")
	}
}

func TestUnknownMangaIs404(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, "u1", "missing", models.CreateCommentRequest{Content: "hi"})
	if appStatus(err) != 404 {
		t.Errorf("create on unknown manga: got %v, want 404 (was 500)", err)
	}
	_, err = svc.GetComments(ctx, "missing", nil, "", 1, 20)
	if appStatus(err) != 404 {
		t.Errorf("list on unknown manga: got %v, want 404", err)
	}

	resp, err := svc.GetComments(ctx, "m2", nil, "", 1, 20)
	if err != nil || resp.Comments == nil || len(resp.Comments) != 0 {
		t.Errorf("empty manga: %#v, %v; want empty non-nil list", resp, err)
	}
}
