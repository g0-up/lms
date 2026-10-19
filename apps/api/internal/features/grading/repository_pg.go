package grading

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/platform/db"
)

// PGRepo là Repo trên Postgres.
type PGRepo struct{}

var _ Repo = PGRepo{}

// queueSQL hợp nhất bài nộp pending và lượt quiz chờ xác nhận. Mỗi nhánh lọc theo partial index của nó
// (ix_submissions_pending, ix_quiz_attempts_pending); thành viên đã rời lớp không vào hàng chờ, nhưng lớp đã kết thúc
// vẫn vào vì bài nộp trước đó vẫn cần chấm.
const queueSQL = `SELECT q.kind, q.item_id, q.submitted_at, q.no, q.is_late, q.auto_submitted, q.score, q.max_score,
	c.teacher_id, u.id AS student_id, u.full_name AS student_name, cm.id AS member_id,
	c.id AS class_id, c.code AS class_code, c.name AS class_name, l.id AS lesson_id, l.title AS lesson_title,
	st.code AS stage_code, st.name AS stage_name
	FROM (
	  SELECT 'homework' AS kind, s.id AS item_id, s.class_member_id, s.lesson_id, s.submitted_at, s.attempt_no AS no,
	    s.is_late, false AS auto_submitted, NULL::integer AS score, NULL::integer AS max_score
	  FROM submissions s WHERE s.status = 'pending'
	  UNION ALL
	  SELECT 'quiz', a.id, a.class_member_id, a.lesson_id, a.submitted_at, a.attempt_no,
	    false, a.auto_submitted, a.score, a.max_score
	  FROM quiz_attempts a WHERE a.review_status = 'pending'
	) q
	JOIN class_members cm ON cm.id = q.class_member_id AND cm.status = 'active'
	JOIN classes c ON c.id = cm.class_id
	JOIN users u ON u.id = cm.user_id
	JOIN lessons l ON l.id = q.lesson_id
	JOIN stage_versions sv ON sv.id = l.stage_version_id
	JOIN stages st ON st.id = sv.stage_id
	WHERE $1::uuid IS NULL OR c.teacher_id = $1
	ORDER BY q.submitted_at, q.item_id`

