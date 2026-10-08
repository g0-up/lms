package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

func csrfEngine() *gin.Engine {
	r := gin.New()
	api := r.Group("/api/v1", httpx.RequireCustomHeader())
	ok := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	api.GET("/x", ok)
	api.POST("/x", ok)
	api.PUT("/x", ok)
	api.PATCH("/x", ok)
	api.DELETE("/x", ok)
	r.GET("/healthz", ok)
	return r
}

func requireForbidden(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusForbidden, rec.Code)
	var env httpx.ErrorEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Equal(t, apperr.CodeForbidden, env.Error.Code)
	require.NotEmpty(t, env.Error.Message)
}

func TestRequireCustomHeader(t *testing.T) {
	r := csrfEngine()
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(m, "/api/v1/x", nil))
		requireForbidden(t, rec)

		rec = httptest.NewRecorder()
		req := httptest.NewRequest(m, "/api/v1/x", nil)
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		r.ServeHTTP(rec, req)
		requireForbidden(t, rec)

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(m, "/api/v1/x", nil)
		req.Header.Set("X-Requested-With", "fetch")
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNoContent, rec.Code, m)
	}
	for _, p := range []string{"/api/v1/x", "/healthz"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		require.Equal(t, http.StatusNoContent, rec.Code, p)
	}
}

func TestCrossOriginProtection(t *testing.T) {
	h, err := httpx.CrossOriginProtection(csrfEngine(), "http://localhost:5173/", "")
	require.NoError(t, err)

	send := func(method, origin, site string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://localhost:8080/api/v1/x", nil)
		req.Header.Set("X-Requested-With", "fetch")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if site != "" {
			req.Header.Set("Sec-Fetch-Site", site)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	requireForbidden(t, send(http.MethodPost, "https://evil.example", ""))
	requireForbidden(t, send(http.MethodPost, "https://evil.example", "cross-site"))
	require.Equal(t, http.StatusNoContent, send(http.MethodGet, "https://evil.example", "cross-site").Code, "GET an toàn")
	require.Equal(t, http.StatusNoContent, send(http.MethodPost, "", "same-origin").Code)
	require.Equal(t, http.StatusNoContent, send(http.MethodPost, "", "").Code, "client không phải trình duyệt")
	require.Equal(t, http.StatusNoContent, send(http.MethodPost, "http://localhost:5173", "cross-site").Code, "origin tin cậy")

	_, err = httpx.CrossOriginProtection(csrfEngine(), "localhost:5173")
	require.Error(t, err)
}
