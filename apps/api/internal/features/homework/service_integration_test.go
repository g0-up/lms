//go:build integration

package homework

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/grading"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/learning"
	"lms/api/internal/features/mailer"
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

var update = flag.Bool("update", false, "ghi lại golden JSON trong testdata/")

var (
	adminID       = uuid.MustParse(testdb.AdminQuanTranID)
	huongID       = uuid.MustParse(testdb.TeacherHuongLeID)
	anID          = uuid.MustParse(testdb.StudentAnNguyenID)
	basic01ID     = uuid.MustParse(testdb.ClassBasic01ID)
	memberAnID    = uuid.MustParse(testdb.MemberAnBasic01ID)
	basicV1ID     = uuid.MustParse(testdb.CourseBasicV1ID)
	stageDBV1ID   = uuid.MustParse(testdb.StageDBV1ID)
	lessonTableID = uuid.MustParse(testdb.LessonDBTableV1ID) // markdown, không phải homework
)

// Dữ liệu thêm chèn bằng SQL trong test; id cố định (tiền tố 01990000-) để golden ổn định.
var (
	baoID         = uuid.MustParse("01990000-0000-7000-8000-000000000901") // giảng viên lớp go01
	linhID        = uuid.MustParse("01990000-0000-7000-8000-000000000902") // học viên go01, không thuộc basic01
	khangID       = uuid.MustParse("01990000-0000-7000-8000-000000000904") // basic01
	thaoID        = uuid.MustParse("01990000-0000-7000-8000-000000000905") // đã rời basic01
	memberKhangID = uuid.MustParse("01990000-0000-7000-8000-000000000914")
	memberThaoID  = uuid.MustParse("01990000-0000-7000-8000-000000000915")
	hwID          = uuid.MustParse("01990000-0000-7000-8000-000000000928") // homework của DB v1
	crQueryID     = uuid.MustParse("01990000-0000-7000-8000-000000000932") // 40 điểm
	crExplainID   = uuid.MustParse("01990000-0000-7000-8000-000000000934") // 60 điểm
	stageGoID     = uuid.MustParse("01990000-0000-7000-8000-000000000941")
	stageGoV1ID   = uuid.MustParse("01990000-0000-7000-8000-000000000942")
	unitGoV1ID    = uuid.MustParse("01990000-0000-7000-8000-000000000943")
	hwOtherID     = uuid.MustParse("01990000-0000-7000-8000-000000000944") // homework của khóa GO, ngoài basic01
	crOtherID     = uuid.MustParse("01990000-0000-7000-8000-000000000945")
	courseGoID    = uuid.MustParse("01990000-0000-7000-8000-000000000946")
	courseGoV1ID  = uuid.MustParse("01990000-0000-7000-8000-000000000947")
	go01ID        = uuid.MustParse("01990000-0000-7000-8000-000000000951") // active, giảng viên Bảo
	basic03ID     = uuid.MustParse("01990000-0000-7000-8000-000000000952") // draft, An là thành viên
	basic00ID     = uuid.MustParse("01990000-0000-7000-8000-000000000953") // ended, An là thành viên
	memberLinhID  = uuid.MustParse("01990000-0000-7000-8000-000000000961")
	memberAn03ID  = uuid.MustParse("01990000-0000-7000-8000-000000000962")
	memberAn00ID  = uuid.MustParse("01990000-0000-7000-8000-000000000963")
)

const (
	anEmail    = "an.nguyen@gmail.com"
	huongEmail = "huong.le@goup.vn"
	adminEmail = "quan.tran@goup.vn"
	baoEmail   = "bao.pham@goup.vn"
	linhEmail  = "linh.do@gmail.com"
	khangEmail = "khang.vu@gmail.com"
	thaoEmail  = "thao.vo@gmail.com"
	publicURL  = "http://localhost:5173"
)

// itEnv dựng homework trên Postgres thật cùng identity thật (đăng nhập, cookie phiên, SessionAuth), mailer outbox
// thật, audit thật và media (upload học viên, URL ký) trên storage trong bộ nhớ.
type itEnv struct {
	t       *testing.T
	dbx     *sqlx.DB
	clk     *clock.Fake
	ident   *identity.Service
	store   *storage.MemStorage
	engine  *gin.Engine
	cookies map[string]string
}

func newIT(t *testing.T) *itEnv {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	testdb.Fixture(t, dbx, "class_basic01_active")

	// Đồng hồ giả lùi 2 giờ so với giờ thật để mốc "now() - interval" của fixture vẫn nằm trước mọi thời điểm test.
	clk := &clock.Fake{T: time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)}
	ident := identity.NewService(identity.ServiceDeps{
		DB: dbx, Tx: db.TxRunner{DB: dbx},
		Users: identity.PGUserRepo{}, Sessions: identity.PGSessionRepo{}, Attempts: identity.PGLoginAttemptRepo{},
		Resets: identity.PGResetTokenRepo{}, Clock: clk, Audit: audit.PG{Clock: clk},
		Cfg: config.Config{
			SessionTTL: 12 * time.Hour, TempPasswordTTL: 72 * time.Hour, ResetTokenTTL: 30 * time.Minute,
			PasswordMinLength: 8, LoginMaxFailures: 5, LoginLockWindow: 15 * time.Minute,
			PublicBaseURL: publicURL, AppEnv: config.EnvE2E,
		},
	})
	store := storage.NewMemStorage()
	mediaSvc := media.NewService(dbx, media.PGRepo{}, store, clk, ids.V7{}, media.Config{
		Limits: media.Limits{MaxVideoBytes: 2 << 30, MaxImageBytes: 5 << 20, MaxDocumentBytes: 20 << 20},
		URLTTL: 2 * time.Hour, StudentTx: db.TxRunner{DB: dbx},
	})
	e := &itEnv{t: t, dbx: dbx, clk: clk, ident: ident, store: store, cookies: map[string]string{}}
	e.seedExtra()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recover(slog.New(slog.DiscardHandler)), middleware.SecurityHeaders())
	authed := engine.Group("/api/v1", identity.NewMiddleware(ident, false).SessionAuth())
	requireStudent := identity.RequireRole(domain.RoleStudent)
	mediaHandler := media.NewHandler(mediaSvc, func(c *gin.Context) (media.Principal, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return media.Principal{}, false
		}
		return media.Principal{ID: u.ID(), Role: u.Role()}, true
	})
	mediaHandler.Register(authed.Group("/media"), identity.RequireRole(domain.RoleAdmin))
	mediaHandler.RegisterStudent(authed.Group("/me/uploads", requireStudent))
	NewHandler(e.service(mailer.NewService(mailer.NewOutboxRepo(), nil, clk)), func(c *gin.Context) (uuid.UUID, domain.Role, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return uuid.Nil, "", false
		}
		return u.ID(), u.Role(), true
	}).Register(authed, Guards{Staff: identity.RequireRole(domain.RoleAdmin, domain.RoleTeacher), Student: requireStudent})
	e.engine = engine
	return e
}

// service dựng Service như production với mailer cho trước.
func (e *itEnv) service(mail mailer.Enqueuer) *Service {
	return NewService(Deps{
		DB: e.dbx, Tx: db.TxRunner{DB: e.dbx}, Repo: PGRepo{}, Members: classes.PGMembershipReader{},
		Lessons: learning.PGCourseStructureRepo{}, Completion: learning.PGProgressRepo{}, Mail: mail,
		Audit: audit.PG{Clock: e.clk}, Clock: e.clk, IDs: ids.V7{},
		Next:          grading.NewService(grading.Deps{Tx: db.TxRunner{DB: e.dbx}, Repo: grading.PGRepo{}, Clock: e.clk}),
		PublicBaseURL: publicURL,
	})
}

