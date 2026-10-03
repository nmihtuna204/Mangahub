package customlist

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"mangahub/internal/testutil"
	"mangahub/pkg/models"
)

func setup(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	db := testutil.NewDB(t)
	testutil.AddUser(t, db.DB, "alice")
	testutil.AddUser(t, db.DB, "bob")
	testutil.AddManga(t, db.DB, "m1", "Alpha")
	testutil.AddManga(t, db.DB, "m2", "Bravo")
	testutil.AddManga(t, db.DB, "m3", "Charlie")
	testutil.AddGenre(t, db.DB, "m1", "Action", "action")
	return NewService(NewRepository(db.DB)), db.DB
}

func status(err error) int {
	var appErr *models.AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode
	}
	return 0
}

func mustCreate(t *testing.T, s *Service, user, name string, public bool) *models.CustomList {
	t.Helper()
	l, err := s.CreateList(context.Background(), user, models.CreateListRequest{Name: name, IsPublic: public})
	if err != nil {
		t.Fatalf("CreateList(%q): %v", name, err)
	}
	return l
}

func TestCreateAndValidate(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()

	l := mustCreate(t, s, "alice", "  Favorites  ", false)
	if l.Name != "Favorites" || l.ID == "" || l.UserID != "alice" {
		t.Errorf("created %+v", l)
	}
	for _, name := range []string{"", "   ", string(make([]byte, 101))} {
		if _, err := s.CreateList(ctx, "alice", models.CreateListRequest{Name: name}); status(err) != 400 {
			t.Errorf("name %q: got %v, want 400", name, err)
		}
	}
}

func TestVisibility(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	private := mustCreate(t, s, "alice", "Private", false)
	public := mustCreate(t, s, "alice", "Public", true)

	mine, err := s.ListsOf(ctx, "alice", "alice")
	if err != nil || mine.Total != 2 {
		t.Errorf("owner sees %d lists, want 2 (%v)", mine.Total, err)
	}
	theirs, _ := s.ListsOf(ctx, "alice", "bob")
	if theirs.Total != 1 || theirs.Lists[0].ID != public.ID {
		t.Errorf("bob sees %+v, want only the public list", theirs.Lists)
	}
	anon, _ := s.ListsOf(ctx, "alice", "")
	if anon.Total != 1 {
		t.Errorf("anonymous sees %d lists, want 1", anon.Total)
	}
	empty, _ := s.ListsOf(ctx, "bob", "bob")
	if empty.Lists == nil || empty.Total != 0 {
		t.Errorf("no lists = %#v, want empty non-nil slice", empty.Lists)
	}

	if _, err := s.GetList(ctx, private.ID, "bob"); status(err) != 404 {
		t.Errorf("bob reading alice's private list: %v, want 404 (not 403: don't reveal it exists)", err)
	}
	if _, err := s.GetList(ctx, public.ID, ""); err != nil {
		t.Errorf("anonymous reading a public list: %v", err)
	}
	if _, err := s.GetList(ctx, "missing", "alice"); status(err) != 404 {
		t.Errorf("missing list: %v", err)
	}
}

func TestOnlyOwnerCanChange(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	private := mustCreate(t, s, "alice", "Private", false)
	public := mustCreate(t, s, "alice", "Public", true)
	name := "Hijacked"

	if _, err := s.UpdateList(ctx, public.ID, "bob", models.UpdateListRequest{Name: &name}); status(err) != 403 {
		t.Errorf("bob renaming a public list: %v, want 403", err)
	}
	if _, err := s.UpdateList(ctx, private.ID, "bob", models.UpdateListRequest{Name: &name}); status(err) != 404 {
		t.Errorf("bob renaming a private list: %v, want 404", err)
	}
	if _, err := s.AddToList(ctx, public.ID, "bob", models.AddToListRequest{MangaID: "m1"}); status(err) != 403 {
		t.Errorf("bob adding to alice's list: %v, want 403", err)
	}
	if err := s.DeleteList(ctx, public.ID, "bob"); status(err) != 403 {
		t.Errorf("bob deleting alice's list: %v, want 403", err)
	}
}

