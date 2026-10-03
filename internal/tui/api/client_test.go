package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Client{httpClient: srv.Client(), baseURL: srv.URL, cache: NewCache()}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func apiErr(code, msg string) map[string]interface{} {
	return map[string]interface{}{"success": false, "error": map[string]string{"code": code, "message": msg}}
}

// Regression: every request was retried with an already-consumed body, and
// POSTs were retried too (duplicate comments/ratings).
func TestRetriesOnlyIdempotentRequestsWithFullBody(t *testing.T) {
	var gets, posts int32
	var mu sync.Mutex
	var bodies []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		switch r.Method {
		case http.MethodPut:
			mu.Lock()
			bodies = append(bodies, string(b))
			n := len(bodies)
			mu.Unlock()
			if n < 3 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			writeJSON(w, 200, map[string]interface{}{"success": true})
		case http.MethodGet:
			atomic.AddInt32(&gets, 1)
			w.WriteHeader(http.StatusInternalServerError)
		case http.MethodPost:
			atomic.AddInt32(&posts, 1)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	ctx := context.Background()

	if err := c.ToggleFavorite(ctx, "m1", true); err != nil {
		t.Fatalf("PUT after two 502s: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	for i, b := range bodies {
		if !strings.Contains(b, `"is_favorite":true`) {
			t.Errorf("attempt %d sent body %q (retries used to send an empty body)", i+1, b)
		}
	}

	if c.HealthCheck(ctx) {
		t.Error("HealthCheck true on 500")
	}
	if atomic.LoadInt32(&gets) != DefaultRetries {
		t.Errorf("GET attempts = %d, want %d", gets, DefaultRetries)
	}
	if err := c.LikeComment(ctx, "c1"); err == nil {
		t.Error("POST 500 reported as success")
	}
	if atomic.LoadInt32(&posts) != 1 {
		t.Errorf("POST attempts = %d, want 1 (no retry)", posts)
	}
}

// Regression: write calls ignored the status code, so 4xx looked like success.
func TestWriteErrorsCarryAPIMessage(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, apiErr("VALIDATION_ERROR", "invalid rating data"))
	})
	err := c.SubmitRating(context.Background(), "m1", 42, "")
	if err == nil || !strings.Contains(err.Error(), "invalid rating data") {
		t.Errorf("got %v, want the API's validation message", err)
	}
}

// Regression: the summary is nested under data.summary; the client read data.* and always showed 0 ratings.
func TestGetRatingsReadsSummary(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"success": true, "data": map[string]interface{}{
			"summary": map[string]interface{}{"manga_id": "m1", "average_rating": 8.5, "rating_count": 2},
			"ratings": []interface{}{},
		}})
	})
	s, err := c.GetRatings(context.Background(), "m1")
	if err != nil || s == nil || s.RatingCount != 2 || s.AverageRating != 8.5 {
		t.Errorf("got %+v, %v", s, err)
	}
}

func TestSearchSendsLimitOffsetAndGenre(t *testing.T) {
	var mu sync.Mutex
	var queries []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queries = append(queries, r.URL.RawQuery)
		mu.Unlock()
		writeJSON(w, 200, map[string]interface{}{"success": true, "data": map[string]interface{}{"data": []interface{}{}, "total": 0}})
	})
	ctx := context.Background()
	c.SearchManga(ctx, "naruto", 3, 10)
	c.SearchMangaByGenre(ctx, "Slice of Life", 1, 20)
	mu.Lock()
	defer mu.Unlock()

	if !strings.Contains(queries[0], "limit=10") || !strings.Contains(queries[0], "offset=20") || !strings.Contains(queries[0], "q=naruto") {
		t.Errorf("search query = %q, want q, limit=10, offset=20", queries[0])
	}
	if !strings.Contains(queries[1], "genre=Slice+of+Life") || strings.Contains(queries[1], "q=") {
		t.Errorf("genre query = %q, want genre=... and no text search", queries[1])
	}
}

// Regression: registration returns a profile without a token; the TUI got a
// nil user and crashed. It now logs in after registering.
func TestRegisterLogsIn(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/register":
			writeJSON(w, 201, map[string]interface{}{"success": true, "data": map[string]string{"id": "u9", "username": "newbie"}})
		case "/auth/login":
			writeJSON(w, 200, map[string]interface{}{"success": true, "data": map[string]interface{}{
				"token": "tok", "user": map[string]string{"id": "u9", "username": "newbie"},
			}})
		}
	})
	u, err := c.Register(context.Background(), "newbie", "n@example.com", "password123")
	if err != nil || u == nil || u.Username != "newbie" || c.GetToken() != "tok" {
		t.Errorf("Register = %+v, %v, token %q", u, err, c.GetToken())
	}

	conflict := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 409, apiErr("CONFLICT", "username or email already exists"))
	})
	if _, err := conflict.Register(context.Background(), "x", "x@example.com", "password123"); err == nil ||
		!strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate register: %v", err)
	}
}

func TestUpdateProgressOmitsFavoriteWhenNil(t *testing.T) {
	var mu sync.Mutex
	var body map[string]interface{}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, 200, map[string]interface{}{"success": true})
	})
	c.UpdateProgress(context.Background(), "m1", 5, "reading", nil)
	mu.Lock()
	defer mu.Unlock()
	if _, sent := body["is_favorite"]; sent {
		t.Errorf("body %v includes is_favorite; the progress modal would clear favorites", body)
	}
}
