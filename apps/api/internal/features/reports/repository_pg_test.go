//go:build integration

package reports

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
	"lms/api/internal/features/courses"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/learning"
	"lms/api/internal/features/media"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
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

// Tài khoản và lớp theo seed.js.
const (
	adminEmail   = "quan.tran@goup.vn"
	huongEmail   = "huong.le@goup.vn" // giảng viên basic01, basic03
	baoEmail     = "bao.pham@goup.vn" // giảng viên basic02
	anEmail      = "an.nguyen@gmail.com"
	cuongEmail   = "cuong.le@outlook.com"
	dungEmail    = "dung.pham@gmail.com" // chưa đăng nhập
	khangEmail   = "khang.vu@gmail.com"  // 21 ngày không hoạt động
	thaoEmail    = "thao.vo@gmail.com"   // đã rời basic01, tài khoản bị vô hiệu hóa
	linhEmail    = "linh.do@gmail.com"   // học viên basic02
	minhEmail    = "minh.bui@gmail.com"  // lời mời basic02 gửi thất bại
	basic01Code  = "basic01"
	basic02Code  = "basic02"
	reportPrefix = "/api/v1/classes/"
)

// itEnv dựng reports trên DB test đã seed theo seed.js, cùng identity thật (đăng nhập, cookie phiên, RequireRole) và
// stages/courses thật để tạo dữ liệu FR-18/FR-17.
type itEnv struct {
	t       *testing.T
	dbx     *sqlx.DB
	clk     *clock.Fake
	ident   *identity.Service
	stages  *stages.Service
	courses *courses.Service
	engine  *gin.Engine
}

func newIT(t *testing.T) *itEnv {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)
	clk := &clock.Fake{T: time.Now().UTC().Truncate(time.Second)}
	_, err = seed.Run(context.Background(), dbx, config.Config{
		AppEnv: config.EnvDev, PublicBaseURL: "http://localhost:5173", TempPasswordTTL: 72 * time.Hour,
		SeedPassword: itSeedPassword, OutboxSecretKey: itOutboxKey, Location: loc,
	}, clk, seed.Options{})
	require.NoError(t, err)

	rec := audit.PG{Clock: clk}
	e := &itEnv{t: t, dbx: dbx, clk: clk}
	e.ident = identity.NewService(identity.ServiceDeps{
		DB: dbx, Tx: db.TxRunner{DB: dbx},
		Users: identity.PGUserRepo{}, Sessions: identity.PGSessionRepo{}, Attempts: identity.PGLoginAttemptRepo{},
		Resets: identity.PGResetTokenRepo{}, Clock: clk, Audit: rec,
		Cfg: config.Config{
			SessionTTL: 12 * time.Hour, TempPasswordTTL: 72 * time.Hour, ResetTokenTTL: 30 * time.Minute,
			PasswordMinLength: 8, LoginMaxFailures: 5, LoginLockWindow: 15 * time.Minute,
			PublicBaseURL: "http://localhost:5173", AppEnv: config.EnvE2E,
		},
	})
	e.stages = stages.NewService(stages.Deps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Stages: stages.PGStageRepo{}, Versions: stages.PGStageVersionRepo{},
		Media: media.PGRepo{}, Render: stages.NewMarkdownRenderer(), Clock: clk, Audit: rec, IDs: ids.V7{},
	})
	e.courses = courses.NewService(courses.Deps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Courses: courses.PGCourseRepo{}, Versions: courses.PGCourseVersionRepo{},
		StageVersions: stageRefs{}, Clock: clk, Audit: rec, IDs: ids.V7{},
	})
	svc := NewService(Deps{
		Tx: ReadTx{DB: dbx}, Reports: PGReportRepo{}, Dashboard: PGDashboardRepo{}, Progress: learning.PGProgressReader{},
		Outdated: e.stages, Audit: audit.PGReader{}, Users: identity.PGUserRepo{}, Clock: clk, StaleDays: 7,
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recover(slog.New(slog.DiscardHandler)), middleware.SecurityHeaders())
	authed := engine.Group("/api/v1", identity.NewMiddleware(e.ident, false).SessionAuth())
	NewHandler(svc, func(c *gin.Context) (uuid.UUID, domain.Role, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return uuid.Nil, "", false
		}
		return u.ID(), u.Role(), true
	}).Register(authed, identity.RequireRole(domain.RoleAdmin, domain.RoleTeacher), identity.RequireRole(domain.RoleAdmin))
	e.engine = engine
	return e
}

