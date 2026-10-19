//go:build integration

package migrations_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
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
const schemaVersion = 14

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

// TestStageUnitsBackfill đứng ngay sau round trip vì cùng lý do: nó migrate down về version 9 (bỏ cột lessons.unit_id)
// rồi up lại. Dữ liệu dựng ở version 9 (2 phiên bản, 3 học liệu, một học viên đã xong một bài bắt buộc); sau migration
// stage_units mỗi phiên bản có đúng một bài học "Nội dung chặng" chứa mọi học liệu, thứ tự và % tiến độ không đổi.
func TestStageUnitsBackfill(t *testing.T) {
	dbx := testdb.Open(t)
	url := testdb.URL(t)
	ctx := testContext(t)
	testdb.Reset(t, dbx)
	t.Cleanup(func() {
		// Trả database về bản mới nhất và rỗng cho test sau, kể cả khi test này dừng giữa chừng.
		require.NoError(t, db.Migrate(context.Background(), url, db.Up, 0))
		testdb.Reset(t, dbx)
	})

	require.NoError(t, db.Migrate(ctx, url, db.Down, schemaVersion-9))
	v, _, err := db.Version(ctx, url)
	require.NoError(t, err)
	require.Equal(t, uint(9), v)

	// Fixture người dùng không chạm bảng chặng nên dùng được ở version 9; phần còn lại dựng inline theo schema cũ.
	testdb.Fixture(t, dbx, "admin_quan_tran")
	testdb.Fixture(t, dbx, "teacher_huong_le")
	testdb.Fixture(t, dbx, "student_an_nguyen")
	admin := uuid.MustParse(testdb.AdminQuanTranID)
	stage, sv1, sv2 := ids.New(), ids.New(), ids.New()
	course, cv, class, member := ids.New(), ids.New(), ids.New(), ids.New()
	done, open, draftLesson := ids.New(), ids.New(), ids.New()
	stmts := []struct {
		q    string
		args []any
	}{
		{`INSERT INTO stages (id, code, name, created_by) VALUES ($1, 'BF', 'Backfill', $2)`, []any{stage, admin}},
		{`INSERT INTO stage_versions (id, stage_id, version_no, status, title, published_at, created_by)
		  VALUES ($1, $2, 1, 'published', 'Backfill', now(), $3)`, []any{sv1, stage, admin}},
		{`INSERT INTO stage_versions (id, stage_id, version_no, status, title, cloned_from_id, created_by)
		  VALUES ($1, $2, 2, 'draft', 'Backfill', $3, $4)`, []any{sv2, stage, sv1, admin}},
		// Chèn ngược thứ tự position để kiểm backfill không dựa vào thứ tự chèn.
		{`INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, required, markdown_source)
		  VALUES ($1, $3, 'bf-open', 2, 'Chưa xong', 'markdown', true, '# b'),
		         ($2, $3, 'bf-done', 1, 'Đã xong', 'markdown', true, '# a'),
		         ($4, $5, 'bf-done', 1, 'Đã xong', 'markdown', true, '# a')`, []any{open, done, sv1, draftLesson, sv2}},
		{`INSERT INTO courses (id, code, name, created_by) VALUES ($1, 'BFC', 'Backfill', $2)`, []any{course, admin}},
		{`INSERT INTO course_versions (id, course_id, version_no, status, title, published_at, created_by)
		  VALUES ($1, $2, 1, 'published', 'Backfill', now(), $3)`, []any{cv, course, admin}},
		{`INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position) VALUES ($1, $2, $3, 1)`,
			[]any{cv, sv1, stage}},
		{`INSERT INTO classes (id, code, name, course_version_id, teacher_id, status, start_date, end_date, created_by)
		  VALUES ($1, 'bf01', 'Backfill', $2, $3, 'active', current_date - 7, current_date + 30, $4)`,
			[]any{class, cv, uuid.MustParse(testdb.TeacherHuongLeID), admin}},
		{`INSERT INTO class_members (id, class_id, user_id, status) VALUES ($1, $2, $3, 'active')`,
			[]any{member, class, uuid.MustParse(testdb.StudentAnNguyenID)}},
		{`INSERT INTO lesson_progress (class_member_id, lesson_id, first_opened_at, completed_at)
		  VALUES ($1, $2, now() - interval '1 hour', now()), ($1, $3, now(), NULL)`, []any{member, done, open}},
	}
	for _, s := range stmts {
		_, err := dbx.ExecContext(ctx, s.q, s.args...)
		require.NoError(t, err, s.q)
	}

	type lessonPos struct {
		ID       uuid.UUID `db:"id"`
		Version  uuid.UUID `db:"stage_version_id"`
		Position int       `db:"position"`
	}
	positions := func() []lessonPos {
		var out []lessonPos
		require.NoError(t, dbx.SelectContext(ctx, &out,
			`SELECT id, stage_version_id, position FROM lessons ORDER BY stage_version_id, position`))
		return out
	}
	// Cùng công thức percentOfRequired của learning (không export): học liệu bắt buộc của phiên bản khóa học lớp.
	percent := func() int {
		var p int
		require.NoError(t, dbx.GetContext(ctx, &p, `
			SELECT (CASE WHEN required_total = 0 THEN 0 ELSE round(100.0 * required_done / required_total) END)::int
			FROM (SELECT count(l.id) AS required_total, count(lp.completed_at) AS required_done
			      FROM class_members cm
			      JOIN classes c ON c.id = cm.class_id
			      LEFT JOIN course_version_stages cvs ON cvs.course_version_id = c.course_version_id
			      LEFT JOIN lessons l ON l.stage_version_id = cvs.stage_version_id AND l.required
			      LEFT JOIN lesson_progress lp ON lp.class_member_id = cm.id AND lp.lesson_id = l.id
			      WHERE cm.id = $1) x`, member))
		return p
	}
	beforePositions, beforePercent := positions(), percent()
	require.Equal(t, 50, beforePercent)

	require.NoError(t, db.Migrate(ctx, url, db.Up, 1))
	v, _, err = db.Version(ctx, url)
	require.NoError(t, err)
	require.Equal(t, uint(10), v)

	var units []struct {
		Version  uuid.UUID `db:"stage_version_id"`
		Key      string    `db:"unit_key"`
		Title    string    `db:"title"`
		Position int       `db:"position"`
	}
	require.NoError(t, dbx.SelectContext(ctx, &units, `SELECT stage_version_id, unit_key, title, position FROM stage_units`))
	require.Len(t, units, 2, "mỗi phiên bản đúng một bài học")
	assert.ElementsMatch(t, []uuid.UUID{sv1, sv2}, []uuid.UUID{units[0].Version, units[1].Version})
	for _, u := range units {
		assert.Equal(t, "noi-dung-chang", u.Key)
		assert.Equal(t, "Nội dung chặng", u.Title)
		assert.Equal(t, 1, u.Position)
	}
	var inOwnUnit int
	require.NoError(t, dbx.GetContext(ctx, &inOwnUnit, `SELECT count(*) FROM lessons l
		JOIN stage_units su ON su.id = l.unit_id AND su.stage_version_id = l.stage_version_id`))
	assert.Equal(t, 3, inOwnUnit, "mọi học liệu nằm trong bài học của chính phiên bản mình")
	assert.Equal(t, beforePositions, positions(), "position học liệu giữ nguyên")
	assert.Equal(t, beforePercent, percent(), "% tiến độ không đổi")

	require.NoError(t, db.Migrate(ctx, url, db.Up, 0))
	v, _, err = db.Version(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, uint(schemaVersion), v)
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

	// CASCADE chỉ cho dữ liệu sở hữu thuần: con của bản nháp phiên bản chặng (bài học, học liệu và nội dung quiz/homework)
	// và chi tiết của một dòng append-only. Mọi FK khác RESTRICT.
	cascade := []string{
		pgerr.FkLessonsStageVersion, pgerr.FkLessonMediaLesson,
		pgerr.FkStageUnitsStageVersion, pgerr.FkLessonsUnit,
		pgerr.FkQuizSettingsLesson, pgerr.FkQuestionsLesson, pgerr.FkQuestionOptionsQuestion,
		pgerr.FkHomeworkSettingsLesson, pgerr.FkRubricCriteriaLesson, pgerr.FkHomeworkAttachmentsLesson,
		pgerr.FkGradeCriterionScoresGrade, pgerr.FkQuizAttemptAnswersAttempt, pgerr.FkQuizReviewNotesReview,
	}
	for _, r := range rows {
		want := "r" // RESTRICT
		if slices.Contains(cascade, r.Name) {
			want = "c"
		}
		assert.Equal(t, want, r.Action, "ON DELETE của %s", r.Name)
		if r.Name == pgerr.FkCvsStageVersion {
			assert.Equal(t, 2, r.NumCols, "fk_cvs_stage_version là FK ghép (stage_version_id, stage_id)")
		}
		if r.Name == pgerr.FkLessonsUnit {
			assert.Equal(t, 2, r.NumCols, "fk_lessons_unit là FK ghép (unit_id, stage_version_id)")
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
// với db-table chưa render, mỗi bản một bài học "Nội dung chặng") và khóa học BASIC v1 dùng DB v1. Phần dựng thêm inline là thứ fixture chung không có:
// media, bài video db-intro ở vị trí 3 của cả hai bản kèm lesson_media, và chặng DS có một bản published.
type fixture struct {
	admin, videoMedia, imageMedia uuid.UUID
	stageA, stageB                uuid.UUID
	svA1, svA2, svB1              uuid.UUID
	unitA1, unitA2                uuid.UUID
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
		unitA1: uuid.MustParse(testdb.UnitDBV1ID), unitA2: uuid.MustParse(testdb.UnitDBV2ID),
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
		{`INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type, video_media_id, duration_seconds)
		  VALUES ($1, $3, $6, 'db-intro', 3, 'Video giới thiệu', 'video', $5, 60),
		         ($2, $4, $7, 'db-intro', 3, 'Video giới thiệu', 'video', $5, 60)`,
			[]any{f.lessonA1Video, f.lessonA2Video, f.svA1, f.svA2, f.videoMedia, f.unitA1, f.unitA2}},
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

	t.Run("deleting an unreferenced draft cascades to units, lessons and their content", func(t *testing.T) {
		tx := begin(t, dbx)
		quiz, _ := addQuiz(t, tx, f.svA2, "quiz-sql", 4)
		homework, _ := addHomework(t, tx, f.svA2, "bt-sql", 5, f.imageMedia)
		assert.EqualValues(t, 1, exec(t, tx, `DELETE FROM stage_versions WHERE id = $1`, f.svA2))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM stage_units WHERE stage_version_id = $1`, f.svA2))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM lessons WHERE stage_version_id = $1`, f.svA2))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM lesson_media WHERE lesson_id IN ($1, $2)`, f.lessonA2Doc, f.lessonA2Video))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM quiz_settings WHERE lesson_id = $1`, quiz))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM questions WHERE lesson_id = $1`, quiz))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM question_options`))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM homework_settings WHERE lesson_id = $1`, homework))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM rubric_criteria WHERE lesson_id = $1`, homework))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM homework_attachments WHERE lesson_id = $1`, homework))
		assert.Equal(t, 3, count(t, tx, `SELECT count(*) FROM lessons WHERE stage_version_id = $1`, f.svA1))
		assert.Equal(t, 1, count(t, tx, `SELECT count(*) FROM stage_units WHERE stage_version_id = $1`, f.svA1))
		assert.Equal(t, 1, count(t, tx, `SELECT count(*) FROM media_files WHERE id = $1`, f.imageMedia), "file kèm RESTRICT, không bị xóa")
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
			`INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type)
			 VALUES ($1, $2, $3, 'no-media', 9, 'x', 'video')`,
			ids.New(), f.svA2, f.unitA2)
	})

	t.Run("negative duration is rejected", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkLessonsDuration,
			`INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type, video_media_id, duration_seconds)
			 VALUES ($1, $2, $3, 'neg-duration', 9, 'x', 'video', $4, -1)`,
			ids.New(), f.svA2, f.unitA2, f.videoMedia)
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
		const q = `INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type, markdown_source)
			SELECT $2, sv.id, (SELECT su.id FROM stage_units su WHERE su.stage_version_id = sv.id AND su.position = 1),
			       'new-lesson', 9, 'Bài mới', 'markdown', '# x'
			FROM stage_versions sv WHERE sv.id = $1 AND sv.status = 'draft'`
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

// addContentLesson chèn học liệu quiz hoặc homework (không có cột nội dung) vào bài học position 1 của phiên bản.
func addContentLesson(t *testing.T, tx *sqlx.Tx, version uuid.UUID, typ, key string, position int) uuid.UUID {
	t.Helper()
	id := ids.New()
	n := exec(t, tx, `INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type)
		SELECT $1, su.stage_version_id, su.id, $3, $4, $3, $5 FROM stage_units su WHERE su.stage_version_id = $2 AND su.position = 1`,
		id, version, key, position, typ)
	require.EqualValues(t, 1, n)
	return id
}

// addQuiz chèn quiz đánh giá có một câu hỏi một lựa chọn với hai phương án; trả id học liệu và id câu hỏi.
func addQuiz(t *testing.T, tx *sqlx.Tx, version uuid.UUID, key string, position int) (lesson, question uuid.UUID) {
	t.Helper()
	lesson, question = addContentLesson(t, tx, version, "quiz", key, position), ids.New()
	exec(t, tx, `INSERT INTO quiz_settings (lesson_id, mode, max_attempts, time_limit_seconds, pass_percent)
		VALUES ($1, 'assessment', 2, 600, 70)`, lesson)
	exec(t, tx, `INSERT INTO questions (id, lesson_id, question_key, type, body_source, points, position)
		VALUES ($1, $2, 'q1', 'single', 'Khóa chính là gì?', 1, 1)`, question, lesson)
	exec(t, tx, `INSERT INTO question_options (id, question_id, body, is_correct, position)
		VALUES ($1, $3, 'Định danh duy nhất của dòng', true, 1), ($2, $3, 'Một chỉ mục bất kỳ', false, 2)`,
		ids.New(), ids.New(), question)
	return lesson, question
}

// addHomework chèn homework có một tiêu chí rubric và một file kèm; trả id học liệu và id tiêu chí.
func addHomework(t *testing.T, tx *sqlx.Tx, version uuid.UUID, key string, position int, attachment uuid.UUID) (lesson, criterion uuid.UUID) {
	t.Helper()
	lesson, criterion = addContentLesson(t, tx, version, "homework", key, position), ids.New()
	exec(t, tx, `INSERT INTO homework_settings (lesson_id, brief_source) VALUES ($1, '# Thiết kế schema')`, lesson)
	exec(t, tx, `INSERT INTO rubric_criteria (id, lesson_id, description, max_points, position)
		VALUES ($1, $2, 'Đủ khóa chính', 10, 1)`, criterion, lesson)
	exec(t, tx, `INSERT INTO homework_attachments (lesson_id, media_id, position) VALUES ($1, $2, 1)`, lesson, attachment)
	return lesson, criterion
}

func TestStageUnitConstraints(t *testing.T) {
	dbx, f := setup(t)
	const insertUnit = `INSERT INTO stage_units (id, stage_version_id, unit_key, title, position) VALUES ($1, $2, $3, $4, $5)`

	t.Run("lesson cannot point at a unit of another version", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkLessonsUnit,
			`INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type, markdown_source)
			 VALUES ($1, $2, $3, 'cross-unit', 9, 'x', 'markdown', '# x')`,
			ids.New(), f.svA2, f.unitA1)
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkLessonsUnit,
			`UPDATE lessons SET unit_id = $1 WHERE id = $2`, f.unitA1, f.lessonA2Doc)
	})

	t.Run("lesson requires a unit", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeNotNullViolation, "",
			`INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, markdown_source)
			 VALUES ($1, $2, 'no-unit', 9, 'x', 'markdown', '# x')`, ids.New(), f.svA2)
	})

	t.Run("unit title is unique per version ignoring case and outer spaces", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqStageUnitsVersionTitle, insertUnit,
			ids.New(), f.svA2, "khac", "  nội dung chặng ", 2)
		exec(t, tx, insertUnit, ids.New(), f.svB1, "noi-dung-chang", "Nội dung chặng", 1)
	})

	t.Run("unit key is unique per version and well formed", func(t *testing.T) {
		tx := begin(t, dbx)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqStageUnitsVersionKey, insertUnit,
			ids.New(), f.svA2, "noi-dung-chang", "Khác", 2)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkStageUnitsKey, insertUnit, ids.New(), f.svA2, "Hoa-Thuong", "Khác", 2)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkStageUnitsTitle, insertUnit, ids.New(), f.svA2, "trong", "   ", 2)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkStageUnitsPosition, insertUnit, ids.New(), f.svA2, "am", "Âm", 0)
	})

	t.Run("deleting a unit cascades to its lessons", func(t *testing.T) {
		tx := begin(t, dbx)
		assert.EqualValues(t, 1, exec(t, tx, `DELETE FROM stage_units WHERE id = $1`, f.unitA2))
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM lessons WHERE stage_version_id = $1`, f.svA2))
	})
}

