package identity

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/middleware"
)

// forgotMessage là phản hồi chung của quên mật khẩu, giống nhau dù email có tồn tại hay không.
const forgotMessage = "Nếu email tồn tại, chúng tôi đã gửi hướng dẫn đặt lại mật khẩu."

// Handler là HTTP adapter của identity; không biết sqlx.
type Handler struct {
	svc *Service
	cfg config.Config
}

// NewHandler tạo Handler.
func NewHandler(svc *Service, cfg config.Config) *Handler {
	return &Handler{svc: svc, cfg: cfg}
}

// Register gắn route /auth/* và /users/* vào r (nhóm /api/v1). Limiter IP tắt khi APP_ENV=e2e.
func (h *Handler) Register(r gin.IRouter, auth *Middleware) {
	enabled := !h.cfg.IsE2E()
	loginLimit := middleware.RateLimit(middleware.ClientIP, h.cfg.RateLimitLoginIPPerMin, enabled)
	forgotLimit := middleware.RateLimit(middleware.ClientIP, h.cfg.RateLimitForgotIPPerMin, enabled)

	a := r.Group("/auth")
	a.POST("/login", loginLimit, h.login)
	a.POST("/forgot-password", forgotLimit, h.forgotPassword)
	a.POST("/reset-password", forgotLimit, h.resetPassword)
	// Đăng xuất luôn 204 kể cả khi phiên đã hết, nên không đặt sau SessionAuth.
	a.POST("/logout", h.logout)

	authed := a.Group("", auth.SessionAuth(), auth.MustChangePassword())
	authed.GET("/me", h.me)
	authed.POST("/change-password", h.changePassword)

	admin := r.Group("/users", auth.SessionAuth(), auth.MustChangePassword(), RequireRole(domain.RoleAdmin))
	admin.GET("", h.listUsers)
	admin.POST("/:id/disable", h.disableUser)
	admin.POST("/:id/enable", h.enableUser)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"` //nolint:gosec // body đăng nhập, không log
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	res, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, middleware.ClientIP(c), c.Request.UserAgent())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	h.setSessionCookie(c, res.Token.Reveal())
	httpx.OK(c, http.StatusOK, loginResponse{User: NewMeDTO(res.User)})
}

func (h *Handler) logout(c *gin.Context) {
	token, _ := c.Cookie(CookieName(h.cfg.CookieSecure))
	if err := h.svc.Logout(c.Request.Context(), token); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.clearSessionCookie(c)
	httpx.NoContent(c)
}

func (h *Handler) me(c *gin.Context) {
	u, ok := CurrentUser(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return
	}
	httpx.OK(c, http.StatusOK, NewMeDTO(u))
}

type changePasswordRequest struct {
	CurrentPassword *string `json:"currentPassword"`
	NewPassword     string  `json:"newPassword"`
	ConfirmPassword string  `json:"confirmPassword"`
}

func (h *Handler) changePassword(c *gin.Context) {
	u, ok := CurrentUser(c)
	sessionID, sok := CurrentSessionID(c)
	if !ok || !sok {
		httpx.Fail(c, apperr.Unauthenticated())
		return
	}
	var req changePasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	out, err := h.svc.ChangePassword(c.Request.Context(), u, sessionID, ChangePasswordCmd(req))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, NewMeDTO(out))
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (h *Handler) forgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	// Email sai định dạng vẫn 202 để phản hồi không khác nhau.
	if err := h.svc.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusAccepted, messageResponse{Message: forgotMessage})
}

type resetPasswordRequest struct {
	Token           string `json:"token"` //nolint:gosec // token trong body, không log
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

func (h *Handler) resetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), ResetPasswordCmd(req)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) listUsers(c *gin.Context) {
	role, err := domain.ParseRole(c.Query("role"))
	if err != nil {
		httpx.Fail(c, apperr.Validation(map[string]string{"role": "Vai trò không hợp lệ"}))
		return
	}
	var status *domain.UserStatus
	if raw := c.Query("status"); raw != "" {
		st, err := domain.ParseUserStatus(raw)
		if err != nil {
			httpx.Fail(c, apperr.Validation(map[string]string{"status": "Trạng thái không hợp lệ"}))
			return
		}
		status = &st
	}
	users, err := h.svc.ListUsers(c.Request.Context(), role, status)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	items := make([]UserDTO, 0, len(users))
	for _, u := range users {
		items = append(items, NewUserDTO(u))
	}
	httpx.OK(c, http.StatusOK, listUsersResponse{Items: items})
}

func (h *Handler) disableUser(c *gin.Context) { h.setEnabled(c, false) }

func (h *Handler) enableUser(c *gin.Context) { h.setEnabled(c, true) }

func (h *Handler) setEnabled(c *gin.Context, enable bool) {
	actor, ok := CurrentUser(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return
	}
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	ctx := c.Request.Context()
	reqID := middleware.RequestIDFrom(ctx)
	var u *User
	if enable {
		u, err = h.svc.EnableUser(ctx, actor, id, reqID)
	} else {
		u, err = h.svc.DisableUser(ctx, actor, id, reqID)
	}
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, NewUserDTO(u))
}

// setSessionCookie đặt cookie phiên không có Max-Age (cookie phiên trình duyệt); hạn thật nằm ở DB và được gia
// hạn trượt mỗi request. Secure theo COOKIE_SECURE vì dev chạy http; production bắt buộc true (config kiểm).
func (h *Handler) setSessionCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{ //nolint:gosec // Secure lấy từ COOKIE_SECURE, xem chú thích trên
		Name: CookieName(h.cfg.CookieSecure), Value: token, Path: "/",
		HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{ //nolint:gosec // Secure lấy từ COOKIE_SECURE như setSessionCookie
		Name: CookieName(h.cfg.CookieSecure), Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}
