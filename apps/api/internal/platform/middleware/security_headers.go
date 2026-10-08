package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// apiCSP chặn mọi tài nguyên khi phản hồi API bị mở trực tiếp trên trình duyệt; ảnh chỉ từ cùng origin (ảnh markdown
// đi qua /api/v1/media/{id}/content) hoặc blob: (xem trước lúc upload). CSP đầy đủ của trang web do Caddy đặt.
const apiCSP = "default-src 'none'; img-src 'self' blob:; frame-ancestors 'none'"

// SecurityHeaders đặt header bảo mật áp cho mọi phản hồi của API; HSTS do reverse proxy (Caddy) đặt.
// Phản hồi dưới /api/ có Cache-Control: no-store vì chứa dữ liệu theo người dùng.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", apiCSP)
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		c.Next()
	}
}
