package stages

import (
	"context"
	"database/sql"
	"encoding/json"
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

// PGStageRepo là StageRepo trên PostgreSQL.
type PGStageRepo struct{}

// PGStageVersionRepo là StageVersionRepo trên PostgreSQL. Mọi câu ghi lên lessons/lesson_media mang điều kiện header
// còn draft ngay trong SQL và chạy qua db.ExecAffectOne: DB không có trigger nên đây là lớp chặn cuối.
type PGStageVersionRepo struct{}

var (
	_ StageRepo        = PGStageRepo{}
	_ StageVersionRepo = PGStageVersionRepo{}
)

// constraintOf trả tên constraint của lỗi PostgreSQL (rỗng nếu không phải lỗi PostgreSQL).
func constraintOf(err error) (code, constraint string) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code, pg.ConstraintName
	}
	return "", ""
}

func (PGStageRepo) Create(ctx context.Context, ex db.Executor, s *Stage) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO stages (id, code, name, created_by, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5)`,
		s.id, s.code.String(), s.name, s.createdBy, s.createdAt)
	if err != nil {
		if code, c := constraintOf(err); code == pgerrcode.UniqueViolation && c == pgerr.UqStagesCode {
			return ErrCodeTaken
		}
		return fmt.Errorf("stages: tạo chặng: %w", pgerr.Map(err))
	}
	return nil
}

// stageColumns đọc mô tả từ phiên bản mới nhất vì bảng stages không có cột mô tả.
const stageColumns = `s.id, s.code, s.name, s.created_at,
	COALESCE((SELECT sv.description FROM stage_versions sv WHERE sv.stage_id = s.id ORDER BY sv.version_no DESC LIMIT 1), '') AS description`

type stageRecord struct {
	ID          uuid.UUID `db:"id"`
	Code        string    `db:"code"`
	Name        string    `db:"name"`
	CreatedAt   time.Time `db:"created_at"`
	Description string    `db:"description"`
}

func (r stageRecord) row() StageRow {
	return StageRow{ID: r.ID, Code: r.Code, Name: r.Name, Description: r.Description, CreatedAt: r.CreatedAt}
}

func (PGStageRepo) ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*StageRow, error) {
	var rec stageRecord
	err := sqlx.GetContext(ctx, ex, &rec, `SELECT `+stageColumns+` FROM stages s WHERE s.id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrStageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("stages: đọc chặng: %w", err)
	}
	row := rec.row()
	return &row, nil
}

func (PGStageRepo) Delete(ctx context.Context, ex db.Executor, id uuid.UUID) error {
	err := db.ExecAffectOne(ctx, ex,
		`DELETE FROM stages s WHERE s.id = $1 AND NOT EXISTS (SELECT 1 FROM stage_versions sv WHERE sv.stage_id = s.id)`, id)
	if err != nil {
		return fmt.Errorf("stages: xóa chặng còn phiên bản hoặc không tồn tại: %w", pgerr.Map(err))
	}
	return nil
}

func (PGStageRepo) LockForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) error {
	var got uuid.UUID
	err := sqlx.GetContext(ctx, ex, &got, `SELECT id FROM stages WHERE id = $1 FOR UPDATE`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrStageNotFound
	}
	if err != nil {
		return fmt.Errorf("stages: khóa chặng: %w", err)
	}
	return nil
}

func (PGStageRepo) List(ctx context.Context, ex db.Executor, q ListQuery) ([]StageListRow, error) {
	const filter = `($1 = '' OR s.code ILIKE '%' || $1 || '%' OR s.name ILIKE '%' || $1 || '%')`
	term := escapeLike(strings.TrimSpace(q.Q))

	var recs []struct {
		stageRecord
		UsedByCourseCount int `db:"used_by_course_count"`
	}
	err := sqlx.SelectContext(ctx, ex, &recs, `SELECT `+stageColumns+`,
		(SELECT count(DISTINCT cv.course_id) FROM course_version_stages cvs
		   JOIN course_versions cv ON cv.id = cvs.course_version_id WHERE cvs.stage_id = s.id) AS used_by_course_count
		FROM stages s WHERE `+filter+` ORDER BY s.code`, term)
	if err != nil {
		return nil, fmt.Errorf("stages: liệt kê chặng: %w", err)
	}
	versions, err := versionSummaries(ctx, ex, `JOIN stages s ON s.id = sv.stage_id WHERE `+filter, term)
	if err != nil {
		return nil, err
	}
	outdated, err := PGStageVersionRepo{}.AllOutdated(ctx, ex)
	if err != nil {
		return nil, err
	}
	byStage := make(map[uuid.UUID][]VersionSummary, len(recs))
	for _, v := range versions {
		byStage[v.StageID] = append(byStage[v.StageID], v)
	}
	outdatedCount := make(map[uuid.UUID]int)
	for _, o := range outdated {
		outdatedCount[o.StageID]++
	}
	out := make([]StageListRow, 0, len(recs))
	for _, r := range recs {
		out = append(out, StageListRow{
			StageRow:            r.row(),
			Versions:            byStage[r.ID],
			UsedByCourseCount:   r.UsedByCourseCount,
			OutdatedCourseCount: outdatedCount[r.ID],
		})
	}
	return out, nil
}