func (PGRepo) Queue(ctx context.Context, ex db.Executor, teacherID *uuid.UUID) ([]QueueItem, error) {
	var rows []struct {
		Kind          string    `db:"kind"`
		ItemID        uuid.UUID `db:"item_id"`
		SubmittedAt   time.Time `db:"submitted_at"`
		No            int       `db:"no"`
		IsLate        bool      `db:"is_late"`
		AutoSubmitted bool      `db:"auto_submitted"`
		Score         *int      `db:"score"`
		MaxScore      *int      `db:"max_score"`
		TeacherID     uuid.UUID `db:"teacher_id"`
		StudentID     uuid.UUID `db:"student_id"`
		StudentName   string    `db:"student_name"`
		MemberID      uuid.UUID `db:"member_id"`
		ClassID       uuid.UUID `db:"class_id"`
		ClassCode     string    `db:"class_code"`
		ClassName     string    `db:"class_name"`
		LessonID      uuid.UUID `db:"lesson_id"`
		LessonTitle   string    `db:"lesson_title"`
		StageCode     string    `db:"stage_code"`
		StageName     string    `db:"stage_name"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, queueSQL, teacherID); err != nil {
		return nil, fmt.Errorf("grading: đọc hàng chờ chấm: %w", err)
	}
	out := make([]QueueItem, len(rows))
	for i, r := range rows {
		out[i] = QueueItem{
			Key: QueueKey{Kind: Kind(r.Kind), ItemID: r.ItemID}, SubmittedAt: r.SubmittedAt, No: r.No,
			IsLate: r.IsLate, AutoSubmitted: r.AutoSubmitted, Score: r.Score, MaxScore: r.MaxScore,
			TeacherID: r.TeacherID, Student: Ref{ID: r.StudentID, Name: r.StudentName}, MemberID: r.MemberID,
			Class:  Ref{ID: r.ClassID, Code: r.ClassCode, Name: r.ClassName},
			Lesson: Ref{ID: r.LessonID, Name: r.LessonTitle},
			Stage:  Ref{Code: r.StageCode, Name: r.StageName},
		}
	}
	return out, nil
}

func (PGRepo) FilterClasses(ctx context.Context, ex db.Executor, teacherID *uuid.UUID) ([]Ref, error) {
	var rows []struct {
		ID   uuid.UUID `db:"id"`
		Code string    `db:"code"`
		Name string    `db:"name"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT id, code, name FROM classes
		WHERE status <> 'draft' AND ($1::uuid IS NULL OR teacher_id = $1)
		ORDER BY code, id`, teacherID); err != nil {
		return nil, fmt.Errorf("grading: đọc lớp cho bộ lọc: %w", err)
	}
	out := make([]Ref, len(rows))
	for i, r := range rows {
		out[i] = Ref(r)
	}
	return out, nil
}

func (PGRepo) FilterLessons(ctx context.Context, ex db.Executor, teacherID *uuid.UUID) ([]LessonOptionRow, error) {
	var rows []struct {
		ClassID uuid.UUID `db:"class_id"`
		ID      uuid.UUID `db:"id"`
		Title   string    `db:"title"`
		Type    string    `db:"type"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT c.id AS class_id, l.id, l.title, l.type
		FROM classes c
		JOIN course_version_stages cvs ON cvs.course_version_id = c.course_version_id
		JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		LEFT JOIN quiz_settings qs ON qs.lesson_id = l.id
		WHERE c.status <> 'draft' AND ($1::uuid IS NULL OR c.teacher_id = $1)
		  AND (l.type = 'homework' OR (l.type = 'quiz' AND qs.mode = 'assessment'))
		ORDER BY c.code, c.id, cvs.position, l.position`, teacherID); err != nil {
		return nil, fmt.Errorf("grading: đọc học liệu cho bộ lọc: %w", err)
	}
	out := make([]LessonOptionRow, len(rows))
	for i, r := range rows {
		kind := KindHomework
		if r.Type == "quiz" {
			kind = KindQuiz
		}
		out[i] = LessonOptionRow{ClassID: r.ClassID, LessonOption: LessonOption{ID: r.ID, Title: r.Title, Kind: kind}}
	}
	return out, nil
}

// Recent lấy lần chấm mới nhất của mỗi bài nộp đã chấm và nhận xét mới nhất của mỗi lượt quiz bài tập đã xác nhận.
// Không lọc trạng thái thành viên: đây là lịch sử việc đã làm, không phải việc còn chờ.
func (PGRepo) Recent(ctx context.Context, ex db.Executor, teacherID *uuid.UUID, limit int) ([]Recent, error) {
	var rows []struct {
		Kind        string    `db:"kind"`
		ItemID      uuid.UUID `db:"item_id"`
		At          time.Time `db:"at"`
		ByID        uuid.UUID `db:"by_id"`
		ByName      string    `db:"by_name"`
		No          int       `db:"no"`
		Points      int       `db:"points"`
		MaxPoints   int       `db:"max_points"`
		Passed      bool      `db:"passed"`
		StudentID   uuid.UUID `db:"student_id"`
		StudentName string    `db:"student_name"`
		ClassID     uuid.UUID `db:"class_id"`
		ClassCode   string    `db:"class_code"`
		ClassName   string    `db:"class_name"`
		LessonID    uuid.UUID `db:"lesson_id"`
		LessonTitle string    `db:"lesson_title"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT e.kind, e.item_id, e.at, b.id AS by_id, b.full_name AS by_name,
		e.no, e.points, e.max_points, e.passed, u.id AS student_id, u.full_name AS student_name,
		c.id AS class_id, c.code AS class_code, c.name AS class_name, l.id AS lesson_id, l.title AS lesson_title
		FROM (
		  SELECT 'homework' AS kind, s.id AS item_id, g.id AS event_id, g.graded_at AS at, g.graded_by AS by_id,
		    s.attempt_no AS no, g.total_points AS points, g.max_points, g.result = 'passed' AS passed,
		    s.class_member_id, s.lesson_id
		  FROM (
		    SELECT DISTINCT ON (submission_id) * FROM submission_grades ORDER BY submission_id, graded_at DESC, id DESC
		  ) g
		  JOIN submissions s ON s.id = g.submission_id
		  UNION ALL
		  SELECT 'quiz', a.id, r.id, r.created_at, r.reviewer_id, a.attempt_no, a.score, a.max_score,
		    COALESCE(a.passed, false), a.class_member_id, a.lesson_id
		  FROM quiz_attempts a
		  JOIN quiz_settings qs ON qs.lesson_id = a.lesson_id AND qs.mode = 'assessment'
		  JOIN LATERAL (
		    SELECT id, created_at, reviewer_id FROM quiz_attempt_reviews
		    WHERE attempt_id = a.id ORDER BY created_at DESC, id DESC LIMIT 1
		  ) r ON true
		  WHERE a.review_status = 'confirmed'
		) e
		JOIN class_members cm ON cm.id = e.class_member_id
		JOIN classes c ON c.id = cm.class_id
		JOIN users u ON u.id = cm.user_id
		JOIN users b ON b.id = e.by_id
		JOIN lessons l ON l.id = e.lesson_id
		WHERE $1::uuid IS NULL OR c.teacher_id = $1
		ORDER BY e.at DESC, e.event_id DESC
		LIMIT $2`, teacherID, limit); err != nil {
		return nil, fmt.Errorf("grading: đọc lần chấm gần đây: %w", err)
	}
	out := make([]Recent, len(rows))
	for i, r := range rows {
		out[i] = Recent{
			Key: QueueKey{Kind: Kind(r.Kind), ItemID: r.ItemID}, At: r.At, By: Ref{ID: r.ByID, Name: r.ByName},
			Student: Ref{ID: r.StudentID, Name: r.StudentName},
			Class:   Ref{ID: r.ClassID, Code: r.ClassCode, Name: r.ClassName},
			Lesson:  Ref{ID: r.LessonID, Name: r.LessonTitle},
			No:      r.No, Points: r.Points, MaxPoints: r.MaxPoints, Passed: r.Passed,
		}
	}
	return out, nil
}

func (PGRepo) Teachers(ctx context.Context, ex db.Executor) ([]Teacher, error) {
	var rows []struct {
		ID    uuid.UUID `db:"id"`
		Name  string    `db:"full_name"`
		Email string    `db:"email"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT id, full_name, email FROM users
		WHERE role = 'teacher' AND status <> 'disabled'
		ORDER BY full_name, id`); err != nil {
		return nil, fmt.Errorf("grading: đọc giảng viên: %w", err)
	}
	out := make([]Teacher, len(rows))
	for i, r := range rows {
		out[i] = Teacher(r)
	}
	return out, nil
}

// AvgHandledHours lấy lần chấm đầu của mỗi bài nộp và nhận xét đầu của mỗi lượt quiz bài tập (với quiz bài tập, nhận
// xét đầu là lần xác nhận), rồi tính trung bình theo người làm lần đó. Lần sửa điểm, sửa nhận xét và nhận xét quiz
// luyện tập không tính.
func (PGRepo) AvgHandledHours(ctx context.Context, ex db.Executor) (map[uuid.UUID]float64, error) {
	var rows []struct {
		ByID     uuid.UUID `db:"by_id"`
		AvgHours float64   `db:"avg_hours"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT e.by_id,
		avg(extract(epoch FROM e.handled_at - e.submitted_at) / 3600)::float8 AS avg_hours
		FROM (
		  (SELECT DISTINCT ON (g.submission_id) g.graded_by AS by_id, g.graded_at AS handled_at, s.submitted_at
		   FROM submission_grades g
		   JOIN submissions s ON s.id = g.submission_id
		   ORDER BY g.submission_id, g.graded_at, g.id)
		  UNION ALL
		  (SELECT DISTINCT ON (r.attempt_id) r.reviewer_id, r.created_at, a.submitted_at
		   FROM quiz_attempt_reviews r
		   JOIN quiz_attempts a ON a.id = r.attempt_id AND a.submitted_at IS NOT NULL
		   JOIN quiz_settings qs ON qs.lesson_id = a.lesson_id AND qs.mode = 'assessment'
		   ORDER BY r.attempt_id, r.created_at, r.id)
		) e
		GROUP BY e.by_id`); err != nil {
		return nil, fmt.Errorf("grading: đọc thời gian xử lý trung bình: %w", err)
	}
	out := make(map[uuid.UUID]float64, len(rows))
	for _, r := range rows {
		out[r.ByID] = r.AvgHours
	}
	return out, nil
}

// ReminderCandidates chọn theo hạn nộp của lớp; homework không có hạn không có dòng class_lesson_deadlines nên không
// bao giờ được nhắc. Bản nộp ở bất kỳ trạng thái nào (kể cả đã bị thay) đều tính là đã nộp.
func (PGRepo) ReminderCandidates(ctx context.Context, ex db.Executor, after, until time.Time) ([]Reminder, error) {
	var rows []struct {
		MemberID     uuid.UUID `db:"member_id"`
		StudentName  string    `db:"student_name"`
		StudentEmail string    `db:"student_email"`
		ClassID      uuid.UUID `db:"class_id"`
		ClassCode    string    `db:"class_code"`
		ClassName    string    `db:"class_name"`
		LessonID     uuid.UUID `db:"lesson_id"`
		LessonTitle  string    `db:"lesson_title"`
		DueAt        time.Time `db:"due_at"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT cm.id AS member_id, u.full_name AS student_name,
		u.email AS student_email, c.id AS class_id, c.code AS class_code, c.name AS class_name,
		l.id AS lesson_id, l.title AS lesson_title, d.due_at
		FROM class_lesson_deadlines d
		JOIN classes c ON c.id = d.class_id AND c.status = 'active'
		JOIN lessons l ON l.id = d.lesson_id AND l.type = 'homework'
		JOIN class_members cm ON cm.class_id = c.id AND cm.status = 'active'
		JOIN users u ON u.id = cm.user_id AND u.status <> 'disabled'
		WHERE d.due_at > $1 AND d.due_at <= $2
		  AND NOT EXISTS (SELECT 1 FROM submissions s WHERE s.class_member_id = cm.id AND s.lesson_id = d.lesson_id)
		ORDER BY d.due_at, c.code, u.full_name, cm.id`, after, until); err != nil {
		return nil, fmt.Errorf("grading: đọc học viên cần nhắc hạn nộp: %w", err)
	}
	out := make([]Reminder, len(rows))
	for i, r := range rows {
		out[i] = Reminder{
			MemberID: r.MemberID, StudentName: r.StudentName, StudentEmail: r.StudentEmail, DueAt: r.DueAt,
			Class:  Ref{ID: r.ClassID, Code: r.ClassCode, Name: r.ClassName},
			Lesson: Ref{ID: r.LessonID, Name: r.LessonTitle},
		}
	}
	return out, nil
}

func (PGRepo) ClaimRun(ctx context.Context, ex db.Executor, key string, at time.Time) (bool, error) {
	res, err := ex.ExecContext(ctx, `INSERT INTO scheduled_job_runs (run_key, ran_at) VALUES ($1, $2)
		ON CONFLICT (run_key) DO NOTHING`, key, at)
	if err != nil {
		return false, fmt.Errorf("grading: ghi khóa việc hằng ngày: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("grading: ghi khóa việc hằng ngày: %w", err)
	}
	return n == 1, nil
}
