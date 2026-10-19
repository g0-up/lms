//go:build integration

package summary_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/grading"
	"lms/api/internal/features/homework"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/learning"
	"lms/api/internal/features/mailer"
	"lms/api/internal/features/media"
	"lms/api/internal/features/quizzes"
	"lms/api/internal/features/reports"
	"lms/api/internal/features/stages"
	"lms/api/internal/features/summary"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/middleware"
	"lms/api/internal/platform/storage"
	"lms/api/internal/platform/testdb"
	"lms/api/internal/seed"
)

var update = flag.Bool("update", false, "ghi lại golden JSON trong testdata/")

// Mật khẩu và khóa chỉ dùng cho DB test.
const (
	itSeedPassword = "Seed-Password-Test-1"
	itOutboxKey    = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	baseURL        = "http://localhost:5173"
)

// Tài khoản theo seed.js.
const (
	adminEmail = "quan.tran@goup.vn"
	huongEmail = "huong.le@goup.vn" // giảng viên basic01, basic03
	baoEmail   = "bao.pham@goup.vn" // giảng viên basic02
	anEmail    = "an.nguyen@gmail.com"
	haEmail    = "ha.hoang@gmail.com"
	thaoEmail  = "thao.vo@gmail.com" // đã rời basic01
	linhEmail  = "linh.do@gmail.com" // học viên basic02
)

// itEnv dựng summary trên DB test đã seed theo seed.js, cùng learning, reports (ba nguồn số % phải khớp nhau) và
// quizzes, homework thật để tạo hoạt động; identity thật cho cookie phiên và RequireRole.
type itEnv struct {
	t        *testing.T
	dbx      *sqlx.DB
	clk      *clock.Fake
	ident    *identity.Service
	quizzes  *quizzes.Service
	homework *homework.Service
	engine   *gin.Engine
	cookies  map[string]string
}

func newIT(t *testing.T) *itEnv {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)
	clk := &clock.Fake{T: time.Now().UTC().Truncate(time.Second)}
	_, err = seed.Run(context.Background(), dbx, config.Config{
		AppEnv: config.EnvDev, PublicBaseURL: baseURL, TempPasswordTTL: 72 * time.Hour,
		SeedPassword: itSeedPassword, OutboxSecretKey: itOutboxKey, Location: loc,
	}, clk, seed.Options{})
	require.NoError(t, err)

	rec := audit.PG{Clock: clk}
	e := &itEnv{t: t, dbx: dbx, clk: clk, cookies: map[string]string{}}
	e.ident = identity.NewService(identity.ServiceDeps{
		DB: dbx, Tx: db.TxRunner{DB: dbx},
		Users: identity.PGUserRepo{}, Sessions: identity.PGSessionRepo{}, Attempts: identity.PGLoginAttemptRepo{},
		Resets: identity.PGResetTokenRepo{}, Clock: clk, Audit: rec,
		Cfg: config.Config{
			SessionTTL: 12 * time.Hour, TempPasswordTTL: 72 * time.Hour, ResetTokenTTL: 30 * time.Minute,
			PasswordMinLength: 8, LoginMaxFailures: 5, LoginLockWindow: 15 * time.Minute,
			PublicBaseURL: baseURL, AppEnv: config.EnvE2E,
		},
	})
	sum := newSummary(dbx, reports.ReadTx{DB: dbx}, clk)
	learn := learning.NewService(learning.Deps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Progress: learning.PGProgressRepo{}, Structure: learning.PGCourseStructureRepo{},
		MyClasses: learning.PGMyClassesRepo{}, Members: classes.PGMembershipReader{}, Clock: clk, Statuses: sum,
		Media: media.NewService(dbx, media.PGRepo{}, storage.NewMemStorage(), clk, ids.V7{}, media.Config{URLTTL: 2 * time.Hour}),
	})
	stageSvc := stages.NewService(stages.Deps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Stages: stages.PGStageRepo{}, Versions: stages.PGStageVersionRepo{},
		Media: media.PGRepo{}, Render: stages.NewMarkdownRenderer(), Clock: clk, Audit: rec, IDs: ids.V7{},
	})
	rep := reports.NewService(reports.Deps{
		Tx: reports.ReadTx{DB: dbx}, Reports: reports.PGReportRepo{}, Dashboard: reports.PGDashboardRepo{},
		Progress: learning.PGProgressReader{}, Outdated: stageSvc, Audit: audit.PGReader{}, Users: identity.PGUserRepo{},
		Clock: clk, StaleDays: 7,
	})
	e.quizzes = quizzes.NewService(quizzes.Deps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Repo: quizzes.PGRepo{}, Members: classes.PGMembershipReader{},
		Progress: learning.PGProgressRepo{}, Clock: clk, IDs: ids.V7{}, Shuffle: quizzes.RandShuffler{},
	})
	e.homework = homework.NewService(homework.Deps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Repo: homework.PGRepo{}, Members: classes.PGMembershipReader{},
		Lessons: learning.PGCourseStructureRepo{}, Completion: learning.PGProgressRepo{},
		Mail: mailer.NewService(mailer.NewOutboxRepo(), nil, clk), Audit: rec, Clock: clk, IDs: ids.V7{},
		Next:          grading.NewService(grading.Deps{Tx: db.TxRunner{DB: dbx}, Repo: grading.PGRepo{}, Clock: clk}),
		PublicBaseURL: baseURL,
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recover(slog.New(slog.DiscardHandler)), middleware.SecurityHeaders())
	authed := engine.Group("/api/v1", identity.NewMiddleware(e.ident, false).SessionAuth())
	actor := func(c *gin.Context) (uuid.UUID, domain.Role, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return uuid.Nil, "", false
		}
		return u.ID(), u.Role(), true
	}
	student, staff := identity.RequireRole(domain.RoleStudent), identity.RequireRole(domain.RoleAdmin, domain.RoleTeacher)
	learning.NewHandler(learn, func(c *gin.Context) (uuid.UUID, bool) {
		id, _, ok := actor(c)
		return id, ok
	}).Register(authed, student)
	reports.NewHandler(rep, actor).Register(authed, staff, identity.RequireRole(domain.RoleAdmin))
	summary.NewHandler(sum, actor).Register(authed, student, staff)
	e.engine = engine
	return e
}

