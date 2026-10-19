package quizzes

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/learning"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

// ActorFunc đọc id và vai trò người dùng đã đăng nhập từ context (middleware phiên đăng nhập gắn vào).
type ActorFunc func(*gin.Context) (uuid.UUID, domain.Role, bool)

// Handler là HTTP adapter của quizzes; chỉ biết Service và ReviewService.
type Handler struct {
	svc     *Service
	review  *ReviewService
	student learning.StudentFunc
	actor   ActorFunc
}

// NewHandler tạo handler.
func NewHandler(svc *Service, review *ReviewService, student learning.StudentFunc, actor ActorFunc) *Handler {
	return &Handler{svc: svc, review: review, student: student, actor: actor}
}

// Guards là middleware vai trò cho từng nhóm route.
type Guards struct {
	Staff   gin.HandlerFunc // admin và giảng viên; phạm vi lớp kiểm ở service
	Student gin.HandlerFunc
}

// Register gắn route quiz vào rg (rg đã qua xác thực phiên): làm quiz của học viên, xem và nhận xét lượt của staff.
// Vai trò khác bị chặn bằng 403. Cache-Control: no-store do middleware SecurityHeaders đặt cho mọi /api.
func (h *Handler) Register(rg *gin.RouterGroup, g Guards) {
	rg.GET("/me/classes/:id/lessons/:lid/attempts", g.Student, h.overview)
	rg.POST("/me/classes/:id/lessons/:lid/attempts", g.Student, h.start)
	me := rg.Group("/me/attempts", g.Student)
	me.GET("/:aid", h.attempt)
	me.PUT("/:aid/answers/:qid", h.saveAnswer)
	me.POST("/:aid/submit", h.submit)
	rg.GET("/quiz-attempts/:aid", g.Staff, h.attemptDetail)
	rg.POST("/quiz-attempts/:aid/reviews", g.Staff, h.saveReview)
	rg.GET("/classes/:id/quizzes", g.Staff, h.classQuizzes)
	rg.POST("/classes/:id/members/:mid/grants", g.Staff, h.grant)
}

func (h *Handler) overview(c *gin.Context) {
	userID, classID, lessonID, ok := h.lessonTarget(c)
	if !ok {
		return
	}
	o, err := h.svc.Overview(c.Request.Context(), userID, classID, lessonID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toOverviewDTO(o))
}

func (h *Handler) start(c *gin.Context) {
	userID, classID, lessonID, ok := h.lessonTarget(c)
	if !ok {
		return
	}
	st, err := h.svc.Start(c.Request.Context(), userID, classID, lessonID)
	if err != nil {
		fail(c, err)
		return
	}
	status := http.StatusCreated
	if st.Resumed {
		status = http.StatusOK
	}
	httpx.OK(c, status, StartDTO{Resumed: st.Resumed, Attempt: toOpenAttemptDTO(st.View)})
}

func (h *Handler) attempt(c *gin.Context) {
	userID, attemptID, ok := h.attemptTarget(c)
	if !ok {
		return
	}
	v, err := h.svc.Attempt(c.Request.Context(), userID, attemptID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toAttemptDTO(v))
}

func (h *Handler) saveAnswer(c *gin.Context) {
	userID, attemptID, ok := h.attemptTarget(c)
	if !ok {
		return
	}
	questionID, ok := uuidParam(c, "qid")
	if !ok {
		return
	}
	var req saveAnswerRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if req.OptionIDs == nil {
		fail(c, ErrOptionIDsRequired)
		return
	}
	optionIDs, err := parseOptionIDs(*req.OptionIDs)
	if err != nil {
		fail(c, err)
		return
	}
	at, err := h.svc.SaveAnswer(c.Request.Context(), userID, attemptID, questionID, optionIDs)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, SavedDTO{SavedAt: at.UTC()})
}

func (h *Handler) submit(c *gin.Context) {
	userID, attemptID, ok := h.attemptTarget(c)
	if !ok {
		return
	}
	v, err := h.svc.Submit(c.Request.Context(), userID, attemptID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toAttemptDTO(v))
}

func (h *Handler) attemptDetail(c *gin.Context) {
	actor, attemptID, ok := h.staffTarget(c, "aid")
	if !ok {
		return
	}
	d, err := h.review.AttemptDetail(c.Request.Context(), actor, attemptID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toAttemptDetailDTO(d))
}

func (h *Handler) saveReview(c *gin.Context) {
	actor, attemptID, ok := h.staffTarget(c, "aid")
	if !ok {
		return
	}
	var req reviewRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	o, err := h.review.Review(c.Request.Context(), actor, attemptID, req.input())
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, toReviewOutcomeDTO(o))
}

func (h *Handler) classQuizzes(c *gin.Context) {
	actor, classID, ok := h.staffTarget(c, "id")
	if !ok {
		return
	}
	v, err := h.review.ClassQuizzes(c.Request.Context(), actor, classID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toClassQuizzesDTO(v))
}

func (h *Handler) grant(c *gin.Context) {
	actor, classID, ok := h.staffTarget(c, "id")
	if !ok {
		return
	}
	memberID, ok := uuidParam(c, "mid")
	if !ok {
		return
	}
	var req grantRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	in, err := req.input()
	if err != nil {
		fail(c, err)
		return
	}
	g, err := h.review.Grant(c.Request.Context(), actor, classID, memberID, in)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, toGrantDTO(g))
}

// staffTarget đọc actor và id trong đường dẫn của route staff.
func (h *Handler) staffTarget(c *gin.Context, param string) (Actor, uuid.UUID, bool) {
	id, role, ok := h.actor(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return Actor{}, uuid.Nil, false
	}
	target, ok := uuidParam(c, param)
	if !ok {
		return Actor{}, uuid.Nil, false
	}
	return Actor{ID: id, Role: role, RequestID: c.GetString(httpx.RequestIDKey)}, target, true
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

func (h *Handler) attemptTarget(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := h.mustStudent(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	attemptID, ok := uuidParam(c, "aid")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return userID, attemptID, true
}

func (h *Handler) mustStudent(c *gin.Context) (uuid.UUID, bool) {
	id, ok := h.student(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return uuid.Nil, false
	}
	return id, true
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

func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := httpx.UUIDParam(c, name)
	if err != nil {
		httpx.Fail(c, err)
		return uuid.Nil, false
	}
	return id, true
}