// stageRefs là courses.StageVersionReader trên repository thật của stages, cùng ánh xạ với adapter ở app/deps.go.
type stageRefs struct{}

func (stageRefs) Refs(ctx context.Context, ex db.Executor, versionIDs []uuid.UUID) (map[uuid.UUID]courses.StageVersionRef, error) {
	refs, err := stages.PGStageVersionRepo{}.Refs(ctx, ex, versionIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]courses.StageVersionRef, len(refs))
	for id, v := range refs {
		out[id] = courses.StageVersionRef{ID: v.ID, StageID: v.StageID, StageCode: v.StageCode, VersionNo: v.VersionNo, Status: v.Status}
	}
	return out, nil
}

// login đăng nhập thật qua identity, trả giá trị cookie phiên.
func (e *itEnv) login(email string) string {
	e.t.Helper()
	res, err := e.ident.Login(context.Background(), email, itSeedPassword, "127.0.0.1", "it")
	require.NoError(e.t, err, "đăng nhập %s", email)
	return res.Token.Reveal()
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

func (e *itEnv) memberID(classCode, email string) uuid.UUID {
	return e.id(`SELECT cm.id FROM class_members cm JOIN classes c ON c.id = cm.class_id JOIN users u ON u.id = cm.user_id
		WHERE c.code = $1 AND u.email = $2`, classCode, email)
}

func (e *itEnv) count(q string, args ...any) int {
	e.t.Helper()
	var n int
	require.NoError(e.t, e.dbx.Get(&n, q, args...), q)
	return n
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

// must gọi API và bắt buộc 200.
func (e *itEnv) must(cookie, path string) []byte {
	e.t.Helper()
	r := e.get(cookie, path)
	require.Equal(e.t, http.StatusOK, r.code, "%s: %s", path, r.body)
	assert.Equal(e.t, "private, no-store", r.header.Get("Cache-Control"), path)
	return r.body
}

// expectError kiểm mã HTTP, mã lỗi và (nếu có) message; trả body để so golden.
func (e *itEnv) expectError(cookie, path string, status int, code, msg string) []byte {
	e.t.Helper()
	r := e.get(cookie, path)
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(e.t, json.Unmarshal(r.body, &env), "%s: %s", path, r.body)
	require.Equal(e.t, status, r.code, "%s: %s", path, r.body)
	assert.Equal(e.t, code, env.Error.Code, path)
	if msg != "" {
		assert.Equal(e.t, msg, env.Error.Message, path)
	}
	return r.body
}

func decode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(raw, &v), "%s", raw)
	return v
}

func reportURL(classID uuid.UUID, query string) string {
	u := reportPrefix + classID.String() + "/report"
	if query != "" {
		u += "?" + query
	}
	return u
}

func memberURL(classID, memberID uuid.UUID) string {
	return reportPrefix + classID.String() + "/report/members/" + memberID.String()
}

func (e *itEnv) report(cookie string, classID uuid.UUID, query string) ClassReportDTO {
	e.t.Helper()
	return decode[ClassReportDTO](e.t, e.must(cookie, reportURL(classID, query)))
}

func emails(rows []ReportRowDTO) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Email
	}
	return out
}

func rowByEmail(t *testing.T, rows []ReportRowDTO, email string) ReportRowDTO {
	t.Helper()
	for _, r := range rows {
		if r.Email == email {
			return r
		}
	}
	t.Fatalf("không có dòng %s", email)
	return ReportRowDTO{}
}

