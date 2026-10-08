// Package app nối cấu hình, hạ tầng và feature thành gin.Engine.
package app

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/courses"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/learning"
	"lms/api/internal/features/mailer"
	"lms/api/internal/features/media"
	"lms/api/internal/features/reports"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/secretbox"
	"lms/api/internal/platform/storage"
)

// Deps là mọi phụ thuộc dùng chung mà router truyền xuống feature qua constructor.
// ID sinh bằng hàm package ids.New nên không có trong Deps.
type Deps struct {
	Cfg    config.Config
	DB     *sqlx.DB
	Clock  clock.Clock
	Audit  audit.Recorder
	Logger *slog.Logger
	// Secrets niêm phong bí mật của email_outbox (OUTBOX_SECRET_KEY); nil thì enqueue email có bí mật trả lỗi.
	Secrets *secretbox.Box
	// Storage là object storage S3 (MinIO dev, R2 production) cho media; serve luôn truyền.
	Storage storage.Storage
}

// newMailer dựng mailer.Service (Enqueuer) cho feature cần gửi email trong transaction nghiệp vụ.
func newMailer(d Deps) *mailer.Service {
	return mailer.NewService(mailer.NewOutboxRepo(), d.Secrets, d.Clock)
}

// identityModule gom service, middleware xác thực và handler của identity.
type identityModule struct {
	svc     *identity.Service
	auth    *identity.Middleware
	handler *identity.Handler
}

// newIdentity dựng identity; không chạm DB lúc dựng.
func newIdentity(d Deps, enq mailer.Enqueuer) identityModule {
	svc := identity.NewService(identity.ServiceDeps{
		DB: d.DB, Tx: db.TxRunner{DB: d.DB},
		Users: identity.PGUserRepo{}, Sessions: identity.PGSessionRepo{},
		Attempts: identity.PGLoginAttemptRepo{}, Resets: identity.PGResetTokenRepo{},
		Mailer: enq, Clock: d.Clock, Audit: d.Audit, Cfg: d.Cfg,
	})
	return identityModule{
		svc:     svc,
		auth:    identity.NewMiddleware(svc, d.Cfg.CookieSecure),
		handler: identity.NewHandler(svc, d.Cfg),
	}
}

// newStages dựng handler chặng; người thao tác lấy từ phiên identity.
func newStages(svc *stages.Service) *stages.Handler {
	return stages.NewHandler(svc, func(c *gin.Context) (uuid.UUID, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return uuid.Nil, false
		}
		return u.ID(), true
	})
}

// newStagesService dựng stages.Service; reports dùng lại làm stages.OutdatedReader.
func newStagesService(d Deps) *stages.Service {
	return stages.NewService(stages.Deps{
		DB: d.DB, Tx: db.TxRunner{DB: d.DB}, Stages: stages.PGStageRepo{}, Versions: stages.PGStageVersionRepo{},
		Media: media.PGRepo{}, Render: stages.NewMarkdownRenderer(), Clock: d.Clock, Audit: d.Audit, IDs: ids.V7{},
	})
}

// newCourses dựng handler khóa học; phiên bản chặng đọc qua adapter để courses không import stages.
func newCourses(d Deps) *courses.Handler {
	return courses.NewHandler(newCoursesService(d), currentUserID)
}

// newCoursesService dựng courses.Service; classes dùng lại làm courses.VersionReader.
func newCoursesService(d Deps) *courses.Service {
	return courses.NewService(courses.Deps{
		DB: d.DB, Tx: db.TxRunner{DB: d.DB}, Courses: courses.PGCourseRepo{}, Versions: courses.PGCourseVersionRepo{},
		StageVersions: stageVersionReader{repo: stages.PGStageVersionRepo{}}, Clock: d.Clock, Audit: d.Audit, IDs: ids.V7{},
	})
}

// newClasses dựng handler lớp học. Mời học viên ghi user qua identity (provisioner), phiên bản khóa học đọc qua
// courses.Service, email đi vào outbox của mailer, tất cả trong transaction của classes.
func newClasses(d Deps, provisioner identity.UserProvisioner, enq mailer.Enqueuer) *classes.Handler {
	svc := classes.NewService(classes.Deps{
		DB: d.DB, Tx: db.TxRunner{DB: d.DB},
		Classes: classes.PGClassRepo{}, Members: classes.PGMemberRepo{}, Invitations: classes.PGInvitationRepo{},
		Users: identity.PGUserRepo{}, Provisioner: provisioner, Locker: identity.PGLoginAttemptRepo{},
		Versions: newCoursesService(d), Mail: enq, Outbox: mailer.NewOutboxRepo(), Progress: classProgress{reader: learning.PGProgressReader{}},
		Clock: d.Clock, Audit: d.Audit, IDs: ids.V7{},
		PublicBaseURL: d.Cfg.PublicBaseURL, StaleDays: d.Cfg.StaleDays,
	})
	return classes.NewHandler(svc, currentUserRole)
}

