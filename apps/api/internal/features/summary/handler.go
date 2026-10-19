package summary

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

// ActorFunc đọc id và vai trò người dùng đã đăng nhập từ context (middleware phiên đăng nhập gắn vào).
type ActorFunc func(*gin.Context) (uuid.UUID, domain.Role, bool)

// Handler là HTTP adapter của summary; chỉ biết Service.
type Handler struct {
	svc   *Service
	actor ActorFunc
}

// NewHandler tạo handler.
func NewHandler(svc *Service, actor ActorFunc) *Handler {
	return &Handler{svc: svc, actor: actor}
}

// Register gắn route tổng kết vào rg (rg đã qua xác thực phiên): student cho tổng kết của chính học viên, staff
// (admin và giảng viên; phạm vi lớp kiểm ở service) cho ma trận lớp, drawer và nhận xét. Phản hồi chứa dữ liệu cá
// nhân học viên nên không cache.
func (h *Handler) Register(rg *gin.RouterGroup, student, staff gin.HandlerFunc) {
	rg.GET("/me/classes/:id/summary", noStore, student, h.learnerSummary)
	rg.GET("/classes/:id/summary", noStore, staff, h.classMatrix)
	rg.GET("/classes/:id/members/:mid/stages/:svid/summary", noStore, staff, h.memberStage)
	rg.PUT("/classes/:id/members/:mid/stages/:svid/review", staff, h.saveReview)
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Next()
}

func (h *Handler) learnerSummary(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	classID, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	v, err := h.svc.LearnerSummary(c.Request.Context(), actor.ID, classID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toLearnerSummaryDTO(v))
}

func (h *Handler) classMatrix(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	classID, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	v, err := h.svc.ClassMatrix(c.Request.Context(), actor, classID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toClassMatrixDTO(v))
}

func (h *Handler) memberStage(c *gin.Context) {
	actor, classID, memberID, stageVersionID, ok := h.memberStageParams(c)
	if !ok {
		return
	}
	v, err := h.svc.MemberStage(c.Request.Context(), actor, classID, memberID, stageVersionID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toMemberStageDTO(v))
}

func (h *Handler) saveReview(c *gin.Context) {
	actor, classID, memberID, stageVersionID, ok := h.memberStageParams(c)
	if !ok {
		return
	}
	var req reviewRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	r, err := h.svc.SaveReview(c.Request.Context(), actor, classID, memberID, stageVersionID, req.Body)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toReviewDTO(r))
}

func (h *Handler) memberStageParams(c *gin.Context) (Actor, uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	actor, ok := h.mustActor(c)
	if !ok {
		return Actor{}, uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	ids := [3]uuid.UUID{}
	for i, name := range [3]string{"id", "mid", "svid"} {
		if ids[i], ok = uuidParam(c, name); !ok {
			return Actor{}, uuid.Nil, uuid.Nil, uuid.Nil, false
		}
	}
	return actor, ids[0], ids[1], ids[2], true
}

func (h *Handler) mustActor(c *gin.Context) (Actor, bool) {
	id, role, ok := h.actor(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return Actor{}, false
	}
	return Actor{ID: id, Role: role, RequestID: c.GetString(httpx.RequestIDKey)}, true
}

// fail: lỗi validate nghiệp vụ thành 422 VALIDATION_FAILED, còn lại để httpx ánh xạ theo loại lỗi.
func fail(c *gin.Context, err error) {
	var de *domain.Error
	if errors.As(err, &de) && de.Kind == domain.KindInvalid {
		httpx.Fail(c, apperr.Wrap(err, http.StatusUnprocessableEntity, apperr.CodeValidationFailed, de.Msg))
		return
	}
	httpx.Fail(c, err)
}

func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := httpx.UUIDParam(c, name)
	if err != nil {
		httpx.Fail(c, err)
		return uuid.Nil, false
	}
	return id, true
}
