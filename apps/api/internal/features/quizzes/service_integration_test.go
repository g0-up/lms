//go:build integration

package quizzes

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/middleware"
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
	unitDBV1ID    = uuid.MustParse(testdb.UnitDBV1ID)
	lessonTableID = uuid.MustParse(testdb.LessonDBTableV1ID) // markdown, không phải quiz
)

// Dữ liệu thêm chèn bằng SQL trong test; id cố định (tiền tố 01990000-) để golden ổn định.
var (
	linhID        = uuid.MustParse("01990000-0000-7000-8000-000000000902") // học viên go01, không thuộc basic01
	khangID       = uuid.MustParse("01990000-0000-7000-8000-000000000904") // basic01
	thaoID        = uuid.MustParse("01990000-0000-7000-8000-000000000905") // đã rời basic01
	baoID         = uuid.MustParse("01990000-0000-7000-8000-000000000901") // giảng viên go01
	memberKhangID = uuid.MustParse("01990000-0000-7000-8000-000000000914")
	memberThaoID  = uuid.MustParse("01990000-0000-7000-8000-000000000915")
	stageGoID     = uuid.MustParse("01990000-0000-7000-8000-000000000941")
	stageGoV1ID   = uuid.MustParse("01990000-0000-7000-8000-000000000942")
	courseGoID    = uuid.MustParse("01990000-0000-7000-8000-000000000946")
	courseGoV1ID  = uuid.MustParse("01990000-0000-7000-8000-000000000947")
	go01ID        = uuid.MustParse("01990000-0000-7000-8000-000000000951") // active, khóa GO
	basic03ID     = uuid.MustParse("01990000-0000-7000-8000-000000000952") // draft, An là thành viên
	basic00ID     = uuid.MustParse("01990000-0000-7000-8000-000000000953") // ended, An là thành viên
	memberLinhID  = uuid.MustParse("01990000-0000-7000-8000-000000000961")
	memberAn03ID  = uuid.MustParse("01990000-0000-7000-8000-000000000962")
	memberAn00ID  = uuid.MustParse("01990000-0000-7000-8000-000000000963")

	// Quiz luyện tập (không trộn): câu 1 một đáp án 2 điểm, câu 2 nhiều đáp án 3 điểm, câu 3 một đáp án 1 điểm.
	practiceID = uuid.MustParse("01990000-0000-7000-8000-000000000701")
	pq1        = uuid.MustParse("01990000-0000-7000-8000-000000000711")
	pq2        = uuid.MustParse("01990000-0000-7000-8000-000000000712")
	pq3        = uuid.MustParse("01990000-0000-7000-8000-000000000713")
	po1a       = uuid.MustParse("01990000-0000-7000-8000-000000000721") // đúng
	po1b       = uuid.MustParse("01990000-0000-7000-8000-000000000722")
	po2a       = uuid.MustParse("01990000-0000-7000-8000-000000000724") // đúng
	po2b       = uuid.MustParse("01990000-0000-7000-8000-000000000725")
	po2c       = uuid.MustParse("01990000-0000-7000-8000-000000000726") // đúng
	po3a       = uuid.MustParse("01990000-0000-7000-8000-000000000727")
	po3b       = uuid.MustParse("01990000-0000-7000-8000-000000000728") // đúng

	// Quiz đánh giá (trộn, 2 lượt, 10 phút, đạt 70%): 3 + 3 + 4 điểm.
	assessmentID = uuid.MustParse("01990000-0000-7000-8000-000000000702")
	aq1          = uuid.MustParse("01990000-0000-7000-8000-000000000731")
	aq2          = uuid.MustParse("01990000-0000-7000-8000-000000000732")
	aq3          = uuid.MustParse("01990000-0000-7000-8000-000000000733")
	ao1a         = uuid.MustParse("01990000-0000-7000-8000-000000000741") // đúng
	ao1b         = uuid.MustParse("01990000-0000-7000-8000-000000000742")
	ao2a         = uuid.MustParse("01990000-0000-7000-8000-000000000743")
	ao2b         = uuid.MustParse("01990000-0000-7000-8000-000000000744") // đúng
	ao3a         = uuid.MustParse("01990000-0000-7000-8000-000000000745") // đúng
	ao3b         = uuid.MustParse("01990000-0000-7000-8000-000000000746") // đúng
	ao3c         = uuid.MustParse("01990000-0000-7000-8000-000000000747")

	// Học liệu quiz thiếu cấu hình: lượt của nó không chấm được, dùng để kiểm lỗi một lượt không chặn lượt khác.
	brokenID = uuid.MustParse("01990000-0000-7000-8000-000000000703")
)

const (
	anEmail    = "an.nguyen@gmail.com"
	huongEmail = "huong.le@goup.vn"
	linhEmail  = "linh.do@gmail.com"
	khangEmail = "khang.vu@gmail.com"
	thaoEmail  = "thao.vo@gmail.com"

	msgAlreadySubmitted = "Lượt làm bài này đã được nộp."
	msgTimeUpNotSaved   = "Đã hết giờ làm bài; câu trả lời không được lưu."
	msgTimeUpAuto       = "Đã hết giờ làm bài; hệ thống đã tự nộp với các câu đã lưu."
	msgClassEnded       = "Lớp đã kết thúc, chỉ xem được học liệu."
)

// itEnv dựng quizzes trên Postgres thật cùng identity thật (đăng nhập, cookie phiên, SessionAuth); Service trộn
// câu bằng reverseShuffler để thứ tự lượt đánh giá xác định.
type itEnv struct {
	t       *testing.T
	dbx     *sqlx.DB
	clk     *clock.Fake
	ident   *identity.Service
	svc     *Service
	review  *ReviewService
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
			PublicBaseURL: "http://localhost:5173", AppEnv: config.EnvE2E,
		},
	})
	svc := NewService(Deps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Repo: PGRepo{}, Members: classes.PGMembershipReader{},
		Progress: learning.PGProgressRepo{}, Clock: clk, IDs: ids.V7{}, Shuffle: reverseShuffler{},
	})
	e := &itEnv{t: t, dbx: dbx, clk: clk, ident: ident, svc: svc, cookies: map[string]string{}}
	e.seedExtra()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recover(slog.New(slog.DiscardHandler)), middleware.SecurityHeaders())
	authed := engine.Group("/api/v1", identity.NewMiddleware(ident, false).SessionAuth())
	e.review = NewReviewService(ReviewDeps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Repo: PGRepo{}, Mail: mailer.NewService(mailer.NewOutboxRepo(), nil, clk),
		Audit: audit.PG{Clock: clk}, Clock: clk, IDs: ids.V7{},
		Next:          grading.NewService(grading.Deps{Tx: db.TxRunner{DB: dbx}, Repo: grading.PGRepo{}, Clock: clk}),
		PublicBaseURL: "http://localhost:5173",
	})
	student := func(c *gin.Context) (uuid.UUID, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return uuid.Nil, false
		}
		return u.ID(), true
	}
	actor := func(c *gin.Context) (uuid.UUID, domain.Role, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return uuid.Nil, "", false
		}
		return u.ID(), u.Role(), true
	}
	NewHandler(svc, e.review, student, actor).Register(authed, Guards{
		Staff:   identity.RequireRole(domain.RoleAdmin, domain.RoleTeacher),
		Student: identity.RequireRole(domain.RoleStudent),
	})
	e.engine = engine
	return e
}