// escapeLike thoát ký tự đặc biệt của LIKE để từ khóa tìm kiếm được so khớp nguyên văn.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

type versionRecord struct {
	ID           uuid.UUID  `db:"id"`
	StageID      uuid.UUID  `db:"stage_id"`
	VersionNo    int        `db:"version_no"`
	Status       string     `db:"status"`
	Title        string     `db:"title"`
	Description  string     `db:"description"`
	ClonedFromID *uuid.UUID `db:"cloned_from_id"`
	PublishedAt  *time.Time `db:"published_at"`
	ArchivedAt   *time.Time `db:"archived_at"`
	CreatedBy    uuid.UUID  `db:"created_by"`
}

const versionColumns = `id, stage_id, version_no, status, title, description, cloned_from_id, published_at, archived_at, created_by`

type lessonRecord struct {
	ID              uuid.UUID  `db:"id"`
	LessonKey       string     `db:"lesson_key"`
	Position        int        `db:"position"`
	Title           string     `db:"title"`
	Type            string     `db:"type"`
	Required        bool       `db:"required"`
	MarkdownSource  *string    `db:"markdown_source"`
	MarkdownHTML    *string    `db:"markdown_html"`
	VideoMediaID    *uuid.UUID `db:"video_media_id"`
	DurationSeconds *int       `db:"duration_seconds"`
	VideoFileName   *string    `db:"video_file_name"`
}

// toLesson dựng LessonContent theo cột type: đây là điểm mở duy nhất khi thêm loại học liệu mới.
func (r lessonRecord) toLesson() (*Lesson, error) {
	l := &Lesson{id: r.ID, key: domain.LessonKey(r.LessonKey), title: r.Title, position: r.Position, required: r.Required}
	switch LessonType(r.Type) {
	case LessonVideo:
		if r.VideoMediaID == nil {
			return nil, fmt.Errorf("stages: học liệu video %s thiếu video_media_id", r.ID)
		}
		c := VideoContent{MediaID: *r.VideoMediaID, DurationSeconds: r.DurationSeconds}
		if r.VideoFileName != nil {
			c.FileName = *r.VideoFileName
		}
		l.content = c
	case LessonMarkdown:
		if r.MarkdownSource == nil {
			return nil, fmt.Errorf("stages: học liệu markdown %s thiếu markdown_source", r.ID)
		}
		l.content = MarkdownContent{Source: *r.MarkdownSource, HTML: r.MarkdownHTML}
	default:
		return nil, fmt.Errorf("stages: loại học liệu lạ %q", r.Type)
	}
	return l, nil
}

// lessonValues tách nội dung thành các cột của bảng lessons.
type lessonValues struct {
	typ             string
	markdownSource  *string
	markdownHTML    *string
	videoMediaID    *uuid.UUID
	durationSeconds *int
}

func valuesOf(l *Lesson) (lessonValues, error) {
	switch c := l.content.(type) {
	case VideoContent:
		id := c.MediaID
		return lessonValues{typ: string(LessonVideo), videoMediaID: &id, durationSeconds: c.DurationSeconds}, nil
	case MarkdownContent:
		src := c.Source
		return lessonValues{typ: string(LessonMarkdown), markdownSource: &src, markdownHTML: c.HTML}, nil
	default:
		return lessonValues{}, fmt.Errorf("stages: loại nội dung học liệu lạ %T", l.content)
	}
}

// mediaIDs là các media học liệu tham chiếu (không trùng): video và mọi ảnh nội bộ trong HTML đã render.
func (v lessonValues) mediaIDs() []uuid.UUID {
	if v.videoMediaID != nil {
		return []uuid.UUID{*v.videoMediaID}
	}
	if v.markdownHTML != nil {
		return mediaIDsInHTML(*v.markdownHTML)
	}
	return nil
}

