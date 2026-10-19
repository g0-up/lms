package summary

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/platform/db"
)

// Repo đọc dữ liệu tổng kết. Các truy vấn hoạt động (tiến độ, bài nộp, lượt quiz, lượt cấp thêm, nhận xét) đọc cho
// lớp classID, memberID nil là mọi thành viên của lớp; mỗi bảng một truy vấn nên số truy vấn không tăng theo số học
// viên.
type Repo interface {
	// Class trả lớp; không có → classes.ErrClassNotFound.
	Class(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassInfo, error)
	// Stages là các chặng của phiên bản khóa học theo thứ tự.
	Stages(ctx context.Context, ex db.Executor, courseVersionID uuid.UUID) ([]StageRef, error)
	// Lessons là mọi học liệu của phiên bản khóa học theo chặng → bài học → học liệu, kèm cấu hình quiz.
	Lessons(ctx context.Context, ex db.Executor, courseVersionID uuid.UUID) ([]Lesson, error)
	// ActiveMembers là thành viên đang học của lớp, sắp theo tên như danh sách học viên của reports.
	ActiveMembers(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]Member, error)
	// Member là thành viên memberID của lớp classID (mọi trạng thái); khác lớp → classes.ErrMemberNotFound.
	Member(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (Member, error)
	// LockMember như Member và khóa dòng class_members (FOR UPDATE) cho đường ghi.
	LockMember(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (Member, error)
	// Activity nạp dữ liệu hoạt động của lớp (một truy vấn mỗi bảng).
	Activity(ctx context.Context, ex db.Executor, classID uuid.UUID, memberID *uuid.UUID) (Activity, error)
	// Reviews là nhận xét tổng kết chặng của thành viên trong lớp kèm tên người lưu gần nhất.
	Reviews(ctx context.Context, ex db.Executor, classID uuid.UUID, memberID *uuid.UUID) ([]Review, error)
	// Review là nhận xét của (thành viên, phiên bản chặng); chưa có → nil.
	Review(ctx context.Context, ex db.Executor, memberID, stageVersionID uuid.UUID) (*Review, error)
	// InsertReview ghi nhận xét đầu tiên.
	InsertReview(ctx context.Context, ex db.Executor, r Review) error
	// UpdateReview ghi lần sửa r khi dòng vẫn ở revision prevRevision; dòng đã đổi → ErrConcurrentEdit.
	UpdateReview(ctx context.Context, ex db.Executor, r Review, prevRevision int) error
}

// ProgressRow là tiến độ của một (thành viên, học liệu) đã mở.
type ProgressRow struct {
	MemberID      uuid.UUID
	LessonID      uuid.UUID
	FirstOpenedAt *time.Time
	CompletedAt   *time.Time
}

// SubmissionRow là bài nộp mới nhất của một (thành viên, homework) cùng tổng số lần nộp.
type SubmissionRow struct {
	MemberID uuid.UUID
	LessonID uuid.UUID
	Count    int
	Latest   Submission
}

// AttemptRow là một lượt quiz; Score nil khi lượt đang làm.
type AttemptRow struct {
	MemberID     uuid.UUID
	LessonID     uuid.UUID
	AttemptNo    int
	Score        *int
	Max          int
	ReviewStatus *ReviewStatus
}

// GrantRow là tổng lượt cấp thêm của một (thành viên, quiz).
type GrantRow struct {
	MemberID uuid.UUID
	LessonID uuid.UUID
	Extra    int
}

// Activity là dữ liệu hoạt động thô của lớp: tiến độ, bài nộp mới nhất, hạn nộp theo học liệu, lượt quiz theo
// attempt_no và lượt cấp thêm.
type Activity struct {
	Progress    []ProgressRow
	Submissions []SubmissionRow
	Deadlines   map[uuid.UUID]time.Time
	Attempts    []AttemptRow
	Grants      []GrantRow
}
