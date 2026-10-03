// Package ratelimit - Per-client request rate limiting
// Token bucket per key (the client IP), with idle keys forgotten so memory
// stays bounded.
package ratelimit

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"mangahub/pkg/models"
)

// idleTTL is how long a client's bucket is kept after its last request
const idleTTL = 10 * time.Minute

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter allows each key `perSecond` requests per second with bursts of `burst`.
type Limiter struct {
	rate      rate.Limit
	burst     int
	mu        sync.Mutex
	visitors  map[string]*visitor
	lastSweep time.Time
}

// New creates a limiter. perSecond <= 0 means unlimited (Allow always succeeds).
func New(perSecond float64, burst int) *Limiter {
	if burst < 1 {
		burst = 1
	}
	return &Limiter{
		rate:      rate.Limit(perSecond),
		burst:     burst,
		visitors:  make(map[string]*visitor),
		lastSweep: time.Now(),
	}
}

// PerMinute is a convenience for limits stated per minute (e.g. logins).
func PerMinute(n float64, burst int) *Limiter {
	return New(n/60, burst)
}

// Enabled reports whether the limiter restricts anything.
func (l *Limiter) Enabled() bool {
	return l.rate > 0
}

// bucket returns key's token bucket, creating it on first use, and forgets
// buckets that have been idle for a while.
func (l *Limiter) bucket(key string, now time.Time) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.visitors[key] = v
	}
	v.lastSeen = now
	if now.Sub(l.lastSweep) > time.Minute {
		for k, other := range l.visitors {
			if now.Sub(other.lastSeen) > idleTTL {
				delete(l.visitors, k)
			}
		}
		l.lastSweep = now
	}
	return v.limiter
}

// Allow reports whether key may make a request now; if not, also how long
// until it may.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if !l.Enabled() {
		return true, 0
	}
	now := time.Now()
	r := l.bucket(key, now).ReserveN(now, 1)
	if delay := r.DelayFrom(now); delay > 0 {
		r.CancelAt(now) // don't consume a token for a rejected request
		return false, delay
	}
	return true, 0
}

// Middleware rejects requests over the limit with 429 and a Retry-After
// header, keyed by client IP.
func (l *Limiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if ok, retry := l.Allow(c.ClientIP()); !ok {
			reject(c, retry, "too many requests, please slow down")
			return
		}
		c.Next()
	}
}

// FailureMiddleware limits only failed requests: a request is rejected while
// the client has used up its budget, but only responses for which failed
// returns true use up the budget. For logins this stops password guessing
// without locking out people who log in correctly (all local clients share
// one IP, so counting every login locked out a demo after 10 logins).
func (l *Limiter) FailureMiddleware(failed func(status int) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Enabled() {
			c.Next()
			return
		}
		key, now := c.ClientIP(), time.Now()
		b := l.bucket(key, now)
		if tokens := b.TokensAt(now); tokens < 1 {
			wait := time.Duration((1 - tokens) / float64(l.rate) * float64(time.Second))
			reject(c, wait, "too many failed attempts, please wait")
			return
		}
		c.Next()
		if failed(c.Writer.Status()) {
			b.AllowN(time.Now(), 1)
		}
	}
}

func reject(c *gin.Context, retry time.Duration, message string) {
	secs := int(math.Ceil(retry.Seconds()))
	if secs < 1 {
		secs = 1
	}
	c.Header("Retry-After", strconv.Itoa(secs))
	c.AbortWithStatusJSON(http.StatusTooManyRequests,
		models.NewErrorResponse(models.ErrCodeRateLimited, message, nil))
}
