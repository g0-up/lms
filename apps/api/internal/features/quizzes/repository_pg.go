package quizzes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

// jsonCol đọc một cột JSON (to_json của mảng uuid, jsonb option_order) vào T.
type jsonCol[T any] struct{ V T }

func (c *jsonCol[T]) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("quizzes: không đọc được JSON từ %T", src)
	}
	if err := json.Unmarshal(raw, &c.V); err != nil {
		return fmt.Errorf("quizzes: cột JSON: %w", err)
	}
	return nil
}

func (PGRepo) QuizInCourseVersion(ctx context.Context, ex db.Executor, courseVersionID, lessonID uuid.UUID) (*stages.QuizContent, error) {
	var ok bool
	if err := sqlx.GetContext(ctx, ex, &ok, `SELECT EXISTS (SELECT 1 FROM course_version_stages cvs
		JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		WHERE cvs.course_version_id = $1 AND l.id = $2 AND l.type = 'quiz')`, courseVersionID, lessonID); err != nil {
		return nil, fmt.Errorf("quizzes: kiểm học liệu quiz: %w", err)
	}
	if !ok {
		return nil, nil
	}
	return PGRepo{}.Quiz(ctx, ex, lessonID)
}

// settingsRecord là một hàng quiz_settings.
type settingsRecord struct {
	Mode             string `db:"mode"`
	MaxAttempts      *int   `db:"max_attempts"`
	TimeLimitSeconds *int   `db:"time_limit_seconds"`
	Shuffle          bool   `db:"shuffle"`
	PassPercent      *int   `db:"pass_percent"`
}

// content là cấu hình quiz chưa có câu hỏi.
func (r settingsRecord) content() (stages.QuizContent, error) {
	mode, err := stages.ParseQuizMode(r.Mode)
	if err != nil {
		return stages.QuizContent{}, fmt.Errorf("quizzes: chế độ quiz %q: %w", r.Mode, err)
	}
	c := stages.QuizContent{Mode: mode, MaxAttempts: r.MaxAttempts, Shuffle: r.Shuffle, PassPercent: r.PassPercent}
	if r.TimeLimitSeconds != nil {
		d := time.Duration(*r.TimeLimitSeconds) * time.Second
		c.TimeLimit = &d
	}
	return c, nil
}

func (PGRepo) Quiz(ctx context.Context, ex db.Executor, lessonID uuid.UUID) (*stages.QuizContent, error) {
	var s settingsRecord
	err := sqlx.GetContext(ctx, ex, &s, `SELECT mode, max_attempts, time_limit_seconds, shuffle, pass_percent
		FROM quiz_settings WHERE lesson_id = $1`, lessonID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("quizzes: đọc cấu hình quiz: %w", err)
	}
	c, err := s.content()
	if err != nil {
		return nil, err
	}
	var questions []struct {
		ID              uuid.UUID `db:"id"`
		QuestionKey     string    `db:"question_key"`
		Type            string    `db:"type"`
		BodyHTML        *string   `db:"body_html"`
		ExplanationHTML *string   `db:"explanation_html"`
		Points          int       `db:"points"`
	}
	if err := sqlx.SelectContext(ctx, ex, &questions, `SELECT id, question_key, type, body_html, explanation_html, points
		FROM questions WHERE lesson_id = $1 ORDER BY position`, lessonID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc câu hỏi: %w", err)
	}
	var options []struct {
		ID         uuid.UUID `db:"id"`
		QuestionID uuid.UUID `db:"question_id"`
		Body       string    `db:"body"`
		IsCorrect  bool      `db:"is_correct"`
	}
	if err := sqlx.SelectContext(ctx, ex, &options, `SELECT o.id, o.question_id, o.body, o.is_correct
		FROM question_options o JOIN questions q ON q.id = o.question_id
		WHERE q.lesson_id = $1 ORDER BY o.question_id, o.position`, lessonID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc phương án: %w", err)
	}
	optionsOf := make(map[uuid.UUID][]stages.Option, len(questions))
	for _, o := range options {
		optionsOf[o.QuestionID] = append(optionsOf[o.QuestionID], stages.Option{ID: o.ID, Body: o.Body, Correct: o.IsCorrect})
	}
	for _, q := range questions {
		t, err := stages.ParseQuestionType(q.Type)
		if err != nil {
			return nil, fmt.Errorf("quizzes: loại câu hỏi %q: %w", q.Type, err)
		}
		c.Questions = append(c.Questions, &stages.Question{
			ID: q.ID, Key: domain.LessonKey(q.QuestionKey), Type: t, BodyHTML: q.BodyHTML,
			ExplanationHTML: q.ExplanationHTML, Points: q.Points, Options: optionsOf[q.ID],
		})
	}
	return &c, nil
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
		return LockedMember{}, fmt.Errorf("quizzes: khóa thành viên: %w", err)
	}
	ms, err := domain.ParseMemberStatus(r.Status)
	if err != nil {
		return LockedMember{}, fmt.Errorf("quizzes: trạng thái thành viên %q: %w", r.Status, err)
	}
	cs, err := domain.ParseClassStatus(r.ClassStatus)
	if err != nil {
		return LockedMember{}, fmt.Errorf("quizzes: trạng thái lớp %q: %w", r.ClassStatus, err)
	}
	return LockedMember{ID: r.ID, Status: ms, ClassStatus: cs}, nil
}