func TestItemsLifecycle(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	l := mustCreate(t, s, "alice", "Top picks", false)

	for _, id := range []string{"m2", "m1", "m3"} {
		if _, err := s.AddToList(ctx, l.ID, "alice", models.AddToListRequest{MangaID: id}); err != nil {
			t.Fatalf("add %s: %v", id, err)
		}
	}
	// Re-adding only updates notes; the position stays
	item, err := s.AddToList(ctx, l.ID, "alice", models.AddToListRequest{MangaID: "m2", Notes: "re-read"})
	if err != nil || item.Notes != "re-read" || item.SortOrder != 0 {
		t.Errorf("re-add = %+v, %v; want notes updated, still first", item, err)
	}
	if _, err := s.AddToList(ctx, l.ID, "alice", models.AddToListRequest{MangaID: "missing"}); status(err) != 404 {
		t.Errorf("unknown manga: %v, want 404", err)
	}
	if _, err := s.AddToList(ctx, l.ID, "alice", models.AddToListRequest{}); status(err) != 400 {
		t.Errorf("missing manga_id: %v, want 400", err)
	}

	got, err := s.GetList(ctx, l.ID, "alice")
	if err != nil {
		t.Fatal(err)
	}
	order := []string{}
	for _, it := range got.Items {
		order = append(order, it.MangaID)
	}
	if len(order) != 3 || order[0] != "m2" || order[1] != "m1" || order[2] != "m3" || got.ItemCount != 3 {
		t.Errorf("items %v (count %d), want [m2 m1 m3]", order, got.ItemCount)
	}
	if got.Items[1].Manga.Title != "Alpha" || len(got.Items[1].Manga.Genres) != 1 {
		t.Errorf("item manga = %+v, want Alpha with its genre", got.Items[1].Manga)
	}

	// Reorder needs exactly the list's item IDs
	ids := []string{got.Items[2].ID, got.Items[0].ID, got.Items[1].ID}
	if err := s.ReorderList(ctx, l.ID, "alice", models.ReorderListRequest{ItemIDs: ids[:2]}); status(err) != 400 {
		t.Errorf("partial reorder: %v, want 400", err)
	}
	if err := s.ReorderList(ctx, l.ID, "alice", models.ReorderListRequest{ItemIDs: []string{ids[0], ids[0], ids[1]}}); status(err) != 400 {
		t.Errorf("duplicate IDs: %v, want 400", err)
	}
	if err := s.ReorderList(ctx, l.ID, "alice", models.ReorderListRequest{ItemIDs: ids}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetList(ctx, l.ID, "alice")
	if got.Items[0].MangaID != "m3" || got.Items[2].MangaID != "m1" {
		t.Errorf("after reorder: %s, %s, %s", got.Items[0].MangaID, got.Items[1].MangaID, got.Items[2].MangaID)
	}

	if err := s.RemoveFromList(ctx, l.ID, "m3", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveFromList(ctx, l.ID, "m3", "alice"); status(err) != 404 {
		t.Errorf("removing twice: %v, want 404", err)
	}

	if err := s.DeleteList(ctx, l.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetList(ctx, l.ID, "alice"); status(err) != 404 {
		t.Errorf("deleted list still readable: %v", err)
	}
}

func TestUpdateList(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	l := mustCreate(t, s, "alice", "Old", false)
	name, desc, public, blank := "New", "desc", true, " "

	got, err := s.UpdateList(ctx, l.ID, "alice", models.UpdateListRequest{Name: &name, Description: &desc, IsPublic: &public})
	if err != nil || got.Name != "New" || got.Description != "desc" || !got.IsPublic {
		t.Errorf("update = %+v, %v", got, err)
	}
	got, err = s.UpdateList(ctx, l.ID, "alice", models.UpdateListRequest{Description: &desc})
	if err != nil || got.Name != "New" {
		t.Errorf("description-only update changed the name: %+v, %v", got, err)
	}
	if _, err := s.UpdateList(ctx, l.ID, "alice", models.UpdateListRequest{Name: &blank}); status(err) != 400 {
		t.Errorf("blank name: %v, want 400", err)
	}
}

// Adding to a public list shows in the activity feed (activity type list_add,
// declared in the schema but never produced before); private lists don't.
func TestListAddActivity(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	public := mustCreate(t, s, "alice", "Must read", true)
	private := mustCreate(t, s, "alice", "Secret", false)

	s.AddToList(ctx, public.ID, "alice", models.AddToListRequest{MangaID: "m1"})
	s.AddToList(ctx, private.ID, "alice", models.AddToListRequest{MangaID: "m2"})

	count := func() (n int, listName string) {
		db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(comment_text), '') FROM activity_feed WHERE activity_type = 'list_add'`).Scan(&n, &listName)
		return
	}
	if n, name := count(); n != 1 || name != "Must read" {
		t.Errorf("list_add activities = %d (%q), want 1 for the public list", n, name)
	}
	s.RemoveFromList(ctx, public.ID, "m1", "alice")
	if n, _ := count(); n != 0 {
		t.Errorf("activity remains after removing the manga")
	}
}