func (v lessonValues) equal(o lessonValues) bool {
	return v.typ == o.typ && eqPtr(v.markdownSource, o.markdownSource) && eqPtr(v.markdownHTML, o.markdownHTML) &&
		eqPtr(v.videoMediaID, o.videoMediaID) && eqPtr(v.durationSeconds, o.durationSeconds)
}

func eqPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (r PGStageVersionRepo) ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*StageVersion, error) {
	return r.load(ctx, ex, id, false)
}

// ByIDForUpdate khóa header FOR UPDATE: mọi use case ghi mở bằng hàm này để tuần tự hóa với Publish/Archive.
func (r PGStageVersionRepo) ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*StageVersion, error) {
	return r.load(ctx, ex, id, true)
}

func (PGStageVersionRepo) load(ctx context.Context, ex db.Executor, id uuid.UUID, forUpdate bool) (*StageVersion, error) {
	q := `SELECT ` + versionColumns + ` FROM stage_versions WHERE id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	var rec versionRecord
	err := sqlx.GetContext(ctx, ex, &rec, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("stages: đọc phiên bản: %w", err)
	}
	status, err := domain.ParseVersionStatus(rec.Status)
	if err != nil {
		return nil, fmt.Errorf("stages: phiên bản %s: %w", rec.ID, err)
	}
	lessons, err := loadLessons(ctx, ex, id)
	if err != nil {
		return nil, err
	}
	return &StageVersion{
		id: rec.ID, stageID: rec.StageID, versionNo: domain.VersionNo(rec.VersionNo), status: status,
		title: rec.Title, description: rec.Description, clonedFromID: rec.ClonedFromID,
		lessons: lessons, publishedAt: rec.PublishedAt, archivedAt: rec.ArchivedAt, createdBy: rec.CreatedBy,
	}, nil
}

func loadLessons(ctx context.Context, ex db.Executor, versionID uuid.UUID) ([]*Lesson, error) {
	var recs []lessonRecord
	err := sqlx.SelectContext(ctx, ex, &recs, `SELECT l.id, l.lesson_key, l.position, l.title, l.type, l.required,
		l.markdown_source, l.markdown_html, l.video_media_id, l.duration_seconds, mf.original_name AS video_file_name
		FROM lessons l LEFT JOIN media_files mf ON mf.id = l.video_media_id
		WHERE l.stage_version_id = $1 ORDER BY l.position`, versionID)
	if err != nil {
		return nil, fmt.Errorf("stages: đọc học liệu: %w", err)
	}
	out := make([]*Lesson, 0, len(recs))
	for _, rec := range recs {
		l, err := rec.toLesson()
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// Create chèn header draft rồi học liệu. Bản nháp thứ hai của cùng chặng chạm partial unique index
// uq_stage_versions_one_draft; ON CONFLICT DO NOTHING giữ giao dịch dùng được để đọc bản nháp đang có.
func (r PGStageVersionRepo) Create(ctx context.Context, ex db.Executor, v *StageVersion) error {
	if v.status != domain.VersionDraft {
		return ErrVersionImmutable
	}
	res, err := ex.ExecContext(ctx, `INSERT INTO stage_versions
		(id, stage_id, version_no, status, title, description, cloned_from_id, created_by)
		VALUES ($1, $2, $3, 'draft', $4, $5, $6, $7)
		ON CONFLICT (stage_id) WHERE status = 'draft' DO NOTHING`,
		v.id, v.stageID, v.versionNo.Int(), v.title, v.description, v.clonedFromID, v.createdBy)
	if err != nil {
		return fmt.Errorf("stages: tạo phiên bản: %w", pgerr.Map(err))
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("stages: tạo phiên bản: %w", err)
	} else if n == 0 {
		d, err := r.DraftOf(ctx, ex, v.stageID)
		if err != nil {
			return err
		}
		if d == nil {
			return fmt.Errorf("stages: tạo phiên bản: xung đột bản nháp nhưng không đọc được bản nháp")
		}
		return &ErrDraftExists{DraftID: d.ID, No: d.VersionNo}
	}
	for _, l := range v.lessons {
		vals, err := valuesOf(l)
		if err != nil {
			return err
		}
		if err := insertLesson(ctx, ex, v.id, l, vals); err != nil {
			return err
		}
		if err := replaceLessonMedia(ctx, ex, l.id, vals.mediaIDs()); err != nil {
			return err
		}
	}
	return nil
}

// SaveDraft so học liệu trong DB với aggregate: xóa, sửa, thêm (kể cả markdown_html, duration_seconds), ghi lại
// lesson_media của từng học liệu thay đổi, rồi mới UPDATE header (không đổi status). Mọi câu đều có điều kiện draft.
func (r PGStageVersionRepo) SaveDraft(ctx context.Context, ex db.Executor, v *StageVersion) error {
	if _, err := ex.ExecContext(ctx, `SET CONSTRAINTS `+pgerr.UqLessonsVersionPosition+` DEFERRED`); err != nil {
		return fmt.Errorf("stages: hoãn kiểm tra vị trí: %w", err)
	}
	current, err := loadLessons(ctx, ex, v.id)
	if err != nil {
		return err
	}
	existing := make(map[uuid.UUID]*Lesson, len(current))
	for _, l := range current {
		existing[l.id] = l
	}
	wanted := make(map[uuid.UUID]bool, len(v.lessons))
	for _, l := range v.lessons {
		wanted[l.id] = true
	}
	for _, l := range current {
		if !wanted[l.id] {
			if err := deleteLesson(ctx, ex, v.id, l.id); err != nil {
				return err
			}
		}
	}
	for _, l := range v.lessons {
		vals, err := valuesOf(l)
		if err != nil {
			return err
		}
		old, ok := existing[l.id]
		switch {
		case !ok:
			if err := insertLesson(ctx, ex, v.id, l, vals); err != nil {
				return err
			}
		default:
			oldVals, err := valuesOf(old)
			if err != nil {
				return err
			}
			if old.title == l.title && old.position == l.position && old.required == l.required && oldVals.equal(vals) {
				continue
			}
			if err := updateLesson(ctx, ex, v.id, l, vals); err != nil {
				return err
			}
		}
		if err := replaceLessonMedia(ctx, ex, l.id, vals.mediaIDs()); err != nil {
			return err
		}
	}
	err = db.ExecAffectOne(ctx, ex,
		`UPDATE stage_versions SET title = $2, description = $3, updated_at = now() WHERE id = $1 AND status = 'draft'`,
		v.id, v.title, v.description)
	return immutableIfNoRows(err, "lưu bản nháp")
}

func immutableIfNoRows(err error, what string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, db.ErrNoRowsAffected):
		return ErrVersionImmutable
	default:
		return lessonWriteError(err, what)
	}
}

func lessonWriteError(err error, what string) error {
	if code, c := constraintOf(err); code == pgerrcode.UniqueViolation && c == pgerr.UqLessonsVersionKey {
		return ErrLessonKeyTaken
	}
	return fmt.Errorf("stages: %s: %w", what, pgerr.Map(err))
}

func insertLesson(ctx context.Context, ex db.Executor, versionID uuid.UUID, l *Lesson, vals lessonValues) error {
	err := db.ExecAffectOne(ctx, ex, `INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, required,
		markdown_source, markdown_html, video_media_id, duration_seconds)
		SELECT $1::uuid, sv.id, $3::text, $4::int, $5::text, $6::text, $7::boolean, $8::text, $9::text, $10::uuid, $11::int
		FROM stage_versions sv WHERE sv.id = $2 AND sv.status = 'draft'`,
		l.id, versionID, l.key.String(), l.position, l.title, vals.typ, l.required,
		vals.markdownSource, vals.markdownHTML, vals.videoMediaID, vals.durationSeconds)
	return immutableIfNoRows(err, "thêm học liệu")
}

func updateLesson(ctx context.Context, ex db.Executor, versionID uuid.UUID, l *Lesson, vals lessonValues) error {
	err := db.ExecAffectOne(ctx, ex, `UPDATE lessons l SET position = $3, title = $4, type = $5, required = $6,
		markdown_source = $7, markdown_html = $8, video_media_id = $9, duration_seconds = $10, updated_at = now()
		FROM stage_versions sv
		WHERE l.id = $1 AND l.stage_version_id = sv.id AND sv.id = $2 AND sv.status = 'draft'`,
		l.id, versionID, l.position, l.title, vals.typ, l.required,
		vals.markdownSource, vals.markdownHTML, vals.videoMediaID, vals.durationSeconds)
	return immutableIfNoRows(err, "sửa học liệu")
}

func deleteLesson(ctx context.Context, ex db.Executor, versionID, lessonID uuid.UUID) error {
	err := db.ExecAffectOne(ctx, ex, `DELETE FROM lessons l USING stage_versions sv
		WHERE l.id = $1 AND l.stage_version_id = sv.id AND sv.id = $2 AND sv.status = 'draft'`, lessonID, versionID)
	return immutableIfNoRows(err, "xóa học liệu")
}

// replaceLessonMedia ghi lại toàn bộ lesson_media của một học liệu. Header bị khóa FOR SHARE trước để không đổi
// trạng thái giữa chừng; từng câu INSERT vẫn mang điều kiện draft.
func replaceLessonMedia(ctx context.Context, ex db.Executor, lessonID uuid.UUID, mediaIDs []uuid.UUID) error {
	var status string
	err := sqlx.GetContext(ctx, ex, &status, `SELECT sv.status FROM lessons l
		JOIN stage_versions sv ON sv.id = l.stage_version_id WHERE l.id = $1 FOR SHARE OF sv`, lessonID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLessonNotFound
	}
	if err != nil {
		return fmt.Errorf("stages: khóa phiên bản của học liệu: %w", err)
	}
	if status != string(domain.VersionDraft) {
		return ErrVersionImmutable
	}
	if _, err := ex.ExecContext(ctx, `DELETE FROM lesson_media lm USING lessons l, stage_versions sv
		WHERE lm.lesson_id = $1 AND l.id = lm.lesson_id AND sv.id = l.stage_version_id AND sv.status = 'draft'`, lessonID); err != nil {
		return fmt.Errorf("stages: xóa lesson_media: %w", pgerr.Map(err))
	}
	for _, mediaID := range mediaIDs {
		err := db.ExecAffectOne(ctx, ex, `INSERT INTO lesson_media (lesson_id, media_id)
			SELECT l.id, $2::uuid FROM lessons l JOIN stage_versions sv ON sv.id = l.stage_version_id
			WHERE l.id = $1 AND sv.status = 'draft'`, lessonID, mediaID)
		if err := immutableIfNoRows(err, "ghi lesson_media"); err != nil {
			return err
		}
	}
	return nil
}

// TransitionStatus chỉ UPDATE header với WHERE status=from; phát hành còn đòi mọi học liệu markdown đã render.
func (PGStageVersionRepo) TransitionStatus(ctx context.Context, ex db.Executor, id uuid.UUID, from, to domain.VersionStatus, publishedAt *time.Time) error {
	if !from.CanTransitionTo(to) {
		return domain.ErrInvalidTransition
	}
	if to == domain.VersionPublished && publishedAt == nil {
		return errors.New("stages: phát hành cần thời điểm phát hành")
	}
	q := `UPDATE stage_versions SET status = $3::text, published_at = COALESCE($4::timestamptz, published_at),
		archived_at = CASE WHEN $3::text = 'archived' THEN now() ELSE archived_at END, updated_at = now()
		WHERE id = $1 AND status = $2::text`
	if to == domain.VersionPublished {
		q += ` AND NOT EXISTS (SELECT 1 FROM lessons WHERE stage_version_id = $1 AND type = 'markdown' AND markdown_html IS NULL)`
	}
	err := db.ExecAffectOne(ctx, ex, q, id, from.String(), to.String(), publishedAt)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, db.ErrNoRowsAffected):
		return fmt.Errorf("stages: chuyển trạng thái phiên bản: %w", pgerr.Map(err))
	case to == domain.VersionPublished:
		return ErrNotRendered
	case to == domain.VersionArchived:
		return ErrArchiveNotPublished
	default:
		return domain.ErrInvalidTransition
	}
}

// Delete chỉ xóa header draft (CASCADE lessons, lesson_media); bản không nháp → ErrVersionImmutable.
func (PGStageVersionRepo) Delete(ctx context.Context, ex db.Executor, id uuid.UUID) error {
	err := db.ExecAffectOne(ctx, ex, `DELETE FROM stage_versions WHERE id = $1 AND status = 'draft'`, id)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, db.ErrNoRowsAffected):
		return ErrVersionImmutable
	}
	if code, _ := constraintOf(err); code == pgerrcode.ForeignKeyViolation {
		return fmt.Errorf("stages: xóa phiên bản: %w", domain.ErrInUse)
	}
	return fmt.Errorf("stages: xóa phiên bản: %w", pgerr.Map(err))
}

type summaryRecord struct {
	ID                  uuid.UUID  `db:"id"`
	StageID             uuid.UUID  `db:"stage_id"`
	VersionNo           int        `db:"version_no"`
	Status              string     `db:"status"`
	PublishedAt         *time.Time `db:"published_at"`
	ClonedFromVersionNo *int       `db:"cloned_from_version_no"`
	LessonCount         int        `db:"lesson_count"`
}

func (r summaryRecord) summary() VersionSummary {
	s := VersionSummary{
		ID: r.ID, StageID: r.StageID, VersionNo: domain.VersionNo(r.VersionNo), Status: domain.VersionStatus(r.Status),
		PublishedAt: r.PublishedAt, LessonCount: r.LessonCount,
	}
	if r.ClonedFromVersionNo != nil {
		no := domain.VersionNo(*r.ClonedFromVersionNo)
		s.ClonedFromVersionNo = &no
	}
	return s
}

// versionSummaries đọc phiên bản (mới nhất trước) kèm số học liệu; where nối sau FROM stage_versions sv.
func versionSummaries(ctx context.Context, ex db.Executor, where string, args ...any) ([]VersionSummary, error) {
	var recs []summaryRecord
	err := sqlx.SelectContext(ctx, ex, &recs, `SELECT sv.id, sv.stage_id, sv.version_no, sv.status, sv.published_at,
		src.version_no AS cloned_from_version_no,
		(SELECT count(*) FROM lessons l WHERE l.stage_version_id = sv.id) AS lesson_count
		FROM stage_versions sv LEFT JOIN stage_versions src ON src.id = sv.cloned_from_id `+where+`
		ORDER BY sv.stage_id, sv.version_no DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("stages: liệt kê phiên bản: %w", err)
	}
	out := make([]VersionSummary, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.summary())
	}
	return out, nil
}