func (PGRepo) LockClassMember(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (ClassMember, error) {
	var r struct {
		ID          uuid.UUID `db:"id"`
		Name        string    `db:"name"`
		Status      string    `db:"status"`
		ClassStatus string    `db:"class_status"`
	}
	err := sqlx.GetContext(ctx, ex, &r, `SELECT cm.id, u.full_name AS name, cm.status, c.status AS class_status
		FROM class_members cm JOIN classes c ON c.id = cm.class_id JOIN users u ON u.id = cm.user_id
		WHERE cm.id = $1 AND cm.class_id = $2 FOR UPDATE OF cm`, memberID, classID)
	if errors.Is(err, sql.ErrNoRows) {
		return ClassMember{}, classes.ErrMemberNotFound
	}
	if err != nil {
		return ClassMember{}, fmt.Errorf("quizzes: khóa thành viên của lớp: %w", err)
	}
	ms, err := domain.ParseMemberStatus(r.Status)
	if err != nil {
		return ClassMember{}, fmt.Errorf("quizzes: trạng thái thành viên %q: %w", r.Status, err)
	}
	cs, err := domain.ParseClassStatus(r.ClassStatus)
	if err != nil {
		return ClassMember{}, fmt.Errorf("quizzes: trạng thái lớp %q: %w", r.ClassStatus, err)
	}
	return ClassMember{ID: r.ID, Name: r.Name, Status: ms, ClassStatus: cs}, nil
}

func (PGRepo) AttemptRef(ctx context.Context, ex db.Executor, attemptID uuid.UUID) (AttemptRef, error) {
	var r struct {
		ID       uuid.UUID `db:"id"`
		MemberID uuid.UUID `db:"class_member_id"`
		ClassID  uuid.UUID `db:"class_id"`
		UserID   uuid.UUID `db:"user_id"`
		LessonID uuid.UUID `db:"lesson_id"`
	}
	err := sqlx.GetContext(ctx, ex, &r, `SELECT a.id, a.class_member_id, cm.class_id, cm.user_id, a.lesson_id
		FROM quiz_attempts a JOIN class_members cm ON cm.id = a.class_member_id WHERE a.id = $1`, attemptID)
	if errors.Is(err, sql.ErrNoRows) {
		return AttemptRef{}, ErrAttemptNotFound
	}
	if err != nil {
		return AttemptRef{}, fmt.Errorf("quizzes: đọc chủ lượt: %w", err)
	}
	return AttemptRef(r), nil
}

const attemptColumns = `id, class_member_id, lesson_id, attempt_no, to_json(question_order) AS question_order, option_order,
	started_at, deadline_at, submitted_at, auto_submitted, score, max_score, passed, review_status`

type attemptRecord struct {
	ID            uuid.UUID                          `db:"id"`
	MemberID      uuid.UUID                          `db:"class_member_id"`
	LessonID      uuid.UUID                          `db:"lesson_id"`
	No            int                                `db:"attempt_no"`
	QuestionOrder jsonCol[[]uuid.UUID]               `db:"question_order"`
	OptionOrder   jsonCol[map[uuid.UUID][]uuid.UUID] `db:"option_order"`
	StartedAt     time.Time                          `db:"started_at"`
	DeadlineAt    *time.Time                         `db:"deadline_at"`
	SubmittedAt   *time.Time                         `db:"submitted_at"`
	AutoSubmitted bool                               `db:"auto_submitted"`
	Score         *int                               `db:"score"`
	MaxScore      int                                `db:"max_score"`
	Passed        *bool                              `db:"passed"`
	ReviewStatus  *string                            `db:"review_status"`
}

func (r attemptRecord) attempt() (Attempt, error) {
	a := Attempt{
		ID: r.ID, MemberID: r.MemberID, LessonID: r.LessonID, No: r.No, QuestionOrder: r.QuestionOrder.V,
		OptionOrder: r.OptionOrder.V, StartedAt: r.StartedAt, DeadlineAt: r.DeadlineAt, SubmittedAt: r.SubmittedAt,
		AutoSubmitted: r.AutoSubmitted, Score: r.Score, MaxScore: r.MaxScore, Passed: r.Passed,
	}
	if r.ReviewStatus != nil {
		st, err := ParseReviewStatus(*r.ReviewStatus)
		if err != nil {
			return Attempt{}, err
		}
		a.ReviewStatus = &st
	}
	return a, nil
}

func (PGRepo) Attempt(ctx context.Context, ex db.Executor, attemptID uuid.UUID) (*Attempt, error) {
	return getAttempt(ctx, ex, `SELECT `+attemptColumns+` FROM quiz_attempts WHERE id = $1`, attemptID)
}

func (PGRepo) AttemptForUpdate(ctx context.Context, ex db.Executor, attemptID uuid.UUID) (*Attempt, error) {
	return getAttempt(ctx, ex, `SELECT `+attemptColumns+` FROM quiz_attempts WHERE id = $1 FOR UPDATE`, attemptID)
}

func getAttempt(ctx context.Context, ex db.Executor, q string, attemptID uuid.UUID) (*Attempt, error) {
	var r attemptRecord
	err := sqlx.GetContext(ctx, ex, &r, q, attemptID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAttemptNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("quizzes: đọc lượt: %w", err)
	}
	a, err := r.attempt()
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (PGRepo) Attempts(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) ([]Attempt, error) {
	var rows []attemptRecord
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT `+attemptColumns+` FROM quiz_attempts
		WHERE class_member_id = $1 AND lesson_id = $2 ORDER BY attempt_no`, memberID, lessonID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc các lượt: %w", err)
	}
	out := make([]Attempt, len(rows))
	for i, r := range rows {
		a, err := r.attempt()
		if err != nil {
			return nil, err
		}
		out[i] = a
	}
	return out, nil
}

func (PGRepo) Granted(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) (int, error) {
	var n int
	if err := sqlx.GetContext(ctx, ex, &n, `SELECT COALESCE(sum(extra_attempts), 0) FROM quiz_attempt_grants
		WHERE class_member_id = $1 AND lesson_id = $2`, memberID, lessonID); err != nil {
		return 0, fmt.Errorf("quizzes: đọc lượt cấp thêm: %w", err)
	}
	return n, nil
}

func (PGRepo) InsertAttempt(ctx context.Context, ex db.Executor, a *Attempt) error {
	order, err := json.Marshal(a.OptionOrder)
	if err != nil {
		return fmt.Errorf("quizzes: option_order: %w", err)
	}
	_, err = ex.ExecContext(ctx, `INSERT INTO quiz_attempts (id, class_member_id, lesson_id, attempt_no, question_order,
		option_order, started_at, deadline_at, max_score) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9)`,
		a.ID, a.MemberID, a.LessonID, a.No, pgtype.FlatArray[uuid.UUID](a.QuestionOrder), string(order), a.StartedAt,
		a.DeadlineAt, a.MaxScore)
	if err != nil {
		return pgerr.Map(err)
	}
	return nil
}

func (PGRepo) Answers(ctx context.Context, ex db.Executor, attemptID uuid.UUID) ([]Answer, error) {
	var rows []struct {
		QuestionID uuid.UUID            `db:"question_id"`
		Selected   jsonCol[[]uuid.UUID] `db:"selected"`
		IsCorrect  *bool                `db:"is_correct"`
		Points     *int                 `db:"points_awarded"`
		SavedAt    time.Time            `db:"saved_at"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT question_id, to_json(selected_option_ids) AS selected, is_correct,
		points_awarded, saved_at FROM quiz_attempt_answers WHERE attempt_id = $1`, attemptID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc câu trả lời: %w", err)
	}
	out := make([]Answer, len(rows))
	for i, r := range rows {
		out[i] = Answer{QuestionID: r.QuestionID, Selected: r.Selected.V, IsCorrect: r.IsCorrect, Points: r.Points, SavedAt: r.SavedAt}
	}
	return out, nil
}

func (PGRepo) UpsertAnswer(ctx context.Context, ex db.Executor, attemptID, questionID uuid.UUID, selected []uuid.UUID, savedAt time.Time) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO quiz_attempt_answers (attempt_id, question_id, selected_option_ids, saved_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (attempt_id, question_id) DO UPDATE SET selected_option_ids = EXCLUDED.selected_option_ids,
		saved_at = EXCLUDED.saved_at`, attemptID, questionID, pgtype.FlatArray[uuid.UUID](selected), savedAt)
	if err != nil {
		return pgerr.Map(err)
	}
	return nil
}

