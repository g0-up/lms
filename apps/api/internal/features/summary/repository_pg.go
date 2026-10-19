package summary

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
)

// PGRepo là Repo trên Postgres.
type PGRepo struct{}

var _ Repo = PGRepo{}

// memberScope lọc dòng theo thành viên của lớp $1, $2 là một thành viên (NULL = mọi thành viên); bảng được lọc
// nối class_members cm.
const memberScope = `cm.class_id = $1 AND ($2::uuid IS NULL OR cm.id = $2::uuid)`

func (PGRepo) Class(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassInfo, error) {
	var r struct {
		ID              uuid.UUID `db:"id"`
		Code            string    `db:"code"`
		Name            string    `db:"name"`
		Status          string    `db:"status"`
		TeacherID       uuid.UUID `db:"teacher_id"`
		CourseVersionID uuid.UUID `db:"course_version_id"`
	}
	err := sqlx.GetContext(ctx, ex, &r, `SELECT c.id, c.code, c.name, c.status, c.teacher_id, c.course_version_id
		FROM classes c WHERE c.id = $1`, classID)
	if errors.Is(err, sql.ErrNoRows) {
		return ClassInfo{}, classes.ErrClassNotFound
	}
	if err != nil {
		return ClassInfo{}, fmt.Errorf("summary: đọc lớp: %w", err)
	}
	st, err := domain.ParseClassStatus(r.Status)
	if err != nil {
		return ClassInfo{}, fmt.Errorf("summary: trạng thái lớp %q: %w", r.Status, err)
	}
	return ClassInfo{ID: r.ID, Code: r.Code, Name: r.Name, Status: st, TeacherID: r.TeacherID, CourseVersionID: r.CourseVersionID}, nil
}

func (PGRepo) Stages(ctx context.Context, ex db.Executor, courseVersionID uuid.UUID) ([]StageRef, error) {
	var rows []struct {
		StageVersionID uuid.UUID `db:"stage_version_id"`
		StageID        uuid.UUID `db:"stage_id"`
		Code           string    `db:"code"`
		Name           string    `db:"name"`
		VersionNo      int       `db:"version_no"`
		Position       int       `db:"position"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT cvs.stage_version_id, cvs.stage_id, s.code, s.name, sv.version_no, cvs.position
		FROM course_version_stages cvs
		JOIN stages s ON s.id = cvs.stage_id
		JOIN stage_versions sv ON sv.id = cvs.stage_version_id
		WHERE cvs.course_version_id = $1
		ORDER BY cvs.position`, courseVersionID); err != nil {
		return nil, fmt.Errorf("summary: đọc chặng của khóa học: %w", err)
	}
	out := make([]StageRef, len(rows))
	for i, r := range rows {
		out[i] = StageRef{
			StageVersionID: r.StageVersionID, StageID: r.StageID, Code: r.Code, Name: r.Name,
			VersionNo: domain.VersionNo(r.VersionNo), Position: r.Position,
		}
	}
	return out, nil
}

func (PGRepo) Lessons(ctx context.Context, ex db.Executor, courseVersionID uuid.UUID) ([]Lesson, error) {
	var rows []struct {
		ID             uuid.UUID `db:"id"`
		StageVersionID uuid.UUID `db:"stage_version_id"`
		UnitID         uuid.UUID `db:"unit_id"`
		UnitTitle      string    `db:"unit_title"`
		UnitPosition   int       `db:"unit_position"`
		Title          string    `db:"title"`
		Type           string    `db:"type"`
		Position       int       `db:"position"`
		Required       bool      `db:"required"`
		QuizMode       *string   `db:"quiz_mode"`
		MaxAttempts    *int      `db:"max_attempts"`
		PassPercent    *int      `db:"pass_percent"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT l.id, l.stage_version_id, u.id AS unit_id, u.title AS unit_title,
		u.position AS unit_position, l.title, l.type, l.position, l.required,
		qs.mode AS quiz_mode, qs.max_attempts, qs.pass_percent
		FROM course_version_stages cvs
		JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		JOIN stage_units u ON u.id = l.unit_id
		LEFT JOIN quiz_settings qs ON qs.lesson_id = l.id
		WHERE cvs.course_version_id = $1
		ORDER BY cvs.position, u.position, l.position`, courseVersionID); err != nil {
		return nil, fmt.Errorf("summary: đọc học liệu của khóa học: %w", err)
	}
	out := make([]Lesson, len(rows))
	for i, r := range rows {
		l := Lesson{
			ID: r.ID, StageVersionID: r.StageVersionID, UnitID: r.UnitID, UnitTitle: r.UnitTitle, UnitPosition: r.UnitPosition,
			Title: r.Title, Type: stages.LessonType(r.Type), Position: r.Position, Required: r.Required,
			MaxAttempts: r.MaxAttempts, PassPercent: r.PassPercent,
		}
		if r.QuizMode != nil {
			l.QuizMode = stages.QuizMode(*r.QuizMode)
		}
		out[i] = l
	}
	return out, nil
}