// newSummary nối summary như app/deps.go; read là transaction đọc (test đếm truy vấn bọc nó).
func newSummary(dbx *sqlx.DB, read db.Tx, clk clock.Clock) *summary.Service {
	return summary.NewService(summary.Deps{
		Read: read, Tx: db.TxRunner{DB: dbx}, Repo: summary.PGRepo{}, Members: classes.PGMembershipReader{},
		Audit: audit.PG{Clock: clk}, Clock: clk, IDs: ids.V7{},
	})
}

// ---- Truy vấn dữ liệu seed ----

func (e *itEnv) exec(q string, args ...any) {
	e.t.Helper()
	_, err := e.dbx.Exec(q, args...)
	require.NoError(e.t, err, q)
}

func (e *itEnv) id(q string, args ...any) uuid.UUID {
	e.t.Helper()
	var id uuid.UUID
	require.NoError(e.t, e.dbx.Get(&id, q, args...), q)
	return id
}

func (e *itEnv) count(q string, args ...any) int {
	e.t.Helper()
	var n int
	require.NoError(e.t, e.dbx.Get(&n, q, args...), q)
	return n
}

func (e *itEnv) classID(code string) uuid.UUID {
	return e.id(`SELECT id FROM classes WHERE code = $1`, code)
}

func (e *itEnv) userID(email string) uuid.UUID {
	return e.id(`SELECT id FROM users WHERE email = $1`, email)
}

func (e *itEnv) memberID(classCode, email string) uuid.UUID {
	return e.id(`SELECT cm.id FROM class_members cm JOIN classes c ON c.id = cm.class_id JOIN users u ON u.id = cm.user_id
		WHERE c.code = $1 AND u.email = $2`, classCode, email)
}

// lessonID là học liệu key của phiên bản khóa học basic01 (basic01, basic02, basic03 chung phiên bản).
func (e *itEnv) lessonID(key string) uuid.UUID {
	return e.id(`SELECT l.id FROM classes c
		JOIN course_version_stages cvs ON cvs.course_version_id = c.course_version_id
		JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		WHERE c.code = 'basic01' AND l.lesson_key = $1`, key)
}

// stageOf là phiên bản chặng chứa học liệu key trong khóa của basic01.
func (e *itEnv) stageOf(key string) uuid.UUID {
	return e.id(`SELECT stage_version_id FROM lessons WHERE id = $1`, e.lessonID(key))
}

func (e *itEnv) required(lessonID uuid.UUID) bool {
	e.t.Helper()
	var r bool
	require.NoError(e.t, e.dbx.Get(&r, `SELECT required FROM lessons WHERE id = $1`, lessonID))
	return r
}

// ---- HTTP ----

type apiResp struct {
	code   int
	body   []byte
	header http.Header
}

// login đăng nhập thật qua identity một lần cho mỗi email, trả giá trị cookie phiên.
func (e *itEnv) login(email string) string {
	e.t.Helper()
	if c, ok := e.cookies[email]; ok {
		return c
	}
	res, err := e.ident.Login(context.Background(), email, itSeedPassword, "127.0.0.1", "it")
	require.NoError(e.t, err, "đăng nhập %s", email)
	e.cookies[email] = res.Token.Reveal()
	return e.cookies[email]
}

// serve gọi engine; không đụng t nên gọi được từ goroutine.
func (e *itEnv) serve(cookie, method, path, body string) apiResp {
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: identity.CookieName(false), Value: cookie})
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return apiResp{code: w.Code, body: w.Body.Bytes(), header: w.Header()}
}

func (e *itEnv) call(email, method, path, body string) apiResp {
	e.t.Helper()
	return e.serve(e.login(email), method, path, body)
}

// ok gọi API và bắt buộc 200; route tổng kết còn phải không cache.
func (e *itEnv) ok(email, method, path, body string) []byte {
	e.t.Helper()
	r := e.call(email, method, path, body)
	require.Equal(e.t, http.StatusOK, r.code, "%s %s: %s", method, path, r.body)
	if method == http.MethodGet && strings.HasSuffix(path, "/summary") {
		assert.Equal(e.t, "private, no-store", r.header.Get("Cache-Control"), path)
	}
	return r.body
}

// expectError kiểm mã HTTP, mã lỗi và (nếu có) message.
func (e *itEnv) expectError(email, method, path, body string, status int, code, msg string) {
	e.t.Helper()
	r := e.call(email, method, path, body)
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(e.t, json.Unmarshal(r.body, &env), "%s %s: %s", method, path, r.body)
	require.Equal(e.t, status, r.code, "%s %s: %s", method, path, r.body)
	assert.Equal(e.t, code, env.Error.Code, path)
	if msg != "" {
		assert.Equal(e.t, msg, env.Error.Message, path)
	}
}

func decode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(raw, &v), "%s", raw)
	return v
}

func learnerURL(classID uuid.UUID) string {
	return "/api/v1/me/classes/" + classID.String() + "/summary"
}

func matrixURL(classID uuid.UUID) string { return "/api/v1/classes/" + classID.String() + "/summary" }

func drawerURL(classID, memberID, stageVersionID uuid.UUID) string {
	return fmt.Sprintf("/api/v1/classes/%s/members/%s/stages/%s/summary", classID, memberID, stageVersionID)
}

func reviewURL(classID, memberID, stageVersionID uuid.UUID) string {
	return fmt.Sprintf("/api/v1/classes/%s/members/%s/stages/%s/review", classID, memberID, stageVersionID)
}

func reviewBody(body string) string {
	raw, _ := json.Marshal(map[string]string{"body": body}) //nolint:errchkjson // map chuỗi luôn marshal được
	return string(raw)
}

// ---- Dạng phản hồi (DTO của summary không export) ----

type statusOut struct {
	Done    bool   `json:"done"`
	Label   string `json:"label"`
	Tone    string `json:"tone"`
	Overdue bool   `json:"overdue"`
	Rework  bool   `json:"rework"`
}

type reviewOut struct {
	Body       string `json:"body"`
	AuthorName string `json:"authorName"`
	Revision   int    `json:"revision"`
}

