package courses

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
)

// PGCourseRepo là CourseRepo trên PostgreSQL.
type PGCourseRepo struct{}

// PGCourseVersionRepo là CourseVersionRepo trên PostgreSQL. Mọi câu ghi lên course_version_stages mang điều kiện
// header còn draft ngay trong SQL và chạy qua db.ExecAffectOne: DB không có trigger nên đây là lớp chặn cuối.
type PGCourseVersionRepo struct{}

var (
	_ CourseRepo        = PGCourseRepo{}
	_ CourseVersionRepo = PGCourseVersionRepo{}
)

// constraintOf trả mã lỗi và tên constraint của lỗi PostgreSQL (rỗng nếu không phải lỗi PostgreSQL).
func constraintOf(err error) (code, constraint string) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code, pg.ConstraintName
	}
	return "", ""
}

func (PGCourseRepo) Create(ctx context.Context, ex db.Executor, c *Course) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO courses (id, code, name, created_by, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5)`,
		c.id, c.code.String(), c.name, c.createdBy, c.createdAt)
	if err != nil {
		if code, cn := constraintOf(err); code == pgerrcode.UniqueViolation && cn == pgerr.UqCoursesCode {
			return ErrCodeTaken
		}
		return fmt.Errorf("courses: tạo khóa học: %w", pgerr.Map(err))
	}
	return nil
}

// courseColumns đọc mô tả từ phiên bản mới nhất vì bảng courses không có cột mô tả.
const courseColumns = `c.id, c.code, c.name, c.created_at,
	COALESCE((SELECT cv.description FROM course_versions cv WHERE cv.course_id = c.id ORDER BY cv.version_no DESC LIMIT 1), '') AS description`

type courseRecord struct {
	ID          uuid.UUID `db:"id"`
	Code        string    `db:"code"`
	Name        string    `db:"name"`
	CreatedAt   time.Time `db:"created_at"`
	Description string    `db:"description"`
}

func (r courseRecord) row() CourseRow {
	return CourseRow{ID: r.ID, Code: r.Code, Name: r.Name, Description: r.Description, CreatedAt: r.CreatedAt}
}

func (PGCourseRepo) ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*CourseRow, error) {
	var rec courseRecord
	err := sqlx.GetContext(ctx, ex, &rec, `SELECT `+courseColumns+` FROM courses c WHERE c.id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCourseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("courses: đọc khóa học: %w", err)
	}
	row := rec.row()
	return &row, nil
}

func (PGCourseRepo) Delete(ctx context.Context, ex db.Executor, id uuid.UUID) error {
	err := db.ExecAffectOne(ctx, ex,
		`DELETE FROM courses c WHERE c.id = $1 AND NOT EXISTS (SELECT 1 FROM course_versions cv WHERE cv.course_id = c.id)`, id)
	if err != nil {
		return fmt.Errorf("courses: xóa khóa học còn phiên bản hoặc không tồn tại: %w", pgerr.Map(err))
	}
	return nil
}

func (PGCourseRepo) LockForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) error {
	var got uuid.UUID
	err := sqlx.GetContext(ctx, ex, &got, `SELECT id FROM courses WHERE id = $1 FOR UPDATE`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCourseNotFound
	}
	if err != nil {
		return fmt.Errorf("courses: khóa khóa học: %w", err)
	}
	return nil
}

func (PGCourseRepo) List(ctx context.Context, ex db.Executor, q ListQuery) ([]CourseListRow, error) {
	const filter = `($1 = '' OR c.code ILIKE '%' || $1 || '%' OR c.name ILIKE '%' || $1 || '%')`
	term := escapeLike(strings.TrimSpace(q.Q))

	var recs []courseRecord
	if err := sqlx.SelectContext(ctx, ex, &recs, `SELECT `+courseColumns+` FROM courses c WHERE `+filter+` ORDER BY c.code`, term); err != nil {
		return nil, fmt.Errorf("courses: liệt kê khóa học: %w", err)
	}
	versions, err := versionRows(ctx, ex, `JOIN courses c ON c.id = cv.course_id WHERE `+filter, term)
	if err != nil {
		return nil, err
	}
	classes, err := classesUsing(ctx, ex, `JOIN courses c ON c.id = cv.course_id WHERE `+filter, term)
	if err != nil {
		return nil, err
	}
	byCourse := make(map[uuid.UUID][]VersionListRow, len(recs))
	for _, v := range versions {
		byCourse[v.CourseID] = append(byCourse[v.CourseID], v)
	}
	classesByCourse := make(map[uuid.UUID][]ClassUsingRow, len(recs))
	for _, cl := range classes {
		classesByCourse[cl.CourseID] = append(classesByCourse[cl.CourseID], cl)
	}
	out := make([]CourseListRow, 0, len(recs))
	for _, r := range recs {
		out = append(out, CourseListRow{CourseRow: r.row(), Versions: byCourse[r.ID], ClassesUsing: classesByCourse[r.ID]})
	}
	return out, nil
}

