//go:build integration

package identity

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
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
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/middleware"
	"lms/api/internal/platform/secretbox"
	"lms/api/internal/platform/testdb"
)

// integrationKey là OUTBOX_SECRET_KEY chỉ dùng trong test (base64 của 32 byte).
const integrationKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

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

// captureSender giữ email đã gửi để đọc mật khẩu tạm hoặc liên kết đặt lại như người dùng đọc hộp thư.
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

// harness dựng identity trên Postgres thật (repo PG, outbox, audit) và router /api/v1 giống router thật.
type harness struct {
	dbx    *sqlx.DB
	svc    *Service
	clk    *clock.Fake
	box    *secretbox.Box
	engine *gin.Engine
	logs   *syncBuffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	for _, f := range []string{"admin_quan_tran", "teacher_huong_le", "student_an_nguyen"} {
		testdb.Fixture(t, dbx, f)
	}

	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	prev := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(prev) })

	box, err := mailer.NewSecretBox(integrationKey)
	require.NoError(t, err)
	// Đồng hồ giả bắt đầu ở giờ thật vì câu claim outbox so run_at với now() của Postgres.
	clk := &clock.Fake{T: time.Now().UTC().Truncate(time.Microsecond)}
	cfg := testServiceConfig()
	cfg.AppEnv = config.EnvE2E // tắt limiter IP
	svc := NewService(ServiceDeps{
		DB: dbx, Tx: db.TxRunner{DB: dbx},
		Users: PGUserRepo{}, Sessions: PGSessionRepo{}, Attempts: PGLoginAttemptRepo{}, Resets: PGResetTokenRepo{},
		Mailer: mailer.NewService(mailer.NewOutboxRepo(), box, clk), Clock: clk, Audit: audit.PG{Clock: clk}, Cfg: cfg,
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Logger(logger), middleware.Recover(logger))
	api := engine.Group("/api/v1", httpx.RequireCustomHeader())
	auth := NewMiddleware(svc, false)
	NewHandler(svc, cfg).Register(api, auth)
	authed := api.Group("", auth.SessionAuth(), auth.MustChangePassword())
	authed.GET("/stages", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"items": []string{}}) })
	return &harness{dbx: dbx, svc: svc, clk: clk, box: box, engine: engine, logs: logs}
}

func (h *harness) login(email, password string) apiResp {
	return do(h.engine, http.MethodPost, "/api/v1/auth/login", `{"email":"`+email+`","password":"`+password+`"}`)
}

func (h *harness) user(t *testing.T, id uuid.UUID) *User {
	t.Helper()
	u, err := PGUserRepo{}.ByID(context.Background(), h.dbx, id)
	require.NoError(t, err)
	return u
}

func (h *harness) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, h.dbx.GetContext(context.Background(), &n, q, args...))
	return n
}

// deliver chạy worker một lượt và trả email đã gửi.
func (h *harness) deliver(t *testing.T) []mailer.RenderedMail {
	t.Helper()
	renderer, err := mailer.NewRenderer(time.UTC)
	require.NoError(t, err)
	sender := &captureSender{}
	w, err := mailer.NewWorker(mailer.WorkerConfig{
		Tx: db.TxRunner{DB: h.dbx}, Repo: mailer.NewOutboxRepo(), Renderer: renderer, Sender: sender, Box: h.box,
		Clock: h.clk, PublicBaseURL: h.svc.Cfg.PublicBaseURL, Logger: slog.Default(),
	})
	require.NoError(t, err)
	_, err = w.RunOnce(context.Background())
	require.NoError(t, err)
	return sender.sent
}

// assertLogsClean kiểm log API và worker không chứa bí mật nào.
func (h *harness) assertLogsClean(t *testing.T, secrets ...string) {
	t.Helper()
	out := h.logs.String()
	require.NotEmpty(t, out, "middleware log phải ghi mỗi request")
	for _, s := range secrets {
		require.NotEmpty(t, s)
		assert.NotContains(t, out, s, "log lộ bí mật")
	}
}

var (
	studentID = uuid.MustParse(testdb.StudentAnNguyenID)
	adminID   = uuid.MustParse(testdb.AdminQuanTranID)
)

const studentEmail = "an.nguyen@gmail.com"

