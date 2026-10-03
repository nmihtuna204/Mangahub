package progress

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"mangahub/internal/testutil"
	"mangahub/pkg/models"
)

func setup(t *testing.T) (Repository, Service, *sql.DB) {
	t.Helper()
	db := testutil.NewDB(t)
	testutil.AddUser(t, db.DB, "u1")
	testutil.AddManga(t, db.DB, "m1", "Alpha")
	testutil.AddManga(t, db.DB, "m2", "Bravo")
	testutil.AddGenre(t, db.DB, "m1", "Action", "action")
	repo := NewRepository(db.DB)
	return repo, NewService(repo), db.DB
}

func intp(v int) *int       { return &v }
func strp(v string) *string { return &v }
func boolp(v bool) *bool    { return &v }
func req(fields models.UpdateProgressRequest) models.UpdateProgressRequest {
	if fields.MangaID == "" {
		fields.MangaID = "m1"
	}
	return fields
}

func mustUpdate(t *testing.T, svc Service, r models.UpdateProgressRequest) *models.ReadingProgress {
	t.Helper()
	p, err := svc.Update(context.Background(), "u1", r)
	if err != nil {
		t.Fatalf("Update(%+v): %v", r, err)
	}
	return p
}

func TestInsertDefaults(t *testing.T) {
	_, svc, _ := setup(t)

	p := mustUpdate(t, svc, req(models.UpdateProgressRequest{}))
	if p.Status != "plan_to_read" || p.CurrentChapter != 0 || p.IsFavorite || p.StartedAt != nil {
		t.Errorf("empty new entry = %+v, want plan_to_read / ch 0 / not started", p)
	}

	p = mustUpdate(t, svc, req(models.UpdateProgressRequest{MangaID: "m2", CurrentChapter: intp(3)}))
	if p.Status != "reading" || p.StartedAt == nil {
		t.Errorf("new entry with a chapter = %+v, want status reading and started_at set", p)
	}
}

// Regression: every update used to overwrite all columns, so a chapter update
// cleared the favorite flag and a status-only update reset the chapter to 0.
func TestPartialUpdates(t *testing.T) {
	_, svc, _ := setup(t)

	mustUpdate(t, svc, req(models.UpdateProgressRequest{CurrentChapter: intp(5), Status: strp("reading"), IsFavorite: boolp(true)}))

	p := mustUpdate(t, svc, req(models.UpdateProgressRequest{CurrentChapter: intp(6)}))
	if p.CurrentChapter != 6 || !p.IsFavorite || p.Status != "reading" {
		t.Errorf("chapter-only update = %+v, want ch 6, still favorite, still reading", p)
	}

	p = mustUpdate(t, svc, req(models.UpdateProgressRequest{IsFavorite: boolp(false)}))
	if p.CurrentChapter != 6 || p.IsFavorite {
		t.Errorf("favorite-only update = %+v, want ch 6 and not favorite", p)
	}

	p = mustUpdate(t, svc, req(models.UpdateProgressRequest{Status: strp("on_hold")}))
	if p.CurrentChapter != 6 || p.Status != "on_hold" {
		t.Errorf("status-only update = %+v, want ch 6 on_hold", p)
	}
}

func TestCompletedAtLifecycle(t *testing.T) {
	_, svc, _ := setup(t)

	p := mustUpdate(t, svc, req(models.UpdateProgressRequest{Status: strp("completed")}))
	if p.CompletedAt == nil || p.StartedAt == nil {
		t.Fatalf("completing = %+v, want started_at and completed_at set", p)
	}
	first := *p.CompletedAt

	time.Sleep(10 * time.Millisecond)
	p = mustUpdate(t, svc, req(models.UpdateProgressRequest{Status: strp("completed")}))
	if p.CompletedAt == nil || !p.CompletedAt.Equal(first) {
		t.Errorf("completing again moved completed_at: %v -> %v", first, p.CompletedAt)
	}

	p = mustUpdate(t, svc, req(models.UpdateProgressRequest{Status: strp("reading")}))
	if p.CompletedAt != nil {
		t.Errorf("leaving completed kept completed_at = %v", p.CompletedAt)
	}
}

