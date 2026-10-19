package homework

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
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
)

// PGRepo là Repo trên Postgres.
type PGRepo struct{}

var _ Repo = PGRepo{}

type classRecord struct {
	ID              uuid.UUID `db:"id"`
	Code            string    `db:"code"`
	Name            string    `db:"name"`
	Status          string    `db:"status"`
	TeacherID       uuid.UUID `db:"teacher_id"`
	CourseVersionID uuid.UUID `db:"course_version_id"`
}

func (r classRecord) info() (ClassInfo, error) {
	st, err := domain.ParseClassStatus(r.Status)
	if err != nil {
		return ClassInfo{}, fmt.Errorf("homework: trạng thái lớp %q: %w", r.Status, err)
	}
	return ClassInfo{ID: r.ID, Code: r.Code, Name: r.Name, Status: st, TeacherID: r.TeacherID, CourseVersionID: r.CourseVersionID}, nil
}

const classColumns = `c.id, c.code, c.name, c.status, c.teacher_id, c.course_version_id`

func (PGRepo) Class(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassInfo, error) {
	return getClass(ctx, ex, `SELECT `+classColumns+` FROM classes c WHERE c.id = $1`, classID)
}

func (PGRepo) ClassForUpdate(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassInfo, error) {
	return getClass(ctx, ex, `SELECT `+classColumns+` FROM classes c WHERE c.id = $1 FOR UPDATE`, classID)
}

func getClass(ctx context.Context, ex db.Executor, q string, classID uuid.UUID) (ClassInfo, error) {
	var r classRecord
	err := sqlx.GetContext(ctx, ex, &r, q, classID)
	if errors.Is(err, sql.ErrNoRows) {
		return ClassInfo{}, classes.ErrClassNotFound
	}
	if err != nil {
		return ClassInfo{}, fmt.Errorf("homework: đọc lớp: %w", err)
	}
	return r.info()
}

func (PGRepo) Deadline(ctx context.Context, ex db.Executor, classID, lessonID uuid.UUID) (*time.Time, error) {
	var due time.Time
	err := sqlx.GetContext(ctx, ex, &due, `SELECT due_at FROM class_lesson_deadlines WHERE class_id = $1 AND lesson_id = $2`,
		classID, lessonID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("homework: đọc hạn nộp: %w", err)
	}
	return &due, nil
}