// escapeLike thoát ký tự đặc biệt của LIKE để từ khóa tìm kiếm được so khớp nguyên văn.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

type versionRecord struct {
	ID           uuid.UUID  `db:"id"`
	CourseID     uuid.UUID  `db:"course_id"`
	VersionNo    int        `db:"version_no"`
	Status       string     `db:"status"`
	Title        string     `db:"title"`
	Description  string     `db:"description"`
	ClonedFromID *uuid.UUID `db:"cloned_from_id"`
	PublishedAt  *time.Time `db:"published_at"`
	ArchivedAt   *time.Time `db:"archived_at"`
	CreatedBy    uuid.UUID  `db:"created_by"`
}

const versionColumns = `id, course_id, version_no, status, title, description, cloned_from_id, published_at, archived_at, created_by`

func (r PGCourseVersionRepo) ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*CourseVersion, error) {
	return r.load(ctx, ex, `WHERE id = $1`, id)
}

// ByIDForUpdate khóa header FOR UPDATE: mọi use case ghi mở bằng hàm này để tuần tự hóa với Publish/Archive.
func (r PGCourseVersionRepo) ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*CourseVersion, error) {
	return r.load(ctx, ex, `WHERE id = $1 FOR UPDATE`, id)
}

func (r PGCourseVersionRepo) LatestPublished(ctx context.Context, ex db.Executor, courseID uuid.UUID) (*CourseVersion, error) {
	return r.optional(r.load(ctx, ex, `WHERE course_id = $1 AND status = 'published' ORDER BY version_no DESC LIMIT 1`, courseID))
}

func (r PGCourseVersionRepo) DraftOf(ctx context.Context, ex db.Executor, courseID uuid.UUID) (*CourseVersion, error) {
	return r.optional(r.load(ctx, ex, `WHERE course_id = $1 AND status = 'draft'`, courseID))
}

// optional đổi ErrVersionNotFound thành (nil, nil) cho truy vấn "có thể không có".
func (PGCourseVersionRepo) optional(v *CourseVersion, err error) (*CourseVersion, error) {
	if errors.Is(err, ErrVersionNotFound) {
		return nil, nil
	}
	return v, err
}

// load đọc header theo where (nối sau FROM course_versions) rồi danh sách chặng theo position.
func (PGCourseVersionRepo) load(ctx context.Context, ex db.Executor, where string, args ...any) (*CourseVersion, error) {
	var rec versionRecord
	err := sqlx.GetContext(ctx, ex, &rec, `SELECT `+versionColumns+` FROM course_versions `+where, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("courses: đọc phiên bản: %w", err)
	}
	status, err := domain.ParseVersionStatus(rec.Status)
	if err != nil {
		return nil, fmt.Errorf("courses: phiên bản %s: %w", rec.ID, err)
	}
	var recs []struct {
		StageID        uuid.UUID `db:"stage_id"`
		StageVersionID uuid.UUID `db:"stage_version_id"`
		Position       int       `db:"position"`
	}
	if err := sqlx.SelectContext(ctx, ex, &recs, `SELECT stage_id, stage_version_id, position
		FROM course_version_stages WHERE course_version_id = $1 ORDER BY position`, rec.ID); err != nil {
		return nil, fmt.Errorf("courses: đọc chặng của phiên bản: %w", err)
	}
	stages := make([]CourseVersionStage, 0, len(recs))
	for _, s := range recs {
		stages = append(stages, CourseVersionStage(s))
	}
	return &CourseVersion{
		id: rec.ID, courseID: rec.CourseID, versionNo: domain.VersionNo(rec.VersionNo), status: status,
		title: rec.Title, description: rec.Description, clonedFromID: rec.ClonedFromID, stages: stages,
		publishedAt: rec.PublishedAt, archivedAt: rec.ArchivedAt, createdBy: rec.CreatedBy,
	}, nil
}

