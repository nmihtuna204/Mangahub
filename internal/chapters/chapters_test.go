package chapters

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"mangahub/internal/testutil"
	"mangahub/pkg/config"
	"mangahub/pkg/external"
	"mangahub/pkg/models"
)

type notification struct {
	mangaID, title string
	chapter        int
	users          []string
}

type fakeNotifier struct {
	mu   sync.Mutex
	sent []notification
	err  error
}

func (f *fakeNotifier) NotifyChapterRelease(ctx context.Context, mangaID, title string, chapter int, users []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	sorted := append([]string(nil), users...)
	sort.Strings(sorted)
	f.sent = append(f.sent, notification{mangaID, title, chapter, sorted})
	return f.err
}

func setup(t *testing.T) (*sql.DB, *fakeNotifier, *Service) {
	t.Helper()
	db := testutil.NewDB(t)
	for _, u := range []string{"alice", "bob", "carol", "dave"} {
		testutil.AddUser(t, db.DB, u)
	}
	testutil.AddManga(t, db.DB, "op", "One Piece") // total_chapters 100
	testutil.AddManga(t, db.DB, "bk", "Berserk")
	testutil.MustExec(t, db.DB, `INSERT INTO reading_progress (id, user_id, manga_id, status) VALUES
		('p1', 'alice', 'op', 'reading'), ('p2', 'bob', 'op', 'plan_to_read'),
		('p3', 'carol', 'op', 'dropped'), ('p4', 'dave', 'bk', 'reading')`)
	n := &fakeNotifier{}
	return db.DB, n, NewService(db.DB, n)
}

func appStatus(err error) int {
	var appErr *models.AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode
	}
	return 0
}

func TestReleaseNotifiesReadersOnly(t *testing.T) {
	db, n, svc := setup(t)

	rel, err := svc.Release(context.Background(), "op", 101)
	if err != nil {
		t.Fatal(err)
	}
	if rel.PreviousLatest != 100 || rel.Chapter != 101 || rel.NotifiedReaders != 2 || rel.Title != "One Piece" {
		t.Errorf("release = %+v", rel)
	}
	var total int
	db.QueryRow(`SELECT total_chapters FROM manga WHERE id = 'op'`).Scan(&total)
	if total != 101 {
		t.Errorf("total_chapters = %d, want 101", total)
	}
	// carol dropped it, dave reads something else
	if len(n.sent) != 1 || strings.Join(n.sent[0].users, ",") != "alice,bob" || n.sent[0].chapter != 101 {
		t.Errorf("notifications = %+v, want one to alice and bob", n.sent)
	}
}

func TestReleaseErrors(t *testing.T) {
	_, n, svc := setup(t)
	ctx := context.Background()

	if _, err := svc.Release(ctx, "op", 100); appStatus(err) != 409 {
		t.Errorf("chapter already known: %v, want 409", err)
	}
	if _, err := svc.Release(ctx, "missing", 5); appStatus(err) != 404 {
		t.Errorf("unknown manga: %v, want 404", err)
	}
	if _, err := svc.Release(ctx, "op", 0); appStatus(err) != 400 {
		t.Errorf("chapter 0: %v, want 400", err)
	}
	if len(n.sent) != 0 {
		t.Errorf("failed releases sent notifications: %+v", n.sent)
	}

	// The release is recorded even if the notification can't be sent
	n.err = errors.New("udp down")
	rel, err := svc.Release(ctx, "op", 102)
	if err == nil || rel == nil || rel.Chapter != 102 {
		t.Errorf("notifier failure: rel %+v, err %v; want the release plus an error", rel, err)
	}
	if _, err := svc.Release(ctx, "op", 102); appStatus(err) != 409 {
		t.Errorf("chapter 102 should already be recorded: %v", err)
	}
}

// Two concurrent releases of the same chapter (e.g. sync + admin) notify once.
func TestConcurrentReleaseNotifiesOnce(t *testing.T) {
	_, n, svc := setup(t)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); svc.Release(context.Background(), "op", 150) }()
	}
	wg.Wait()
	if len(n.sent) != 1 {
		t.Errorf("%d notifications for one chapter, want 1", len(n.sent))
	}
}

type fakeSource struct {
	ids    map[string]string // title -> external ID
	latest map[string]int    // external ID -> chapter
	failOn string
	finds  int
}

func (f *fakeSource) Name() string { return "fake" }
func (f *fakeSource) FindID(ctx context.Context, title string) (string, error) {
	f.finds++
	return f.ids[title], nil
}
func (f *fakeSource) LatestChapter(ctx context.Context, id string) (int, error) {
	if id == f.failOn {
		return 0, errors.New("boom")
	}
	return f.latest[id], nil
}

