//go:build integration

package grading_test

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
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/features/grading"
	"lms/api/internal/features/identity"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/middleware"
	"lms/api/internal/platform/testdb"
	"lms/api/internal/seed"
)

var update = flag.Bool("update", false, "ghi lại golden JSON trong testdata/")

// Mật khẩu và khóa chỉ dùng cho DB test.
const (
	itSeedPassword = "Seed-Password-Test-1"
	itOutboxKey    = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
)

// Tài khoản theo seed.js. Hàng chờ của seed: basic01 (Hương) có Bích ds-hw 4,5 ngày, Hà db-test lượt 2 khoảng 84 giờ,
// Cường re-test khoảng 60 giờ, Bích ds-test khoảng 43 giờ, Phong ds-hw 1 ngày, Cường go-hw 0,5 ngày, An go-quiz khoảng
// 7 giờ; basic02 (Bảo) có Phong ds-hw 3,5 ngày, Linh db-test khoảng 36 giờ, Linh ds-hw 0,5 ngày, Phong ds-test khoảng
// 8 giờ.
const (
	adminEmail = "quan.tran@goup.vn"
	huongEmail = "huong.le@goup.vn" // giảng viên basic01, basic03 (nháp)
	baoEmail   = "bao.pham@goup.vn" // giảng viên basic02
	anEmail    = "an.nguyen@gmail.com"
	bichEmail  = "bich.tran@gmail.com"
	phongEmail = "phong.dang@gmail.com"
	haEmail    = "ha.hoang@gmail.com"
	queueURL   = "/api/v1/grading/queue"
	recentURL  = "/api/v1/grading/recent"
)

// Lượt quiz chèn bằng SQL (bảng có từ trước khi có luồng làm quiz); id cố định để golden ổn định.
var (
	attemptPendingID   = uuid.MustParse("01990000-0000-7000-8000-000000000701") // An web-test lượt 2, chờ xác nhận
	attemptConfirmedID = uuid.MustParse("01990000-0000-7000-8000-000000000702") // An web-test lượt 1, đã xác nhận
	attemptPracticeID  = uuid.MustParse("01990000-0000-7000-8000-000000000703") // An web-practice, luyện tập đã nhận xét
)

type itEnv struct {
	t      *testing.T
	dbx    *sqlx.DB
	clk    *clock.Fake
	ident  *identity.Service
	svc    *grading.Service
	engine *gin.Engine
}

func newIT(t *testing.T) *itEnv {
	t.Helper()
	return newITAt(t, time.Now().UTC().Truncate(time.Second))
}

// newITAt seed DB theo mốc now (dữ liệu seed tính tương đối so với now) và dựng env với đồng hồ giả dừng ở now.
func newITAt(t *testing.T, now time.Time) *itEnv {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)
	clk := &clock.Fake{T: now}
	_, err = seed.Run(context.Background(), dbx, config.Config{
		AppEnv: config.EnvDev, PublicBaseURL: "http://localhost:5173", TempPasswordTTL: 72 * time.Hour,
		SeedPassword: itSeedPassword, OutboxSecretKey: itOutboxKey, Location: loc,
	}, clk, seed.Options{})
	require.NoError(t, err)

	e := &itEnv{t: t, dbx: dbx, clk: clk}
	e.ident = identity.NewService(identity.ServiceDeps{
		DB: dbx, Tx: db.TxRunner{DB: dbx},
		Users: identity.PGUserRepo{}, Sessions: identity.PGSessionRepo{}, Attempts: identity.PGLoginAttemptRepo{},
		Resets: identity.PGResetTokenRepo{}, Clock: clk, Audit: audit.PG{Clock: clk},
		Cfg: config.Config{
			SessionTTL: 12 * time.Hour, TempPasswordTTL: 72 * time.Hour, ResetTokenTTL: 30 * time.Minute,
			PasswordMinLength: 8, LoginMaxFailures: 5, LoginLockWindow: 15 * time.Minute,
			PublicBaseURL: "http://localhost:5173", AppEnv: config.EnvE2E,
		},
	})
	e.svc = grading.NewService(grading.Deps{Tx: readTx{dbx}, Repo: grading.PGRepo{}, Clock: clk})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recover(slog.New(slog.DiscardHandler)), middleware.SecurityHeaders())
	authed := engine.Group("/api/v1", identity.NewMiddleware(e.ident, false).SessionAuth())
	grading.NewHandler(e.svc, func(c *gin.Context) (uuid.UUID, domain.Role, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return uuid.Nil, "", false
		}
		return u.ID(), u.Role(), true
	}).Register(authed, identity.RequireRole(domain.RoleAdmin, domain.RoleTeacher))
	e.engine = engine
	return e
}

