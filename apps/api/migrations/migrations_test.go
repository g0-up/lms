//go:build integration

package migrations_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/testdb"
)

// Mã SQLSTATE dùng trong test.
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
	codeNotNullViolation    = "23502"
)

// schemaVersion là version mới nhất mà bộ migration hiện tại phải đạt.
const schemaVersion = 9

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

// TestMigrationRoundTrip đứng đầu file: drop/tạo lại bảng trước khi pool chung chuẩn bị câu lệnh nào trên bảng ứng dụng.
func TestMigrationRoundTrip(t *testing.T) {
	testdb.Open(t)
	url := testdb.URL(t)
	ctx := testContext(t)

	require.Equal(t, uint(schemaVersion), testdb.LatestVersion(t))

	require.NoError(t, db.Migrate(ctx, url, db.Down, 0))
	_, _, err := db.Version(ctx, url)
	require.ErrorIs(t, err, db.ErrNilVersion)

	require.NoError(t, db.Migrate(ctx, url, db.Up, 0))
	v, dirty, err := db.Version(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, uint(schemaVersion), v)
	assert.False(t, dirty)
}

// TestNoUserDefinedDatabaseObjects bảo vệ quyết định "chỉ ràng buộc khai báo": schema không có trigger, function,
// enum type hay extension do người dùng tạo.
func TestNoUserDefinedDatabaseObjects(t *testing.T) {
	dbx := testdb.Open(t)
	ctx := testContext(t)

	queries := map[string]string{
		"trigger":   `SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal`,
		"function":  `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname = 'public'`,
		"enum type": `SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace WHERE n.nspname = 'public' AND t.typtype = 'e'`,
		"extension": `SELECT count(*) FROM pg_extension WHERE extname <> 'plpgsql'`,
	}
	for name, q := range queries {
		var n int
		require.NoError(t, dbx.GetContext(ctx, &n, q), name)
		assert.Zero(t, n, "schema không được có %s do người dùng tạo", name)
	}
}

func TestForeignKeyDeleteActions(t *testing.T) {
	dbx := testdb.Open(t)
	ctx := testContext(t)

	var rows []struct {
		Name    string `db:"conname"`
		Action  string `db:"confdeltype"`
		NumCols int    `db:"ncols"`
	}
	require.NoError(t, dbx.SelectContext(ctx, &rows, `
		SELECT c.conname, c.confdeltype::text AS confdeltype, array_length(c.conkey, 1) AS ncols
		FROM pg_constraint c
		JOIN pg_namespace n ON n.oid = c.connamespace
		WHERE n.nspname = 'public' AND c.contype = 'f'`))
	require.NotEmpty(t, rows)

	cascade := []string{pgerr.FkLessonsStageVersion, pgerr.FkLessonMediaLesson}
	for _, r := range rows {
		want := "r" // RESTRICT
		if slices.Contains(cascade, r.Name) {
			want = "c"
		}
		assert.Equal(t, want, r.Action, "ON DELETE của %s", r.Name)
		if r.Name == pgerr.FkCvsStageVersion {
			assert.Equal(t, 2, r.NumCols, "fk_cvs_stage_version là FK ghép (stage_version_id, stage_id)")
		}
	}
}

func TestEmailOutboxShape(t *testing.T) {
	dbx := testdb.Open(t)
	ctx := testContext(t)

	var cols []string
	require.NoError(t, dbx.SelectContext(ctx, &cols,
		`SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'email_outbox'`))
	assert.Contains(t, cols, "secret_enc")
	assert.NotContains(t, cols, "updated_at", "mọi chuyển trạng thái outbox ghi cột thời điểm riêng")

	var def string
	require.NoError(t, dbx.GetContext(ctx, &def, `SELECT indexdef FROM pg_indexes WHERE indexname = 'ix_email_outbox_claim'`))
	assert.Contains(t, def, "(run_at)")
	assert.Contains(t, def, "'queued'")
	assert.Contains(t, def, "'sending'")
}

// fixture là dữ liệu đã commit: testdb.Fixture nạp chặng DB (v1 published với db-table/db-index đã render, v2 draft
// với db-table chưa render) và khóa học BASIC v1 dùng DB v1. Phần dựng thêm inline là thứ fixture chung không có:
// media, bài video db-intro ở vị trí 3 của cả hai bản kèm lesson_media, và chặng DS có một bản published.
type fixture struct {
	admin, videoMedia, imageMedia uuid.UUID
	stageA, stageB                uuid.UUID
	svA1, svA2, svB1              uuid.UUID
	lessonA1Video, lessonA1Doc    uuid.UUID
	lessonA2Doc, lessonA2Video    uuid.UUID
	lessonA2Index                 uuid.UUID
	course, cv1                   uuid.UUID
}