func TestSyncLinksAndReleases(t *testing.T) {
	db, n, svc := setup(t)
	src := &fakeSource{
		ids:    map[string]string{"One Piece": "md-op"}, // Berserk has no match
		latest: map[string]int{"md-op": 120, "md-bk": 50},
	}
	sy := NewSyncer(db, svc, src)
	ctx := context.Background()

	// Without --link, nothing is linked yet, so nothing is checked
	results, err := sy.Sync(ctx, SyncOptions{})
	if err != nil || len(results) != 0 {
		t.Fatalf("sync without links: %v %v", results, err)
	}

	// Dry run: finds the ID and the new chapter, changes nothing
	results, _ = sy.Sync(ctx, SyncOptions{Link: true, DryRun: true})
	byTitle := map[string]SyncResult{}
	for _, r := range results {
		byTitle[r.Title] = r
	}
	if r := byTitle["One Piece"]; !r.Linked || r.Latest != 120 || r.Released == nil || r.Released.Chapter != 120 {
		t.Errorf("dry run One Piece = %+v", r)
	}
	if r := byTitle["Berserk"]; r.Error == "" || r.Released != nil {
		t.Errorf("dry run Berserk = %+v, want 'no match'", r)
	}
	var links int
	db.QueryRow(`SELECT COUNT(*) FROM manga_external_ids`).Scan(&links)
	if links != 0 || len(n.sent) != 0 {
		t.Errorf("dry run wrote %d links and sent %d notifications", links, len(n.sent))
	}

	// Real run: links One Piece and releases chapter 120 to its readers
	results, _ = sy.Sync(ctx, SyncOptions{Link: true})
	if len(n.sent) != 1 || n.sent[0].chapter != 120 {
		t.Fatalf("notifications = %+v", n.sent)
	}
	for _, r := range results {
		if r.Title == "One Piece" && (!r.Linked || r.Released == nil || r.Released.NotifiedReaders != 2) {
			t.Errorf("One Piece result = %+v, want linked and released to 2 readers", r)
		}
	}
	var mdID string
	db.QueryRow(`SELECT mangadex_id FROM manga_external_ids WHERE manga_id = 'op'`).Scan(&mdID)
	if mdID != "md-op" {
		t.Errorf("saved mangadex_id = %q", mdID)
	}

	// Next run: already linked (no lookup) and up to date (no release)
	src.finds = 0
	results, _ = sy.Sync(ctx, SyncOptions{})
	if len(results) != 1 || results[0].Released != nil || results[0].Known != 120 || src.finds != 0 {
		t.Errorf("second sync = %+v (lookups %d)", results, src.finds)
	}
}

func TestSyncContinuesPastErrorsAndHonorsFilters(t *testing.T) {
	db, _, svc := setup(t)
	testutil.MustExec(t, db, `INSERT INTO manga_external_ids (manga_id, mangadex_id) VALUES ('op', 'md-op'), ('bk', 'md-bk')`)
	src := &fakeSource{latest: map[string]int{"md-op": 130, "md-bk": 999}, failOn: "md-bk"}
	sy := NewSyncer(db, svc, src)

	results, err := sy.Sync(context.Background(), SyncOptions{})
	if err != nil || len(results) != 2 {
		t.Fatalf("results %+v, %v", results, err)
	}
	for _, r := range results {
		if r.Title == "Berserk" && r.Error == "" {
			t.Error("Berserk's source error was not reported")
		}
		if r.Title == "One Piece" && (r.Released == nil || r.Released.Chapter != 130) {
			t.Errorf("One Piece = %+v; an error on another manga must not stop the run", r)
		}
	}

	results, _ = sy.Sync(context.Background(), SyncOptions{MangaID: "bk"})
	if len(results) != 1 || results[0].MangaID != "bk" {
		t.Errorf("--manga filter: %+v", results)
	}
	results, _ = sy.Sync(context.Background(), SyncOptions{Limit: 1})
	if len(results) != 1 {
		t.Errorf("--limit 1: %d results", len(results))
	}
}

func TestRunEveryStopsOnCancel(t *testing.T) {
	db, _, svc := setup(t)
	sy := NewSyncer(db, svc, &fakeSource{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { sy.RunEvery(ctx, 10*time.Millisecond); close(done) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunEvery did not stop")
	}
}

// MangaDexSource against a fake MangaDex API with real response shapes.
func TestMangaDexSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/manga":
			// The match is the second result, by an alt title
			w.Write([]byte(`{"result":"ok","data":[
				{"id":"wrong","attributes":{"title":{"en":"One Piece Party"},"altTitles":[]}},
				{"id":"a1c7c817","attributes":{"title":{"ja-ro":"Wan Pīsu"},"altTitles":[{"en":"ONE PIECE!"},{"ja":"ワンピース"}]}}]}`))
		case "/chapter":
			if r.URL.Query().Get("order[chapter]") != "desc" || r.URL.Query().Get("limit") != "1" ||
				r.URL.Query().Get("translatedLanguage[]") != "en" {
				t.Errorf("chapter query = %s", r.URL.RawQuery)
			}
			if r.URL.Query().Get("manga") == "empty" {
				w.Write([]byte(`{"result":"ok","data":[]}`))
				return
			}
			w.Write([]byte(`{"result":"ok","data":[{"id":"c1","attributes":{"chapter":"1194.5","translatedLanguage":"en"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := external.NewMangaDexClient(&config.MangaDexConfig{BaseURL: srv.URL, RateLimit: 100, Timeout: 5 * time.Second})
	src := NewMangaDexSource(client, "")
	ctx := context.Background()

	if id, err := src.FindID(ctx, "One Piece"); err != nil || id != "a1c7c817" {
		t.Errorf("FindID = %q, %v; want the alt-title match, not the partial one", id, err)
	}
	if id, _ := src.FindID(ctx, "Naruto"); id != "" {
		t.Errorf("FindID(Naruto) = %q, want no match", id)
	}
	if n, err := src.LatestChapter(ctx, "a1c7c817"); err != nil || n != 1194 {
		t.Errorf("LatestChapter = %d, %v; want 1194", n, err)
	}
	if n, err := src.LatestChapter(ctx, "empty"); err != nil || n != 0 {
		t.Errorf("no chapters = %d, %v", n, err)
	}
}

func TestParseChapter(t *testing.T) {
	for in, want := range map[string]int{"1194": 1194, "1095.5": 1095, " 12 ": 12, "": 0, "Oneshot": 0, "-1": 0} {
		if got := parseChapter(in); got != want {
			t.Errorf("parseChapter(%q) = %d, want %d", in, got, want)
		}
	}
}