// Create chèn header luôn với status 'draft' rồi danh sách chặng. Bản nháp thứ hai của cùng khóa học chạm partial
// unique index uq_course_versions_one_draft; ON CONFLICT DO NOTHING giữ giao dịch dùng được để đọc bản nháp đang có.
func (r PGCourseVersionRepo) Create(ctx context.Context, ex db.Executor, v *CourseVersion) error {
	if v.status != domain.VersionDraft {
		return ErrVersionImmutable
	}
	if _, err := ex.ExecContext(ctx, `SET CONSTRAINTS `+pgerr.UqCvsVersionPosition+` DEFERRED`); err != nil {
		return fmt.Errorf("courses: hoãn kiểm tra vị trí: %w", err)
	}
	res, err := ex.ExecContext(ctx, `INSERT INTO course_versions
		(id, course_id, version_no, status, title, description, cloned_from_id, created_by)
		VALUES ($1, $2, $3, 'draft', $4, $5, $6, $7)
		ON CONFLICT (course_id) WHERE status = 'draft' DO NOTHING`,
		v.id, v.courseID, v.versionNo.Int(), v.title, v.description, v.clonedFromID, v.createdBy)
	if err != nil {
		return fmt.Errorf("courses: tạo phiên bản: %w", pgerr.Map(err))
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("courses: tạo phiên bản: %w", err)
	} else if n == 0 {
		d, err := r.DraftOf(ctx, ex, v.courseID)
		if err != nil {
			return err
		}
		if d == nil {
			return errors.New("courses: tạo phiên bản: xung đột bản nháp nhưng không đọc được bản nháp")
		}
		return &ErrDraftExists{DraftID: d.id, No: d.versionNo}
	}
	return insertStages(ctx, ex, v)
}

// SaveDraft ghi lại toàn bộ danh sách chặng (xóa rồi chèn, mọi câu có điều kiện draft) rồi mới UPDATE header
// (không đổi status). Vị trí được hoãn kiểm tới cuối transaction để sắp lại không va uq_cvs_version_position.
func (PGCourseVersionRepo) SaveDraft(ctx context.Context, ex db.Executor, v *CourseVersion) error {
	if _, err := ex.ExecContext(ctx, `SET CONSTRAINTS `+pgerr.UqCvsVersionPosition+` DEFERRED`); err != nil {
		return fmt.Errorf("courses: hoãn kiểm tra vị trí: %w", err)
	}
	if err := deleteStages(ctx, ex, v.id); err != nil {
		return err
	}
	if err := insertStages(ctx, ex, v); err != nil {
		return err
	}
	err := db.ExecAffectOne(ctx, ex,
		`UPDATE course_versions SET title = $2, description = $3, updated_at = now() WHERE id = $1 AND status = 'draft'`,
		v.id, v.title, v.description)
	return immutableIfNoRows(err, "lưu bản nháp")
}

// deleteStages xóa mọi chặng của phiên bản khi header còn draft. Bản nháp có thể không có chặng nào nên không đếm
// dòng bị xóa mà đếm header draft khớp (CTE ghi luôn chạy): 0 → ErrVersionImmutable, không dòng nào bị xóa.
func deleteStages(ctx context.Context, ex db.Executor, versionID uuid.UUID) error {
	var drafts int
	err := sqlx.GetContext(ctx, ex, &drafts, `WITH h AS (SELECT id FROM course_versions WHERE id = $1 AND status = 'draft'),
		d AS (DELETE FROM course_version_stages cvs USING h WHERE cvs.course_version_id = h.id)
		SELECT count(*) FROM h`, versionID)
	if err != nil {
		return fmt.Errorf("courses: xóa chặng của phiên bản: %w", pgerr.Map(err))
	}
	if drafts == 0 {
		return ErrVersionImmutable
	}
	return nil
}

