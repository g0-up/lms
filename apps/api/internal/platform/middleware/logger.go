package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"lms/api/internal/platform/httpx"
)

// UserIDKey là khóa gin.Context mà middleware xác thực đặt id người dùng (uuid.UUID hoặc string); Logger ghi nó
// thành user_id khi có.
const UserIDKey = "user_id"

// Logger ghi một dòng log mỗi request: method, path (route pattern; URL path khi không khớp route), status,
// latency_ms, request_id, user_id, ip. Không bao giờ ghi query string, body, header hay cookie vì có thể chứa
// token hoặc mật khẩu. Status >= 500 ghi mức ERROR.
func Logger(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
			slog.String("request_id", c.GetString(httpx.RequestIDKey)),
			slog.String("ip", c.ClientIP()),
		}
		if uid, ok := c.Get(UserIDKey); ok {
			attrs = append(attrs, slog.Any("user_id", uid))
		}
		l.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}
