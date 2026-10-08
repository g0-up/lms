package learning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
)

// PGProgressRepo là ProgressRepo trên PostgreSQL.
type PGProgressRepo struct{}

// PGCourseStructureRepo là CourseStructureRepo trên PostgreSQL.
type PGCourseStructureRepo struct{}

// PGMyClassesRepo là MyClassesRepo trên PostgreSQL.
type PGMyClassesRepo struct{}

// PGProgressReader là ProgressReader trên PostgreSQL.
type PGProgressReader struct{}

var (
	_ ProgressRepo        = PGProgressRepo{}
	_ CourseStructureRepo = PGCourseStructureRepo{}
	_ MyClassesRepo       = PGMyClassesRepo{}
	_ ProgressReader      = PGProgressReader{}
)

// percentOfRequired là phần trăm từ hai cột required_done/required_total của truy vấn bao quanh; round(numeric)
// làm tròn nửa ra xa 0 nên trùng domain.Percent.
const percentOfRequired = `(CASE WHEN required_total = 0 THEN 0 ELSE round(100.0 * required_done / required_total) END)::int`

// requiredOfMembers nối thành viên với học liệu bắt buộc của phiên bản khóa học lớp và tiến độ của họ;
// count(l.id) là tổng bắt buộc, count(lp.completed_at) là số đã hoàn thành.
const requiredOfMembers = `FROM class_members cm
	JOIN classes c ON c.id = cm.class_id
	LEFT JOIN course_version_stages cvs ON cvs.course_version_id = c.course_version_id
	LEFT JOIN lessons l ON l.stage_version_id = cvs.stage_version_id AND l.required
	LEFT JOIN lesson_progress lp ON lp.class_member_id = cm.id AND lp.lesson_id = l.id `

func (PGProgressRepo) Open(ctx context.Context, ex db.Executor, p *LessonProgress) (bool, error) {
	res, err := ex.ExecContext(ctx, `INSERT INTO lesson_progress (class_member_id, lesson_id, first_opened_at, updated_at)
		VALUES ($1, $2, $3, $3) ON CONFLICT (class_member_id, lesson_id) DO NOTHING`,
		p.memberID, p.lessonID, p.firstOpenedAt)
	if err != nil {
		return false, fmt.Errorf("learning: ghi lần mở học liệu: %w", pgerr.Map(err))
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("learning: ghi lần mở học liệu: %w", err)
	}
	return n == 1, nil
}

type progressRecord struct {
	LessonID      uuid.UUID  `db:"lesson_id"`
	FirstOpenedAt time.Time  `db:"first_opened_at"`
	CompletedAt   *time.Time `db:"completed_at"`
}

func getProgress(ctx context.Context, ex db.Executor, q string, memberID, lessonID uuid.UUID) (*LessonProgress, error) {
	var r progressRecord
	err := sqlx.GetContext(ctx, ex, &r, q, memberID, lessonID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("learning: đọc tiến độ: %w", err)
	}
	return RestoreProgress(memberID, r.LessonID, r.FirstOpenedAt, r.CompletedAt), nil
}

const progressSelect = `SELECT lesson_id, first_opened_at, completed_at FROM lesson_progress
	WHERE class_member_id = $1 AND lesson_id = $2`

func (PGProgressRepo) Get(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) (*LessonProgress, error) {
	return getProgress(ctx, ex, progressSelect, memberID, lessonID)
}

func (PGProgressRepo) GetForUpdate(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) (*LessonProgress, error) {
	return getProgress(ctx, ex, progressSelect+` FOR UPDATE`, memberID, lessonID)
}

// SetCompleted chỉ khớp khi dòng đang ở trạng thái ngược lại (chưa tích khi tích, đã tích khi bỏ tích); 0 dòng
// nghĩa là có thay đổi đồng thời ngoài khóa dòng → CONFLICT.
func (PGProgressRepo) SetCompleted(ctx context.Context, ex db.Executor, p *LessonProgress, now time.Time) error {
	err := db.ExecAffectOne(ctx, ex, `UPDATE lesson_progress SET completed_at = $3, updated_at = $4
		WHERE class_member_id = $1 AND lesson_id = $2 AND (completed_at IS NULL) = ($3::timestamptz IS NOT NULL)`,
		p.memberID, p.lessonID, p.completedAt, now)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, db.ErrNoRowsAffected):
		return domain.ErrConflict.WithMsg("Tiến độ vừa thay đổi, vui lòng thử lại.")
	default:
		return fmt.Errorf("learning: ghi hoàn thành: %w", pgerr.Map(err))
	}
}

