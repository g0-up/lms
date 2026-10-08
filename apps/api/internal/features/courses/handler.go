package courses

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

// ActorFunc đọc id người dùng đã đăng nhập từ context (middleware phiên đăng nhập gắn vào).
type ActorFunc func(*gin.Context) (uuid.UUID, bool)

// Handler là HTTP adapter của courses; chỉ biết Service.
type Handler struct {
	svc   *Service
	actor ActorFunc
}

// NewHandler tạo handler.
func NewHandler(svc *Service, actor ActorFunc) *Handler {
	return &Handler{svc: svc, actor: actor}
}

// Register gắn /courses, /course-versions (route phẳng) và POST /stage-versions/{vid}/apply vào rg; guards là
// middleware xác thực + quyền admin. Route apply nằm dưới /stage-versions nhưng hành vi là của khóa học.
func (h *Handler) Register(rg *gin.RouterGroup, guards ...gin.HandlerFunc) {
	co := rg.Group("/courses", guards...)
	co.GET("", h.list)
	co.POST("", h.create)
	co.GET("/:id", h.get)

	cv := rg.Group("/course-versions", guards...)
	cv.GET("/:vid", h.getVersion)
	cv.DELETE("/:vid", h.deleteVersion)
	cv.PUT("/:vid/stages", h.setStages)
	cv.POST("/:vid/publish", h.publish)
	cv.POST("/:vid/clone", h.clone)
	cv.POST("/:vid/archive", h.archive)

	rg.Group("/stage-versions", guards...).POST("/:vid/apply", h.apply)
}

func (h *Handler) list(c *gin.Context) {
	rows, err := h.svc.ListCourses(c.Request.Context(), ListQuery{Q: c.Query("q")})
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toListDTO(rows))
}

func (h *Handler) create(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	var req createCourseRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.CreateCourse(c.Request.Context(), actor, CreateCourseCmd(req))
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, toCourseDetailDTO(d))
}

func (h *Handler) get(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	d, err := h.svc.GetCourse(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toCourseDetailDTO(d))
}

func (h *Handler) getVersion(c *gin.Context) {
	vid, ok := uuidParam(c, "vid")
	if !ok {
		return
	}
	d, err := h.svc.GetVersion(c.Request.Context(), vid)
	h.respondVersion(c, http.StatusOK, d, err)
}

func (h *Handler) setStages(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	var req setStagesRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.SetStages(c.Request.Context(), actor, vid, req.StageVersionIDs)
	h.respondVersion(c, http.StatusOK, d, err)
}

func (h *Handler) publish(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	d, err := h.svc.Publish(c.Request.Context(), actor, vid)
	h.respondVersion(c, http.StatusOK, d, err)
}

func (h *Handler) clone(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	d, err := h.svc.Clone(c.Request.Context(), actor, vid)
	h.respondVersion(c, http.StatusCreated, d, err)
}

func (h *Handler) archive(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	d, err := h.svc.Archive(c.Request.Context(), actor, vid)
	h.respondVersion(c, http.StatusOK, d, err)
}

func (h *Handler) deleteVersion(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	deleted, err := h.svc.Delete(c.Request.Context(), actor, vid)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, DeleteVersionDTO{CourseDeleted: deleted})
}

// apply trả 200 với kết quả từng khóa học kể cả khi có khóa học thất bại; chỉ lỗi toàn cục mới là phản hồi lỗi.
func (h *Handler) apply(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	var req applyRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	results, err := h.svc.ApplyStageVersion(c.Request.Context(), actor, vid, req.CourseIDs)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toApplyResultsDTO(results))
}

func (h *Handler) respondVersion(c *gin.Context, status int, d VersionDetail, err error) {
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, status, toVersionDTO(d))
}

func (h *Handler) actorAndVersion(c *gin.Context) (Actor, uuid.UUID, bool) {
	actor, ok := h.mustActor(c)
	if !ok {
		return Actor{}, uuid.Nil, false
	}
	vid, ok := uuidParam(c, "vid")
	if !ok {
		return Actor{}, uuid.Nil, false
	}
	return actor, vid, true
}

func (h *Handler) mustActor(c *gin.Context) (Actor, bool) {
	id, ok := h.actor(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return Actor{}, false
	}
	return Actor{ID: id, RequestID: c.GetString(httpx.RequestIDKey)}, true
}

func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := httpx.UUIDParam(c, name)
	if err != nil {
		httpx.Fail(c, err)
		return uuid.Nil, false
	}
	return id, true
}

// errorEnvelope giống httpx.ErrorEnvelope nhưng details có cấu trúc (DRAFT_EXISTS, IN_USE).
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// fail trả lỗi theo envelope: DRAFT_EXISTS/IN_USE kèm details có cấu trúc; lỗi dữ liệu nghiệp vụ là 422 (lỗi cú
// pháp JSON vẫn 400); còn lại theo httpx.Fail.
func fail(c *gin.Context, err error) {
	var draft *ErrDraftExists
	if errors.As(err, &draft) {
		c.AbortWithStatusJSON(http.StatusConflict, errorEnvelope{Error: errorBody{
			Code: apperr.CodeDraftExists, Message: draft.Error(),
			Details: DraftExistsDetails{DraftVersionID: draft.DraftID.String(), DraftVersionNo: draft.No.Int()},
		}})
		return
	}
	var inUse *ErrInUse
	if errors.As(err, &inUse) {
		c.AbortWithStatusJSON(http.StatusConflict, errorEnvelope{Error: errorBody{
			Code: apperr.CodeInUse, Message: inUse.Error(), Details: InUseDetails{UsedBy: usedByClassDTOs(inUse.UsedBy)},
		}})
		return
	}
	var de *domain.Error
	if errors.As(err, &de) && de.Kind == domain.KindInvalid {
		httpx.Fail(c, apperr.Wrap(err, http.StatusUnprocessableEntity, apperr.CodeValidationFailed, de.Msg))
		return
	}
	httpx.Fail(c, err)
}