// seedExtra thêm homework "Bài tập: truy vấn SELECT" (40 + 60 điểm, ngưỡng 70%) vào chặng Database của BASIC v1;
// khóa GO riêng có homework khác và lớp go01 của giảng viên Bảo; basic03 nháp và basic00 đã kết thúc có An; basic01
// thêm Khang (đang học) và Thảo (đã rời).
func (e *itEnv) seedExtra() {
	e.t.Helper()
	const hash = `$argon2id$v=19$m=19456,t=2,p=1$hVgN0I6aZI+AWq7rW6j/XQ$i8SWRmwDmbK6kHOJBAyuz4EOO7oK2G/nLDOcK3Q/NHE`
	users := []struct {
		id                uuid.UUID
		email, name, role string
	}{
		{baoID, baoEmail, "Phạm Quốc Bảo", "teacher"},
		{linhID, linhEmail, "Đỗ Ngọc Linh", "student"},
		{khangID, khangEmail, "Vũ Đức Khang", "student"},
		{thaoID, thaoEmail, "Võ Phương Thảo", "student"},
	}
	for _, u := range users {
		e.exec(`INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash)
			VALUES ($1, $2, $2, $3, $4, 'active', $5)`, u.id, u.email, u.name, u.role, hash)
	}
	e.exec(`INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type, required)
		VALUES ($1, $2, $3, 'db-hw', 3, 'Bài tập: truy vấn SELECT', 'homework', true)`, hwID, stageDBV1ID, testdb.UnitDBV1ID)
	e.exec(`INSERT INTO homework_settings (lesson_id, brief_source, brief_html, pass_percent)
		VALUES ($1, '# Truy vấn SELECT', '<h1>Truy vấn SELECT</h1>', 70)`, hwID)
	e.exec(`INSERT INTO rubric_criteria (id, lesson_id, description, max_points, position) VALUES
		($1, $3, 'Giải thích cách làm', 60, 2), ($2, $3, 'Truy vấn đúng kết quả', 40, 1)`, crExplainID, crQueryID, hwID)

	e.exec(`INSERT INTO stages (id, code, name, created_by) VALUES ($1, 'GO', 'Go cơ bản', $2)`, stageGoID, adminID)
	e.exec(`INSERT INTO stage_versions (id, stage_id, version_no, status, title, description, published_at, created_by)
		VALUES ($1, $2, 1, 'published', 'Go cơ bản', 'Nhập môn Go.', now() - interval '30 days', $3)`, stageGoV1ID, stageGoID, adminID)
	e.exec(`INSERT INTO stage_units (id, stage_version_id, unit_key, title, position)
		VALUES ($1, $2, 'noi-dung-chang', 'Nội dung chặng', 1)`, unitGoV1ID, stageGoV1ID)
	e.exec(`INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type, required)
		VALUES ($1, $2, $3, 'go-hw', 1, 'Bài tập: HTTP server', 'homework', true)`, hwOtherID, stageGoV1ID, unitGoV1ID)
	e.exec(`INSERT INTO homework_settings (lesson_id, brief_source, brief_html, pass_percent)
		VALUES ($1, '# HTTP server', '<h1>HTTP server</h1>', 60)`, hwOtherID)
	e.exec(`INSERT INTO rubric_criteria (id, lesson_id, description, max_points, position) VALUES ($1, $2, 'Chạy được', 10, 1)`,
		crOtherID, hwOtherID)
	e.exec(`INSERT INTO courses (id, code, name, created_by) VALUES ($1, 'GO', 'Go cơ bản', $2)`, courseGoID, adminID)
	e.exec(`INSERT INTO course_versions (id, course_id, version_no, status, title, description, published_at, created_by)
		VALUES ($1, $2, 1, 'published', 'Go cơ bản', 'Lộ trình Go.', now() - interval '29 days', $3)`, courseGoV1ID, courseGoID, adminID)
	e.exec(`INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position) VALUES ($1, $2, $3, 1)`,
		courseGoV1ID, stageGoV1ID, stageGoID)

	e.exec(`INSERT INTO classes (id, code, name, course_version_id, teacher_id, status, start_date, end_date, created_by) VALUES
		($1, 'go01', 'Go cơ bản – khóa 1', $4, $5, 'active', current_date - 20, current_date + 95, $7),
		($2, 'basic03', 'Lập trình cơ bản – khóa 3', $6, $8, 'draft', current_date + 28, current_date + 140, $7),
		($3, 'basic00', 'Lập trình cơ bản – khóa 0', $6, $8, 'ended', current_date - 200, current_date - 20, $7)`,
		go01ID, basic03ID, basic00ID, courseGoV1ID, baoID, basicV1ID, adminID, huongID)
	e.exec(`INSERT INTO class_members (id, class_id, user_id, status, joined_at, dropped_at) VALUES
		($1, $2, $3, 'active', now() - interval '20 days', NULL),
		($4, $5, $6, 'active', now() - interval '1 day', NULL),
		($7, $8, $6, 'active', now() - interval '200 days', NULL),
		($9, $10, $11, 'active', now() - interval '50 days', NULL),
		($12, $10, $13, 'dropped', now() - interval '50 days', now() - interval '15 days')`,
		memberLinhID, go01ID, linhID, memberAn03ID, basic03ID, anID, memberAn00ID, basic00ID,
		memberKhangID, basic01ID, khangID, memberThaoID, thaoID)
}

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

// as đăng nhập thật qua identity (một lần cho mỗi email) và trả cookie phiên.
func (e *itEnv) as(email string) string {
	e.t.Helper()
	if c, ok := e.cookies[email]; ok {
		return c
	}
	res, err := e.ident.Login(e.ctx(), email, testdb.FixturePassword, "127.0.0.1", "it")
	if err != nil {
		e.t.Fatalf("đăng nhập %s: %v", email, err)
	}
	e.cookies[email] = res.Token.Reveal()
	return e.cookies[email]
}

// advance đẩy đồng hồ và bỏ phiên đã lưu; lần gọi sau đăng nhập lại nên vượt thời hạn phiên 12 giờ vẫn hợp lệ.
func (e *itEnv) advance(d time.Duration) {
	e.clk.Advance(d)
	clear(e.cookies)
}

type apiResp struct {
	code int
	body []byte
}

type errorEnvelope struct {
	Error struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Details map[string]string `json:"details"`
	} `json:"error"`
}

// serve gọi engine; an toàn khi chạy song song (không đụng trạng thái của itEnv).
func (e *itEnv) serve(cookie, method, path, body string) apiResp {
	req := httptest.NewRequestWithContext(e.ctx(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: identity.CookieName(false), Value: cookie})
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return apiResp{code: w.Code, body: w.Body.Bytes()}
}

// must gọi API với phiên của email và bắt buộc đúng mã HTTP.
func (e *itEnv) must(email, method, path, body string, want int) []byte {
	e.t.Helper()
	r := e.serve(e.as(email), method, path, body)
	if r.code != want {
		e.t.Fatalf("%s %s %s = %d, muốn %d: %s", email, method, path, r.code, want, r.body)
	}
	return r.body
}

// expectError gọi API và kiểm mã HTTP, mã lỗi và (nếu có) message; trả details theo trường.
func (e *itEnv) expectError(email, method, path, body string, status int, code, msg string) map[string]string {
	e.t.Helper()
	r := e.serve(e.as(email), method, path, body)
	var env errorEnvelope
	if err := json.Unmarshal(r.body, &env); err != nil {
		e.t.Fatalf("%s %s: envelope lỗi: %v: %s", method, path, err, r.body)
	}
	if r.code != status || env.Error.Code != code || (msg != "" && env.Error.Message != msg) {
		e.t.Fatalf("%s %s %s = %d %s %q; muốn %d %s %q", email, method, path, r.code, env.Error.Code, env.Error.Message,
			status, code, msg)
	}
	return env.Error.Details
}

func decode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode: %v: %s", err, raw)
	}
	return v
}

func mySubmissionsURL(classID, lessonID uuid.UUID) string {
	return "/api/v1/me/classes/" + classID.String() + "/lessons/" + lessonID.String() + "/submissions"
}

func deadlineURL(classID, lessonID uuid.UUID) string {
	return "/api/v1/classes/" + classID.String() + "/deadlines/" + lessonID.String()
}

func homeworkURL(classID uuid.UUID) string {
	return "/api/v1/classes/" + classID.String() + "/homework"
}

func submissionURL(id string) string { return "/api/v1/submissions/" + id }

func gradesURL(id string) string { return submissionURL(id) + "/grades" }

func repoBody(repo string) string { return fmt.Sprintf(`{"repoUrl":%q}`, repo) }

// gradeBody là form chấm homework db-hw: điểm "Truy vấn đúng kết quả" (40) và "Giải thích cách làm" (60).
func gradeBody(query, explain int, comment string) string {
	return fmt.Sprintf(`{"scores":[{"criterionId":%q,"points":%d},{"criterionId":%q,"points":%d,"comment":"Nên nêu lý do chọn JOIN."}],"comment":%q}`,
		crQueryID, query, crExplainID, explain, comment)
}

func dueBody(t time.Time) string { return fmt.Sprintf(`{"dueAt":%q}`, t.Format(time.RFC3339)) }

// submit nộp homework db-hw của basic01 và trả bản nộp mới nhất.
func (e *itEnv) submit(email, body string) SubmissionDTO {
	e.t.Helper()
	v := decode[MySubmissionsDTO](e.t, e.must(email, http.MethodPost, mySubmissionsURL(basic01ID, hwID), body, http.StatusCreated))
	return v.Submissions[0]
}

