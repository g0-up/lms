package grading

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/ids"
)

// ActorFunc đọc id và vai trò người dùng đã đăng nhập từ context (middleware phiên đăng nhập gắn vào).
type ActorFunc func(*gin.Context) (uuid.UUID, domain.Role, bool)

// Handler là HTTP adapter của grading; chỉ biết Service.
type Handler struct {
	svc   *Service
	actor ActorFunc
}

// NewHandler tạo handler.
func NewHandler(svc *Service, actor ActorFunc) *Handler {
	return &Handler{svc: svc, actor: actor}
}

// Register gắn hàng chờ chấm vào rg (rg đã qua xác thực phiên); staff cho admin và giảng viên, phạm vi lớp lọc ở
// service.
func (h *Handler) Register(rg *gin.RouterGroup, staff gin.HandlerFunc) {
	rg.GET("/grading/queue", noStore, staff, h.queue)
	rg.GET("/grading/recent", noStore, staff, h.recent)
}

// noStore: hàng chờ chứa tên và điểm học viên, không cache ở trình duyệt hay proxy.
func noStore(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Next()
}

func (h *Handler) queue(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	f, err := parseFilter(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	q, err := h.svc.Queue(c.Request.Context(), actor, f)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toQueueDTO(q))
}

func (h *Handler) recent(c *gin.Context) {
	actor, ok := h.mustActor(c)
	if !ok {
		return
	}
	limit := DefaultRecentLimit
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			httpx.Fail(c, ErrInvalidFilter)
			return
		}
		limit = n
	}
	rs, err := h.svc.Recent(c.Request.Context(), actor, limit)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, toRecentListDTO(rs))
}

func (h *Handler) mustActor(c *gin.Context) (Actor, bool) {
	id, role, ok := h.actor(c)
	if !ok {
		httpx.Fail(c, apperr.Unauthenticated())
		return Actor{}, false
	}
	return Actor{ID: id, Role: role}, true
}

// parseFilter đọc query kind, classId, lessonId, overdue; giá trị không đọc được là ErrInvalidFilter.
func parseFilter(c *gin.Context) (Filter, error) {
	var f Filter
	var err error
	if f.Kind, err = ParseKind(c.Query("kind")); err != nil {
		return Filter{}, err
	}
	if f.ClassID, err = queryUUID(c, "classId"); err != nil {
		return Filter{}, err
	}
	if f.LessonID, err = queryUUID(c, "lessonId"); err != nil {
		return Filter{}, err
	}
	if raw := c.Query("overdue"); raw != "" {
		if f.OverdueOnly, err = strconv.ParseBool(raw); err != nil {
			return Filter{}, ErrInvalidFilter
		}
	}
	return f, nil
}

// queryUUID: vắng hoặc rỗng là nil (không lọc).
func queryUUID(c *gin.Context, name string) (*uuid.UUID, error) {
	raw := c.Query(name)
	if raw == "" {
		return nil, nil
	}
	id, err := ids.Parse(raw)
	if err != nil {
		return nil, ErrInvalidFilter
	}
	return &id, nil
}
