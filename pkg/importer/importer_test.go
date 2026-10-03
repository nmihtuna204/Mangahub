package importer

import (
	"context"
	"testing"

	"mangahub/internal/testutil"
	"mangahub/pkg/models"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Slice of Life": "slice-of-life",
		"Sci-Fi":        "sci-fi",
		"  Boys' Love ": "boys-love",
		"Action":        "action",
		"!!!":           "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// Regression: unknown statuses were passed through (or became "unknown") and
// failed the manga.status CHECK constraint, so those titles never imported.
func TestNormalizeStatusAlwaysValid(t *testing.T) {
	valid := map[string]bool{"ongoing": true, "completed": true, "hiatus": true, "cancelled": true}
	for _, in := range []string{"Publishing", "Finished", "On Hiatus", "Discontinued", "Upcoming", "", "weird"} {
		if got := normalizeStatus(in); !valid[got] {
			t.Errorf("normalizeStatus(%q) = %q, not accepted by the schema", in, got)
		}
	}
	if normalizeStatus("Finished") != "completed" || normalizeStatus("Publishing") != "ongoing" {
		t.Error("known statuses mapped incorrectly")
	}
}

func TestImportLinksGenresAndExternalIDs(t *testing.T) {
	db := testutil.NewDB(t)
	testutil.MustExec(t, db.DB, `INSERT INTO genres (id, name, slug) VALUES ('g-action', 'Action', 'action')`)
	imp := NewImporter(db.DB, nil) // no Redis cache
	ctx := context.Background()

	ext := models.ExternalMangaData{
		Source: models.SourceJikan, ExternalID: "13", Title: "One Piece", Status: "Publishing",
		Genres: []string{"Action", "Slice of Life", "action"}, Authors: []string{"Oda"},
	}
	m, err := imp.ImportOne(ctx, ext)
	if err != nil {
		t.Fatal(err)
	}

	var genreLinks, genres int
	db.QueryRow(`SELECT COUNT(*) FROM manga_genres WHERE manga_id = ?`, m.ID).Scan(&genreLinks)
	db.QueryRow(`SELECT COUNT(*) FROM genres`).Scan(&genres)
	if genreLinks != 2 || genres != 2 {
		t.Errorf("genre links = %d, genres = %d; want 2 and 2 (reuse Action, create Slice of Life)", genreLinks, genres)
	}

	var malID int
	if err := db.QueryRow(`SELECT mal_id FROM manga_external_ids WHERE manga_id = ?`, m.ID).Scan(&malID); err != nil || malID != 13 {
		t.Errorf("external mapping: mal_id %d, %v; want 13 (the insert used a nonexistent id column)", malID, err)
	}

	// Same title from MangaDex: updates the manga and adds the second ID to the same mapping row
	ext2 := models.ExternalMangaData{Source: models.SourceMangaDex, ExternalID: "abc-123", Title: "one piece", Status: "ongoing"}
	m2, err := imp.ImportOne(ctx, ext2)
	if err != nil {
		t.Fatal(err)
	}
	var mdID string
	db.QueryRow(`SELECT mangadex_id, mal_id FROM manga_external_ids WHERE manga_id = ?`, m.ID).Scan(&mdID, &malID)
	if m2.ID != m.ID || mdID != "abc-123" || malID != 13 {
		t.Errorf("after second source: manga %s vs %s, mangadex_id %q, mal_id %d", m2.ID, m.ID, mdID, malID)
	}
}
