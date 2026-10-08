package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/httpx"
)

func init() { gin.SetMode(gin.TestMode) }

func newEngine(t *testing.T, mws ...gin.HandlerFunc) *gin.Engine {
	t.Helper()
	e := gin.New()
	require.NoError(t, TrustProxies(e, nil))
	e.Use(mws...)
	return e
}

func do(e http.Handler, method, target string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestRequestID(t *testing.T) {
	e := newEngine(t, RequestID())
	var fromCtx, fromGin string
	e.GET("/x", func(c *gin.Context) {
		fromCtx = RequestIDFrom(c.Request.Context())
		fromGin = c.GetString(httpx.RequestIDKey)
	})

	rec := do(e, http.MethodGet, "/x", map[string]string{RequestIDHeader: "abc-123"})
	require.Equal(t, "abc-123", rec.Header().Get(RequestIDHeader))
	require.Equal(t, "abc-123", fromCtx)
	require.Equal(t, "abc-123", fromGin)

	for _, bad := range []string{strings.Repeat("a", 65), "bad id\nINJECT", "a.b", "a_b", ""} {
		rec := do(e, http.MethodGet, "/x", map[string]string{RequestIDHeader: bad})
		got := rec.Header().Get(RequestIDHeader)
		id, err := uuid.Parse(got)
		require.NoError(t, err, "%q phải được thay bằng uuid", bad)
		require.Equal(t, uuid.Version(7), id.Version())
		require.Equal(t, got, fromCtx)
	}
	require.Len(t, do(e, http.MethodGet, "/x", map[string]string{RequestIDHeader: strings.Repeat("a", 64)}).Header().Get(RequestIDHeader), 64)
	require.Empty(t, RequestIDFrom(context.Background()))
}

func TestLoggerWritesOneJSONLineWithoutQuery(t *testing.T) {
	var logs bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&logs, nil))
	e := newEngine(t, RequestID(), Logger(l))
	e.GET("/items/:id", func(c *gin.Context) {
		c.Set(UserIDKey, "u-1")
		c.Status(http.StatusOK)
	})

	do(e, http.MethodGet, "/items/42?token=abc", map[string]string{RequestIDHeader: "req-7", "Cookie": "sid=s3cr3t"})
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	require.Len(t, lines, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &entry))
	require.Equal(t, "INFO", entry["level"])
	require.Equal(t, "GET", entry["method"])
	require.Equal(t, "/items/:id", entry["path"], "route pattern, không phải URL thật")
	require.InDelta(t, 200, entry["status"], 0)
	require.Equal(t, "req-7", entry["request_id"])
	require.Equal(t, "u-1", entry["user_id"])
	require.Equal(t, "192.0.2.1", entry["ip"])
	require.Contains(t, entry, "latency_ms")
	require.NotContains(t, logs.String(), "token")
	require.NotContains(t, logs.String(), "abc")
	require.NotContains(t, logs.String(), "s3cr3t")

	logs.Reset()
	do(e, http.MethodGet, "/khong-co?token=abc", nil)
	require.Contains(t, logs.String(), `"path":"/khong-co"`)
	require.Contains(t, logs.String(), `"status":404`)
	require.NotContains(t, logs.String(), "token")
	require.NotContains(t, logs.String(), "user_id")
}

func TestRecoverTurnsPanicIntoEnvelope(t *testing.T) {
	var logs bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&logs, nil))
	e := newEngine(t, RequestID(), Logger(l), Recover(l), SecurityHeaders())
	e.GET("/api/v1/boom", func(*gin.Context) { panic("boom") })
	e.GET("/api/v1/half", func(c *gin.Context) {
		c.String(http.StatusAccepted, "đã ghi")
		panic(errors.New("sau khi ghi"))
	})

	rec := do(e, http.MethodGet, "/api/v1/boom?token=s3cr3t", map[string]string{RequestIDHeader: "req-9"})
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"error":{"code":"INTERNAL","message":"Lỗi hệ thống, vui lòng thử lại"}}`, rec.Body.String())
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Contains(t, logs.String(), `"msg":"panic recovered"`)
	require.Contains(t, logs.String(), `"panic":"boom"`)
	require.Contains(t, logs.String(), `"stack":"goroutine`)
	require.Contains(t, logs.String(), `"request_id":"req-9"`)
	require.Contains(t, logs.String(), `"level":"ERROR","msg":"http request"`)
	require.NotContains(t, logs.String(), "s3cr3t")
	require.Equal(t, 1, strings.Count(logs.String(), "panic recovered"), "Fail không log lại")

	rec = do(e, http.MethodGet, "/api/v1/half", nil)
	require.Equal(t, http.StatusAccepted, rec.Code, "phản hồi đã ghi thì giữ nguyên")
	require.Equal(t, "đã ghi", rec.Body.String())
}

