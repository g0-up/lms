//go:build integration

package learning

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/media"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/middleware"
	"lms/api/internal/platform/storage"
	"lms/api/internal/platform/testdb"
)

var (
	adminID       = uuid.MustParse(testdb.AdminQuanTranID)
	huongID       = uuid.MustParse(testdb.TeacherHuongLeID)
	anID          = uuid.MustParse(testdb.StudentAnNguyenID)
	basic01ID     = uuid.MustParse(testdb.ClassBasic01ID)
	memberAnID    = uuid.MustParse(testdb.MemberAnBasic01ID)
	basicV1ID     = uuid.MustParse(testdb.CourseBasicV1ID)
	stageDBID     = uuid.MustParse(testdb.StageDBID)
	stageDBV1ID   = uuid.MustParse(testdb.StageDBV1ID)
	lessonTableID = uuid.MustParse(testdb.LessonDBTableV1ID)
	lessonIndexID = uuid.MustParse(testdb.LessonDBIndexV1ID)
	lessonV2ID    = uuid.MustParse(testdb.LessonDBTableV2ID) // thuộc DB v2 nháp, không thuộc BASIC v1
)

// Dữ liệu thêm theo prototype/seed.js, chèn bằng SQL trong test; id cố định (tiền tố 01990000-) để golden ổn định.
var (
	baoID         = uuid.MustParse("01990000-0000-7000-8000-000000000901") // giảng viên Phạm Quốc Bảo
	linhID        = uuid.MustParse("01990000-0000-7000-8000-000000000902") // học viên, không thuộc basic01
	khangID       = uuid.MustParse("01990000-0000-7000-8000-000000000904") // basic01, 21 ngày không hoạt động
	thaoID        = uuid.MustParse("01990000-0000-7000-8000-000000000905") // đã rời basic01
	dungID        = uuid.MustParse("01990000-0000-7000-8000-000000000906") // basic01, chưa đăng nhập (invited)
	memberKhangID = uuid.MustParse("01990000-0000-7000-8000-000000000914")
	memberThaoID  = uuid.MustParse("01990000-0000-7000-8000-000000000915")
	memberDungID  = uuid.MustParse("01990000-0000-7000-8000-000000000916")
	stageDSID     = uuid.MustParse("01990000-0000-7000-8000-000000000921")
	stageDSV1ID   = uuid.MustParse("01990000-0000-7000-8000-000000000922")
	lessonArrayID = uuid.MustParse("01990000-0000-7000-8000-000000000923") // ds-array, video
	lessonStackID = uuid.MustParse("01990000-0000-7000-8000-000000000924") // ds-stack, markdown
	lessonHashID  = uuid.MustParse("01990000-0000-7000-8000-000000000925") // ds-hash, video
	basic02ID     = uuid.MustParse("01990000-0000-7000-8000-000000000951") // active, An đã rời
	basic03ID     = uuid.MustParse("01990000-0000-7000-8000-000000000952") // draft, An đã được thêm
	basic00ID     = uuid.MustParse("01990000-0000-7000-8000-000000000953") // ended
	memberAn02ID  = uuid.MustParse("01990000-0000-7000-8000-000000000961")
	memberAn03ID  = uuid.MustParse("01990000-0000-7000-8000-000000000962")
	memberAn00ID  = uuid.MustParse("01990000-0000-7000-8000-000000000963")
	mediaIntroID  = uuid.MustParse("01990000-0000-7000-8000-000000000981")
	mediaArrayID  = uuid.MustParse("01990000-0000-7000-8000-000000000982")
	mediaHashID   = uuid.MustParse("01990000-0000-7000-8000-000000000983")
	lessonIntroID = uuid.MustParse("01990000-0000-7000-8000-000000000991") // db-intro "Giới thiệu SQL", video 18:24
)

// itEnv dựng learning trên Postgres thật cùng identity thật (đăng nhập, cookie phiên, SessionAuth) và media ký URL
// trên storage trong bộ nhớ.
type itEnv struct {
	t      *testing.T
	dbx    *sqlx.DB
	clk    *clock.Fake
	ident  *identity.Service
	svc    *Service
	engine *gin.Engine
	cookie string
}