func (e *itEnv) grade(email, sid, body string) GradeOutcomeDTO {
	e.t.Helper()
	return decode[GradeOutcomeDTO](e.t, e.must(email, http.MethodPost, gradesURL(sid), body, http.StatusCreated))
}

// upload tải một file bài tập qua /me/uploads tới ready và trả media id.
func (e *itEnv) upload(email, name string) string {
	e.t.Helper()
	ticket := decode[media.UploadTicketDTO](e.t, e.must(email, http.MethodPost, "/api/v1/me/uploads",
		fmt.Sprintf(`{"kind":"document","fileName":%q,"contentType":"application/zip","sizeBytes":5}`, name), http.StatusCreated))
	var key string
	if err := e.dbx.Get(&key, `SELECT storage_key FROM media_files WHERE id = $1`, ticket.MediaID); err != nil {
		e.t.Fatal(err)
	}
	if err := e.store.Put(e.ctx(), key, strings.NewReader("12345"), 5, ticket.Headers["Content-Type"]); err != nil {
		e.t.Fatal(err)
	}
	e.must(email, http.MethodPost, "/api/v1/me/uploads/"+ticket.MediaID+"/complete", "", http.StatusOK)
	return ticket.MediaID
}

func (e *itEnv) statuses(memberID uuid.UUID) []string {
	e.t.Helper()
	var out []string
	if err := e.dbx.Select(&out, `SELECT status FROM submissions WHERE class_member_id = $1 AND lesson_id = $2 ORDER BY attempt_no`,
		memberID, hwID); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// completedAt đọc completed_at của homework; ok=false khi chưa có bản ghi tiến độ.
func (e *itEnv) completedAt(memberID uuid.UUID) (completed *time.Time, ok bool) {
	e.t.Helper()
	err := e.dbx.Get(&completed, `SELECT completed_at FROM lesson_progress WHERE class_member_id = $1 AND lesson_id = $2`, memberID, hwID)
	if err != nil {
		return nil, false
	}
	return completed, true
}

func (e *itEnv) auditCount(action string) int {
	e.t.Helper()
	return e.count(`SELECT count(*) FROM audit_logs WHERE action = $1`, action)
}

func TestSubmitRepoAndFiles(t *testing.T) {
	e := newIT(t)

	first := e.submit(anEmail, repoBody(" https://github.com/an/sql-select "))
	if first.No != 1 || first.Status != "pending" || first.RepoURL == nil || *first.RepoURL != "https://github.com/an/sql-select" ||
		first.CommitRef != nil || len(first.Files) != 0 || first.IsLate || first.Grade != nil || !first.SubmittedAt.Equal(e.clk.Now()) {
		t.Fatalf("lần nộp 1 = %+v", first)
	}

	a, b := e.upload(anEmail, "truy-van.sql"), e.upload(anEmail, "giai-thich.md")
	e.advance(time.Hour)
	raw := e.must(anEmail, http.MethodPost, mySubmissionsURL(basic01ID, hwID),
		fmt.Sprintf(`{"repoUrl":"https://gitlab.com/an/sql","commitRef":"v1.0","mediaIds":[%q,%q]}`, b, a), http.StatusCreated)
	v := decode[MySubmissionsDTO](t, raw)
	if v.Status != "pending" || !v.CanSubmit || v.LockedReason != nil || v.MaxFiles != 5 || len(v.Submissions) != 2 {
		t.Fatalf("sau lần nộp 2 = %s", raw)
	}
	second := v.Submissions[0]
	if second.No != 2 || second.CommitRef == nil || *second.CommitRef != "v1.0" || len(second.Files) != 2 ||
		second.Files[0] != (FileDTO{MediaID: b, FileName: "giai-thich.md", SizeBytes: 5}) || second.Files[1].MediaID != a {
		t.Fatalf("lần nộp 2 = %+v", second)
	}
	if v.Submissions[1].ID != first.ID || v.Submissions[1].Status != "superseded" {
		t.Fatalf("lần nộp 1 sau khi nộp lại = %+v", v.Submissions[1])
	}

	// Chỉ file: không repo, không commit.
	c := e.upload(anEmail, "bai-3.zip")
	only := e.submit(anEmail, fmt.Sprintf(`{"mediaIds":[%q]}`, c))
	if only.No != 3 || only.RepoURL != nil || len(only.Files) != 1 {
		t.Fatalf("lần nộp chỉ file = %+v", only)
	}
	if got := e.statuses(memberAnID); strings.Join(got, ",") != "superseded,superseded,pending" {
		t.Fatalf("trạng thái = %v", got)
	}

	// GET trả cùng dạng.
	got := decode[MySubmissionsDTO](t, e.must(anEmail, http.MethodGet, mySubmissionsURL(basic01ID, hwID), "", http.StatusOK))
	if got.Status != "pending" || len(got.Submissions) != 3 || got.Submissions[0].ID != only.ID {
		t.Fatalf("GET = %+v", got)
	}
}

func TestSubmitValidation(t *testing.T) {
	e := newIT(t)
	path := mySubmissionsURL(basic01ID, hwID)
	six := make([]string, 6)
	for i := range six {
		six[i] = fmt.Sprintf("%q", uuid.New())
	}
	cases := []struct {
		name, body string
		fields     map[string]string
	}{
		{"rỗng", `{}`, map[string]string{"repoUrl": ErrNoContent.Msg}},
		{"repo http", repoBody("http://github.com/an/sql"), map[string]string{"repoUrl": ErrRepoURL.Msg}},
		{"host khác", repoBody("https://example.com/an/sql"), map[string]string{"repoUrl": ErrRepoURL.Msg}},
		{"commit không repo", fmt.Sprintf(`{"commitRef":"abc","mediaIds":[%q]}`, uuid.New()),
			map[string]string{"commitRef": ErrCommitNeedsRepo.Msg}},
		{"commit ký tự lạ", `{"repoUrl":"https://github.com/an/sql","commitRef":"a b"}`, map[string]string{"commitRef": ErrCommitRef.Msg}},
		{"6 file", `{"mediaIds":[` + strings.Join(six, ",") + `]}`, map[string]string{"mediaIds": ErrTooManyFiles.Msg}},
		{"file trùng", fmt.Sprintf(`{"mediaIds":[%s,%s]}`, six[0], six[0]), map[string]string{"mediaIds": ErrDuplicateFile.Msg}},
		{"file không tồn tại", `{"mediaIds":[` + six[0] + `]}`, map[string]string{"mediaIds": ErrFileUnavailable.Msg}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			details := e.expectError(anEmail, http.MethodPost, path, tc.body, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "")
			if fmt.Sprint(details) != fmt.Sprint(tc.fields) {
				t.Fatalf("details = %v, muốn %v", details, tc.fields)
			}
		})
	}
	e.expectError(anEmail, http.MethodPost, path, `{"repoUrl":"https://github.com/an/sql","x":1}`, http.StatusBadRequest,
		"VALIDATION_FAILED", "")
	e.expectError(anEmail, http.MethodPost, path, `{"mediaIds":["khong-phai-uuid"]}`, http.StatusBadRequest, "VALIDATION_FAILED", "")

	// File của học viên khác, file chưa tải xong và file đã nộp đều bị từ chối như file không tồn tại.
	khangFile := e.upload(khangEmail, "cua-khang.zip")
	ticket := decode[media.UploadTicketDTO](t, e.must(anEmail, http.MethodPost, "/api/v1/me/uploads",
		`{"kind":"document","fileName":"dang-tai.zip","contentType":"application/zip","sizeBytes":5}`, http.StatusCreated))
	for _, id := range []string{khangFile, ticket.MediaID} {
		details := e.expectError(anEmail, http.MethodPost, path, fmt.Sprintf(`{"mediaIds":[%q]}`, id), http.StatusUnprocessableEntity,
			"VALIDATION_FAILED", ErrFileUnavailable.Msg)
		if details["mediaIds"] != ErrFileUnavailable.Msg {
			t.Fatalf("details = %v", details)
		}
	}
	mine := e.upload(anEmail, "bai.zip")
	e.submit(anEmail, fmt.Sprintf(`{"mediaIds":[%q]}`, mine))
	e.expectError(anEmail, http.MethodPost, path, fmt.Sprintf(`{"mediaIds":[%q]}`, mine), http.StatusUnprocessableEntity,
		"VALIDATION_FAILED", ErrFileUnavailable.Msg)

	// Lần nộp bị từ chối không ghi gì và không làm bản đang chờ chấm bị thay.
	if got := e.statuses(memberAnID); strings.Join(got, ",") != "pending" {
		t.Fatalf("trạng thái = %v", got)
	}
}