// readTx là transaction chỉ đọc REPEATABLE READ như reports.ReadTx mà production truyền vào (grading không import
// reports).
type readTx struct{ dbx *sqlx.DB }

func (r readTx) Transact(ctx context.Context, fn func(tx db.Executor) error) error {
	return db.TransactWithOptions(ctx, r.dbx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, fn)
}

// seedQuiz chèn lượt quiz bài tập của An ở basic01 trên học liệu seed chưa có lượt nào của An: lượt 2 chờ xác nhận (nộp 50 giờ trước, tự nộp khi hết giờ, 7/10),
// lượt 1 đã xác nhận (Admin nhận xét 3 giờ trước, Hương sửa 2 giờ trước) và một lượt luyện tập đã nhận xét 1 giờ trước
// (không phải việc của hàng chờ nên không vào danh sách gần đây).
func (e *itEnv) seedQuiz() {
	e.t.Helper()
	now := e.clk.Now()
	member := e.memberID("basic01", anEmail)
	test, practice := e.lessonID("web-test"), e.lessonID("web-practice")
	adminID, huongID := e.userID(adminEmail), e.userID(huongEmail)
	ins := `INSERT INTO quiz_attempts (id, class_member_id, lesson_id, attempt_no, question_order, option_order, started_at,
		submitted_at, auto_submitted, score, max_score, passed, review_status)
		VALUES ($1, $2, $3, $4, '{}', '{}', $5, $6, $7, $8, 10, $9, $10)`
	e.exec(ins, attemptConfirmedID, member, test, 1, now.Add(-60*time.Hour), now.Add(-59*time.Hour), false, 5, false, "confirmed")
	e.exec(ins, attemptPendingID, member, test, 2, now.Add(-51*time.Hour), now.Add(-50*time.Hour), true, 7, true, "pending")
	e.exec(ins, attemptPracticeID, member, practice, 1, now.Add(-3*time.Hour), now.Add(-2*time.Hour), false, 9, true, "confirmed")
	rev := `INSERT INTO quiz_attempt_reviews (id, attempt_id, reviewer_id, comment, created_at) VALUES ($1, $2, $3, $4, $5)`
	e.exec(rev, uuid.MustParse("01990000-0000-7000-8000-000000000711"), attemptConfirmedID, adminID, "Ôn lại phần hash.", now.Add(-3*time.Hour))
	e.exec(rev, uuid.MustParse("01990000-0000-7000-8000-000000000712"), attemptConfirmedID, huongID, "Ôn lại phần hash và queue.", now.Add(-2*time.Hour))
	e.exec(rev, uuid.MustParse("01990000-0000-7000-8000-000000000713"), attemptPracticeID, huongID, "Tốt.", now.Add(-time.Hour))
}

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

// lessonID là học liệu key của phiên bản khóa học basic01 dùng (basic01 và basic02 chung phiên bản).
func (e *itEnv) lessonID(key string) uuid.UUID {
	return e.id(`SELECT l.id FROM classes c
		JOIN course_version_stages cvs ON cvs.course_version_id = c.course_version_id
		JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		WHERE c.code = 'basic01' AND l.lesson_key = $1`, key)
}

// pendingSubmission là bản nộp pending của email ở lớp classCode cho học liệu key.
func (e *itEnv) pendingSubmission(classCode, email, key string) uuid.UUID {
	return e.id(`SELECT s.id FROM submissions s WHERE s.class_member_id = $1 AND s.lesson_id = $2 AND s.status = 'pending'`,
		e.memberID(classCode, email), e.lessonID(key))
}

