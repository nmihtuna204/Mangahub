package progress

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"mangahub/internal/auth"
	"mangahub/internal/protocols"
	"mangahub/pkg/models"
)

type fakeBridge struct{ events chan protocols.ProgressEvent }

func (f *fakeBridge) BroadcastProgressUpdate(ev protocols.ProgressEvent) error {
	f.events <- ev
	return nil
}

type recorded struct {
	kind    string
	chapter int
	ctxErr  error
}

type fakeRecorder struct{ calls chan recorded }

func (f *fakeRecorder) RecordChapterRead(ctx context.Context, userID, username, mangaID, mangaTitle string, chapterNum int) error {
	f.calls <- recorded{"chapter", chapterNum, ctx.Err()}
	return ctx.Err()
}

func (f *fakeRecorder) RecordMangaCompleted(ctx context.Context, userID, username, mangaID, mangaTitle string) error {
	f.calls <- recorded{"completed", 0, ctx.Err()}
	return ctx.Err()
}

type fakeMangaSvc struct{}

func (fakeMangaSvc) GetByID(ctx context.Context, id string) (*models.Manga, error) {
	return &models.Manga{ID: id, Title: "Alpha"}, nil
}

// recv waits for the next value, failing (instead of hanging the whole test
// run) if it doesn't come.
func recv[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		panic("unreachable")
	}
}

func newRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(auth.ContextUserKey, &models.UserProfile{ID: "u1", Username: "user1"})
	})
	r.PUT("/users/progress", h.UpdateProgress)
	return r
}

func put(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/users/progress", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// Regression: the background work used gin's request context, which is
// canceled when the handler returns, so activity was never recorded; and the
// bridge was sent the raw request instead of the saved state.
func TestUpdateProgress_BackgroundWork(t *testing.T) {
	_, svc, _ := setup(t)
	bridge := &fakeBridge{events: make(chan protocols.ProgressEvent, 4)}
	rec := &fakeRecorder{calls: make(chan recorded, 4)}
	r := newRouter(NewHandlerWithActivity(svc, bridge, rec, fakeMangaSvc{}))

	if w := put(r, `{"manga_id":"m1","current_chapter":5,"status":"reading"}`); w.Code != 200 {
		t.Fatalf("first update: %d %s", w.Code, w.Body)
	}
	recv(t, bridge.events, "bridge event")
	recv(t, rec.calls, "chapter activity")

	// Chapter only: the bridge must see the stored status, not an empty one
	if w := put(r, `{"manga_id":"m1","current_chapter":6}`); w.Code != 200 {
		t.Fatalf("second update: %d %s", w.Code, w.Body)
	}
	select {
	case ev := <-bridge.events:
		if ev.Chapter != 6 || ev.Status != "reading" || ev.MangaTitle != "Alpha" || ev.Username != "user1" {
			t.Errorf("bridge event = %+v, want chapter 6, status reading, title Alpha", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("bridge was not called")
	}
	select {
	case c := <-rec.calls:
		if c.kind != "chapter" || c.chapter != 6 || c.ctxErr != nil {
			t.Errorf("activity = %+v, want chapter 6 recorded with a live context", c)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("chapter activity was not recorded")
	}

	// Favorite toggle: no chapter sent, so no chapter activity
	put(r, `{"manga_id":"m1","is_favorite":true}`)
	recv(t, bridge.events, "bridge event for the favorite toggle")
	select {
	case c := <-rec.calls:
		t.Errorf("favorite toggle recorded activity %+v", c)
	case <-time.After(200 * time.Millisecond):
	}

	put(r, `{"manga_id":"m1","status":"completed"}`)
	recv(t, bridge.events, "bridge event for completion")
	select {
	case c := <-rec.calls:
		if c.kind != "completed" || c.ctxErr != nil {
			t.Errorf("activity = %+v, want completion", c)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("completion activity was not recorded")
	}
}

func TestUpdateProgress_Errors(t *testing.T) {
	_, svc, _ := setup(t)
	r := newRouter(NewHandler(svc))

	cases := []struct {
		body string
		code int
	}{
		{`{"manga_id":"m1","status":"bogus"}`, 400},
		{`{"manga_id":"m1","current_chapter":-1}`, 400},
		{`{"manga_id":"nope","current_chapter":1}`, 404},
		{`not json`, 400},
		{`{"manga_id":"m1"}`, 200},
	}
	for _, c := range cases {
		if w := put(r, c.body); w.Code != c.code {
			t.Errorf("%s: got %d (%s), want %d", c.body, w.Code, w.Body, c.code)
		}
	}
}