func TestPassedLocksResubmitAndOnlyLatestIsGraded(t *testing.T) {
	e := newIT(t)
	s1 := e.submit(anEmail, repoBody("https://github.com/an/sql"))
	e.advance(time.Hour)
	s2 := e.submit(anEmail, repoBody("https://github.com/an/sql-2"))

	// Bản bị thay không chấm được.
	e.expectError(huongEmail, http.MethodPost, gradesURL(s1.ID), gradeBody(40, 60, "Tốt"), http.StatusConflict,
		"INVALID_TRANSITION", ErrNotLatest.Msg)

	e.advance(time.Hour)
	if o := e.grade(huongEmail, s2.ID, gradeBody(30, 20, "Cần giải thích kỹ hơn")); o.SubmissionStatus != "rework" || o.Updated {
		t.Fatalf("chấm lần 2 = %+v", o)
	}
	// Cần làm lại: nộp tiếp được, bản rework giữ nguyên trạng thái.
	e.advance(time.Hour)
	s3 := e.submit(anEmail, repoBody("https://github.com/an/sql-3"))
	if got := e.statuses(memberAnID); strings.Join(got, ",") != "superseded,rework,pending" {
		t.Fatalf("trạng thái = %v", got)
	}
	// Bản rework cũ không còn là mới nhất nên không sửa điểm được.
	e.expectError(huongEmail, http.MethodPost, gradesURL(s2.ID), gradeBody(40, 60, "Tốt"), http.StatusConflict,
		"INVALID_TRANSITION", ErrNotLatest.Msg)

	e.advance(time.Hour)
	e.grade(huongEmail, s3.ID, gradeBody(40, 50, "Đạt yêu cầu"))
	e.expectError(anEmail, http.MethodPost, mySubmissionsURL(basic01ID, hwID), repoBody("https://github.com/an/sql-4"),
		http.StatusConflict, "INVALID_TRANSITION", ErrAlreadyPassed.Msg)
	v := decode[MySubmissionsDTO](t, e.must(anEmail, http.MethodGet, mySubmissionsURL(basic01ID, hwID), "", http.StatusOK))
	if v.Status != "passed" || v.CanSubmit || v.LockedReason == nil || *v.LockedReason != ErrAlreadyPassed.Msg {
		t.Fatalf("sau khi Đạt = %+v", v)
	}
	if v.Submissions[0].Grade == nil || v.Submissions[0].Grade.TotalPoints != 90 || v.Submissions[1].Grade == nil ||
		v.Submissions[1].Grade.Result != "rework" || v.Submissions[2].Grade != nil {
		t.Fatalf("lịch sử = %+v", v.Submissions)
	}

	// Trang chấm của bản cũ: isLatest false, trỏ tới bản mới nhất.
	d := decode[SubmissionDetailDTO](t, e.must(huongEmail, http.MethodGet, submissionURL(s1.ID), "", http.StatusOK))
	if d.IsLatest || d.LatestSubmissionID != s3.ID || d.Submission.ID != s1.ID || len(d.History) != 3 {
		t.Fatalf("trang chấm bản cũ = %+v", d)
	}
}

func TestGradeThresholdCompletionAndHistory(t *testing.T) {
	e := newIT(t)
	s := e.submit(anEmail, repoBody("https://github.com/an/sql"))

	// 69/100 dưới ngưỡng 70%: cần làm lại, không có mốc hoàn thành.
	e.advance(time.Hour)
	o := e.grade(huongEmail, s.ID, gradeBody(40, 29, "Thiếu giải thích"))
	if o.SubmissionStatus != "rework" || o.Updated || o.Grade.TotalPoints != 69 || o.Grade.MaxPoints != 100 ||
		o.Grade.Result != "rework" || o.Grade.GradedBy != "Lê Thu Hương" || !o.Grade.GradedAt.Equal(e.clk.Now()) || len(o.Grade.Scores) != 2 ||
		o.Grade.Scores[0].CriterionID != crQueryID.String() || o.Grade.Scores[1].Comment != "Nên nêu lý do chọn JOIN." {
		t.Fatalf("chấm 69 = %+v", o)
	}
	if c, _ := e.completedAt(memberAnID); c != nil {
		t.Fatalf("rework không được có completed_at: %v", c)
	}

	// Sửa điểm lên đúng ngưỡng 70%: Đạt, ghi completed_at trong cùng transaction.
	e.advance(time.Hour)
	passedAt := e.clk.Now()
	o = e.grade(huongEmail, s.ID, gradeBody(40, 30, "Đạt ngưỡng"))
	if o.SubmissionStatus != "passed" || !o.Updated || o.Grade.TotalPoints != 70 {
		t.Fatalf("chấm 70 = %+v", o)
	}
	if c, ok := e.completedAt(memberAnID); !ok || c == nil || !c.Equal(passedAt) {
		t.Fatalf("completed_at = %v ok=%v, muốn %v", c, ok, passedAt)
	}
	// Sửa điểm vẫn Đạt giữ mốc hoàn thành đầu tiên.
	e.advance(time.Hour)
	e.grade(huongEmail, s.ID, gradeBody(40, 60, "Rất tốt"))
	if c, _ := e.completedAt(memberAnID); c == nil || !c.Equal(passedAt) {
		t.Fatalf("completed_at sau khi sửa điểm vẫn Đạt = %v", c)
	}
	// Sửa xuống Cần làm lại: xóa mốc hoàn thành.
	e.advance(time.Hour)
	o = e.grade(adminEmail, s.ID, gradeBody(10, 10, "Chấm lại: chưa đúng đề"))
	if o.SubmissionStatus != "rework" || !o.Updated || o.Grade.GradedBy != "Trần Minh Quân" {
		t.Fatalf("admin chấm lại = %+v", o)
	}
	if c, ok := e.completedAt(memberAnID); !ok || c != nil {
		t.Fatalf("completed_at sau khi về rework = %v ok=%v", c, ok)
	}

	// Lịch sử giữ đủ 4 lần chấm, mới nhất trước, mỗi lần đủ điểm tiêu chí.
	d := decode[SubmissionDetailDTO](t, e.must(huongEmail, http.MethodGet, submissionURL(s.ID), "", http.StatusOK))
	g := d.Submission.Grades
	if len(g) != 4 || g[0].TotalPoints != 20 || g[1].TotalPoints != 100 || g[2].TotalPoints != 70 || g[3].TotalPoints != 69 ||
		d.Submission.Grade == nil || d.Submission.Grade.ID != g[0].ID || d.Submission.Status != "rework" {
		t.Fatalf("lịch sử chấm = %+v", d.Submission)
	}
	for _, x := range g {
		if len(x.Scores) != 2 {
			t.Fatalf("lần chấm %s thiếu điểm tiêu chí: %+v", x.ID, x.Scores)
		}
	}
	if n := e.count(`SELECT count(*) FROM grade_criterion_scores`); n != 8 {
		t.Fatalf("số điểm tiêu chí = %d", n)
	}
}