// seedExtra thêm vào chặng Database của BASIC v1 một quiz luyện tập, một quiz đánh giá và một quiz thiếu cấu hình;
// khóa GO riêng với lớp go01 của Linh; basic03 nháp và basic00 đã kết thúc có An; basic01 thêm Khang (đang học) và
// Thảo (đã rời).
func (e *itEnv) seedExtra() {
	e.t.Helper()
	const hash = `$argon2id$v=19$m=19456,t=2,p=1$hVgN0I6aZI+AWq7rW6j/XQ$i8SWRmwDmbK6kHOJBAyuz4EOO7oK2G/nLDOcK3Q/NHE`
	users := []struct {
		id                uuid.UUID
		email, name, role string
	}{
		{baoID, "bao.pham@goup.vn", "Phạm Quốc Bảo", "teacher"},
		{linhID, linhEmail, "Đỗ Ngọc Linh", "student"},
		{khangID, khangEmail, "Vũ Đức Khang", "student"},
		{thaoID, thaoEmail, "Võ Phương Thảo", "student"},
	}
	for _, u := range users {
		e.exec(`INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash)
			VALUES ($1, $2, $2, $3, $4, 'active', $5)`, u.id, u.email, u.name, u.role, hash)
	}

	e.exec(`INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type, required) VALUES
		($1, $4, $5, 'db-quiz-luyen', 3, 'Luyện tập: khóa chính', 'quiz', false),
		($2, $4, $5, 'db-quiz-kt', 4, 'Kiểm tra: chỉ mục', 'quiz', true),
		($3, $4, $5, 'db-quiz-hong', 5, 'Quiz thiếu cấu hình', 'quiz', false)`,
		practiceID, assessmentID, brokenID, stageDBV1ID, unitDBV1ID)
	e.exec(`INSERT INTO quiz_settings (lesson_id, mode, max_attempts, time_limit_seconds, shuffle, pass_percent) VALUES
		($1, 'practice', NULL, NULL, false, NULL), ($2, 'assessment', 2, 600, true, 70)`, practiceID, assessmentID)
	type option struct {
		id      uuid.UUID
		body    string
		correct bool
	}
	questions := []struct {
		id, lesson         uuid.UUID
		key, typ, body, ex string
		points             int
		options            []option
	}{
		{pq1, practiceID, "khoa-chinh", "single", "Khóa chính dùng để làm gì?", "Khóa chính định danh duy nhất mỗi dòng.", 2,
			[]option{{po1a, "Định danh duy nhất mỗi dòng", true}, {po1b, "Tăng tốc mọi truy vấn", false}}},
		{pq2, practiceID, "rang-buoc", "multi", "Ràng buộc nào chặn trùng giá trị?", "PRIMARY KEY và UNIQUE đều chặn trùng.", 3,
			[]option{{po2a, "PRIMARY KEY", true}, {po2b, "CHECK", false}, {po2c, "UNIQUE", true}}},
		{pq3, practiceID, "khoa-ngoai", "single", "Khóa ngoại tham chiếu tới đâu?", "", 1,
			[]option{{po3a, "Một cột bất kỳ", false}, {po3b, "Khóa của bảng khác", true}}},
		{aq1, assessmentID, "chi-muc", "single", "Chỉ mục B-tree phù hợp với?", "B-tree hỗ trợ so sánh khoảng.", 3,
			[]option{{ao1a, "Truy vấn khoảng", true}, {ao1b, "Tìm toàn văn", false}}},
		{aq2, assessmentID, "chi-phi", "single", "Chỉ mục làm chậm thao tác nào?", "Mỗi lần ghi phải cập nhật chỉ mục.", 3,
			[]option{{ao2a, "SELECT", false}, {ao2b, "INSERT", true}}},
		{aq3, assessmentID, "ket-hop", "multi", "Chỉ mục (a, b) dùng được cho điều kiện nào?", "Tiền tố trái của chỉ mục.", 4,
			[]option{{ao3a, "WHERE a = 1", true}, {ao3b, "WHERE a = 1 AND b = 2", true}, {ao3c, "WHERE b = 2", false}}},
	}
	position := map[uuid.UUID]int{}
	for _, q := range questions {
		position[q.lesson]++
		var exHTML *string
		if q.ex != "" {
			h := "<p>Giải thích: " + q.ex + "</p>"
			exHTML = &h
		}
		e.exec(`INSERT INTO questions (id, lesson_id, question_key, type, body_source, body_html, explanation_source,
			explanation_html, points, position) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			q.id, q.lesson, q.key, q.typ, q.body, "<p>"+q.body+"</p>", q.ex, exHTML, q.points, position[q.lesson])
		for i, o := range q.options {
			e.exec(`INSERT INTO question_options (id, question_id, body, is_correct, position) VALUES ($1, $2, $3, $4, $5)`,
				o.id, q.id, o.body, o.correct, i+1)
		}
	}

	e.exec(`INSERT INTO stages (id, code, name, created_by) VALUES ($1, 'GO', 'Go cơ bản', $2)`, stageGoID, adminID)
	e.exec(`INSERT INTO stage_versions (id, stage_id, version_no, status, title, description, published_at, created_by)
		VALUES ($1, $2, 1, 'published', 'Go cơ bản', 'Nhập môn Go.', now() - interval '30 days', $3)`, stageGoV1ID, stageGoID, adminID)
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

// as đăng nhập thật qua identity (một lần cho mỗi email) và trả cookie phiên. Không an toàn khi gọi song song: lấy
// cookie trước khi mở goroutine.
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

type apiResp struct {
	code int
	body []byte
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
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

// expectError gọi API và kiểm mã HTTP, mã lỗi và (nếu có) message.
func (e *itEnv) expectError(email, method, path, body string, status int, code, msg string) {
	e.t.Helper()
	r := e.serve(e.as(email), method, path, body)
	env := errorOf(e.t, r)
	if r.code != status || env.Error.Code != code || (msg != "" && env.Error.Message != msg) {
		e.t.Fatalf("%s %s %s = %d %s %q; muốn %d %s %q", email, method, path, r.code, env.Error.Code, env.Error.Message,
			status, code, msg)
	}
}

func errorOf(t *testing.T, r apiResp) errorEnvelope {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(r.body, &env); err != nil {
		t.Fatalf("envelope lỗi: %v: %s", err, r.body)
	}
	return env
}

func decode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode: %v: %s", err, raw)
	}
	return v
}

func attemptsURL(classID, lessonID uuid.UUID) string {
	return "/api/v1/me/classes/" + classID.String() + "/lessons/" + lessonID.String() + "/attempts"
}

func attemptURL(id string) string { return "/api/v1/me/attempts/" + id }

func answerURL(id string, questionID uuid.UUID) string {
	return attemptURL(id) + "/answers/" + questionID.String()
}

func submitURL(id string) string { return attemptURL(id) + "/submit" }

func pick(optionIDs ...uuid.UUID) string {
	quoted := make([]string, len(optionIDs))
	for i, id := range optionIDs {
		quoted[i] = `"` + id.String() + `"`
	}
	return `{"optionIds":[` + strings.Join(quoted, ",") + `]}`
}

func (e *itEnv) overview(email string, classID, lessonID uuid.UUID) OverviewDTO {
	e.t.Helper()
	return decode[OverviewDTO](e.t, e.must(email, http.MethodGet, attemptsURL(classID, lessonID), "", http.StatusOK))
}

// start bắt đầu lượt mới (201) trên basic01 cho Khang hoặc An.
func (e *itEnv) start(email string, lessonID uuid.UUID) OpenAttemptDTO {
	e.t.Helper()
	st := decode[StartDTO](e.t, e.must(email, http.MethodPost, attemptsURL(basic01ID, lessonID), "", http.StatusCreated))
	if st.Resumed {
		e.t.Fatalf("lượt mới bị báo là tiếp tục: %+v", st)
	}
	return st.Attempt
}

func (e *itEnv) save(email, attemptID string, questionID uuid.UUID, optionIDs ...uuid.UUID) SavedDTO {
	e.t.Helper()
	return decode[SavedDTO](e.t, e.must(email, http.MethodPut, answerURL(attemptID, questionID), pick(optionIDs...), http.StatusOK))
}

func (e *itEnv) submit(email, attemptID string) []byte {
	e.t.Helper()
	return e.must(email, http.MethodPost, submitURL(attemptID), "", http.StatusOK)
}

// attemptRow là trạng thái lượt đọc thẳng từ DB.
type attemptRow struct {
	No            int        `db:"attempt_no"`
	DeadlineAt    *time.Time `db:"deadline_at"`
	SubmittedAt   *time.Time `db:"submitted_at"`
	AutoSubmitted bool       `db:"auto_submitted"`
	Score         *int       `db:"score"`
	Passed        *bool      `db:"passed"`
	ReviewStatus  *string    `db:"review_status"`
}

func (e *itEnv) attemptRow(id string) attemptRow {
	e.t.Helper()
	var r attemptRow
	if err := e.dbx.Get(&r, `SELECT attempt_no, deadline_at, submitted_at, auto_submitted, score, passed, review_status
		FROM quiz_attempts WHERE id = $1`, id); err != nil {
		e.t.Fatal(err)
	}
	return r
}

// reviewStatuses là review_status các lượt của (thành viên, quiz) theo attempt_no; NULL là "-".
func (e *itEnv) reviewStatuses(memberID, lessonID uuid.UUID) []string {
	e.t.Helper()
	var out []string
	if err := e.dbx.Select(&out, `SELECT COALESCE(review_status, '-') FROM quiz_attempts
		WHERE class_member_id = $1 AND lesson_id = $2 ORDER BY attempt_no`, memberID, lessonID); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// completedAt đọc completed_at của quiz; ok=false khi chưa có bản ghi tiến độ.
func (e *itEnv) completedAt(memberID, lessonID uuid.UUID) (completed *time.Time, ok bool) {
	e.t.Helper()
	err := e.dbx.Get(&completed, `SELECT completed_at FROM lesson_progress WHERE class_member_id = $1 AND lesson_id = $2`,
		memberID, lessonID)
	if err != nil {
		return nil, false
	}
	return completed, true
}

// insertOpenAttempt chèn thẳng lượt mở có hạn nộp cho trường hợp không bắt đầu được qua API (thành viên đã rời, lớp
// đã kết thúc, quiz thiếu cấu hình).
func (e *itEnv) insertOpenAttempt(memberID, lessonID uuid.UUID, deadline time.Time) string {
	e.t.Helper()
	id := uuid.Must(uuid.NewV7())
	e.exec(`INSERT INTO quiz_attempts (id, class_member_id, lesson_id, attempt_no, question_order, option_order, started_at,
		deadline_at, max_score) VALUES ($1, $2, $3, 1, '{}', '{}', $4, $5, 10)`,
		id, memberID, lessonID, deadline.Add(-10*time.Minute), deadline)
	return id.String()
}

// holdMemberLock giữ khóa dòng class_members trong transaction riêng tới khi gọi release (gọi lại nhiều lần vô hại).
func (e *itEnv) holdMemberLock(memberID uuid.UUID) (release func()) {
	e.t.Helper()
	tx, err := e.dbx.Beginx()
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := tx.Exec(`SELECT 1 FROM class_members WHERE id = $1 FOR UPDATE`, memberID); err != nil {
		_ = tx.Rollback()
		e.t.Fatal(err)
	}
	var once sync.Once
	release = func() { once.Do(func() { _ = tx.Rollback() }) }
	e.t.Cleanup(release)
	return release
}

// waitForLockWaiters chờ tới khi đủ n phiên Postgres đang đợi khóa dòng. Bỏ qua phiên đợi advisory lock: đó là tiến
// trình test khác đang xếp hàng chờ database lms_test, không phải giao dịch của test này.
func (e *itEnv) waitForLockWaiters(n int) {
	e.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if e.count(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'
			AND wait_event <> 'advisory'`) >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.t.Fatalf("không đủ %d phiên đợi khóa thành viên", n)
}

func (e *itEnv) quizQueue() []grading.QueueItem {
	e.t.Helper()
	svc := grading.NewService(grading.Deps{Tx: db.TxRunner{DB: e.dbx}, Repo: grading.PGRepo{}, Clock: e.clk})
	q, err := svc.Queue(e.ctx(), grading.Actor{ID: adminID, Role: domain.RoleAdmin}, grading.Filter{Kind: grading.KindQuiz})
	if err != nil {
		e.t.Fatal(err)
	}
	return q.Items
}

func queueIDs(items []grading.QueueItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Key.ItemID.String())
	}
	sort.Strings(out)
	return out
}

