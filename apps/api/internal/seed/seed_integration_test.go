//go:build integration

package seed

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/features/identity"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/testdb"
)

const testSeedPassword = "Seed-Password-Test-1"

// testOutboxKey là khóa AES-256 (base64 của 32 byte) chỉ dùng trong test.
const testOutboxKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func testConfig(t *testing.T) config.Config {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)
	return config.Config{
		AppEnv:          config.EnvDev,
		PublicBaseURL:   "http://localhost:5173",
		TempPasswordTTL: 72 * time.Hour,
		SeedPassword:    testSeedPassword,
		OutboxSecretKey: testOutboxKey,
		Location:        loc,
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

// expectedCounts là số bản ghi mỗi bảng suy ra từ data.go.
func expectedCounts() map[string]int {
	return map[string]int{
		"users":                 len(users),
		"media_files":           videoLessonCount(),
		"stages":                len(stages),
		"stage_versions":        len(stageVersions),
		"lessons":               lessonCount(),
		"lesson_media":          videoLessonCount(),
		"courses":               len(courses),
		"course_versions":       len(courseVersions),
		"course_version_stages": len(courseVersions[0].stageVersions),
		"classes":               len(classes),
		"class_members":         len(enrollments),
		"email_outbox":          len(enrollments),
		"invitations":           len(enrollments),
		"lesson_progress":       progressCount(),
		"audit_logs":            len(audits),
	}
}

func tableCounts(t *testing.T, dbx *sqlx.DB) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, table := range insertOrder {
		var n int
		// Tên bảng lấy từ danh sách hằng insertOrder, không từ đầu vào ngoài.
		require.NoError(t, dbx.GetContext(testContext(t), &n, "SELECT count(*) FROM "+table))
		out[table] = n
	}
	return out
}

func resultCounts(res Result) map[string]int {
	out := map[string]int{}
	for _, c := range res.Counts {
		out[c.Table] = c.Rows
	}
	return out
}

func adminID(t *testing.T, dbx *sqlx.DB) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, dbx.GetContext(testContext(t), &id, `SELECT id FROM users WHERE email_normalized = $1`, markerEmail))
	return id
}

func TestRunSeedsPrototypeDatasetIdempotently(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	ctx := testContext(t)
	cfg := testConfig(t)
	clk := fixedClock{time.Now()}

	res, err := Run(ctx, dbx, cfg, clk, Options{})
	require.NoError(t, err)
	assert.False(t, res.AlreadySeeded)
	want := expectedCounts()
	assert.Equal(t, want, resultCounts(res))
	assert.Equal(t, want, tableCounts(t, dbx))
	assert.Equal(t, 17, want["users"])
	assert.Equal(t, 16, want["lessons"])

	again, err := Run(ctx, dbx, cfg, clk, Options{})
	require.NoError(t, err)
	assert.True(t, again.AlreadySeeded)
	assert.Empty(t, again.Counts)
	assert.Equal(t, want, tableCounts(t, dbx), "chạy lần hai không đổi dữ liệu")
}

func TestRunResetReplacesData(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	ctx := testContext(t)
	cfg := testConfig(t)
	clk := fixedClock{time.Now()}

	_, err := Run(ctx, dbx, cfg, clk, Options{})
	require.NoError(t, err)
	before := adminID(t, dbx)

	res, err := Run(ctx, dbx, cfg, clk, Options{Reset: true})
	require.NoError(t, err)
	assert.False(t, res.AlreadySeeded)
	assert.Equal(t, expectedCounts(), tableCounts(t, dbx))
	assert.NotEqual(t, before, adminID(t, dbx), "reset xóa dữ liệu cũ rồi seed lại với id mới")
}

func TestRunRejectsInvalidInputWithoutWriting(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	ctx := testContext(t)
	clk := fixedClock{time.Now()}

	prod := testConfig(t)
	prod.AppEnv = config.EnvProduction
	_, err := Run(ctx, dbx, prod, clk, Options{Reset: true})
	require.ErrorIs(t, err, ErrProduction)

	noPassword := testConfig(t)
	noPassword.SeedPassword = ""
	_, err = Run(ctx, dbx, noPassword, clk, Options{})
	require.ErrorIs(t, err, ErrSeedPassword)

	var n int
	require.NoError(t, dbx.GetContext(ctx, &n, `SELECT count(*) FROM users`))
	assert.Zero(t, n)
}