func TestLoginLockoutAfterFiveFailures(t *testing.T) {
	h := newHarness(t)
	for i := range 5 {
		r := h.login(studentEmail, "sai-mat-khau")
		require.Equal(t, http.StatusUnauthorized, r.Code, "lần %d", i+1)
		assert.Equal(t, apperr.CodeUnauthenticated, r.errorCode(t))
	}
	h.clk.Advance(time.Minute)
	r := h.login(studentEmail, testdb.FixturePassword)
	require.Equal(t, http.StatusTooManyRequests, r.Code, "mật khẩu đúng vẫn bị khóa trong cửa sổ")
	assert.Equal(t, apperr.CodeTooManyAttempts, r.errorCode(t))
	assert.Equal(t, "840", r.Header().Get("Retry-After"))
	assert.Equal(t, 5, h.count(t, `SELECT count(*) FROM login_attempts WHERE email_normalized = $1 AND NOT succeeded`, studentEmail))

	h.clk.Advance(15 * time.Minute)
	r = h.login(studentEmail, testdb.FixturePassword)
	require.Equal(t, http.StatusOK, r.Code, "hết cửa sổ thì đăng nhập lại được")
	h.assertLogsClean(t, testdb.FixturePassword, sessionCookie(t, r).Value)
}

func TestParallelWrongLoginsRecordExactlyMaxFailures(t *testing.T) {
	h := newHarness(t)
	const n = 20
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			_, errs[i] = h.svc.Login(context.Background(), "An.Nguyen@Gmail.com", "sai-mat-khau", "10.0.0.1", "test")
		})
	}
	wg.Wait()
	var invalid, locked int
	for _, err := range errs {
		var ae *apperr.Error
		require.ErrorAs(t, err, &ae)
		switch ae.Code {
		case apperr.CodeUnauthenticated:
			invalid++
		case apperr.CodeTooManyAttempts:
			locked++
		default:
			t.Fatalf("lỗi không mong đợi: %v", err)
		}
	}
	assert.Equal(t, 5, invalid)
	assert.Equal(t, n-5, locked)
	assert.Equal(t, 5, h.count(t, `SELECT count(*) FROM login_attempts WHERE email_normalized = $1`, studentEmail))
}

func TestUnknownEmailCountsTowardsLockout(t *testing.T) {
	h := newHarness(t)
	for range 5 {
		r := h.login("khong.ton.tai@gmail.com", "x")
		require.Equal(t, http.StatusUnauthorized, r.Code)
	}
	assert.Equal(t, http.StatusTooManyRequests, h.login("khong.ton.tai@gmail.com", "x").Code)
}

// provision tạo học viên invited như feature classes và trả mật khẩu tạm.
func (h *harness) provision(t *testing.T, email string) (*User, string) {
	t.Helper()
	var (
		u   *User
		tmp TemporaryPassword
	)
	e, err := domain.ParseEmail(email)
	require.NoError(t, err)
	require.NoError(t, h.svc.Tx.Transact(context.Background(), func(tx db.Executor) error {
		var created bool
		var err error
		u, tmp, created, err = h.svc.ProvisionStudent(context.Background(), tx, e, "Bùi Minh", h.clk.Now())
		if err == nil && !created {
			err = errors.New("học viên đã tồn tại")
		}
		return err
	}))
	return u, tmp.Reveal()
}

func TestProvisionRejectsDuplicateEmail(t *testing.T) {
	h := newHarness(t)
	e, err := domain.ParseEmail("minh.bui@gmail.com")
	require.NoError(t, err)
	hash, err := NewPasswordHash("matkhau-tam-1")
	require.NoError(t, err)
	u1, err := NewInvitedStudent(uuid.New(), e, "Bùi Minh", hash, h.clk.Now(), time.Hour)
	require.NoError(t, err)
	require.NoError(t, PGUserRepo{}.Create(context.Background(), h.dbx, u1))
	upper, err := domain.ParseEmail("Minh.Bui@Gmail.com")
	require.NoError(t, err)
	u2, err := NewInvitedStudent(uuid.New(), upper, "Bùi Minh", hash, h.clk.Now(), time.Hour)
	require.NoError(t, err)
	require.ErrorIs(t, PGUserRepo{}.Create(context.Background(), h.dbx, u2), ErrEmailTaken)
}

func TestExpiredTemporaryPassword(t *testing.T) {
	h := newHarness(t)
	_, tmp := h.provision(t, "minh.bui@gmail.com")
	h.clk.Advance(72*time.Hour + time.Second)
	r := h.login("minh.bui@gmail.com", tmp)
	require.Equal(t, http.StatusUnauthorized, r.Code)
	assert.Equal(t, apperr.CodeTempPasswordExpired, r.errorCode(t))
	assert.Zero(t, h.count(t, `SELECT count(*) FROM login_attempts WHERE NOT succeeded`), "hết hạn không tính là lần sai")
	h.assertLogsClean(t, tmp)
}