func (PGStageVersionRepo) ListByStage(ctx context.Context, ex db.Executor, stageID uuid.UUID) ([]VersionSummary, error) {
	return versionSummaries(ctx, ex, `WHERE sv.stage_id = $1`, stageID)
}

func (PGStageVersionRepo) CountByStage(ctx context.Context, ex db.Executor, stageID uuid.UUID) (int, error) {
	var n int
	if err := sqlx.GetContext(ctx, ex, &n, `SELECT count(*) FROM stage_versions WHERE stage_id = $1`, stageID); err != nil {
		return 0, fmt.Errorf("stages: đếm phiên bản: %w", err)
	}
	return n, nil
}

func (PGStageVersionRepo) NextVersionNo(ctx context.Context, ex db.Executor, stageID uuid.UUID) (domain.VersionNo, error) {
	if err := (PGStageRepo{}).LockForUpdate(ctx, ex, stageID); err != nil {
		return 0, err
	}
	var next int
	if err := sqlx.GetContext(ctx, ex, &next,
		`SELECT COALESCE(max(version_no), 0) + 1 FROM stage_versions WHERE stage_id = $1`, stageID); err != nil {
		return 0, fmt.Errorf("stages: số phiên bản kế tiếp: %w", err)
	}
	return domain.ParseVersionNo(next)
}