func (PGRepo) GradeAnswers(ctx context.Context, ex db.Executor, attemptID uuid.UUID, per []QuestionResult, at time.Time) error {
	for _, r := range per {
		// Câu chưa lưu được chèn với mảng rỗng để thống kê câu hỏi có đủ mẫu số; câu đã lưu giữ phương án và saved_at.
		_, err := ex.ExecContext(ctx, `INSERT INTO quiz_attempt_answers (attempt_id, question_id, selected_option_ids,
			is_correct, points_awarded, saved_at) VALUES ($1, $2, '{}', $3, $4, $5)
			ON CONFLICT (attempt_id, question_id) DO UPDATE SET is_correct = EXCLUDED.is_correct,
			points_awarded = EXCLUDED.points_awarded`, attemptID, r.QuestionID, r.Correct, r.Points, at)
		if err != nil {
			return pgerr.Map(err)
		}
	}
	return nil
}

func (PGRepo) Finish(ctx context.Context, ex db.Executor, a *Attempt) error {
	var review *string
	if a.ReviewStatus != nil {
		s := string(*a.ReviewStatus)
		review = &s
	}
	return affectOne(db.ExecAffectOne(ctx, ex, `UPDATE quiz_attempts SET submitted_at = $2, auto_submitted = $3, score = $4,
		max_score = $5, passed = $6, review_status = $7 WHERE id = $1 AND submitted_at IS NULL`,
		a.ID, a.SubmittedAt, a.AutoSubmitted, a.Score, a.MaxScore, a.Passed, review))
}