func setup(t *testing.T) (*sqlx.DB, fixture) {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	ctx := testContext(t)

	testdb.Fixture(t, dbx, "course_basic_published")
	testdb.Fixture(t, dbx, "stage_db_draft")

	f := fixture{
		admin:      uuid.MustParse(testdb.AdminQuanTranID),
		videoMedia: ids.New(), imageMedia: ids.New(),
		stageA: uuid.MustParse(testdb.StageDBID), stageB: ids.New(),
		svA1: uuid.MustParse(testdb.StageDBV1ID), svA2: uuid.MustParse(testdb.StageDBV2ID), svB1: ids.New(),
		lessonA1Video: ids.New(), lessonA1Doc: uuid.MustParse(testdb.LessonDBTableV1ID),
		lessonA2Doc: uuid.MustParse(testdb.LessonDBTableV2ID), lessonA2Video: ids.New(),
		lessonA2Index: uuid.MustParse(testdb.LessonDBIndexV2ID),
		course:        uuid.MustParse(testdb.CourseBasicID), cv1: uuid.MustParse(testdb.CourseBasicV1ID),
	}
	stmts := []struct {
		q    string
		args []any
	}{
		{`INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by, ready_at)
		  VALUES ($1, 'video', 'test/v.mp4', 'v.mp4', 'video/mp4', 10, 'ready', $3, now()),
		         ($2, 'image', 'test/i.png', 'i.png', 'image/png', 10, 'ready', $3, now())`,
			[]any{f.videoMedia, f.imageMedia, f.admin}},
		{`INSERT INTO stages (id, code, name, created_by) VALUES ($1, 'DS', 'Cấu trúc dữ liệu', $2)`, []any{f.stageB, f.admin}},
		{`INSERT INTO stage_versions (id, stage_id, version_no, status, title, published_at, created_by)
		  VALUES ($1, $2, 1, 'published', 'Cấu trúc dữ liệu', now(), $3)`, []any{f.svB1, f.stageB, f.admin}},
		{`INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, video_media_id, duration_seconds)
		  VALUES ($1, $3, 'db-intro', 3, 'Video giới thiệu', 'video', $5, 60),
		         ($2, $4, 'db-intro', 3, 'Video giới thiệu', 'video', $5, 60)`,
			[]any{f.lessonA1Video, f.lessonA2Video, f.svA1, f.svA2, f.videoMedia}},
		{`INSERT INTO lesson_media (lesson_id, media_id) VALUES ($1, $4), ($2, $4), ($3, $5)`,
			[]any{f.lessonA1Video, f.lessonA2Video, f.lessonA2Doc, f.videoMedia, f.imageMedia}},
	}
	for _, s := range stmts {
		_, err := dbx.ExecContext(ctx, s.q, s.args...)
		require.NoError(t, err, s.q)
	}
	return dbx, f
}