func (PGStageVersionRepo) DraftOf(ctx context.Context, ex db.Executor, stageID uuid.UUID) (*VersionSummary, error) {
	list, err := versionSummaries(ctx, ex, `WHERE sv.stage_id = $1 AND sv.status = 'draft'`, stageID)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

// jsonStrings đọc mảng JSON chuỗi (json_agg) thành []string.
type jsonStrings []string

func (s *jsonStrings) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*s = []string{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("stages: không đọc được mảng chuỗi từ %T", src)
	}
	out := []string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("stages: mảng chuỗi: %w", err)
	}
	*s = out
	return nil
}

type usedByRecord struct {
	CourseID        uuid.UUID   `db:"course_id"`
	CourseCode      string      `db:"course_code"`
	CourseName      string      `db:"course_name"`
	CourseVersionID uuid.UUID   `db:"course_version_id"`
	VersionNo       int         `db:"version_no"`
	Status          string      `db:"status"`
	ClassCodes      jsonStrings `db:"class_codes"`
	StageVersionNo  int         `db:"stage_version_no"`
	Outdated        bool        `db:"outdated"`
}

func (r usedByRecord) row() UsedByRow {
	return UsedByRow{
		CourseID: r.CourseID, CourseCode: r.CourseCode, CourseName: r.CourseName, CourseVersionID: r.CourseVersionID,
		VersionNo: domain.VersionNo(r.VersionNo), Status: domain.VersionStatus(r.Status), ClassCodes: []string(r.ClassCodes),
	}
}