func TestQuizHomeworkContentConstraints(t *testing.T) {
	dbx, f := setup(t)

	t.Run("quiz and homework lessons carry no video or markdown", func(t *testing.T) {
		tx := begin(t, dbx)
		for _, typ := range []string{"quiz", "homework"} {
			expectPgErr(t, tx, codeCheckViolation, pgerr.CkLessonsTypeContent,
				`INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type, markdown_source)
				 VALUES ($1, $2, $3, 'has-markdown', 9, 'x', $4, '# x')`, ids.New(), f.svA2, f.unitA2, typ)
		}
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkLessonsType,
			`INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type)
			 VALUES ($1, $2, $3, 'survey', 9, 'x', 'survey')`, ids.New(), f.svA2, f.unitA2)
		addQuiz(t, tx, f.svA2, "quiz-ok", 4)
		addHomework(t, tx, f.svA2, "bt-ok", 5, f.imageMedia)
	})

	t.Run("quiz settings rules follow the mode", func(t *testing.T) {
		tx := begin(t, dbx)
		const q = `INSERT INTO quiz_settings (lesson_id, mode, max_attempts, time_limit_seconds, pass_percent) VALUES ($1, $2, $3, $4, $5)`
		cases := []struct {
			name                    string
			mode                    string
			attempts, limit, passed any
			constraint              string
		}{
			{"practice with limits", "practice", 3, nil, nil, pgerr.CkQuizSettingsRules},
			{"assessment without pass percent", "assessment", 3, 600, nil, pgerr.CkQuizSettingsRules},
			{"assessment without attempts", "assessment", nil, 600, 70, pgerr.CkQuizSettingsRules},
			{"assessment with too many attempts", "assessment", 11, 600, 70, pgerr.CkQuizSettingsRules},
			{"assessment with too short time", "assessment", 3, 59, 70, pgerr.CkQuizSettingsRules},
			{"assessment with too long time", "assessment", 3, 10801, 70, pgerr.CkQuizSettingsRules},
			{"assessment with zero pass percent", "assessment", 3, 600, 0, pgerr.CkQuizSettingsRules},
			{"unknown mode", "exam", nil, nil, nil, pgerr.CkQuizSettingsMode},
		}
		for i, c := range cases {
			lesson := addContentLesson(t, tx, f.svA2, "quiz", fmt.Sprintf("quiz-%d", i), 10+i)
			expectPgErr(t, tx, codeCheckViolation, c.constraint, q, lesson, c.mode, c.attempts, c.limit, c.passed)
		}
		practice := addContentLesson(t, tx, f.svA2, "quiz", "luyen-tap", 30)
		assessment := addContentLesson(t, tx, f.svA2, "quiz", "danh-gia", 31)
		exec(t, tx, q, practice, "practice", nil, nil, nil)
		exec(t, tx, q, assessment, "assessment", 10, 10800, 100)
		var shuffle bool
		require.NoError(t, tx.GetContext(testContext(t), &shuffle, `SELECT shuffle FROM quiz_settings WHERE lesson_id = $1`, practice))
		assert.True(t, shuffle, "mặc định trộn câu")
	})

	t.Run("questions and options", func(t *testing.T) {
		tx := begin(t, dbx)
		lesson, question := addQuiz(t, tx, f.svA2, "quiz-q", 4)
		const insertQuestion = `INSERT INTO questions (id, lesson_id, question_key, type, body_source, points, position)
			VALUES ($1, $2, $3, $4, 'Câu hỏi', $5, $6)`
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqQuestionsLessonKey, insertQuestion, ids.New(), lesson, "q1", "single", 1, 2)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqQuestionsLessonPosition, insertQuestion, ids.New(), lesson, "q2", "single", 1, 1)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuestionsType, insertQuestion, ids.New(), lesson, "q2", "essay", 1, 2)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuestionsPoints, insertQuestion, ids.New(), lesson, "q2", "multi", 0, 2)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuestionsPoints, insertQuestion, ids.New(), lesson, "q2", "multi", 101, 2)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuestionsKey, insertQuestion, ids.New(), lesson, "Q 2", "multi", 1, 2)

		const insertOption = `INSERT INTO question_options (id, question_id, body, position) VALUES ($1, $2, $3, $4)`
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuestionOptionsBody, insertOption, ids.New(), question, " ", 3)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuestionOptionsBody, insertOption, ids.New(), question, strings.Repeat("a", 501), 3)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqQuestionOptionsQuestionPosition, insertOption, ids.New(), question, "Trùng vị trí", 1)

		// Đổi chỗ hai phương án trong một transaction nhờ ràng buộc position DEFERRABLE.
		exec(t, tx, `SET CONSTRAINTS `+pgerr.UqQuestionOptionsQuestionPosition+` DEFERRED`)
		exec(t, tx, `UPDATE question_options SET position = 3 - position WHERE question_id = $1`, question)
	})

	t.Run("homework settings, rubric and attachments", func(t *testing.T) {
		tx := begin(t, dbx)
		doc := ids.New()
		exec(t, tx, `INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by, ready_at)
			VALUES ($1, 'document', 'test/de-bai.pdf', 'de-bai.pdf', 'application/pdf', 10, 'ready', $2, now())`, doc, f.admin)
		lesson, _ := addHomework(t, tx, f.svA2, "bt-rubric", 4, doc)
		assert.Equal(t, 70, count(t, tx, `SELECT pass_percent FROM homework_settings WHERE lesson_id = $1`, lesson))
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkHomeworkSettingsPassPercent,
			`UPDATE homework_settings SET pass_percent = 0 WHERE lesson_id = $1`, lesson)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkRubricCriteriaMaxPoints,
			`INSERT INTO rubric_criteria (id, lesson_id, description, max_points, position) VALUES ($1, $2, 'x', 0, 2)`, ids.New(), lesson)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqRubricCriteriaLessonPosition,
			`INSERT INTO rubric_criteria (id, lesson_id, description, max_points, position) VALUES ($1, $2, 'x', 5, 1)`, ids.New(), lesson)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.PkHomeworkAttachments,
			`INSERT INTO homework_attachments (lesson_id, media_id, position) VALUES ($1, $2, 2)`, lesson, doc)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqHomeworkAttachmentsLessonPosition,
			`INSERT INTO homework_attachments (lesson_id, media_id, position) VALUES ($1, $2, 1)`, lesson, f.videoMedia)
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkHomeworkAttachmentsMedia, `DELETE FROM media_files WHERE id = $1`, doc)
	})

	t.Run("media kind accepts document", func(t *testing.T) {
		tx := begin(t, dbx)
		const q = `INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by)
			VALUES ($1, $2, $3, 'de-bai.pdf', 'application/pdf', 10, 'pending', $4)`
		exec(t, tx, q, ids.New(), "document", "test/de-bai.pdf", f.admin)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkMediaFilesKind, q, ids.New(), "archive", "test/de-bai.zip", f.admin)
	})
}

