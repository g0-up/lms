package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
)

func newTestEngine(t *testing.T, dbx *sqlx.DB) (*gin.Engine, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	deps := Deps{DB: dbx, Clock: &clock.Fake{T: time.Now()}, Audit: audit.Noop{}, Logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	return New(deps), &logs
}

func serve(engine http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	engine, _ := newTestEngine(t, nil)
	rec := serve(engine, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Empty(t, rec.Header().Get("Cache-Control"), "no-store chỉ áp cho /api/*")
}

func TestAPIHealthzForProxyCheck(t *testing.T) {
	engine, _ := newTestEngine(t, nil)
	rec := serve(engine, httptest.NewRequest(http.MethodGet, "/api/healthz", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestReadyzUnavailableWhenDatabaseDown(t *testing.T) {
	sqlDB, err := sql.Open("pgx", "postgres://lms:lms@127.0.0.1:1/lms?sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	engine, logs := newTestEngine(t, sqlx.NewDb(sqlDB, "pgx"))
	rec := serve(engine, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"status":"unavailable"}`, rec.Body.String())
	assert.Contains(t, logs.String(), "readyz")
}

func TestRequestIDEchoedOrGenerated(t *testing.T) {
	engine, _ := newTestEngine(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "abc-123")
	assert.Equal(t, "abc-123", serve(engine, req).Header().Get("X-Request-ID"))

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "bad id\nINJECT")
	got := serve(engine, req).Header().Get("X-Request-ID")
	assert.NotContains(t, got, "INJECT")
	assert.Len(t, got, 36, "thay bằng uuid")

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", strings.Repeat("a", 65))
	assert.Len(t, serve(engine, req).Header().Get("X-Request-ID"), 36, "quá 64 ký tự thì sinh mới")
}

func TestRecoverReturnsEnvelopeAndAPINoStore(t *testing.T) {
	engine, logs := newTestEngine(t, nil)
	engine.GET("/api/v1/boom", func(*gin.Context) { panic("boom") })

	rec := serve(engine, httptest.NewRequest(http.MethodGet, "/api/v1/boom?token=s3cr3t", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	var body map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "INTERNAL", body["error"]["code"])
	assert.Equal(t, "Lỗi hệ thống, vui lòng thử lại", body["error"]["message"])
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	assert.True(t, strings.Contains(logs.String(), `"path":"/api/v1/boom"`))
	assert.NotContains(t, logs.String(), "s3cr3t", "log không chứa query")
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body struct {
		Error map[string]any `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Error
}

func TestUnknownRouteReturnsNotFoundEnvelope(t *testing.T) {
	engine, _ := newTestEngine(t, nil)
	rec := serve(engine, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	body := decodeEnvelope(t, rec)
	assert.Equal(t, "NOT_FOUND", body["code"])
	assert.Equal(t, "Không tìm thấy đường dẫn", body["message"])
	assert.Len(t, rec.Header().Get("X-Request-ID"), 36)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestWrongMethodReturnsMethodNotAllowedEnvelope(t *testing.T) {
	engine, _ := newTestEngine(t, nil)
	rec := serve(engine, httptest.NewRequest(http.MethodPost, "/healthz", nil))

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	body := decodeEnvelope(t, rec)
	assert.Equal(t, "METHOD_NOT_ALLOWED", body["code"])
	assert.Equal(t, "Phương thức không được hỗ trợ", body["message"])
	assert.Equal(t, "GET", rec.Header().Get("Allow"))
}

func TestInvalidTrustedProxiesFallsBackToDirectClientIP(t *testing.T) {
	var logs bytes.Buffer
	engine := New(Deps{
		Cfg:    config.Config{TrustedProxies: []string{"not-a-cidr"}},
		Clock:  &clock.Fake{T: time.Now()},
		Audit:  audit.Noop{},
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
	})
	var ip string
	engine.GET("/ip", func(c *gin.Context) { ip = c.ClientIP() })

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "203.0.113.7:4444"
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	serve(engine, req)

	assert.Equal(t, "203.0.113.7", ip)
	assert.Contains(t, logs.String(), "TRUSTED_PROXIES")
}

func TestLearningRoutesRequireSession(t *testing.T) {
	engine, _ := newTestEngine(t, nil)
	const id = "01990000-0000-7000-8000-000000000301"
	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/me/classes"},
		{http.MethodGet, "/api/v1/me/classes/" + id},
		{http.MethodGet, "/api/v1/me/classes/" + id + "/lessons/" + id},
		{http.MethodPut, "/api/v1/me/classes/" + id + "/lessons/" + id + "/completion"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"completed":true}`))
		req.Header.Set("X-Requested-With", "fetch")
		rec := serve(engine, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, tc.path)
		assert.Equal(t, "UNAUTHENTICATED", decodeEnvelope(t, rec)["code"], tc.path)
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"), tc.path)
	}
}

func TestStagesPreviewRequiresSession(t *testing.T) {
	engine, _ := newTestEngine(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stages/markdown-preview", strings.NewReader(`{"markdownSource":"# A"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "fetch")
	rec := serve(engine, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "UNAUTHENTICATED", decodeEnvelope(t, rec)["code"])
}

func TestReportRoutesRequireSession(t *testing.T) {
	engine, _ := newTestEngine(t, nil)
	const id = "01990000-0000-7000-8000-000000000301"
	for _, path := range []string{
		"/api/v1/classes/" + id + "/report",
		"/api/v1/classes/" + id + "/report/members/" + id,
		"/api/v1/dashboard",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Requested-With", "fetch")
		rec := serve(engine, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, path)
		assert.Equal(t, "UNAUTHENTICATED", decodeEnvelope(t, rec)["code"], path)
	}
}