func sorted(xs ...string) []string {
	out := slices.Clone(xs)
	sort.Strings(out)
	return out
}

// Khóa được phép của từng kiểu phản hồi lượt: lượt mở và kết quả đánh giá không có khóa nào mang đáp án đúng, giải
// thích hay phương án đã chọn của kết quả.
var (
	openKeys = []string{"resumed", "attempt", "id", "status", "mode", "no", "limit", "deadlineAt", "serverNow", "savedAt",
		"questions", "type", "bodyHtml", "points", "options", "body", "selectedOptionIds"}
	assessmentKeys = []string{"id", "status", "mode", "no", "startedAt", "submittedAt", "autoSubmitted", "score", "maxScore",
		"percent", "passed", "passPercent", "reviewStatus", "best", "attemptsLeft", "review", "comment", "reviewerName",
		"reviewedAt", "updated", "notes", "questionId", "position", "bodyHtml", "note"}
	practiceKeys = []string{"id", "status", "mode", "no", "startedAt", "submittedAt", "autoSubmitted", "score", "maxScore",
		"percent", "questions", "type", "bodyHtml", "points", "pointsAwarded", "correct", "explanationHtml", "note", "options",
		"body", "isCorrect", "selected", "review", "comment", "reviewerName", "reviewedAt", "updated"}
)

// assertOnlyKeys kiểm mọi khóa object (mọi cấp) trong JSON thô đều thuộc allow-list.
func assertOnlyKeys(t *testing.T, raw []byte, allowed []string) {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("json: %v: %s", err, raw)
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				if !slices.Contains(allowed, k) {
					t.Errorf("khóa %q không được phép trong phản hồi: %s", k, raw)
				}
				walk(val)
			}
		case []any:
			for _, val := range x {
				walk(val)
			}
		}
	}
	walk(v)
}

// assertHidesAnswers kiểm phản hồi không chứa id hay nội dung phương án, đúng/sai hay giải thích của quiz đánh giá.
func assertHidesAnswers(t *testing.T, raw []byte, includeOptions bool) {
	t.Helper()
	s := string(raw)
	banned := []string{"isCorrect", `"correct"`, "explanation", "Giải thích", `"selected"`, "pointsAwarded"}
	if !includeOptions {
		banned = append(banned, `"options"`)
		for _, id := range []uuid.UUID{ao1a, ao1b, ao2a, ao2b, ao3a, ao3b, ao3c} {
			banned = append(banned, id.String())
		}
		banned = append(banned, "Truy vấn khoảng", "WHERE a = 1", "INSERT")
	}
	for _, b := range banned {
		if strings.Contains(s, b) {
			t.Errorf("phản hồi lộ %q: %s", b, raw)
		}
	}
}