func (PGRepo) SupersedePending(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) error {
	_, err := ex.ExecContext(ctx, `UPDATE quiz_attempts SET review_status = 'superseded'
		WHERE class_member_id = $1 AND lesson_id = $2 AND review_status = 'pending'`, memberID, lessonID)
	if err != nil {
		return pgerr.Map(err)
	}
	return nil
}

func (PGRepo) ExpiredOpen(ctx context.Context, ex db.Executor, before time.Time, limit int) ([]uuid.UUID, error) {
	var out []uuid.UUID
	// Lượt của thành viên đã rời hoặc lớp không còn chạy giữ nguyên trạng thái mở: không ai nộp được và không vào
	// hàng chờ xác nhận.
	if err := sqlx.SelectContext(ctx, ex, &out, `SELECT a.id FROM quiz_attempts a
		JOIN class_members cm ON cm.id = a.class_member_id AND cm.status = 'active'
		JOIN classes c ON c.id = cm.class_id AND c.status = 'active'
		WHERE a.submitted_at IS NULL AND a.deadline_at < $1
		ORDER BY a.deadline_at, a.id LIMIT $2`, before, limit); err != nil {
		return nil, fmt.Errorf("quizzes: đọc lượt quá giờ: %w", err)
	}
	return out, nil
}