type memberRecord struct {
	ID     uuid.UUID `db:"id"`
	UserID uuid.UUID `db:"user_id"`
	Name   string    `db:"name"`
	Email  string    `db:"email"`
	Status string    `db:"status"`
}

func (r memberRecord) member() (Member, error) {
	st, err := domain.ParseMemberStatus(r.Status)
	if err != nil {
		return Member{}, fmt.Errorf("summary: trạng thái thành viên %q: %w", r.Status, err)
	}
	return Member{ID: r.ID, UserID: r.UserID, Name: r.Name, Email: r.Email, Status: st}, nil
}

const memberColumns = `cm.id, cm.user_id, u.full_name AS name, u.email, cm.status
	FROM class_members cm JOIN users u ON u.id = cm.user_id`

// ActiveMembers sắp theo u.full_name rồi cm.id, đúng biểu thức "m.name, m.member_id" của danh sách học viên ở
// reports (không collation riêng), để hai màn hình cùng thứ tự trên mọi Postgres.
func (PGRepo) ActiveMembers(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]Member, error) {
	var rows []memberRecord
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT `+memberColumns+`
		WHERE cm.class_id = $1 AND cm.status = 'active'
		ORDER BY u.full_name, cm.id`, classID); err != nil {
		return nil, fmt.Errorf("summary: đọc thành viên của lớp: %w", err)
	}
	out := make([]Member, len(rows))
	for i, r := range rows {
		m, err := r.member()
		if err != nil {
			return nil, err
		}
		out[i] = m
	}
	return out, nil
}

func (PGRepo) Member(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (Member, error) {
	return getMember(ctx, ex, `SELECT `+memberColumns+` WHERE cm.class_id = $1 AND cm.id = $2`, classID, memberID)
}

func (PGRepo) LockMember(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (Member, error) {
	return getMember(ctx, ex, `SELECT `+memberColumns+` WHERE cm.class_id = $1 AND cm.id = $2 FOR UPDATE OF cm`, classID, memberID)
}

func getMember(ctx context.Context, ex db.Executor, q string, classID, memberID uuid.UUID) (Member, error) {
	var r memberRecord
	err := sqlx.GetContext(ctx, ex, &r, q, classID, memberID)
	if errors.Is(err, sql.ErrNoRows) {
		return Member{}, classes.ErrMemberNotFound
	}
	if err != nil {
		return Member{}, fmt.Errorf("summary: đọc thành viên: %w", err)
	}
	return r.member()
}

func (PGRepo) Activity(ctx context.Context, ex db.Executor, classID uuid.UUID, memberID *uuid.UUID) (Activity, error) {
	var a Activity
	var err error
	if a.Progress, err = progressRows(ctx, ex, classID, memberID); err != nil {
		return Activity{}, err
	}
	if a.Submissions, err = submissionRows(ctx, ex, classID, memberID); err != nil {
		return Activity{}, err
	}
	if a.Deadlines, err = deadlines(ctx, ex, classID); err != nil {
		return Activity{}, err
	}
	if a.Attempts, err = attemptRows(ctx, ex, classID, memberID); err != nil {
		return Activity{}, err
	}
	if a.Grants, err = grantRows(ctx, ex, classID, memberID); err != nil {
		return Activity{}, err
	}
	return a, nil
}

func progressRows(ctx context.Context, ex db.Executor, classID uuid.UUID, memberID *uuid.UUID) ([]ProgressRow, error) {
	var rows []struct {
		MemberID      uuid.UUID  `db:"member_id"`
		LessonID      uuid.UUID  `db:"lesson_id"`
		FirstOpenedAt *time.Time `db:"first_opened_at"`
		CompletedAt   *time.Time `db:"completed_at"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT lp.class_member_id AS member_id, lp.lesson_id, lp.first_opened_at, lp.completed_at
		FROM lesson_progress lp JOIN class_members cm ON cm.id = lp.class_member_id
		WHERE `+memberScope, classID, memberID); err != nil {
		return nil, fmt.Errorf("summary: đọc tiến độ: %w", err)
	}
	out := make([]ProgressRow, len(rows))
	for i, r := range rows {
		out[i] = ProgressRow{MemberID: r.MemberID, LessonID: r.LessonID, FirstOpenedAt: r.FirstOpenedAt, CompletedAt: r.CompletedAt}
	}
	return out, nil
}

// submissionRows: bản nộp mới nhất (attempt_no lớn nhất) của mỗi (thành viên, học liệu) cùng số lần nộp và lần chấm
// hiện hành (graded_at mới nhất).
func submissionRows(ctx context.Context, ex db.Executor, classID uuid.UUID, memberID *uuid.UUID) ([]SubmissionRow, error) {
	var rows []struct {
		MemberID    uuid.UUID `db:"member_id"`
		LessonID    uuid.UUID `db:"lesson_id"`
		Count       int       `db:"submission_count"`
		AttemptNo   int       `db:"attempt_no"`
		Status      string    `db:"status"`
		IsLate      bool      `db:"is_late"`
		SubmittedAt time.Time `db:"submitted_at"`
		TotalPoints *int      `db:"total_points"`
		MaxPoints   *int      `db:"max_points"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT s.class_member_id AS member_id, s.lesson_id, s.submission_count,
		s.attempt_no, s.status, s.is_late, s.submitted_at, g.total_points, g.max_points
		FROM (
		  SELECT DISTINCT ON (s.class_member_id, s.lesson_id) s.id, s.class_member_id, s.lesson_id, s.attempt_no, s.status,
		    s.is_late, s.submitted_at, count(*) OVER (PARTITION BY s.class_member_id, s.lesson_id) AS submission_count
		  FROM submissions s JOIN class_members cm ON cm.id = s.class_member_id
		  WHERE `+memberScope+`
		  ORDER BY s.class_member_id, s.lesson_id, s.attempt_no DESC
		) s
		LEFT JOIN LATERAL (
		  SELECT total_points, max_points FROM submission_grades
		  WHERE submission_id = s.id ORDER BY graded_at DESC, id DESC LIMIT 1
		) g ON true`, classID, memberID); err != nil {
		return nil, fmt.Errorf("summary: đọc bài nộp: %w", err)
	}
	out := make([]SubmissionRow, len(rows))
	for i, r := range rows {
		out[i] = SubmissionRow{MemberID: r.MemberID, LessonID: r.LessonID, Count: r.Count, Latest: Submission{
			AttemptNo: r.AttemptNo, Status: SubmissionStatus(r.Status), IsLate: r.IsLate, SubmittedAt: r.SubmittedAt,
			Total: r.TotalPoints, Max: r.MaxPoints,
		}}
	}
	return out, nil
}

func deadlines(ctx context.Context, ex db.Executor, classID uuid.UUID) (map[uuid.UUID]time.Time, error) {
	var rows []struct {
		LessonID uuid.UUID `db:"lesson_id"`
		DueAt    time.Time `db:"due_at"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT lesson_id, due_at FROM class_lesson_deadlines WHERE class_id = $1`, classID); err != nil {
		return nil, fmt.Errorf("summary: đọc hạn nộp: %w", err)
	}
	out := make(map[uuid.UUID]time.Time, len(rows))
	for _, r := range rows {
		out[r.LessonID] = r.DueAt
	}
	return out, nil
}

func attemptRows(ctx context.Context, ex db.Executor, classID uuid.UUID, memberID *uuid.UUID) ([]AttemptRow, error) {
	var rows []struct {
		MemberID     uuid.UUID `db:"member_id"`
		LessonID     uuid.UUID `db:"lesson_id"`
		AttemptNo    int       `db:"attempt_no"`
		Score        *int      `db:"score"`
		MaxScore     int       `db:"max_score"`
		ReviewStatus *string   `db:"review_status"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT a.class_member_id AS member_id, a.lesson_id, a.attempt_no, a.score,
		a.max_score, a.review_status
		FROM quiz_attempts a JOIN class_members cm ON cm.id = a.class_member_id
		WHERE `+memberScope+`
		ORDER BY a.class_member_id, a.lesson_id, a.attempt_no`, classID, memberID); err != nil {
		return nil, fmt.Errorf("summary: đọc lượt quiz: %w", err)
	}
	out := make([]AttemptRow, len(rows))
	for i, r := range rows {
		out[i] = AttemptRow{MemberID: r.MemberID, LessonID: r.LessonID, AttemptNo: r.AttemptNo, Score: r.Score, Max: r.MaxScore}
		if r.ReviewStatus != nil {
			rs := ReviewStatus(*r.ReviewStatus)
			out[i].ReviewStatus = &rs
		}
	}
	return out, nil
}

func grantRows(ctx context.Context, ex db.Executor, classID uuid.UUID, memberID *uuid.UUID) ([]GrantRow, error) {
	var rows []struct {
		MemberID uuid.UUID `db:"member_id"`
		LessonID uuid.UUID `db:"lesson_id"`
		Extra    int       `db:"extra"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT g.class_member_id AS member_id, g.lesson_id, sum(g.extra_attempts)::int AS extra
		FROM quiz_attempt_grants g JOIN class_members cm ON cm.id = g.class_member_id
		WHERE `+memberScope+`
		GROUP BY g.class_member_id, g.lesson_id`, classID, memberID); err != nil {
		return nil, fmt.Errorf("summary: đọc lượt cấp thêm: %w", err)
	}
	out := make([]GrantRow, len(rows))
	for i, r := range rows {
		out[i] = GrantRow{MemberID: r.MemberID, LessonID: r.LessonID, Extra: r.Extra}
	}
	return out, nil
}

type reviewRecord struct {
	ID             uuid.UUID `db:"id"`
	MemberID       uuid.UUID `db:"class_member_id"`
	StageVersionID uuid.UUID `db:"stage_version_id"`
	Body           string    `db:"body"`
	AuthorID       uuid.UUID `db:"author_id"`
	AuthorName     string    `db:"author_name"`
	Revision       int       `db:"revision"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

func (r reviewRecord) review() Review {
	return Review(r)
}

const reviewColumns = `r.id, r.class_member_id, r.stage_version_id, r.body, r.author_id, u.full_name AS author_name,
	r.revision, r.created_at, r.updated_at
	FROM stage_reviews r JOIN users u ON u.id = r.author_id`

func (PGRepo) Reviews(ctx context.Context, ex db.Executor, classID uuid.UUID, memberID *uuid.UUID) ([]Review, error) {
	var rows []reviewRecord
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT `+reviewColumns+`
		JOIN class_members cm ON cm.id = r.class_member_id
		WHERE `+memberScope, classID, memberID); err != nil {
		return nil, fmt.Errorf("summary: đọc nhận xét chặng: %w", err)
	}
	out := make([]Review, len(rows))
	for i, r := range rows {
		out[i] = r.review()
	}
	return out, nil
}

func (PGRepo) Review(ctx context.Context, ex db.Executor, memberID, stageVersionID uuid.UUID) (*Review, error) {
	var r reviewRecord
	err := sqlx.GetContext(ctx, ex, &r, `SELECT `+reviewColumns+`
		WHERE r.class_member_id = $1 AND r.stage_version_id = $2`, memberID, stageVersionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("summary: đọc nhận xét chặng: %w", err)
	}
	out := r.review()
	return &out, nil
}

func (PGRepo) InsertReview(ctx context.Context, ex db.Executor, r Review) error {
	if _, err := ex.ExecContext(ctx, `INSERT INTO stage_reviews
		(id, class_member_id, stage_version_id, body, author_id, revision, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		r.ID, r.MemberID, r.StageVersionID, r.Body, r.AuthorID, r.Revision, r.CreatedAt, r.UpdatedAt); err != nil {
		return fmt.Errorf("summary: ghi nhận xét chặng: %w", pgerr.Map(err))
	}
	return nil
}

func (PGRepo) UpdateReview(ctx context.Context, ex db.Executor, r Review, prevRevision int) error {
	err := db.ExecAffectOne(ctx, ex, `UPDATE stage_reviews SET body = $2, author_id = $3, revision = $4, updated_at = $5
		WHERE id = $1 AND revision = $6`, r.ID, r.Body, r.AuthorID, r.Revision, r.UpdatedAt, prevRevision)
	if errors.Is(err, db.ErrNoRowsAffected) {
		return ErrConcurrentEdit
	}
	if err != nil {
		return fmt.Errorf("summary: sửa nhận xét chặng: %w", pgerr.Map(err))
	}
	return nil
}