func (PGRepo) UpsertDeadline(ctx context.Context, ex db.Executor, d Deadline) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO class_lesson_deadlines (class_id, lesson_id, due_at, set_by, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (class_id, lesson_id) DO UPDATE SET due_at = EXCLUDED.due_at, set_by = EXCLUDED.set_by, updated_at = EXCLUDED.updated_at`,
		d.ClassID, d.LessonID, d.DueAt, d.SetBy, d.UpdatedAt)
	if err != nil {
		return pgerr.Map(err)
	}
	return nil
}

func (PGRepo) DeleteDeadline(ctx context.Context, ex db.Executor, classID, lessonID uuid.UUID) error {
	if _, err := ex.ExecContext(ctx, `DELETE FROM class_lesson_deadlines WHERE class_id = $1 AND lesson_id = $2`, classID, lessonID); err != nil {
		return pgerr.Map(err)
	}
	return nil
}

func (PGRepo) LockMember(ctx context.Context, ex db.Executor, memberID uuid.UUID) (LockedMember, error) {
	var r struct {
		ID          uuid.UUID `db:"id"`
		Status      string    `db:"status"`
		ClassStatus string    `db:"class_status"`
	}
	err := sqlx.GetContext(ctx, ex, &r, `SELECT cm.id, cm.status, c.status AS class_status
		FROM class_members cm JOIN classes c ON c.id = cm.class_id
		WHERE cm.id = $1 FOR UPDATE OF cm`, memberID)
	if errors.Is(err, sql.ErrNoRows) {
		return LockedMember{}, classes.ErrMemberNotFound
	}
	if err != nil {
		return LockedMember{}, fmt.Errorf("homework: khóa thành viên: %w", err)
	}
	ms, err := domain.ParseMemberStatus(r.Status)
	if err != nil {
		return LockedMember{}, fmt.Errorf("homework: trạng thái thành viên %q: %w", r.Status, err)
	}
	cs, err := domain.ParseClassStatus(r.ClassStatus)
	if err != nil {
		return LockedMember{}, fmt.Errorf("homework: trạng thái lớp %q: %w", r.ClassStatus, err)
	}
	return LockedMember{ID: r.ID, Status: ms, ClassStatus: cs}, nil
}

func (PGRepo) Latest(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) (*LatestSubmission, error) {
	var r struct {
		ID        uuid.UUID `db:"id"`
		AttemptNo int       `db:"attempt_no"`
		Status    string    `db:"status"`
	}
	err := sqlx.GetContext(ctx, ex, &r, `SELECT id, attempt_no, status FROM submissions
		WHERE class_member_id = $1 AND lesson_id = $2 ORDER BY attempt_no DESC LIMIT 1`, memberID, lessonID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("homework: đọc bài nộp mới nhất: %w", err)
	}
	return &LatestSubmission{ID: r.ID, AttemptNo: r.AttemptNo, Status: SubmissionStatus(r.Status)}, nil
}

func (PGRepo) Supersede(ctx context.Context, ex db.Executor, submissionID uuid.UUID) error {
	return affectOne(db.ExecAffectOne(ctx, ex,
		`UPDATE submissions SET status = 'superseded' WHERE id = $1 AND status = 'pending'`, submissionID))
}

func (PGRepo) UsableFiles(ctx context.Context, ex db.Executor, userID uuid.UUID, ids []uuid.UUID) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var n int
	err := sqlx.GetContext(ctx, ex, &n, `SELECT count(*) FROM media_files mf
		WHERE mf.id = ANY($1::uuid[]) AND mf.uploaded_by = $2 AND mf.kind = 'document' AND mf.status = 'ready'
		  AND NOT EXISTS (SELECT 1 FROM submission_files sf WHERE sf.media_id = mf.id)`, uuidStrings(ids), userID)
	if err != nil {
		return 0, fmt.Errorf("homework: kiểm file nộp: %w", err)
	}
	return n, nil
}

func (PGRepo) InsertSubmission(ctx context.Context, ex db.Executor, s NewSubmission) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO submissions (id, class_member_id, lesson_id, attempt_no, repo_url, commit_ref,
		submitted_at, is_late, status) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')`,
		s.ID, s.MemberID, s.LessonID, s.AttemptNo, s.RepoURL, s.CommitRef, s.SubmittedAt, s.IsLate)
	if err != nil {
		return pgerr.Map(err)
	}
	for i, id := range s.MediaIDs {
		if _, err := ex.ExecContext(ctx, `INSERT INTO submission_files (submission_id, media_id, position) VALUES ($1, $2, $3)`,
			s.ID, id, i+1); err != nil {
			return pgerr.Map(err)
		}
	}
	return nil
}

func (PGRepo) SubmissionContext(ctx context.Context, ex db.Executor, submissionID uuid.UUID) (SubmissionContext, error) {
	var r struct {
		ID           uuid.UUID `db:"submission_id"`
		LessonID     uuid.UUID `db:"lesson_id"`
		AttemptNo    int       `db:"attempt_no"`
		Status       string    `db:"submission_status"`
		MemberID     uuid.UUID `db:"member_id"`
		MemberStatus string    `db:"member_status"`
		UserID       uuid.UUID `db:"user_id"`
		StudentName  string    `db:"student_name"`
		StudentEmail string    `db:"student_email"`
		classRecord
	}
	// Cột của bài nộp đặt bí danh để không trùng id, status của lớp.
	err := sqlx.GetContext(ctx, ex, &r, `SELECT s.id AS submission_id, s.lesson_id, s.attempt_no, s.status AS submission_status,
		cm.id AS member_id, cm.status AS member_status, cm.user_id, u.full_name AS student_name, u.email AS student_email,
		`+classColumns+`
		FROM submissions s
		JOIN class_members cm ON cm.id = s.class_member_id
		JOIN users u ON u.id = cm.user_id
		JOIN classes c ON c.id = cm.class_id
		WHERE s.id = $1`, submissionID)
	if errors.Is(err, sql.ErrNoRows) {
		return SubmissionContext{}, ErrSubmissionNotFound
	}
	if err != nil {
		return SubmissionContext{}, fmt.Errorf("homework: đọc bài nộp: %w", err)
	}
	cls, err := r.info()
	if err != nil {
		return SubmissionContext{}, err
	}
	ms, err := domain.ParseMemberStatus(r.MemberStatus)
	if err != nil {
		return SubmissionContext{}, fmt.Errorf("homework: trạng thái thành viên %q: %w", r.MemberStatus, err)
	}
	return SubmissionContext{
		ID: r.ID, LessonID: r.LessonID, AttemptNo: r.AttemptNo, Status: SubmissionStatus(r.Status),
		MemberID: r.MemberID, MemberStatus: ms, UserID: r.UserID, StudentName: r.StudentName, StudentEmail: r.StudentEmail,
		Class: cls,
	}, nil
}

