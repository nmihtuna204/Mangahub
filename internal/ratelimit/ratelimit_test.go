package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestBurstThenThrottle(t *testing.T) {
	l := New(1, 3) // 1/s, bursts of 3
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("request %d within the burst was rejected", i+1)
		}
	}
	ok, retry := l.Allow("a")
	if ok || retry <= 0 || retry > time.Second {
		t.Errorf("4th request: allowed %v, retry %v; want rejected with retry ≤ 1s", ok, retry)
	}
	// Rejected requests don't use up tokens, and other clients are independent
	if ok, _ := l.Allow("b"); !ok {
		t.Error("another client was throttled")
	}
	time.Sleep(1100 * time.Millisecond)
	if ok, _ := l.Allow("a"); !ok {
		t.Error("no token after waiting the retry time")
	}
}

func TestDisabled(t *testing.T) {
	l := New(0, 0)
	for i := 0; i < 1000; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatal("a disabled limiter rejected a request")
		}
	}
	if PerMinute(0, 5).Enabled() {
		t.Error("PerMinute(0) should be disabled")
	}
}

// Only failed responses use up the budget: correct logins are never limited,
// wrong passwords are.
func TestFailureMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(PerMinute(60, 2).FailureMiddleware(func(status int) bool { return status == http.StatusUnauthorized }))
	r.POST("/login", func(c *gin.Context) {
		if c.Query("pw") == "right" {
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusUnauthorized)
	})
	login := func(pw string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login?pw="+pw, nil)
		req.RemoteAddr = "10.0.0.2:1234"
		r.ServeHTTP(w, req)
		return w
	}

	for i := 0; i < 20; i++ {
		if w := login("right"); w.Code != http.StatusOK {
			t.Fatalf("correct login %d got %d; successful logins must not be limited", i+1, w.Code)
		}
	}
	for i := 0; i < 2; i++ {
		if code := login("wrong").Code; code != 401 {
			t.Fatalf("wrong password %d got %d; the first two should reach the handler", i+1, code)
		}
	}
	w := login("right")
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Errorf("after 2 failures (burst 2): %d, Retry-After %q; want 429 until the budget refills", w.Code, w.Header().Get("Retry-After"))
	}
}

func TestMiddlewareResponds429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(New(1, 1).Middleware())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	do := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		r.ServeHTTP(w, req)
		return w
	}
	if w := do(); w.Code != http.StatusOK {
		t.Fatalf("first request: %d", w.Code)
	}
	w := do()
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") != "1" || body.Error.Code != "RATE_LIMITED" {
		t.Errorf("second request: %d, Retry-After %q, code %q", w.Code, w.Header().Get("Retry-After"), body.Error.Code)
	}
}
