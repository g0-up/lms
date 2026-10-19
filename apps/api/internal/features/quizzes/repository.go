package quizzes

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/learning"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/db"
)

// ProgressWriter ghi tiến độ học liệu quiz qua learning: lần mở đầu tiên khi bắt đầu lượt và mốc hoàn thành khi nộp,
// bằng Executor của caller để commit cùng sự kiện quyết định nó.
type ProgressWriter interface {
	learning.CompletionWriter
	Open(ctx context.Context, ex db.Executor, p *learning.LessonProgress) (bool, error)
}

// AttemptRef là chủ của một lượt (đọc không khóa) để kiểm quyền và chọn thành viên cần khóa.
type AttemptRef struct {
	ID       uuid.UUID
	MemberID uuid.UUID
	ClassID  uuid.UUID
	UserID   uuid.UUID
	LessonID uuid.UUID
}

// LockedMember là trạng thái thành viên và lớp đọc lại sau khi khóa hàng class_members.
type LockedMember struct {
	ID          uuid.UUID
	Status      domain.MemberStatus
	ClassStatus domain.ClassStatus
}

// Answer là câu trả lời đã lưu của một câu; IsCorrect và Points chỉ có sau khi chấm.
type Answer struct {
	QuestionID uuid.UUID
	Selected   []uuid.UUID
	IsCorrect  *bool
	Points     *int
	SavedAt    time.Time
}

// Review là nhận xét hiện hành (dòng mới nhất) của giảng viên cho một lượt; Updated khi đã có nhận xét trước đó.
type Review struct {
	Comment      string
	ReviewerName string
	CreatedAt    time.Time
	Updated      bool
	Notes        map[uuid.UUID]string
}

// ClassInfo là lớp đọc cho use case của staff.
type ClassInfo struct {
	ID              uuid.UUID
	Code            string
	Name            string
	Status          domain.ClassStatus
	TeacherID       uuid.UUID
	CourseVersionID uuid.UUID
}

// AttemptContext là lượt đã nộp cùng học viên, lớp, học liệu và chặng; đọc không khóa để kiểm quyền staff.
type AttemptContext struct {
	AttemptID    uuid.UUID
	MemberID     uuid.UUID
	MemberStatus domain.MemberStatus
	UserID       uuid.UUID
	StudentName  string
	StudentEmail string
	Class        ClassInfo
	LessonID     uuid.UUID
	LessonTitle  string
	StageID      uuid.UUID
	StageName    string
}

// ReviewRecord là một lần nhận xét của giảng viên cho một lượt; bảng append-only nên dòng mới nhất là hiện hành.
// Notes theo thứ tự soạn của câu.
type ReviewRecord struct {
	ID           uuid.UUID
	AttemptID    uuid.UUID
	ReviewerID   uuid.UUID
	ReviewerName string
	Comment      string
	CreatedAt    time.Time
	Notes        []ReviewNote
}

// NewReview là một lần nhận xét cần ghi.
type NewReview struct {
	ID         uuid.UUID
	AttemptID  uuid.UUID
	ReviewerID uuid.UUID
	Comment    string
	Notes      []ReviewNote
	CreatedAt  time.Time
}

// QuizLesson là một quiz trong phiên bản khóa học của lớp kèm cấu hình và số câu.
type QuizLesson struct {
	LessonID      uuid.UUID
	Title         string
	StageID       uuid.UUID
	StageName     string
	Quiz          stages.QuizContent
	QuestionCount int
}

// ActiveMember là thành viên đang học của lớp.
type ActiveMember struct {
	MemberID uuid.UUID
	UserID   uuid.UUID
	Name     string
}

// ClassMember là thành viên của một lớp cùng tên học viên, đọc sau khi khóa hàng class_members.
type ClassMember struct {
	ID          uuid.UUID
	Name        string
	Status      domain.MemberStatus
	ClassStatus domain.ClassStatus
}

// NewGrant là một lần cấp thêm lượt cần ghi.
type NewGrant struct {
	ID            uuid.UUID
	MemberID      uuid.UUID
	LessonID      uuid.UUID
	ExtraAttempts int
	Reason        string
	GrantedBy     uuid.UUID
	GrantedAt     time.Time
}

// Grant là một lần cấp thêm lượt quiz bài tập cho một thành viên, kèm tên học viên và người cấp.
type Grant struct {
	ID            uuid.UUID
	MemberID      uuid.UUID
	LessonID      uuid.UUID
	StudentName   string
	ExtraAttempts int
	Reason        string
	GrantedByName string
	GrantedAt     time.Time
}

// QuestionTally là một câu hỏi của quiz trong phiên bản khóa học cùng số câu trả lời đúng (điểm > 0) trong các
// lượt đã nộp của thành viên đang học của lớp.
type QuestionTally struct {
	LessonID   uuid.UUID
	QuestionID uuid.UUID
	BodySource string
	BodyHTML   *string
	Correct    int
}