func TestRecoverRepanicsAbortHandler(t *testing.T) {
	l := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	e := newEngine(t, Recover(l))
	e.GET("/abort", func(*gin.Context) { panic(http.ErrAbortHandler) })
	require.PanicsWithError(t, http.ErrAbortHandler.Error(), func() { do(e, http.MethodGet, "/abort", nil) })
}

func TestSecurityHeaders(t *testing.T) {
	e := newEngine(t, SecurityHeaders())
	e.GET("/api/v1/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	e.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := do(e, http.MethodGet, "/api/v1/x", nil)
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	rec = do(e, http.MethodGet, "/healthz", nil)
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	require.Empty(t, rec.Header().Get("Cache-Control"))
}

func TestRateLimit(t *testing.T) {
	e := newEngine(t, RateLimit(ClientIP, 2, true))
	e.POST("/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	require.Equal(t, http.StatusOK, do(e, http.MethodPost, "/auth/login", nil).Code)
	require.Equal(t, http.StatusOK, do(e, http.MethodPost, "/auth/login", nil).Code)
	rec := do(e, http.MethodPost, "/auth/login", nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "30", rec.Header().Get("Retry-After"), "2/phút: token kế tiếp sau 30 giây")
	var body httpx.ErrorEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "RATE_LIMITED", body.Error.Code)
	require.Equal(t, "Bạn thao tác quá nhanh, thử lại sau", body.Error.Message)

	// IP khác có hạn mức riêng.
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "198.51.100.7:1234"
	other := httptest.NewRecorder()
	e.ServeHTTP(other, req)
	require.Equal(t, http.StatusOK, other.Code)
}

func TestRateLimitDisabledOrEmptyKeyPassesThrough(t *testing.T) {
	for name, mw := range map[string]gin.HandlerFunc{
		"disabled":  RateLimit(ClientIP, 2, false),
		"zero":      RateLimit(ClientIP, 0, true),
		"empty key": RateLimit(func(*gin.Context) string { return "" }, 2, true),
	} {
		t.Run(name, func(t *testing.T) {
			e := newEngine(t, mw)
			e.POST("/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })
			for range 3 {
				require.Equal(t, http.StatusOK, do(e, http.MethodPost, "/auth/login", nil).Code)
			}
		})
	}
}

type fakeNow struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeNow) now() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.t }
func (f *fakeNow) add(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

func TestRateLimiterRefillsAndSweeps(t *testing.T) {
	clk := &fakeNow{t: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)}
	rl := newRateLimiter(2, clk.now)

	_, ok := rl.allow("ip-a")
	require.True(t, ok)
	_, ok = rl.allow("ip-a")
	require.True(t, ok)
	wait, ok := rl.allow("ip-a")
	require.False(t, ok)
	require.Equal(t, 30*time.Second, wait)

	clk.add(10 * time.Second)
	wait, ok = rl.allow("ip-a")
	require.False(t, ok, "từ chối không tiêu token")
	require.Equal(t, 20*time.Second, wait)

	clk.add(20 * time.Second)
	_, ok = rl.allow("ip-a")
	require.True(t, ok, "sau 30 giây có lại một token")

	_, ok = rl.allow("ip-b")
	require.True(t, ok)
	clk.add(rateLimitIdle + time.Second)
	_, ok = rl.allow("ip-c") // kích hoạt dọn
	require.True(t, ok)
	_, aStill := rl.entries.Load("ip-a")
	_, bStill := rl.entries.Load("ip-b")
	_, cStill := rl.entries.Load("ip-c")
	require.False(t, aStill)
	require.False(t, bStill)
	require.True(t, cStill)

	require.Equal(t, 1, retryAfterSeconds(10*time.Millisecond))
	require.Equal(t, 2, retryAfterSeconds(1500*time.Millisecond))
}

func TestClientIPIgnoresForwardedForWithoutTrustedProxies(t *testing.T) {
	e := newEngine(t)
	var got string
	e.GET("/ip", func(c *gin.Context) { got = ClientIP(c) })

	do(e, http.MethodGet, "/ip", map[string]string{"X-Forwarded-For": "203.0.113.9", "X-Real-IP": "203.0.113.10"})
	require.Equal(t, "192.0.2.1", got, "không tin proxy: dùng RemoteAddr")

	require.NoError(t, TrustProxies(e, []string{"192.0.2.0/24"}))
	do(e, http.MethodGet, "/ip", map[string]string{"X-Forwarded-For": "203.0.113.9"})
	require.Equal(t, "203.0.113.9", got, "RemoteAddr thuộc CIDR tin cậy: đọc X-Forwarded-For")

	require.Error(t, TrustProxies(e, []string{"khong-phai-cidr"}))
	do(e, http.MethodGet, "/ip", map[string]string{"X-Forwarded-For": "203.0.113.9"})
	require.Equal(t, "192.0.2.1", got, "CIDR sai: quay về không tin proxy nào")
}