func TestFirstLoginMustChangePassword(t *testing.T) {
	h := newHarness(t)
	u, tmp := h.provision(t, "minh.bui@gmail.com")

	r := h.login("minh.bui@gmail.com", tmp)
	require.Equal(t, http.StatusOK, r.Code, r.Body.String())
	assert.Contains(t, r.Body.String(), `"mustChangePassword":true`)
	first := sessionCookie(t, r)
	other := sessionCookie(t, h.login("minh.bui@gmail.com", tmp))

	r = do(h.engine, http.MethodGet, "/api/v1/stages", "", first)
	require.Equal(t, http.StatusForbidden, r.Code)
	assert.Equal(t, apperr.CodePasswordChangeRequired, r.errorCode(t))
	assert.Equal(t, http.StatusOK, do(h.engine, http.MethodGet, "/api/v1/auth/me", "", first).Code)

	// Học viên invited đổi mật khẩu với body 2 trường, không cần currentPassword.
	r = do(h.engine, http.MethodPost, "/api/v1/auth/change-password",
		`{"newPassword":"matkhau-moi-2026","confirmPassword":"matkhau-moi-2026"}`, first)
	require.Equal(t, http.StatusOK, r.Code, r.Body.String())

	var row struct {
		MustChange bool       `db:"must_change_password"`
		Status     string     `db:"status"`
		TempExp    *time.Time `db:"temp_password_expires_at"`
	}
	require.NoError(t, h.dbx.GetContext(context.Background(), &row,
		`SELECT must_change_password, status, temp_password_expires_at FROM users WHERE id = $1`, u.ID()))
	assert.False(t, row.MustChange)
	assert.Equal(t, "active", row.Status)
	assert.Nil(t, row.TempExp)
	assert.Equal(t, 1, h.count(t, `SELECT count(*) FROM sessions WHERE user_id = $1`, u.ID()), "phiên khác bị xóa")

	assert.Equal(t, http.StatusOK, do(h.engine, http.MethodGet, "/api/v1/stages", "", first).Code)
	assert.Equal(t, http.StatusUnauthorized, do(h.engine, http.MethodGet, "/api/v1/stages", "", other).Code)
	assert.Equal(t, http.StatusUnauthorized, h.login("minh.bui@gmail.com", tmp).Code, "mật khẩu tạm hết hiệu lực")
	assert.Equal(t, http.StatusOK, h.login("minh.bui@gmail.com", "matkhau-moi-2026").Code)

	r = do(h.engine, http.MethodPost, "/api/v1/auth/logout", "", first)
	assert.Equal(t, http.StatusNoContent, r.Code)
	assert.Equal(t, http.StatusUnauthorized, do(h.engine, http.MethodGet, "/api/v1/auth/me", "", first).Code)
	h.assertLogsClean(t, tmp, "matkhau-moi-2026", first.Value, other.Value)
}

func TestLogoutAllowedWhilePasswordChangeRequired(t *testing.T) {
	h := newHarness(t)
	_, tmp := h.provision(t, "minh.bui@gmail.com")
	c := sessionCookie(t, h.login("minh.bui@gmail.com", tmp))
	assert.Equal(t, http.StatusNoContent, do(h.engine, http.MethodPost, "/api/v1/auth/logout", "", c).Code)
	assert.Zero(t, h.count(t, `SELECT count(*) FROM sessions`))
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	h := newHarness(t)
	keep := sessionCookie(t, h.login(studentEmail, testdb.FixturePassword))
	drop := sessionCookie(t, h.login(studentEmail, testdb.FixturePassword))

	r := do(h.engine, http.MethodPost, "/api/v1/auth/change-password",
		`{"newPassword":"matkhau-moi-2026","confirmPassword":"matkhau-moi-2026"}`, keep)
	require.Equal(t, http.StatusUnprocessableEntity, r.Code, "tài khoản active phải nhập mật khẩu hiện tại")

	r = do(h.engine, http.MethodPost, "/api/v1/auth/change-password",
		`{"currentPassword":"`+testdb.FixturePassword+`","newPassword":"matkhau-moi-2026","confirmPassword":"matkhau-moi-2026"}`, keep)
	require.Equal(t, http.StatusOK, r.Code, r.Body.String())
	assert.Equal(t, http.StatusOK, do(h.engine, http.MethodGet, "/api/v1/auth/me", "", keep).Code)
	assert.Equal(t, http.StatusUnauthorized, do(h.engine, http.MethodGet, "/api/v1/auth/me", "", drop).Code)
}

