//go:build integration

package quizzes_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	"lms/api/internal/features/identity"
	"lms/api/internal/features/learning"
	"lms/api/internal/features/mailer"
	"lms/api/internal/features/quizzes"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/middleware"
	"lms/api/internal/platform/testdb"
	"lms/api/internal/seed"
)

// Mật khẩu và khóa chỉ dùng cho DB test.
const (
	itSeedPassword = "Seed-Password-Test-1"
	itOutboxKey    = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
)

// seedEnv dựng route quiz của staff trên dữ liệu seed (theo seed.js của prototype).
type seedEnv struct {
	t      *testing.T
	dbx    *sqlx.DB
	ident  *identity.Service
	engine *gin.Engine
}

func newSeedEnv(t *testing.T) *seedEnv {
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

	e := &seedEnv{t: t, dbx: dbx}
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
	svc := quizzes.NewService(quizzes.Deps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Repo: quizzes.PGRepo{}, Members: classes.PGMembershipReader{},
		Progress: learning.PGProgressRepo{}, Clock: clk, IDs: ids.V7{}, Shuffle: quizzes.RandShuffler{},
	})
	review := quizzes.NewReviewService(quizzes.ReviewDeps{
		DB: dbx, Tx: db.TxRunner{DB: dbx}, Repo: quizzes.PGRepo{}, Mail: mailer.NewService(mailer.NewOutboxRepo(), nil, clk),
		Audit: audit.PG{Clock: clk}, Clock: clk, IDs: ids.V7{},
		Next:          grading.NewService(grading.Deps{Tx: db.TxRunner{DB: dbx}, Repo: grading.PGRepo{}, Clock: clk}),
		PublicBaseURL: "http://localhost:5173",
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recover(slog.New(slog.DiscardHandler)), middleware.SecurityHeaders())
	authed := engine.Group("/api/v1", identity.NewMiddleware(e.ident, false).SessionAuth())
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
	quizzes.NewHandler(svc, review, student, actor).Register(authed, quizzes.Guards{
		Staff:   identity.RequireRole(domain.RoleAdmin, domain.RoleTeacher),
		Student: identity.RequireRole(domain.RoleStudent),
	})
	e.engine = engine
	return e
}

func (e *seedEnv) id(q string, args ...any) uuid.UUID {
	e.t.Helper()
	var id uuid.UUID
	require.NoError(e.t, e.dbx.Get(&id, q, args...), q)
	return id
}

// get đăng nhập bằng mật khẩu seed rồi gọi GET; trả mã và thân phản hồi.
func (e *seedEnv) get(email, path string) (int, []byte) {
	e.t.Helper()
	res, err := e.ident.Login(context.Background(), email, itSeedPassword, "127.0.0.1", "it")
	require.NoError(e.t, err, "đăng nhập %s", email)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: identity.CookieName(false), Value: res.Token.Reveal()})
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

// Tab Quiz của basic01 trên seed: thống kê và trạng thái từng thành viên khớp dữ liệu prototype.
func TestClassQuizzesMatchesSeed(t *testing.T) {
	e := newSeedEnv(t)
	basic01 := e.id(`SELECT id FROM classes WHERE code = 'basic01'`)
	basic02 := e.id(`SELECT id FROM classes WHERE code = 'basic02'`)

	code, raw := e.get("huong.le@goup.vn", "/api/v1/classes/"+basic01.String()+"/quizzes")
	require.Equal(t, http.StatusOK, code, "%s", raw)
	var v quizzes.ClassQuizzesDTO
	require.NoError(t, json.Unmarshal(raw, &v))
	assert.Equal(t, "basic01", v.Class.Code)
	pending := 0
	for _, it := range v.Items {
		assert.Equal(t, it.Stats.Members, len(it.Members), it.Lesson.Title)
		pending += it.Stats.Pending
	}
	var queued int
	require.NoError(t, e.dbx.Get(&queued, `SELECT count(DISTINCT (a.class_member_id, a.lesson_id)) FROM quiz_attempts a
		JOIN class_members cm ON cm.id = a.class_member_id
		WHERE cm.class_id = $1 AND cm.status = 'active' AND a.review_status = 'pending'`, basic01))
	assert.Equal(t, queued, pending, "số thành viên chờ xác nhận khớp lượt pending của seed")
	quizzes.AssertGolden(t, "class-quizzes.json", raw)

	code, _ = e.get("huong.le@goup.vn", "/api/v1/classes/"+basic02.String()+"/quizzes")
	assert.Equal(t, http.StatusForbidden, code, "Hương không phụ trách basic02")

	// Lượt db-test của Phong ở basic02 được Bảo xác nhận trong seed: Bảo xem được, Hương thì không.
	attempt := e.id(`SELECT r.attempt_id FROM quiz_attempt_reviews r JOIN users u ON u.id = r.reviewer_id
		WHERE u.email = 'bao.pham@goup.vn'`)
	code, raw = e.get("bao.pham@goup.vn", "/api/v1/quiz-attempts/"+attempt.String())
	require.Equal(t, http.StatusOK, code, "%s", raw)
	var d quizzes.AttemptDetailDTO
	require.NoError(t, json.Unmarshal(raw, &d))
	require.Len(t, d.Reviews, 1)
	assert.Equal(t, "Phạm Quốc Bảo", d.Reviews[0].Reviewer.Name)
	assert.Equal(t, "basic02", d.Class.Code)
	require.NotNil(t, d.ReviewStatus)
	assert.Equal(t, "confirmed", *d.ReviewStatus)
	code, _ = e.get("huong.le@goup.vn", "/api/v1/quiz-attempts/"+attempt.String())
	assert.Equal(t, http.StatusForbidden, code)
}