func newIT(t *testing.T) *itEnv {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	testdb.Fixture(t, dbx, "class_basic01_active")
	testdb.Fixture(t, dbx, "stage_db_draft")

	// Đồng hồ giả lùi 2 giờ so với giờ thật để mốc "now() - interval" của fixture vẫn nằm trước mọi thời điểm test.
	clk := &clock.Fake{T: time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)}
	ident := identity.NewService(identity.ServiceDeps{
		DB: dbx, Tx: db.TxRunner{DB: dbx},
		Users: identity.PGUserRepo{}, Sessions: identity.PGSessionRepo{}, Attempts: identity.PGLoginAttemptRepo{},
		Resets: identity.PGResetTokenRepo{}, Clock: clk, Audit: audit.PG{Clock: clk},
		Cfg: config.Config{
			SessionTTL: 12 * time.Hour, TempPasswordTTL: 72 * time.Hour, ResetTokenTTL: 30 * time.Minute,
			PasswordMinLength: 8, LoginMaxFailures: 5, LoginLockWindow: 15 * time.Minute,
			PublicBaseURL: "http://localhost:5173", AppEnv: config.EnvE2E,
		},
	})
	signer := media.NewService(dbx, media.PGRepo{}, storage.NewMemStorage(), clk, ids.V7{}, media.Config{URLTTL: 2 * time.Hour})
	e := &itEnv{t: t, dbx: dbx, clk: clk, ident: ident}
	e.svc = NewService(Deps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Progress: PGProgressRepo{}, Structure: PGCourseStructureRepo{},
		MyClasses: PGMyClassesRepo{}, Members: classes.PGMembershipReader{}, Media: signer, Clock: clk,
	})
	e.seedExtra()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recover(slog.New(slog.DiscardHandler)), middleware.SecurityHeaders())
	authMW := identity.NewMiddleware(ident, false)
	authed := engine.Group("/api/v1", authMW.SessionAuth())
	NewHandler(e.svc, func(c *gin.Context) (uuid.UUID, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return uuid.Nil, false
		}
		return u.ID(), true
	}).Register(authed, identity.RequireRole(domain.RoleStudent))
	e.engine = engine
	return e
}

