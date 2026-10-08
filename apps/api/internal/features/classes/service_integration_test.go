//go:build integration

package classes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/features/courses"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/middleware"
	"lms/api/internal/platform/secretbox"
	"lms/api/internal/platform/testdb"
)

// integrationKey là OUTBOX_SECRET_KEY chỉ dùng trong test (base64 của 32 byte).
const integrationKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

var (
	adminID    = uuid.MustParse(testdb.AdminQuanTranID)
	huongID    = uuid.MustParse(testdb.TeacherHuongLeID)
	anID       = uuid.MustParse(testdb.StudentAnNguyenID)
	basic01ID  = uuid.MustParse(testdb.ClassBasic01ID)
	memberAnID = uuid.MustParse(testdb.MemberAnBasic01ID)
	basicV1ID  = uuid.MustParse(testdb.CourseBasicV1ID)
	lessonID   = uuid.MustParse(testdb.LessonDBTableV1ID)
)

// Dữ liệu thêm theo prototype/seed.js, chèn bằng SQL trong test; id cố định (tiền tố 01990000-) để golden ổn định.
var (
	baoID          = uuid.MustParse("01990000-0000-7000-8000-000000000901") // giảng viên Phạm Quốc Bảo
	minhID         = uuid.MustParse("01990000-0000-7000-8000-000000000902") // invited, mật khẩu tạm còn hạn
	thaoID         = uuid.MustParse("01990000-0000-7000-8000-000000000903") // disabled
	khangID        = uuid.MustParse("01990000-0000-7000-8000-000000000904") // 21 ngày không hoạt động
	bichID         = uuid.MustParse("01990000-0000-7000-8000-000000000905") // active, chưa vào lớp nào
	dungID         = uuid.MustParse("01990000-0000-7000-8000-000000000906") // invited, mật khẩu tạm đã hết hạn
	teacherOffID   = uuid.MustParse("01990000-0000-7000-8000-000000000907") // giảng viên đã bị vô hiệu hóa
	basic02ID      = uuid.MustParse("01990000-0000-7000-8000-000000000951") // active, Lê Thu Hương, chưa có thành viên
	basic03ID      = uuid.MustParse("01990000-0000-7000-8000-000000000952") // draft, Phạm Quốc Bảo
	basicV2DraftID = uuid.MustParse("01990000-0000-7000-8000-000000000961")
	basicV3ID      = uuid.MustParse("01990000-0000-7000-8000-000000000962") // published, chưa có chặng
	basicV0ArchID  = uuid.MustParse("01990000-0000-7000-8000-000000000963")
)

var tempPasswordLine = regexp.MustCompile(`Mật khẩu tạm: (\S+)`)

// syncBuffer là bộ đệm log an toàn khi nhiều goroutine cùng ghi.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureSender giữ email đã gửi để đọc mật khẩu tạm như người dùng đọc hộp thư.
type captureSender struct {
	mu   sync.Mutex
	sent []mailer.RenderedMail
}

func (s *captureSender) Send(_ context.Context, m mailer.RenderedMail) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, m)
	return nil
}

// failingEnqueuer giả outbox hỏng để kiểm transaction mời rollback toàn bộ.
type failingEnqueuer struct{}

func (failingEnqueuer) Enqueue(context.Context, db.Executor, mailer.Message) (uuid.UUID, error) {
	return uuid.Nil, errors.New("outbox không ghi được")
}

// itEnv dựng classes trên Postgres thật cùng identity, outbox và audit thật, và router /api/v1 với actor đổi được.
type itEnv struct {
	t       *testing.T
	dbx     *sqlx.DB
	clk     *clock.Fake
	box     *secretbox.Box
	ident   *identity.Service
	mail    *mailer.Service
	svc     *Service
	logs    *syncBuffer
	engine  *gin.Engine
	actor   Actor
	secrets []string // mật khẩu tạm đọc từ email; không được lộ ở response, log, audit, payload
	bodies  []string // mọi response HTTP của test
}

func newIT(t *testing.T) *itEnv {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	testdb.Fixture(t, dbx, "class_basic01_active")

	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	prev := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(prev) })

	box, err := mailer.NewSecretBox(integrationKey)
	if err != nil {
		t.Fatal(err)
	}
	// Đồng hồ giả lùi 2 giờ so với giờ thật: câu claim outbox so run_at với now() của Postgres, test tua tới được
	// mà email vẫn tới hạn gửi.
	clk := &clock.Fake{T: time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)}
	mail := mailer.NewService(mailer.NewOutboxRepo(), box, clk)
	ident := identity.NewService(identity.ServiceDeps{
		DB: dbx, Tx: db.TxRunner{DB: dbx},
		Users: identity.PGUserRepo{}, Sessions: identity.PGSessionRepo{}, Attempts: identity.PGLoginAttemptRepo{},
		Resets: identity.PGResetTokenRepo{}, Mailer: mail, Clock: clk, Audit: audit.PG{Clock: clk},
		Cfg: config.Config{
			SessionTTL: 12 * time.Hour, TempPasswordTTL: 72 * time.Hour, ResetTokenTTL: 30 * time.Minute,
			PasswordMinLength: 8, LoginMaxFailures: 5, LoginLockWindow: 15 * time.Minute,
			PublicBaseURL: "http://localhost:5173", AppEnv: config.EnvE2E,
		},
	})
	e := &itEnv{t: t, dbx: dbx, clk: clk, box: box, ident: ident, mail: mail, logs: logs,
		actor: Actor{ID: adminID, Role: domain.RoleAdmin, RequestID: "it"}}
	e.svc = e.service(mail)
	e.seedExtra()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Logger(logger), middleware.Recover(logger))
	pass := func(c *gin.Context) { c.Next() }
	NewHandler(e.svc, func(*gin.Context) (uuid.UUID, domain.Role, bool) { return e.actor.ID, e.actor.Role, true }).
		Register(engine.Group("/api/v1"), Guards{
			Admin: e.requireRole(domain.RoleAdmin), Staff: e.requireRole(domain.RoleAdmin, domain.RoleTeacher),
			Teacher: e.requireRole(domain.RoleTeacher), InviteLimit: pass,
		})
	e.engine = engine
	return e
}