func (PGRepo) LockSubmission(ctx context.Context, ex db.Executor, submissionID uuid.UUID) (SubmissionStatus, error) {
	var st string
	err := sqlx.GetContext(ctx, ex, &st, `SELECT status FROM submissions WHERE id = $1 FOR UPDATE`, submissionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrSubmissionNotFound
	}
	if err != nil {
		return "", fmt.Errorf("homework: khóa bài nộp: %w", err)
	}
	return SubmissionStatus(st), nil
}

func (PGRepo) InsertGrade(ctx context.Context, ex db.Executor, g NewGradeRow) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO submission_grades (id, submission_id, graded_by, total_points, max_points, result,
		comment, graded_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		g.ID, g.SubmissionID, g.GradedBy, g.Grade.TotalPoints, g.Grade.MaxPoints, string(g.Grade.Result), g.Grade.Comment, g.GradedAt)
	if err != nil {
		return pgerr.Map(err)
	}
	for _, s := range g.Grade.Scores {
		if _, err := ex.ExecContext(ctx, `INSERT INTO grade_criterion_scores (grade_id, criterion_id, points, comment)
			VALUES ($1, $2, $3, $4)`, g.ID, s.CriterionID, s.Points, s.Comment); err != nil {
			return pgerr.Map(err)
		}
	}
	return nil
}

func (PGRepo) SetResult(ctx context.Context, ex db.Executor, submissionID uuid.UUID, r Result) error {
	return affectOne(db.ExecAffectOne(ctx, ex,
		`UPDATE submissions SET status = $2 WHERE id = $1 AND status <> 'superseded'`, submissionID, string(r)))
}

// affectOne đổi "không đúng một dòng" của lệnh chuyển trạng thái có điều kiện thành ErrConcurrentChange.
func affectOne(err error) error {
	if errors.Is(err, db.ErrNoRowsAffected) {
		return ErrConcurrentChange
	}
	if err != nil {
		return pgerr.Map(err)
	}
	return nil
}