func (PGRepo) LatestReview(ctx context.Context, ex db.Executor, attemptID uuid.UUID) (*Review, error) {
	var r struct {
		ID           uuid.UUID `db:"id"`
		Comment      string    `db:"comment"`
		ReviewerName string    `db:"reviewer_name"`
		CreatedAt    time.Time `db:"created_at"`
		Total        int       `db:"total"`
	}
	err := sqlx.GetContext(ctx, ex, &r, `SELECT r.id, r.comment, u.full_name AS reviewer_name, r.created_at,
		count(*) OVER () AS total
		FROM quiz_attempt_reviews r JOIN users u ON u.id = r.reviewer_id
		WHERE r.attempt_id = $1 ORDER BY r.created_at DESC, r.id DESC LIMIT 1`, attemptID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("quizzes: đọc nhận xét: %w", err)
	}
	var notes []struct {
		QuestionID uuid.UUID `db:"question_id"`
		Note       string    `db:"note"`
	}
	if err := sqlx.SelectContext(ctx, ex, &notes, `SELECT question_id, note FROM quiz_review_notes WHERE review_id = $1`,
		r.ID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc ghi chú từng câu: %w", err)
	}
	out := &Review{Comment: r.Comment, ReviewerName: r.ReviewerName, CreatedAt: r.CreatedAt, Updated: r.Total > 1,
		Notes: make(map[uuid.UUID]string, len(notes))}
	for _, n := range notes {
		out.Notes[n.QuestionID] = n.Note
	}
	return out, nil
}

// ---- Xác nhận và nhận xét của staff ----

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
		return ClassInfo{}, fmt.Errorf("quizzes: trạng thái lớp %q: %w", r.Status, err)
	}
	return ClassInfo{ID: r.ID, Code: r.Code, Name: r.Name, Status: st, TeacherID: r.TeacherID, CourseVersionID: r.CourseVersionID}, nil
}

const classColumns = `c.id, c.code, c.name, c.status, c.teacher_id, c.course_version_id`

func (PGRepo) SubmittedAttemptContext(ctx context.Context, ex db.Executor, attemptID uuid.UUID) (AttemptContext, error) {
	var r struct {
		AttemptID    uuid.UUID `db:"attempt_id"`
		LessonID     uuid.UUID `db:"lesson_id"`
		LessonTitle  string    `db:"lesson_title"`
		StageID      uuid.UUID `db:"stage_id"`
		StageName    string    `db:"stage_name"`
		MemberID     uuid.UUID `db:"member_id"`
		MemberStatus string    `db:"member_status"`
		UserID       uuid.UUID `db:"user_id"`
		StudentName  string    `db:"student_name"`
		StudentEmail string    `db:"student_email"`
		classRecord
	}
	// Cột của lượt, học liệu và chặng đặt bí danh để không trùng id, name, status của lớp.
	err := sqlx.GetContext(ctx, ex, &r, `SELECT a.id AS attempt_id, a.lesson_id, l.title AS lesson_title,
		s.id AS stage_id, s.name AS stage_name, cm.id AS member_id, cm.status AS member_status, cm.user_id,
		u.full_name AS student_name, u.email AS student_email, `+classColumns+`
		FROM quiz_attempts a
		JOIN class_members cm ON cm.id = a.class_member_id
		JOIN users u ON u.id = cm.user_id
		JOIN classes c ON c.id = cm.class_id
		JOIN lessons l ON l.id = a.lesson_id
		JOIN stage_versions sv ON sv.id = l.stage_version_id
		JOIN stages s ON s.id = sv.stage_id
		WHERE a.id = $1 AND a.submitted_at IS NOT NULL`, attemptID)
	if errors.Is(err, sql.ErrNoRows) {
		return AttemptContext{}, ErrSubmittedNotFound
	}
	if err != nil {
		return AttemptContext{}, fmt.Errorf("quizzes: đọc lượt đã nộp: %w", err)
	}
	cls, err := r.info()
	if err != nil {
		return AttemptContext{}, err
	}
	ms, err := domain.ParseMemberStatus(r.MemberStatus)
	if err != nil {
		return AttemptContext{}, fmt.Errorf("quizzes: trạng thái thành viên %q: %w", r.MemberStatus, err)
	}
	return AttemptContext{
		AttemptID: r.AttemptID, MemberID: r.MemberID, MemberStatus: ms, UserID: r.UserID, StudentName: r.StudentName,
		StudentEmail: r.StudentEmail, Class: cls, LessonID: r.LessonID, LessonTitle: r.LessonTitle, StageID: r.StageID,
		StageName: r.StageName,
	}, nil
}

