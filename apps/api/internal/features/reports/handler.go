package reports

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

// ActorFunc đọc id và vai trò người dùng đã đăng nhập từ context (middleware phiên đăng nhập gắn vào).
type ActorFunc func(*gin.Context) (uuid.UUID, domain.Role, bool)

// Handler là HTTP adapter của reports; chỉ biết Service.
type Handler struct {
	svc   *Service
	actor ActorFunc
}

// NewHandler tạo handler.
func NewHandler(svc *Service, actor ActorFunc) *Handler {
	return &Handler{svc: svc, actor: actor}
}

// Register gắn báo cáo lớp và dashboard vào rg (rg đã qua xác thực phiên). staff cho admin và giảng viên (phạm
// vi lớp kiểm ở service), admin chỉ cho admin.
func (h *Handler) Register(rg *gin.RouterGroup, staff, admin gin.HandlerFunc) {
	rg.GET("/classes/:id/report", noStore, staff, h.classReport)
	rg.GET("/classes/:id/report/members/:mid", noStore, staff, h.memberReport)
	rg.GET("/dashboard", noStore, admin, h.dashboard)
}

// noStore: báo cáo chứa dữ liệu cá nhân học viên, không cache ở trình duyệt hay proxy.
func noStore(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Next()
}

func (h *Handler) classReport(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	classID, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	f, err := parseFilter(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	r, err := h.svc.ClassReport(c.Request.Context(), actor, classID, f)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toClassReportDTO(r))
}

func (h *Handler) memberReport(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	classID, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	memberID, err := httpx.UUIDParam(c, "mid")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	r, err := h.svc.MemberReport(c.Request.Context(), actor, classID, memberID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toMemberReportDTO(r))
}

func (h *Handler) dashboard(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	d, err := h.svc.Dashboard(c.Request.Context(), actor)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toDashboardDTO(d))
}

func (h *Handler) mustActor(c *gin.Context) (Actor, bool) {
	id, role, ok := h.actor(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return Actor{}, false
	}
	return Actor{ID: id, Role: role}, true
}

// parseFilter đọc query notLoggedIn, inactiveDays, belowPercent, includeDropped, sort; giá trị không đọc được là
// ErrInvalidFilter, kiểm biên do Filter.Validate ở service.
func parseFilter(c *gin.Context) (Filter, error) {
	var f Filter
	var err error
	if f.NotLoggedIn, err = queryBool(c, "notLoggedIn"); err != nil {
		return Filter{}, err
	}
	if f.IncludeDropped, err = queryBool(c, "includeDropped"); err != nil {
		return Filter{}, err
	}
	if f.InactiveDays, err = queryInt(c, "inactiveDays"); err != nil {
		return Filter{}, err
	}
	if f.BelowPercent, err = queryInt(c, "belowPercent"); err != nil {
		return Filter{}, err
	}
	if f.Sort, err = ParseSort(c.Query("sort")); err != nil {
		return Filter{}, err
	}
	return f, nil
}

// queryBool: vắng hoặc rỗng là false.
func queryBool(c *gin.Context, name string) (bool, error) {
	raw := c.Query(name)
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, ErrInvalidFilter
	}
	return v, nil
}

// queryInt: vắng hoặc rỗng là nil (không lọc).
func queryInt(c *gin.Context, name string) (*int, error) {
	raw := c.Query(name)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, ErrInvalidFilter
	}
	return &v, nil
}