func (e *itEnv) service(enq mailer.Enqueuer) *Service {
	return NewService(Deps{
		DB: e.dbx, Tx: db.TxRunner{DB: e.dbx},
		Classes: PGClassRepo{}, Members: PGMemberRepo{}, Invitations: PGInvitationRepo{},
		Users: identity.PGUserRepo{}, Provisioner: e.ident, Locker: identity.PGLoginAttemptRepo{},
		Versions: courses.NewService(courses.Deps{
			DB: e.dbx, Tx: db.TxRunner{DB: e.dbx}, Courses: courses.PGCourseRepo{}, Versions: courses.PGCourseVersionRepo{},
			Clock: e.clk, Audit: audit.PG{Clock: e.clk}, IDs: ids.V7{},
		}),
		Mail: enq, Outbox: mailer.NewOutboxRepo(), Progress: avgProgress{}, Clock: e.clk, Audit: audit.PG{Clock: e.clk},
		IDs: ids.V7{}, PublicBaseURL: "http://localhost:5173", StaleDays: 7,
	})
}

// avgProgress là ProgressReader tối giản trên lesson_progress, đủ để kiểm avgPercent đi tới response.
type avgProgress struct{}

func (avgProgress) AvgPercentByClass(ctx context.Context, ex db.Executor, classIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	out := map[uuid.UUID]int{}
	for _, id := range classIDs {
		var n int
		if err := sqlx.GetContext(ctx, ex, &n, `SELECT count(*) FROM lesson_progress lp
			JOIN class_members cm ON cm.id = lp.class_member_id WHERE cm.class_id = $1 AND lp.completed_at IS NOT NULL`, id); err != nil {
			return nil, err
		}
		if n > 0 {
			out[id] = 50
		}
	}
	return out, nil
}

func (e *itEnv) requireRole(roles ...domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, r := range roles {
			if e.actor.Role == r {
				c.Next()
				return
			}
		}
		httpx.Fail(c, domain.ErrForbidden)
		c.Abort()
	}
}

func (e *itEnv) as(id uuid.UUID, role domain.Role) {
	e.actor = Actor{ID: id, Role: role, RequestID: "it"}
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

func (e *itEnv) str(q string, args ...any) string {
	e.t.Helper()
	var s string
	if err := e.dbx.Get(&s, q, args...); err != nil {
		e.t.Fatal(err)
	}
	return s
}

// seedExtra chèn người dùng, lớp và phiên bản khóa học theo seed.js mà fixture chung chưa có.
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
		{minhID, "minh.bui@gmail.com", "Bùi Quang Minh", "student", "invited", true, ptr(now.Add(48 * time.Hour)), nil},
		{thaoID, "thao.vo@gmail.com", "Võ Phương Thảo", "student", "disabled", false, nil, ptr(now.Add(-15 * 24 * time.Hour))},
		{khangID, "khang.vu@gmail.com", "Vũ Đức Khang", "student", "active", false, nil, ptr(now.Add(-21 * 24 * time.Hour))},
		{bichID, "bich.tran@gmail.com", "Trần Thị Bích", "student", "active", false, nil, ptr(now.Add(-48 * time.Hour))},
		{dungID, "dung.pham@gmail.com", "Phạm Minh Dũng", "student", "invited", true, ptr(now.Add(-55 * 24 * time.Hour)), nil},
		{teacherOffID, "giang.vien.nghi@goup.vn", "Giảng Viên Nghỉ", "teacher", "disabled", false, nil, nil},
	}
	for _, u := range users {
		var disabledAt *time.Time
		if u.status == "disabled" {
			disabledAt = ptr(now.Add(-15 * 24 * time.Hour))
		}
		e.exec(`INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash, must_change_password,
			temp_password_expires_at, last_login_at, last_active_at, disabled_at)
			VALUES ($1, $2, $2, $3, $4, $5, $6, $7, $8, $9, $9, $10)`,
			u.id, u.email, u.name, u.role, u.status, hash, u.mustChange, u.tempExpires, u.lastActive, disabledAt)
	}
	e.exec(`INSERT INTO course_versions (id, course_id, version_no, status, title, published_at, archived_at, created_by) VALUES
		($1, $4, 2, 'draft', 'Lập trình cơ bản', NULL, NULL, $5),
		($2, $4, 3, 'published', 'Lập trình cơ bản', now(), NULL, $5),
		($3, $4, 4, 'archived', 'Lập trình cơ bản', now(), now(), $5)`,
		basicV2DraftID, basicV3ID, basicV0ArchID, uuid.MustParse(testdb.CourseBasicID), adminID)
	e.exec(`INSERT INTO classes (id, code, name, course_version_id, teacher_id, status, start_date, end_date, created_by) VALUES
		($1, 'basic02', 'Lập trình cơ bản – khóa 2', $3, $4, 'active', DATE '2026-09-01', DATE '2027-01-15', $6),
		($2, 'basic03', 'Lập trình cơ bản – khóa 3', $3, $5, 'draft', DATE '2026-11-01', DATE '2027-03-01', $6)`,
		basic02ID, basic03ID, basicV1ID, huongID, baoID, adminID)
}

