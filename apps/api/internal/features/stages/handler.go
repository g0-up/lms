package stages

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

// Handler là HTTP adapter của stages; chỉ biết Service.
type Handler struct {
	svc   *Service
	actor ActorFunc
}

// NewHandler tạo handler.
func NewHandler(svc *Service, actor ActorFunc) *Handler {
	return &Handler{svc: svc, actor: actor}
}

// Register gắn /stages và /stage-versions (route phẳng) vào rg; guards là middleware xác thực + quyền admin.
func (h *Handler) Register(rg *gin.RouterGroup, guards ...gin.HandlerFunc) {
	st := rg.Group("/stages", guards...)
	st.GET("", h.list)
	st.POST("", h.create)
	st.POST("/markdown-preview", h.previewMarkdown)
	st.GET("/:id", h.get)

	sv := rg.Group("/stage-versions", guards...)
	sv.GET("/:vid", h.getVersion)
	sv.DELETE("/:vid", h.deleteVersion)
	sv.POST("/:vid/units", h.addUnit)
	sv.PUT("/:vid/units/order", h.reorderUnits)
	sv.PATCH("/:vid/units/:uid", h.renameUnit)
	sv.DELETE("/:vid/units/:uid", h.removeUnit)
	sv.POST("/:vid/lessons", h.addLesson)
	sv.PUT("/:vid/lessons/order", h.reorder)
	sv.PATCH("/:vid/lessons/:lid", h.updateLesson)
	sv.DELETE("/:vid/lessons/:lid", h.removeLesson)
	sv.POST("/:vid/lessons/:lid/questions", h.addQuestion)
	sv.PUT("/:vid/lessons/:lid/questions/order", h.reorderQuestions)
	sv.PATCH("/:vid/lessons/:lid/questions/:qid", h.updateQuestion)
	sv.DELETE("/:vid/lessons/:lid/questions/:qid", h.removeQuestion)
	sv.POST("/:vid/lessons/:lid/criteria", h.addCriterion)
	sv.PUT("/:vid/lessons/:lid/criteria/order", h.reorderCriteria)
	sv.PATCH("/:vid/lessons/:lid/criteria/:cid", h.updateCriterion)
	sv.DELETE("/:vid/lessons/:lid/criteria/:cid", h.removeCriterion)
	sv.POST("/:vid/publish", h.publish)
	sv.POST("/:vid/clone", h.clone)
	sv.POST("/:vid/archive", h.archive)
}

func (h *Handler) list(c *gin.Context) {
	rows, err := h.svc.ListStages(c.Request.Context(), ListQuery{Q: c.Query("q")})
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
	var req createStageRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.CreateStage(c.Request.Context(), actor, CreateStageCmd(req))
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, toStageDetailDTO(d))
}

func (h *Handler) previewMarkdown(c *gin.Context) {
	var req previewRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	html, err := h.svc.PreviewMarkdown(req.MarkdownSource)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, MarkdownPreviewDTO{HTML: html})
}

func (h *Handler) get(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	d, err := h.svc.GetStage(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toStageDetailDTO(d))
}

func (h *Handler) getVersion(c *gin.Context) {
	vid, ok := uuidParam(c, "vid")
	if !ok {
		return
	}
	d, err := h.svc.GetVersion(c.Request.Context(), vid)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toVersionDTO(d))
}

func (h *Handler) addLesson(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	var req lessonRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	l, err := h.svc.AddLesson(c.Request.Context(), actor, vid, req.cmd())
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, toLessonDTO(l, domain.VersionDraft))
}

func (h *Handler) updateLesson(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	lid, ok := uuidParam(c, "lid")
	if !ok {
		return
	}
	var req lessonRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	l, err := h.svc.UpdateLesson(c.Request.Context(), actor, vid, lid, req.cmd())
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toLessonDTO(l, domain.VersionDraft))
}