func (e *itEnv) login(email string) string {
	e.t.Helper()
	res, err := e.ident.Login(context.Background(), email, itSeedPassword, "127.0.0.1", "it")
	require.NoError(e.t, err, "đăng nhập %s", email)
	return res.Token.Reveal()
}

func (e *itEnv) actor(email string) grading.Actor {
	var role domain.Role
	require.NoError(e.t, e.dbx.Get(&role, `SELECT role FROM users WHERE email = $1`, email))
	return grading.Actor{ID: e.userID(email), Role: role}
}

type apiResp struct {
	code   int
	body   []byte
	header http.Header
}

func (e *itEnv) get(cookie, path string) apiResp {
	e.t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: identity.CookieName(false), Value: cookie})
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return apiResp{code: w.Code, body: w.Body.Bytes(), header: w.Header()}
}

// must gọi API, bắt buộc 200 và header không cache.
func (e *itEnv) must(cookie, path string) []byte {
	e.t.Helper()
	r := e.get(cookie, path)
	require.Equal(e.t, http.StatusOK, r.code, "%s: %s", path, r.body)
	assert.Equal(e.t, "private, no-store", r.header.Get("Cache-Control"), path)
	return r.body
}

func (e *itEnv) expectError(cookie, path string, status int, code string) {
	e.t.Helper()
	r := e.get(cookie, path)
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(e.t, json.Unmarshal(r.body, &env), "%s: %s", path, r.body)
	require.Equal(e.t, status, r.code, "%s: %s", path, r.body)
	assert.Equal(e.t, code, env.Error.Code, path)
}

func decode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(raw, &v), "%s", raw)
	return v
}

func (e *itEnv) queue(cookie, query string) grading.QueueDTO {
	e.t.Helper()
	path := queueURL
	if query != "" {
		path += "?" + query
	}
	return decode[grading.QueueDTO](e.t, e.must(cookie, path))
}

// rows là tóm tắt một mục để so thứ tự: loại, học viên, lớp, học liệu.
func rows(items []grading.QueueItemDTO) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = fmt.Sprintf("%s %s %s %s", it.Kind, it.Student.Name, it.Class.Code, it.Lesson.Title)
	}
	return out
}

func classCodes(cs []grading.ClassDTO) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Code
	}
	return out
}

func itemIDs(items []grading.QueueItemDTO) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ItemID
	}
	return out
}

const (
	dsHW    = "Bài tập: Cài đặt Stack và Queue"
	goHW    = "Bài tập: Đọc CSV và thống kê"
	dsTest  = "Kiểm tra cuối chặng Data structure"
	dbTest  = "Kiểm tra cuối chặng Database"
	goTest  = "Kiểm tra cuối chặng Go"
	reTest  = "Kiểm tra cuối chặng ReactJS"
	webTest = "Kiểm tra cuối chặng HTML CSS JS"
)