const usedBySelect = `SELECT c.id AS course_id, c.code AS course_code, c.name AS course_name, cv.id AS course_version_id,
	cv.version_no, cv.status,
	COALESCE((SELECT json_agg(cl.code ORDER BY cl.code) FROM classes cl WHERE cl.course_version_id = cv.id), '[]') AS class_codes`

const usedByFrom = ` FROM course_version_stages cvs
	JOIN course_versions cv ON cv.id = cvs.course_version_id
	JOIN courses c ON c.id = cv.course_id
	JOIN stage_versions sv ON sv.id = cvs.stage_version_id`

func (PGStageVersionRepo) UsedBy(ctx context.Context, ex db.Executor, versionID uuid.UUID) ([]UsedByRow, error) {
	var recs []usedByRecord
	err := sqlx.SelectContext(ctx, ex, &recs, usedBySelect+`, sv.version_no AS stage_version_no, false AS outdated`+usedByFrom+`
		WHERE cvs.stage_version_id = $1 ORDER BY c.name, cv.version_no`, versionID)
	if err != nil {
		return nil, fmt.Errorf("stages: nơi dùng phiên bản: %w", err)
	}
	out := make([]UsedByRow, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.row())
	}
	return out, nil
}

func (PGStageVersionRepo) UsedByStage(ctx context.Context, ex db.Executor, stageID uuid.UUID) ([]StageUsedByRow, error) {
	var recs []usedByRecord
	err := sqlx.SelectContext(ctx, ex, &recs, usedBySelect+`, sv.version_no AS stage_version_no,
		sv.version_no < COALESCE((SELECT max(p.version_no) FROM stage_versions p
		                          WHERE p.stage_id = cvs.stage_id AND p.status = 'published'), 0) AS outdated`+usedByFrom+`
		WHERE cvs.stage_id = $1 ORDER BY c.name, cv.version_no`, stageID)
	if err != nil {
		return nil, fmt.Errorf("stages: nơi dùng chặng: %w", err)
	}
	out := make([]StageUsedByRow, 0, len(recs))
	for _, r := range recs {
		out = append(out, StageUsedByRow{UsedByRow: r.row(), StageVersionNo: domain.VersionNo(r.StageVersionNo), Outdated: r.Outdated})
	}
	return out, nil
}