func (PGRepo) Reviews(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) ([]ReviewRecord, error) {
	var rows []struct {
		ID           uuid.UUID `db:"id"`
		AttemptID    uuid.UUID `db:"attempt_id"`
		ReviewerID   uuid.UUID `db:"reviewer_id"`
		ReviewerName string    `db:"reviewer_name"`
		Comment      string    `db:"comment"`
		CreatedAt    time.Time `db:"created_at"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT r.id, r.attempt_id, r.reviewer_id, u.full_name AS reviewer_name,
		r.comment, r.created_at
		FROM quiz_attempt_reviews r
		JOIN quiz_attempts a ON a.id = r.attempt_id
		JOIN users u ON u.id = r.reviewer_id
		WHERE a.class_member_id = $1 AND a.lesson_id = $2
		ORDER BY r.created_at DESC, r.id DESC`, memberID, lessonID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc lịch sử nhận xét: %w", err)
	}
	var notes []struct {
		ReviewID   uuid.UUID `db:"review_id"`
		QuestionID uuid.UUID `db:"question_id"`
		Note       string    `db:"note"`
	}
	// Ghi chú theo thứ tự câu học viên thấy trong lượt.
	if err := sqlx.SelectContext(ctx, ex, &notes, `SELECT n.review_id, n.question_id, n.note
		FROM quiz_review_notes n
		JOIN quiz_attempt_reviews r ON r.id = n.review_id
		JOIN quiz_attempts a ON a.id = r.attempt_id
		WHERE a.class_member_id = $1 AND a.lesson_id = $2
		ORDER BY n.review_id, array_position(a.question_order, n.question_id)`, memberID, lessonID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc ghi chú từng câu: %w", err)
	}
	notesOf := make(map[uuid.UUID][]ReviewNote, len(rows))
	for _, n := range notes {
		notesOf[n.ReviewID] = append(notesOf[n.ReviewID], ReviewNote{QuestionID: n.QuestionID, Note: n.Note})
	}
	out := make([]ReviewRecord, len(rows))
	for i, r := range rows {
		out[i] = ReviewRecord{
			ID: r.ID, AttemptID: r.AttemptID, ReviewerID: r.ReviewerID, ReviewerName: r.ReviewerName, Comment: r.Comment,
			CreatedAt: r.CreatedAt, Notes: notesOf[r.ID],
		}
	}
	return out, nil
}

func (PGRepo) InsertReview(ctx context.Context, ex db.Executor, r NewReview) error {
	if _, err := ex.ExecContext(ctx, `INSERT INTO quiz_attempt_reviews (id, attempt_id, reviewer_id, comment, created_at)
		VALUES ($1, $2, $3, $4, $5)`, r.ID, r.AttemptID, r.ReviewerID, r.Comment, r.CreatedAt); err != nil {
		return pgerr.Map(err)
	}
	if len(r.Notes) == 0 {
		return nil
	}
	questions := make([]uuid.UUID, len(r.Notes))
	texts := make([]string, len(r.Notes))
	for i, n := range r.Notes {
		questions[i], texts[i] = n.QuestionID, n.Note
	}
	if _, err := ex.ExecContext(ctx, `INSERT INTO quiz_review_notes (review_id, question_id, note)
		SELECT $1, t.question_id, t.note FROM unnest($2::uuid[], $3::text[]) AS t(question_id, note)`,
		r.ID, pgtype.FlatArray[uuid.UUID](questions), pgtype.FlatArray[string](texts)); err != nil {
		return pgerr.Map(err)
	}
	return nil
}