func (PGProgressRepo) ByMember(ctx context.Context, ex db.Executor, memberID uuid.UUID) (map[uuid.UUID]*LessonProgress, error) {
	var recs []progressRecord
	if err := sqlx.SelectContext(ctx, ex, &recs, `SELECT lesson_id, first_opened_at, completed_at FROM lesson_progress
		WHERE class_member_id = $1`, memberID); err != nil {
		return nil, fmt.Errorf("learning: đọc tiến độ thành viên: %w", err)
	}
	out := make(map[uuid.UUID]*LessonProgress, len(recs))
	for _, r := range recs {
		out[r.LessonID] = RestoreProgress(memberID, r.LessonID, r.FirstOpenedAt, r.CompletedAt)
	}
	return out, nil
}

type structureRecord struct {
	StageID         uuid.UUID  `db:"stage_id"`
	StageVersionID  uuid.UUID  `db:"stage_version_id"`
	Code            string     `db:"code"`
	Name            string     `db:"name"`
	VersionNo       int        `db:"version_no"`
	StagePosition   int        `db:"stage_position"`
	LessonID        *uuid.UUID `db:"lesson_id"`
	LessonKey       *string    `db:"lesson_key"`
	Title           *string    `db:"title"`
	Type            *string    `db:"type"`
	Position        *int       `db:"position"`
	Required        *bool      `db:"required"`
	DurationSeconds *int       `db:"duration_seconds"`
}

// StagesOfCourseVersion đọc chặng và học liệu bằng một truy vấn; chặng chưa có học liệu vẫn có mặt.
func (PGCourseStructureRepo) StagesOfCourseVersion(ctx context.Context, ex db.Executor, courseVersionID uuid.UUID) ([]StageView, error) {
	var recs []structureRecord
	if err := sqlx.SelectContext(ctx, ex, &recs, `SELECT cvs.stage_id, cvs.stage_version_id, s.code, s.name, sv.version_no,
		cvs.position AS stage_position, l.id AS lesson_id, l.lesson_key, l.title, l.type, l.position, l.required, l.duration_seconds
		FROM course_version_stages cvs
		JOIN stages s ON s.id = cvs.stage_id
		JOIN stage_versions sv ON sv.id = cvs.stage_version_id
		LEFT JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		WHERE cvs.course_version_id = $1
		ORDER BY cvs.position, l.position`, courseVersionID); err != nil {
		return nil, fmt.Errorf("learning: đọc cấu trúc khóa học: %w", err)
	}
	out := []StageView{}
	for _, r := range recs {
		if len(out) == 0 || out[len(out)-1].StageVersionID != r.StageVersionID {
			out = append(out, StageView{
				StageID: r.StageID, StageVersionID: r.StageVersionID, Code: r.Code, Name: r.Name,
				VersionNo: domain.VersionNo(r.VersionNo), Position: r.StagePosition, Lessons: []LessonView{},
			})
		}
		if r.LessonID == nil || r.LessonKey == nil || r.Title == nil || r.Type == nil || r.Position == nil || r.Required == nil {
			continue
		}
		st := &out[len(out)-1]
		st.Lessons = append(st.Lessons, LessonView{
			ID: *r.LessonID, Key: domain.LessonKey(*r.LessonKey), Title: *r.Title, Type: stages.LessonType(*r.Type),
			Position: *r.Position, Required: *r.Required, State: StateNotOpened, DurationSeconds: r.DurationSeconds,
		})
	}
	return out, nil
}