// outdatedSQL là truy vấn FR-18 dùng chung cho một chặng ($1) và toàn hệ thống ($1 NULL): khóa học mà phiên bản
// phát hành mới nhất còn dùng phiên bản chặng cũ hơn bản phát hành mới nhất của chặng đó.
const outdatedSQL = `WITH latest AS (
	SELECT DISTINCT ON (stage_id) stage_id, id, version_no FROM stage_versions
	WHERE status = 'published' AND ($1::uuid IS NULL OR stage_id = $1)
	ORDER BY stage_id, version_no DESC
), lp AS (
	SELECT DISTINCT ON (cv.course_id) cv.id AS cv_id, cv.course_id, cv.version_no
	FROM course_versions cv WHERE cv.status = 'published' ORDER BY cv.course_id, cv.version_no DESC
)
SELECT c.id AS course_id, c.code AS course_code, c.name AS course_name, lp.cv_id AS course_version_id,
	lp.version_no AS course_version_no, s.id AS stage_id, s.code AS stage_code, s.name AS stage_name,
	sv.version_no AS using_version_no, latest.id AS latest_version_id, latest.version_no AS latest_version_no,
	d.id AS draft_version_id, d.version_no AS draft_version_no
FROM lp
JOIN course_version_stages cvs ON cvs.course_version_id = lp.cv_id
JOIN stage_versions sv ON sv.id = cvs.stage_version_id
JOIN latest ON latest.stage_id = sv.stage_id
JOIN stages s ON s.id = sv.stage_id
JOIN courses c ON c.id = lp.course_id
LEFT JOIN course_versions d ON d.course_id = c.id AND d.status = 'draft'
WHERE sv.version_no < latest.version_no
ORDER BY c.name, s.code`

