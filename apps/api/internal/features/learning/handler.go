package learning

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

// StudentFunc đọc id học viên đã đăng nhập từ context (middleware phiên đăng nhập gắn vào).
type StudentFunc func(*gin.Context) (uuid.UUID, bool)

// Handler là HTTP adapter của learning; chỉ biết Service.
type Handler struct {
	svc     *Service
	student StudentFunc
}

// NewHandler tạo handler.
func NewHandler(svc *Service, student StudentFunc) *Handler {
	return &Handler{svc: svc, student: student}
}

// Register gắn /me/classes vào rg (rg đã qua xác thực phiên); requireStudent chặn vai trò khác bằng 403.
// Cache-Control: no-store do middleware SecurityHeaders đặt cho mọi /api (phản hồi chứa URL ký).
func (h *Handler) Register(rg *gin.RouterGroup, requireStudent gin.HandlerFunc) {
	me := rg.Group("/me/classes", requireStudent)
	me.GET("", h.list)
	me.GET("/:id", h.roadmap)
	me.GET("/:id/lessons/:lid", h.lesson)
	me.PUT("/:id/lessons/:lid/completion", h.completion)
}

func (h *Handler) list(c *gin.Context) {
	userID, ok := h.mustStudent(c)
	if !ok {
		return
	}
	rows, err := h.svc.MyClasses(c.Request.Context(), userID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toMyClassesDTO(rows))
}

func (h *Handler) roadmap(c *gin.Context) {
	userID, ok := h.mustStudent(c)
	if !ok {
		return
	}
	classID, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	r, err := h.svc.Roadmap(c.Request.Context(), userID, classID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toRoadmapDTO(r))
}

func (h *Handler) lesson(c *gin.Context) {
	userID, classID, lessonID, ok := h.lessonTarget(c)
	if !ok {
		return
	}
	p, err := h.svc.OpenLesson(c.Request.Context(), userID, classID, lessonID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toLessonPageDTO(p))
}

func (h *Handler) completion(c *gin.Context) {
	userID, classID, lessonID, ok := h.lessonTarget(c)
	if !ok {
		return
	}
	var req completionRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	out, err := h.svc.SetCompletion(c.Request.Context(), userID, classID, lessonID, *req.Completed)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toCompletionDTO(out))
}

func (h *Handler) lessonTarget(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	userID, ok := h.mustStudent(c)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	classID, ok := uuidParam(c, "id")
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	lessonID, ok := uuidParam(c, "lid")
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return userID, classID, lessonID, true
}

func (h *Handler) mustStudent(c *gin.Context) (uuid.UUID, bool) {
	id, ok := h.student(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return uuid.Nil, false
	}
	return id, true
}

func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := httpx.UUIDParam(c, name)
	if err != nil {
		httpx.Fail(c, err)
		return uuid.Nil, false
	}
	return id, true
}