func TestPracticeAttemptLifecycle(t *testing.T) {
	e := newIT(t)
	e.as(anEmail)
	auditBefore := e.count(`SELECT count(*) FROM audit_logs`)

	o := e.overview(anEmail, basic01ID, practiceID)
	if o.Mode != "practice" || o.QuestionCount != 3 || o.MaxScore != 6 || o.Limit != nil || o.Used != 0 || !o.CanStart ||
		o.OpenAttemptID != nil || o.Best != nil || len(o.Attempts) != 0 || o.PassPercent != nil || o.TimeLimitMinutes != nil {
		t.Fatalf("màn bắt đầu = %+v", o)
	}
	if _, ok := e.completedAt(memberAnID, practiceID); ok {
		t.Fatal("đọc màn bắt đầu không được ghi tiến độ")
	}

	startedAt := e.clk.Now()
	a := e.start(anEmail, practiceID)
	if a.Status != "open" || a.Mode != "practice" || a.No != 1 || a.Limit != nil || a.DeadlineAt != nil || a.SavedAt != nil ||
		!a.ServerNow.Equal(startedAt) || len(a.Questions) != 3 ||
		a.Questions[0].ID != pq1.String() || a.Questions[1].ID != pq2.String() || a.Questions[2].ID != pq3.String() ||
		a.Questions[1].Options[2].ID != po2c.String() {
		t.Fatalf("lượt mới = %+v", a)
	}
	if completed, ok := e.completedAt(memberAnID, practiceID); !ok || completed != nil {
		t.Fatalf("bắt đầu lượt phải ghi lần mở đầu, chưa hoàn thành: ok=%v completed=%v", ok, completed)
	}

	if s := e.save(anEmail, a.ID, pq1, po1a); !s.SavedAt.Equal(startedAt) {
		t.Fatalf("savedAt = %v, muốn %v", s.SavedAt, startedAt)
	}
	e.clk.Advance(time.Minute)
	e.save(anEmail, a.ID, pq2, po2c, po2a, po2c)
	e.save(anEmail, a.ID, pq3, po3a)

	raw := e.must(anEmail, http.MethodGet, attemptURL(a.ID), "", http.StatusOK)
	assertOnlyKeys(t, raw, openKeys)
	assertHidesAnswers(t, raw, true)
	open := decode[OpenAttemptDTO](t, raw)
	if open.SavedAt == nil || !open.SavedAt.Equal(e.clk.Now()) ||
		!slices.Equal(open.Questions[1].SelectedOptionIDs, []string{po2a.String(), po2c.String()}) {
		t.Fatalf("lượt mở sau khi lưu = %s", raw)
	}
	assertGolden(t, "attempt-open.json", raw)

	e.clk.Advance(time.Minute)
	submittedAt := e.clk.Now()
	raw = e.submit(anEmail, a.ID)
	assertOnlyKeys(t, raw, practiceKeys)
	res := decode[PracticeResultDTO](t, raw)
	if res.Status != "submitted" || res.Score != 5 || res.MaxScore != 6 || res.Percent != 83 || res.AutoSubmitted ||
		!res.SubmittedAt.Equal(submittedAt) || len(res.Questions) != 3 ||
		!res.Questions[0].Correct || res.Questions[0].PointsAwarded != 2 || !res.Questions[1].Correct ||
		res.Questions[2].Correct || res.Questions[2].PointsAwarded != 0 || res.Questions[2].ExplanationHTML != nil ||
		!res.Questions[2].Options[0].Selected || !res.Questions[2].Options[1].IsCorrect {
		t.Fatalf("kết quả luyện tập = %s", raw)
	}
	if completed, _ := e.completedAt(memberAnID, practiceID); completed == nil || !completed.Equal(submittedAt) {
		t.Fatalf("lần nộp đầu của luyện tập phải hoàn thành học liệu: %v", completed)
	}
	if row := e.attemptRow(a.ID); row.ReviewStatus != nil || row.Passed != nil {
		t.Fatalf("lượt luyện tập không chờ xác nhận và không có đạt/trượt: %+v", row)
	}

	// Nhận xét của giảng viên (dòng mới nhất là hiện hành) kèm ghi chú theo câu.
	review := uuid.Must(uuid.NewV7())
	e.exec(`INSERT INTO quiz_attempt_reviews (id, attempt_id, reviewer_id, comment, created_at) VALUES ($1, $2, $3, $4, $5)`,
		review, a.ID, huongID, "Câu khóa ngoại cần xem lại.", e.clk.Now())
	e.exec(`INSERT INTO quiz_review_notes (review_id, question_id, note) VALUES ($1, $2, 'Khóa ngoại trỏ tới khóa của bảng khác.')`,
		review, pq3)
	raw = e.must(anEmail, http.MethodGet, attemptURL(a.ID), "", http.StatusOK)
	assertOnlyKeys(t, raw, practiceKeys)
	assertGolden(t, "attempt-result-practice.json", raw)

	e.expectError(anEmail, http.MethodPost, submitURL(a.ID), "", http.StatusConflict, "INVALID_TRANSITION", msgAlreadySubmitted)
	e.expectError(anEmail, http.MethodPut, answerURL(a.ID, pq1), pick(po1b), http.StatusConflict, "INVALID_TRANSITION",
		msgAlreadySubmitted)

	// Luyện tập không giới hạn lượt; lượt sau tệ hơn không đổi mốc hoàn thành và không đổi lượt tốt nhất.
	e.clk.Advance(time.Hour)
	second := e.start(anEmail, practiceID)
	if second.No != 2 || second.SavedAt != nil || len(second.Questions[0].SelectedOptionIDs) != 0 {
		t.Fatalf("lượt 2 = %+v", second)
	}
	e.submit(anEmail, second.ID)
	if completed, _ := e.completedAt(memberAnID, practiceID); completed == nil || !completed.Equal(submittedAt) {
		t.Fatalf("lượt sau không được đổi mốc hoàn thành: %v", completed)
	}
	o = e.overview(anEmail, basic01ID, practiceID)
	if o.Used != 2 || len(o.Attempts) != 2 || o.Best == nil || o.Best.AttemptID != a.ID || o.Best.Percent != 83 ||
		o.Best.Passed != nil || !o.CanStart || o.OpenAttemptID != nil || o.Attempts[1].Score != 0 {
		t.Fatalf("màn bắt đầu sau hai lượt = %+v", o)
	}

	if n := e.count(`SELECT count(*) FROM audit_logs`); n != auditBefore {
		t.Fatalf("làm quiz không ghi audit: %d dòng mới", n-auditBefore)
	}
}