func TestHTTPClassReportColumnsAndDefaults(t *testing.T) {
	e := newIT(t)
	admin := e.login(adminEmail)
	basic01 := e.classID(basic01Code)

	raw := e.must(admin, reportURL(basic01, ""))
	r := decode[ClassReportDTO](t, raw)
	assert.True(t, r.SelfReported)
	assert.Equal(t, basic01Code, r.Class.Code)
	assert.Equal(t, "Lập trình cơ bản", r.Class.CourseName)
	assert.Equal(t, 1, r.Class.CourseVersionNo)
	assert.Equal(t, "Lê Thu Hương", r.Class.Teacher.Name)

	codes := make([]string, len(r.Stages))
	for i, s := range r.Stages {
		codes[i] = s.Code
		assert.Equal(t, i+1, s.Position)
	}
	assert.Equal(t, []string{"DB", "DS", "GO", "RE", "WEB"}, codes)

	// Không lọc: 7 thành viên active sắp theo tên (collation của DB), không có Thảo.
	var wantNames []string
	require.NoError(t, e.dbx.Select(&wantNames, `SELECT u.full_name FROM class_members cm JOIN users u ON u.id = cm.user_id
		WHERE cm.class_id = $1 AND cm.status = 'active' ORDER BY u.full_name, cm.id`, basic01))
	require.Len(t, r.Rows, 7)
	names := make([]string, len(r.Rows))
	for i, row := range r.Rows {
		names[i] = row.Name
	}
	assert.Equal(t, wantNames, names)
	assert.NotContains(t, emails(r.Rows), thaoEmail)

	// Đủ cột FR-40 trên dòng của An: % từng chặng theo thứ tự chặng, % tổng chỉ tính bài bắt buộc (10/14).
	an := rowByEmail(t, r.Rows, anEmail)
	assert.Equal(t, "Nguyễn Hoàng An", an.Name)
	assert.Equal(t, "active", an.AccountStatus)
	assert.Equal(t, "active", an.MemberStatus)
	assert.False(t, an.MustChangePassword)
	assert.Equal(t, 10, an.RequiredDone)
	assert.Equal(t, 14, an.RequiredTotal)
	assert.Equal(t, 71, an.Percent)
	require.Len(t, an.StagePercents, 5)
	pcts := make([]int, 5)
	for i, sp := range an.StagePercents {
		assert.Equal(t, r.Stages[i].StageID, sp.StageID)
		assert.Equal(t, r.Stages[i].RequiredTotal, sp.RequiredTotal)
		pcts[i] = sp.Percent
	}
	assert.Equal(t, []int{100, 100, 100, 67, 0}, pcts)
	require.NotNil(t, an.Invite)
	assert.Equal(t, "invite", an.Invite.Kind)
	require.NotNil(t, an.Invite.Status)
	assert.Equal(t, "sent", *an.Invite.Status)
	assert.NotNil(t, an.LastLoginAt)
	assert.NotNil(t, an.LastActivityAt)

	dung := rowByEmail(t, r.Rows, dungEmail)
	assert.Equal(t, "invited", dung.AccountStatus)
	assert.True(t, dung.MustChangePassword)
	assert.Nil(t, dung.LastLoginAt)
	assert.Nil(t, dung.LastActivityAt)
	assert.Equal(t, 0, dung.Percent)

	// Tổng hợp theo cùng quy tắc includeDropped; ngưỡng mặc định 7 ngày và 50%.
	assert.Equal(t, SummaryDTO{
		MemberCount: 7, ActiveCount: 7, AvgPercent: r.Summary.AvgPercent, NotLoggedInCount: 1, InactiveCount: 2, BelowCount: 4,
		InactiveDays: 7, BelowPercent: 50,
	}, r.Summary)
	sum := 0
	for _, row := range r.Rows {
		sum += row.Percent
	}
	assert.Equal(t, int(float64(sum)/7+0.5), r.Summary.AvgPercent)
	assert.Equal(t, FilterDTO{Sort: "name"}, r.Filter)
	assertGolden(t, "class-report.json", raw)

	// includeDropped: thêm Thảo với memberStatus dropped; summary đếm cả dòng dropped.
	withDropped := e.report(admin, basic01, "includeDropped=true")
	require.Len(t, withDropped.Rows, 8)
	thao := rowByEmail(t, withDropped.Rows, thaoEmail)
	assert.Equal(t, "dropped", thao.MemberStatus)
	assert.Equal(t, "disabled", thao.AccountStatus)
	assert.Equal(t, 8, withDropped.Summary.MemberCount)
	assert.Equal(t, 7, withDropped.Summary.ActiveCount)
	assert.True(t, withDropped.Filter.IncludeDropped)
}

