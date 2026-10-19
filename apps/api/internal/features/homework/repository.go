package homework

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/learning"
	"lms/api/internal/platform/db"
)

// LessonReader đọc học liệu trong phiên bản khóa học của lớp (learning.PGCourseStructureRepo hiện thực); nil, nil
// khi học liệu không thuộc phiên bản.
type LessonReader interface {
	LessonInCourseVersion(ctx context.Context, ex db.Executor, courseVersionID, lessonID uuid.UUID) (*learning.LessonRow, error)
}

// ClassInfo là phần lớp mà homework cần.
type ClassInfo struct {
	ID              uuid.UUID
	Code            string
	Name            string
	Status          domain.ClassStatus
	TeacherID       uuid.UUID
	CourseVersionID uuid.UUID
}

// Deadline là hạn nộp một bài tập của một lớp.
type Deadline struct {
	ClassID   uuid.UUID
	LessonID  uuid.UUID
	DueAt     time.Time
	SetBy     uuid.UUID
	UpdatedAt time.Time
}

// LockedMember là hàng class_members đã khóa cùng trạng thái lớp đọc sau khóa.
type LockedMember struct {
	ID          uuid.UUID
	Status      domain.MemberStatus
	ClassStatus domain.ClassStatus
}

// NewSubmission là một lần nộp sắp ghi.
type NewSubmission struct {
	ID          uuid.UUID
	MemberID    uuid.UUID
	LessonID    uuid.UUID
	AttemptNo   int
	RepoURL     *string
	CommitRef   *string
	SubmittedAt time.Time
	IsLate      bool
	MediaIDs    []uuid.UUID // theo thứ tự client gửi
}

// LatestSubmission là lần nộp mới nhất của (thành viên, bài tập).
type LatestSubmission struct {
	ID        uuid.UUID
	AttemptNo int
	Status    SubmissionStatus
}

// SubmissionContext là bài nộp cùng thành viên, học viên và lớp, đọc không khóa để kiểm quyền.
type SubmissionContext struct {
	ID           uuid.UUID
	LessonID     uuid.UUID
	AttemptNo    int
	Status       SubmissionStatus
	MemberID     uuid.UUID
	MemberStatus domain.MemberStatus
	UserID       uuid.UUID
	StudentName  string
	StudentEmail string
	Class        ClassInfo
}

// FileRef là một file trong bài nộp.
type FileRef struct {
	MediaID   uuid.UUID
	FileName  string
	SizeBytes int64
}

// ScoreRecord là điểm một tiêu chí của một lần chấm, kèm mô tả và điểm tối đa của tiêu chí.
type ScoreRecord struct {
	CriterionID uuid.UUID
	Description string
	Points      int
	MaxPoints   int
	Comment     string
}

// GradeRecord là một dòng submission_grades cùng điểm từng tiêu chí (theo thứ tự tiêu chí).
type GradeRecord struct {
	ID          uuid.UUID
	GradedByID  uuid.UUID
	GradedBy    string
	TotalPoints int
	MaxPoints   int
	Result      Result
	Comment     string
	GradedAt    time.Time
	Scores      []ScoreRecord
}

// SubmissionRecord là một lần nộp cùng file và mọi lần chấm (mới nhất trước).
type SubmissionRecord struct {
	ID          uuid.UUID
	AttemptNo   int
	RepoURL     *string
	CommitRef   *string
	SubmittedAt time.Time
	IsLate      bool
	Status      SubmissionStatus
	Files       []FileRef
	Grades      []GradeRecord
}

// Current là lần chấm hiện hành; nil khi chưa chấm.
func (s SubmissionRecord) Current() *GradeRecord {
	if len(s.Grades) == 0 {
		return nil
	}
	return &s.Grades[0]
}

// NewGradeRow là một lần chấm sắp ghi.
type NewGradeRow struct {
	ID           uuid.UUID
	SubmissionID uuid.UUID
	GradedBy     uuid.UUID
	Grade        Grade
	GradedAt     time.Time
}