func TestLastReadAtOnlyMovesWithChapter(t *testing.T) {
	_, svc, _ := setup(t)

	p := mustUpdate(t, svc, req(models.UpdateProgressRequest{CurrentChapter: intp(1)}))
	lastRead := p.LastReadAt

	time.Sleep(10 * time.Millisecond)
	p = mustUpdate(t, svc, req(models.UpdateProgressRequest{IsFavorite: boolp(true)}))
	if !p.LastReadAt.Equal(lastRead) {
		t.Errorf("toggling favorite moved last_read_at (reorders the library): %v -> %v", lastRead, p.LastReadAt)
	}

	p = mustUpdate(t, svc, req(models.UpdateProgressRequest{CurrentChapter: intp(2)}))
	if !p.LastReadAt.After(lastRead) {
		t.Errorf("reading a chapter did not move last_read_at")
	}
}

func TestServiceValidation(t *testing.T) {
	_, svc, _ := setup(t)
	ctx := context.Background()

	// An explicit empty status means "unchanged", like omitting it
	mustUpdate(t, svc, req(models.UpdateProgressRequest{Status: strp("reading")}))
	p := mustUpdate(t, svc, req(models.UpdateProgressRequest{Status: strp(""), CurrentChapter: intp(2)}))
	if p.Status != "reading" {
		t.Errorf("status \"\" changed status to %q", p.Status)
	}

	cases := map[string]models.UpdateProgressRequest{
		"invalid status":   req(models.UpdateProgressRequest{Status: strp("bogus")}),
		"negative chapter": req(models.UpdateProgressRequest{CurrentChapter: intp(-1)}),
		"missing manga_id": {CurrentChapter: intp(1)},
	}
	for name, r := range cases {
		_, err := svc.Update(ctx, "u1", r)
		var appErr *models.AppError
		if !errors.As(err, &appErr) || appErr.StatusCode != 400 {
			t.Errorf("%s: got %v, want 400", name, err)
		}
	}

	_, err := svc.Update(ctx, "u1", req(models.UpdateProgressRequest{MangaID: "nope", CurrentChapter: intp(1)}))
	var appErr *models.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode != 404 {
		t.Errorf("unknown manga: got %v, want 404", err)
	}
}

// Regression: check-then-insert let concurrent first updates of the same
// (user, manga) fail on the UNIQUE constraint.
func TestConcurrentFirstUpdates(t *testing.T) {
	_, svc, db := setup(t)

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(ch int) {
			defer wg.Done()
			if _, err := svc.Update(context.Background(), "u1", req(models.UpdateProgressRequest{CurrentChapter: intp(ch)})); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent update failed: %v", err)
	}

	var rows int
	db.QueryRow(`SELECT COUNT(*) FROM reading_progress WHERE user_id = 'u1' AND manga_id = 'm1'`).Scan(&rows)
	if rows != 1 {
		t.Errorf("got %d rows, want 1", rows)
	}
}

func TestListByUser(t *testing.T) {
	repo, svc, db := setup(t)
	mustUpdate(t, svc, req(models.UpdateProgressRequest{CurrentChapter: intp(1)}))
	mustUpdate(t, svc, req(models.UpdateProgressRequest{MangaID: "m2", CurrentChapter: intp(1)}))

	// One pooled connection: genres must load after the rows are closed
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	list, err := repo.ListByUser(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d entries, want 2", len(list))
	}
	for _, e := range list {
		if e.MangaID == "m1" && (len(e.Manga.Genres) != 1 || e.Manga.Genres[0].Slug != "action") {
			t.Errorf("m1 genres = %+v, want [action]", e.Manga.Genres)
		}
	}

	empty, err := repo.ListByUser(ctx, "nobody")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("empty library = %#v, %v; want empty non-nil slice", empty, err)
	}
}

func TestDelete(t *testing.T) {
	_, svc, _ := setup(t)
	ctx := context.Background()
	mustUpdate(t, svc, req(models.UpdateProgressRequest{}))

	if err := svc.Delete(ctx, "u1", "m1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	err := svc.Delete(ctx, "u1", "m1")
	var appErr *models.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode != 404 {
		t.Errorf("second delete: got %v, want 404", err)
	}
}