type stageOut struct {
	StageVersionID uuid.UUID `json:"stageVersionId"`
	StageID        uuid.UUID `json:"stageId"`
	Units          []struct {
		Items []struct {
			LessonID uuid.UUID `json:"lessonId"`
			Status   statusOut `json:"status"`
		} `json:"items"`
	} `json:"units"`
	Required int  `json:"required"`
	Done     int  `json:"done"`
	Percent  int  `json:"percent"`
	Complete bool `json:"complete"`
	Missing  []struct {
		LessonID uuid.UUID `json:"lessonId"`
	} `json:"missing"`
	Rework  bool       `json:"rework"`
	Overdue bool       `json:"overdue"`
	Review  *reviewOut `json:"review"`
}

// item là trạng thái học liệu lessonID trong chặng.
func (s stageOut) item(t *testing.T, lessonID uuid.UUID) statusOut {
	t.Helper()
	for _, u := range s.Units {
		for _, it := range u.Items {
			if it.LessonID == lessonID {
				return it.Status
			}
		}
	}
	t.Fatalf("chặng %s không có học liệu %s", s.StageVersionID, lessonID)
	return statusOut{}
}

type courseStageOut struct {
	StageVersionID uuid.UUID `json:"stageVersionId"`
	Name           string    `json:"name"`
	Complete       bool      `json:"complete"`
	MissingCount   int       `json:"missingCount"`
	Assessments    []struct {
		LessonID    uuid.UUID `json:"lessonId"`
		BestPercent *int      `json:"bestPercent"`
		Passed      bool      `json:"passed"`
	} `json:"assessments"`
	Homework []struct {
		LessonID uuid.UUID `json:"lessonId"`
		Total    *int      `json:"total"`
		Max      *int      `json:"max"`
		Status   string    `json:"status"`
	} `json:"homework"`
}

type courseOut struct {
	StagesComplete int              `json:"stagesComplete"`
	StagesTotal    int              `json:"stagesTotal"`
	Stages         []courseStageOut `json:"stages"`
}

type learnerOut struct {
	Stages        []stageOut `json:"stages"`
	CoursePercent int        `json:"coursePercent"`
	RequiredDone  int        `json:"requiredDone"`
	RequiredTotal int        `json:"requiredTotal"`
	Course        courseOut  `json:"course"`
}

// courseStage là chặng stageVersionID trong tổng kết toàn khóa.
func (l learnerOut) courseStage(t *testing.T, stageVersionID uuid.UUID) courseStageOut {
	t.Helper()
	for _, s := range l.Course.Stages {
		if s.StageVersionID == stageVersionID {
			return s
		}
	}
	t.Fatalf("tổng kết toàn khóa không có chặng %s", stageVersionID)
	return courseStageOut{}
}

func (l learnerOut) stage(t *testing.T, stageVersionID uuid.UUID) stageOut {
	t.Helper()
	for _, s := range l.Stages {
		if s.StageVersionID == stageVersionID {
			return s
		}
	}
	t.Fatalf("không có chặng %s", stageVersionID)
	return stageOut{}
}

type cellOut struct {
	StageVersionID uuid.UUID `json:"stageVersionId"`
	Complete       bool      `json:"complete"`
	Done           int       `json:"done"`
	Required       int       `json:"required"`
	Missing        int       `json:"missing"`
	Rework         bool      `json:"rework"`
	Overdue        bool      `json:"overdue"`
	HasReview      bool      `json:"hasReview"`
}

type rowOut struct {
	MemberID      uuid.UUID `json:"memberId"`
	Email         string    `json:"email"`
	CoursePercent int       `json:"coursePercent"`
	Cells         []cellOut `json:"cells"`
}

type matrixOut struct {
	Rows []rowOut `json:"rows"`
}

func (m matrixOut) row(t *testing.T, email string) rowOut {
	t.Helper()
	for _, r := range m.Rows {
		if r.Email == email {
			return r
		}
	}
	t.Fatalf("ma trận không có dòng %s", email)
	return rowOut{}
}

type drawerOut struct {
	Member struct {
		ID uuid.UUID `json:"id"`
	} `json:"member"`
	Summary stageOut `json:"summary"`
}

// ---- Phạm vi truy cập ----

// Giảng viên lớp khác nhận 403 trên ma trận, drawer và nhận xét; giảng viên phụ trách dùng id thành viên của lớp
// khác hoặc chặng ngoài khóa học thì 404 và không ghi dòng nhận xét nào. Admin xem được mọi lớp.
func TestStaffScope(t *testing.T) {
	e := newIT(t)
	basic01, basic02 := e.classID("basic01"), e.classID("basic02")
	an, linhInBasic02 := e.memberID("basic01", anEmail), e.memberID("basic02", linhEmail)
	ds := e.stageOf("ds-hw")
	// Bản nháp mới của chặng Data structure: cùng chặng nhưng không thuộc phiên bản khóa học của lớp.
	foreignStage := uuid.Must(uuid.NewV7())
	e.exec(`INSERT INTO stage_versions (id, stage_id, version_no, status, title, created_by)
		SELECT $1, stage_id, version_no + 1, 'draft', title, $2 FROM stage_versions WHERE id = $3`, foreignStage, e.userID(adminEmail), ds)
	reviews := func() int { return e.count(`SELECT count(*) FROM stage_reviews`) }
	audits := func() int { return e.count(`SELECT count(*) FROM audit_logs WHERE action = 'stage_review.saved'`) }
	before, auditsBefore := reviews(), audits()

	const notOwn = "Bạn chỉ xem được lớp mình phụ trách."
	e.expectError(baoEmail, http.MethodGet, matrixURL(basic01), "", http.StatusForbidden, "FORBIDDEN", notOwn)
	e.expectError(baoEmail, http.MethodGet, drawerURL(basic01, an, ds), "", http.StatusForbidden, "FORBIDDEN", notOwn)
	e.expectError(baoEmail, http.MethodPut, reviewURL(basic01, an, ds), reviewBody("Tốt."), http.StatusForbidden, "FORBIDDEN",
		"Bạn chỉ nhận xét học viên của lớp mình phụ trách.")
	e.ok(baoEmail, http.MethodGet, matrixURL(basic02), "")

	const noMember = "Không tìm thấy thành viên."
	e.expectError(huongEmail, http.MethodGet, drawerURL(basic01, linhInBasic02, ds), "", http.StatusNotFound, "NOT_FOUND", noMember)
	e.expectError(huongEmail, http.MethodPut, reviewURL(basic01, linhInBasic02, ds), reviewBody("Tốt."), http.StatusNotFound, "NOT_FOUND", noMember)
	const noStage = "Chặng không thuộc khóa học của lớp này."
	e.expectError(huongEmail, http.MethodGet, drawerURL(basic01, an, foreignStage), "", http.StatusNotFound, "NOT_FOUND", noStage)
	e.expectError(huongEmail, http.MethodPut, reviewURL(basic01, an, foreignStage), reviewBody("Tốt."), http.StatusNotFound, "NOT_FOUND", noStage)
	unknown := uuid.Must(uuid.NewV7())
	e.expectError(huongEmail, http.MethodGet, matrixURL(unknown), "", http.StatusNotFound, "NOT_FOUND", "Không tìm thấy lớp.")
	e.expectError(huongEmail, http.MethodPut, reviewURL(unknown, an, ds), reviewBody("Tốt."), http.StatusNotFound, "NOT_FOUND", "Không tìm thấy lớp.")
	assert.Equal(t, before, reviews(), "đường bị từ chối không ghi nhận xét")
	assert.Equal(t, auditsBefore, audits(), "đường bị từ chối không ghi audit")

	// Học viên không gọi được route của staff.
	e.expectError(anEmail, http.MethodGet, matrixURL(basic01), "", http.StatusForbidden, "FORBIDDEN", "")
	e.ok(adminEmail, http.MethodGet, matrixURL(basic01), "")
	e.ok(adminEmail, http.MethodGet, drawerURL(basic01, an, ds), "")
}