// classProgress là classes.ProgressReader trên learning.ProgressReader: phần trăm trung bình của thành viên active.
type classProgress struct {
	reader learning.ProgressReader
}

func (p classProgress) AvgPercentByClass(ctx context.Context, ex db.Executor, classIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	return p.reader.ClassAveragePercent(ctx, ex, classIDs)
}

// stageVersionReader là courses.StageVersionReader trên stages.VersionRefReader (anti-corruption: chỉ chép trường
// courses cần).
type stageVersionReader struct {
	repo stages.VersionRefReader
}

func (r stageVersionReader) Refs(ctx context.Context, ex db.Executor, versionIDs []uuid.UUID) (map[uuid.UUID]courses.StageVersionRef, error) {
	refs, err := r.repo.Refs(ctx, ex, versionIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]courses.StageVersionRef, len(refs))
	for id, v := range refs {
		out[id] = courses.StageVersionRef{ID: v.ID, StageID: v.StageID, StageCode: v.StageCode, VersionNo: v.VersionNo, Status: v.Status}
	}
	return out, nil
}

// currentUserID là id người dùng của phiên identity hiện tại.
func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	u, ok := identity.CurrentUser(c)
	if !ok {
		return uuid.Nil, false
	}
	return u.ID(), true
}

// currentUserRole là id và vai trò người dùng của phiên identity hiện tại.
func currentUserRole(c *gin.Context) (uuid.UUID, domain.Role, bool) {
	u, ok := identity.CurrentUser(c)
	if !ok {
		return uuid.Nil, "", false
	}
	return u.ID(), u.Role(), true
}

// newMediaService dựng media.Service; learning dùng lại làm media.URLSigner.
func newMediaService(d Deps) *media.Service {
	return media.NewService(d.DB, media.PGRepo{}, d.Storage, d.Clock, ids.V7{}, media.Config{
		Limits: media.Limits{MaxVideoBytes: d.Cfg.MaxVideoBytes, MaxImageBytes: d.Cfg.MaxImageBytes},
		URLTTL: d.Cfg.MediaURLTTL,
	})
}

// newMedia dựng handler upload và cấp URL xem media.
func newMedia(d Deps) *media.Handler {
	return media.NewHandler(newMediaService(d), func(c *gin.Context) (media.Principal, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return media.Principal{}, false
		}
		return media.Principal{ID: u.ID(), Role: u.Role()}, true
	})
}

// newLearning dựng handler học của học viên; quyền theo lớp đọc qua classes.MembershipReader, URL video ký qua
// media.Service.
func newLearning(d Deps) *learning.Handler {
	svc := learning.NewService(learning.Deps{
		DB: d.DB, Tx: db.TxRunner{DB: d.DB},
		Progress: learning.PGProgressRepo{}, Structure: learning.PGCourseStructureRepo{}, MyClasses: learning.PGMyClassesRepo{},
		Members: classes.PGMembershipReader{}, Media: newMediaService(d), Clock: d.Clock,
	})
	return learning.NewHandler(svc, currentUserID)
}

// newReports dựng handler báo cáo lớp và dashboard: chỉ đọc, trong transaction REPEATABLE READ read-only; khóa
// học dùng chặng cũ đọc qua stages.Service.
func newReports(d Deps, outdated stages.OutdatedReader) *reports.Handler {
	svc := reports.NewService(reports.Deps{
		Tx: reports.ReadTx{DB: d.DB}, Reports: reports.PGReportRepo{}, Dashboard: reports.PGDashboardRepo{},
		Progress: learning.PGProgressReader{}, Outdated: outdated, Audit: audit.PGReader{}, Users: identity.PGUserRepo{},
		Clock: d.Clock, StaleDays: d.Cfg.StaleDays,
	})
	return reports.NewHandler(svc, currentUserRole)
}

// userKey là khóa rate limit theo người dùng đã đăng nhập.
func userKey(c *gin.Context) string {
	if u, ok := identity.CurrentUser(c); ok {
		return u.ID().String()
	}
	return ""
}