func TestDisableUserRevokesAccessAndAudits(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	student := sessionCookie(t, h.login(studentEmail, testdb.FixturePassword))
	admin := sessionCookie(t, h.login("quan.tran@goup.vn", testdb.FixturePassword))

	// Token đặt lại tạo trước khi bị vô hiệu hóa.
	require.NoError(t, h.svc.ForgotPassword(ctx, studentEmail))
	mails := h.deliver(t)
	require.Len(t, mails, 1)
	token := resetToken(t, mails[0])
	before := h.user(t, studentID)

	r := do(h.engine, http.MethodPost, "/api/v1/users/"+testdb.StudentAnNguyenID+"/disable", "", admin)
	require.Equal(t, http.StatusOK, r.Code, r.Body.String())

	r = do(h.engine, http.MethodGet, "/api/v1/auth/me", "", student)
	assert.Equal(t, http.StatusUnauthorized, r.Code, "phiên cũ bị từ chối ngay")
	r = h.login(studentEmail, testdb.FixturePassword)
	require.Equal(t, http.StatusForbidden, r.Code)
	assert.Equal(t, apperr.CodeAccountDisabled, r.errorCode(t))
	r = h.login(studentEmail, "sai-mat-khau")
	require.Equal(t, http.StatusUnauthorized, r.Code, "sai mật khẩu không lộ trạng thái disabled")
	assert.Equal(t, apperr.CodeUnauthenticated, r.errorCode(t))

	r = do(h.engine, http.MethodPost, "/api/v1/auth/reset-password",
		`{"token":"`+token+`","newPassword":"matkhau-moi-2026","confirmPassword":"matkhau-moi-2026"}`)
	require.Equal(t, http.StatusForbidden, r.Code)
	assert.Equal(t, apperr.CodeAccountDisabled, r.errorCode(t))
	assert.Equal(t, before.PasswordHash().PHC(), h.user(t, studentID).PasswordHash().PHC(), "mật khẩu không đổi")

	r = do(h.engine, http.MethodPost, "/api/v1/users/"+testdb.StudentAnNguyenID+"/enable", "", admin)
	require.Equal(t, http.StatusOK, r.Code, r.Body.String())
	assert.Equal(t, http.StatusOK, h.login(studentEmail, testdb.FixturePassword).Code)

	var rows []struct {
		ActorID  uuid.UUID `db:"actor_id"`
		Action   string    `db:"action"`
		TargetID uuid.UUID `db:"target_id"`
	}
	require.NoError(t, h.dbx.SelectContext(ctx, &rows,
		`SELECT actor_id, action, target_id FROM audit_logs WHERE target_type = 'user' ORDER BY at, action`))
	require.Len(t, rows, 2)
	for _, row := range rows {
		assert.Equal(t, adminID, row.ActorID)
		assert.Equal(t, studentID, row.TargetID)
	}
	assert.ElementsMatch(t, []string{audit.ActionUserDisabled, audit.ActionUserEnabled}, []string{rows[0].Action, rows[1].Action})
	h.assertLogsClean(t, testdb.FixturePassword, token, student.Value, admin.Value)
}

// resetToken lấy token từ liên kết trong email đặt lại.
func resetToken(t *testing.T, m mailer.RenderedMail) string {
	t.Helper()
	const marker = "/reset-password#token="
	i := strings.Index(m.Text, marker)
	require.GreaterOrEqual(t, i, 0, "email không có liên kết đặt lại")
	tok := m.Text[i+len(marker):]
	if j := strings.IndexAny(tok, " \r\n"); j >= 0 {
		tok = tok[:j]
	}
	require.NotEmpty(t, tok)
	return tok
}