// Tổng kết của học viên: chỉ thành viên đang học của lớp; học viên lớp khác và học viên đã rời lớp nhận 403.
func TestLearnerScope(t *testing.T) {
	e := newIT(t)
	basic01 := e.classID("basic01")

	v := decode[learnerOut](t, e.ok(anEmail, http.MethodGet, learnerURL(basic01), ""))
	ds := v.stage(t, e.stageOf("ds-hw"))
	require.NotNil(t, ds.Review, "An thấy nhận xét chặng Data structure của giảng viên")
	assert.Equal(t, "Lê Thu Hương", ds.Review.AuthorName)

	e.expectError(linhEmail, http.MethodGet, learnerURL(basic01), "", http.StatusForbidden, "FORBIDDEN", "Bạn không học lớp này.")
	e.expectError(huongEmail, http.MethodGet, learnerURL(basic01), "", http.StatusForbidden, "FORBIDDEN", "")
	e.expectError(anEmail, http.MethodGet, learnerURL(uuid.Must(uuid.NewV7())), "", http.StatusNotFound, "NOT_FOUND", "Không tìm thấy lớp.")

	// Tài khoản Thảo bị vô hiệu hóa nên không đăng nhập được; gọi thẳng service để kiểm quy tắc rời lớp.
	sum := newSummary(e.dbx, reports.ReadTx{DB: e.dbx}, e.clk)
	_, err := sum.LearnerSummary(context.Background(), e.userID(thaoEmail), basic01)
	require.ErrorIs(t, err, summary.ErrMemberDropped)
}

// ---- Nhận xét tổng kết chặng ----