// Lần chấm đầu trả mục cũ nhất còn lại trong hàng chờ của người chấm (giảng viên chỉ lớp mình, admin mọi lớp); sửa
// điểm và hết hàng chờ trả null.
func TestGradeReturnsNextQueueItem(t *testing.T) {
	e := newIT(t)
	var linh SubmissionDTO
	{
		v := decode[MySubmissionsDTO](t, e.must(linhEmail, http.MethodPost, mySubmissionsURL(go01ID, hwOtherID),
			repoBody("https://github.com/linh/http"), http.StatusCreated))
		linh = v.Submissions[0]
	}
	e.advance(time.Hour)
	an := e.submit(anEmail, repoBody("https://github.com/an/sql"))
	e.advance(time.Hour)
	khang := e.submit(khangEmail, repoBody("https://github.com/khang/sql"))
	e.advance(time.Hour)

	// Bài của Linh (lớp go01) cũ nhất nhưng ngoài lớp của Hương.
	o := e.grade(huongEmail, an.ID, gradeBody(40, 60, "Tốt"))
	n := o.NextQueueItem
	if o.Updated || n == nil || n.Kind != "homework" || n.ItemID != khang.ID || n.No != 1 || n.Student.ID != khangID.String() ||
		n.Student.Name != "Vũ Đức Khang" || n.Class.ID != basic01ID.String() || n.Lesson.ID != hwID.String() ||
		n.Lesson.Title != "Bài tập: truy vấn SELECT" || n.Stage.Name == "" || n.ScorePercent != nil ||
		n.WaitedHours != 1 || n.Overdue || !n.SubmittedAt.Equal(e.clk.Now().Add(-time.Hour)) {
		t.Fatalf("nextQueueItem sau lần chấm đầu = %+v", n)
	}
	if o = e.grade(huongEmail, an.ID, gradeBody(40, 50, "Sửa điểm")); !o.Updated || o.NextQueueItem != nil {
		t.Fatalf("sửa điểm phải trả nextQueueItem null: %+v", o)
	}

	// Admin thấy mọi lớp: chấm Khang thì mục kế tiếp là bài của Linh.
	if o = e.grade(adminEmail, khang.ID, gradeBody(40, 60, "Tốt")); o.NextQueueItem == nil || o.NextQueueItem.ItemID != linh.ID ||
		o.NextQueueItem.Class.ID != go01ID.String() || o.NextQueueItem.WaitedHours != 3 {
		t.Fatalf("nextQueueItem của admin = %+v", o.NextQueueItem)
	}

	// Hết hàng chờ.
	body := fmt.Sprintf(`{"scores":[{"criterionId":%q,"points":10}],"comment":"Chạy tốt"}`, crOtherID)
	if o = e.grade(baoEmail, linh.ID, body); o.Updated || o.NextQueueItem != nil {
		t.Fatalf("hết hàng chờ phải trả nextQueueItem null: %+v", o)
	}
}

func TestGradeValidation(t *testing.T) {
	e := newIT(t)
	s := e.submit(anEmail, repoBody("https://github.com/an/sql"))
	path := gradesURL(s.ID)

	details := e.expectError(huongEmail, http.MethodPost, path, gradeBody(41, 60, " "), http.StatusUnprocessableEntity, "VALIDATION_FAILED", "")
	if details["scores."+crQueryID.String()] != `Điểm tiêu chí "Truy vấn đúng kết quả" phải từ 0 đến 40.` ||
		details["comment"] != ErrCommentRequired.Msg || len(details) != 2 {
		t.Fatalf("details = %v", details)
	}
	details = e.expectError(huongEmail, http.MethodPost, path,
		fmt.Sprintf(`{"scores":[{"criterionId":%q,"points":10},{"criterionId":%q,"points":1}],"comment":"x"}`, crQueryID, crOtherID),
		http.StatusUnprocessableEntity, "VALIDATION_FAILED", "")
	if details["scores"] != ErrUnknownCriterion.Msg || details["scores."+crExplainID.String()] != `Nhập điểm cho tiêu chí "Giải thích cách làm".` {
		t.Fatalf("details = %v", details)
	}
	e.expectError(huongEmail, http.MethodPost, path, `{"scores":[],"comment":"x","extra":1}`, http.StatusBadRequest, "VALIDATION_FAILED", "")

	if n := e.count(`SELECT count(*) FROM submission_grades`); n != 0 {
		t.Fatalf("chấm lỗi vẫn ghi %d lần chấm", n)
	}
	if got := e.statuses(memberAnID); strings.Join(got, ",") != "pending" {
		t.Fatalf("trạng thái = %v", got)
	}
}

// failingMail giả lập outbox lỗi để chứng minh lần chấm rollback trọn vẹn.
type failingMail struct{}

func (failingMail) Enqueue(context.Context, db.Executor, mailer.Message) (uuid.UUID, error) {
	return uuid.Nil, errors.New("outbox lỗi")
}

func TestGradeIsAtomic(t *testing.T) {
	e := newIT(t)
	s := e.submit(anEmail, repoBody("https://github.com/an/sql"))
	svc := e.service(failingMail{})
	pts := func(n int) *int { return &n }
	_, err := svc.Grade(e.ctx(), Actor{ID: huongID, Role: domain.RoleTeacher}, uuid.MustParse(s.ID), GradeInput{
		Scores:  []ScoreInput{{CriterionID: crQueryID, Points: pts(40)}, {CriterionID: crExplainID, Points: pts(60)}},
		Comment: "Tốt",
	})
	if err == nil || !strings.Contains(err.Error(), "outbox lỗi") {
		t.Fatalf("err = %v", err)
	}
	// Không còn dấu vết: lần chấm, trạng thái, mốc hoàn thành và audit đều rollback.
	if n := e.count(`SELECT count(*) FROM submission_grades`); n != 0 {
		t.Fatalf("lần chấm = %d", n)
	}
	if got := e.statuses(memberAnID); strings.Join(got, ",") != "pending" {
		t.Fatalf("trạng thái = %v", got)
	}
	if _, ok := e.completedAt(memberAnID); ok {
		t.Fatal("mốc hoàn thành bị ghi dù transaction lỗi")
	}
	if n := e.auditCount(audit.ActionHomeworkGraded); n != 0 {
		t.Fatalf("audit = %d", n)
	}
}

func TestGradeEnqueuesEmailAndRecordsAudit(t *testing.T) {
	e := newIT(t)
	s := e.submit(anEmail, repoBody("https://github.com/an/sql"))
	e.grade(huongEmail, s.ID, gradeBody(40, 20, "Thiếu giải thích"))
	e.advance(time.Hour)
	e.grade(huongEmail, s.ID, gradeBody(40, 40, "Đã đủ"))

	type outbox struct {
		To      string `db:"to_email"`
		Payload []byte `db:"payload"`
	}
	var rows []outbox
	if err := e.dbx.Select(&rows, `SELECT to_email, payload FROM email_outbox WHERE template = 'homework_graded' ORDER BY created_at, id`); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("số email = %d", len(rows))
	}
	wantURL := publicURL + "/learn/classes/" + basic01ID.String() + "/lessons/" + hwID.String()
	for i, r := range rows {
		p := decode[map[string]any](t, r.Payload)
		if r.To != anEmail || p["ResultURL"] != wantURL || p["Name"] != "Nguyễn Hoàng An" || p["ClassCode"] != "basic01" ||
			p["LessonTitle"] != "Bài tập: truy vấn SELECT" || p["MaxPoints"] != float64(100) || p["Updated"] != (i == 1) {
			t.Fatalf("email %d = %s", i, r.Payload)
		}
		for _, k := range []string{"Link", "Token", "TempPassword"} {
			if _, ok := p[k]; ok {
				t.Fatalf("payload email chứa khóa %s: %s", k, r.Payload)
			}
		}
	}
	first, second := decode[map[string]any](t, rows[0].Payload), decode[map[string]any](t, rows[1].Payload)
	if first["ResultLabel"] != "Cần làm lại" || first["TotalPoints"] != float64(60) || second["ResultLabel"] != "Đạt" ||
		second["Comment"] != "Đã đủ" {
		t.Fatalf("email = %v / %v", first, second)
	}

	type auditRow struct {
		Action    string  `db:"action"`
		Actor     string  `db:"actor_id"`
		Target    string  `db:"target_id"`
		Before    []byte  `db:"before"`
		After     []byte  `db:"after"`
		RequestID *string `db:"request_id"`
	}
	var logs []auditRow
	if err := e.dbx.Select(&logs, `SELECT action, actor_id, target_id, before, after, request_id FROM audit_logs
		WHERE action LIKE 'homework.%' AND target_type = 'class' ORDER BY at, id`); err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[0].Action != audit.ActionHomeworkGraded || logs[1].Action != audit.ActionHomeworkGradeUpdated {
		t.Fatalf("audit = %+v", logs)
	}
	for _, l := range logs {
		if l.Actor != huongID.String() || l.Target != basic01ID.String() || l.RequestID == nil || *l.RequestID == "" {
			t.Fatalf("audit = %+v", l)
		}
	}
	if logs[0].Before != nil {
		t.Fatalf("lần chấm đầu không có before: %s", logs[0].Before)
	}
	before, after := decode[map[string]any](t, logs[1].Before), decode[map[string]any](t, logs[1].After)
	if before["result"] != "rework" || before["totalPoints"] != float64(60) || after["result"] != "passed" ||
		after["totalPoints"] != float64(80) || after["submissionId"] != s.ID || after["studentName"] != "Nguyễn Hoàng An" {
		t.Fatalf("audit sửa điểm before=%v after=%v", before, after)
	}
}