// learner dựng trong tx một học viên của lớp basic01 (BASIC v1 gồm DB v1) cùng một quiz và một homework của DB v1.
type learner struct {
	member, quiz, question, homework, criterion uuid.UUID
}

func newLearner(t *testing.T, tx *sqlx.Tx, f fixture) learner {
	t.Helper()
	testdb.Fixture(t, tx, "class_basic01_active")
	l := learner{member: uuid.MustParse(testdb.MemberAnBasic01ID)}
	l.quiz, l.question = addQuiz(t, tx, f.svA1, "quiz-db", 4)
	l.homework, l.criterion = addHomework(t, tx, f.svA1, "bt-db", 5, f.imageMedia)
	return l
}

func TestSubmissionConstraints(t *testing.T) {
	dbx, f := setup(t)
	const insertSubmission = `INSERT INTO submissions (id, class_member_id, lesson_id, attempt_no, repo_url, commit_ref, submitted_at, status)
		VALUES ($1, $2, $3, $4, $5, $6, now(), $7)`

	t.Run("one pending submission per member and lesson", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		first := ids.New()
		exec(t, tx, insertSubmission, first, l.member, l.homework, 1, "https://github.com/an/db", "main", "pending")
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqSubmissionsOnePending, insertSubmission,
			ids.New(), l.member, l.homework, 2, nil, nil, "pending")
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqSubmissionsMemberLessonNo, insertSubmission,
			ids.New(), l.member, l.homework, 1, nil, nil, "passed")
		// Nộp lại: bản cũ sang superseded rồi chèn bản mới trong cùng transaction.
		exec(t, tx, `UPDATE submissions SET status = 'superseded' WHERE id = $1`, first)
		exec(t, tx, insertSubmission, ids.New(), l.member, l.homework, 2, nil, nil, "pending")
	})

	t.Run("submission row checks", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkSubmissionsCommitNeedsRepo, insertSubmission,
			ids.New(), l.member, l.homework, 1, nil, "abc123", "pending")
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkSubmissionsRepoUrl, insertSubmission,
			ids.New(), l.member, l.homework, 1, "http://github.com/an/db", nil, "pending")
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkSubmissionsAttemptNo, insertSubmission,
			ids.New(), l.member, l.homework, 0, nil, nil, "pending")
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkSubmissionsStatus, insertSubmission,
			ids.New(), l.member, l.homework, 1, nil, nil, "draft")
	})

	t.Run("a file belongs to one submission", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		a, b := ids.New(), ids.New()
		exec(t, tx, insertSubmission, a, l.member, l.homework, 1, nil, nil, "rework")
		exec(t, tx, insertSubmission, b, l.member, l.homework, 2, nil, nil, "pending")
		const attach = `INSERT INTO submission_files (submission_id, media_id, position) VALUES ($1, $2, $3)`
		exec(t, tx, attach, a, f.videoMedia, 1)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqSubmissionFilesMedia, attach, b, f.videoMedia, 1)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqSubmissionFilesPosition, attach, a, f.imageMedia, 1)
	})

	t.Run("grades are checked and criterion scores follow the grade", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		sub := ids.New()
		exec(t, tx, insertSubmission, sub, l.member, l.homework, 1, nil, nil, "pending")
		const grade = `INSERT INTO submission_grades (id, submission_id, graded_by, total_points, max_points, result, comment, graded_at)
			VALUES ($1, $2, $3, $4, 10, $5, $6, now())`
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkSubmissionGradesPoints, grade, ids.New(), sub, f.admin, 11, "passed", "Tốt")
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkSubmissionGradesPoints, grade, ids.New(), sub, f.admin, -1, "rework", "Làm lại")
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkSubmissionGradesComment, grade, ids.New(), sub, f.admin, 8, "passed", "  ")
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkSubmissionGradesResult, grade, ids.New(), sub, f.admin, 8, "superseded", "Tốt")
		g := ids.New()
		exec(t, tx, grade, g, sub, f.admin, 8, "passed", "Tốt")
		const score = `INSERT INTO grade_criterion_scores (grade_id, criterion_id, points) VALUES ($1, $2, $3)`
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkGradeCriterionScoresPoints, score, g, l.criterion, -1)
		exec(t, tx, score, g, l.criterion, 8)
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkSubmissionGradesSubmission, `DELETE FROM submissions WHERE id = $1`, sub)
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkGradeCriterionScoresCriterion,
			`DELETE FROM rubric_criteria WHERE id = $1`, l.criterion)
		exec(t, tx, `DELETE FROM submission_grades WHERE id = $1`, g)
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM grade_criterion_scores WHERE grade_id = $1`, g))
	})

	t.Run("lesson with submissions cannot be deleted", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		exec(t, tx, insertSubmission, ids.New(), l.member, l.homework, 1, nil, nil, "pending")
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkSubmissionsLesson, `DELETE FROM lessons WHERE id = $1`, l.homework)
	})
}

func TestQuizAttemptConstraints(t *testing.T) {
	dbx, f := setup(t)
	const insertAttempt = `INSERT INTO quiz_attempts (id, class_member_id, lesson_id, attempt_no, question_order, option_order,
		started_at, deadline_at, submitted_at, score, max_score, passed, review_status)
		VALUES ($1, $2, $3, $4, ARRAY[$5::uuid], '{}'::jsonb, now(), now() + interval '10 minutes', $6, $7, 1, $8, $9)`

	t.Run("one open attempt per member and quiz", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		first := ids.New()
		exec(t, tx, insertAttempt, first, l.member, l.quiz, 1, l.question, nil, nil, nil, nil)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqQuizAttemptsOneOpen, insertAttempt,
			ids.New(), l.member, l.quiz, 2, l.question, nil, nil, nil, nil)
		exec(t, tx, `UPDATE quiz_attempts SET submitted_at = now(), score = 1, passed = true, review_status = 'pending' WHERE id = $1`, first)
		exec(t, tx, insertAttempt, ids.New(), l.member, l.quiz, 2, l.question, nil, nil, nil, nil)
	})

	t.Run("one pending review per member and quiz", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		first := ids.New()
		exec(t, tx, insertAttempt, first, l.member, l.quiz, 1, l.question, time.Now(), 1, true, "pending")
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqQuizAttemptsOnePending, insertAttempt,
			ids.New(), l.member, l.quiz, 2, l.question, time.Now(), 0, false, "pending")
		exec(t, tx, `UPDATE quiz_attempts SET review_status = 'superseded' WHERE id = $1`, first)
		exec(t, tx, insertAttempt, ids.New(), l.member, l.quiz, 2, l.question, time.Now(), 0, false, "pending")
	})

	t.Run("attempt row checks", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuizAttemptsScored, insertAttempt,
			ids.New(), l.member, l.quiz, 1, l.question, nil, 1, nil, nil)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuizAttemptsScored, insertAttempt,
			ids.New(), l.member, l.quiz, 1, l.question, time.Now(), nil, nil, nil)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuizAttemptsReviewAfterSubmit, insertAttempt,
			ids.New(), l.member, l.quiz, 1, l.question, nil, nil, nil, "pending")
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuizAttemptsReviewStatus, insertAttempt,
			ids.New(), l.member, l.quiz, 1, l.question, time.Now(), 1, true, "rejected")
		a := ids.New()
		exec(t, tx, insertAttempt, a, l.member, l.quiz, 1, l.question, time.Now(), 1, true, nil)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqQuizAttemptsMemberLessonNo, insertAttempt,
			ids.New(), l.member, l.quiz, 1, l.question, time.Now(), 1, true, nil)
	})

	t.Run("answers, reviews and notes", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		a := ids.New()
		exec(t, tx, insertAttempt, a, l.member, l.quiz, 1, l.question, time.Now(), 1, true, "pending")
		exec(t, tx, `INSERT INTO quiz_attempt_answers (attempt_id, question_id, selected_option_ids, saved_at)
			VALUES ($1, $2, '{}'::uuid[], now())`, a, l.question)
		const review = `INSERT INTO quiz_attempt_reviews (id, attempt_id, reviewer_id, comment) VALUES ($1, $2, $3, $4)`
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuizAttemptReviewsComment, review, ids.New(), a, f.admin, " ")
		r := ids.New()
		exec(t, tx, review, r, a, f.admin, "Đã xem")
		const note = `INSERT INTO quiz_review_notes (review_id, question_id, note) VALUES ($1, $2, $3)`
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuizReviewNotesNote, note, r, l.question, "")
		exec(t, tx, note, r, l.question, "Xem lại khái niệm khóa chính")
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqQuizReviewNotesReviewQuestion, note, r, l.question, "Lần hai")
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkQuizAttemptReviewsAttempt, `DELETE FROM quiz_attempts WHERE id = $1`, a)
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkQuizAttemptAnswersQuestion, `DELETE FROM questions WHERE id = $1`, l.question)
		exec(t, tx, `DELETE FROM quiz_attempt_reviews WHERE id = $1`, r)
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM quiz_review_notes WHERE review_id = $1`, r))
		exec(t, tx, `DELETE FROM quiz_attempts WHERE id = $1`, a)
		assert.Zero(t, count(t, tx, `SELECT count(*) FROM quiz_attempt_answers WHERE attempt_id = $1`, a))
	})

	t.Run("extra attempt grants", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		const grant = `INSERT INTO quiz_attempt_grants (id, class_member_id, lesson_id, extra_attempts, reason, granted_by, granted_at)
			VALUES ($1, $2, $3, $4, $5, $6, now())`
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuizAttemptGrantsExtra, grant, ids.New(), l.member, l.quiz, 0, "Ốm", f.admin)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuizAttemptGrantsExtra, grant, ids.New(), l.member, l.quiz, 4, "Ốm", f.admin)
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkQuizAttemptGrantsReason, grant, ids.New(), l.member, l.quiz, 1, " ", f.admin)
		exec(t, tx, grant, ids.New(), l.member, l.quiz, 3, "Mất mạng giữa giờ", f.admin)
	})
}

