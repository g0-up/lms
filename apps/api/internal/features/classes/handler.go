package classes

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/ids"
)

// ActorFunc đọc id và vai trò người dùng đã đăng nhập từ context (middleware phiên đăng nhập gắn vào).
type ActorFunc func(*gin.Context) (uuid.UUID, domain.Role, bool)

// Guards là middleware phân quyền cho từng nhóm route; InviteLimit là rate limit của POST …/invitations.
type Guards struct {
	Admin       gin.HandlerFunc
	Staff       gin.HandlerFunc // admin hoặc giảng viên; phạm vi lớp kiểm ở service
	Teacher     gin.HandlerFunc
	InviteLimit gin.HandlerFunc
}

// Handler là HTTP adapter của classes; chỉ biết Service.
type Handler struct {
	svc   *Service
	actor ActorFunc
}

// NewHandler tạo handler.
func NewHandler(svc *Service, actor ActorFunc) *Handler {
	return &Handler{svc: svc, actor: actor}
}

// Register gắn /classes và /teach/classes vào rg (rg đã qua xác thực phiên).
func (h *Handler) Register(rg *gin.RouterGroup, g Guards) {
	cl := rg.Group("/classes")
	cl.GET("", g.Admin, h.list)
	cl.POST("", g.Admin, h.create)
	cl.GET("/:id", g.Staff, h.get)
	cl.PATCH("/:id", g.Admin, h.update)
	cl.POST("/:id/activate", g.Admin, h.activate)
	cl.POST("/:id/end", g.Admin, h.end)
	cl.GET("/:id/members", g.Staff, h.members)
	cl.POST("/:id/invitations", g.Admin, g.InviteLimit, h.invite)
	cl.POST("/:id/members/:mid/resend", g.Admin, h.resend)
	cl.DELETE("/:id/members/:mid", g.Admin, h.remove)

	rg.GET("/teach/classes", g.Teacher, h.teaching)
}

func (h *Handler) list(c *gin.Context) {
	q := ListQuery{Q: c.Query("q")}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		st, err := domain.ParseClassStatus(raw)
		if err != nil {
			fail(c, err)
			return
		}
		q.Status = &st
	}
	if raw := strings.TrimSpace(c.Query("teacherId")); raw != "" {
		id, err := ids.Parse(raw)
		if err != nil {
			fail(c, ErrTeacherInvalid)
			return
		}
		q.TeacherID = &id
	}
	rows, err := h.svc.ListClasses(c.Request.Context(), q)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toClassListItems(rows))
}

func (h *Handler) create(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	var req createClassRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.CreateClass(c.Request.Context(), actor, CreateClassCmd(req))
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, toClassDetailDTO(d))
}

func (h *Handler) get(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	d, err := h.svc.GetClass(c.Request.Context(), actor, id)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toClassDetailDTO(d))
}

func (h *Handler) update(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	var req updateClassRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.UpdateClass(c.Request.Context(), actor, id, UpdateClassCmd(req))
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toClassDetailDTO(d))
}

func (h *Handler) activate(c *gin.Context) { h.transition(c, h.svc.Activate) }

func (h *Handler) end(c *gin.Context) { h.transition(c, h.svc.End) }

func (h *Handler) transition(c *gin.Context, run func(context.Context, Actor, uuid.UUID) (*ClassDetail, error)) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	d, err := run(c.Request.Context(), actor, id)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toClassDetailDTO(d))
}

func (h *Handler) members(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	includeDropped := false
	if raw := c.Query("includeDropped"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			fail(c, domain.ErrInvalid.WithMsg("includeDropped phải là true hoặc false."))
			return
		}
		includeDropped = v
	}
	rows, err := h.svc.ListMembers(c.Request.Context(), actor, id, includeDropped)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toMemberItems(rows))
}

func (h *Handler) invite(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	var req inviteRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	res, err := h.svc.Invite(c.Request.Context(), actor, id, InviteCmd(req))
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, InviteResponse{Kind: string(res.Kind), Member: toMemberDTO(res.Member)})
}

func (h *Handler) resend(c *gin.Context) {
	actor, classID, memberID, ok := h.memberTarget(c)
	if !ok {
		return
	}
	row, err := h.svc.Resend(c.Request.Context(), actor, classID, memberID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, MemberResponse{Member: toMemberDTO(*row)})
}

func (h *Handler) remove(c *gin.Context) {
	actor, classID, memberID, ok := h.memberTarget(c)
	if !ok {
		return
	}
	row, err := h.svc.RemoveMember(c.Request.Context(), actor, classID, memberID)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toMemberDTO(*row))
}

func (h *Handler) teaching(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	rows, err := h.svc.ListTeachingClasses(c.Request.Context(), actor)
	if err != nil {
		fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toTeachingItems(rows))
}

func (h *Handler) memberTarget(c *gin.Context) (Actor, uuid.UUID, uuid.UUID, bool) {
	actor, ok := h.mustActor(c)
	if !ok {
		return Actor{}, uuid.Nil, uuid.Nil, false
	}
	classID, ok := uuidParam(c, "id")
	if !ok {
		return Actor{}, uuid.Nil, uuid.Nil, false
	}
	memberID, ok := uuidParam(c, "mid")
	if !ok {
		return Actor{}, uuid.Nil, uuid.Nil, false
	}
	return actor, classID, memberID, true
}

func (h *Handler) mustActor(c *gin.Context) (Actor, bool) {
	id, role, ok := h.actor(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return Actor{}, false
	}
	return Actor{ID: id, Role: role, RequestID: c.GetString(httpx.RequestIDKey)}, true
}

func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := httpx.UUIDParam(c, name)
	if err != nil {
		httpx.Fail(c, err)
		return uuid.Nil, false
	}
	return id, true
}

// fail trả lỗi theo envelope chung; lỗi dữ liệu nghiệp vụ là 422 (lỗi cú pháp JSON vẫn 400). Lỗi đã mang mã HTTP
// riêng (ACCOUNT_DISABLED, RATE_LIMITED, email sai) đi thẳng qua httpx.Fail.
func fail(c *gin.Context, err error) {
	var de *domain.Error
	if errors.As(err, &de) && de.Kind == domain.KindInvalid {
		httpx.Fail(c, apperr.Wrap(err, http.StatusUnprocessableEntity, apperr.CodeValidationFailed, de.Msg))
		return
	}
	httpx.Fail(c, err)
}
