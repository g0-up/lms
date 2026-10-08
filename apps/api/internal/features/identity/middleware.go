package identity

import (
	"errors"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/middleware"
)

// Khóa gin.Context do SessionAuth đặt.
const (
	ctxUserKey      = "identity.user"
	ctxSessionIDKey = "identity.session_id"
)

// Tên cookie phiên: __Host-sid bắt buộc Secure nên dev http dùng sid.
const (
	cookieNameSecure   = "__Host-sid"
	cookieNameInsecure = "sid"
)

// passwordChangeAllowlist là các route (method, full path) dùng được khi còn phải đổi mật khẩu tạm.
var passwordChangeAllowlist = map[string]struct{}{
	http.MethodPost + " /api/v1/auth/change-password": {},
	http.MethodPost + " /api/v1/auth/logout":          {},
	http.MethodGet + " /api/v1/auth/me":               {},
}

// Middleware là chuỗi xác thực cho route: SessionAuth → MustChangePassword → RequireRole.
type Middleware struct {
	svc    *Service
	cookie string
}

// NewMiddleware tạo Middleware; secureCookie chọn tên cookie (COOKIE_SECURE).
func NewMiddleware(svc *Service, secureCookie bool) *Middleware {
	return &Middleware{svc: svc, cookie: CookieName(secureCookie)}
}

// CookieName là tên cookie phiên theo COOKIE_SECURE.
func CookieName(secure bool) string {
	if secure {
		return cookieNameSecure
	}
	return cookieNameInsecure
}

// SessionAuth đọc cookie phiên, gắn user và session id vào context; thiếu, sai, hết hạn hoặc user bị vô hiệu
// hóa → 401 UNAUTHENTICATED.
func (m *Middleware) SessionAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.Cookie(m.cookie)
		sess, u, err := m.svc.Authenticate(c.Request.Context(), token)
		if errors.Is(err, ErrSessionNotFound) {
			httpx.Fail(c, apperr.Unauthenticated())
			return
		}
		if err != nil {
			httpx.Fail(c, err)
			return
		}
		c.Set(ctxUserKey, u)
		c.Set(ctxSessionIDKey, sess.ID)
		c.Set(middleware.UserIDKey, u.ID())
		c.Next()
	}
}

// MustChangePassword chặn mọi route ngoài allowlist bằng 403 PASSWORD_CHANGE_REQUIRED khi user còn phải đổi
// mật khẩu tạm. Đặt sau SessionAuth.
func (m *Middleware) MustChangePassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := CurrentUser(c)
		if !ok {
			httpx.Fail(c, apperr.Unauthenticated())
			return
		}
		if u.MustChangePassword() {
			if _, allowed := passwordChangeAllowlist[c.Request.Method+" "+c.FullPath()]; !allowed {
				httpx.Fail(c, ErrPasswordChange)
				return
			}
		}
		c.Next()
	}
}

// RequireRole chỉ cho user có một trong roles; khác → 403 FORBIDDEN. Đặt sau SessionAuth.
func RequireRole(roles ...domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := CurrentUser(c)
		if !ok {
			httpx.Fail(c, apperr.Unauthenticated())
			return
		}
		if !slices.Contains(roles, u.Role()) {
			httpx.Fail(c, apperr.Forbidden())
			return
		}
		c.Next()
	}
}

// CurrentUser trả user do SessionAuth gắn vào context.
func CurrentUser(c *gin.Context) (*User, bool) {
	v, ok := c.Get(ctxUserKey)
	if !ok {
		return nil, false
	}
	u, ok := v.(*User)
	return u, ok && u != nil
}

// CurrentSessionID trả id phiên do SessionAuth gắn vào context.
func CurrentSessionID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ctxSessionIDKey)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}