func TestAssessmentResultHidesAnswers(t *testing.T) {
	e := newIT(t)

	o := e.overview(anEmail, basic01ID, assessmentID)
	if o.Mode != "assessment" || o.QuestionCount != 3 || o.MaxScore != 10 || o.Limit == nil || *o.Limit != 2 ||
		o.PassPercent == nil || *o.PassPercent != 70 || o.TimeLimitMinutes == nil || *o.TimeLimitMinutes != 10 || !o.CanStart {
		t.Fatalf("màn bắt đầu đánh giá = %+v", o)
	}

	startedAt := e.clk.Now()
	raw := e.must(anEmail, http.MethodPost, attemptsURL(basic01ID, assessmentID), "", http.StatusCreated)
	assertOnlyKeys(t, raw, openKeys)
	assertHidesAnswers(t, raw, true)
	assertGolden(t, "attempts-start.json", raw)
	a := decode[StartDTO](t, raw).Attempt
	// Trộn một lần lúc bắt đầu (reverseShuffler đảo ngược) cả câu lẫn phương án.
	if a.DeadlineAt == nil || !a.DeadlineAt.Equal(startedAt.Add(10*time.Minute)) || a.Limit == nil || *a.Limit != 2 ||
		a.Questions[0].ID != aq3.String() || a.Questions[2].ID != aq1.String() ||
		a.Questions[0].Options[0].ID != ao3c.String() || a.Questions[2].Options[0].ID != ao1b.String() {
		t.Fatalf("lượt đánh giá = %s", raw)
	}

	e.save(anEmail, a.ID, aq1, ao1a)
	e.save(anEmail, a.ID, aq2, ao2a)
	e.save(anEmail, a.ID, aq3, ao3b, ao3a)
	e.clk.Advance(5 * time.Minute)
	raw = e.submit(anEmail, a.ID)
	assertOnlyKeys(t, raw, assessmentKeys)
	assertHidesAnswers(t, raw, false)
	res := decode[AssessmentResultDTO](t, raw)
	// 7/10 đúng ngưỡng 70% là đạt.
	if res.Score != 7 || res.MaxScore != 10 || res.Percent != 70 || res.Passed == nil || !*res.Passed ||
		res.PassPercent == nil || *res.PassPercent != 70 || res.ReviewStatus == nil || *res.ReviewStatus != "pending" ||
		res.Best == nil || res.Best.Percent != 70 || res.AttemptsLeft == nil || *res.AttemptsLeft != 1 || res.Review != nil {
		t.Fatalf("kết quả đánh giá = %s", raw)
	}
	if completed, _ := e.completedAt(memberAnID, assessmentID); completed == nil || !completed.Equal(e.clk.Now()) {
		t.Fatalf("lượt đạt phải hoàn thành học liệu: %v", completed)
	}
	var graded int
	if err := e.dbx.Get(&graded, `SELECT count(*) FROM quiz_attempt_answers
		WHERE attempt_id = $1 AND is_correct IS NOT NULL AND points_awarded IS NOT NULL`, a.ID); err != nil || graded != 3 {
		t.Fatalf("câu đã chấm = %d (%v), muốn 3", graded, err)
	}

	queue := e.quizQueue()
	if len(queue) != 1 || queue[0].Key.ItemID.String() != a.ID || *queue[0].Score != 7 || *queue[0].MaxScore != 10 ||
		queue[0].AutoSubmitted {
		t.Fatalf("hàng chờ xác nhận = %+v", queue)
	}

	// Giảng viên xác nhận, sửa nhận xét một lần (bản mới nhất là hiện hành) và ghi chú câu 2.
	older, newer := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	e.exec(`INSERT INTO quiz_attempt_reviews (id, attempt_id, reviewer_id, comment, created_at) VALUES
		($1, $3, $4, 'Bản nháp nhận xét.', $5), ($2, $3, $4, 'Ổn, xem lại chi phí ghi của chỉ mục.', $6)`,
		older, newer, a.ID, huongID, e.clk.Now(), e.clk.Now().Add(time.Minute))
	e.exec(`INSERT INTO quiz_review_notes (review_id, question_id, note) VALUES
		($1, $3, 'Ghi chú cũ.'), ($2, $3, 'INSERT phải cập nhật mọi chỉ mục.')`, older, newer, aq2)
	e.exec(`UPDATE quiz_attempts SET review_status = 'confirmed' WHERE id = $1`, a.ID)

	raw = e.must(anEmail, http.MethodGet, attemptURL(a.ID), "", http.StatusOK)
	assertOnlyKeys(t, raw, assessmentKeys)
	res = decode[AssessmentResultDTO](t, raw)
	if res.Review == nil || !res.Review.Updated || res.Review.Comment != "Ổn, xem lại chi phí ghi của chỉ mục." ||
		res.Review.ReviewerName != "Lê Thu Hương" || len(res.Review.Notes) != 1 || res.Review.Notes[0].Position != 2 ||
		res.Review.Notes[0].QuestionID != aq2.String() {
		t.Fatalf("kết quả sau nhận xét = %s", raw)
	}
	assertGolden(t, "attempt-result-assessment.json", raw)
	if len(e.quizQueue()) != 0 {
		t.Fatal("lượt đã xác nhận không còn trong hàng chờ")
	}
}

func TestConcurrentStartsCreateOneAttempt(t *testing.T) {
	e := newIT(t)
	cookie := e.as(anEmail)
	const n = 5

	release := e.holdMemberLock(memberAnID)
	results := make([]apiResp, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { results[i] = e.serve(cookie, http.MethodPost, attemptsURL(basic01ID, assessmentID), "") })
	}
	e.waitForLockWaiters(n)
	release()
	wg.Wait()

	created, ids := 0, map[string]bool{}
	for _, r := range results {
		switch r.code {
		case http.StatusCreated:
			created++
		case http.StatusOK:
			if st := decode[StartDTO](t, r.body); !st.Resumed {
				t.Fatalf("200 phải là tiếp tục lượt mở: %s", r.body)
			}
		default:
			t.Fatalf("bắt đầu song song = %d: %s", r.code, r.body)
		}
		ids[decode[StartDTO](t, r.body).Attempt.ID] = true
	}
	if created != 1 || len(ids) != 1 {
		t.Fatalf("tạo %d lượt, %d id khác nhau; muốn đúng 1 lượt", created, len(ids))
	}
	if got := e.count(`SELECT count(*) FROM quiz_attempts WHERE class_member_id = $1`, memberAnID); got != 1 {
		t.Fatalf("số lượt trong DB = %d", got)
	}
	if o := e.overview(anEmail, basic01ID, assessmentID); o.Used != 1 || !o.CanStart || o.OpenAttemptID == nil {
		t.Fatalf("bắt đầu song song không được tiêu nhiều lượt: %+v", o)
	}
}

// Giới hạn 2 lượt cộng lượt cấp thêm; lượt pending cũ nhường chỗ cho lượt mới nộp; hoàn thành ghi khi có lượt đạt
// và giữ nguyên khi lượt sau trượt.
func TestAttemptLimitGrantSupersedeAndCompletion(t *testing.T) {
	e := newIT(t)

	first := e.start(anEmail, assessmentID)
	e.save(anEmail, first.ID, aq1, ao1a) // 3/10: trượt
	e.submit(anEmail, first.ID)
	if _, ok := e.completedAt(memberAnID, assessmentID); !ok {
		t.Fatal("bắt đầu lượt phải ghi lần mở đầu")
	}
	if completed, _ := e.completedAt(memberAnID, assessmentID); completed != nil {
		t.Fatalf("lượt trượt không hoàn thành học liệu: %v", completed)
	}

	e.clk.Advance(time.Hour)
	second := e.start(anEmail, assessmentID)
	e.save(anEmail, second.ID, aq1, ao1a)
	e.save(anEmail, second.ID, aq3, ao3a, ao3b) // 7/10: đạt
	passedAt := e.clk.Now()
	res := decode[AssessmentResultDTO](t, e.submit(anEmail, second.ID))
	if res.AttemptsLeft == nil || *res.AttemptsLeft != 0 || res.Best == nil || res.Best.Percent != 70 {
		t.Fatalf("kết quả lượt 2 = %+v", res)
	}
	if got := e.reviewStatuses(memberAnID, assessmentID); !slices.Equal(got, []string{"superseded", "pending"}) {
		t.Fatalf("review_status = %v", got)
	}
	if completed, _ := e.completedAt(memberAnID, assessmentID); completed == nil || !completed.Equal(passedAt) {
		t.Fatalf("lượt đạt phải hoàn thành học liệu: %v", completed)
	}

	e.expectError(anEmail, http.MethodPost, attemptsURL(basic01ID, assessmentID), "", http.StatusConflict,
		"INVALID_TRANSITION", "Bạn đã dùng hết 2 lượt làm bài.")
	o := e.overview(anEmail, basic01ID, assessmentID)
	if o.CanStart || o.BlockedReason != "Bạn đã dùng hết 2 lượt" || o.Used != 2 || *o.Limit != 2 || o.Granted != 0 ||
		o.Best == nil || o.Best.AttemptID != second.ID || o.Best.Passed == nil || !*o.Best.Passed {
		t.Fatalf("màn bắt đầu khi hết lượt = %+v", o)
	}

	e.exec(`INSERT INTO quiz_attempt_grants (id, class_member_id, lesson_id, extra_attempts, reason, granted_by, granted_at)
		VALUES ($1, $2, $3, 1, 'Mất mạng giữa giờ', $4, $5)`, uuid.Must(uuid.NewV7()), memberAnID, assessmentID, huongID, e.clk.Now())
	o = e.overview(anEmail, basic01ID, assessmentID)
	if !o.CanStart || *o.Limit != 3 || o.Granted != 1 {
		t.Fatalf("màn bắt đầu sau khi cấp thêm lượt = %+v", o)
	}

	e.clk.Advance(time.Hour)
	third := e.start(anEmail, assessmentID)
	if third.No != 3 || *third.Limit != 3 {
		t.Fatalf("lượt 3 = %+v", third)
	}
	res = decode[AssessmentResultDTO](t, e.submit(anEmail, third.ID)) // bỏ trống: trượt
	if res.Passed == nil || *res.Passed || res.Best == nil || res.Best.Percent != 70 || *res.AttemptsLeft != 0 {
		t.Fatalf("kết quả lượt 3 = %+v", res)
	}
	if got := e.reviewStatuses(memberAnID, assessmentID); !slices.Equal(got, []string{"superseded", "superseded", "pending"}) {
		t.Fatalf("review_status = %v", got)
	}
	if completed, _ := e.completedAt(memberAnID, assessmentID); completed == nil || !completed.Equal(passedAt) {
		t.Fatalf("lượt sau trượt không xóa hoàn thành: %v", completed)
	}
	if items := e.quizQueue(); len(items) != 1 || items[0].Key.ItemID.String() != third.ID {
		t.Fatalf("hàng chờ chỉ có lượt pending mới nhất: %+v", items)
	}
	e.expectError(anEmail, http.MethodPost, attemptsURL(basic01ID, assessmentID), "", http.StatusConflict,
		"INVALID_TRANSITION", "Bạn đã dùng hết 3 lượt làm bài.")
}