func TestQueueScopeUnifiedOrderAndCounts(t *testing.T) {
	e := newIT(t)
	e.seedQuiz()

	// Giảng viên basic01: homework và quiz chung một thứ tự, cũ nhất trước; không có mục của basic02.
	raw := e.must(e.login(huongEmail), queueURL)
	q := decode[grading.QueueDTO](t, raw)
	assert.Equal(t, []string{
		"homework Trần Thị Bích basic01 " + dsHW,
		"quiz Hoàng Thu Hà basic01 " + dbTest,
		"quiz Lê Văn Cường basic01 " + reTest,
		"quiz Nguyễn Hoàng An basic01 " + webTest,
		"quiz Trần Thị Bích basic01 " + dsTest,
		"homework Đặng Hải Phong basic01 " + dsHW,
		"homework Lê Văn Cường basic01 " + goHW,
		"quiz Nguyễn Hoàng An basic01 " + goTest,
	}, rows(q.Items))
	assert.Equal(t, 8, q.Total)
	assert.Equal(t, 5, q.QuizPending)
	assert.Equal(t, 2, q.OverdueCount)
	assert.Equal(t, []string{"basic01"}, classCodes(q.Filters.Classes), "lớp nháp basic03 không có trong ô lọc")
	for i := 1; i < len(q.Items); i++ {
		assert.False(t, q.Items[i].SubmittedAt.Before(q.Items[i-1].SubmittedAt), "cũ nhất trước")
	}

	bich, ha, quiz := q.Items[0], q.Items[1], q.Items[3]
	assert.Equal(t, 108, bich.WaitedHours)
	assert.True(t, bich.Overdue)
	assert.Nil(t, bich.ScorePercent)
	assert.True(t, ha.Overdue, "lượt quiz chờ quá 72 giờ cũng quá hạn")
	assert.Equal(t, attemptPendingID.String(), quiz.ItemID)
	assert.Equal(t, 2, quiz.No)
	assert.True(t, quiz.AutoSubmitted)
	require.NotNil(t, quiz.ScorePercent)
	assert.Equal(t, 70, *quiz.ScorePercent)
	assert.Equal(t, 50, quiz.WaitedHours)
	assert.False(t, quiz.Overdue)
	assert.Equal(t, "HTML CSS JS", quiz.Stage.Name)
	// Mục quiz chỉ có % tổng: không có đáp án, điểm từng câu hay điểm thô.
	assert.NotContains(t, string(raw), "answers")
	assert.NotContains(t, string(raw), "points")
	assert.NotContains(t, string(raw), `"score"`)
	assertGolden(t, "queue-teacher.json", raw)

	// Giảng viên basic02 chỉ thấy lớp mình.
	q = e.queue(e.login(baoEmail), "")
	assert.Equal(t, []string{
		"homework Đặng Hải Phong basic02 " + dsHW,
		"quiz Đỗ Ngọc Linh basic02 " + dbTest,
		"homework Đỗ Ngọc Linh basic02 " + dsHW,
		"quiz Đặng Hải Phong basic02 " + dsTest,
	}, rows(q.Items))
	assert.Equal(t, []string{"basic02"}, classCodes(q.Filters.Classes))
	assert.Equal(t, 1, q.OverdueCount)
	assert.Equal(t, 2, q.QuizPending)

	// Admin thấy mọi lớp.
	raw = e.must(e.login(adminEmail), queueURL)
	q = decode[grading.QueueDTO](t, raw)
	assert.Len(t, q.Items, 12)
	assert.Equal(t, 12, q.Total)
	assert.Equal(t, 3, q.OverdueCount)
	assert.Equal(t, []string{"basic01", "basic02"}, classCodes(q.Filters.Classes))
	assertGolden(t, "queue-admin.json", raw)
}