// Lưu nhận xét lần đầu revision 1, mỗi lần sửa + 1 và người viết là người sửa; audit ghi cùng transaction với
// before là nội dung cũ. Nội dung rỗng hoặc quá 5000 ký tự là 422 và không ghi gì.
func TestSaveReviewRevisionsAndAudit(t *testing.T) {
	e := newIT(t)
	basic01 := e.classID("basic01")
	ha, ds := e.memberID("basic01", haEmail), e.stageOf("ds-hw")
	url := reviewURL(basic01, ha, ds)
	audits := func() int {
		return e.count(`SELECT count(*) FROM audit_logs WHERE action = 'stage_review.saved' AND after->>'memberId' = $1`, ha.String())
	}

	e.expectError(huongEmail, http.MethodPut, url, reviewBody(" \n "), http.StatusUnprocessableEntity, "VALIDATION_FAILED",
		"Nhập nhận xét tổng kết chặng.")
	e.expectError(huongEmail, http.MethodPut, url, reviewBody(strings.Repeat("ữ", 5001)), http.StatusUnprocessableEntity,
		"VALIDATION_FAILED", "Nhận xét tối đa 5000 ký tự.")
	require.Zero(t, audits())
	require.Zero(t, e.count(`SELECT count(*) FROM stage_reviews WHERE class_member_id = $1`, ha))

	first := decode[reviewOut](t, e.ok(huongEmail, http.MethodPut, url, reviewBody("  Nắm chắc stack.  ")))
	assert.Equal(t, reviewOut{Body: "Nắm chắc stack.", AuthorName: "Lê Thu Hương", Revision: 1}, first)
	e.clk.Advance(time.Minute)
	second := decode[reviewOut](t, e.ok(huongEmail, http.MethodPut, url, reviewBody("Nắm chắc stack, cần luyện hash.")))
	assert.Equal(t, 2, second.Revision)
	e.clk.Advance(time.Minute)
	third := decode[reviewOut](t, e.ok(adminEmail, http.MethodPut, url, reviewBody(strings.Repeat("ữ", 5000))))
	assert.Equal(t, 3, third.Revision, "sửa hai lần thì revision = 3")
	assert.Equal(t, "Trần Minh Quân", third.AuthorName, "người viết là người sửa gần nhất")

	var row struct {
		Revision  int       `db:"revision"`
		AuthorID  uuid.UUID `db:"author_id"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}
	require.NoError(t, e.dbx.Get(&row, `SELECT revision, author_id, created_at, updated_at FROM stage_reviews
		WHERE class_member_id = $1 AND stage_version_id = $2`, ha, ds))
	assert.Equal(t, 3, row.Revision)
	assert.Equal(t, e.userID(adminEmail), row.AuthorID)
	assert.True(t, row.UpdatedAt.Sub(row.CreatedAt) == 2*time.Minute, "created_at giữ mốc lần đầu, updated_at là lần sửa")

	type auditRow struct {
		ActorID uuid.UUID `db:"actor_id"`
		Target  uuid.UUID `db:"target_id"`
		Before  []byte    `db:"before"`
		After   []byte    `db:"after"`
	}
	var logs []auditRow
	require.NoError(t, e.dbx.Select(&logs, `SELECT actor_id, target_id, before, after FROM audit_logs
		WHERE action = 'stage_review.saved' AND after->>'memberId' = $1 ORDER BY (after->>'revision')::int`, ha.String()))
	require.Len(t, logs, 3)
	for _, l := range logs {
		assert.Equal(t, basic01, l.Target)
	}
	assert.Nil(t, logs[0].Before, "lần lưu đầu không có before")
	type payload struct {
		Body        string    `json:"body"`
		Revision    int       `json:"revision"`
		AuthorID    uuid.UUID `json:"authorId"`
		StageName   string    `json:"stageName"`
		StudentName string    `json:"studentName"`
		ClassCode   string    `json:"classCode"`
	}
	before2, after2 := decode[payload](t, logs[1].Before), decode[payload](t, logs[1].After)
	assert.Equal(t, payload{Body: "Nắm chắc stack.", Revision: 1, AuthorID: e.userID(huongEmail), StageName: "Data structure",
		StudentName: "Hoàng Thu Hà", ClassCode: "basic01"}, before2)
	assert.Equal(t, 2, after2.Revision)
	before3, after3 := decode[payload](t, logs[2].Before), decode[payload](t, logs[2].After)
	assert.Equal(t, "Nắm chắc stack, cần luyện hash.", before3.Body)
	assert.Equal(t, e.userID(huongEmail), before3.AuthorID)
	assert.Equal(t, e.userID(adminEmail), after3.AuthorID)
	assert.Equal(t, e.userID(adminEmail), logs[2].ActorID)

	// Drawer, học viên đều thấy bản mới nhất; ma trận bật cờ đã có nhận xét.
	d := decode[drawerOut](t, e.ok(huongEmail, http.MethodGet, drawerURL(basic01, ha, ds), ""))
	assert.Equal(t, ha, d.Member.ID)
	require.NotNil(t, d.Summary.Review)
	assert.Equal(t, reviewOut{Body: strings.Repeat("ữ", 5000), AuthorName: "Trần Minh Quân", Revision: 3}, *d.Summary.Review)
	v := decode[learnerOut](t, e.ok(haEmail, http.MethodGet, learnerURL(basic01), ""))
	require.NotNil(t, v.stage(t, ds).Review)
	assert.Equal(t, 3, v.stage(t, ds).Review.Revision)
	m := decode[matrixOut](t, e.ok(huongEmail, http.MethodGet, matrixURL(basic01), ""))
	for _, c := range m.row(t, haEmail).Cells {
		assert.Equal(t, c.StageVersionID == ds, c.HasReview, "chỉ chặng vừa nhận xét có cờ")
	}
}

// Hai lần lưu đồng thời trên cùng (thành viên, chặng) chưa có nhận xét: khóa class_members xếp hàng hai transaction
// nên đúng một dòng, revision 1 rồi 2, không lần nào lỗi.
func TestSaveReviewConcurrentSavesSerialize(t *testing.T) {
	e := newIT(t)
	basic01 := e.classID("basic01")
	ha, ds := e.memberID("basic01", haEmail), e.stageOf("ds-hw")
	url := reviewURL(basic01, ha, ds)
	cookies := []string{e.login(huongEmail), e.login(adminEmail)}

	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make([]apiResp, len(cookies))
	for i, c := range cookies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = e.serve(c, http.MethodPut, url, reviewBody(fmt.Sprintf("Nhận xét %d", i)))
		}()
	}
	close(start)
	wg.Wait()

	revisions := []int{}
	for _, r := range results {
		require.Equal(t, http.StatusOK, r.code, "%s", r.body)
		revisions = append(revisions, decode[reviewOut](t, r.body).Revision)
	}
	sort.Ints(revisions)
	assert.Equal(t, []int{1, 2}, revisions)
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM stage_reviews WHERE class_member_id = $1 AND stage_version_id = $2`, ha, ds))
	assert.Equal(t, 2, e.count(`SELECT revision FROM stage_reviews WHERE class_member_id = $1 AND stage_version_id = $2`, ha, ds))
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM audit_logs WHERE action = 'stage_review.saved'
		AND after->>'memberId' = $1 AND before IS NOT NULL`, ha.String()))
}

// ---- Golden từ seed ----

func TestGoldens(t *testing.T) {
	e := newIT(t)
	basic01 := e.classID("basic01")
	assertGolden(t, "class-summary.json", e.ok(huongEmail, http.MethodGet, matrixURL(basic01), ""))
	assertGolden(t, "learner-summary.json", e.ok(anEmail, http.MethodGet, learnerURL(basic01), ""))
	assertGolden(t, "member-stage-summary.json",
		e.ok(huongEmail, http.MethodGet, drawerURL(basic01, e.memberID("basic01", anEmail), e.stageOf("ds-hw")), ""))
}

// Tổng kết toàn khóa không có điểm tổng: không khóa nào trong JSON (ở mọi độ sâu) là totalScore hay courseScore.
func TestLearnerSummaryHasNoCourseTotalScore(t *testing.T) {
	e := newIT(t)
	var body any
	require.NoError(t, json.Unmarshal(e.ok(anEmail, http.MethodGet, learnerURL(e.classID("basic01")), ""), &body))
	var walk func(v any, path string)
	walk = func(v any, path string) {
		switch n := v.(type) {
		case map[string]any:
			for k, c := range n {
				assert.NotContains(t, []string{"totalScore", "courseScore"}, k, "trường điểm tổng ở %s", path)
				walk(c, path+"."+k)
			}
		case []any:
			for i, c := range n {
				walk(c, fmt.Sprintf("%s[%d]", path, i))
			}
		}
	}
	walk(body, "$")
	top, ok := body.(map[string]any)
	require.True(t, ok)
	course, ok := top["course"].(map[string]any)
	require.True(t, ok, "có tổng kết toàn khóa")
	assert.ElementsMatch(t, []string{"stagesComplete", "stagesTotal", "stages"}, keys(course))
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ---- Một định nghĩa hoàn thành ----

// sources là số % khóa học và từng chặng của một học viên đọc từ ba nguồn: lộ trình của learning, báo cáo lớp của
// reports và tổng kết (cả bản của học viên lẫn dòng ma trận của giảng viên).
func (e *itEnv) assertSourcesAgree(step, email string, classID uuid.UUID) learnerOut {
	e.t.Helper()
	t := e.t
	path := "/api/v1/me/classes/" + classID.String()
	road := decode[learning.RoadmapDTO](t, e.ok(email, http.MethodGet, path, ""))
	rep := decode[reports.ClassReportDTO](t, e.ok(huongEmail, http.MethodGet, "/api/v1/classes/"+classID.String()+"/report", ""))
	var repRow *reports.ReportRowDTO
	for i := range rep.Rows {
		if rep.Rows[i].Email == email {
			repRow = &rep.Rows[i]
		}
	}
	require.NotNil(t, repRow, "%s: báo cáo không có dòng %s", step, email)
	sum := decode[learnerOut](t, e.ok(email, http.MethodGet, learnerURL(classID), ""))
	row := decode[matrixOut](t, e.ok(huongEmail, http.MethodGet, matrixURL(classID), "")).row(t, email)

	assert.Equal(t, road.Percent, repRow.Percent, "%s: %% khóa learning và reports", step)
	assert.Equal(t, road.Percent, sum.CoursePercent, "%s: %% khóa learning và summary", step)
	assert.Equal(t, road.Percent, row.CoursePercent, "%s: %% khóa learning và ma trận", step)
	assert.Equal(t, road.RequiredDone, sum.RequiredDone, "%s: số bắt buộc đã xong", step)
	assert.Equal(t, road.RequiredTotal, sum.RequiredTotal, "%s: tổng số bắt buộc", step)

	require.Len(t, sum.Stages, len(road.Stages), step)
	require.Len(t, row.Cells, len(sum.Stages), step)
	require.Len(t, sum.Course.Stages, len(sum.Stages), step)
	assert.Equal(t, len(sum.Stages), sum.Course.StagesTotal, "%s: tổng số chặng toàn khóa", step)
	complete := 0
	for i, s := range sum.Stages {
		if s.Complete {
			complete++
		}
		cs := sum.Course.Stages[i]
		assert.Equal(t, s.StageVersionID, cs.StageVersionID, "%s: thứ tự chặng toàn khóa", step)
		assert.Equal(t, s.Complete, cs.Complete, "%s: chặng toàn khóa hoàn thành", step)
		assert.Equal(t, len(s.Missing), cs.MissingCount, "%s: chặng toàn khóa còn thiếu", step)
	}
	assert.Equal(t, complete, sum.Course.StagesComplete, "%s: số chặng hoàn thành toàn khóa", step)
	repStages := map[string]reports.StagePercentDTO{}
	for _, sp := range repRow.StagePercents {
		repStages[sp.StageID] = sp
	}
	for i, rs := range road.Stages {
		s, c := sum.Stages[i], row.Cells[i]
		require.Equal(t, rs.ID, s.StageID.String(), "%s: thứ tự chặng", step)
		assert.Equal(t, rs.RequiredDone, s.Done, "%s: chặng %s đã xong (summary)", step, rs.Name)
		assert.Equal(t, rs.RequiredTotal, s.Required, "%s: chặng %s bắt buộc (summary)", step, rs.Name)
		assert.Equal(t, rs.RequiredDone, repStages[rs.ID].RequiredDone, "%s: chặng %s đã xong (reports)", step, rs.Name)
		assert.Equal(t, rs.RequiredTotal, repStages[rs.ID].RequiredTotal, "%s: chặng %s bắt buộc (reports)", step, rs.Name)
		assert.Equal(t, repStages[rs.ID].Percent, s.Percent, "%s: %% chặng %s", step, rs.Name)
		assert.Equal(t, s.Done, c.Done, "%s: ô ma trận chặng %s", step, rs.Name)
		assert.Equal(t, s.Complete, c.Complete, "%s: ô ma trận chặng %s", step, rs.Name)
		assert.Equal(t, len(s.Missing), c.Missing, "%s: ô ma trận chặng %s", step, rs.Name)
		assert.Equal(t, s.Required > 0 && s.Done == s.Required, s.Complete, "%s: chặng %s hoàn thành", step, rs.Name)
	}
	return sum
}

// correctOptions là các đáp án đúng của từng câu hỏi của quiz lessonID.
func (e *itEnv) correctOptions(lessonID uuid.UUID) map[uuid.UUID][]uuid.UUID {
	e.t.Helper()
	var rows []struct {
		Question uuid.UUID `db:"question_id"`
		Option   uuid.UUID `db:"id"`
	}
	require.NoError(e.t, e.dbx.Select(&rows, `SELECT o.question_id, o.id FROM question_options o
		JOIN questions q ON q.id = o.question_id WHERE q.lesson_id = $1 AND o.is_correct ORDER BY o.position`, lessonID))
	out := map[uuid.UUID][]uuid.UUID{}
	for _, r := range rows {
		out[r.Question] = append(out[r.Question], r.Option)
	}
	return out
}

// passQuiz làm một lượt quiz qua service quizzes thật, trả lời đúng mọi câu rồi nộp.
func (e *itEnv) passQuiz(userID, classID, lessonID uuid.UUID) {
	e.t.Helper()
	ctx := context.Background()
	st, err := e.quizzes.Start(ctx, userID, classID, lessonID)
	require.NoError(e.t, err)
	correct := e.correctOptions(lessonID)
	for _, qid := range st.View.Attempt.QuestionOrder {
		_, err := e.quizzes.SaveAnswer(ctx, userID, st.View.Attempt.ID, qid, correct[qid])
		require.NoError(e.t, err)
	}
	_, err = e.quizzes.Submit(ctx, userID, st.View.Attempt.ID)
	require.NoError(e.t, err)
}

// grade chấm bản nộp mới nhất của memberID cho homework lessonID: mọi tiêu chí đủ điểm hoặc 0 điểm.
func (e *itEnv) grade(memberID, lessonID uuid.UUID, full bool) {
	e.t.Helper()
	var criteria []struct {
		ID  uuid.UUID `db:"id"`
		Max int       `db:"max_points"`
	}
	require.NoError(e.t, e.dbx.Select(&criteria, `SELECT id, max_points FROM rubric_criteria WHERE lesson_id = $1 ORDER BY position`, lessonID))
	scores := make([]homework.ScoreInput, len(criteria))
	for i, c := range criteria {
		p := 0
		if full {
			p = c.Max
		}
		scores[i] = homework.ScoreInput{CriterionID: c.ID, Points: &p}
	}
	sub := e.id(`SELECT id FROM submissions WHERE class_member_id = $1 AND lesson_id = $2 ORDER BY attempt_no DESC LIMIT 1`, memberID, lessonID)
	_, err := e.homework.Grade(context.Background(), homework.Actor{ID: e.userID(huongEmail), Role: domain.RoleTeacher}, sub,
		homework.GradeInput{Scores: scores, Comment: "Nhận xét của giảng viên."})
	require.NoError(e.t, err)
}

// Số % của một học viên khớp giữa learning, reports và summary sau mỗi loại sự kiện quyết định hoàn thành: tích
// video, nộp quiz luyện tập, đạt quiz bài tập, chấm đạt homework rồi chấm lại thành cần làm lại.
func TestCompletionAgreesAcrossSources(t *testing.T) {
	e := newIT(t)
	ctx := context.Background()
	basic01 := e.classID("basic01")
	haUser, haMember := e.userID(haEmail), e.memberID("basic01", haEmail)
	ds := e.stageOf("ds-hw")
	hash, practice, test, hw := e.lessonID("ds-hash"), e.lessonID("ds-quiz"), e.lessonID("ds-test"), e.lessonID("ds-hw")

	base := e.assertSourcesAgree("ban đầu", haEmail, basic01)
	hwStatus := base.stage(t, ds).item(t, hw)
	assert.Equal(t, statusOut{Label: "Quá hạn", Tone: "danger", Overdue: true}, hwStatus, "hạn ds-hw đã qua 20 ngày, chưa nộp")
	assert.True(t, base.stage(t, ds).Overdue)

	// step chạy một sự kiện rồi kiểm số bắt buộc đã xong đổi đúng delta và ba nguồn vẫn khớp.
	prev := base
	step := func(name string, lessonID uuid.UUID, delta int, run func()) learnerOut {
		t.Helper()
		run()
		if !e.required(lessonID) {
			delta = 0
		}
		got := e.assertSourcesAgree(name, haEmail, basic01)
		assert.Equal(t, prev.RequiredDone+delta, got.RequiredDone, "%s: số bắt buộc đã xong", name)
		prev = got
		return got
	}

	v := step("tích video", hash, 1, func() {
		e.ok(haEmail, http.MethodPut, "/api/v1/me/classes/"+basic01.String()+"/lessons/"+hash.String()+"/completion", `{"completed":true}`)
	})
	assert.True(t, v.stage(t, ds).item(t, hash).Done)

	v = step("nộp quiz luyện tập", practice, 1, func() { e.passQuiz(haUser, basic01, practice) })
	assert.Equal(t, statusOut{Done: true, Label: "Đã làm", Tone: "ok"}, v.stage(t, ds).item(t, practice))

	assessment := base.courseStage(t, ds).Assessments
	require.Len(t, assessment, 1, "chặng Data structure có một quiz bài tập")
	assert.Equal(t, test, assessment[0].LessonID)
	assert.Nil(t, assessment[0].BestPercent, "chưa nộp lượt nào thì chưa có điểm")
	assert.False(t, assessment[0].Passed)

	v = step("đạt quiz bài tập", test, 1, func() { e.passQuiz(haUser, basic01, test) })
	assert.Equal(t, statusOut{Done: true, Label: "Đạt 100%", Tone: "ok"}, v.stage(t, ds).item(t, test))
	assessment = v.courseStage(t, ds).Assessments
	require.NotNil(t, assessment[0].BestPercent)
	assert.Equal(t, 100, *assessment[0].BestPercent)
	assert.True(t, assessment[0].Passed)

	v = step("nộp homework muộn", hw, 0, func() {
		_, err := e.homework.Submit(ctx, haUser, basic01, hw, homework.SubmitInput{RepoURL: "https://github.com/hahoang/ds-stack"})
		require.NoError(t, err)
	})
	assert.Equal(t, statusOut{Label: "Chờ chấm", Tone: "info"}, v.stage(t, ds).item(t, hw), "đã nộp thì hết quá hạn")
	assert.False(t, v.stage(t, ds).Overdue)

	homeworkScore := v.courseStage(t, ds).Homework
	require.Len(t, homeworkScore, 1, "chặng Data structure có một homework")
	assert.Equal(t, hw, homeworkScore[0].LessonID)
	assert.Equal(t, "pending", homeworkScore[0].Status)
	assert.Nil(t, homeworkScore[0].Total, "chưa chấm thì chưa có điểm")

	v = step("chấm đạt homework", hw, 1, func() { e.grade(haMember, hw, true) })
	assert.Equal(t, statusOut{Done: true, Label: "Đạt", Tone: "ok"}, v.stage(t, ds).item(t, hw))
	assert.True(t, v.stage(t, ds).Complete, "đủ mọi học liệu bắt buộc của chặng")
	homeworkScore = v.courseStage(t, ds).Homework
	assert.Equal(t, "passed", homeworkScore[0].Status)
	require.NotNil(t, homeworkScore[0].Total)
	require.NotNil(t, homeworkScore[0].Max)
	assert.Equal(t, *homeworkScore[0].Max, *homeworkScore[0].Total, "chấm đủ điểm mọi tiêu chí")
	assert.True(t, v.courseStage(t, ds).Complete)

	v = step("chấm lại thành cần làm lại", hw, -1, func() { e.grade(haMember, hw, false) })
	assert.Equal(t, statusOut{Label: "Cần làm lại", Tone: "warn", Rework: true}, v.stage(t, ds).item(t, hw))
	assert.Equal(t, "rework", v.courseStage(t, ds).Homework[0].Status)
	assert.True(t, v.stage(t, ds).Rework)
	assert.False(t, v.stage(t, ds).Complete)
	cells := decode[matrixOut](t, e.ok(huongEmail, http.MethodGet, matrixURL(basic01), "")).row(t, haEmail).Cells
	for _, c := range cells {
		if c.StageVersionID == ds {
			assert.True(t, c.Rework, "ô ma trận của chặng có cờ cần làm lại")
		}
	}
}

// Chặng không có học liệu bắt buộc: chưa hoàn thành, 0%, và tổng của khóa vẫn khớp learning, reports.
func TestStageWithoutRequiredLessons(t *testing.T) {
	e := newIT(t)
	basic01 := e.classID("basic01")
	ds := e.stageOf("ds-hw")
	e.exec(`UPDATE lessons SET required = false WHERE stage_version_id = $1`, ds)

	v := e.assertSourcesAgree("chặng không bắt buộc", anEmail, basic01)
	st := v.stage(t, ds)
	assert.Zero(t, st.Required)
	assert.Zero(t, st.Percent)
	assert.False(t, st.Complete, "chặng không có học liệu bắt buộc không tính là hoàn thành")
	assert.Empty(t, st.Missing)
}

// ---- Số truy vấn ----

// countingTx bọc db.Tx và đếm mọi truy vấn chạy qua executor của transaction.
type countingTx struct {
	inner db.Tx
	n     *int
}

func (c countingTx) Transact(ctx context.Context, fn func(tx db.Executor) error) error {
	return c.inner.Transact(ctx, func(tx db.Executor) error { return fn(countingEx{Executor: tx, n: c.n}) })
}

type countingEx struct {
	db.Executor
	n *int
}

func (c countingEx) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	*c.n++
	return c.Executor.QueryContext(ctx, q, args...)
}

func (c countingEx) QueryxContext(ctx context.Context, q string, args ...any) (*sqlx.Rows, error) {
	*c.n++
	return c.Executor.QueryxContext(ctx, q, args...)
}

func (c countingEx) QueryRowxContext(ctx context.Context, q string, args ...any) *sqlx.Row {
	*c.n++
	return c.Executor.QueryRowxContext(ctx, q, args...)
}

func (c countingEx) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	*c.n++
	return c.Executor.ExecContext(ctx, q, args...)
}

func (c countingEx) PrepareContext(ctx context.Context, q string) (*sql.Stmt, error) {
	*c.n++
	return c.Executor.PrepareContext(ctx, q)
}

// Ma trận chạy số truy vấn cố định: thêm học viên (có tiến độ, bài nộp, lượt quiz) không làm tăng số truy vấn.
func TestClassMatrixQueryCountIndependentOfMembers(t *testing.T) {
	e := newIT(t)
	basic01 := e.classID("basic01")
	n := 0
	sum := newSummary(e.dbx, countingTx{inner: reports.ReadTx{DB: e.dbx}, n: &n}, e.clk)
	admin := summary.Actor{ID: e.userID(adminEmail), Role: domain.RoleAdmin}

	m, err := sum.ClassMatrix(context.Background(), admin, basic01)
	require.NoError(t, err)
	before, rows := n, len(m.Rows)
	require.Positive(t, before)

	const extra = 5
	const hash = `$argon2id$v=19$m=19456,t=2,p=1$hVgN0I6aZI+AWq7rW6j/XQ$i8SWRmwDmbK6kHOJBAyuz4EOO7oK2G/nLDOcK3Q/NHE`
	now := e.clk.Now()
	video, hw, test := e.lessonID("ds-hash"), e.lessonID("ds-hw"), e.lessonID("ds-test")
	for i := range extra {
		userID, memberID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
		email := fmt.Sprintf("extra%d@example.com", i)
		e.exec(`INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash)
			VALUES ($1, $2, $2, $3, 'student', 'active', $4)`, userID, email, fmt.Sprintf("Học viên %d", i), hash)
		e.exec(`INSERT INTO class_members (id, class_id, user_id, status, joined_at) VALUES ($1, $2, $3, 'active', $4)`,
			memberID, basic01, userID, now.Add(-48*time.Hour))
		e.exec(`INSERT INTO lesson_progress (class_member_id, lesson_id, first_opened_at, completed_at, updated_at)
			VALUES ($1, $2, $3, $3, $3)`, memberID, video, now.Add(-time.Hour))
		e.exec(`INSERT INTO submissions (id, class_member_id, lesson_id, attempt_no, repo_url, submitted_at, status)
			VALUES ($1, $2, $3, 1, 'https://github.com/x/y', $4, 'pending')`, uuid.Must(uuid.NewV7()), memberID, hw, now.Add(-time.Hour))
		e.exec(`INSERT INTO quiz_attempts (id, class_member_id, lesson_id, attempt_no, question_order, option_order, started_at,
			submitted_at, auto_submitted, score, max_score, passed, review_status)
			VALUES ($1, $2, $3, 1, '{}', '{}', $4, $4, false, 3, 10, false, 'pending')`, uuid.Must(uuid.NewV7()), memberID, test, now.Add(-time.Hour))
	}

	n = 0
	m, err = sum.ClassMatrix(context.Background(), admin, basic01)
	require.NoError(t, err)
	assert.Len(t, m.Rows, rows+extra)
	assert.Equal(t, before, n, "số truy vấn không đổi khi thêm %d học viên", extra)
}

// ---- Golden ----

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizeGolden thay thời điểm (khóa "at" hoặc kết thúc bằng "At") bằng mốc cố định và id sinh lúc seed bằng id
// giả theo thứ tự xuất hiện, để golden ổn định giữa các lần chạy.
func normalizeGolden(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal(raw, &v), "golden: %s", raw)
	seen := map[string]string{}
	var walk func(key string, v any) any
	walk = func(key string, v any) any {
		switch x := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				x[k] = walk(k, x[k])
			}
			return x
		case []any:
			for i := range x {
				x[i] = walk(key, x[i])
			}
			return x
		case string:
			if key == "at" || strings.HasSuffix(key, "At") {
				return "2026-10-05T08:00:00Z"
			}
			return uuidPattern.ReplaceAllStringFunc(x, func(id string) string {
				if p, ok := seen[id]; ok {
					return p
				}
				p := fmt.Sprintf("0199ffff-0000-7000-8000-%012d", len(seen)+1)
				seen[id] = p
				return p
			})
		default:
			return x
		}
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	require.NoError(t, enc.Encode(walk("", v)))
	return out.Bytes()
}

func assertGolden(t *testing.T, name string, raw []byte) {
	t.Helper()
	got := normalizeGolden(t, raw)
	path := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644)) //nolint:gosec // golden JSON công khai trong repo
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // đường dẫn cố định trong testdata
	require.NoError(t, err, "golden %s (chạy lại với -update)", name)
	assert.Equal(t, string(want), string(got), "golden %s khác (chạy lại với -update nếu thay đổi là chủ ý)", name)
}