func (PGCourseStructureRepo) LessonInCourseVersion(ctx context.Context, ex db.Executor, courseVersionID, lessonID uuid.UUID) (*LessonRow, error) {
	var r struct {
		ID              uuid.UUID  `db:"id"`
		LessonKey       string     `db:"lesson_key"`
		Title           string     `db:"title"`
		Type            string     `db:"type"`
		Position        int        `db:"position"`
		Required        bool       `db:"required"`
		DurationSeconds *int       `db:"duration_seconds"`
		MarkdownHTML    *string    `db:"markdown_html"`
		VideoMediaID    *uuid.UUID `db:"video_media_id"`
		StageID         uuid.UUID  `db:"stage_id"`
		StageName       string     `db:"stage_name"`
	}
	err := sqlx.GetContext(ctx, ex, &r, `SELECT l.id, l.lesson_key, l.title, l.type, l.position, l.required, l.duration_seconds,
		l.markdown_html, l.video_media_id, cvs.stage_id, s.name AS stage_name
		FROM course_version_stages cvs
		JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		JOIN stages s ON s.id = cvs.stage_id
		WHERE cvs.course_version_id = $1 AND l.id = $2`, courseVersionID, lessonID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("learning: đọc học liệu: %w", err)
	}
	row := &LessonRow{
		ID: r.ID, Key: domain.LessonKey(r.LessonKey), Title: r.Title, Position: r.Position, Required: r.Required,
		DurationSeconds: r.DurationSeconds, StageID: r.StageID, StageName: r.StageName,
	}
	switch stages.LessonType(r.Type) {
	case stages.LessonVideo:
		if r.VideoMediaID == nil {
			return nil, fmt.Errorf("learning: học liệu video %s thiếu media", r.ID)
		}
		row.Content = stages.VideoContent{MediaID: *r.VideoMediaID, DurationSeconds: r.DurationSeconds}
	case stages.LessonMarkdown:
		row.Content = stages.MarkdownContent{HTML: r.MarkdownHTML}
	default:
		return nil, fmt.Errorf("learning: loại học liệu %q không hỗ trợ", r.Type)
	}
	return row, nil
}

const classSummaryColumns = `c.id, c.code, c.name, c.status, c.start_date, c.end_date, t.full_name AS teacher_name,
	co.name AS course_name, cv.version_no AS course_version_no`

const classSummaryJoins = `JOIN users t ON t.id = c.teacher_id
	JOIN course_versions cv ON cv.id = c.course_version_id
	JOIN courses co ON co.id = cv.course_id `

type classSummaryRecord struct {
	ID              uuid.UUID `db:"id"`
	Code            string    `db:"code"`
	Name            string    `db:"name"`
	Status          string    `db:"status"`
	StartDate       time.Time `db:"start_date"`
	EndDate         time.Time `db:"end_date"`
	TeacherName     string    `db:"teacher_name"`
	CourseName      string    `db:"course_name"`
	CourseVersionNo int       `db:"course_version_no"`
}

func (r classSummaryRecord) summary() (ClassSummary, error) {
	st, err := domain.ParseClassStatus(r.Status)
	if err != nil {
		return ClassSummary{}, fmt.Errorf("learning: trạng thái lớp %q: %w", r.Status, err)
	}
	return ClassSummary{
		ID: r.ID, Code: r.Code, Name: r.Name, Status: st, StartDate: r.StartDate, EndDate: r.EndDate,
		TeacherName: r.TeacherName, CourseName: r.CourseName, CourseVersionNo: r.CourseVersionNo,
	}, nil
}

// ListForStudent tính tiến độ toàn khóa và học liệu kế tiếp (LATERAL … LIMIT 1 theo thứ tự chặng → position)
// trong một truy vấn.
func (PGMyClassesRepo) ListForStudent(ctx context.Context, ex db.Executor, userID uuid.UUID) ([]MyClassRow, error) {
	var recs []struct {
		classSummaryRecord
		MemberID      uuid.UUID  `db:"member_id"`
		RequiredTotal int        `db:"required_total"`
		RequiredDone  int        `db:"required_done"`
		Percent       int        `db:"percent"`
		NextLessonID  *uuid.UUID `db:"next_lesson_id"`
		NextTitle     *string    `db:"next_title"`
		NextStageName *string    `db:"next_stage_name"`
	}
	err := sqlx.SelectContext(ctx, ex, &recs, `WITH mt AS (
		  SELECT cm.id AS member_id, count(l.id) AS required_total, count(lp.completed_at) AS required_done
		  `+requiredOfMembers+`
		  WHERE cm.user_id = $1 AND cm.status = 'active' AND c.status IN ('draft', 'active', 'ended')
		  GROUP BY cm.id
		)
		SELECT `+classSummaryColumns+`, mt.member_id, mt.required_total, mt.required_done, `+percentOfRequired+` AS percent,
		  nl.lesson_id AS next_lesson_id, nl.title AS next_title, nl.stage_name AS next_stage_name
		FROM mt
		JOIN class_members cm ON cm.id = mt.member_id
		JOIN classes c ON c.id = cm.class_id
		`+classSummaryJoins+`
		LEFT JOIN LATERAL (
		  SELECT l.id AS lesson_id, l.title, s.name AS stage_name
		  FROM course_version_stages cvs
		  JOIN stages s ON s.id = cvs.stage_id
		  JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		  LEFT JOIN lesson_progress lp ON lp.class_member_id = cm.id AND lp.lesson_id = l.id
		  WHERE cvs.course_version_id = c.course_version_id AND lp.completed_at IS NULL AND c.status <> 'draft'
		  ORDER BY cvs.position, l.position
		  LIMIT 1
		) nl ON true
		ORDER BY cm.joined_at, c.code`, userID)
	if err != nil {
		return nil, fmt.Errorf("learning: liệt kê lớp của học viên: %w", err)
	}
	out := make([]MyClassRow, 0, len(recs))
	for _, r := range recs {
		cls, err := r.summary()
		if err != nil {
			return nil, err
		}
		row := MyClassRow{
			Class: cls, MemberID: r.MemberID, RequiredDone: r.RequiredDone, RequiredTotal: r.RequiredTotal, Percent: r.Percent,
		}
		if r.NextLessonID != nil && r.NextTitle != nil && r.NextStageName != nil {
			row.NextLesson = &LessonRef{LessonID: *r.NextLessonID, Title: *r.NextTitle, StageName: *r.NextStageName}
		}
		out = append(out, row)
	}
	return out, nil
}