func (PGRepo) ConfirmReview(ctx context.Context, ex db.Executor, attemptID uuid.UUID, from *ReviewStatus) error {
	var cur *string
	if from != nil {
		s := string(*from)
		cur = &s
	}
	return affectOne(db.ExecAffectOne(ctx, ex, `UPDATE quiz_attempts SET review_status = 'confirmed'
		WHERE id = $1 AND submitted_at IS NOT NULL AND review_status IS NOT DISTINCT FROM $2::text`, attemptID, cur))
}

// ---- Tổng quan quiz của lớp ----

func (PGRepo) Class(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassInfo, error) {
	var r classRecord
	err := sqlx.GetContext(ctx, ex, &r, `SELECT `+classColumns+` FROM classes c WHERE c.id = $1`, classID)
	if errors.Is(err, sql.ErrNoRows) {
		return ClassInfo{}, classes.ErrClassNotFound
	}
	if err != nil {
		return ClassInfo{}, fmt.Errorf("quizzes: đọc lớp: %w", err)
	}
	return r.info()
}

func (PGRepo) QuizLessons(ctx context.Context, ex db.Executor, courseVersionID uuid.UUID) ([]QuizLesson, error) {
	var rows []struct {
		LessonID      uuid.UUID `db:"lesson_id"`
		Title         string    `db:"title"`
		StageID       uuid.UUID `db:"stage_id"`
		StageName     string    `db:"stage_name"`
		QuestionCount int       `db:"question_count"`
		settingsRecord
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT l.id AS lesson_id, l.title, cvs.stage_id, s.name AS stage_name,
		qs.mode, qs.max_attempts, qs.time_limit_seconds, qs.shuffle, qs.pass_percent, COALESCE(qc.n, 0) AS question_count
		FROM course_version_stages cvs
		JOIN lessons l ON l.stage_version_id = cvs.stage_version_id AND l.type = 'quiz'
		JOIN stages s ON s.id = cvs.stage_id
		JOIN quiz_settings qs ON qs.lesson_id = l.id
		LEFT JOIN LATERAL (SELECT count(*)::int AS n FROM questions q WHERE q.lesson_id = l.id) qc ON true
		WHERE cvs.course_version_id = $1
		ORDER BY cvs.position, l.position`, courseVersionID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc quiz của khóa: %w", err)
	}
	out := make([]QuizLesson, len(rows))
	for i, r := range rows {
		c, err := r.content()
		if err != nil {
			return nil, err
		}
		out[i] = QuizLesson{
			LessonID: r.LessonID, Title: r.Title, StageID: r.StageID, StageName: r.StageName, Quiz: c,
			QuestionCount: r.QuestionCount,
		}
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
		return nil, fmt.Errorf("quizzes: đọc thành viên: %w", err)
	}
	out := make([]ActiveMember, len(rows))
	for i, r := range rows {
		out[i] = ActiveMember(r)
	}
	return out, nil
}

func (PGRepo) ClassAttempts(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]Attempt, error) {
	var rows []attemptRecord
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT `+attemptColumns+` FROM quiz_attempts
		WHERE class_member_id IN (SELECT id FROM class_members WHERE class_id = $1 AND status = 'active')
		ORDER BY class_member_id, lesson_id, attempt_no`, classID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc lượt của lớp: %w", err)
	}
	out := make([]Attempt, len(rows))
	for i, r := range rows {
		a, err := r.attempt()
		if err != nil {
			return nil, err
		}
		out[i] = a
	}
	return out, nil
}