func TestQueueFilters(t *testing.T) {
	e := newIT(t)
	e.seedQuiz()
	huong := e.login(huongEmail)
	dsHWID := e.lessonID("ds-hw")

	q := e.queue(huong, "kind=quiz")
	assert.Equal(t, []string{
		"quiz Hoàng Thu Hà basic01 " + dbTest,
		"quiz Lê Văn Cường basic01 " + reTest,
		"quiz Nguyễn Hoàng An basic01 " + webTest,
		"quiz Trần Thị Bích basic01 " + dsTest,
		"quiz Nguyễn Hoàng An basic01 " + goTest,
	}, rows(q.Items))
	assert.Contains(t, itemIDs(q.Items), attemptPendingID.String())
	// Số đếm luôn trên toàn hàng chờ; ô lọc học liệu chỉ còn quiz bài tập.
	assert.Equal(t, 8, q.Total)
	assert.Equal(t, 5, q.QuizPending)
	assert.Equal(t, 2, q.OverdueCount)
	require.NotEmpty(t, q.Filters.Lessons)
	for _, l := range q.Filters.Lessons {
		assert.Equal(t, "quiz", l.Kind, l.Title)
	}

	q = e.queue(huong, "kind=homework")
	assert.Equal(t, []string{
		"homework Trần Thị Bích basic01 " + dsHW,
		"homework Đặng Hải Phong basic01 " + dsHW,
		"homework Lê Văn Cường basic01 " + goHW,
	}, rows(q.Items))
	for _, l := range q.Filters.Lessons {
		assert.Equal(t, "homework", l.Kind, l.Title)
	}

	q = e.queue(huong, "lessonId="+dsHWID.String())
	assert.Equal(t, []string{
		"homework Trần Thị Bích basic01 " + dsHW,
		"homework Đặng Hải Phong basic01 " + dsHW,
	}, rows(q.Items))

	q = e.queue(huong, "overdue=1")
	assert.Equal(t, []string{
		"homework Trần Thị Bích basic01 " + dsHW,
		"quiz Hoàng Thu Hà basic01 " + dbTest,
	}, rows(q.Items))
	q = e.queue(huong, "overdue=1&kind=homework")
	assert.Equal(t, []string{"homework Trần Thị Bích basic01 " + dsHW}, rows(q.Items))
	q = e.queue(huong, "overdue=1&kind=quiz")
	assert.Equal(t, []string{"quiz Hoàng Thu Hà basic01 " + dbTest}, rows(q.Items))

	q = e.queue(huong, "classId="+e.classID("basic01").String()+"&lessonId="+e.lessonID("go-hw").String())
	assert.Equal(t, []string{"homework Lê Văn Cường basic01 " + goHW}, rows(q.Items))

	// Lớp của giảng viên khác: không có mục và không có học liệu nào để chọn.
	q = e.queue(huong, "classId="+e.classID("basic02").String())
	assert.Empty(t, q.Items)
	assert.Empty(t, q.Filters.Lessons)
	assert.Equal(t, 8, q.Total)

	// Admin lọc theo lớp: học liệu dùng chung phiên bản khóa chỉ hiện một lần.
	admin := e.login(adminEmail)
	q = e.queue(admin, "classId="+e.classID("basic02").String())
	assert.Equal(t, []string{
		"homework Đặng Hải Phong basic02 " + dsHW,
		"quiz Đỗ Ngọc Linh basic02 " + dbTest,
		"homework Đỗ Ngọc Linh basic02 " + dsHW,
		"quiz Đặng Hải Phong basic02 " + dsTest,
	}, rows(q.Items))
	all := e.queue(admin, "")
	seen := map[string]bool{}
	for _, l := range all.Filters.Lessons {
		assert.False(t, seen[l.ID], "học liệu %s lặp", l.Title)
		seen[l.ID] = true
	}
	assert.Equal(t, len(q.Filters.Lessons), len(all.Filters.Lessons))

	for _, bad := range []string{"kind=video", "classId=abc", "lessonId=1", "overdue=maybe"} {
		e.expectError(huong, queueURL+"?"+bad, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	}
}

// overdue chỉ bật khi đã chờ quá đúng 72 giờ.
func TestQueueOverdueBoundary(t *testing.T) {
	e := newIT(t)
	ctx := context.Background()
	admin := e.actor(adminEmail)
	bichID := e.pendingSubmission("basic01", bichEmail, "ds-hw")
	var submittedAt time.Time
	require.NoError(t, e.dbx.Get(&submittedAt, `SELECT submitted_at FROM submissions WHERE id = $1`, bichID))

	find := func() grading.QueueItem {
		q, err := e.svc.Queue(ctx, admin, grading.Filter{})
		require.NoError(t, err)
		for _, it := range q.Items {
			if it.Key.ItemID == bichID {
				return it
			}
		}
		t.Fatalf("không thấy bài của Bích")
		return grading.QueueItem{}
	}

	e.clk.T = submittedAt.Add(grading.GradingSLA)
	it := find()
	assert.False(t, it.Overdue, "đúng 72 giờ chưa quá hạn")
	assert.Equal(t, 72, it.WaitedHours)
	q, err := e.svc.Queue(ctx, admin, grading.Filter{OverdueOnly: true})
	require.NoError(t, err)
	assert.NotContains(t, queueKeys(q.Items), grading.QueueKey{Kind: grading.KindHomework, ItemID: bichID})

	e.clk.T = submittedAt.Add(grading.GradingSLA + time.Microsecond)
	it = find()
	assert.True(t, it.Overdue, "quá 72 giờ")
	q, err = e.svc.Queue(ctx, admin, grading.Filter{OverdueOnly: true})
	require.NoError(t, err)
	assert.Contains(t, queueKeys(q.Items), grading.QueueKey{Kind: grading.KindHomework, ItemID: bichID})
}

func queueKeys(items []grading.QueueItem) []grading.QueueKey {
	out := make([]grading.QueueKey, len(items))
	for i, it := range items {
		out[i] = it.Key
	}
	return out
}

func TestQueueHidesDroppedMembersAndSuperseded(t *testing.T) {
	e := newIT(t)
	admin := e.login(adminEmail)

	// Bản nộp superseded (Cường ds-hw lần 1) và lượt quiz superseded (Cường go-quiz lượt 1) của seed không vào hàng chờ.
	var superseded []string
	require.NoError(t, e.dbx.Select(&superseded, `SELECT id::text FROM submissions WHERE status = 'superseded'
		UNION ALL SELECT id::text FROM quiz_attempts WHERE review_status = 'superseded'`))
	require.Len(t, superseded, 2)
	before := e.queue(admin, "")
	for _, id := range superseded {
		assert.NotContains(t, itemIDs(before.Items), id)
	}
	require.Equal(t, 3, before.OverdueCount)

	// Phong rời basic02: bài quá hạn và lượt quiz chờ xác nhận của Phong rời hàng chờ, số đếm và mục kế tiếp; cả hai vẫn
	// còn pending trong DB.
	phong02 := e.pendingSubmission("basic02", phongEmail, "ds-hw")
	phongQuiz := e.id(`SELECT id FROM quiz_attempts WHERE class_member_id = $1 AND review_status = 'pending'`,
		e.memberID("basic02", phongEmail))
	e.exec(`UPDATE class_members SET status = 'dropped', dropped_at = $2 WHERE id = $1`,
		e.memberID("basic02", phongEmail), e.clk.Now())
	after := e.queue(admin, "")
	assert.NotContains(t, itemIDs(after.Items), phong02.String())
	assert.NotContains(t, itemIDs(after.Items), phongQuiz.String())
	assert.Equal(t, before.Total-2, after.Total)
	assert.Equal(t, before.QuizPending-1, after.QuizPending)
	assert.Equal(t, 2, after.OverdueCount)
	assert.Empty(t, e.queue(admin, "overdue=1&classId="+e.classID("basic02").String()).Items)

	bao := e.queue(e.login(baoEmail), "")
	assert.Equal(t, []string{
		"quiz Đỗ Ngọc Linh basic02 " + dbTest,
		"homework Đỗ Ngọc Linh basic02 " + dsHW,
	}, rows(bao.Items))
	assert.Zero(t, bao.OverdueCount)

	next, err := e.svc.Next(context.Background(), e.dbx, e.actor(baoEmail), grading.QueueKey{})
	require.NoError(t, err)
	require.NotNil(t, next)
	assert.Equal(t, "Đỗ Ngọc Linh", next.Student.Name)
}

func TestNextSkipsHandledItemWithinScope(t *testing.T) {
	e := newIT(t)
	e.seedQuiz()
	ctx := context.Background()
	huong, bao := e.actor(huongEmail), e.actor(baoEmail)
	bichKey := grading.QueueKey{Kind: grading.KindHomework, ItemID: e.pendingSubmission("basic01", bichEmail, "ds-hw")}

	n, err := e.svc.Next(ctx, e.dbx, huong, grading.QueueKey{})
	require.NoError(t, err)
	require.NotNil(t, n)
	assert.Equal(t, bichKey, n.Key)

	// Mục vừa xử lý bị bỏ qua dù vẫn còn trong hàng chờ: kế tiếp là lượt quiz lượt 2 của Hà.
	haAttempt := e.id(`SELECT id FROM quiz_attempts WHERE class_member_id = $1 AND review_status = 'pending'`,
		e.memberID("basic01", haEmail))
	n, err = e.svc.Next(ctx, e.dbx, huong, bichKey)
	require.NoError(t, err)
	require.NotNil(t, n)
	assert.Equal(t, grading.QueueKey{Kind: grading.KindQuiz, ItemID: haAttempt}, n.Key)
	assert.Equal(t, huong.ID, n.TeacherID)
	assert.True(t, n.Overdue)

	// Giảng viên basic02 không bao giờ nhận mục của basic01.
	n, err = e.svc.Next(ctx, e.dbx, bao, grading.QueueKey{})
	require.NoError(t, err)
	require.NotNil(t, n)
	assert.Equal(t, "basic02", n.Class.Code)

	_, err = e.svc.Next(ctx, e.dbx, e.actor(anEmail), grading.QueueKey{})
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestRecentGradedAndConfirmed(t *testing.T) {
	e := newIT(t)
	e.seedQuiz()
	huong := e.login(huongEmail)

	raw := e.must(huong, recentURL)
	r := decode[grading.RecentListDTO](t, raw).Items
	got := make([]string, len(r))
	for i, x := range r {
		got[i] = fmt.Sprintf("%s %s #%d %d/%d %v %s", x.Kind, x.Student.Name, x.No, x.Points, x.MaxPoints, x.Passed, x.By.Name)
	}
	// Lần chấm hiện hành của mỗi bài nộp và nhận xét hiện hành của lượt quiz bài tập đã xác nhận (của test và của
	// seed), mới nhất trước, tối đa DefaultRecentLimit; lượt luyện tập đã nhận xét không thuộc hàng chờ nên vắng.
	assert.Equal(t, []string{
		"quiz Nguyễn Hoàng An #1 5/10 false Lê Thu Hương",
		"quiz Lê Văn Cường #2 3/5 false Lê Thu Hương",
		"quiz Đặng Hải Phong #1 4/5 true Lê Thu Hương",
		"quiz Nguyễn Hoàng An #1 5/5 true Lê Thu Hương",
		"homework Nguyễn Hoàng An #2 94/100 true Lê Thu Hương",
		"quiz Lê Văn Cường #1 4/5 true Lê Thu Hương",
	}, got)
	assert.Equal(t, attemptConfirmedID.String(), r[0].ItemID)
	assert.True(t, r[0].At.Equal(e.clk.Now().Add(-2*time.Hour)))
	for i := 1; i < len(r); i++ {
		assert.False(t, r[i].At.After(r[i-1].At), "mới nhất trước")
	}
	assertGolden(t, "queue-recent.json", raw)

	assert.Len(t, decode[grading.RecentListDTO](t, e.must(huong, recentURL+"?limit=2")).Items, 2)
	bao := decode[grading.RecentListDTO](t, e.must(e.login(baoEmail), recentURL)).Items
	// basic02 trong seed chỉ có bài ds-hw Cần làm lại của Nhàn và lượt db-test của Phong, đều do Bảo chấm/xác nhận.
	baoGot := make([]string, len(bao))
	for i, x := range bao {
		baoGot[i] = fmt.Sprintf("%s %s %s #%d %d/%d %v %s", x.Class.Code, x.Kind, x.Student.Name, x.No, x.Points, x.MaxPoints,
			x.Passed, x.By.Name)
	}
	assert.Equal(t, []string{
		"basic02 homework Ngô Thanh Nhàn #1 45/100 false Phạm Quốc Bảo",
		"basic02 quiz Đặng Hải Phong #1 4/5 true Phạm Quốc Bảo",
	}, baoGot)
	assert.Len(t, decode[grading.RecentListDTO](t, e.must(e.login(adminEmail), recentURL)).Items, grading.DefaultRecentLimit)

	for _, bad := range []string{"limit=0", "limit=51", "limit=abc"} {
		e.expectError(huong, recentURL+"?"+bad, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	}
}

func TestGradingRoutesRequireStaff(t *testing.T) {
	e := newIT(t)
	an := e.login(anEmail)
	e.expectError(an, queueURL, http.StatusForbidden, "FORBIDDEN")
	e.expectError(an, recentURL, http.StatusForbidden, "FORBIDDEN")
	e.expectError("", queueURL, http.StatusUnauthorized, "UNAUTHENTICATED")
}

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizeGolden thay mốc thời gian (khóa "at" hoặc đuôi "At") bằng một mốc cố định và uuid bằng id giả theo thứ tự
// xuất hiện, để golden không đổi giữa các lần seed.
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