func TestStageReviewsJobsAndEmails(t *testing.T) {
	dbx, f := setup(t)

	t.Run("one stage review per member and stage version", func(t *testing.T) {
		tx := begin(t, dbx)
		testdb.Fixture(t, tx, "class_basic01_active")
		member := uuid.MustParse(testdb.MemberAnBasic01ID)
		const q = `INSERT INTO stage_reviews (id, class_member_id, stage_version_id, body, author_id) VALUES ($1, $2, $3, $4, $5)`
		expectPgErr(t, tx, codeCheckViolation, pgerr.CkStageReviewsBody, q, ids.New(), member, f.svA1, "  ", f.admin)
		exec(t, tx, q, ids.New(), member, f.svA1, "Nắm chắc thiết kế bảng", f.admin)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.UqStageReviewsMemberVersion, q, ids.New(), member, f.svA1, "Lần hai", f.admin)
		assert.Equal(t, 1, count(t, tx, `SELECT revision FROM stage_reviews WHERE class_member_id = $1`, member))
	})

	t.Run("scheduled job run key claims once", func(t *testing.T) {
		tx := begin(t, dbx)
		const claim = `INSERT INTO scheduled_job_runs (run_key, ran_at) VALUES ($1, now()) ON CONFLICT (run_key) DO NOTHING`
		assert.EqualValues(t, 1, exec(t, tx, claim, "grading_digest:t1:2026-10-18"))
		assert.Zero(t, exec(t, tx, claim, "grading_digest:t1:2026-10-18"))
	})

	t.Run("new email templates are accepted", func(t *testing.T) {
		tx := begin(t, dbx)
		for _, tpl := range []string{"homework_graded", "quiz_reviewed", "grading_digest", "deadline_reminder"} {
			exec(t, tx, `INSERT INTO email_outbox (id, to_email, template) VALUES ($1, 'a@example.com', $2)`, ids.New(), tpl)
		}
	})

	t.Run("class lesson deadline is one per class and lesson", func(t *testing.T) {
		tx := begin(t, dbx)
		l := newLearner(t, tx, f)
		const q = `INSERT INTO class_lesson_deadlines (class_id, lesson_id, due_at, set_by) VALUES ($1, $2, now() + interval '7 days', $3)`
		class := uuid.MustParse(testdb.ClassBasic01ID)
		exec(t, tx, q, class, l.homework, f.admin)
		expectPgErr(t, tx, codeUniqueViolation, pgerr.PkClassLessonDeadlines, q, class, l.homework, f.admin)
		expectPgErr(t, tx, codeForeignKeyViolation, pgerr.FkClassLessonDeadlinesLesson, `DELETE FROM lessons WHERE id = $1`, l.homework)
	})
}
