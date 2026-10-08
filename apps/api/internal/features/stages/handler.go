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
	sv.POST("/:vid/lessons", h.addLesson)
	sv.PUT("/:vid/lessons/order", h.reorder)
	sv.PATCH("/:vid/lessons/:lid", h.updateLesson)
	sv.DELETE("/:vid/lessons/:lid", h.removeLesson)
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
	d, err := h.svc.ReorderLessons(c.Request.Context(), actor, vid, req.LessonIDs)
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
