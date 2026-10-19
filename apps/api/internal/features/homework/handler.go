package homework

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

// ActorFunc đọc id và vai trò người dùng đã đăng nhập từ context (middleware phiên đăng nhập gắn vào).
type ActorFunc func(*gin.Context) (uuid.UUID, domain.Role, bool)

// Handler là HTTP adapter của homework; chỉ biết Service.
type Handler struct {
	svc   *Service
	actor ActorFunc
}

// NewHandler tạo handler.
func NewHandler(svc *Service, actor ActorFunc) *Handler {
	return &Handler{svc: svc, actor: actor}
}

// Guards là middleware vai trò cho từng nhóm route.
type Guards struct {
	Staff   gin.HandlerFunc // admin và giảng viên; phạm vi lớp kiểm ở service
	Student gin.HandlerFunc
}

// Register gắn route homework vào rg (rg đã qua xác thực phiên). Phản hồi chứa dữ liệu cá nhân học viên nên không
// cache.
func (h *Handler) Register(rg *gin.RouterGroup, g Guards) {
	rg.PUT("/classes/:id/deadlines/:lid", g.Staff, h.setDeadline)
	rg.DELETE("/classes/:id/deadlines/:lid", g.Staff, h.clearDeadline)
	rg.GET("/classes/:id/homework", noStore, g.Staff, h.classHomework)
	rg.GET("/submissions/:sid", noStore, g.Staff, h.submission)
	rg.POST("/submissions/:sid/grades", g.Staff, h.grade)
	rg.GET("/me/classes/:id/lessons/:lid/submissions", noStore, g.Student, h.mySubmissions)
	rg.POST("/me/classes/:id/lessons/:lid/submissions", g.Student, h.submit)
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Next()
}

func (h *Handler) setDeadline(c *gin.Context) {
	actor, classID, lessonID, ok := h.classLesson(c)
	if !ok {
		return
	}
	var req deadlineRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	dueAt, err := time.Parse(time.RFC3339, req.DueAt)
	if err != nil {
		fe := &FieldErrors{}
		fe.add("dueAt", ErrInvalidDueAt)
		fail(c, fe)
		return
	}
	v, err := h.svc.SetDeadline(c.Request.Context(), actor, classID, lessonID, dueAt)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toDeadlineDTO(v))
}

func (h *Handler) clearDeadline(c *gin.Context) {
	actor, classID, lessonID, ok := h.classLesson(c)
	if !ok {
		return
	}
	if _, err := h.svc.ClearDeadline(c.Request.Context(), actor, classID, lessonID); err != nil {
		fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) classHomework(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	classID, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	v, err := h.svc.ClassHomework(c.Request.Context(), actor, classID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toClassHomeworkDTO(v))
}

func (h *Handler) submission(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	sid, ok := uuidParam(c, "sid")
	if !ok {
		return
	}
	d, err := h.svc.SubmissionDetail(c.Request.Context(), actor, sid)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toSubmissionDetailDTO(d))
}

func (h *Handler) grade(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	sid, ok := uuidParam(c, "sid")
	if !ok {
		return
	}
	var req gradeRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	o, err := h.svc.Grade(c.Request.Context(), actor, sid, req.input())
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, toGradeOutcomeDTO(o))
}

func (h *Handler) mySubmissions(c *gin.Context) {
	actor, classID, lessonID, ok := h.classLesson(c)
	if !ok {
		return
	}
	v, err := h.svc.MySubmissions(c.Request.Context(), actor.ID, classID, lessonID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toMySubmissionsDTO(v))
}

func (h *Handler) submit(c *gin.Context) {
	actor, classID, lessonID, ok := h.classLesson(c)
	if !ok {
		return
	}
	var req submitRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	v, err := h.svc.Submit(c.Request.Context(), actor.ID, classID, lessonID, SubmitInput(req))
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, toMySubmissionsDTO(v))
}

func (h *Handler) classLesson(c *gin.Context) (Actor, uuid.UUID, uuid.UUID, bool) {
	actor, ok := h.mustActor(c)
	if !ok {
		return Actor{}, uuid.Nil, uuid.Nil, false
	}
	classID, ok := uuidParam(c, "id")
	if !ok {
		return Actor{}, uuid.Nil, uuid.Nil, false
	}
	lessonID, ok := uuidParam(c, "lid")
	if !ok {
		return Actor{}, uuid.Nil, uuid.Nil, false
	}
	return actor, classID, lessonID, true
}

func (h *Handler) mustActor(c *gin.Context) (Actor, bool) {
	id, role, ok := h.actor(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return Actor{}, false
	}
	return Actor{ID: id, Role: role, RequestID: c.GetString(httpx.RequestIDKey)}, true
}

// fail trả lỗi theo envelope; lỗi dữ liệu sai là 422 VALIDATION_FAILED theo hợp đồng API (lỗi cú pháp JSON vẫn 400),
// lỗi form kèm details theo trường.
func fail(c *gin.Context, err error) {
	var fe *FieldErrors
	if errors.As(err, &fe) {
		e := apperr.Wrap(err, http.StatusUnprocessableEntity, apperr.CodeValidationFailed, fe.Error())
		e.Details = fe.Fields
		httpx.Fail(c, e)
		return
	}
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