func insertStages(ctx context.Context, ex db.Executor, v *CourseVersion) error {
	for _, s := range v.stages {
		if err := insertStage(ctx, ex, v.id, s); err != nil {
			return err
		}
	}
	return nil
}

// insertStage chỉ khớp khi header còn draft; stage_id lệch stage_versions.stage_id bị FK ghép fk_cvs_stage_version chặn.
func insertStage(ctx context.Context, ex db.Executor, versionID uuid.UUID, s CourseVersionStage) error {
	err := db.ExecAffectOne(ctx, ex, `INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position)
		SELECT cv.id, $2::uuid, $3::uuid, $4::int FROM course_versions cv WHERE cv.id = $1 AND cv.status = 'draft'`,
		versionID, s.StageVersionID, s.StageID, s.Position)
	return immutableIfNoRows(err, "gắn chặng")
}

func immutableIfNoRows(err error, what string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, db.ErrNoRowsAffected):
		return ErrVersionImmutable
	}
	if code, cn := constraintOf(err); code == pgerrcode.UniqueViolation && (cn == pgerr.UqCvsVersionStage || cn == pgerr.PkCourseVersionStages) {
		return ErrDuplicateStage
	}
	return fmt.Errorf("courses: %s: %w", what, pgerr.Map(err))
}

// TransitionStatus chỉ UPDATE header với WHERE status=from; không chạm course_version_stages.
func (PGCourseVersionRepo) TransitionStatus(ctx context.Context, ex db.Executor, id uuid.UUID, from, to domain.VersionStatus, publishedAt *time.Time) error {
	if !from.CanTransitionTo(to) {
		return domain.ErrInvalidTransition
	}
	if to == domain.VersionPublished && publishedAt == nil {
		return errors.New("courses: phát hành cần thời điểm phát hành")
	}
	err := db.ExecAffectOne(ctx, ex, `UPDATE course_versions SET status = $3::text, published_at = COALESCE($4::timestamptz, published_at),
		archived_at = CASE WHEN $3::text = 'archived' THEN now() ELSE archived_at END, updated_at = now()
		WHERE id = $1 AND status = $2::text`, id, from.String(), to.String(), publishedAt)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, db.ErrNoRowsAffected):
		return fmt.Errorf("courses: chuyển trạng thái phiên bản: %w", pgerr.Map(err))
	case to == domain.VersionPublished:
		return ErrPublishNotDraft
	case to == domain.VersionArchived:
		return ErrArchiveNotPublished
	default:
		return domain.ErrInvalidTransition
	}
}

// Delete chỉ xóa bản nháp: chặng trước (FK RESTRICT, câu có điều kiện draft) rồi header; bản không nháp →
// ErrVersionImmutable, lớp còn tham chiếu → domain.ErrInUse.
func (PGCourseVersionRepo) Delete(ctx context.Context, ex db.Executor, id uuid.UUID) error {
	if err := deleteStages(ctx, ex, id); err != nil {
		return err
	}
	err := db.ExecAffectOne(ctx, ex, `DELETE FROM course_versions WHERE id = $1 AND status = 'draft'`, id)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, db.ErrNoRowsAffected):
		return ErrVersionImmutable
	}
	if code, _ := constraintOf(err); code == pgerrcode.ForeignKeyViolation {
		return fmt.Errorf("courses: xóa phiên bản: %w", domain.ErrInUse)
	}
	return fmt.Errorf("courses: xóa phiên bản: %w", pgerr.Map(err))
}

func (PGCourseVersionRepo) CountByCourse(ctx context.Context, ex db.Executor, courseID uuid.UUID) (int, error) {
	var n int
	if err := sqlx.GetContext(ctx, ex, &n, `SELECT count(*) FROM course_versions WHERE course_id = $1`, courseID); err != nil {
		return 0, fmt.Errorf("courses: đếm phiên bản: %w", err)
	}
	return n, nil
}