func TestDeadlineGraceAndSubmitAfterTimeUp(t *testing.T) {
	e := newIT(t)
	a := e.start(anEmail, assessmentID)
	deadline := *a.DeadlineAt
	e.save(anEmail, a.ID, aq1, ao1a)

	// Đúng biên ân hạn 30 giây sau hạn nộp vẫn lưu được; quá biên một micro giây thì không.
	e.clk.Set(deadline.Add(SubmitGrace))
	e.save(anEmail, a.ID, aq3, ao3a, ao3b)
	e.clk.Set(deadline.Add(SubmitGrace + time.Microsecond))
	e.expectError(anEmail, http.MethodPut, answerURL(a.ID, aq2), pick(ao2b), http.StatusConflict, "INVALID_TRANSITION",
		msgTimeUpNotSaved)

	e.expectError(anEmail, http.MethodPost, submitURL(a.ID), "", http.StatusConflict, "INVALID_TRANSITION", msgTimeUpAuto)
	row := e.attemptRow(a.ID)
	if row.SubmittedAt == nil || !row.SubmittedAt.Equal(deadline) || !row.AutoSubmitted || row.Score == nil || *row.Score != 7 ||
		row.Passed == nil || !*row.Passed || row.ReviewStatus == nil || *row.ReviewStatus != "pending" {
		t.Fatalf("lượt tự nộp = %+v", row)
	}
	if completed, _ := e.completedAt(memberAnID, assessmentID); completed == nil || !completed.Equal(deadline) {
		t.Fatalf("tự nộp đạt phải hoàn thành tại hạn nộp: %v", completed)
	}

	res := decode[AssessmentResultDTO](t, e.must(anEmail, http.MethodGet, attemptURL(a.ID), "", http.StatusOK))
	if !res.AutoSubmitted || !res.SubmittedAt.Equal(deadline) || res.Score != 7 {
		t.Fatalf("kết quả tự nộp = %+v", res)
	}
	e.expectError(anEmail, http.MethodPost, submitURL(a.ID), "", http.StatusConflict, "INVALID_TRANSITION", msgAlreadySubmitted)
	if items := e.quizQueue(); len(items) != 1 || !items[0].AutoSubmitted {
		t.Fatalf("lượt tự nộp vào hàng chờ với cờ tự nộp: %+v", items)
	}
}

func TestSubmitWithinGraceIsManual(t *testing.T) {
	e := newIT(t)
	a := e.start(anEmail, assessmentID)
	e.save(anEmail, a.ID, aq1, ao1a)
	e.clk.Set(a.DeadlineAt.Add(SubmitGrace))
	res := decode[AssessmentResultDTO](t, e.submit(anEmail, a.ID))
	if res.AutoSubmitted || !res.SubmittedAt.Equal(e.clk.Now()) || res.Score != 3 {
		t.Fatalf("nộp trong ân hạn = %+v", res)
	}
}

func TestExpiredAttemptIsAutoSubmittedOnRead(t *testing.T) {
	expire := func(e *itEnv) OpenAttemptDTO {
		a := e.start(anEmail, assessmentID)
		e.save(anEmail, a.ID, aq3, ao3a, ao3b) // 4/10
		e.clk.Set(a.DeadlineAt.Add(time.Hour))
		return a
	}
	assertAuto := func(e *itEnv, a OpenAttemptDTO) {
		e.t.Helper()
		row := e.attemptRow(a.ID)
		if row.SubmittedAt == nil || !row.SubmittedAt.Equal(*a.DeadlineAt) || !row.AutoSubmitted || *row.Score != 4 ||
			*row.Passed {
			e.t.Fatalf("lượt quá giờ = %+v", row)
		}
	}

	t.Run("đọc lượt", func(t *testing.T) {
		e := newIT(t)
		a := expire(e)
		res := decode[AssessmentResultDTO](t, e.must(anEmail, http.MethodGet, attemptURL(a.ID), "", http.StatusOK))
		if res.Status != "submitted" || !res.AutoSubmitted || res.Score != 4 {
			t.Fatalf("kết quả = %+v", res)
		}
		assertAuto(e, a)
	})
	t.Run("đọc màn bắt đầu", func(t *testing.T) {
		e := newIT(t)
		a := expire(e)
		o := e.overview(anEmail, basic01ID, assessmentID)
		if o.OpenAttemptID != nil || o.Used != 1 || len(o.Attempts) != 1 || !o.Attempts[0].AutoSubmitted || !o.CanStart {
			t.Fatalf("màn bắt đầu = %+v", o)
		}
		assertAuto(e, a)
	})
	t.Run("bắt đầu lượt mới", func(t *testing.T) {
		e := newIT(t)
		a := expire(e)
		next := e.start(anEmail, assessmentID)
		if next.No != 2 || next.ID == a.ID {
			t.Fatalf("lượt mới = %+v", next)
		}
		assertAuto(e, a)
	})
}

func TestCloseExpired(t *testing.T) {
	e := newIT(t)
	an := e.start(anEmail, assessmentID)
	e.save(anEmail, an.ID, aq1, ao1a)
	khang := e.start(khangEmail, assessmentID)
	practice := e.start(anEmail, practiceID) // không giới hạn thời gian: không bao giờ tự nộp
	deadline := *an.DeadlineAt

	// Lượt hỏng có hạn sớm nhất nên được xử lý đầu tiên; lượt của thành viên đã rời và của lớp đã kết thúc giữ nguyên.
	broken := e.insertOpenAttempt(memberKhangID, brokenID, deadline.Add(-time.Hour))
	dropped := e.insertOpenAttempt(memberThaoID, assessmentID, deadline.Add(-time.Hour))
	ended := e.insertOpenAttempt(memberAn00ID, assessmentID, deadline.Add(-time.Hour))

	e.clk.Set(deadline.Add(SubmitGrace))
	if closed, err := e.svc.CloseExpired(e.ctx(), e.clk.Now()); closed != 0 || err == nil || !strings.Contains(err.Error(), broken) {
		t.Fatalf("đúng biên ân hạn chỉ lượt hỏng đã quá hạn: closed=%d err=%v", closed, err)
	}

	e.clk.Set(deadline.Add(SubmitGrace + time.Second))
	closed, err := e.svc.CloseExpired(e.ctx(), e.clk.Now())
	if closed != 2 || err == nil || !strings.Contains(err.Error(), broken) {
		t.Fatalf("CloseExpired = %d, %v; muốn 2 lượt và lỗi của lượt hỏng", closed, err)
	}
	for _, id := range []string{an.ID, khang.ID} {
		if row := e.attemptRow(id); row.SubmittedAt == nil || !row.SubmittedAt.Equal(deadline) || !row.AutoSubmitted {
			t.Fatalf("lượt %s = %+v", id, row)
		}
	}
	if *e.attemptRow(an.ID).Score != 3 || *e.attemptRow(khang.ID).Score != 0 {
		t.Fatal("tự nộp phải chấm các câu đã lưu")
	}
	for _, id := range []string{broken, dropped, ended, practice.ID} {
		if row := e.attemptRow(id); row.SubmittedAt != nil {
			t.Fatalf("lượt %s không được tự nộp: %+v", id, row)
		}
	}
	if got, want := queueIDs(e.quizQueue()), sorted(an.ID, khang.ID); !slices.Equal(got, want) {
		t.Fatalf("hàng chờ = %v, muốn %v", got, want)
	}

	// Worker gọi Cleanup: lượt hỏng vẫn báo lỗi, lượt đã đóng không bị chấm lại.
	if err := e.svc.Cleanup(e.ctx(), e.clk.Now()); err == nil {
		t.Fatal("Cleanup phải báo lỗi của lượt hỏng")
	}
	e.exec(`DELETE FROM quiz_attempts WHERE id = $1`, broken)
	if err := e.svc.Cleanup(e.ctx(), e.clk.Now()); err != nil {
		t.Fatalf("Cleanup = %v", err)
	}
	if got := e.count(`SELECT count(*) FROM quiz_attempts WHERE submitted_at IS NOT NULL`); got != 2 {
		t.Fatalf("số lượt đã nộp = %d", got)
	}
}