type outdatedRecord struct {
	CourseID        uuid.UUID  `db:"course_id"`
	CourseCode      string     `db:"course_code"`
	CourseName      string     `db:"course_name"`
	CourseVersionID uuid.UUID  `db:"course_version_id"`
	CourseVersionNo int        `db:"course_version_no"`
	StageID         uuid.UUID  `db:"stage_id"`
	StageCode       string     `db:"stage_code"`
	StageName       string     `db:"stage_name"`
	UsingVersionNo  int        `db:"using_version_no"`
	LatestVersionID uuid.UUID  `db:"latest_version_id"`
	LatestVersionNo int        `db:"latest_version_no"`
	DraftVersionID  *uuid.UUID `db:"draft_version_id"`
	DraftVersionNo  *int       `db:"draft_version_no"`
}

func outdated(ctx context.Context, ex db.Executor, stageID *uuid.UUID) ([]OutdatedCourse, error) {
	var recs []outdatedRecord
	if err := sqlx.SelectContext(ctx, ex, &recs, outdatedSQL, stageID); err != nil {
		return nil, fmt.Errorf("stages: khóa học dùng phiên bản cũ: %w", err)
	}
	out := make([]OutdatedCourse, 0, len(recs))
	for _, r := range recs {
		o := OutdatedCourse{
			CourseID: r.CourseID, CourseCode: r.CourseCode, CourseName: r.CourseName, CourseVersionID: r.CourseVersionID,
			CourseVersionNo: domain.VersionNo(r.CourseVersionNo), StageID: r.StageID, StageCode: r.StageCode, StageName: r.StageName,
			UsingVersionNo: domain.VersionNo(r.UsingVersionNo), LatestVersionID: r.LatestVersionID,
			LatestVersionNo: domain.VersionNo(r.LatestVersionNo), DraftVersionID: r.DraftVersionID,
		}
		if r.DraftVersionNo != nil {
			no := domain.VersionNo(*r.DraftVersionNo)
			o.DraftVersionNo = &no
		}
		out = append(out, o)
	}
	return out, nil
}

func (PGStageVersionRepo) OutdatedCourses(ctx context.Context, ex db.Executor, stageID uuid.UUID) ([]OutdatedCourse, error) {
	return outdated(ctx, ex, &stageID)
}

func (PGStageVersionRepo) AllOutdated(ctx context.Context, ex db.Executor) ([]OutdatedCourse, error) {
	return outdated(ctx, ex, nil)
}

var _ VersionRefReader = PGStageVersionRepo{}

type refRecord struct {
	ID          uuid.UUID `db:"id"`
	StageID     uuid.UUID `db:"stage_id"`
	StageCode   string    `db:"stage_code"`
	StageName   string    `db:"stage_name"`
	VersionNo   int       `db:"version_no"`
	Status      string    `db:"status"`
	LessonCount int       `db:"lesson_count"`
}

func (r refRecord) ref() VersionRef {
	return VersionRef{
		ID: r.ID, StageID: r.StageID, StageCode: r.StageCode, StageName: r.StageName,
		VersionNo: domain.VersionNo(r.VersionNo), Status: domain.VersionStatus(r.Status), LessonCount: r.LessonCount,
	}
}

const refSelect = `SELECT sv.id, sv.stage_id, s.code AS stage_code, s.name AS stage_name, sv.version_no, sv.status,
	(SELECT count(*) FROM lessons l WHERE l.stage_version_id = sv.id) AS lesson_count
	FROM stage_versions sv JOIN stages s ON s.id = sv.stage_id `

func (PGStageVersionRepo) Refs(ctx context.Context, ex db.Executor, ids []uuid.UUID) (map[uuid.UUID]VersionRef, error) {
	out := make(map[uuid.UUID]VersionRef, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	var recs []refRecord
	if err := sqlx.SelectContext(ctx, ex, &recs, refSelect+`WHERE sv.id = ANY($1::uuid[])`, strs); err != nil {
		return nil, fmt.Errorf("stages: đọc phiên bản theo id: %w", err)
	}
	for _, r := range recs {
		out[r.ID] = r.ref()
	}
	return out, nil
}

func (PGStageVersionRepo) LatestPublishedOfStage(ctx context.Context, ex db.Executor, stageID uuid.UUID) (VersionRef, error) {
	var rec refRecord
	err := sqlx.GetContext(ctx, ex, &rec, refSelect+`WHERE sv.stage_id = $1 AND sv.status = 'published'
		ORDER BY sv.version_no DESC LIMIT 1`, stageID)
	if errors.Is(err, sql.ErrNoRows) {
		return VersionRef{}, ErrVersionNotFound
	}
	if err != nil {
		return VersionRef{}, fmt.Errorf("stages: bản phát hành mới nhất: %w", err)
	}
	return rec.ref(), nil
}