func (PGMyClassesRepo) ClassSummary(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassSummary, error) {
	var r classSummaryRecord
	err := sqlx.GetContext(ctx, ex, &r, `SELECT `+classSummaryColumns+` FROM classes c `+classSummaryJoins+`WHERE c.id = $1`, classID)
	if errors.Is(err, sql.ErrNoRows) {
		return ClassSummary{}, ErrNotFound
	}
	if err != nil {
		return ClassSummary{}, fmt.Errorf("learning: đọc lớp: %w", err)
	}
	return r.summary()
}

// ClassProgress tính một dòng mỗi (thành viên, chặng) rồi cộng toàn khóa bằng window theo thành viên; lớp chưa có
// chặng cho mỗi thành viên một dòng với stage_id NULL.
func (PGProgressReader) ClassProgress(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]MemberProgress, error) {
	var recs []struct {
		MemberID       uuid.UUID  `db:"member_id"`
		UserID         uuid.UUID  `db:"user_id"`
		MemberStatus   string     `db:"member_status"`
		StageID        *uuid.UUID `db:"stage_id"`
		RequiredTotal  int        `db:"required_total"`
		RequiredDone   int        `db:"required_done"`
		Percent        int        `db:"percent"`
		MemberTotal    int        `db:"member_required_total"`
		MemberDone     int        `db:"member_required_done"`
		MemberPercent  int        `db:"member_percent"`
		LastActivityAt *time.Time `db:"last_activity_at"`
	}
	err := sqlx.SelectContext(ctx, ex, &recs, `WITH per_stage AS (
		  SELECT cm.id AS member_id, cm.user_id, cm.status AS member_status, cm.joined_at, cvs.stage_id, cvs.position,
		    count(l.id) FILTER (WHERE l.required) AS required_total,
		    count(lp.completed_at) FILTER (WHERE l.required) AS required_done,
		    max(greatest(lp.first_opened_at, lp.completed_at)) AS last_activity_at
		  FROM class_members cm
		  JOIN classes c ON c.id = cm.class_id
		  LEFT JOIN course_version_stages cvs ON cvs.course_version_id = c.course_version_id
		  LEFT JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		  LEFT JOIN lesson_progress lp ON lp.class_member_id = cm.id AND lp.lesson_id = l.id
		  WHERE cm.class_id = $1
		  GROUP BY cm.id, cvs.stage_id, cvs.position
		), per_member AS (
		  SELECT *, sum(required_total) OVER m AS member_required_total, sum(required_done) OVER m AS member_required_done,
		    max(last_activity_at) OVER m AS member_last_activity_at
		  FROM per_stage WINDOW m AS (PARTITION BY member_id)
		)
		SELECT member_id, user_id, member_status, stage_id, required_total, required_done, `+percentOfRequired+` AS percent,
		  member_required_total, member_required_done,
		  (CASE WHEN member_required_total = 0 THEN 0
		    ELSE round(100.0 * member_required_done / member_required_total) END)::int AS member_percent,
		  member_last_activity_at AS last_activity_at
		FROM per_member
		ORDER BY joined_at, member_id, position`, classID)
	if err != nil {
		return nil, fmt.Errorf("learning: đọc tiến độ lớp: %w", err)
	}
	out := []MemberProgress{}
	for _, r := range recs {
		if len(out) == 0 || out[len(out)-1].MemberID != r.MemberID {
			ms, err := domain.ParseMemberStatus(r.MemberStatus)
			if err != nil {
				return nil, fmt.Errorf("learning: trạng thái thành viên %q: %w", r.MemberStatus, err)
			}
			out = append(out, MemberProgress{
				MemberID: r.MemberID, UserID: r.UserID, MemberStatus: ms, Percent: r.MemberPercent,
				RequiredDone: r.MemberDone, RequiredTotal: r.MemberTotal, Stages: []StageProgress{}, LastActivityAt: r.LastActivityAt,
			})
		}
		if r.StageID == nil {
			continue
		}
		m := &out[len(out)-1]
		m.Stages = append(m.Stages, StageProgress{
			StageID: *r.StageID, Percent: r.Percent, RequiredDone: r.RequiredDone, RequiredTotal: r.RequiredTotal,
		})
	}
	return out, nil
}