// begin mở transaction luôn rollback khi subtest kết thúc.
func begin(t *testing.T, dbx *sqlx.DB) *sqlx.Tx {
	t.Helper()
	tx, err := dbx.BeginTxx(testContext(t), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

func exec(t *testing.T, tx *sqlx.Tx, q string, args ...any) int64 {
	t.Helper()
	res, err := tx.ExecContext(testContext(t), q, args...)
	require.NoError(t, err, q)
	n, err := res.RowsAffected()
	require.NoError(t, err)
	return n
}

// expectPgErr chạy câu lệnh trong savepoint, kiểm SQLSTATE và tên constraint rồi quay lại savepoint để tx dùng tiếp.
// constraint rỗng nghĩa là không kiểm tên (NOT NULL không có constraint có tên).
func expectPgErr(t *testing.T, tx *sqlx.Tx, code, constraint, q string, args ...any) {
	t.Helper()
	ctx := testContext(t)
	_, err := tx.ExecContext(ctx, "SAVEPOINT expect_err")
	require.NoError(t, err)
	_, execErr := tx.ExecContext(ctx, q, args...)
	_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT expect_err")
	require.NoError(t, err)

	var pgErr *pgconn.PgError
	require.True(t, errors.As(execErr, &pgErr), "cần *pgconn.PgError, nhận %v", execErr)
	assert.Equal(t, code, pgErr.Code, pgErr.Message)
	if constraint != "" {
		assert.Equal(t, constraint, pgErr.ConstraintName, pgErr.Message)
	}
}

func count(t *testing.T, tx *sqlx.Tx, q string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, tx.GetContext(testContext(t), &n, q, args...))
	return n
}

func TestVersionHeaderConstraints(t *testing.T) {
	dbx, f := setup(t)

	t.Run("second draft stage version is rejected", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqStageVersionsOneDraft,
			`INSERT INTO stage_versions (id, stage_id, version_no, status, title, created_by) VALUES ($1, $2, 3, 'draft', 'A v3', $3)`,
			ids.New(), f.stageA, f.admin)
	})

	t.Run("second draft course version is rejected", func(t *testing.T) {
		tx := begin(t, dbx)
		exec(t, tx, `INSERT INTO course_versions (id, course_id, version_no, status, title, created_by) VALUES ($1, $2, 2, 'draft', 'v2', $3)`,
			ids.New(), f.course, f.admin)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqCourseVersionsOneDraft,
			`INSERT INTO course_versions (id, course_id, version_no, status, title, created_by) VALUES ($1, $2, 3, 'draft', 'v3', $3)`,
			ids.New(), f.course, f.admin)
	})

	cases := []struct {
		name, status, publishedAt, archivedAt string
		stageConstraint, courseConstraint     string
	}{
		{"published without published_at", "published", "NULL", "NULL", pgerr.CkStageVersionsPublishedAt, pgerr.CkCourseVersionsPublishedAt},
		{"archived without archived_at", "archived", "now()", "NULL", pgerr.CkStageVersionsArchivedAt, pgerr.CkCourseVersionsArchivedAt},
		{"draft with archived_at", "draft", "NULL", "now()", pgerr.CkStageVersionsArchivedAt, pgerr.CkCourseVersionsArchivedAt},
	}
	for _, c := range cases {
		t.Run("stage version "+c.name, func(t *testing.T) {
			tx := begin(t, dbx)
			expectPgErr(t, tx, codeCheckViolation, c.stageConstraint,
				`INSERT INTO stage_versions (id, stage_id, version_no, status, title, published_at, archived_at, created_by)
				 VALUES ($1, $2, 9, $3, 't', `+c.publishedAt+`, `+c.archivedAt+`, $4)`,
				ids.New(), f.stageB, c.status, f.admin)
		})
		t.Run("course version "+c.name, func(t *testing.T) {
			tx := begin(t, dbx)
			expectPgErr(t, tx, codeCheckViolation, c.courseConstraint,
				`INSERT INTO course_versions (id, course_id, version_no, status, title, published_at, archived_at, created_by)
				 VALUES ($1, $2, 9, $3, 't', `+c.publishedAt+`, `+c.archivedAt+`, $4)`,
				ids.New(), f.course, c.status, f.admin)
		})
	}

	t.Run("valid archived version is accepted", func(t *testing.T) {
		tx := begin(t, dbx)
		n := exec(t, tx, `UPDATE stage_versions SET status = 'archived', archived_at = now() WHERE id = $1`, f.svA1)
		assert.EqualValues(t, 1, n)
	})
}

func TestCourseVersionStagesCompositeForeignKey(t *testing.T) {
	dbx, f := setup(t)
	insertDraftCourseVersion := func(t *testing.T, tx *sqlx.Tx) uuid.UUID {
		t.Helper()
		id := ids.New()
		exec(t, tx, `INSERT INTO course_versions (id, course_id, version_no, status, title, created_by) VALUES ($1, $2, 2, 'draft', 'v2', $3)`,
			id, f.course, f.admin)
		return id
	}
	const insertCVS = `INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position) VALUES ($1, $2, $3, $4)`

	t.Run("stage_id must match the stage version", func(t *testing.T) {
		tx := begin(t, dbx)
		cv := insertDraftCourseVersion(t, tx)
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkCvsStageVersion, insertCVS, cv, f.svA1, f.stageB, 1)
		assert.EqualValues(t, 1, exec(t, tx, insertCVS, cv, f.svB1, f.stageB, 1))
		expectPgErr(t, tx, codeNotNullViolation, "", insertCVS, cv, f.svA1, nil, 2)
	})

	t.Run("referenced draft stage version cannot be deleted", func(t *testing.T) {
		tx := begin(t, dbx)
		svB2 := ids.New()
		exec(t, tx, `INSERT INTO stage_versions (id, stage_id, version_no, status, title, created_by) VALUES ($1, $2, 2, 'draft', 'B v2', $3)`,
			svB2, f.stageB, f.admin)
		cv := insertDraftCourseVersion(t, tx)
		exec(t, tx, insertCVS, cv, svB2, f.stageB, 1)
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkCvsStageVersion, `DELETE FROM stage_versions WHERE id = $1`, svB2)
	})

	t.Run("deleting an unreferenced draft cascades to lessons and lesson media", func(t *testing.T) {
		tx := begin(t, dbx)
		assert.EqualValues(t, 1, exec(t, tx, `DELETE FROM stage_versions WHERE id = $1`, f.svA2))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM lessons WHERE stage_version_id = $1`, f.svA2))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM lesson_media WHERE lesson_id IN ($1, $2)`, f.lessonA2Doc, f.lessonA2Video))
		assert.Equal(t, 3, count(t, tx, `SELECT count(*) FROM lessons WHERE stage_version_id = $1`, f.svA1))
	})
}