func TestDeadlineSetClearAndLateFlag(t *testing.T) {
	e := newIT(t)
	path := deadlineURL(basic01ID, hwID)
	saigon := time.FixedZone("ICT", 7*3600)
	// Hạn nằm trong thời hạn phiên 12 giờ để đẩy đồng hồ tới hạn không làm phiên đăng nhập hết hạn.
	due := e.clk.Now().Add(3 * time.Hour).Truncate(time.Second)

	v := decode[DeadlineDTO](t, e.must(huongEmail, http.MethodPut, path, dueBody(due.In(saigon)), http.StatusOK))
	if v.LessonID != hwID.String() || v.DueAt == nil || !v.DueAt.Equal(due) || v.DueAt.Location() != time.UTC {
		t.Fatalf("đặt hạn = %+v", v)
	}
	var stored time.Time
	if err := e.dbx.Get(&stored, `SELECT due_at FROM class_lesson_deadlines WHERE class_id = $1 AND lesson_id = $2`, basic01ID, hwID); err != nil || !stored.Equal(due) {
		t.Fatalf("due_at = %v err=%v", stored, err)
	}
	// Đổi hạn ghi đè, lùi sớm hơn 1 giờ.
	due = due.Add(-time.Hour)
	e.must(huongEmail, http.MethodPut, path, dueBody(due), http.StatusOK)
	if n := e.count(`SELECT count(*) FROM class_lesson_deadlines`); n != 1 {
		t.Fatalf("số hạn nộp = %d", n)
	}
	my := decode[MySubmissionsDTO](t, e.must(anEmail, http.MethodGet, mySubmissionsURL(basic01ID, hwID), "", http.StatusOK))
	if my.Deadline == nil || !my.Deadline.Equal(due) || my.Status != "notSubmitted" || !my.CanSubmit {
		t.Fatalf("hạn của học viên = %+v", my)
	}

	// Nộp đúng mốc hạn không muộn; sau hạn một giây là muộn.
	e.clk.Set(due)
	if s := e.submit(anEmail, repoBody("https://github.com/an/sql")); s.IsLate {
		t.Fatalf("nộp đúng hạn bị đánh muộn: %+v", s)
	}
	e.advance(time.Second)
	if s := e.submit(anEmail, repoBody("https://github.com/an/sql-2")); !s.IsLate {
		t.Fatalf("nộp sau hạn không bị đánh muộn: %+v", s)
	}

	// Xóa hạn: 204, lần xóa thứ hai không đổi gì và không ghi audit.
	e.must(huongEmail, http.MethodDelete, path, "", http.StatusNoContent)
	e.must(huongEmail, http.MethodDelete, path, "", http.StatusNoContent)
	if n := e.count(`SELECT count(*) FROM class_lesson_deadlines`); n != 0 {
		t.Fatalf("số hạn nộp sau khi xóa = %d", n)
	}
	raw := e.must(anEmail, http.MethodGet, mySubmissionsURL(basic01ID, hwID), "", http.StatusOK)
	if !strings.Contains(string(raw), `"deadline":null`) {
		t.Fatalf("sau khi xóa hạn = %s", raw)
	}

	// Ba lần đổi hạn là ba dòng audit với hạn cũ và mới.
	var logs []struct {
		Before []byte `db:"before"`
		After  []byte `db:"after"`
	}
	if err := e.dbx.Select(&logs, `SELECT before, after FROM audit_logs WHERE action = $1 AND target_id = $2 ORDER BY at, id`,
		audit.ActionClassDeadlineChanged, basic01ID); err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 {
		t.Fatalf("audit hạn nộp = %d", len(logs))
	}
	first, last := decode[map[string]any](t, logs[0].Before), decode[map[string]any](t, logs[2].After)
	if first["dueAt"] != nil || first["lessonTitle"] != "Bài tập: truy vấn SELECT" || first["classCode"] != "basic01" ||
		last["dueAt"] != nil || decode[map[string]any](t, logs[2].Before)["dueAt"] != due.UTC().Format(time.RFC3339) {
		t.Fatalf("audit hạn nộp = %s / %s", logs[0].Before, logs[2].Before)
	}

	// Lỗi dữ liệu.
	details := e.expectError(huongEmail, http.MethodPut, path, `{"dueAt":"ngày mai"}`, http.StatusUnprocessableEntity, "VALIDATION_FAILED",
		ErrInvalidDueAt.Msg)
	if details["dueAt"] != ErrInvalidDueAt.Msg {
		t.Fatalf("details = %v", details)
	}
	e.expectError(huongEmail, http.MethodPut, path, `{}`, http.StatusBadRequest, "VALIDATION_FAILED", "")

	// Lớp nháp đặt được hạn; lớp đã kết thúc thì không; admin đặt được cho lớp bất kỳ.
	e.must(huongEmail, http.MethodPut, deadlineURL(basic03ID, hwID), dueBody(due), http.StatusOK)
	e.expectError(huongEmail, http.MethodPut, deadlineURL(basic00ID, hwID), dueBody(due), http.StatusConflict, "INVALID_TRANSITION",
		ErrClassEndedDeadline.Msg)
	e.expectError(huongEmail, http.MethodDelete, deadlineURL(basic00ID, hwID), "", http.StatusConflict, "INVALID_TRANSITION",
		ErrClassEndedDeadline.Msg)
	e.must(adminEmail, http.MethodPut, deadlineURL(go01ID, hwOtherID), dueBody(due), http.StatusOK)
}

func TestStudentGates(t *testing.T) {
	e := newIT(t)
	body := repoBody("https://github.com/an/sql")

	// Đã rời lớp và người ngoài lớp không đọc, không nộp được.
	for _, tc := range []struct{ email, msg string }{{thaoEmail, ErrMemberDropped.Msg}, {linhEmail, ErrNotMember.Msg}} {
		e.expectError(tc.email, http.MethodGet, mySubmissionsURL(basic01ID, hwID), "", http.StatusForbidden, "FORBIDDEN", tc.msg)
		e.expectError(tc.email, http.MethodPost, mySubmissionsURL(basic01ID, hwID), body, http.StatusForbidden, "FORBIDDEN", tc.msg)
	}
	// Lớp nháp và lớp đã kết thúc: xem được, không nộp được, lý do khóa đúng thông điệp lỗi.
	for _, tc := range []struct {
		classID uuid.UUID
		msg     string
	}{{basic03ID, "Lớp chưa bắt đầu."}, {basic00ID, ErrClassEndedSubmit.Msg}} {
		v := decode[MySubmissionsDTO](t, e.must(anEmail, http.MethodGet, mySubmissionsURL(tc.classID, hwID), "", http.StatusOK))
		if v.CanSubmit || v.LockedReason == nil || *v.LockedReason != tc.msg {
			t.Fatalf("lớp %s = %+v", tc.classID, v)
		}
		e.expectError(anEmail, http.MethodPost, mySubmissionsURL(tc.classID, hwID), body, http.StatusConflict, "INVALID_TRANSITION", tc.msg)
	}
	// Vai trò: staff không dùng route học viên, học viên không dùng route staff.
	e.expectError(huongEmail, http.MethodGet, mySubmissionsURL(basic01ID, hwID), "", http.StatusForbidden, "FORBIDDEN", "")
	s := e.submit(anEmail, body)
	for _, r := range []struct{ method, path, body string }{
		{http.MethodGet, submissionURL(s.ID), ""},
		{http.MethodPost, gradesURL(s.ID), gradeBody(40, 60, "Tự chấm")},
		{http.MethodGet, homeworkURL(basic01ID), ""},
		{http.MethodPut, deadlineURL(basic01ID, hwID), dueBody(e.clk.Now())},
		{http.MethodDelete, deadlineURL(basic01ID, hwID), ""},
	} {
		e.expectError(anEmail, r.method, r.path, r.body, http.StatusForbidden, "FORBIDDEN", "")
	}
	if n := e.count(`SELECT count(*) FROM submissions`); n != 1 {
		t.Fatalf("số bản nộp = %d", n)
	}
}

