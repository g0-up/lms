package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// TrustProxies cấu hình danh sách CIDR proxy được tin cho X-Forwarded-For (cfg.TrustedProxies). Danh sách rỗng
// nghĩa là không tin proxy nào và ClientIP là RemoteAddr; gin.New() mặc định tin mọi địa chỉ nên luôn phải gọi.
// CIDR sai trả lỗi và để engine ở trạng thái không tin proxy nào.
func TrustProxies(engine *gin.Engine, cidrs []string) error {
	if len(cidrs) == 0 {
		return engine.SetTrustedProxies(nil)
	}
	if err := engine.SetTrustedProxies(cidrs); err != nil {
		_ = engine.SetTrustedProxies(nil)
		return fmt.Errorf("TRUSTED_PROXIES không hợp lệ: %w", err)
	}
	return nil
}

// ClientIP là IP client theo cấu hình TrustProxies; dùng làm key của RateLimit theo IP.
func ClientIP(c *gin.Context) string {
	return c.ClientIP()
}