// Repo là truy cập dữ liệu của quizzes; mọi hàm nhận Executor để chạy trong transaction của Service.
type Repo interface {
	// QuizInCourseVersion đọc quiz (cấu hình, câu hỏi, phương án kèm đáp án) của học liệu thuộc phiên bản khóa học;
	// học liệu khác phiên bản hoặc không phải quiz → nil.
	QuizInCourseVersion(ctx context.Context, ex db.Executor, courseVersionID, lessonID uuid.UUID) (*stages.QuizContent, error)
	// Quiz đọc quiz của học liệu của một lượt đã có (câu hỏi của phiên bản đã phát hành không đổi).
	Quiz(ctx context.Context, ex db.Executor, lessonID uuid.UUID) (*stages.QuizContent, error)

	// LockMember khóa hàng class_members (FOR UPDATE) và trả trạng thái thành viên, lớp; luôn khóa trước lượt.
	LockMember(ctx context.Context, ex db.Executor, memberID uuid.UUID) (LockedMember, error)
	// LockClassMember như LockMember nhưng chỉ nhận thành viên của lớp classID, kèm tên học viên; thành viên không có
	// hoặc thuộc lớp khác → classes.ErrMemberNotFound.
	LockClassMember(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (ClassMember, error)
	// AttemptRef đọc chủ lượt không khóa; không có → ErrAttemptNotFound.
	AttemptRef(ctx context.Context, ex db.Executor, attemptID uuid.UUID) (AttemptRef, error)
	// Attempt đọc một lượt; AttemptForUpdate khóa lượt (sau khi đã khóa thành viên).
	Attempt(ctx context.Context, ex db.Executor, attemptID uuid.UUID) (*Attempt, error)
	AttemptForUpdate(ctx context.Context, ex db.Executor, attemptID uuid.UUID) (*Attempt, error)
	// Attempts là mọi lượt của thành viên ở một quiz theo attempt_no.
	Attempts(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) ([]Attempt, error)
	// Granted là tổng lượt cấp thêm của thành viên ở một quiz.
	Granted(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) (int, error)
	InsertAttempt(ctx context.Context, ex db.Executor, a *Attempt) error

	// Answers là câu trả lời đã lưu của một lượt.
	Answers(ctx context.Context, ex db.Executor, attemptID uuid.UUID) ([]Answer, error)
	// UpsertAnswer ghi phương án đã chọn của một câu (lượt đang mở, đã khóa).
	UpsertAnswer(ctx context.Context, ex db.Executor, attemptID, questionID uuid.UUID, selected []uuid.UUID, savedAt time.Time) error
	// GradeAnswers ghi is_correct, points_awarded cho mọi câu của lượt; câu chưa lưu được chèn dòng rỗng với saved_at = at.
	GradeAnswers(ctx context.Context, ex db.Executor, attemptID uuid.UUID, per []QuestionResult, at time.Time) error
	// Finish ghi kết quả nộp với điều kiện lượt còn mở; lượt đã nộp → ErrConcurrentChange.
	Finish(ctx context.Context, ex db.Executor, a *Attempt) error
	// SupersedePending chuyển lượt pending của (thành viên, quiz) sang superseded.
	SupersedePending(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) error
	// ExpiredOpen là id các lượt mở có deadline_at < before của thành viên đang học ở lớp đang chạy, sớm nhất trước.
	ExpiredOpen(ctx context.Context, ex db.Executor, before time.Time, limit int) ([]uuid.UUID, error)

	// LatestReview là nhận xét hiện hành của lượt; chưa có → nil.
	LatestReview(ctx context.Context, ex db.Executor, attemptID uuid.UUID) (*Review, error)

	// SubmittedAttemptContext đọc ngữ cảnh lượt đã nộp không khóa; lượt chưa nộp hoặc không có → ErrSubmittedNotFound.
	SubmittedAttemptContext(ctx context.Context, ex db.Executor, attemptID uuid.UUID) (AttemptContext, error)
	// Reviews là mọi lần nhận xét của các lượt của thành viên ở một quiz, mới nhất trước.
	Reviews(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) ([]ReviewRecord, error)
	// InsertReview ghi một lần nhận xét kèm ghi chú từng câu.
	InsertReview(ctx context.Context, ex db.Executor, r NewReview) error
	// ConfirmReview đặt review_status = confirmed với điều kiện trạng thái hiện tại vẫn là from (lượt đã khóa);
	// trạng thái đã đổi → ErrConcurrentChange.
	ConfirmReview(ctx context.Context, ex db.Executor, attemptID uuid.UUID, from *ReviewStatus) error

	// Class đọc lớp; không có → classes.ErrClassNotFound.
	Class(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassInfo, error)
	// QuizLessons là các quiz của phiên bản khóa học theo thứ tự chặng, học liệu.
	QuizLessons(ctx context.Context, ex db.Executor, courseVersionID uuid.UUID) ([]QuizLesson, error)
	// ActiveMembers là thành viên đang học của lớp theo tên.
	ActiveMembers(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]ActiveMember, error)
	// ClassAttempts là mọi lượt (cả lượt mở) của thành viên đang học của lớp, theo thành viên, học liệu, attempt_no.
	ClassAttempts(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]Attempt, error)
	// ClassGrants là mọi lần cấp thêm lượt của thành viên của lớp (cả thành viên đã rời), mới nhất trước.
	ClassGrants(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]Grant, error)
	// InsertGrant ghi một lần cấp thêm lượt (thành viên đã khóa) và đọc lại kèm tên học viên, người cấp.
	InsertGrant(ctx context.Context, ex db.Executor, g NewGrant) (Grant, error)
	// QuestionTallies là mọi câu hỏi của các quiz trong phiên bản khóa học theo thứ tự chặng, học liệu, câu, kèm số
	// câu trả lời đúng trong các lượt đã nộp (kể cả lượt bị thay thế) của thành viên đang học của lớp.
	QuestionTallies(ctx context.Context, ex db.Executor, classID, courseVersionID uuid.UUID) ([]QuestionTally, error)
}