func TestSeededDataSatisfiesApplicationInvariants(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	ctx := testContext(t)
	cfg := testConfig(t)
	now := time.Now()
	_, err := Run(ctx, dbx, cfg, fixedClock{now}, Options{})
	require.NoError(t, err)

	count := func(q string, args ...any) int {
		t.Helper()
		var n int
		require.NoError(t, dbx.GetContext(ctx, &n, q, args...), q)
		return n
	}

	t.Run("published markdown lessons are rendered", func(t *testing.T) {
		assert.Zero(t, count(`SELECT count(*) FROM lessons l JOIN stage_versions sv ON sv.id = l.stage_version_id
			WHERE sv.status = 'published' AND l.type = 'markdown' AND l.markdown_html IS NULL`))
		assert.Equal(t, 7, count(`SELECT count(*) FROM lessons WHERE type = 'markdown'`))
	})

	t.Run("every video lesson has duration and lesson media", func(t *testing.T) {
		assert.Zero(t, count(`SELECT count(*) FROM lessons l WHERE l.type = 'video'
			AND (l.duration_seconds IS NULL OR NOT EXISTS (SELECT 1 FROM lesson_media lm WHERE lm.lesson_id = l.id AND lm.media_id = l.video_media_id))`))
		assert.Equal(t, 1104, count(`SELECT duration_seconds FROM lessons WHERE lesson_key = 'db-intro'`))
		assert.Equal(t, 9, count(`SELECT count(*) FROM media_files WHERE status = 'ready' AND storage_key LIKE 'seed/%.mp4'`))
	})

	t.Run("password hash matches SEED_PASSWORD", func(t *testing.T) {
		var hashes []string
		require.NoError(t, dbx.SelectContext(ctx, &hashes, `SELECT DISTINCT password_hash FROM users`))
		require.Len(t, hashes, 1, "mọi user dùng cùng hash của SEED_PASSWORD")
		assert.True(t, identity.PasswordHashFromPHC(hashes[0]).Verify(testSeedPassword))
	})

	t.Run("user states follow the prototype", func(t *testing.T) {
		assert.Equal(t, 5, count(`SELECT count(*) FROM users WHERE status = 'invited' AND must_change_password AND temp_password_expires_at IS NOT NULL`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM users WHERE status = 'disabled' AND disabled_at IS NOT NULL`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM class_members WHERE status = 'dropped' AND dropped_at IS NOT NULL`))
	})

	t.Run("outbox rows follow the worker contract", func(t *testing.T) {
		assert.Zero(t, count(`SELECT count(*) FROM email_outbox WHERE secret_enc IS NOT NULL AND status <> 'queued'`))
		var sealed [][]byte
		require.NoError(t, dbx.SelectContext(ctx, &sealed, `SELECT secret_enc FROM email_outbox WHERE status = 'queued'`))
		require.Len(t, sealed, 1)
		box, err := mailer.NewSecretBox(testOutboxKey)
		require.NoError(t, err)
		plain, err := box.Open(sealed[0])
		require.NoError(t, err, "hàng queued phải có secret_enc mở được để worker gửi")
		assert.Equal(t, testSeedPassword, string(plain))
		assert.Zero(t, count(`SELECT count(*) FROM email_outbox WHERE payload::text ILIKE '%password%'`))
		assert.Equal(t, 13, count(`SELECT count(*) FROM email_outbox WHERE status = 'sent' AND sent_at IS NOT NULL AND template = 'invite'`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM email_outbox WHERE status = 'failed' AND attempts = 3 AND last_error IS NOT NULL`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM email_outbox WHERE status = 'queued' AND attempts = 0`))
		assert.Equal(t, len(enrollments), count(`SELECT count(*) FROM invitations i JOIN email_outbox o ON o.id = i.email_outbox_id
			JOIN users u ON u.id = i.user_id WHERE o.to_email = u.email AND i.kind = 'invite'`))
	})

	t.Run("classes and course reference fixed versions", func(t *testing.T) {
		assert.Equal(t, 2, count(`SELECT count(*) FROM classes WHERE status = 'active'`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM classes WHERE status = 'draft' AND code = 'basic03'`))
		var start string
		require.NoError(t, dbx.GetContext(ctx, &start, `SELECT start_date::text FROM classes WHERE code = 'basic01'`))
		assert.Equal(t, ago(58, 0).at(now).In(cfg.Location).Format(time.DateOnly), start)
		assert.Equal(t, 5, count(`SELECT count(*) FROM course_version_stages`))
	})

	t.Run("audit uses canonical actions without secrets", func(t *testing.T) {
		assert.Zero(t, count(`SELECT count(*) FROM audit_logs WHERE action NOT IN (
			'stage_version.published', 'course_version.published', 'class.activated', 'class.member_invited',
			'user.disabled', 'class.created', 'class.invitation_resent')`))
		assert.Zero(t, count(`SELECT count(*) FROM audit_logs WHERE coalesce(after::text, '') ILIKE '%password%' OR coalesce(before::text, '') ILIKE '%password%'`))
		assert.Zero(t, count(`SELECT count(*) FROM audit_logs WHERE actor_id IS NULL OR target_id IS NULL`))
	})
}
