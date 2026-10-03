package manga

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"mangahub/internal/testutil"
	"mangahub/pkg/models"
)

func setupRepo(t *testing.T) (Repository, *sql.DB) {
	t.Helper()
	db := testutil.NewDB(t)
	testutil.AddManga(t, db.DB, "m1", "Alpha")
	testutil.AddManga(t, db.DB, "m2", "Bravo")
	testutil.AddManga(t, db.DB, "m3", "Charlie")
	testutil.MustExec(t, db.DB, `UPDATE manga SET type = 'manhwa', status = 'completed', year = 2020 WHERE id = 'm2'`)
	testutil.MustExec(t, db.DB, `UPDATE manga SET year = 2010 WHERE id = 'm1'`)
	testutil.MustExec(t, db.DB, `UPDATE manga SET year = 2015 WHERE id = 'm3'`)
	testutil.AddGenre(t, db.DB, "m1", "Slice of Life", "slice-of-life")
	testutil.AddGenre(t, db.DB, "m1", "Action", "action")
	testutil.AddGenre(t, db.DB, "m2", "Action", "action")
	return NewRepository(db.DB), db.DB
}

func ids(list []models.Manga) []string {
	out := make([]string, len(list))
	for i, m := range list {
		out[i] = m.ID
	}
	return out
}

func TestList_PaginationAndTotal(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	page1, total, err := repo.List(ctx, models.MangaSearchRequest{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	page2, _, err := repo.List(ctx, models.MangaSearchRequest{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if got := ids(page1); len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Errorf("page1 = %v, want [m1 m2] (title order)", got)
	}
	if got := ids(page2); len(got) != 1 || got[0] != "m3" {
		t.Errorf("page2 = %v, want [m3]", got)
	}
}

func TestList_GenreFilterMatchesSlugOrName(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	for _, genre := range []string{"slice-of-life", "Slice of Life", "SLICE OF LIFE"} {
		list, total, err := repo.List(ctx, models.MangaSearchRequest{Genres: []string{genre}})
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 || len(list) != 1 || list[0].ID != "m1" {
			t.Errorf("genre %q: got %v (total %d), want [m1]", genre, ids(list), total)
		}
	}

	list, _, err := repo.List(ctx, models.MangaSearchRequest{Genres: []string{"action"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(list); len(got) != 2 {
		t.Errorf("action: got %v, want m1 and m2", got)
	}
}

func TestList_TypeStatusAndSort(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	list, _, _ := repo.List(ctx, models.MangaSearchRequest{Type: "manhwa"})
	if got := ids(list); len(got) != 1 || got[0] != "m2" {
		t.Errorf("type=manhwa: got %v, want [m2]", got)
	}
	list, _, _ = repo.List(ctx, models.MangaSearchRequest{Status: "completed"})
	if got := ids(list); len(got) != 1 || got[0] != "m2" {
		t.Errorf("status=completed: got %v, want [m2]", got)
	}

	list, _, _ = repo.List(ctx, models.MangaSearchRequest{SortBy: "year"})
	if got := ids(list); got[0] != "m2" || got[2] != "m1" {
		t.Errorf("sort_by=year: got %v, want newest first [m2 m3 m1]", got)
	}
	list, _, _ = repo.List(ctx, models.MangaSearchRequest{SortBy: "year", Order: "asc"})
	if got := ids(list); got[0] != "m1" {
		t.Errorf("sort_by=year order=asc: got %v, want oldest first", got)
	}
	// Unknown sort values fall back to title order instead of reaching the SQL
	list, _, err := repo.List(ctx, models.MangaSearchRequest{SortBy: "title; DROP TABLE manga", Order: "sideways"})
	if err != nil || ids(list)[0] != "m1" {
		t.Errorf("bogus sort: got %v, %v", ids(list), err)
	}
}

func TestList_EmptyResultIsEmptySliceNotNil(t *testing.T) {
	repo, _ := setupRepo(t)
	list, total, err := repo.List(context.Background(), models.MangaSearchRequest{Query: "no-such-title"})
	if err != nil {
		t.Fatal(err)
	}
	if list == nil || len(list) != 0 || total != 0 {
		t.Errorf("got %#v (total %d), want empty non-nil slice (JSON [] not null)", list, total)
	}
}

func TestList_LoadsGenres(t *testing.T) {
	repo, _ := setupRepo(t)
	list, _, err := repo.List(context.Background(), models.MangaSearchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list[0].Genres) != 2 || list[0].Genres[0].Slug != "action" {
		t.Errorf("m1 genres = %+v, want [action slice-of-life]", list[0].Genres)
	}
	if len(list[2].Genres) != 0 {
		t.Errorf("m3 genres = %+v, want none", list[2].Genres)
	}
}

// Regression: genre lookups used to run inside the open rows loop, holding two
// pooled connections per request, so more concurrent requests than the pool
// size deadlocked. With a single connection that deadlock is immediate.
func TestList_WorksWithSingleConnection(t *testing.T) {
	repo, sqlDB := setupRepo(t)
	sqlDB.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	list, _, err := repo.List(ctx, models.MangaSearchRequest{})
	if err != nil {
		t.Fatalf("List with one pooled connection: %v (nested query deadlock?)", err)
	}
	// The old loader swallowed its errors, so a deadlock showed up as missing genres
	if len(list) != 3 || len(list[0].Genres) != 2 {
		t.Errorf("got %d manga, m1 genres %v; want 3 manga and 2 genres", len(list), list[0].Genres)
	}
	if _, err := repo.GetByID(ctx, "m1"); err != nil {
		t.Fatalf("GetByID with one pooled connection: %v", err)
	}
}

func TestGetByID(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	m, err := repo.GetByID(ctx, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Alpha" || len(m.Genres) != 2 {
		t.Errorf("got %+v", m)
	}

	_, err = repo.GetByID(ctx, "missing")
	var appErr *models.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode != 404 {
		t.Errorf("missing manga: got %v, want 404 AppError", err)
	}
}

func TestAttachGenres_Empty(t *testing.T) {
	db := testutil.NewDB(t)
	if err := AttachGenres(context.Background(), db.DB, nil); err != nil {
		t.Errorf("AttachGenres(nil) = %v", err)
	}
}