// grantRecord là một dòng quiz_attempt_grants kèm tên học viên và người cấp.
type grantRecord struct {
	ID            uuid.UUID `db:"id"`
	MemberID      uuid.UUID `db:"class_member_id"`
	LessonID      uuid.UUID `db:"lesson_id"`
	StudentName   string    `db:"student_name"`
	ExtraAttempts int       `db:"extra_attempts"`
	Reason        string    `db:"reason"`
	GrantedByName string    `db:"granted_by_name"`
	GrantedAt     time.Time `db:"granted_at"`
}

// grantNames nối một tập dòng grant g (có class_member_id, granted_by) với tên học viên và người cấp.
const grantNames = `JOIN class_members cm ON cm.id = g.class_member_id JOIN users su ON su.id = cm.user_id
		JOIN users gu ON gu.id = g.granted_by`

const grantColumns = `g.id, g.class_member_id, g.lesson_id, su.full_name AS student_name, g.extra_attempts, g.reason,
		gu.full_name AS granted_by_name, g.granted_at`

func (PGRepo) ClassGrants(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]Grant, error) {
	var rows []grantRecord
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT `+grantColumns+` FROM quiz_attempt_grants g `+grantNames+`
		WHERE cm.class_id = $1 ORDER BY g.granted_at DESC, g.id DESC`, classID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc lượt cấp thêm của lớp: %w", err)
	}
	out := make([]Grant, len(rows))
	for i, r := range rows {
		out[i] = Grant(r)
	}
	return out, nil
}

func (PGRepo) InsertGrant(ctx context.Context, ex db.Executor, g NewGrant) (Grant, error) {
	var r grantRecord
	err := sqlx.GetContext(ctx, ex, &r, `WITH g AS (
			INSERT INTO quiz_attempt_grants (id, class_member_id, lesson_id, extra_attempts, reason, granted_by, granted_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *
		) SELECT `+grantColumns+` FROM g `+grantNames,
		g.ID, g.MemberID, g.LessonID, g.ExtraAttempts, g.Reason, g.GrantedBy, g.GrantedAt)
	if err != nil {
		return Grant{}, pgerr.Map(err)
	}
	return Grant(r), nil
}

func (PGRepo) QuestionTallies(ctx context.Context, ex db.Executor, classID, courseVersionID uuid.UUID) ([]QuestionTally, error) {
	var rows []struct {
		LessonID   uuid.UUID `db:"lesson_id"`
		QuestionID uuid.UUID `db:"question_id"`
		BodySource string    `db:"body_source"`
		BodyHTML   *string   `db:"body_html"`
		Correct    int       `db:"correct"`
	}
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT q.lesson_id, q.id AS question_id, q.body_source, q.body_html,
		COALESCE(s.correct, 0) AS correct
		FROM course_version_stages cvs
		JOIN lessons l ON l.stage_version_id = cvs.stage_version_id AND l.type = 'quiz'
		JOIN questions q ON q.lesson_id = l.id
		LEFT JOIN (
			SELECT ans.question_id, count(*) FILTER (WHERE ans.points_awarded > 0)::int AS correct
			FROM quiz_attempt_answers ans
			JOIN quiz_attempts a ON a.id = ans.attempt_id AND a.submitted_at IS NOT NULL
			JOIN class_members cm ON cm.id = a.class_member_id AND cm.class_id = $1 AND cm.status = 'active'
			GROUP BY ans.question_id
		) s ON s.question_id = q.id
		WHERE cvs.course_version_id = $2
		ORDER BY cvs.position, l.position, q.position`, classID, courseVersionID); err != nil {
		return nil, fmt.Errorf("quizzes: đọc thống kê câu hỏi: %w", err)
	}
	out := make([]QuestionTally, len(rows))
	for i, r := range rows {
		out[i] = QuestionTally(r)
	}
	return out, nil
}

func affectOne(err error) error {
	if errors.Is(err, db.ErrNoRowsAffected) {
		return ErrConcurrentChange
	}
	if err != nil {
		return pgerr.Map(err)
	}
	return nil
}
