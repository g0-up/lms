package media

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

// contentCacheControl cho trình duyệt giữ redirect ảnh markdown ngắn hơn hạn URL ký.
const contentCacheControl = "private, max-age=300"

// PrincipalFunc đọc người dùng đã đăng nhập từ context (middleware phiên đăng nhập gắn vào).
type PrincipalFunc func(*gin.Context) (Principal, bool)

// Handler là HTTP adapter của media; chỉ biết Service.
type Handler struct {
	svc       *Service
	principal PrincipalFunc
}

// NewHandler tạo handler.
func NewHandler(svc *Service, principal PrincipalFunc) *Handler {
	return &Handler{svc: svc, principal: principal}
}

// Register gắn route vào nhóm /media đã qua xác thực và limiter; upload thêm requireAdmin. Xem URL mở cho mọi vai
// trò, quyền kiểm ở service.
func (h *Handler) Register(rg *gin.RouterGroup, requireAdmin gin.HandlerFunc) {
	rg.POST("/uploads", requireAdmin, h.initUpload)
	rg.POST("/uploads/:id/complete", requireAdmin, h.completeUpload)
	rg.GET("/:id/url", h.signedURL)
	rg.GET("/:id/content", h.content)
}

func (h *Handler) initUpload(c *gin.Context) {
	p, ok := h.mustPrincipal(c)
	if !ok {
		return
	}
	var req initUploadRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.InitUpload(c.Request.Context(), p, InitUploadCmd(req))
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, ticketDTO(t))
}

func (h *Handler) completeUpload(c *gin.Context) {
	p, ok := h.mustPrincipal(c)
	if !ok {
		return
	}
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	m, err := h.svc.CompleteUpload(c.Request.Context(), p, id)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, ToDTO(m))
}

func (h *Handler) signedURL(c *gin.Context) {
	s, ok := h.sign(c)
	if !ok {
		return
	}
	httpx.OK(c, http.StatusOK, signedURLDTO(s))
}

// content chuyển hướng tới URL ký; ảnh markdown trỏ vào đây nên src trong HTML không bao giờ chứa chữ ký.
func (h *Handler) content(c *gin.Context) {
	s, ok := h.sign(c)
	if !ok {
		return
	}
	c.Header("Cache-Control", contentCacheControl)
	c.Redirect(http.StatusFound, s.URL)
}

func (h *Handler) sign(c *gin.Context) (SignedURL, bool) {
	p, ok := h.mustPrincipal(c)
	if !ok {
		return SignedURL{}, false
	}
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return SignedURL{}, false
	}
	s, err := h.svc.SignedURL(c.Request.Context(), p, id)
	if err != nil {
		fail(c, err)
		return SignedURL{}, false
	}
	return s, true
}

func (h *Handler) mustPrincipal(c *gin.Context) (Principal, bool) {
	p, ok := h.principal(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return Principal{}, false
	}
	return p, true
}

// fail trả lỗi theo envelope; lỗi nghiệp vụ dữ liệu sai là 422 theo hợp đồng API (lỗi cú pháp JSON vẫn 400).
func fail(c *gin.Context, err error) {
	var de *domain.Error
	if errors.As(err, &de) && de.Kind == domain.KindInvalid {
		httpx.Fail(c, apperr.Wrap(err, http.StatusUnprocessableEntity, apperr.CodeValidationFailed, de.Msg))
		return
	}
	httpx.Fail(c, err)
}