// History đọc bằng bốn câu tập hợp (bài nộp, file, lần chấm, điểm tiêu chí) rồi ghép trong bộ nhớ.
func (PGRepo) History(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) ([]SubmissionRecord, error) {
	var subs []struct {
		ID          uuid.UUID `db:"id"`
		AttemptNo   int       `db:"attempt_no"`
		RepoURL     *string   `db:"repo_url"`
		CommitRef   *string   `db:"commit_ref"`
		SubmittedAt time.Time `db:"submitted_at"`
		IsLate      bool      `db:"is_late"`
		Status      string    `db:"status"`
	}
	if err := sqlx.SelectContext(ctx, ex, &subs, `SELECT id, attempt_no, repo_url, commit_ref, submitted_at, is_late, status
		FROM submissions WHERE class_member_id = $1 AND lesson_id = $2 ORDER BY attempt_no DESC`, memberID, lessonID); err != nil {
		return nil, fmt.Errorf("homework: đọc bài nộp: %w", err)
	}
	if len(subs) == 0 {
		return []SubmissionRecord{}, nil
	}
	out := make([]SubmissionRecord, len(subs))
	index := make(map[uuid.UUID]int, len(subs))
	subIDs := make([]uuid.UUID, len(subs))
	for i, s := range subs {
		out[i] = SubmissionRecord{
			ID: s.ID, AttemptNo: s.AttemptNo, RepoURL: s.RepoURL, CommitRef: s.CommitRef, SubmittedAt: s.SubmittedAt,
			IsLate: s.IsLate, Status: SubmissionStatus(s.Status), Files: []FileRef{}, Grades: []GradeRecord{},
		}
		index[s.ID], subIDs[i] = i, s.ID
	}

	var files []struct {
		SubmissionID uuid.UUID `db:"submission_id"`
		MediaID      uuid.UUID `db:"media_id"`
		FileName     string    `db:"file_name"`
		SizeBytes    int64     `db:"size_bytes"`
	}
	if err := sqlx.SelectContext(ctx, ex, &files, `SELECT sf.submission_id, sf.media_id, mf.original_name AS file_name, mf.size_bytes
		FROM submission_files sf JOIN media_files mf ON mf.id = sf.media_id
		WHERE sf.submission_id = ANY($1::uuid[]) ORDER BY sf.submission_id, sf.position`, uuidStrings(subIDs)); err != nil {
		return nil, fmt.Errorf("homework: đọc file nộp: %w", err)
	}
	for _, f := range files {
		i := index[f.SubmissionID]
		out[i].Files = append(out[i].Files, FileRef{MediaID: f.MediaID, FileName: f.FileName, SizeBytes: f.SizeBytes})
	}

	var grades []struct {
		ID           uuid.UUID `db:"id"`
		SubmissionID uuid.UUID `db:"submission_id"`
		GradedByID   uuid.UUID `db:"graded_by"`
		GradedBy     string    `db:"graded_by_name"`
		TotalPoints  int       `db:"total_points"`
		MaxPoints    int       `db:"max_points"`
		Result       string    `db:"result"`
		Comment      string    `db:"comment"`
		GradedAt     time.Time `db:"graded_at"`
	}
	if err := sqlx.SelectContext(ctx, ex, &grades, `SELECT g.id, g.submission_id, g.graded_by, u.full_name AS graded_by_name,
		g.total_points, g.max_points, g.result, g.comment, g.graded_at
		FROM submission_grades g JOIN users u ON u.id = g.graded_by
		WHERE g.submission_id = ANY($1::uuid[]) ORDER BY g.graded_at DESC, g.id DESC`, uuidStrings(subIDs)); err != nil {
		return nil, fmt.Errorf("homework: đọc lần chấm: %w", err)
	}
	if len(grades) == 0 {
		return out, nil
	}
	gradeIDs := make([]uuid.UUID, len(grades))
	where := make(map[uuid.UUID][2]int, len(grades)) // grade id → (vị trí bài nộp, vị trí lần chấm)
	for k, g := range grades {
		i := index[g.SubmissionID]
		out[i].Grades = append(out[i].Grades, GradeRecord{
			ID: g.ID, GradedByID: g.GradedByID, GradedBy: g.GradedBy, TotalPoints: g.TotalPoints, MaxPoints: g.MaxPoints,
			Result: Result(g.Result), Comment: g.Comment, GradedAt: g.GradedAt, Scores: []ScoreRecord{},
		})
		gradeIDs[k] = g.ID
		where[g.ID] = [2]int{i, len(out[i].Grades) - 1}
	}

	var scores []struct {
		GradeID     uuid.UUID `db:"grade_id"`
		CriterionID uuid.UUID `db:"criterion_id"`
		Description string    `db:"description"`
		Points      int       `db:"points"`
		MaxPoints   int       `db:"max_points"`
		Comment     string    `db:"comment"`
	}
	if err := sqlx.SelectContext(ctx, ex, &scores, `SELECT gcs.grade_id, gcs.criterion_id, rc.description, gcs.points,
		rc.max_points, gcs.comment
		FROM grade_criterion_scores gcs JOIN rubric_criteria rc ON rc.id = gcs.criterion_id
		WHERE gcs.grade_id = ANY($1::uuid[]) ORDER BY gcs.grade_id, rc.position`, uuidStrings(gradeIDs)); err != nil {
		return nil, fmt.Errorf("homework: đọc điểm tiêu chí: %w", err)
	}
	for _, s := range scores {
		w := where[s.GradeID]
		g := &out[w[0]].Grades[w[1]]
		g.Scores = append(g.Scores, ScoreRecord{
			CriterionID: s.CriterionID, Description: s.Description, Points: s.Points, MaxPoints: s.MaxPoints, Comment: s.Comment,
		})
	}
	return out, nil
}