// Học viên bấm nộp đúng lúc worker tự nộp: chỉ một bên chấm, bên kia thấy lượt đã nộp.
func TestSubmitRacesCloseExpired(t *testing.T) {
	e := newIT(t)
	cookie := e.as(anEmail)
	a := e.start(anEmail, assessmentID)
	e.save(anEmail, a.ID, aq1, ao1a)
	e.save(anEmail, a.ID, aq3, ao3a, ao3b)
	e.clk.Set(a.DeadlineAt.Add(time.Minute))

	release := e.holdMemberLock(memberAnID)
	var (
		wg       sync.WaitGroup
		submit   apiResp
		closed   int
		closeErr error
	)
	wg.Go(func() { submit = e.serve(cookie, http.MethodPost, submitURL(a.ID), "") })
	wg.Go(func() { closed, closeErr = e.svc.CloseExpired(e.ctx(), e.clk.Now()) })
	e.waitForLockWaiters(2)
	release()
	wg.Wait()

	if closeErr != nil {
		t.Fatalf("CloseExpired: %v", closeErr)
	}
	env := errorOf(t, submit)
	submitClosed := 0
	switch {
	case submit.code == http.StatusConflict && env.Error.Message == msgTimeUpAuto:
		submitClosed = 1
	case submit.code == http.StatusConflict && env.Error.Message == msgAlreadySubmitted:
	default:
		t.Fatalf("nộp = %d: %s", submit.code, submit.body)
	}
	if closed+submitClosed != 1 {
		t.Fatalf("lượt được đóng %d lần (worker %d, nộp %d)", closed+submitClosed, closed, submitClosed)
	}
	row := e.attemptRow(a.ID)
	if row.SubmittedAt == nil || !row.SubmittedAt.Equal(*a.DeadlineAt) || !row.AutoSubmitted || *row.Score != 7 {
		t.Fatalf("lượt = %+v", row)
	}
}

// Lưu câu đúng lúc nộp: câu trả lời đã chấm luôn khớp phương án đã lưu, bất kể bên nào lấy khóa trước.
func TestSaveRacesSubmit(t *testing.T) {
	e := newIT(t)
	cookie := e.as(anEmail)
	a := e.start(anEmail, practiceID)
	e.save(anEmail, a.ID, pq1, po1b)

	release := e.holdMemberLock(memberAnID)
	var (
		wg           sync.WaitGroup
		save, submit apiResp
	)
	wg.Go(func() { save = e.serve(cookie, http.MethodPut, answerURL(a.ID, pq1), pick(po1a)) })
	wg.Go(func() { submit = e.serve(cookie, http.MethodPost, submitURL(a.ID), "") })
	e.waitForLockWaiters(2)
	release()
	wg.Wait()

	if submit.code != http.StatusOK {
		t.Fatalf("nộp = %d: %s", submit.code, submit.body)
	}
	var ans struct {
		Selected  string `db:"selected"`
		IsCorrect bool   `db:"is_correct"`
	}
	if err := e.dbx.Get(&ans, `SELECT selected_option_ids::text AS selected, is_correct FROM quiz_attempt_answers
		WHERE attempt_id = $1 AND question_id = $2`, a.ID, pq1); err != nil {
		t.Fatal(err)
	}
	score := *e.attemptRow(a.ID).Score
	switch save.code {
	case http.StatusOK: // lưu trước nộp
		if ans.Selected != "{"+po1a.String()+"}" || !ans.IsCorrect || score != 2 {
			t.Fatalf("lưu trước: %+v, điểm %d", ans, score)
		}
	case http.StatusConflict: // nộp trước lưu
		if env := errorOf(t, save); env.Error.Message != msgAlreadySubmitted || ans.Selected != "{"+po1b.String()+"}" ||
			ans.IsCorrect || score != 0 {
			t.Fatalf("nộp trước: %s, %+v, điểm %d", save.body, ans, score)
		}
	default:
		t.Fatalf("lưu = %d: %s", save.code, save.body)
	}
}