func TestHTTPClassReportFiltersAndSorts(t *testing.T) {
	e := newIT(t)
	admin := e.login(adminEmail)
	basic01 := e.classID(basic01Code)

	all := e.report(admin, basic01, "includeDropped=true&sort=name")
	require.Len(t, all.Rows, 8)

	filtered := func(query string) []string {
		return emails(e.report(admin, basic01, query).Rows)
	}
	assert.Equal(t, []string{dungEmail}, filtered("notLoggedIn=true"))
	assert.ElementsMatch(t, []string{dungEmail, khangEmail}, filtered("inactiveDays=7"))
	below := filtered("belowPercent=50")
	assert.Len(t, below, 4)
	for _, em := range below {
		assert.Less(t, rowByEmail(t, all.Rows, em).Percent, 50, em)
	}
	assert.Equal(t, []string{dungEmail}, filtered("notLoggedIn=true&inactiveDays=7&belowPercent=30"))
	assert.ElementsMatch(t, []string{dungEmail, khangEmail}, filtered("inactiveDays=7&belowPercent=30"))
	assert.Empty(t, filtered("notLoggedIn=true&belowPercent=0"))

	// SQL (Filter.Apply) và Filter.Match cho cùng tập dòng trên mọi tổ hợp lọc.
	full := make([]ReportRow, 0, len(all.Rows))
	for _, d := range all.Rows {
		row := ReportRow{
			Email: d.Email, MemberStatus: domain.MemberStatus(d.MemberStatus), Percent: d.Percent,
			LastLoginAt: d.LastLoginAt, LastActivityAt: d.LastActivityAt,
		}
		full = append(full, row)
	}
	now := e.clk.Now()
	for _, notLogged := range []bool{false, true} {
		for _, inactive := range []*int{nil, intp(1), intp(3), intp(7), intp(30)} {
			for _, belowPct := range []*int{nil, intp(0), intp(30), intp(50), intp(100)} {
				for _, dropped := range []bool{false, true} {
					f := Filter{NotLoggedIn: notLogged, InactiveDays: inactive, BelowPercent: belowPct, IncludeDropped: dropped, Sort: SortName}
					var want []string
					for _, row := range full {
						if f.Match(row, now) {
							want = append(want, row.Email)
						}
					}
					got := filtered(filterQuery(f))
					assert.ElementsMatch(t, want, got, filterQuery(f))
				}
			}
		}
	}

	// Năm sort: thứ tự đúng khóa, hòa thì theo tên (thứ tự tên lấy từ sort=name của DB), null nhất quán.
	rank := map[string]int{}
	for i, r := range all.Rows {
		rank[r.Email] = i
	}
	byName := func(a, b ReportRowDTO) bool { return rank[a.Email] < rank[b.Email] }
	activity := func(r ReportRowDTO) time.Time {
		if r.LastActivityAt == nil {
			return time.Time{}
		}
		return *r.LastActivityAt
	}
	less := map[string]func(a, b ReportRowDTO) bool{
		"name": byName,
		"pct": func(a, b ReportRowDTO) bool {
			if a.Percent != b.Percent {
				return a.Percent < b.Percent
			}
			return byName(a, b)
		},
		"pct-desc": func(a, b ReportRowDTO) bool {
			if a.Percent != b.Percent {
				return a.Percent > b.Percent
			}
			return byName(a, b)
		},
		"activity": func(a, b ReportRowDTO) bool { // gần nhất trước, null cuối
			if !activity(a).Equal(activity(b)) {
				return activity(a).After(activity(b))
			}
			return byName(a, b)
		},
		"activity-asc": func(a, b ReportRowDTO) bool { // null đầu
			if !activity(a).Equal(activity(b)) {
				return activity(a).Before(activity(b))
			}
			return byName(a, b)
		},
	}
	for s, lessFn := range less {
		got := e.report(admin, basic01, "includeDropped=true&sort="+s).Rows
		want := append([]ReportRowDTO(nil), all.Rows...)
		sort.SliceStable(want, func(i, j int) bool { return lessFn(want[i], want[j]) })
		assert.Equal(t, emails(want), emails(got), s)
		assert.Equal(t, s, e.report(admin, basic01, "sort="+s).Filter.Sort)
	}
	assert.Equal(t, cuongEmail, e.report(admin, basic01, "sort=pct-desc").Rows[0].Email, "Cường 13/14 dẫn đầu")
	act := e.report(admin, basic01, "sort=activity").Rows
	assert.Equal(t, dungEmail, act[len(act)-1].Email, "chưa hoạt động nằm cuối")
	assert.Equal(t, dungEmail, e.report(admin, basic01, "sort=activity-asc").Rows[0].Email)

	f := e.report(admin, basic01, "notLoggedIn=true&inactiveDays=7&belowPercent=30&sort=pct-desc").Filter
	assert.Equal(t, FilterDTO{NotLoggedIn: true, InactiveDays: intp(7), BelowPercent: intp(30), Sort: "pct-desc"}, f)

	// Tham số sai → 422 trước khi đọc dữ liệu.
	invalid := []string{
		"sort=evil", "sort=name%3B%20DROP%20TABLE%20users", "inactiveDays=0", "inactiveDays=-3", "inactiveDays=abc",
		"belowPercent=101", "belowPercent=-1", "belowPercent=x", "notLoggedIn=maybe", "includeDropped=2",
	}
	for i, q := range invalid {
		body := e.expectError(admin, reportURL(basic01, q), http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Tham số lọc không hợp lệ.")
		if i == 0 {
			assertGolden(t, "class-report-invalid-filter.json", body)
		}
	}
}

// filterQuery dựng query string tương ứng f.
func filterQuery(f Filter) string {
	parts := []string{fmt.Sprintf("notLoggedIn=%t", f.NotLoggedIn), fmt.Sprintf("includeDropped=%t", f.IncludeDropped), "sort=" + string(f.Sort)}
	if f.InactiveDays != nil {
		parts = append(parts, fmt.Sprintf("inactiveDays=%d", *f.InactiveDays))
	}
	if f.BelowPercent != nil {
		parts = append(parts, fmt.Sprintf("belowPercent=%d", *f.BelowPercent))
	}
	return strings.Join(parts, "&")
}

func TestHTTPClassReportScope(t *testing.T) {
	e := newIT(t)
	admin, huong, bao, student := e.login(adminEmail), e.login(huongEmail), e.login(baoEmail), e.login(linhEmail)
	basic01, basic02 := e.classID(basic01Code), e.classID(basic02Code)
	anMember := e.memberID(basic01Code, anEmail)

	e.must(admin, reportURL(basic01, ""))
	e.must(admin, reportURL(basic02, ""))
	e.must(huong, reportURL(basic01, ""))
	e.must(huong, memberURL(basic01, anMember))
	e.must(bao, reportURL(basic02, ""))

	const notOwn = "Bạn chỉ xem được lớp mình phụ trách."
	e.expectError(huong, reportURL(basic02, ""), http.StatusForbidden, "FORBIDDEN", notOwn)
	e.expectError(bao, reportURL(basic01, ""), http.StatusForbidden, "FORBIDDEN", notOwn)
	e.expectError(bao, memberURL(basic01, anMember), http.StatusForbidden, "FORBIDDEN", notOwn)
	e.expectError(student, reportURL(basic02, ""), http.StatusForbidden, "FORBIDDEN", "")
	e.expectError(student, memberURL(basic01, anMember), http.StatusForbidden, "FORBIDDEN", "")
	e.expectError("", reportURL(basic01, ""), http.StatusUnauthorized, "UNAUTHENTICATED", "")

	e.expectError(admin, reportURL(uuid.New(), ""), http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(admin, reportPrefix+"not-a-uuid/report", http.StatusNotFound, "NOT_FOUND", "")
	// Tham số lọc sai bị từ chối trước khi đọc lớp, nên 422 không lộ gì về lớp của người khác.
	e.expectError(huong, reportURL(basic02, "sort=evil"), http.StatusUnprocessableEntity, "VALIDATION_FAILED", "")
}

func TestHTTPMemberReport(t *testing.T) {
	e := newIT(t)
	admin, huong := e.login(adminEmail), e.login(huongEmail)
	basic01 := e.classID(basic01Code)
	anMember := e.memberID(basic01Code, anEmail)

	raw := e.must(admin, memberURL(basic01, anMember))
	r := decode[MemberReportDTO](t, raw)
	assert.True(t, r.SelfReported)
	assert.Equal(t, basic01Code, r.Class.Code)
	assert.Equal(t, anMember.String(), r.Member.MemberID)
	assert.Equal(t, 71, r.Member.Percent)

	// Đủ mọi học liệu của course version (16 bài, kể cả bài không bắt buộc) với state và mốc thời gian.
	require.Len(t, r.Stages, 5)
	lessons := 0
	states := map[string]int{}
	for _, st := range r.Stages {
		for _, l := range st.Lessons {
			lessons++
			states[l.State]++
			switch l.State {
			case "completed":
				assert.NotNil(t, l.FirstOpenedAt, l.Title)
				assert.NotNil(t, l.CompletedAt, l.Title)
			case "opened":
				assert.NotNil(t, l.FirstOpenedAt, l.Title)
				assert.Nil(t, l.CompletedAt, l.Title)
			default:
				assert.Equal(t, "not_opened", l.State, l.Title)
				assert.Nil(t, l.FirstOpenedAt, l.Title)
				assert.Nil(t, l.CompletedAt, l.Title)
			}
		}
	}
	assert.Equal(t, 16, lessons)
	assert.Equal(t, map[string]int{"completed": 11, "opened": 1, "not_opened": 4}, states)
	assert.Equal(t, "Hooks", r.Stages[3].Lessons[2].Title)
	assert.Equal(t, "opened", r.Stages[3].Lessons[2].State)
	assertGolden(t, "member-report.json", raw)

	// Thành viên đã rời lớp vẫn xem được drilldown.
	thao := decode[MemberReportDTO](t, e.must(huong, memberURL(basic01, e.memberID(basic01Code, thaoEmail))))
	assert.Equal(t, "dropped", thao.Member.MemberStatus)

	// mid phải thuộc lớp của URL: thành viên basic02 gọi trên basic01 → 404, mid không tồn tại → 404.
	linhMember := e.memberID(basic02Code, linhEmail)
	e.expectError(huong, memberURL(basic01, linhMember), http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(admin, memberURL(basic01, linhMember), http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(huong, memberURL(basic01, uuid.New()), http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(huong, reportPrefix+basic01.String()+"/report/members/abc", http.StatusNotFound, "NOT_FOUND", "")
}

func TestHTTPDashboard(t *testing.T) {
	e := newIT(t)
	admin, huong, student := e.login(adminEmail), e.login(huongEmail), e.login(linhEmail)
	e.expectError(huong, "/api/v1/dashboard", http.StatusForbidden, "FORBIDDEN", "")
	e.expectError(student, "/api/v1/dashboard", http.StatusForbidden, "FORBIDDEN", "")

	d := decode[DashboardDTO](t, e.must(admin, "/api/v1/dashboard"))
	assert.Empty(t, d.Outdated, "seed chỉ có một phiên bản mỗi chặng")
	assert.Equal(t, 0, d.Hints.OutdatedCourses)

	// FR-18: phát hành Database v2 → khóa BASIC (đang dùng Database v1) lỗi thời.
	ctx := context.Background()
	adminID := e.id(`SELECT id FROM users WHERE email = $1`, adminEmail)
	dbV1 := e.id(`SELECT sv.id FROM stage_versions sv JOIN stages s ON s.id = sv.stage_id WHERE s.code = 'DB' AND sv.version_no = 1`)
	e.clk.Advance(time.Minute)
	clone, err := e.stages.Clone(ctx, stages.Actor{ID: adminID}, dbV1)
	require.NoError(t, err)
	e.clk.Advance(time.Minute)
	_, err = e.stages.Publish(ctx, stages.Actor{ID: adminID}, clone.Version.ID())
	require.NoError(t, err)

	raw := e.must(admin, "/api/v1/dashboard")
	d = decode[DashboardDTO](t, raw)

	// kpis/hints khớp đếm SQL thủ công.
	assert.Equal(t, KPIsDTO{
		Stages:  e.count(`SELECT count(*) FROM stages`),
		Courses: e.count(`SELECT count(*) FROM courses`),
		Classes: e.count(`SELECT count(*) FROM classes WHERE status = 'active'`),
		Students: e.count(`SELECT count(DISTINCT cm.user_id) FROM class_members cm JOIN classes c ON c.id = cm.class_id
			WHERE cm.status = 'active' AND c.status = 'active'`),
	}, d.KPIs)
	assert.Equal(t, KPIsDTO{Stages: 5, Courses: 1, Classes: 2, Students: 11}, d.KPIs)
	assert.Equal(t, HintsDTO{
		DraftClasses: e.count(`SELECT count(*) FROM classes WHERE status = 'draft'`),
		NotLoggedIn: e.count(`SELECT count(DISTINCT u.id) FROM users u JOIN class_members cm ON cm.user_id = u.id
			JOIN classes c ON c.id = cm.class_id WHERE cm.status = 'active' AND c.status = 'active' AND u.last_login_at IS NULL`),
		OutdatedCourses: 1,
		FailedInvites: e.count(`SELECT count(*) FROM email_outbox eo JOIN invitations i ON i.email_outbox_id = eo.id
			WHERE eo.status = 'failed' AND eo.attempts = 3 AND eo.last_error = 'Mailbox không tồn tại (550 5.1.1)'`),
	}, d.Hints)
	assert.Equal(t, HintsDTO{DraftClasses: 1, NotLoggedIn: 3, OutdatedCourses: 1, FailedInvites: 1}, d.Hints)

	require.Len(t, d.Outdated, 1)
	o := d.Outdated[0]
	// Cảnh báo nêu bản khóa học đang dùng chặng cũ và trỏ tới bản chặng mới nhất để áp dụng.
	assert.Equal(t, e.id(`SELECT cv.id FROM course_versions cv JOIN courses c ON c.id = cv.course_id
		WHERE c.code = 'BASIC' AND cv.version_no = 1`).String(), o.CourseVersionID)
	assert.Equal(t, e.id(`SELECT sv.id FROM stage_versions sv JOIN stages s ON s.id = sv.stage_id
		WHERE s.code = 'DB' AND sv.version_no = 2`).String(), o.LatestVersionID)
	o.CourseID, o.StageID, o.CourseVersionID, o.LatestVersionID = "", "", "", ""
	assert.Equal(t, OutdatedDTO{
		CourseCode: "BASIC", CourseName: "Lập trình cơ bản", CourseVersionNo: 1, StageCode: "DB", StageName: "Database",
		CurrentVersionNo: 1, LatestPublishedNo: 2,
	}, o)

	// classes[]: basic01 dùng BASIC v1 với 7 thành viên active; avgPercent khớp summary của báo cáo lớp.
	require.Len(t, d.Classes, 3)
	basic01 := d.Classes[0]
	assert.Equal(t, basic01Code, basic01.Code)
	assert.Equal(t, "Lập trình cơ bản", basic01.CourseName)
	assert.Equal(t, 1, basic01.CourseVersionNo)
	assert.Equal(t, 7, basic01.MemberCount)
	assert.Equal(t, e.report(admin, e.classID(basic01Code), "").Summary.AvgPercent, basic01.AvgPercent)

	// recentActivity: 8 bản mới nhất, mới nhất trước, nhãn thuộc bảng prototype.
	require.Len(t, d.RecentActivity, recentActivityLimit)
	labels := map[string]bool{}
	for _, l := range actionLabels {
		labels[l] = true
	}
	for i, a := range d.RecentActivity {
		assert.True(t, labels[a.ActionLabel], a.ActionLabel)
		assert.Equal(t, "Trần Minh Quân", a.ActorName)
		assert.NotEmpty(t, a.Target.Label)
		if i > 0 {
			assert.False(t, a.At.After(d.RecentActivity[i-1].At))
		}
	}
	assert.Equal(t, "Phát hành chặng", d.RecentActivity[0].ActionLabel)
	assert.Equal(t, "Database v2", d.RecentActivity[0].Target.Label)
	assert.Equal(t, "Nhân bản chặng", d.RecentActivity[1].ActionLabel)
	assert.Equal(t, "Database v1 → v2", d.RecentActivity[1].Summary)
	assertGolden(t, "dashboard.json", raw)

	// FR-17: áp dụng Database v2 cho BASIC → hết lỗi thời.
	e.clk.Advance(time.Minute)
	courseID := e.id(`SELECT id FROM courses WHERE code = 'BASIC'`)
	res, err := e.courses.ApplyStageVersion(ctx, courses.Actor{ID: adminID}, clone.Version.ID(), []uuid.UUID{courseID})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Nil(t, res[0].Error)

	d = decode[DashboardDTO](t, e.must(admin, "/api/v1/dashboard"))
	assert.Empty(t, d.Outdated)
	assert.Equal(t, 0, d.Hints.OutdatedCourses)
	var applied *ActivityDTO
	for i := range d.RecentActivity {
		if d.RecentActivity[i].ActionLabel == "Áp dụng chặng cho khóa học" {
			applied = &d.RecentActivity[i]
		}
	}
	require.NotNil(t, applied, "nhật ký có dòng áp dụng chặng")
	assert.Equal(t, "Lập trình cơ bản", applied.Target.Label)
	assert.Equal(t, "Database v2 → Lập trình cơ bản v2", applied.Summary)
}

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