// HomeworkLesson là một bài tập của phiên bản khóa học kèm hạn nộp của lớp (tổng quan homework của lớp).
type HomeworkLesson struct {
	LessonID    uuid.UUID
	Title       string
	StageID     uuid.UUID
	StageName   string
	PassPercent int
	MaxPoints   int
	DueAt       *time.Time
}

// ActiveMember là một thành viên đang học của lớp.
type ActiveMember struct {
	MemberID uuid.UUID
	UserID   uuid.UUID
	Name     string
}

// LatestRow là lần nộp mới nhất của một (thành viên, bài tập) trong lớp cùng điểm hiện hành.
type LatestRow struct {
	SubmissionID uuid.UUID
	MemberID     uuid.UUID
	LessonID     uuid.UUID
	AttemptNo    int
	SubmittedAt  time.Time
	IsLate       bool
	HasRepo      bool
	FileCount    int
	Status       SubmissionStatus
	TotalPoints  *int
	MaxPoints    *int
}

// Repo là truy cập dữ liệu của homework; mọi hàm chạy bằng Executor của caller.
type Repo interface {
	// Class đọc lớp; không có → classes.ErrClassNotFound. ClassForUpdate thêm khóa hàng lớp tới hết tx.
	Class(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassInfo, error)
	ClassForUpdate(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassInfo, error)

	// Deadline trả nil khi bài tập chưa có hạn nộp ở lớp.
	Deadline(ctx context.Context, ex db.Executor, classID, lessonID uuid.UUID) (*time.Time, error)
	UpsertDeadline(ctx context.Context, ex db.Executor, d Deadline) error
	DeleteDeadline(ctx context.Context, ex db.Executor, classID, lessonID uuid.UUID) error

	// LockMember khóa class_members FOR UPDATE (luôn khóa trước bài nộp) và đọc lại trạng thái thành viên, lớp.
	LockMember(ctx context.Context, ex db.Executor, memberID uuid.UUID) (LockedMember, error)
	// Latest trả nil khi thành viên chưa nộp bài tập này.
	Latest(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) (*LatestSubmission, error)
	// Supersede đổi bài đang chờ chấm thành superseded; bài không còn pending → ErrConcurrentChange.
	Supersede(ctx context.Context, ex db.Executor, submissionID uuid.UUID) error
	// UsableFiles đếm trong ids các document ready do userID tải lên và chưa thuộc bài nộp nào.
	UsableFiles(ctx context.Context, ex db.Executor, userID uuid.UUID, ids []uuid.UUID) (int, error)
	// InsertSubmission ghi bài nộp và submission_files.
	InsertSubmission(ctx context.Context, ex db.Executor, s NewSubmission) error

	// SubmissionContext đọc bài nộp không khóa; không có → ErrSubmissionNotFound.
	SubmissionContext(ctx context.Context, ex db.Executor, submissionID uuid.UUID) (SubmissionContext, error)
	// LockSubmission khóa bài nộp FOR UPDATE và trả trạng thái đọc sau khóa.
	LockSubmission(ctx context.Context, ex db.Executor, submissionID uuid.UUID) (SubmissionStatus, error)
	// InsertGrade ghi submission_grades và grade_criterion_scores.
	InsertGrade(ctx context.Context, ex db.Executor, g NewGradeRow) error
	// SetResult đặt status = kết quả chấm, chỉ khi bài chưa bị thay; không đúng một dòng → ErrConcurrentChange.
	SetResult(ctx context.Context, ex db.Executor, submissionID uuid.UUID, r Result) error

	// History trả mọi lần nộp của (thành viên, bài tập), mới nhất trước, kèm file và mọi lần chấm.
	History(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) ([]SubmissionRecord, error)

	// HomeworkLessons trả bài tập của phiên bản khóa học theo thứ tự chặng → position, kèm hạn nộp của lớp.
	HomeworkLessons(ctx context.Context, ex db.Executor, classID, courseVersionID uuid.UUID) ([]HomeworkLesson, error)
	// ActiveMembers trả thành viên đang học của lớp theo tên.
	ActiveMembers(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]ActiveMember, error)
	// LatestByClass trả lần nộp mới nhất của mỗi (thành viên đang học, bài tập) trong lớp.
	LatestByClass(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]LatestRow, error)
}