func TestResumeKeepsOrderAndAnswers(t *testing.T) {
	e := newIT(t)
	a := e.start(anEmail, assessmentID)
	e.clk.Advance(time.Minute)
	e.save(anEmail, a.ID, aq3, ao3b)

	e.clk.Advance(time.Minute)
	st := decode[StartDTO](t, e.must(anEmail, http.MethodPost, attemptsURL(basic01ID, assessmentID), "", http.StatusOK))
	if !st.Resumed || st.Attempt.ID != a.ID || !st.Attempt.DeadlineAt.Equal(*a.DeadlineAt) || st.Attempt.SavedAt == nil ||
		!st.Attempt.SavedAt.Equal(a.ServerNow.Add(time.Minute)) {
		t.Fatalf("tiếp tục = %+v", st)
	}
	order := func(v OpenAttemptDTO) []string {
		var out []string
		for _, q := range v.Questions {
			out = append(out, q.ID)
			for _, o := range q.Options {
				out = append(out, o.ID)
			}
		}
		return out
	}
	if !slices.Equal(order(st.Attempt), order(a)) {
		t.Fatalf("thứ tự đổi khi tiếp tục: %v → %v", order(a), order(st.Attempt))
	}
	if !slices.Equal(st.Attempt.Questions[0].SelectedOptionIDs, []string{ao3b.String()}) {
		t.Fatalf("câu đã lưu = %v", st.Attempt.Questions[0].SelectedOptionIDs)
	}
	var stored []string
	if err := e.dbx.Select(&stored, `SELECT unnest(question_order)::text FROM quiz_attempts WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stored, []string{aq3.String(), aq2.String(), aq1.String()}) {
		t.Fatalf("thứ tự câu đã chốt = %v", stored)
	}
	if o := e.overview(anEmail, basic01ID, assessmentID); o.OpenAttemptID == nil || *o.OpenAttemptID != a.ID || !o.CanStart {
		t.Fatalf("màn bắt đầu khi có lượt mở = %+v", o)
	}
}

func TestDroppedMemberCannotUseAttempt(t *testing.T) {
	e := newIT(t)
	a := e.start(anEmail, assessmentID)
	e.save(anEmail, a.ID, aq1, ao1a)
	e.exec(`UPDATE class_members SET status = 'dropped', dropped_at = now() WHERE id = $1`, memberAnID)

	e.expectError(anEmail, http.MethodPut, answerURL(a.ID, aq1), pick(ao1b), http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(anEmail, http.MethodPost, submitURL(a.ID), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(anEmail, http.MethodGet, attemptURL(a.ID), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(anEmail, http.MethodGet, attemptsURL(basic01ID, assessmentID), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(anEmail, http.MethodPost, attemptsURL(basic01ID, assessmentID), "", http.StatusNotFound, "NOT_FOUND", "")
	if row := e.attemptRow(a.ID); row.SubmittedAt != nil {
		t.Fatalf("lượt của thành viên đã rời giữ nguyên: %+v", row)
	}
	if items := e.quizQueue(); len(items) != 0 {
		t.Fatalf("lượt của thành viên đã rời không vào hàng chờ: %+v", items)
	}
}

func TestEndedClassIsReadOnly(t *testing.T) {
	e := newIT(t)
	a := e.start(anEmail, assessmentID)
	e.save(anEmail, a.ID, aq1, ao1a)
	e.exec(`UPDATE classes SET status = 'ended' WHERE id = $1`, basic01ID)
	e.clk.Set(a.DeadlineAt.Add(time.Hour))

	e.expectError(anEmail, http.MethodPut, answerURL(a.ID, aq1), pick(ao1b), http.StatusConflict, "INVALID_TRANSITION", msgClassEnded)
	e.expectError(anEmail, http.MethodPost, submitURL(a.ID), "", http.StatusConflict, "INVALID_TRANSITION", msgClassEnded)
	e.expectError(anEmail, http.MethodPost, attemptsURL(basic01ID, assessmentID), "", http.StatusConflict,
		"INVALID_TRANSITION", msgClassEnded)

	// Đọc vẫn được nhưng không tự nộp: lớp đã kết thúc không ghi gì nữa.
	open := decode[OpenAttemptDTO](t, e.must(anEmail, http.MethodGet, attemptURL(a.ID), "", http.StatusOK))
	if open.Status != "open" || open.ID != a.ID {
		t.Fatalf("lượt mở của lớp đã kết thúc = %+v", open)
	}
	o := e.overview(anEmail, basic01ID, assessmentID)
	if o.CanStart || o.BlockedReason != "Lớp đã kết thúc" || o.OpenAttemptID == nil {
		t.Fatalf("màn bắt đầu lớp đã kết thúc = %+v", o)
	}
	if row := e.attemptRow(a.ID); row.SubmittedAt != nil {
		t.Fatalf("lượt của lớp đã kết thúc không được tự nộp: %+v", row)
	}
	if closed, err := e.svc.CloseExpired(e.ctx(), e.clk.Now()); closed != 0 || err != nil {
		t.Fatalf("CloseExpired = %d, %v", closed, err)
	}
	if items := e.quizQueue(); len(items) != 0 {
		t.Fatalf("lượt của lớp đã kết thúc không vào hàng chờ: %+v", items)
	}
}

func TestAccessScoping(t *testing.T) {
	e := newIT(t)
	a := e.start(anEmail, practiceID)

	// Linh học go01: không thấy quiz của basic01, quiz không thuộc khóa GO, không chạm được lượt của An.
	e.expectError(linhEmail, http.MethodGet, attemptsURL(basic01ID, practiceID), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(linhEmail, http.MethodPost, attemptsURL(go01ID, practiceID), "", http.StatusNotFound, "NOT_FOUND",
		"Bài quiz không thuộc lớp này.")
	e.expectError(linhEmail, http.MethodGet, attemptURL(a.ID), "", http.StatusNotFound, "NOT_FOUND", "Không tìm thấy lượt làm bài.")
	e.expectError(linhEmail, http.MethodPut, answerURL(a.ID, pq1), pick(po1a), http.StatusNotFound, "NOT_FOUND",
		"Không tìm thấy lượt làm bài.")
	e.expectError(linhEmail, http.MethodPost, submitURL(a.ID), "", http.StatusNotFound, "NOT_FOUND", "Không tìm thấy lượt làm bài.")
	e.expectError(thaoEmail, http.MethodGet, attemptsURL(basic01ID, practiceID), "", http.StatusNotFound, "NOT_FOUND", "")

	e.expectError(anEmail, http.MethodGet, attemptsURL(basic01ID, lessonTableID), "", http.StatusNotFound, "NOT_FOUND",
		"Bài quiz không thuộc lớp này.")
	e.expectError(anEmail, http.MethodGet, attemptsURL(uuid.Must(uuid.NewV7()), practiceID), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(anEmail, http.MethodGet, attemptURL(uuid.Must(uuid.NewV7()).String()), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(anEmail, http.MethodGet, attemptURL("khong-phai-uuid"), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(huongEmail, http.MethodGet, attemptsURL(basic01ID, practiceID), "", http.StatusForbidden, "FORBIDDEN", "")

	e.expectError(anEmail, http.MethodPost, attemptsURL(basic03ID, practiceID), "", http.StatusConflict, "INVALID_TRANSITION",
		"Lớp chưa bắt đầu.")
	e.expectError(anEmail, http.MethodPost, attemptsURL(basic00ID, practiceID), "", http.StatusConflict, "INVALID_TRANSITION",
		msgClassEnded)
	if o := e.overview(anEmail, basic03ID, practiceID); o.CanStart || o.BlockedReason != "Lớp chưa bắt đầu" {
		t.Fatalf("màn bắt đầu lớp nháp = %+v", o)
	}
	if o := e.overview(anEmail, basic00ID, practiceID); o.CanStart || o.BlockedReason != "Lớp đã kết thúc" || o.Used != 0 {
		t.Fatalf("màn bắt đầu lớp đã kết thúc = %+v", o)
	}
	if got := e.count(`SELECT count(*) FROM quiz_attempts`); got != 1 {
		t.Fatalf("số lượt = %d", got)
	}
}

func TestSaveAnswerValidation(t *testing.T) {
	e := newIT(t)
	a := e.start(anEmail, practiceID)

	const required = "Gửi danh sách phương án đã chọn; danh sách rỗng là bỏ chọn."
	const notInQuestion = "Phương án không thuộc câu hỏi này."
	cases := []struct {
		name, body string
		question   uuid.UUID
		status     int
		code, msg  string
	}{
		{"thiếu optionIds", `{}`, pq1, http.StatusUnprocessableEntity, "VALIDATION_FAILED", required},
		{"optionIds null", `{"optionIds":null}`, pq1, http.StatusUnprocessableEntity, "VALIDATION_FAILED", required},
		{"id không hợp lệ", `{"optionIds":["x"]}`, pq1, http.StatusUnprocessableEntity, "VALIDATION_FAILED", notInQuestion},
		{"phương án của câu khác", pick(po2a), pq1, http.StatusUnprocessableEntity, "VALIDATION_FAILED", notInQuestion},
		{"câu một đáp án chọn hai", pick(po1a, po1b), pq1, http.StatusUnprocessableEntity, "VALIDATION_FAILED",
			"Câu một đáp án chỉ chọn được một phương án."},
		{"câu không thuộc lượt", pick(ao1a), aq1, http.StatusNotFound, "NOT_FOUND", "Câu hỏi không thuộc lượt làm bài này."},
		{"JSON hỏng", `{"optionIds":`, pq1, http.StatusBadRequest, "VALIDATION_FAILED", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e.expectError(anEmail, http.MethodPut, answerURL(a.ID, tc.question), tc.body, tc.status, tc.code, tc.msg)
		})
	}
	if got := e.count(`SELECT count(*) FROM quiz_attempt_answers`); got != 0 {
		t.Fatalf("yêu cầu lỗi không được lưu gì: %d", got)
	}

	e.save(anEmail, a.ID, pq2, po2c, po2a, po2c)
	e.save(anEmail, a.ID, pq1, po1a)
	e.save(anEmail, a.ID, pq1) // bỏ chọn
	open := decode[OpenAttemptDTO](t, e.must(anEmail, http.MethodGet, attemptURL(a.ID), "", http.StatusOK))
	if len(open.Questions[0].SelectedOptionIDs) != 0 || open.Questions[0].SelectedOptionIDs == nil ||
		!slices.Equal(open.Questions[1].SelectedOptionIDs, []string{po2a.String(), po2c.String()}) {
		t.Fatalf("phương án đã lưu = %+v", open.Questions)
	}
}

// ---- golden ----

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizeGolden thay thời điểm (khóa kết thúc bằng "At" và serverNow) bằng mốc cố định và id sinh lúc chạy (không
// phải id cố định 01990000-…) bằng id giả theo thứ tự xuất hiện, để golden ổn định giữa các lần chạy.
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
			if strings.HasSuffix(key, "At") || key == "serverNow" {
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
	out, err := json.MarshalIndent(walk("", v), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(out, '\n')
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
