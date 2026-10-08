package middleware

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeadersCSP(t *testing.T) {
	e := newEngine(t, SecurityHeaders())
	e.GET("/api/v1/media/:id/content", func(c *gin.Context) { c.Status(http.StatusFound) })
	e.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, path := range []string{"/api/v1/media/x/content", "/healthz"} {
		csp := do(e, http.MethodGet, path, nil).Header().Get("Content-Security-Policy")
		require.Equal(t, "default-src 'none'; img-src 'self' blob:; frame-ancestors 'none'", csp, path)
		require.NotContains(t, strings.ToLower(csp), "https:", "ảnh ngoài không được phép")
	}
}
