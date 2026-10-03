package manga

import (
	"context"
	"testing"

	"mangahub/internal/testutil"
	"mangahub/pkg/models"
)

func setupSearch(t *testing.T) Repository {
	t.Helper()
	db := testutil.NewDB(t)
	for _, m := range []struct{ id, title, author, desc string }{
		{"op", "One Piece", "Eiichiro Oda", "Pirates search for the legendary treasure."},
		{"opm", "One Punch Man", "ONE", "A hero who defeats anyone with one punch."},
		{"pk", "Pokémon Adventures", "Hidenori Kusaka", "Trainers and their monsters."},
		{"tr", "Treasure Hunters", "Someone", "Nothing about pirates here."},
		{"bk", "Berserk", "Kentaro Miura", "A mercenary in a dark world. One of the classics."},
	} {
		testutil.MustExec(t, db.DB, `INSERT INTO manga (id, title, author, description) VALUES (?, ?, ?, ?)`, m.id, m.title, m.author, m.desc)
	}
	testutil.AddGenre(t, db.DB, "op", "Adventure", "adventure")
	return NewRepository(db.DB)
}

func search(t *testing.T, repo Repository, req models.MangaSearchRequest) []string {
	t.Helper()
	list, total, err := repo.List(context.Background(), req)
	if err != nil {
		t.Fatalf("search %+v: %v", req, err)
	}
	if total != len(list) && req.Limit == 0 {
		t.Errorf("search %q: total %d but %d results", req.Query, total, len(list))
	}
	return ids(list)
}

func TestSearch_WordPrefixes(t *testing.T) {
	repo := setupSearch(t)

	if got := search(t, repo, models.MangaSearchRequest{Query: "pie"}); len(got) != 1 || got[0] != "op" {
		t.Errorf("'pie' = %v, want [op] (prefix of Piece)", got)
	}
	if got := search(t, repo, models.MangaSearchRequest{Query: "one piece"}); len(got) != 1 || got[0] != "op" {
		t.Errorf("'one piece' = %v, want only One Piece (all words must match)", got)
	}
	if got := search(t, repo, models.MangaSearchRequest{Query: "miura"}); len(got) != 1 || got[0] != "bk" {
		t.Errorf("author search = %v, want [bk]", got)
	}
	// unicode61 folds accents: "pokemon" finds "Pokémon"
	if got := search(t, repo, models.MangaSearchRequest{Query: "POKEMON"}); len(got) != 1 || got[0] != "pk" {
		t.Errorf("'POKEMON' = %v, want [pk]", got)
	}
}

func TestSearch_RanksTitleMatchesFirst(t *testing.T) {
	repo := setupSearch(t)

	// "treasure" is in Treasure Hunters' title but only One Piece's description
	got := search(t, repo, models.MangaSearchRequest{Query: "treasure"})
	if len(got) != 2 || got[0] != "tr" || got[1] != "op" {
		t.Errorf("'treasure' = %v, want [tr op]: title match before description match", got)
	}
	// "one" is in two titles (and Berserk's description, which ranks last)
	got = search(t, repo, models.MangaSearchRequest{Query: "one"})
	if len(got) != 3 || got[2] != "bk" {
		t.Errorf("'one' = %v, want the description-only match (bk) last", got)
	}
	// An explicit sort overrides relevance
	got = search(t, repo, models.MangaSearchRequest{Query: "treasure", SortBy: "title", Order: "asc"})
	if len(got) != 2 || got[0] != "op" {
		t.Errorf("sorted by title = %v, want [op tr]", got)
	}
}

func TestSearch_CombinesWithFiltersAndPaging(t *testing.T) {
	repo := setupSearch(t)

	got := search(t, repo, models.MangaSearchRequest{Query: "treasure", Genres: []string{"adventure"}})
	if len(got) != 1 || got[0] != "op" {
		t.Errorf("search + genre = %v, want [op]", got)
	}
	list, total, err := repo.List(context.Background(), models.MangaSearchRequest{Query: "one", Limit: 1, Offset: 1})
	if err != nil || total != 3 || len(list) != 1 {
		t.Errorf("paged search: %d results, total %d, %v; want 1 of 3", len(list), total, err)
	}
}

// User input must never be able to break or steer the FTS5 query syntax.
func TestSearch_HostileInputIsSafe(t *testing.T) {
	repo := setupSearch(t)
	for _, q := range []string{`"`, `one"`, `NEAR(one piece)`, `title:one`, `one OR berserk`, `*`, `-one`, `'; DROP TABLE manga; --`, `!!!`} {
		if _, _, err := repo.List(context.Background(), models.MangaSearchRequest{Query: q}); err != nil {
			t.Errorf("query %q: %v", q, err)
		}
	}
	// OR is treated as a plain word, not an operator
	if got := search(t, repo, models.MangaSearchRequest{Query: "one OR berserk"}); len(got) != 0 {
		t.Errorf("'one OR berserk' = %v, want no results (all three words required)", got)
	}
	if got := search(t, repo, models.MangaSearchRequest{Query: "!!!"}); len(got) != 0 {
		t.Errorf("punctuation-only query = %v, want none", got)
	}
}

// The index follows title changes through the manga_fts_update trigger.
func TestSearch_FollowsUpdates(t *testing.T) {
	db := testutil.NewDB(t)
	testutil.AddManga(t, db.DB, "m1", "Old Title")
	repo := NewRepository(db.DB)

	testutil.MustExec(t, db.DB, `UPDATE manga SET title = 'Fresh Name' WHERE id = 'm1'`)
	if got := search(t, repo, models.MangaSearchRequest{Query: "fresh"}); len(got) != 1 {
		t.Errorf("new title not found: %v", got)
	}
	if got := search(t, repo, models.MangaSearchRequest{Query: "old"}); len(got) != 0 {
		t.Errorf("old title still found: %v", got)
	}
	testutil.MustExec(t, db.DB, `DELETE FROM manga WHERE id = 'm1'`)
	if got := search(t, repo, models.MangaSearchRequest{Query: "fresh"}); len(got) != 0 {
		t.Errorf("deleted manga still found: %v", got)
	}
}