// Route theo lớp nhận học liệu hoặc id của lớp khác thì NOT_FOUND, không lộ dữ liệu lớp khác.
func TestClassScopedRoutesRejectForeignIDs(t *testing.T) {
	e := newIT(t)
	s := e.submit(anEmail, repoBody("https://github.com/an/sql"))
	due := dueBody(e.clk.Now().Add(time.Hour))
	unknown := uuid.MustParse("01990000-0000-7000-8000-0000000009ff")
	for _, lesson := range []uuid.UUID{hwOtherID, lessonTableID, unknown} {
		e.expectError(huongEmail, http.MethodPut, deadlineURL(basic01ID, lesson), due, http.StatusNotFound, "NOT_FOUND", ErrLessonNotInClass.Msg)
		e.expectError(huongEmail, http.MethodDelete, deadlineURL(basic01ID, lesson), "", http.StatusNotFound, "NOT_FOUND", ErrLessonNotInClass.Msg)
		e.expectError(anEmail, http.MethodGet, mySubmissionsURL(basic01ID, lesson), "", http.StatusNotFound, "NOT_FOUND", ErrLessonNotInClass.Msg)
		e.expectError(anEmail, http.MethodPost, mySubmissionsURL(basic01ID, lesson), repoBody("https://github.com/an/sql"),
			http.StatusNotFound, "NOT_FOUND", ErrLessonNotInClass.Msg)
	}
	// Lớp không tồn tại.
	e.expectError(adminEmail, http.MethodPut, deadlineURL(unknown, hwID), due, http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(adminEmail, http.MethodDelete, deadlineURL(unknown, hwID), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(adminEmail, http.MethodGet, homeworkURL(unknown), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(anEmail, http.MethodGet, mySubmissionsURL(unknown, hwID), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(anEmail, http.MethodPost, mySubmissionsURL(unknown, hwID), repoBody("https://github.com/an/sql"), http.StatusNotFound, "NOT_FOUND", "")
	// Bản nộp không tồn tại; id sai định dạng cũng là NOT_FOUND như mọi route khác.
	e.expectError(adminEmail, http.MethodGet, submissionURL(unknown.String()), "", http.StatusNotFound, "NOT_FOUND", ErrSubmissionNotFound.Msg)
	e.expectError(adminEmail, http.MethodPost, gradesURL(unknown.String()), gradeBody(1, 1, "x"), http.StatusNotFound, "NOT_FOUND",
		ErrSubmissionNotFound.Msg)
	e.expectError(adminEmail, http.MethodGet, submissionURL("abc"), "", http.StatusNotFound, "NOT_FOUND", "")

	if n := e.count(`SELECT count(*) FROM class_lesson_deadlines`); n != 0 {
		t.Fatalf("hạn nộp bị ghi = %d", n)
	}
	if got := e.statuses(memberAnID); len(got) != 1 || got[0] != "pending" || s.No != 1 {
		t.Fatalf("trạng thái = %v", got)
	}
}

func TestOtherTeacherIsForbidden(t *testing.T) {
	e := newIT(t)
	s := e.submit(anEmail, repoBody("https://github.com/an/sql"))
	due := dueBody(e.clk.Now().Add(time.Hour))

	e.expectError(baoEmail, http.MethodGet, submissionURL(s.ID), "", http.StatusForbidden, "FORBIDDEN", classes.ErrNotOwnClass.Msg)
	e.expectError(baoEmail, http.MethodPost, gradesURL(s.ID), gradeBody(40, 60, "Tốt"), http.StatusForbidden, "FORBIDDEN", ErrNotOwnGrade.Msg)
	e.expectError(baoEmail, http.MethodPut, deadlineURL(basic01ID, hwID), due, http.StatusForbidden, "FORBIDDEN", ErrNotOwnDeadline.Msg)
	e.expectError(baoEmail, http.MethodDelete, deadlineURL(basic01ID, hwID), "", http.StatusForbidden, "FORBIDDEN", ErrNotOwnDeadline.Msg)
	e.expectError(baoEmail, http.MethodGet, homeworkURL(basic01ID), "", http.StatusForbidden, "FORBIDDEN", classes.ErrNotOwnClass.Msg)
	// Giảng viên basic01 cũng không chạm được lớp go01 của Bảo.
	e.expectError(huongEmail, http.MethodGet, homeworkURL(go01ID), "", http.StatusForbidden, "FORBIDDEN", classes.ErrNotOwnClass.Msg)
	e.expectError(huongEmail, http.MethodPut, deadlineURL(go01ID, hwOtherID), due, http.StatusForbidden, "FORBIDDEN", ErrNotOwnDeadline.Msg)

	if n := e.count(`SELECT count(*) FROM submission_grades`) + e.count(`SELECT count(*) FROM class_lesson_deadlines`) +
		e.count(`SELECT count(*) FROM audit_logs WHERE action LIKE 'homework.%' OR action = $1`, audit.ActionClassDeadlineChanged); n != 0 {
		t.Fatalf("thao tác bị từ chối vẫn ghi %d dòng", n)
	}
	// Admin xem và chấm được mọi lớp.
	e.must(adminEmail, http.MethodGet, submissionURL(s.ID), "", http.StatusOK)
	e.grade(adminEmail, s.ID, gradeBody(40, 60, "Tốt"))
}

func TestConcurrentSubmitsAreSerialized(t *testing.T) {
	e := newIT(t)
	cookie := e.as(anEmail)
	const n = 6
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = e.serve(cookie, http.MethodPost, mySubmissionsURL(basic01ID, hwID), repoBody(fmt.Sprintf("https://github.com/an/sql-%d", i))).code
		}()
	}
	wg.Wait()
	for i, c := range codes {
		if c != http.StatusCreated {
			t.Fatalf("lần nộp %d = %d", i, c)
		}
	}
	// Mỗi lần nộp thay bản trước: số lần nộp liên tiếp 1..n, đúng một bản chờ chấm và đó là bản mới nhất.
	got := e.statuses(memberAnID)
	if len(got) != n || got[n-1] != "pending" || strings.Count(strings.Join(got, ","), "superseded") != n-1 {
		t.Fatalf("trạng thái = %v", got)
	}
	if max := e.count(`SELECT max(attempt_no) FROM submissions WHERE class_member_id = $1`, memberAnID); max != n {
		t.Fatalf("attempt lớn nhất = %d", max)
	}
}

func TestConcurrentGradesAreSerialized(t *testing.T) {
	e := newIT(t)
	s := e.submit(anEmail, repoBody("https://github.com/an/sql"))
	cookie := e.as(huongEmail)
	const n = 4
	outs := make([]apiResp, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			outs[i] = e.serve(cookie, http.MethodPost, gradesURL(s.ID), gradeBody(40, 10*i, fmt.Sprintf("Lần %d", i)))
		}()
	}
	wg.Wait()
	updated := 0
	for i, r := range outs {
		if r.code != http.StatusCreated {
			t.Fatalf("lần chấm %d = %d: %s", i, r.code, r.body)
		}
		if decode[GradeOutcomeDTO](t, r.body).Updated {
			updated++
		}
	}
	// Đúng một lần là chấm đầu, các lần sau là sửa điểm; email và audit khớp từng lần.
	if updated != n-1 {
		t.Fatalf("số lần sửa điểm = %d", updated)
	}
	if g, u := e.auditCount(audit.ActionHomeworkGraded), e.auditCount(audit.ActionHomeworkGradeUpdated); g != 1 || u != n-1 {
		t.Fatalf("audit graded=%d updated=%d", g, u)
	}
	if m := e.count(`SELECT count(*) FROM email_outbox WHERE template = 'homework_graded'`); m != n {
		t.Fatalf("email = %d", m)
	}
	if m := e.count(`SELECT count(*) FROM submission_grades WHERE submission_id = $1`, s.ID); m != n {
		t.Fatalf("lần chấm = %d", m)
	}
}

// Chấm Đạt và nộp lại cùng lúc: đúng một bên thắng, không bao giờ có bản bị thay đã có điểm.
func TestConcurrentGradeAndResubmit(t *testing.T) {
	e := newIT(t)
	an, huong := e.as(anEmail), e.as(huongEmail)
	for round := range 5 {
		e.exec(`DELETE FROM grade_criterion_scores`)
		e.exec(`DELETE FROM submission_grades`)
		e.exec(`DELETE FROM submissions`)
		s := e.submit(anEmail, repoBody("https://github.com/an/sql"))
		var grade, resubmit apiResp
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			grade = e.serve(huong, http.MethodPost, gradesURL(s.ID), gradeBody(40, 60, "Tốt"))
		}()
		go func() {
			defer wg.Done()
			resubmit = e.serve(an, http.MethodPost, mySubmissionsURL(basic01ID, hwID), repoBody("https://github.com/an/sql-2"))
		}()
		wg.Wait()
		got := strings.Join(e.statuses(memberAnID), ",")
		switch {
		case grade.code == http.StatusCreated && resubmit.code == http.StatusConflict:
			if got != "passed" || !strings.Contains(string(resubmit.body), ErrAlreadyPassed.Msg) {
				t.Fatalf("vòng %d chấm thắng: trạng thái %s, nộp lại %s", round, got, resubmit.body)
			}
		case resubmit.code == http.StatusCreated && grade.code == http.StatusConflict:
			if got != "superseded,pending" || !strings.Contains(string(grade.body), ErrNotLatest.Msg) {
				t.Fatalf("vòng %d nộp lại thắng: trạng thái %s, chấm %s", round, got, grade.body)
			}
		default:
			t.Fatalf("vòng %d: chấm %d %s, nộp lại %d %s", round, grade.code, grade.body, resubmit.code, resubmit.body)
		}
		if n := e.count(`SELECT count(*) FROM submissions s JOIN submission_grades g ON g.submission_id = s.id
			WHERE s.status = 'superseded'`); n != 0 {
			t.Fatalf("vòng %d: bản bị thay có điểm", round)
		}
	}
}

