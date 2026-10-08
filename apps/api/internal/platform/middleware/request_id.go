// Package middleware chứa middleware gin dùng chung cho mọi route: request id, log truy cập, bắt panic,
// header bảo mật, giới hạn tần suất và IP client. Auth/CSRF thuộc feature identity vì cần bảng sessions.
package middleware

import (
	"context"
	"regexp"

	"github.com/gin-gonic/gin"

	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/ids"
)

// RequestIDHeader là header nhận và trả request id.
const RequestIDHeader = "X-Request-ID"

// requestIDPattern giới hạn request id do client gửi để không chèn được ký tự lạ vào log và header.
var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

type requestIDCtxKey struct{}

// RequestID giữ X-Request-ID hợp lệ của client hoặc sinh uuid v7, rồi đặt vào header phản hồi,
// gin.Context (httpx.RequestIDKey) và context của request (đọc bằng RequestIDFrom ở service/repository).
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !requestIDPattern.MatchString(id) {
			id = ids.New().String()
		}
		c.Set(httpx.RequestIDKey, id)
		c.Header(RequestIDHeader, id)
		c.Request = c.Request.WithContext(WithRequestID(c.Request.Context(), id))
		c.Next()
	}
}

// WithRequestID gắn request id vào ctx.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey{}, id)
}

// RequestIDFrom trả request id trong ctx, rỗng nếu không có (ví dụ job của worker). Dùng cho audit.Entry.RequestID
// và log của tầng dưới handler.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDCtxKey{}).(string)
	return id
}