// seedExtra hoàn thiện khóa BASIC v1 theo seed.js: chặng Database có thêm video "Giới thiệu SQL" ở đầu, chặng
// Data structure (3 bài bắt buộc) ở vị trí 2; thêm lớp đã kết thúc, basic02 (An đã rời), basic03 nháp và học viên.
func (e *itEnv) seedExtra() {
	e.t.Helper()
	const hash = `$argon2id$v=19$m=19456,t=2,p=1$hVgN0I6aZI+AWq7rW6j/XQ$i8SWRmwDmbK6kHOJBAyuz4EOO7oK2G/nLDOcK3Q/NHE`
	now := e.clk.Now()
	users := []struct {
		id                        uuid.UUID
		email, name, role, status string
		mustChange                bool
		tempExpires, lastActive   *time.Time
	}{
		{baoID, "bao.pham@goup.vn", "Phạm Quốc Bảo", "teacher", "active", false, nil, ptr(now.Add(-72 * time.Hour))},
		{linhID, "linh.do@gmail.com", "Đỗ Ngọc Linh", "student", "active", false, nil, ptr(now.Add(-24 * time.Hour))},
		{khangID, "khang.vu@gmail.com", "Vũ Đức Khang", "student", "active", false, nil, ptr(now.Add(-21 * 24 * time.Hour))},
		{thaoID, "thao.vo@gmail.com", "Võ Phương Thảo", "student", "active", false, nil, ptr(now.Add(-15 * 24 * time.Hour))},
		{dungID, "dung.pham@gmail.com", "Phạm Minh Dũng", "student", "invited", true, ptr(now.Add(48 * time.Hour)), nil},
	}
	for _, u := range users {
		e.exec(`INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash, must_change_password,
			temp_password_expires_at, last_login_at, last_active_at)
			VALUES ($1, $2, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
			u.id, u.email, u.name, u.role, u.status, hash, u.mustChange, u.tempExpires, u.lastActive)
	}
	e.exec(`INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by, ready_at) VALUES
		($1, 'video', 'videos/01990000-0000-7000-8000-000000000981.mp4', 'gioi-thieu-sql.mp4', 'video/mp4', 73400320, 'ready', $4, now()),
		($2, 'video', 'videos/01990000-0000-7000-8000-000000000982.mp4', 'mang-va-dslk.mp4', 'video/mp4', 88080384, 'ready', $4, now()),
		($3, 'video', 'videos/01990000-0000-7000-8000-000000000983.mp4', 'hash-table.mp4', 'video/mp4', 66060288, 'ready', $4, now())`,
		mediaIntroID, mediaArrayID, mediaHashID, adminID)
	// Một câu UPDATE: uq_lessons_version_position DEFERRABLE nên kiểm ở cuối câu lệnh.
	e.exec(`UPDATE lessons SET position = position + 1 WHERE stage_version_id = $1`, stageDBV1ID)
	e.exec(`INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, required, video_media_id, duration_seconds)
		VALUES ($1, $2, 'db-intro', 1, 'Giới thiệu SQL', 'video', true, $3, 1104)`, lessonIntroID, stageDBV1ID, mediaIntroID)
	e.exec(`INSERT INTO stages (id, code, name, created_by) VALUES ($1, 'DS', 'Data structure', $2)`, stageDSID, adminID)
	e.exec(`INSERT INTO stage_versions (id, stage_id, version_no, status, title, description, published_at, created_by)
		VALUES ($1, $2, 1, 'published', 'Data structure', 'Cấu trúc dữ liệu cơ bản.', now() - interval '60 days', $3)`,
		stageDSV1ID, stageDSID, adminID)
	e.exec(`INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, required, markdown_source, markdown_html,
		video_media_id, duration_seconds) VALUES
		($1, $4, 'ds-array', 1, 'Mảng và danh sách liên kết', 'video', true, NULL, NULL, $5, 1330),
		($2, $4, 'ds-stack', 2, 'Stack và Queue', 'markdown', true, E'# Stack và Queue\n\nLIFO và FIFO.',
		 E'<h1>Stack và Queue</h1>\n<p>LIFO và FIFO.</p>\n', NULL, 900),
		($3, $4, 'ds-hash', 3, 'Hash table', 'video', true, NULL, NULL, $6, 1005)`,
		lessonArrayID, lessonStackID, lessonHashID, stageDSV1ID, mediaArrayID, mediaHashID)
	e.exec(`INSERT INTO lesson_media (lesson_id, media_id) VALUES ($1, $2), ($3, $4), ($5, $6)`,
		lessonIntroID, mediaIntroID, lessonArrayID, mediaArrayID, lessonHashID, mediaHashID)
	e.exec(`INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position) VALUES ($1, $2, $3, 2)`,
		basicV1ID, stageDSV1ID, stageDSID)
	e.exec(`INSERT INTO classes (id, code, name, course_version_id, teacher_id, status, start_date, end_date, created_by) VALUES
		($1, 'basic02', 'Lập trình cơ bản – khóa 2', $4, $5, 'active', current_date - 20, current_date + 95, $7),
		($2, 'basic03', 'Lập trình cơ bản – khóa 3', $4, $6, 'draft', current_date + 28, current_date + 140, $7),
		($3, 'basic00', 'Lập trình cơ bản – khóa 0', $4, $6, 'ended', current_date - 200, current_date - 20, $7)`,
		basic02ID, basic03ID, basic00ID, basicV1ID, baoID, huongID, adminID)
	e.exec(`INSERT INTO class_members (id, class_id, user_id, status, joined_at, dropped_at) VALUES
		($1, $2, $3, 'dropped', now() - interval '22 days', now() - interval '10 days'),
		($4, $5, $3, 'active', now() - interval '1 day', NULL),
		($6, $7, $3, 'active', now() - interval '200 days', NULL),
		($8, $9, $10, 'active', now() - interval '50 days', NULL),
		($11, $9, $12, 'dropped', now() - interval '50 days', now() - interval '15 days'),
		($13, $9, $14, 'active', now() - interval '40 days', NULL)`,
		memberAn02ID, basic02ID, anID, memberAn03ID, basic03ID, memberAn00ID, basic00ID,
		memberKhangID, basic01ID, khangID, memberThaoID, thaoID, memberDungID, dungID)
	// Lớp đã kết thúc: An đã xong video mở đầu và mới mở bài thiết kế bảng.
	e.progress(memberAn00ID, lessonIntroID, now.Add(-150*24*time.Hour), ptr(now.Add(-150*24*time.Hour+30*time.Minute)))
	e.progress(memberAn00ID, lessonTableID, now.Add(-149*24*time.Hour), nil)
}

func (e *itEnv) progress(memberID, lessonID uuid.UUID, opened time.Time, completed *time.Time) {
	e.t.Helper()
	e.exec(`INSERT INTO lesson_progress (class_member_id, lesson_id, first_opened_at, completed_at, updated_at)
		VALUES ($1, $2, $3, $4, $3)`, memberID, lessonID, opened, completed)
}

func ptr[T any](v T) *T { return &v }

func (e *itEnv) ctx() context.Context { return context.Background() }

func (e *itEnv) exec(q string, args ...any) {
	e.t.Helper()
	if _, err := e.dbx.Exec(q, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *itEnv) count(q string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.dbx.Get(&n, q, args...); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// login đăng nhập thật qua identity và giữ cookie phiên cho các request sau.
func (e *itEnv) login(email string) {
	e.t.Helper()
	res, err := e.ident.Login(e.ctx(), email, testdb.FixturePassword, "127.0.0.1", "it")
	if err != nil {
		e.t.Fatalf("đăng nhập %s: %v", email, err)
	}
	e.cookie = res.Token.Reveal()
}

type apiResp struct {
	code   int
	body   []byte
	header http.Header
}

func (r apiResp) errorCode(t *testing.T) (string, string) {
	t.Helper()
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(r.body, &env); err != nil {
		t.Fatalf("envelope lỗi: %v: %s", err, r.body)
	}
	return env.Error.Code, env.Error.Message
}

func (e *itEnv) call(method, path, body string) apiResp {
	e.t.Helper()
	req := httptest.NewRequestWithContext(e.ctx(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if e.cookie != "" {
		req.AddCookie(&http.Cookie{Name: identity.CookieName(false), Value: e.cookie})
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return apiResp{code: w.Code, body: w.Body.Bytes(), header: w.Header()}
}

// must gọi API và bắt buộc đúng mã HTTP.
func (e *itEnv) must(method, path, body string, want int) []byte {
	e.t.Helper()
	r := e.call(method, path, body)
	if r.code != want {
		e.t.Fatalf("%s %s = %d, muốn %d: %s", method, path, r.code, want, r.body)
	}
	return r.body
}

// expectError gọi API và kiểm mã HTTP, mã lỗi và (nếu có) message.
func (e *itEnv) expectError(method, path, body string, status int, code, msg string) {
	e.t.Helper()
	r := e.call(method, path, body)
	gotCode, gotMsg := r.errorCode(e.t)
	if r.code != status || gotCode != code || (msg != "" && gotMsg != msg) {
		e.t.Fatalf("%s %s = %d %s %q; muốn %d %s %q", method, path, r.code, gotCode, gotMsg, status, code, msg)
	}
}

func decode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode: %v: %s", err, raw)
	}
	return v
}

func classURL(classID uuid.UUID) string { return "/api/v1/me/classes/" + classID.String() }

func lessonURL(classID, lessonID uuid.UUID) string {
	return classURL(classID) + "/lessons/" + lessonID.String()
}

func completionURL(classID, lessonID uuid.UUID) string {
	return lessonURL(classID, lessonID) + "/completion"
}

const (
	tick   = `{"completed":true}`
	untick = `{"completed":false}`
)

// openedAt đọc first_opened_at và completed_at; ok=false khi chưa có bản ghi.
func (e *itEnv) openedAt(memberID, lessonID uuid.UUID) (opened time.Time, completed *time.Time, ok bool) {
	e.t.Helper()
	var r struct {
		Opened    time.Time  `db:"first_opened_at"`
		Completed *time.Time `db:"completed_at"`
	}
	err := e.dbx.Get(&r, `SELECT first_opened_at, completed_at FROM lesson_progress WHERE class_member_id = $1 AND lesson_id = $2`,
		memberID, lessonID)
	if err != nil {
		return time.Time{}, nil, false
	}
	return r.Opened, r.Completed, true
}

func TestOpenLessonRecordsFirstOpenOnce(t *testing.T) {
	e := newIT(t)
	e.login("an.nguyen@gmail.com")

	page := decode[LessonPageDTO](t, e.must(http.MethodGet, lessonURL(basic01ID, lessonTableID), "", http.StatusOK))
	first := e.clk.Now()
	opened, _, ok := e.openedAt(memberAnID, lessonTableID)
	if !ok || !opened.Equal(first) || page.Progress.FirstOpenedAt == nil || !page.Progress.FirstOpenedAt.Equal(first) {
		t.Fatalf("lần mở đầu: db=%v ok=%v dto=%v, muốn %v", opened, ok, page.Progress.FirstOpenedAt, first)
	}
	if page.Content.Type != "markdown" || page.Content.HTML == nil || !strings.Contains(*page.Content.HTML, "<h1>Thiết kế bảng và khóa</h1>") {
		t.Fatalf("nội dung markdown = %+v", page.Content)
	}
	if page.Prev == nil || page.Prev.LessonID != lessonIntroID.String() || page.Next == nil || page.Next.LessonID != lessonIndexID.String() {
		t.Fatalf("prev=%+v next=%+v", page.Prev, page.Next)
	}

	e.clk.Advance(time.Minute)
	page = decode[LessonPageDTO](t, e.must(http.MethodGet, lessonURL(basic01ID, lessonTableID), "", http.StatusOK))
	if again, _, _ := e.openedAt(memberAnID, lessonTableID); !again.Equal(first) || !page.Progress.FirstOpenedAt.Equal(first) {
		t.Fatalf("lần mở sau đổi first_opened_at: %v → %v", first, again)
	}

	// Video: URL ký và hạn; học liệu cuối chặng Database có next là bài đầu chặng Data structure.
	video := decode[LessonPageDTO](t, e.must(http.MethodGet, lessonURL(basic01ID, lessonIntroID), "", http.StatusOK))
	if video.Content.Type != "video" || video.Content.MediaID != mediaIntroID.String() || video.Content.URL == "" ||
		video.Content.ExpiresAt == nil || !video.Content.ExpiresAt.Equal(e.clk.Now().Add(2*time.Hour)) {
		t.Fatalf("nội dung video = %+v", video.Content)
	}
	if video.Lesson.DurationSeconds == nil || *video.Lesson.DurationSeconds != 1104 || video.Prev != nil {
		t.Fatalf("học liệu video = %+v prev=%+v", video.Lesson, video.Prev)
	}
	index := decode[LessonPageDTO](t, e.must(http.MethodGet, lessonURL(basic01ID, lessonIndexID), "", http.StatusOK))
	if index.Next == nil || index.Next.LessonID != lessonArrayID.String() {
		t.Fatalf("next của bài cuối chặng = %+v", index.Next)
	}
}

func TestCompletionTickAndUntick(t *testing.T) {
	e := newIT(t)
	e.login("an.nguyen@gmail.com")

	e.expectError(http.MethodPut, completionURL(basic01ID, lessonTableID), tick, http.StatusConflict, "CONFLICT",
		"Mở học liệu trước khi tích hoàn thành.")
	if _, _, ok := e.openedAt(memberAnID, lessonTableID); ok {
		t.Fatal("tích trước khi mở không được tạo bản ghi")
	}
	e.expectError(http.MethodPut, completionURL(basic01ID, lessonTableID), `{}`, http.StatusBadRequest, "VALIDATION_FAILED", "")
	e.expectError(http.MethodPut, completionURL(basic01ID, lessonTableID), `{"completed":true,"x":1}`, http.StatusBadRequest,
		"VALIDATION_FAILED", "")

	e.must(http.MethodGet, lessonURL(basic01ID, lessonTableID), "", http.StatusOK)
	e.clk.Advance(5 * time.Minute)
	done := decode[CompletionDTO](t, e.must(http.MethodPut, completionURL(basic01ID, lessonTableID), tick, http.StatusOK))
	if done.CompletedAt == nil || !done.CompletedAt.Equal(e.clk.Now()) || done.Percent != 20 || done.RequiredDone != 1 ||
		done.RequiredTotal != 5 {
		t.Fatalf("tích = %+v", done)
	}
	if _, c, _ := e.openedAt(memberAnID, lessonTableID); c == nil || !c.Equal(*done.CompletedAt) {
		t.Fatalf("completed_at = %v", c)
	}

	e.clk.Advance(time.Minute)
	again := decode[CompletionDTO](t, e.must(http.MethodPut, completionURL(basic01ID, lessonTableID), tick, http.StatusOK))
	if again.CompletedAt == nil || !again.CompletedAt.Equal(*done.CompletedAt) || again.Percent != 20 {
		t.Fatalf("tích lại phải giữ completedAt: %+v", again)
	}

	off := e.must(http.MethodPut, completionURL(basic01ID, lessonTableID), untick, http.StatusOK)
	if !strings.Contains(string(off), `"completedAt":null`) || decode[CompletionDTO](t, off).Percent != 0 {
		t.Fatalf("bỏ tích = %s", off)
	}
	if _, c, _ := e.openedAt(memberAnID, lessonTableID); c != nil {
		t.Fatalf("bỏ tích phải ghi NULL, được %v", c)
	}
	if off := decode[CompletionDTO](t, e.must(http.MethodPut, completionURL(basic01ID, lessonTableID), untick, http.StatusOK)); off.CompletedAt != nil {
		t.Fatalf("bỏ tích lại = %+v", off)
	}

	// Học liệu tùy chọn không đổi phần trăm.
	e.must(http.MethodGet, lessonURL(basic01ID, lessonIndexID), "", http.StatusOK)
	if opt := decode[CompletionDTO](t, e.must(http.MethodPut, completionURL(basic01ID, lessonIndexID), tick, http.StatusOK)); opt.Percent != 0 || opt.RequiredDone != 0 {
		t.Fatalf("tích học liệu tùy chọn = %+v", opt)
	}
}

func TestConcurrentCompletionIsSerialized(t *testing.T) {
	e := newIT(t)
	e.login("an.nguyen@gmail.com")
	e.must(http.MethodGet, lessonURL(basic01ID, lessonTableID), "", http.StatusOK)

	const n = 8
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := tick
			if i%2 == 1 {
				body = untick
			}
			req := httptest.NewRequestWithContext(e.ctx(), http.MethodPut, completionURL(basic01ID, lessonTableID), strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: identity.CookieName(false), Value: e.cookie})
			w := httptest.NewRecorder()
			e.engine.ServeHTTP(w, req)
			codes[i] = w.Code
		}()
	}
	wg.Wait()
	for i, c := range codes {
		if c != http.StatusOK {
			t.Fatalf("request %d = %d", i, c)
		}
	}
	if n := e.count(`SELECT count(*) FROM lesson_progress WHERE class_member_id = $1 AND lesson_id = $2`, memberAnID, lessonTableID); n != 1 {
		t.Fatalf("số bản ghi = %d", n)
	}
}

func TestLessonOutsideCourseVersionIsNotFound(t *testing.T) {
	e := newIT(t)
	e.login("an.nguyen@gmail.com")
	const msg = "Học liệu không thuộc phiên bản khóa học của lớp."
	for _, id := range []uuid.UUID{lessonV2ID, uuid.New()} {
		e.expectError(http.MethodGet, lessonURL(basic01ID, id), "", http.StatusNotFound, "NOT_FOUND", msg)
		e.expectError(http.MethodPut, completionURL(basic01ID, id), tick, http.StatusNotFound, "NOT_FOUND", msg)
	}
	if n := e.count(`SELECT count(*) FROM lesson_progress WHERE lesson_id = $1`, lessonV2ID); n != 0 {
		t.Fatalf("ghi tiến độ cho học liệu ngoài khóa: %d", n)
	}
	e.expectError(http.MethodGet, "/api/v1/me/classes/khong-phai-uuid", "", http.StatusNotFound, "NOT_FOUND", "")
}

func TestNonMemberAndDroppedAreNotFound(t *testing.T) {
	e := newIT(t)
	const msg = "Bạn không còn là thành viên đang học của lớp."
	check := func(classID uuid.UUID) {
		t.Helper()
		e.expectError(http.MethodGet, classURL(classID), "", http.StatusNotFound, "NOT_FOUND", msg)
		e.expectError(http.MethodGet, lessonURL(classID, lessonTableID), "", http.StatusNotFound, "NOT_FOUND", msg)
		e.expectError(http.MethodPut, completionURL(classID, lessonTableID), tick, http.StatusNotFound, "NOT_FOUND", msg)
	}

	e.login("an.nguyen@gmail.com")
	check(basic02ID)  // đã rời
	check(uuid.New()) // lớp không tồn tại
	list := decode[MyClassesDTO](t, e.must(http.MethodGet, "/api/v1/me/classes", "", http.StatusOK))
	for _, it := range list.Items {
		if it.ID == basic02ID.String() {
			t.Fatal("lớp đã rời không được có trong /me/classes")
		}
	}

	e.login("thao.vo@gmail.com")
	check(basic01ID)
	if got := decode[MyClassesDTO](t, e.must(http.MethodGet, "/api/v1/me/classes", "", http.StatusOK)); len(got.Items) != 0 {
		t.Fatalf("Thảo đã rời mọi lớp: %+v", got.Items)
	}

	e.login("linh.do@gmail.com")
	check(basic01ID)
}

func TestEndedClassIsReadOnly(t *testing.T) {
	e := newIT(t)
	e.login("an.nguyen@gmail.com")

	r := decode[RoadmapDTO](t, e.must(http.MethodGet, classURL(basic00ID), "", http.StatusOK))
	if !r.ReadOnly || r.ReadOnlyReason != "ended" || r.Class.Status != "ended" || r.Percent != 20 || r.NextLesson == nil ||
		r.NextLesson.LessonID != lessonTableID.String() {
		t.Fatalf("roadmap lớp ended = %+v", r)
	}

	page := decode[LessonPageDTO](t, e.must(http.MethodGet, lessonURL(basic00ID, lessonIndexID), "", http.StatusOK))
	if !page.ReadOnly || page.Progress.FirstOpenedAt != nil {
		t.Fatalf("trang học lớp ended = readOnly %v progress %+v", page.ReadOnly, page.Progress)
	}
	if _, _, ok := e.openedAt(memberAn00ID, lessonIndexID); ok {
		t.Fatal("lớp ended không được ghi first_opened_at")
	}
	video := decode[LessonPageDTO](t, e.must(http.MethodGet, lessonURL(basic00ID, lessonIntroID), "", http.StatusOK))
	if video.Content.URL == "" || video.Progress.CompletedAt == nil {
		t.Fatalf("lớp ended vẫn xem được video và tiến độ cũ: %+v", video)
	}

	const msg = "Lớp chưa bắt đầu hoặc đã kết thúc; không ghi nhận tiến độ."
	e.expectError(http.MethodPut, completionURL(basic00ID, lessonTableID), tick, http.StatusConflict, "INVALID_TRANSITION", msg)
	e.expectError(http.MethodPut, completionURL(basic00ID, lessonIntroID), untick, http.StatusConflict, "INVALID_TRANSITION", msg)
	if _, c, _ := e.openedAt(memberAn00ID, lessonIntroID); c == nil {
		t.Fatal("PUT trên lớp ended không được đổi completed_at")
	}
}

func TestDraftClassIsListedReadOnly(t *testing.T) {
	e := newIT(t)
	e.login("an.nguyen@gmail.com")

	list := decode[MyClassesDTO](t, e.must(http.MethodGet, "/api/v1/me/classes", "", http.StatusOK))
	codes := make([]string, 0, len(list.Items))
	var draft *MyClassItemDTO
	for i, it := range list.Items {
		codes = append(codes, it.Code)
		if it.ID == basic03ID.String() {
			draft = &list.Items[i]
		}
	}
	if strings.Join(codes, ",") != "basic00,basic01,basic03" {
		t.Fatalf("thứ tự lớp theo ngày tham gia = %v", codes)
	}
	if draft == nil || draft.ReadOnlyReason != "draft" || draft.Status != "draft" || draft.NextLesson != nil || draft.RequiredTotal != 5 {
		t.Fatalf("lớp draft trong danh sách = %+v", draft)
	}

	r := decode[RoadmapDTO](t, e.must(http.MethodGet, classURL(basic03ID), "", http.StatusOK))
	if !r.ReadOnly || r.ReadOnlyReason != "draft" || !r.SelfReported || r.NextLesson != nil || len(r.Stages) != 2 {
		t.Fatalf("roadmap lớp draft = %+v", r)
	}
	e.expectError(http.MethodPut, completionURL(basic03ID, lessonTableID), tick, http.StatusConflict, "INVALID_TRANSITION",
		"Lớp chưa bắt đầu hoặc đã kết thúc; không ghi nhận tiến độ.")
	e.expectError(http.MethodGet, lessonURL(basic03ID, lessonTableID), "", http.StatusConflict, "INVALID_TRANSITION",
		"Lớp chưa bắt đầu. Bạn sẽ vào học được khi lớp kích hoạt.")
	if n := e.count(`SELECT count(*) FROM lesson_progress WHERE class_member_id = $1`, memberAn03ID); n != 0 {
		t.Fatalf("lớp draft không được ghi tiến độ: %d", n)
	}
}

func TestStudentRequestTouchesLastActive(t *testing.T) {
	e := newIT(t)
	e.login("an.nguyen@gmail.com")
	e.exec(`UPDATE users SET last_active_at = now() - interval '3 days' WHERE id = $1`, anID)
	e.clk.Advance(2 * time.Minute) // middleware phiên ghi tối đa mỗi phút một lần

	r := e.call(http.MethodGet, "/api/v1/me/classes", "")
	if r.code != http.StatusOK || r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("GET /me/classes = %d, Cache-Control %q", r.code, r.header.Get("Cache-Control"))
	}
	var last time.Time
	if err := e.dbx.Get(&last, `SELECT last_active_at FROM users WHERE id = $1`, anID); err != nil {
		t.Fatal(err)
	}
	if !last.Equal(e.clk.Now()) {
		t.Fatalf("last_active_at = %v, muốn %v", last, e.clk.Now())
	}
}

func TestOnlyStudentsUseLearningRoutes(t *testing.T) {
	e := newIT(t)
	e.login("huong.le@goup.vn")
	e.expectError(http.MethodGet, "/api/v1/me/classes", "", http.StatusForbidden, "FORBIDDEN", "")
	e.expectError(http.MethodGet, classURL(basic01ID), "", http.StatusForbidden, "FORBIDDEN", "")

	e.cookie = ""
	e.expectError(http.MethodGet, "/api/v1/me/classes", "", http.StatusUnauthorized, "UNAUTHENTICATED", "")
}