func TestForgotResetRoundTripThroughOutbox(t *testing.T) {
	h := newHarness(t)
	old := sessionCookie(t, h.login(studentEmail, testdb.FixturePassword))

	known := do(h.engine, http.MethodPost, "/api/v1/auth/forgot-password", `{"email":"`+studentEmail+`"}`)
	unknown := do(h.engine, http.MethodPost, "/api/v1/auth/forgot-password", `{"email":"khong.ton.tai@gmail.com"}`)
	require.Equal(t, http.StatusAccepted, known.Code)
	assert.Equal(t, known.Code, unknown.Code)
	assert.Equal(t, known.Body.String(), unknown.Body.String(), "phản hồi giống nhau cho email có/không tồn tại")
	assert.Equal(t, 1, h.count(t, `SELECT count(*) FROM email_outbox`))
	assert.Zero(t, h.count(t, `SELECT count(*) FROM email_outbox WHERE payload::text ILIKE '%token%' OR payload::text ILIKE '%link%'`))

	mails := h.deliver(t)
	require.Len(t, mails, 1)
	assert.Equal(t, studentEmail, mails[0].To)
	assert.Contains(t, mails[0].Text, "http://localhost:5173/reset-password#token=")
	token := resetToken(t, mails[0])
	assert.Zero(t, h.count(t, `SELECT count(*) FROM email_outbox WHERE secret_enc IS NOT NULL`))
	var stored []byte
	require.NoError(t, h.dbx.GetContext(context.Background(), &stored, `SELECT token_hash FROM password_reset_tokens`))
	assert.Equal(t, HashToken(token), stored, "DB chỉ lưu sha256 của token")

	body := `{"token":"` + token + `","newPassword":"matkhau-moi-2026","confirmPassword":"matkhau-moi-2026"}`
	r := do(h.engine, http.MethodPost, "/api/v1/auth/reset-password", body)
	require.Equal(t, http.StatusNoContent, r.Code, r.Body.String())
	r = do(h.engine, http.MethodPost, "/api/v1/auth/reset-password", body)
	require.Equal(t, http.StatusBadRequest, r.Code, "token dùng lần hai")

	assert.Equal(t, http.StatusUnauthorized, do(h.engine, http.MethodGet, "/api/v1/auth/me", "", old).Code, "reset thu hồi mọi phiên")
	assert.Equal(t, http.StatusUnauthorized, h.login(studentEmail, testdb.FixturePassword).Code)
	assert.Equal(t, http.StatusOK, h.login(studentEmail, "matkhau-moi-2026").Code)
	h.assertLogsClean(t, token, "matkhau-moi-2026", old.Value)
}

func TestResetTokenExpires(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.svc.ForgotPassword(context.Background(), studentEmail))
	token := resetToken(t, h.deliver(t)[0])
	h.clk.Advance(30*time.Minute + time.Second)
	err := h.svc.ResetPassword(context.Background(), ResetPasswordCmd{Token: token, NewPassword: "matkhau-moi-2026", ConfirmPassword: "matkhau-moi-2026"})
	require.ErrorIs(t, err, ErrResetTokenInvalid)
}

func TestNewForgotRequestInvalidatesOlderToken(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	require.NoError(t, h.svc.ForgotPassword(ctx, studentEmail))
	require.NoError(t, h.svc.ForgotPassword(ctx, studentEmail))
	mails := h.deliver(t)
	require.Len(t, mails, 2)
	cmd := ResetPasswordCmd{NewPassword: "matkhau-moi-2026", ConfirmPassword: "matkhau-moi-2026"}
	tokens := []string{resetToken(t, mails[0]), resetToken(t, mails[1])}
	var ok, invalid int
	for _, tok := range tokens {
		cmd.Token = tok
		switch err := h.svc.ResetPassword(ctx, cmd); {
		case err == nil:
			ok++
		case errors.Is(err, ErrResetTokenInvalid):
			invalid++
		default:
			t.Fatalf("lỗi không mong đợi: %v", err)
		}
	}
	assert.Equal(t, 1, ok)
	assert.Equal(t, 1, invalid)
}

func TestCleanupRemovesOldAttemptsAndExpiredSessions(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.login(studentEmail, "sai-mat-khau")
	sessionCookie(t, h.login(studentEmail, testdb.FixturePassword))
	require.NoError(t, h.svc.Cleanup(ctx, h.clk.Now()))
	assert.Equal(t, 2, h.count(t, `SELECT count(*) FROM login_attempts`))
	assert.Equal(t, 1, h.count(t, `SELECT count(*) FROM sessions`))

	require.NoError(t, h.svc.Cleanup(ctx, h.clk.Now().Add(31*24*time.Hour)))
	assert.Zero(t, h.count(t, `SELECT count(*) FROM login_attempts`))
	assert.Zero(t, h.count(t, `SELECT count(*) FROM sessions`))
}
