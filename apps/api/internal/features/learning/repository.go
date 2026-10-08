package learning

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/db"
)

// ProgressRepo lưu lesson_progress.
type ProgressRepo interface {
	// Open ghi lần mở đầu tiên bằng INSERT … ON CONFLICT DO NOTHING; inserted=false khi đã mở từ trước.
	Open(ctx context.Context, ex db.Executor, p *LessonProgress) (inserted bool, err error)
	// Get trả nil, nil khi học viên chưa mở học liệu.
	Get(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) (*LessonProgress, error)
	// GetForUpdate như Get và khóa dòng tới hết transaction.
	GetForUpdate(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) (*LessonProgress, error)
	// SetCompleted ghi completed_at của p, chỉ khi dòng đang ở trạng thái ngược lại (câu UPDATE có điều kiện).
	SetCompleted(ctx context.Context, ex db.Executor, p *LessonProgress, now time.Time) error
	// ByMember trả tiến độ của một thành viên theo lesson id.
	ByMember(ctx context.Context, ex db.Executor, memberID uuid.UUID) (map[uuid.UUID]*LessonProgress, error)
}

// LessonRow là một học liệu thuộc phiên bản khóa học của lớp, kèm nội dung để dựng trang học.
type LessonRow struct {
	ID              uuid.UUID
	Key             domain.LessonKey
	Title           string
	Position        int
	Required        bool
	DurationSeconds *int
	// Content là stages.VideoContent (MediaID) hoặc stages.MarkdownContent (HTML đã sanitize; Source không đọc).
	Content   stages.LessonContent
	StageID   uuid.UUID
	StageName string
}

// CourseStructureRepo đọc cấu trúc phiên bản khóa học (chặng → học liệu); chỉ đọc.
type CourseStructureRepo interface {
	// StagesOfCourseVersion trả chặng theo course_version_stages.position, học liệu theo position; chưa gắn tiến độ.
	StagesOfCourseVersion(ctx context.Context, ex db.Executor, courseVersionID uuid.UUID) ([]StageView, error)
	// LessonInCourseVersion trả nil, nil khi học liệu không thuộc phiên bản khóa học.
	LessonInCourseVersion(ctx context.Context, ex db.Executor, courseVersionID, lessonID uuid.UUID) (*LessonRow, error)
}

// MyClassRow là một lớp trên "Lớp của tôi" cùng tiến độ tính khi đọc.
type MyClassRow struct {
	Class         ClassSummary
	MemberID      uuid.UUID
	RequiredDone  int
	RequiredTotal int
	Percent       int
	NextLesson    *LessonRef
}

// MyClassesRepo đọc lớp của học viên.
type MyClassesRepo interface {
	// ListForStudent: lớp mà học viên là thành viên active (lớp draft cũng có, membership dropped bị loại);
	// NextLesson là học liệu đầu tiên chưa hoàn thành, nil với lớp nháp.
	ListForStudent(ctx context.Context, ex db.Executor, userID uuid.UUID) ([]MyClassRow, error)
	// ClassSummary trả thông tin lớp; lớp không tồn tại → ErrNotFound.
	ClassSummary(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassSummary, error)
}

// StageProgress là tiến độ của một thành viên trong một chặng.
type StageProgress struct {
	StageID       uuid.UUID
	Percent       int
	RequiredDone  int
	RequiredTotal int
}

// MemberProgress là tiến độ của một thành viên lớp (mọi trạng thái membership). Stages theo thứ tự chặng của
// phiên bản khóa học. LastActivityAt là lần mở hoặc tích gần nhất trên học liệu của khóa.
type MemberProgress struct {
	MemberID       uuid.UUID
	UserID         uuid.UUID
	MemberStatus   domain.MemberStatus
	Percent        int
	RequiredDone   int
	RequiredTotal  int
	Stages         []StageProgress
	LastActivityAt *time.Time
}

// ActivityCounts là số thành viên active chưa đăng nhập (tài khoản invited) và không hoạt động từ mốc cutoff
// (chỉ tính khi lớp đang chạy), cùng quy tắc với /teach/classes.
type ActivityCounts struct {
	NotLoggedIn int
	Inactive    int
}

// ProgressReader là tiến độ cho báo cáo và trang lớp của giảng viên, chạy trên Executor của caller. Phần trăm
// tính bằng round() của PostgreSQL, trùng domain.Percent.
type ProgressReader interface {
	// ClassProgress trả tiến độ mọi thành viên của lớp (kể cả đã rời), theo thứ tự tham gia.
	ClassProgress(ctx context.Context, ex db.Executor, classID uuid.UUID) ([]MemberProgress, error)
	// MemberLessonProgress là lộ trình chi tiết của một thành viên (mọi trạng thái membership); memberID phải thuộc
	// classID, không thì ErrMemberNotFound.
	MemberLessonProgress(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (Roadmap, error)
	// ClassAveragePercent là trung bình phần trăm của thành viên active theo lớp; lớp không có thành viên active
	// vắng trong map.
	ClassAveragePercent(ctx context.Context, ex db.Executor, classIDs []uuid.UUID) (map[uuid.UUID]int, error)
	// ClassActivityCounts đếm thành viên chưa đăng nhập và không hoạt động từ staleCutoff theo lớp; lớp nào cũng
	// có mặt trong map.
	ClassActivityCounts(ctx context.Context, ex db.Executor, classIDs []uuid.UUID, staleCutoff time.Time) (map[uuid.UUID]ActivityCounts, error)
}