func ptr[T any](v T) *T { return &v }

type apiResp struct {
	code int
	body []byte
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
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	e.bodies = append(e.bodies, w.Body.String())
	return apiResp{code: w.Code, body: w.Body.Bytes()}
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

func classPath(id uuid.UUID, suffix string) string { return "/api/v1/classes/" + id.String() + suffix }

func inviteBody(email, name string) string {
	b, _ := json.Marshal(map[string]string{"email": email, "fullName": name})
	return string(b)
}

func (e *itEnv) invite(classID uuid.UUID, email, name string) InviteResponse {
	e.t.Helper()
	var out InviteResponse
	if err := json.Unmarshal(e.must(http.MethodPost, classPath(classID, "/invitations"), inviteBody(email, name), http.StatusCreated), &out); err != nil {
		e.t.Fatal(err)
	}
	return out
}

func (e *itEnv) members(classID uuid.UUID, query string) []MemberDTO {
	e.t.Helper()
	var out ListResponse[MemberDTO]
	if err := json.Unmarshal(e.must(http.MethodGet, classPath(classID, "/members"+query), "", http.StatusOK), &out); err != nil {
		e.t.Fatal(err)
	}
	return out.Items
}

// deliver chạy worker một lượt; trả email đã gửi và ghi nhớ mật khẩu tạm để kiểm không bị lộ.
func (e *itEnv) deliver() []mailer.RenderedMail {
	e.t.Helper()
	renderer, err := mailer.NewRenderer(time.UTC)
	if err != nil {
		e.t.Fatal(err)
	}
	sender := &captureSender{}
	w, err := mailer.NewWorker(mailer.WorkerConfig{
		Tx: db.TxRunner{DB: e.dbx}, Repo: mailer.NewOutboxRepo(), Renderer: renderer, Sender: sender, Box: e.box,
		Clock: e.clk, PublicBaseURL: "http://localhost:5173", Logger: slog.Default(),
	})
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := w.RunOnce(e.ctx()); err != nil {
		e.t.Fatal(err)
	}
	for _, m := range sender.sent {
		if p := tempPasswordLine.FindStringSubmatch(m.Text); p != nil {
			e.secrets = append(e.secrets, p[1])
		}
	}
	return sender.sent
}

// tempPassword lấy mật khẩu tạm trong email gửi tới addr (lần gửi cuối).
func tempPassword(t *testing.T, sent []mailer.RenderedMail, addr string) string {
	t.Helper()
	pw := ""
	for _, m := range sent {
		if p := tempPasswordLine.FindStringSubmatch(m.Text); p != nil && strings.EqualFold(m.To, addr) {
			pw = p[1]
		}
	}
	if pw == "" {
		t.Fatalf("không có email mang mật khẩu tạm tới %s trong %d email", addr, len(sent))
	}
	return pw
}

// assertNoSecretLeak kiểm mật khẩu tạm không có trong response, log, audit hay payload outbox.
func (e *itEnv) assertNoSecretLeak() {
	e.t.Helper()
	if len(e.secrets) == 0 {
		e.t.Fatal("test chưa đọc được mật khẩu tạm nào")
	}
	auditText := e.str(`SELECT coalesce(string_agg(before::text || ' ' || after::text, ' '), '') FROM audit_logs`)
	payloads := e.str(`SELECT coalesce(string_agg(payload::text, ' '), '') FROM email_outbox`)
	logs := e.logs.String()
	responses := strings.Join(e.bodies, "\n")
	for _, s := range e.secrets {
		for where, text := range map[string]string{"response": responses, "log": logs, "audit": auditText, "payload": payloads} {
			if strings.Contains(text, s) {
				e.t.Fatalf("mật khẩu tạm lộ trong %s", where)
			}
		}
	}
}

func (e *itEnv) passwordHash(id uuid.UUID) string {
	return e.str(`SELECT password_hash FROM users WHERE id = $1`, id)
}

func TestInviteNewEmailCreatesInvitedStudent(t *testing.T) {
	e := newIT(t)
	res := e.invite(basic01ID, "  Moi.Hoc.Vien@Gmail.com ", "Học Viên Mới")
	if res.Kind != "invited" || res.Member.InviteStatus == nil || *res.Member.InviteStatus != "queued" ||
		res.Member.Email != "moi.hoc.vien@gmail.com" || res.Member.AccountStatus != "invited" || res.Member.TempPasswordExpiresAt == nil {
		t.Fatalf("invite = %+v", res)
	}
	var u struct {
		Role       string    `db:"role"`
		Status     string    `db:"status"`
		MustChange bool      `db:"must_change_password"`
		Expires    time.Time `db:"temp_password_expires_at"`
	}
	if err := e.dbx.Get(&u, `SELECT role, status, must_change_password, temp_password_expires_at FROM users
		WHERE email_normalized = 'moi.hoc.vien@gmail.com'`); err != nil {
		t.Fatal(err)
	}
	if u.Role != "student" || u.Status != "invited" || !u.MustChange || !u.Expires.Equal(e.clk.Now().Add(72*time.Hour)) {
		t.Fatalf("user = %+v", u)
	}
	if n := e.count(`SELECT count(*) FROM class_members cm JOIN users u ON u.id = cm.user_id
		WHERE cm.class_id = $1 AND u.email_normalized = 'moi.hoc.vien@gmail.com' AND cm.status = 'active'`, basic01ID); n != 1 {
		t.Fatalf("thành viên active = %d", n)
	}
	if k := e.str(`SELECT i.kind FROM invitations i JOIN users u ON u.id = i.user_id WHERE u.email_normalized = 'moi.hoc.vien@gmail.com'`); k != "invite" {
		t.Fatalf("invitations.kind = %s", k)
	}
	if n := e.count(`SELECT count(*) FROM email_outbox WHERE to_email = 'moi.hoc.vien@gmail.com' AND template = 'invite'
		AND status = 'queued' AND secret_enc IS NOT NULL AND NOT payload ?| array['TempPassword', 'Password', 'Secret']`); n != 1 {
		t.Fatalf("outbox invite queued có secret = %d", n)
	}

	sent := e.deliver()
	if len(sent) != 1 || sent[0].Subject != "[GoUp LMS] Lời mời vào lớp basic01" {
		t.Fatalf("email = %+v", sent)
	}
	pw := tempPassword(t, sent, "moi.hoc.vien@gmail.com")
	login, err := e.ident.Login(e.ctx(), "MOI.HOC.VIEN@gmail.com", pw, "10.0.0.1", "test")
	if err != nil || !login.User.MustChangePassword() {
		t.Fatalf("Login bằng mật khẩu tạm: %v", err)
	}
	m := e.members(basic01ID, "")
	var got *MemberDTO
	for i := range m {
		if m[i].Email == "moi.hoc.vien@gmail.com" {
			got = &m[i]
		}
	}
	// attempts chỉ đếm lần gửi thất bại nên gửi thành công ngay lần đầu vẫn là 0.
	if got == nil || got.InviteStatus == nil || *got.InviteStatus != "sent" || got.InviteAttempts == nil || *got.InviteAttempts != 0 {
		t.Fatalf("thành viên sau khi gửi = %+v", got)
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = $1 AND target_id = $2`, audit.ActionClassMemberInvited, basic01ID); n != 1 {
		t.Fatalf("audit member_invited = %d", n)
	}
	e.assertNoSecretLeak()
}

func TestInviteExistingAccounts(t *testing.T) {
	e := newIT(t)

	// Học viên active → added, template added, không đụng mật khẩu.
	anHash := e.passwordHash(anID)
	res := e.invite(basic02ID, "An.Nguyen@Gmail.com", "")
	if res.Kind != "added" || res.Member.UserID != anID.String() || res.Member.TempPasswordExpiresAt != nil {
		t.Fatalf("active = %+v", res)
	}
	if e.passwordHash(anID) != anHash {
		t.Fatal("mời học viên active không được đổi mật khẩu")
	}
	if tpl := e.str(`SELECT template FROM email_outbox WHERE to_email = 'an.nguyen@gmail.com'`); tpl != "added" {
		t.Fatalf("template = %s", tpl)
	}

	// Invited còn hạn mật khẩu tạm → added, giữ mật khẩu, payload báo dùng mật khẩu đã gửi.
	minhHash := e.passwordHash(minhID)
	res = e.invite(basic01ID, "minh.bui@gmail.com", "")
	if res.Kind != "added" || res.Member.TempPasswordExpiresAt == nil {
		t.Fatalf("invited còn hạn = %+v", res)
	}
	if e.passwordHash(minhID) != minhHash {
		t.Fatal("mật khẩu tạm còn hạn không được xoay")
	}
	if n := e.count(`SELECT count(*) FROM email_outbox WHERE to_email = 'minh.bui@gmail.com' AND template = 'added'
		AND secret_enc IS NULL AND payload->>'UsePreviousTempPassword' = 'true'`); n != 1 {
		t.Fatalf("outbox added cho invited còn hạn = %d", n)
	}

	// Invited hết hạn → xoay mật khẩu, template invite; email mời cũ còn chờ gửi bị hủy.
	e.invite(basic03ID, "Moi.Hoc.Vien@Gmail.com", "Học Viên Mới")
	newID := uuid.MustParse(e.str(`SELECT id::text FROM users WHERE email_normalized = 'moi.hoc.vien@gmail.com'`))
	oldHash := e.passwordHash(newID)
	e.clk.Advance(73 * time.Hour)
	res = e.invite(basic01ID, "moi.hoc.vien@gmail.com", "")
	if res.Kind != "invited" || res.Member.InviteKind == nil || *res.Member.InviteKind != "invite" {
		t.Fatalf("invited hết hạn = %+v", res)
	}
	if e.passwordHash(newID) == oldHash {
		t.Fatal("mật khẩu tạm hết hạn phải được xoay")
	}
	if n := e.count(`SELECT count(*) FROM email_outbox WHERE to_email = 'moi.hoc.vien@gmail.com'
		AND status = 'failed' AND last_error = 'superseded' AND secret_enc IS NULL`); n != 1 {
		t.Fatalf("hàng queued cũ superseded = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM email_outbox WHERE to_email = 'moi.hoc.vien@gmail.com' AND status = 'queued'
		AND template = 'invite'`); n != 1 {
		t.Fatalf("hàng invite mới queued = %d", n)
	}
	// Mật khẩu tạm hết hạn từ seed (dung.pham) cũng xoay được.
	if res := e.invite(basic02ID, "dung.pham@gmail.com", ""); res.Kind != "invited" {
		t.Fatalf("dung.pham = %+v", res)
	}
}

func TestInviteErrors(t *testing.T) {
	e := newIT(t)
	path := classPath(basic01ID, "/invitations")
	e.expectError(http.MethodPost, path, inviteBody("an.nguyen@gmail.com", ""), http.StatusConflict, "CONFLICT", "Học viên đã có trong lớp.")
	e.expectError(http.MethodPost, path, inviteBody("huong.le@goup.vn", ""), http.StatusForbidden, "FORBIDDEN",
		"Email này thuộc tài khoản nội bộ, không mời làm học viên được.")
	e.expectError(http.MethodPost, path, inviteBody("thao.vo@gmail.com", ""), http.StatusConflict, "ACCOUNT_DISABLED", "")
	e.expectError(http.MethodPost, path, inviteBody("khong-hop-le", "A"), http.StatusUnprocessableEntity, "VALIDATION_FAILED", "")
	e.expectError(http.MethodPost, path, inviteBody("moi@gmail.com", " "), http.StatusUnprocessableEntity, "VALIDATION_FAILED", "")
	e.expectError(http.MethodPost, classPath(uuid.New(), "/invitations"), inviteBody("moi@gmail.com", "A"), http.StatusNotFound, "NOT_FOUND", "")

	if _, err := e.svc.End(e.ctx(), e.actor, basic02ID); err != nil {
		t.Fatal(err)
	}
	e.expectError(http.MethodPost, classPath(basic02ID, "/invitations"), inviteBody("bich.tran@gmail.com", ""),
		http.StatusConflict, "INVALID_TRANSITION", "Không mời được vào lớp đã kết thúc.")
	if n := e.count(`SELECT count(*) FROM invitations`); n != 0 {
		t.Fatalf("lời mời lỗi không được ghi: %d", n)
	}

	// Học viên không mời được lớp: guard admin chặn trước service.
	e.as(anID, domain.RoleStudent)
	e.expectError(http.MethodPost, path, inviteBody("bich.tran@gmail.com", ""), http.StatusForbidden, "FORBIDDEN", "")
}

func TestRemoveAndRejoinKeepsProgress(t *testing.T) {
	e := newIT(t)
	e.exec(`INSERT INTO lesson_progress (class_member_id, lesson_id, completed_at) VALUES ($1, $2, now())`, memberAnID, lessonID)
	joined := e.str(`SELECT joined_at::text FROM class_members WHERE id = $1`, memberAnID)

	var dropped MemberDTO
	if err := json.Unmarshal(e.must(http.MethodDelete, classPath(basic01ID, "/members/"+memberAnID.String()), "", http.StatusOK), &dropped); err != nil {
		t.Fatal(err)
	}
	if dropped.MemberStatus != "dropped" || dropped.DroppedAt == nil {
		t.Fatalf("gỡ = %+v", dropped)
	}
	e.expectError(http.MethodDelete, classPath(basic01ID, "/members/"+memberAnID.String()), "", http.StatusConflict, "INVALID_TRANSITION", "Học viên đã rời lớp.")
	if n := e.count(`SELECT count(*) FROM lesson_progress WHERE class_member_id = $1`, memberAnID); n != 1 {
		t.Fatalf("tiến độ sau khi gỡ = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = $1`, audit.ActionClassMemberDropped); n != 1 {
		t.Fatalf("audit member_dropped = %d", n)
	}

	// Danh sách mặc định ẩn dropped, includeDropped=true hiện.
	if m := e.members(basic01ID, ""); len(m) != 0 {
		t.Fatalf("mặc định = %+v", m)
	}
	if m := e.members(basic01ID, "?includeDropped=true"); len(m) != 1 || m[0].MemberStatus != "dropped" {
		t.Fatalf("includeDropped = %+v", m)
	}

	res := e.invite(basic01ID, "an.nguyen@gmail.com", "")
	if res.Kind != "added" || res.Member.ID != memberAnID.String() || res.Member.MemberStatus != "active" || res.Member.DroppedAt != nil {
		t.Fatalf("mời lại = %+v", res)
	}
	if got := e.str(`SELECT joined_at::text FROM class_members WHERE id = $1`, memberAnID); got != joined {
		t.Fatalf("joined_at = %s, muốn giữ %s", got, joined)
	}
	if n := e.count(`SELECT count(*) FROM lesson_progress WHERE class_member_id = $1`, memberAnID); n != 1 {
		t.Fatalf("tiến độ sau khi quay lại = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM class_members WHERE class_id = $1`, basic01ID); n != 1 {
		t.Fatalf("quay lại không được tạo hàng thành viên mới: %d", n)
	}

	// Lớp đã kết thúc chỉ xem: không gỡ được thành viên.
	if _, err := e.svc.End(e.ctx(), e.actor, basic01ID); err != nil {
		t.Fatal(err)
	}
	e.expectError(http.MethodDelete, classPath(basic01ID, "/members/"+memberAnID.String()), "", http.StatusConflict, "INVALID_TRANSITION",
		"Lớp đã kết thúc, không sửa được.")
}

func TestResendRotatesPasswordWithLimit(t *testing.T) {
	e := newIT(t)
	res := e.invite(basic01ID, "moi.hoc.vien@gmail.com", "Học Viên Mới")
	userID := uuid.MustParse(res.Member.UserID)
	resendPath := classPath(basic01ID, "/members/"+res.Member.ID+"/resend")
	first := tempPassword(t, e.deliver(), "moi.hoc.vien@gmail.com")
	hash0 := e.passwordHash(userID)

	e.clk.Advance(10 * time.Minute)
	var out MemberResponse
	if err := json.Unmarshal(e.must(http.MethodPost, resendPath, "", http.StatusOK), &out); err != nil {
		t.Fatal(err)
	}
	if out.Member.InviteKind == nil || *out.Member.InviteKind != "resend" || *out.Member.InviteStatus != "queued" ||
		out.Member.TempPasswordExpiresAt == nil || !out.Member.TempPasswordExpiresAt.Equal(e.clk.Now().Add(72*time.Hour)) {
		t.Fatalf("resend = %+v", out.Member)
	}
	if e.passwordHash(userID) == hash0 {
		t.Fatal("resend phải đổi hash")
	}
	// Lần gửi lại thứ hai trước khi worker chạy: email gửi lại đầu tiên còn queued → superseded.
	e.must(http.MethodPost, resendPath, "", http.StatusOK)
	if n := e.count(`SELECT count(*) FROM email_outbox WHERE to_email = 'moi.hoc.vien@gmail.com' AND template = 'resend'
		AND status = 'failed' AND last_error = 'superseded'`); n != 1 {
		t.Fatalf("resend queued cũ superseded = %d", n)
	}
	sent := e.deliver()
	if len(sent) != 1 || sent[0].Subject != "[GoUp LMS] Mật khẩu tạm mới cho lớp basic01" {
		t.Fatalf("email gửi lại = %+v", sent)
	}
	latest := tempPassword(t, sent, "moi.hoc.vien@gmail.com")
	if latest == first {
		t.Fatal("mật khẩu tạm mới phải khác")
	}
	if _, err := e.ident.Login(e.ctx(), "moi.hoc.vien@gmail.com", first, "10.0.0.1", "test"); !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("mật khẩu cũ: %v", err)
	}
	if _, err := e.ident.Login(e.ctx(), "moi.hoc.vien@gmail.com", latest, "10.0.0.1", "test"); err != nil {
		t.Fatalf("mật khẩu mới: %v", err)
	}

	e.must(http.MethodPost, resendPath, "", http.StatusOK)
	e.expectError(http.MethodPost, resendPath, "", http.StatusTooManyRequests, "RATE_LIMITED", "Đã gửi lại quá nhiều lần. Thử lại sau.")
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = $1`, audit.ActionClassInvitationResent); n != 3 {
		t.Fatalf("audit invitation_resent = %d", n)
	}
	// Hết cửa sổ 1 giờ thì gửi lại được.
	e.clk.Advance(61 * time.Minute)
	e.must(http.MethodPost, resendPath, "", http.StatusOK)

	// Học viên đã đổi mật khẩu → 409.
	e.invite(basic01ID, "minh.bui@gmail.com", "")
	login, err := e.ident.Login(e.ctx(), "minh.bui@gmail.com", testdb.FixturePassword, "10.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	current := testdb.FixturePassword
	if _, err := e.ident.ChangePassword(e.ctx(), login.User, login.Session.ID, identity.ChangePasswordCmd{
		CurrentPassword: &current, NewPassword: "mat-khau-moi-2026", ConfirmPassword: "mat-khau-moi-2026",
	}); err != nil {
		t.Fatal(err)
	}
	minhMember := e.str(`SELECT id::text FROM class_members WHERE class_id = $1 AND user_id = $2`, basic01ID, minhID)
	e.expectError(http.MethodPost, classPath(basic01ID, "/members/"+minhMember+"/resend"), "", http.StatusConflict, "CONFLICT", "")

	// Học viên active và thành viên của lớp khác.
	e.expectError(http.MethodPost, classPath(basic01ID, "/members/"+memberAnID.String()+"/resend"), "", http.StatusConflict, "CONFLICT", "")
	e.expectError(http.MethodPost, classPath(basic02ID, "/members/"+memberAnID.String()+"/resend"), "", http.StatusNotFound, "NOT_FOUND", "")
	e.assertNoSecretLeak()
}

func TestInviteRollsBackWhenOutboxFails(t *testing.T) {
	e := newIT(t)
	_, err := e.service(failingEnqueuer{}).Invite(e.ctx(), e.actor, basic01ID, InviteCmd{Email: "rollback@gmail.com", FullName: "Rollback"})
	if err == nil {
		t.Fatal("Invite phải lỗi khi outbox lỗi")
	}
	if n := e.count(`SELECT count(*) FROM users WHERE email_normalized = 'rollback@gmail.com'`); n != 0 {
		t.Fatalf("user còn sau rollback: %d", n)
	}
	if n := e.count(`SELECT (SELECT count(*) FROM class_members WHERE class_id = $1) + (SELECT count(*) FROM invitations)
		+ (SELECT count(*) FROM audit_logs WHERE action = $2)`, basic01ID, audit.ActionClassMemberInvited); n != 1 {
		t.Fatalf("chỉ còn thành viên fixture, có %d hàng", n)
	}
}

func TestConcurrentInvitesSameEmail(t *testing.T) {
	e := newIT(t)
	const n = 10
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			_, errs[i] = e.svc.Invite(e.ctx(), e.actor, basic01ID, InviteCmd{Email: "Song.Song@Gmail.com", FullName: "Song Song"})
		})
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrAlreadyMember):
			conflict++
		default:
			t.Fatalf("lỗi lạ: %v", err)
		}
	}
	if ok != 1 || conflict != n-1 {
		t.Fatalf("thành công %d, trùng %d", ok, conflict)
	}
	if u := e.count(`SELECT count(*) FROM users WHERE email_normalized = 'song.song@gmail.com'`); u != 1 {
		t.Fatalf("users = %d", u)
	}
	if m := e.count(`SELECT count(*) FROM class_members cm JOIN users u ON u.id = cm.user_id WHERE u.email_normalized = 'song.song@gmail.com'`); m != 1 {
		t.Fatalf("class_members = %d", m)
	}
}

func TestMembersListShape(t *testing.T) {
	e := newIT(t)
	// An không có lời mời nào → các field invite* vắng.
	m := e.members(basic01ID, "")
	if len(m) != 1 || m[0].InviteStatus != nil || m[0].InviteKind != nil || m[0].InvitedAt != nil || m[0].TempPasswordExpiresAt != nil {
		t.Fatalf("thành viên không có lời mời = %+v", m)
	}
	raw := string(e.must(http.MethodGet, classPath(basic01ID, "/members"), "", http.StatusOK))
	if strings.Contains(raw, "inviteStatus") {
		t.Fatalf("inviteStatus phải vắng: %s", raw)
	}
	e.expectError(http.MethodGet, classPath(basic01ID, "/members?includeDropped=co"), "", http.StatusUnprocessableEntity, "VALIDATION_FAILED", "")
}

func TestCreateAndUpdateClass(t *testing.T) {
	e := newIT(t)
	body := func(code, version, start, end, teacher string) string {
		b, _ := json.Marshal(map[string]string{"code": code, "name": "Lập trình cơ bản 04", "courseVersionId": version,
			"startDate": start, "endDate": end, "teacherId": teacher})
		return string(b)
	}
	v1, h := basicV1ID.String(), huongID.String()
	e.expectError(http.MethodPost, "/api/v1/classes", body("basic04", v1, "2026-11-01", "2026-10-01", h), 422, "VALIDATION_FAILED",
		"Ngày kết thúc phải sau ngày bắt đầu.")
	e.expectError(http.MethodPost, "/api/v1/classes", body("BASIC04", v1, "2026-11-01", "2027-03-01", h), 422, "VALIDATION_FAILED", "")
	for _, v := range []uuid.UUID{basicV2DraftID, basicV0ArchID, uuid.New()} {
		e.expectError(http.MethodPost, "/api/v1/classes", body("basic04", v.String(), "2026-11-01", "2027-03-01", h), 422, "VALIDATION_FAILED",
			"Chọn một phiên bản khóa học đã phát hành.")
	}
	for _, teacher := range []uuid.UUID{teacherOffID, anID} {
		e.expectError(http.MethodPost, "/api/v1/classes", body("basic04", v1, "2026-11-01", "2027-03-01", teacher.String()), 422,
			"VALIDATION_FAILED", "Giảng viên không hợp lệ hoặc đã bị vô hiệu hóa.")
	}
	e.expectError(http.MethodPost, "/api/v1/classes", body("basic01", v1, "2026-11-01", "2027-03-01", h), 409, "CONFLICT", "Mã lớp đã tồn tại.")

	var d ClassDetailDTO
	if err := json.Unmarshal(e.must(http.MethodPost, "/api/v1/classes", body("basic04", v1, "2026-11-01", "2027-03-01", h), 201), &d); err != nil {
		t.Fatal(err)
	}
	if d.Status != "draft" || d.Teacher.ID != h || d.CourseVersion.ID != v1 || d.CourseVersion.StageCount != 1 ||
		d.CourseVersion.LessonCount != 2 || d.MemberCount != 0 || d.ActivatedAt != nil || d.StartDate != "2026-11-01" {
		t.Fatalf("tạo lớp = %+v", d)
	}
	id := uuid.MustParse(d.ID)
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = $1 AND target_id = $2`, audit.ActionClassCreated, id); n != 1 {
		t.Fatalf("audit class.created = %d", n)
	}

	// Đổi phiên bản khi còn nháp: chỉ sang bản đã phát hành, có audit.
	patch := func(field, value string) string {
		b, _ := json.Marshal(map[string]string{field: value})
		return string(b)
	}
	e.expectError(http.MethodPatch, classPath(id, ""), patch("courseVersionId", basicV2DraftID.String()), 422, "VALIDATION_FAILED",
		"Chỉ đổi sang phiên bản đã phát hành.")
	if err := json.Unmarshal(e.must(http.MethodPatch, classPath(id, ""), patch("courseVersionId", basicV3ID.String()), 200), &d); err != nil {
		t.Fatal(err)
	}
	if d.CourseVersion.ID != basicV3ID.String() || d.CourseVersion.VersionNo != 3 || d.CourseVersion.StageCount != 0 {
		t.Fatalf("đổi phiên bản = %+v", d.CourseVersion)
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = $1 AND target_id = $2
		AND before->>'courseVersionId' = $3 AND after->>'courseVersionId' = $4`,
		audit.ActionClassCourseVersionChanged, id, v1, basicV3ID.String()); n != 1 {
		t.Fatalf("audit course_version_changed = %d", n)
	}
	e.expectError(http.MethodPatch, classPath(id, ""), patch("endDate", "2026-10-01"), 422, "VALIDATION_FAILED", "Ngày kết thúc phải sau ngày bắt đầu.")
	if err := json.Unmarshal(e.must(http.MethodPatch, classPath(id, ""), `{"name":"Lớp 04 mới","endDate":"2027-04-01","teacherId":"`+baoID.String()+`"}`, 200), &d); err != nil {
		t.Fatal(err)
	}
	if d.Name != "Lớp 04 mới" || d.EndDate != "2027-04-01" || d.Teacher.ID != baoID.String() || d.Teacher.Name != "Phạm Quốc Bảo" {
		t.Fatalf("sửa cài đặt = %+v", d)
	}

	// Một chiều: draft → end sai; activate; activate lại sai; đổi phiên bản khi active sai; end; sửa khi ended sai.
	oneWay := "Chỉ chuyển trạng thái một chiều: nháp → đang chạy → đã kết thúc."
	e.expectError(http.MethodPost, classPath(id, "/end"), "", 409, "INVALID_TRANSITION", oneWay)
	if err := json.Unmarshal(e.must(http.MethodPost, classPath(id, "/activate"), "", 200), &d); err != nil {
		t.Fatal(err)
	}
	if d.Status != "active" || d.ActivatedAt == nil || d.EndedAt != nil {
		t.Fatalf("activate = %+v", d)
	}
	e.expectError(http.MethodPost, classPath(id, "/activate"), "", 409, "INVALID_TRANSITION", oneWay)
	e.expectError(http.MethodPatch, classPath(id, ""), patch("courseVersionId", v1), 409, "INVALID_TRANSITION", "Chỉ đổi phiên bản khi lớp còn nháp.")
	if err := json.Unmarshal(e.must(http.MethodPost, classPath(id, "/end"), "", 200), &d); err != nil {
		t.Fatal(err)
	}
	if d.Status != "ended" || d.EndedAt == nil {
		t.Fatalf("end = %+v", d)
	}
	e.expectError(http.MethodPost, classPath(id, "/end"), "", 409, "INVALID_TRANSITION", oneWay)
	e.expectError(http.MethodPatch, classPath(id, ""), patch("name", "x"), 409, "INVALID_TRANSITION", "Lớp đã kết thúc, không sửa được.")
	for action, after := range map[string]string{audit.ActionClassActivated: "active", audit.ActionClassEnded: "ended"} {
		if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = $1 AND target_id = $2 AND after->>'status' = $3`, action, id, after); n != 1 {
			t.Fatalf("audit %s = %d", action, n)
		}
	}
}