// MemberLessonProgress dựng lộ trình của thành viên bằng cùng BuildRoadmap của trang học viên.
func (PGProgressReader) MemberLessonProgress(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (Roadmap, error) {
	var cv uuid.UUID
	err := sqlx.GetContext(ctx, ex, &cv, `SELECT c.course_version_id FROM class_members cm JOIN classes c ON c.id = cm.class_id
		WHERE cm.id = $2 AND cm.class_id = $1`, classID, memberID)
	if errors.Is(err, sql.ErrNoRows) {
		return Roadmap{}, ErrMemberNotFound
	}
	if err != nil {
		return Roadmap{}, fmt.Errorf("learning: đọc thành viên: %w", err)
	}
	cls, err := PGMyClassesRepo{}.ClassSummary(ctx, ex, classID)
	if err != nil {
		return Roadmap{}, err
	}
	structure, err := PGCourseStructureRepo{}.StagesOfCourseVersion(ctx, ex, cv)
	if err != nil {
		return Roadmap{}, err
	}
	progress, err := PGProgressRepo{}.ByMember(ctx, ex, memberID)
	if err != nil {
		return Roadmap{}, err
	}
	return BuildRoadmap(cls, structure, progress), nil
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func (PGProgressReader) ClassAveragePercent(ctx context.Context, ex db.Executor, classIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	out := make(map[uuid.UUID]int, len(classIDs))
	if len(classIDs) == 0 {
		return out, nil
	}
	var recs []struct {
		ClassID    uuid.UUID `db:"class_id"`
		AvgPercent int       `db:"avg_percent"`
	}
	if err := sqlx.SelectContext(ctx, ex, &recs, `WITH mt AS (
		  SELECT cm.class_id, count(l.id) AS required_total, count(lp.completed_at) AS required_done
		  `+requiredOfMembers+`
		  WHERE cm.class_id = ANY($1::uuid[]) AND cm.status = 'active'
		  GROUP BY cm.id, cm.class_id
		)
		SELECT class_id, round(avg(`+percentOfRequired+`))::int AS avg_percent FROM mt GROUP BY class_id`,
		uuidStrings(classIDs)); err != nil {
		return nil, fmt.Errorf("learning: tính tiến độ trung bình: %w", err)
	}
	for _, r := range recs {
		out[r.ClassID] = r.AvgPercent
	}
	return out, nil
}

func (PGProgressReader) ClassActivityCounts(ctx context.Context, ex db.Executor, classIDs []uuid.UUID, staleCutoff time.Time) (map[uuid.UUID]ActivityCounts, error) {
	out := make(map[uuid.UUID]ActivityCounts, len(classIDs))
	if len(classIDs) == 0 {
		return out, nil
	}
	var recs []struct {
		ClassID     uuid.UUID `db:"class_id"`
		NotLoggedIn int       `db:"not_logged_in"`
		Inactive    int       `db:"inactive"`
	}
	if err := sqlx.SelectContext(ctx, ex, &recs, `SELECT c.id AS class_id,
		  count(cm.id) FILTER (WHERE cm.status = 'active' AND u.status = 'invited') AS not_logged_in,
		  count(cm.id) FILTER (WHERE c.status = 'active' AND cm.status = 'active'
		    AND (u.last_active_at IS NULL OR u.last_active_at < $2)) AS inactive
		FROM classes c
		LEFT JOIN class_members cm ON cm.class_id = c.id
		LEFT JOIN users u ON u.id = cm.user_id
		WHERE c.id = ANY($1::uuid[])
		GROUP BY c.id`, uuidStrings(classIDs), staleCutoff); err != nil {
		return nil, fmt.Errorf("learning: đếm hoạt động lớp: %w", err)
	}
	for _, r := range recs {
		out[r.ClassID] = ActivityCounts{NotLoggedIn: r.NotLoggedIn, Inactive: r.Inactive}
	}
	return out, nil
}