func (h *Handler) removeLesson(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	lid, ok := uuidParam(c, "lid")
	if !ok {
		return
	}
	if err := h.svc.RemoveLesson(c.Request.Context(), actor, vid, lid); err != nil {
		fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) reorder(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	var req reorderRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.ReorderLessons(c.Request.Context(), actor, vid, req.layout())
	h.respondVersion(c, http.StatusOK, d, err)
}

func (h *Handler) addQuestion(c *gin.Context) {
	actor, vid, lid, ok := h.actorVersionLesson(c)
	if !ok {
		return
	}
	var req questionRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	l, err := h.svc.AddQuestion(c.Request.Context(), actor, vid, lid, req.cmd())
	h.respondLesson(c, http.StatusCreated, l, err)
}

func (h *Handler) updateQuestion(c *gin.Context) {
	actor, vid, lid, ok := h.actorVersionLesson(c)
	if !ok {
		return
	}
	qid, ok := uuidParam(c, "qid")
	if !ok {
		return
	}
	var req questionRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	l, err := h.svc.UpdateQuestion(c.Request.Context(), actor, vid, lid, qid, req.cmd())
	h.respondLesson(c, http.StatusOK, l, err)
}

func (h *Handler) removeQuestion(c *gin.Context) {
	actor, vid, lid, ok := h.actorVersionLesson(c)
	if !ok {
		return
	}
	qid, ok := uuidParam(c, "qid")
	if !ok {
		return
	}
	if err := h.svc.RemoveQuestion(c.Request.Context(), actor, vid, lid, qid); err != nil {
		fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) reorderQuestions(c *gin.Context) {
	actor, vid, lid, ok := h.actorVersionLesson(c)
	if !ok {
		return
	}
	var req reorderQuestionsRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	l, err := h.svc.ReorderQuestions(c.Request.Context(), actor, vid, lid, req.QuestionIDs)
	h.respondLesson(c, http.StatusOK, l, err)
}

func (h *Handler) addCriterion(c *gin.Context) {
	actor, vid, lid, ok := h.actorVersionLesson(c)
	if !ok {
		return
	}
	var req criterionRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	l, err := h.svc.AddCriterion(c.Request.Context(), actor, vid, lid, CriterionCmd(req))
	h.respondLesson(c, http.StatusCreated, l, err)
}

func (h *Handler) updateCriterion(c *gin.Context) {
	actor, vid, lid, ok := h.actorVersionLesson(c)
	if !ok {
		return
	}
	cid, ok := uuidParam(c, "cid")
	if !ok {
		return
	}
	var req criterionRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	l, err := h.svc.UpdateCriterion(c.Request.Context(), actor, vid, lid, cid, CriterionCmd(req))
	h.respondLesson(c, http.StatusOK, l, err)
}

func (h *Handler) removeCriterion(c *gin.Context) {
	actor, vid, lid, ok := h.actorVersionLesson(c)
	if !ok {
		return
	}
	cid, ok := uuidParam(c, "cid")
	if !ok {
		return
	}
	if err := h.svc.RemoveCriterion(c.Request.Context(), actor, vid, lid, cid); err != nil {
		fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) reorderCriteria(c *gin.Context) {
	actor, vid, lid, ok := h.actorVersionLesson(c)
	if !ok {
		return
	}
	var req reorderCriteriaRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	l, err := h.svc.ReorderCriteria(c.Request.Context(), actor, vid, lid, req.CriterionIDs)
	h.respondLesson(c, http.StatusOK, l, err)
}

func (h *Handler) addUnit(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	var req unitRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.AddUnit(c.Request.Context(), actor, vid, req.Title)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, toUnitDTO(d))
}

func (h *Handler) renameUnit(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	uid, ok := uuidParam(c, "uid")
	if !ok {
		return
	}
	var req unitRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.RenameUnit(c.Request.Context(), actor, vid, uid, req.Title)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toUnitDTO(d))
}

func (h *Handler) removeUnit(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	uid, ok := uuidParam(c, "uid")
	if !ok {
		return
	}
	if _, err := h.svc.RemoveUnit(c.Request.Context(), actor, vid, uid); err != nil {
		fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) reorderUnits(c *gin.Context) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return
	}
	var req reorderUnitsRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.ReorderUnits(c.Request.Context(), actor, vid, req.UnitIDs)
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
	httpx.OK(c, http.StatusOK, DeleteVersionDTO{StageDeleted: deleted})
}

func (h *Handler) respondVersion(c *gin.Context, status int, d VersionDetail, err error) {
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, status, toVersionDTO(d))
}

// respondLesson trả học liệu của bản nháp sau lệnh soạn câu hỏi/tiêu chí.
func (h *Handler) respondLesson(c *gin.Context, status int, l *Lesson, err error) {
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, status, toLessonDTO(l, domain.VersionDraft))
}

func (h *Handler) actorVersionLesson(c *gin.Context) (Actor, uuid.UUID, uuid.UUID, bool) {
	actor, vid, ok := h.actorAndVersion(c)
	if !ok {
		return Actor{}, uuid.Nil, uuid.Nil, false
	}
	lid, ok := uuidParam(c, "lid")
	if !ok {
		return Actor{}, uuid.Nil, uuid.Nil, false
	}
	return actor, vid, lid, true
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
			Code: apperr.CodeInUse, Message: inUse.Error(), Details: InUseDetails{UsedBy: usedByDTOs(inUse.UsedBy)},
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