func TestLessonConstraints(t *testing.T) {
	dbx, f := setup(t)

	t.Run("reorder succeeds only with deferred position constraint", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqLessonsVersionPosition,
			`UPDATE lessons SET position = 2 WHERE id = $1`, f.lessonA2Doc)

		exec(t, tx, `SET CONSTRAINTS `+pgerr.UqLessonsVersionPosition+` DEFERRED`)
		exec(t, tx, `UPDATE lessons SET position = 2 WHERE id = $1`, f.lessonA2Doc)
		exec(t, tx, `UPDATE lessons SET position = 1 WHERE id = $1`, f.lessonA2Index)
		require.NoError(t, tx.Commit())

		var pos int
		require.NoError(t, dbx.GetContext(testContext(t), &pos, `SELECT position FROM lessons WHERE id = $1`, f.lessonA2Doc))
		assert.Equal(t, 2, pos)
	})

	t.Run("video lesson requires media", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkLessonsTypeContent,
			`INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type) VALUES ($1, $2, 'no-media', 9, 'x', 'video')`,
			ids.New(), f.svA2)
	})

	t.Run("negative duration is rejected", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkLessonsDuration,
			`INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, video_media_id, duration_seconds)
			 VALUES ($1, $2, 'neg-duration', 9, 'x', 'video', $3, -1)`,
			ids.New(), f.svA2, f.videoMedia)
	})

	t.Run("deleting a lesson cascades to lesson media", func(t *testing.T) {
		tx := begin(t, dbx)
		assert.EqualValues(t, 1, exec(t, tx, `DELETE FROM lessons WHERE id = $1`, f.lessonA2Doc))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM lesson_media WHERE lesson_id = $1`, f.lessonA2Doc))
	})

	t.Run("media referenced by lesson media cannot be deleted", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkLessonMediaMedia, `DELETE FROM media_files WHERE id = $1`, f.imageMedia)
	})
}

func TestUserConstraints(t *testing.T) {
	dbx, _ := setup(t)
	const insertUser = `INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash)
		VALUES ($1, $2, $3, 'Người dùng', 'student', 'active', 'x')`

	t.Run("normalized email must be lowercase", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkUsersEmailNormalized, insertUser, ids.New(), "An@Example.com", "An@Example.com")
	})

	t.Run("normalized email is unique", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqUsersEmailNormalized, insertUser, ids.New(), "Quan.Tran@goup.vn", "quan.tran@goup.vn")
	})
}

func TestClassConstraints(t *testing.T) {
	dbx, f := setup(t)
	const insertClass = `INSERT INTO classes (id, code, name, course_version_id, teacher_id, status, start_date, end_date, created_by)
		VALUES ($1, $2, 'Lớp', $3, $4, $5, $6, $7, $4)`

	t.Run("end date must follow start date", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkClassesDates, insertClass,
			ids.New(), "class-dates", f.cv1, f.admin, "draft", "2026-10-05", "2026-10-05")
	})

	t.Run("status accepts draft active ended only", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkClassesStatus, insertClass,
			ids.New(), "class-closed", f.cv1, f.admin, "closed", "2026-10-05", "2026-12-05")

		class := ids.New()
		exec(t, tx, insertClass, class, "class-ended", f.cv1, f.admin, "ended", "2026-10-05", "2026-12-05")
		n := exec(t, tx, `INSERT INTO class_members (id, class_id, user_id, status) VALUES ($1, $2, $3, 'completed')`,
			ids.New(), class, f.admin)
		assert.EqualValues(t, 1, n)
	})
}

func TestEmailOutboxContract(t *testing.T) {
	dbx, _ := setup(t)

	t.Run("unknown template is rejected", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkEmailOutboxTemplate,
			`INSERT INTO email_outbox (id, to_email, template) VALUES ($1, 'a@example.com', 'account_disabled')`, ids.New())
	})

	t.Run("claim takes due queued rows and stale sending rows", func(t *testing.T) {
		tx := begin(t, dbx)
		dueQueued, staleSending := ids.New(), ids.New()
		const insert = `INSERT INTO email_outbox (id, to_email, template, status, run_at, locked_until, attempts)
			VALUES ($1, 'a@example.com', 'invite', $2, now() + $3::interval, now() + $4::interval, $5)`
		exec(t, tx, insert, dueQueued, "queued", "-1 minute", "-1 minute", 0)
		exec(t, tx, insert, ids.New(), "queued", "1 hour", "-1 minute", 0)
		exec(t, tx, insert, staleSending, "sending", "-10 minutes", "-1 minute", 1)
		exec(t, tx, insert, ids.New(), "sending", "-10 minutes", "1 minute", 1)
		exec(t, tx, insert, ids.New(), "failed", "-1 hour", "-1 hour", 3)
		exec(t, tx, insert, ids.New(), "sent", "-1 hour", "-1 hour", 0)

		var claimed []uuid.UUID
		require.NoError(t, tx.SelectContext(testContext(t), &claimed, `
			UPDATE email_outbox SET status = 'sending', locked_until = now() + interval '2 minutes'
			WHERE id IN (
				SELECT id FROM email_outbox
				WHERE (status = 'queued' AND run_at <= now()) OR (status = 'sending' AND locked_until < now())
				ORDER BY run_at
				LIMIT $1
				FOR UPDATE SKIP LOCKED)
			RETURNING id`, 10))
		assert.ElementsMatch(t, []uuid.UUID{dueQueued, staleSending}, claimed)
	})
}

// TestConditionalSQLContract chứng minh các mẫu SQL có điều kiện trạng thái mà repository phải dùng chạy đúng trên
// schema thật: không chạm bản đã phát hành, chỉ ghi con khi header còn draft, không phát hành khi còn bài chưa render.
func TestConditionalSQLContract(t *testing.T) {
	dbx, f := setup(t)

	t.Run("header update requires draft", func(t *testing.T) {
		tx := begin(t, dbx)
		const q = `UPDATE stage_versions SET title = 'x', updated_at = now() WHERE id = $1 AND status = 'draft'`
		assert.Zero(t, exec(t, tx, q, f.svA1))
		assert.EqualValues(t, 1, exec(t, tx, q, f.svA2))
	})

	t.Run("child insert requires draft parent", func(t *testing.T) {
		tx := begin(t, dbx)
		const q = `INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, markdown_source)
			SELECT $2, $1, 'new-lesson', 9, 'Bài mới', 'markdown', '# x' FROM stage_versions WHERE id = $1 AND status = 'draft'`
		assert.Zero(t, exec(t, tx, q, f.svA1, ids.New()))
		assert.EqualValues(t, 1, exec(t, tx, q, f.svA2, ids.New()))
	})

	t.Run("child update and delete require draft parent", func(t *testing.T) {
		tx := begin(t, dbx)
		const upd = `UPDATE lessons l SET title = 'x', updated_at = now() FROM stage_versions sv
			WHERE l.id = $1 AND sv.id = l.stage_version_id AND sv.status = 'draft'`
		const del = `DELETE FROM lessons l USING stage_versions sv WHERE l.id = $1 AND sv.id = l.stage_version_id AND sv.status = 'draft'`
		assert.Zero(t, exec(t, tx, upd, f.lessonA1Doc))
		assert.Zero(t, exec(t, tx, del, f.lessonA1Doc))
		assert.EqualValues(t, 1, exec(t, tx, upd, f.lessonA2Doc))
		assert.EqualValues(t, 1, exec(t, tx, del, f.lessonA2Doc))
	})

	t.Run("publish requires every markdown lesson rendered", func(t *testing.T) {
		tx := begin(t, dbx)
		const publish = `UPDATE stage_versions SET status = 'published', published_at = now(), updated_at = now()
			WHERE id = $1 AND status = 'draft'
			  AND NOT EXISTS (SELECT 1 FROM lessons WHERE stage_version_id = $1 AND type = 'markdown' AND markdown_html IS NULL)`
		assert.Zero(t, exec(t, tx, publish, f.svA2))
		exec(t, tx, `UPDATE lessons SET markdown_html = '<h1>A</h1>' WHERE stage_version_id = $1 AND type = 'markdown'`, f.svA2)
		assert.EqualValues(t, 1, exec(t, tx, publish, f.svA2))
		assert.Zero(t, exec(t, tx, publish, f.svA2), "phát hành lần hai không còn draft")
	})
}
