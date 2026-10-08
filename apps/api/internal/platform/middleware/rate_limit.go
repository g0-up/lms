package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

// rateLimitIdle là thời gian không dùng sau đó entry của một key bị dọn; cũng là chu kỳ dọn.
const rateLimitIdle = 10 * time.Minute

// RateLimit giới hạn perMin request mỗi phút cho mỗi key (burst = perMin), ví dụ RateLimit(ClientIP,
// cfg.RateLimitLoginIPPerMin, !cfg.IsE2E()). Vượt giới hạn trả 429 RATE_LIMITED kèm Retry-After (giây).
// enabled=false, perMin <= 0 hoặc key rỗng thì cho qua. Trạng thái nằm trong bộ nhớ tiến trình: đúng cho một
// replica; chạy nhiều replica thì mỗi replica đếm riêng.
func RateLimit(key func(*gin.Context) string, perMin int, enabled bool) gin.HandlerFunc {
	if !enabled || perMin <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	rl := newRateLimiter(perMin, time.Now)
	return func(c *gin.Context) {
		k := key(c)
		if k == "" {
			c.Next()
			return
		}
		if wait, ok := rl.allow(k); !ok {
			e := apperr.New(http.StatusTooManyRequests, apperr.CodeRateLimited, apperr.Message(apperr.CodeRateLimited))
			e.Details = map[string]string{"retry_after": strconv.Itoa(retryAfterSeconds(wait))}
			httpx.Fail(c, e)
			return
		}
		c.Next()
	}
}

type limiterEntry struct {
	lim      *rate.Limiter
	lastSeen atomic.Int64 // unix nano
}

type rateLimiter struct {
	perMin  int
	now     func() time.Time
	entries sync.Map // key -> *limiterEntry

	sweepMu   sync.Mutex
	lastSweep time.Time
}

func newRateLimiter(perMin int, now func() time.Time) *rateLimiter {
	return &rateLimiter{perMin: perMin, now: now, lastSweep: now()}
}

// allow lấy một token cho key; khi hết token trả thời gian phải chờ tới token kế tiếp.
func (r *rateLimiter) allow(key string) (time.Duration, bool) {
	now := r.now()
	r.sweep(now)

	v, ok := r.entries.Load(key)
	if !ok {
		v, _ = r.entries.LoadOrStore(key, &limiterEntry{lim: rate.NewLimiter(rate.Limit(float64(r.perMin)/60), r.perMin)})
	}
	e := v.(*limiterEntry) //nolint:forcetypeassert // map chỉ chứa *limiterEntry
	e.lastSeen.Store(now.UnixNano())

	res := e.lim.ReserveN(now, 1)
	if delay := res.DelayFrom(now); delay > 0 {
		res.CancelAt(now)
		return delay, false
	}
	return 0, true
}

// sweep xóa entry không dùng quá rateLimitIdle, tối đa một lần mỗi rateLimitIdle, chạy trong request nên không cần
// goroutine nền.
func (r *rateLimiter) sweep(now time.Time) {
	r.sweepMu.Lock()
	if now.Sub(r.lastSweep) < rateLimitIdle {
		r.sweepMu.Unlock()
		return
	}
	r.lastSweep = now
	r.sweepMu.Unlock()

	cutoff := now.Add(-rateLimitIdle).UnixNano()
	r.entries.Range(func(k, v any) bool {
		if v.(*limiterEntry).lastSeen.Load() < cutoff { //nolint:forcetypeassert // map chỉ chứa *limiterEntry
			r.entries.Delete(k)
		}
		return true
	})
}

func retryAfterSeconds(wait time.Duration) int {
	return max(1, int(math.Ceil(wait.Seconds())))
}