func (PGCourseVersionRepo) NextVersionNo(ctx context.Context, ex db.Executor, courseID uuid.UUID) (domain.VersionNo, error) {
	if err := (PGCourseRepo{}).LockForUpdate(ctx, ex, courseID); err != nil {
		return 0, err
	}
	var next int
	if err := sqlx.GetContext(ctx, ex, &next,
		`SELECT COALESCE(max(version_no), 0) + 1 FROM course_versions WHERE course_id = $1`, courseID); err != nil {
		return 0, fmt.Errorf("courses: số phiên bản kế tiếp: %w", err)
	}
	return domain.ParseVersionNo(next)
}

type versionRowRecord struct {
	ID                  uuid.UUID  `db:"id"`
	CourseID            uuid.UUID  `db:"course_id"`
	VersionNo           int        `db:"version_no"`
	Status              string     `db:"status"`
	PublishedAt         *time.Time `db:"published_at"`
	ClonedFromVersionNo *int       `db:"cloned_from_version_no"`
	StageCount          int        `db:"stage_count"`
	OutdatedStageCount  int        `db:"outdated_stage_count"`
	ClassCount          int        `db:"class_count"`
}

// latestPublishedOfStage là version_no phát hành lớn nhất của chặng x.stage_id (NULL nếu chưa có).
const latestPublishedOfStage = `(SELECT max(p.version_no) FROM stage_versions p WHERE p.stage_id = x.stage_id AND p.status = 'published')`

