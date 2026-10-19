package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/homework"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/quizzes"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/middleware"
)

const (
	readyzTimeout = 2 * time.Second
	// mediaRequestsPerMin là giới hạn request /media mỗi phút cho một người dùng.
	mediaRequestsPerMin = 120
	// invitationsPerMin là giới hạn request mời học viên mỗi phút cho một IP (chống spam email).
	invitationsPerMin = 30
)

// New là nơi duy nhất gắn middleware và route của feature.
func New(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.HandleMethodNotAllowed = true
	// Không tin X-Forwarded-For khi vận hành chưa khai báo CIDR proxy (mặc định rỗng).
	if err := middleware.TrustProxies(engine, d.Cfg.TrustedProxies); err != nil {
		d.Logger.Warn("bỏ qua header proxy", "err", err)
	}

	engine.Use(middleware.RequestID(), middleware.Logger(d.Logger), middleware.Recover(d.Logger), middleware.SecurityHeaders())
	engine.NoRoute(func(c *gin.Context) { httpx.Fail(c, apperr.NotFound("đường dẫn")) })
	engine.NoMethod(func(c *gin.Context) {
		httpx.Fail(c, apperr.New(http.StatusMethodNotAllowed, apperr.CodeMethodNotAllowed, apperr.Message(apperr.CodeMethodNotAllowed)))
	})

	healthz := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) }
	engine.GET("/healthz", healthz)
	// Cùng endpoint dưới /api để kiểm tra đường proxy của web (Vite dev, nginx, Caddy).
	engine.GET("/api/healthz", healthz)
	engine.GET("/readyz", readyz(d))

	// Feature đăng ký route dưới /api/v1. Request thay đổi dữ liệu phải có X-Requested-With: fetch (lớp CSRF thứ
	// hai; lớp thứ nhất là http.CrossOriginProtection bọc engine trong lệnh serve).
	api := engine.Group("/api/v1", httpx.RequireCustomHeader())
	mail := newMailer(d)
	ident := newIdentity(d, mail)
	authMW := ident.auth
	ident.handler.Register(api, authMW)
	// authed: đã đăng nhập và đã đổi mật khẩu tạm; feature gắn RequireRole theo route.
	authed := api.Group("", authMW.SessionAuth(), authMW.MustChangePassword())

	requireAdmin := identity.RequireRole(domain.RoleAdmin)
	requireStaff := identity.RequireRole(domain.RoleAdmin, domain.RoleTeacher)
	requireStudent := identity.RequireRole(domain.RoleStudent)
	stagesSvc := newStagesService(d)
	newStages(stagesSvc).Register(authed, requireAdmin)
	// courses gắn cả POST /stage-versions/:vid/apply: prefix là của chặng nhưng thao tác ghi vào khóa học.
	newCourses(d).Register(authed, requireAdmin)
	// Trang học tải nhiều ảnh markdown qua /content nên limiter theo user, không theo IP.
	mediaLimit := middleware.RateLimit(userKey, mediaRequestsPerMin, !d.Cfg.IsE2E())
	mediaHandler := newMedia(d)
	mediaHandler.Register(authed.Group("/media", mediaLimit), requireAdmin)
	// Học viên tải file bài nộp; xem file vẫn qua /media/{id}/url (quyền theo bản nộp).
	mediaHandler.RegisterStudent(authed.Group("/me/uploads", mediaLimit, requireStudent))
	newClasses(d, ident.svc, mail).Register(authed, classes.Guards{
		Admin:       requireAdmin,
		Staff:       requireStaff,
		Teacher:     identity.RequireRole(domain.RoleTeacher),
		InviteLimit: middleware.RateLimit(middleware.ClientIP, invitationsPerMin, !d.Cfg.IsE2E()),
	})
	summarySvc := newSummaryService(d)
	newLearning(d, summarySvc).Register(authed, requireStudent)
	newSummary(summarySvc).Register(authed, requireStudent, requireStaff)
	gradingSvc := newGradingService(d)
	newGrading(gradingSvc).Register(authed, requireStaff)
	newQuizzes(d, mail, gradingSvc).Register(authed, quizzes.Guards{Staff: requireStaff, Student: requireStudent})
	newHomework(d, mail, gradingSvc).Register(authed, homework.Guards{Staff: requireStaff, Student: requireStudent})
	newReports(d, stagesSvc, gradingSvc).Register(authed, requireStaff, requireAdmin)

	return engine
}

func readyz(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readyzTimeout)
		defer cancel()
		if d.DB == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		if err := d.DB.PingContext(ctx); err != nil {
			d.Logger.WarnContext(ctx, "readyz: database không phản hồi", "err", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