func TestTeacherScope(t *testing.T) {
	e := newIT(t)
	e.exec(`INSERT INTO class_members (id, class_id, user_id, status, joined_at) VALUES
		('01990000-0000-7000-8000-000000000971', $1, $2, 'active', now() - interval '30 days'),
		('01990000-0000-7000-8000-000000000972', $1, $3, 'active', now() - interval '3 days')`, basic01ID, khangID, minhID)
	e.exec(`INSERT INTO lesson_progress (class_member_id, lesson_id, completed_at) VALUES ($1, $2, now())`, memberAnID, lessonID)

	e.as(baoID, domain.RoleTeacher)
	e.expectError(http.MethodGet, classPath(basic01ID, ""), "", http.StatusForbidden, "FORBIDDEN", "Bạn chỉ xem được lớp mình phụ trách.")
	e.expectError(http.MethodGet, classPath(basic01ID, "/members"), "", http.StatusForbidden, "FORBIDDEN", "Bạn chỉ xem được lớp mình phụ trách.")
	e.expectError(http.MethodGet, "/api/v1/classes", "", http.StatusForbidden, "FORBIDDEN", "")
	e.must(http.MethodGet, classPath(basic03ID, ""), "", http.StatusOK)
	var bao ListResponse[TeachingClassItem]
	if err := json.Unmarshal(e.must(http.MethodGet, "/api/v1/teach/classes", "", http.StatusOK), &bao); err != nil {
		t.Fatal(err)
	}
	if len(bao.Items) != 1 || bao.Items[0].ID != basic03ID.String() || bao.Items[0].AvgPercent != nil {
		t.Fatalf("lớp của Bảo = %+v", bao.Items)
	}

	e.as(huongID, domain.RoleTeacher)
	var huong ListResponse[TeachingClassItem]
	if err := json.Unmarshal(e.must(http.MethodGet, "/api/v1/teach/classes", "", http.StatusOK), &huong); err != nil {
		t.Fatal(err)
	}
	byID := map[string]TeachingClassItem{}
	for _, it := range huong.Items {
		byID[it.ID] = it
	}
	b1, ok := byID[basic01ID.String()]
	if len(huong.Items) != 2 || !ok || b1.MemberCount != 3 || b1.NotLoggedIn != 1 || b1.InactiveOver7Days != 2 ||
		b1.AvgPercent == nil || *b1.AvgPercent != 50 {
		t.Fatalf("lớp của Hương = %+v", huong.Items)
	}
	if m := e.members(basic01ID, ""); len(m) != 3 {
		t.Fatalf("giảng viên xem thành viên lớp mình = %d", len(m))
	}
	e.expectError(http.MethodPost, classPath(basic01ID, "/invitations"), inviteBody("bich.tran@gmail.com", ""), http.StatusForbidden, "FORBIDDEN", "")
}
