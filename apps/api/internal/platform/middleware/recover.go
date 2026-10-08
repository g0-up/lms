package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

// Recover bắt panic của handler phía sau: log ERROR kèm stack và request_id, trả 500 INTERNAL theo envelope
// (nếu chưa ghi phản hồi) rồi dừng chuỗi handler. http.ErrAbortHandler được panic lại để net/http hủy kết nối
// như thiết kế.
func Recover(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			l.ErrorContext(c.Request.Context(), "panic recovered",
				slog.String("panic", fmt.Sprint(rec)),
				slog.String("stack", string(debug.Stack())),
				slog.String("request_id", c.GetString(httpx.RequestIDKey)),
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
			)
			if c.Writer.Written() {
				c.Abort()
				return
			}
			// Đã log ở trên kèm stack; Internal(nil) để Fail không log lần nữa.
			httpx.Fail(c, apperr.Internal(nil))
		}()
		c.Next()
	}
}