// versionRows đọc phiên bản (mới nhất trước) kèm số chặng, số chặng đã có bản mới hơn và số lớp; where nối sau
// FROM course_versions cv.
func versionRows(ctx context.Context, ex db.Executor, where string, args ...any) ([]VersionListRow, error) {
	var recs []versionRowRecord
	err := sqlx.SelectContext(ctx, ex, &recs, `SELECT cv.id, cv.course_id, cv.version_no, cv.status, cv.published_at,
		src.version_no AS cloned_from_version_no,
		(SELECT count(*) FROM course_version_stages x WHERE x.course_version_id = cv.id) AS stage_count,
		(SELECT count(*) FROM course_version_stages x JOIN stage_versions sv ON sv.id = x.stage_version_id
		  WHERE x.course_version_id = cv.id AND sv.version_no < COALESCE(`+latestPublishedOfStage+`, 0)) AS outdated_stage_count,
		(SELECT count(*) FROM classes cl WHERE cl.course_version_id = cv.id) AS class_count
		FROM course_versions cv LEFT JOIN course_versions src ON src.id = cv.cloned_from_id `+where+`
		ORDER BY cv.course_id, cv.version_no DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("courses: liệt kê phiên bản: %w", err)
	}
	out := make([]VersionListRow, 0, len(recs))
	for _, r := range recs {
		row := VersionListRow{
			ID: r.ID, CourseID: r.CourseID, VersionNo: domain.VersionNo(r.VersionNo), Status: domain.VersionStatus(r.Status),
			PublishedAt: r.PublishedAt, StageCount: r.StageCount, OutdatedStageCount: r.OutdatedStageCount, ClassCount: r.ClassCount,
		}
		if r.ClonedFromVersionNo != nil {
			no := domain.VersionNo(*r.ClonedFromVersionNo)
			row.ClonedFromVersionNo = &no
		}
		out = append(out, row)
	}
	return out, nil
}

func (PGCourseVersionRepo) ListByCourse(ctx context.Context, ex db.Executor, courseID uuid.UUID) ([]VersionListRow, error) {
	return versionRows(ctx, ex, `WHERE cv.course_id = $1`, courseID)
}

type classUsingRecord struct {
	CourseID  uuid.UUID `db:"course_id"`
	ClassID   uuid.UUID `db:"class_id"`
	Code      string    `db:"code"`
	Name      string    `db:"name"`
	VersionNo int       `db:"version_no"`
}

// classesUsing đọc lớp trỏ tới phiên bản khóa học; where nối sau FROM classes cl JOIN course_versions cv.
func classesUsing(ctx context.Context, ex db.Executor, where string, args ...any) ([]ClassUsingRow, error) {
	var recs []classUsingRecord
	err := sqlx.SelectContext(ctx, ex, &recs, `SELECT cv.course_id, cl.id AS class_id, cl.code, cl.name, cv.version_no
		FROM classes cl JOIN course_versions cv ON cv.id = cl.course_version_id `+where+` ORDER BY cl.code`, args...)
	if err != nil {
		return nil, fmt.Errorf("courses: lớp dùng khóa học: %w", err)
	}
	out := make([]ClassUsingRow, 0, len(recs))
	for _, r := range recs {
		out = append(out, ClassUsingRow{CourseID: r.CourseID, ClassID: r.ClassID, Code: r.Code, Name: r.Name, VersionNo: domain.VersionNo(r.VersionNo)})
	}
	return out, nil
}

func (PGCourseVersionRepo) ClassesUsing(ctx context.Context, ex db.Executor, courseID uuid.UUID) ([]ClassUsingRow, error) {
	return classesUsing(ctx, ex, `WHERE cv.course_id = $1`, courseID)
}

type usedByClassRecord struct {
	ClassID     uuid.UUID `db:"class_id"`
	Code        string    `db:"code"`
	Name        string    `db:"name"`
	Status      string    `db:"status"`
	MemberCount int       `db:"member_count"`
}

func (PGCourseVersionRepo) UsedByClasses(ctx context.Context, ex db.Executor, versionID uuid.UUID) ([]UsedByClassRow, error) {
	var recs []usedByClassRecord
	err := sqlx.SelectContext(ctx, ex, &recs, `SELECT cl.id AS class_id, cl.code, cl.name, cl.status,
		(SELECT count(*) FROM class_members m WHERE m.class_id = cl.id AND m.status <> 'dropped') AS member_count
		FROM classes cl WHERE cl.course_version_id = $1 ORDER BY cl.code`, versionID)
	if err != nil {
		return nil, fmt.Errorf("courses: lớp dùng phiên bản: %w", err)
	}
	out := make([]UsedByClassRow, 0, len(recs))
	for _, r := range recs {
		out = append(out, UsedByClassRow(r))
	}
	return out, nil
}

type stageRowRecord struct {
	Position          int       `db:"position"`
	StageID           uuid.UUID `db:"stage_id"`
	StageCode         string    `db:"stage_code"`
	StageName         string    `db:"stage_name"`
	StageVersionID    uuid.UUID `db:"stage_version_id"`
	StageVersionNo    int       `db:"stage_version_no"`
	LessonCount       int       `db:"lesson_count"`
	RequiredCount     int       `db:"required_count"`
	LatestPublishedNo *int      `db:"latest_published_no"`
}

func (PGCourseVersionRepo) StageRows(ctx context.Context, ex db.Executor, versionID uuid.UUID) ([]VersionStageRow, error) {
	var recs []stageRowRecord
	err := sqlx.SelectContext(ctx, ex, &recs, `SELECT x.position, x.stage_id, s.code AS stage_code, s.name AS stage_name,
		x.stage_version_id, sv.version_no AS stage_version_no,
		(SELECT count(*) FROM lessons l WHERE l.stage_version_id = sv.id) AS lesson_count,
		(SELECT count(*) FROM lessons l WHERE l.stage_version_id = sv.id AND l.required) AS required_count,
		`+latestPublishedOfStage+` AS latest_published_no
		FROM course_version_stages x JOIN stage_versions sv ON sv.id = x.stage_version_id JOIN stages s ON s.id = x.stage_id
		WHERE x.course_version_id = $1 ORDER BY x.position`, versionID)
	if err != nil {
		return nil, fmt.Errorf("courses: chặng của phiên bản: %w", err)
	}
	out := make([]VersionStageRow, 0, len(recs))
	for _, r := range recs {
		row := VersionStageRow{
			Position: r.Position, StageID: r.StageID, StageCode: r.StageCode, StageName: r.StageName,
			StageVersionID: r.StageVersionID, StageVersionNo: domain.VersionNo(r.StageVersionNo),
			LessonCount: r.LessonCount, RequiredCount: r.RequiredCount,
		}
		if r.LatestPublishedNo != nil {
			no := domain.VersionNo(*r.LatestPublishedNo)
			row.LatestPublishedNo = &no
		}
		out = append(out, row)
	}
	return out, nil
}