func TestSubmissionFilesDownloadAsAttachment(t *testing.T) {
	e := newIT(t)
	id := e.upload(anEmail, "bai-nop.sql")
	e.submit(anEmail, fmt.Sprintf(`{"mediaIds":[%q]}`, id))
	url := "/api/v1/media/" + id + "/url"
	for _, email := range []string{anEmail, huongEmail, adminEmail} {
		s := decode[media.SignedURLDTO](t, e.must(email, http.MethodGet, url, "", http.StatusOK))
		if !strings.Contains(s.URL, "response-content-disposition=attachment") {
			t.Fatalf("%s tải file không phải attachment: %s", email, s.URL)
		}
	}
	for _, email := range []string{khangEmail, baoEmail, linhEmail} {
		e.expectError(email, http.MethodGet, url, "", http.StatusForbidden, "FORBIDDEN", "")
	}

	// Giới hạn upload của học viên: chỉ file bài tập, tối đa 20 MB, đúng định dạng.
	for _, tc := range []struct {
		body   string
		status int
		code   string
	}{
		{`{"kind":"video","fileName":"a.mp4","contentType":"video/mp4","sizeBytes":5}`, http.StatusForbidden, "FORBIDDEN"},
		{`{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":20971521}`, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{`{"kind":"document","fileName":"a.exe","contentType":"application/octet-stream","sizeBytes":5}`, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
	} {
		e.expectError(anEmail, http.MethodPost, "/api/v1/me/uploads", tc.body, tc.status, tc.code, "")
	}
	e.must(anEmail, http.MethodPost, "/api/v1/me/uploads", `{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":20971520}`,
		http.StatusCreated)
	// Học viên đã rời lớp không tải lên được; admin route upload vẫn chỉ admin.
	e.expectError(thaoEmail, http.MethodPost, "/api/v1/me/uploads", `{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":5}`,
		http.StatusForbidden, "FORBIDDEN", "")
	e.expectError(anEmail, http.MethodPost, "/api/v1/media/uploads", `{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":5}`,
		http.StatusForbidden, "FORBIDDEN", "")
}

func TestClassHomeworkOverview(t *testing.T) {
	e := newIT(t)
	due := e.clk.Now().Add(time.Hour).Truncate(time.Second)
	e.must(huongEmail, http.MethodPut, deadlineURL(basic01ID, hwID), dueBody(due), http.StatusOK)
	sAn := e.submit(anEmail, repoBody("https://github.com/an/sql"))
	e.grade(huongEmail, sAn.ID, gradeBody(40, 60, "Tốt"))

	// Chưa quá hạn: Khang chưa nộp không tính quá hạn.
	v := decode[ClassHomeworkDTO](t, e.must(huongEmail, http.MethodGet, homeworkURL(basic01ID), "", http.StatusOK))
	if len(v.Items) != 1 || v.Items[0].Stats != (StatsDTO{Submitted: 1, Passed: 1}) {
		t.Fatalf("tổng quan = %+v", v)
	}

	e.advance(2 * time.Hour)
	e.submit(khangEmail, repoBody("https://github.com/khang/sql"))
	e.advance(3*24*time.Hour + time.Hour)
	v = decode[ClassHomeworkDTO](t, e.must(huongEmail, http.MethodGet, homeworkURL(basic01ID), "", http.StatusOK))
	it := v.Items[0]
	if it.Lesson.ID != hwID.String() || it.MaxPoints != 100 || it.PassPercent != 70 || it.Deadline == nil || !it.Deadline.Equal(due) ||
		it.Stats != (StatsDTO{Submitted: 2, Pending: 1, Passed: 1}) || len(it.Members) != 2 {
		t.Fatalf("tổng quan = %+v", it)
	}
	// Thành viên đang học theo tên: Nguyễn Hoàng An rồi Vũ Đức Khang; Thảo đã rời không có mặt.
	an, khang := it.Members[0], it.Members[1]
	if an.MemberID != memberAnID.String() || an.Status != "passed" || an.Latest == nil || an.Latest.TotalPoints == nil ||
		*an.Latest.TotalPoints != 100 || an.WaitingDays != nil {
		t.Fatalf("An = %+v", an)
	}
	if khang.MemberID != memberKhangID.String() || khang.Status != "pending" || khang.Latest == nil || !khang.Latest.IsLate ||
		!khang.Latest.HasRepo || khang.WaitingDays == nil || *khang.WaitingDays != 3 {
		t.Fatalf("Khang = %+v", khang)
	}

	// Quá hạn mà chưa nộp được đếm riêng.
	e.exec(`UPDATE class_members SET status = 'active', dropped_at = NULL WHERE id = $1`, memberThaoID)
	v = decode[ClassHomeworkDTO](t, e.must(adminEmail, http.MethodGet, homeworkURL(basic01ID), "", http.StatusOK))
	thao := 0
	for _, m := range v.Items[0].Members {
		if m.MemberID == memberThaoID.String() && m.Status == "notSubmitted" && m.Latest == nil {
			thao++
		}
	}
	if v.Items[0].Stats.OverdueNotSubmitted != 1 || len(v.Items[0].Members) != 3 || thao != 1 {
		t.Fatalf("tổng quan có Thảo = %+v", v.Items[0])
	}
}

// TestGoldenJSON ghi phản hồi 3 route đọc; web dùng các file này làm fixture.
func TestGoldenJSON(t *testing.T) {
	e := newIT(t)
	due := e.clk.Now().Add(24 * time.Hour)
	e.must(huongEmail, http.MethodPut, deadlineURL(basic01ID, hwID), dueBody(due), http.StatusOK)
	s1 := e.submit(anEmail, `{"repoUrl":"https://github.com/an/sql-select","commitRef":"a1b2c3d"}`)
	e.advance(time.Hour)
	e.grade(huongEmail, s1.ID, gradeBody(30, 20, "Truy vấn đúng nhưng thiếu phần giải thích."))
	e.advance(25 * time.Hour)
	file := e.upload(anEmail, "giai-thich.md")
	s2 := e.submit(anEmail, fmt.Sprintf(`{"repoUrl":"https://github.com/an/sql-select","commitRef":"d4e5f6a","mediaIds":[%q]}`, file))
	e.advance(time.Hour)
	e.submit(khangEmail, repoBody("https://github.com/khang/sql"))

	assertGolden(t, "my-submissions.json", e.must(anEmail, http.MethodGet, mySubmissionsURL(basic01ID, hwID), "", http.StatusOK))
	e.advance(time.Hour)
	e.grade(huongEmail, s2.ID, gradeBody(40, 45, "Đã bổ sung giải thích, đạt yêu cầu."))
	assertGolden(t, "submission-detail.json", e.must(huongEmail, http.MethodGet, submissionURL(s2.ID), "", http.StatusOK))
	e.advance(2 * 24 * time.Hour)
	assertGolden(t, "class-homework.json", e.must(huongEmail, http.MethodGet, homeworkURL(basic01ID), "", http.StatusOK))
}

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizeGolden thay thời điểm (khóa kết thúc bằng "At" và khóa "deadline") bằng mốc cố định và id sinh lúc chạy
// (không phải id cố định 01990000-…) bằng id giả theo thứ tự xuất hiện, để golden ổn định giữa các lần chạy.
func normalizeGolden(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("golden: %v: %s", err, raw)
	}
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
			if strings.HasSuffix(key, "At") || key == "deadline" {
				return "2026-10-05T08:00:00Z"
			}
			return uuidPattern.ReplaceAllStringFunc(x, func(id string) string {
				if strings.HasPrefix(id, "01990000-") {
					return id
				}
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
	if err := enc.Encode(walk("", v)); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func assertGolden(t *testing.T, name string, raw []byte) {
	t.Helper()
	got := normalizeGolden(t, raw)
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil { //nolint:gosec // golden JSON công khai trong repo
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // đường dẫn cố định trong testdata
	if err != nil {
		t.Fatalf("golden %s: %v (chạy lại với -update)", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden %s khác (chạy lại với -update nếu thay đổi là chủ ý):\n%s", name, got)
	}
}