func (PGRepo) HomeworkLessons(ctx context.Context, ex db.Executor, classID, courseVersionID uuid.UUID) ([]HomeworkLesson, error) {
	var rows []struct {
		LessonID    uuid.UUID  `db:"lesson_id"`
		Title       string     `db:"title"`
		StageID     uuid.UUID  `db:"stage_id"`
		StageName   string     `db:"stage_name"`
		PassPercent int        `db:"pass_percent"`
		MaxPoints   int        `db:"max_points"`
		DueAt       *time.Time `db:"due_at"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT l.id AS lesson_id, l.title, cvs.stage_id, s.name AS stage_name,
		hs.pass_percent, COALESCE(rc.max_points, 0) AS max_points, d.due_at
		FROM course_version_stages cvs
		JOIN lessons l ON l.stage_version_id = cvs.stage_version_id AND l.type = 'homework'
		JOIN stages s ON s.id = cvs.stage_id
		JOIN homework_settings hs ON hs.lesson_id = l.id
		LEFT JOIN LATERAL (SELECT sum(rcr.max_points)::int AS max_points FROM rubric_criteria rcr WHERE rcr.lesson_id = l.id) rc
		  ON true
		LEFT JOIN class_lesson_deadlines d ON d.class_id = $2 AND d.lesson_id = l.id
		WHERE cvs.course_version_id = $1
		ORDER BY cvs.position, l.position`, courseVersionID, classID); err != nil {
		return nil, fmt.Errorf("homework: đọc bài tập của khóa: %w", err)
	}
	out := make([]HomeworkLesson, len(rows))
	for i, r := range rows {
		out[i] = HomeworkLesson(r)
	}
	return out, nil
}

func (PGRepo) ActiveMembers(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]ActiveMember, error) {
	var rows []struct {
		MemberID uuid.UUID `db:"member_id"`
		UserID   uuid.UUID `db:"user_id"`
		Name     string    `db:"name"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT cm.id AS member_id, cm.user_id, u.full_name AS name
		FROM class_members cm JOIN users u ON u.id = cm.user_id
		WHERE cm.class_id = $1 AND cm.status = 'active' ORDER BY u.full_name, cm.id`, classID); err != nil {
		return nil, fmt.Errorf("homework: đọc thành viên: %w", err)
	}
	out := make([]ActiveMember, len(rows))
	for i, r := range rows {
		out[i] = ActiveMember(r)
	}
	return out, nil
}

func (PGRepo) LatestByClass(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]LatestRow, error) {
	var rows []struct {
		SubmissionID uuid.UUID `db:"submission_id"`
		MemberID     uuid.UUID `db:"member_id"`
		LessonID     uuid.UUID `db:"lesson_id"`
		AttemptNo    int       `db:"attempt_no"`
		SubmittedAt  time.Time `db:"submitted_at"`
		IsLate       bool      `db:"is_late"`
		HasRepo      bool      `db:"has_repo"`
		FileCount    int       `db:"file_count"`
		Status       string    `db:"status"`
		TotalPoints  *int      `db:"total_points"`
		MaxPoints    *int      `db:"max_points"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT s.id AS submission_id, s.class_member_id AS member_id, s.lesson_id,
		s.attempt_no, s.submitted_at, s.is_late, s.repo_url IS NOT NULL AS has_repo,
		(SELECT count(*) FROM submission_files sf WHERE sf.submission_id = s.id) AS file_count,
		s.status, g.total_points, g.max_points
		FROM (
		  SELECT DISTINCT ON (s.class_member_id, s.lesson_id) s.*
		  FROM submissions s JOIN class_members cm ON cm.id = s.class_member_id
		  WHERE cm.class_id = $1 AND cm.status = 'active'
		  ORDER BY s.class_member_id, s.lesson_id, s.attempt_no DESC
		) s
		LEFT JOIN LATERAL (
		  SELECT total_points, max_points FROM submission_grades
		  WHERE submission_id = s.id ORDER BY graded_at DESC, id DESC LIMIT 1
		) g ON true`, classID); err != nil {
		return nil, fmt.Errorf("homework: đọc bài nộp của lớp: %w", err)
	}
	out := make([]LatestRow, len(rows))
	for i, r := range rows {
		out[i] = LatestRow{
			SubmissionID: r.SubmissionID, MemberID: r.MemberID, LessonID: r.LessonID, AttemptNo: r.AttemptNo,
			SubmittedAt: r.SubmittedAt, IsLate: r.IsLate, HasRepo: r.HasRepo, FileCount: r.FileCount,
			Status: SubmissionStatus(r.Status), TotalPoints: r.TotalPoints, MaxPoints: r.MaxPoints,
		}
	}
	return out, nil
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
