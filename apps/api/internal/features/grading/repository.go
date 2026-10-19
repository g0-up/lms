package grading

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/platform/db"
)

// Repo đọc hàng chờ chấm, tồn đọng và dữ liệu của việc hằng ngày; ghi duy nhất khóa khử trùng lặp của việc hằng ngày.
// teacherID nil là mọi lớp (admin), khác nil là chỉ lớp giảng viên đó phụ trách.
type Repo interface {
	// Teachers là giảng viên không bị khóa, theo tên.
	Teachers(ctx context.Context, ex db.Executor) ([]Teacher, error)
	// AvgHandledHours là số giờ trung bình từ lúc nộp tới lần xử lý đầu, theo người xử lý lần đầu đó: lần chấm đầu
	// của bài nộp và lần xác nhận đầu của lượt quiz bài tập. Người chưa xử lý mục nào không có trong map.
	AvgHandledHours(ctx context.Context, ex db.Executor) (map[uuid.UUID]float64, error)
	// ReminderCandidates là các cặp (thành viên, homework) cần nhắc: hạn nộp trong (after, until], lớp và thành viên
	// active, tài khoản học viên không bị khóa, chưa có bản nộp nào.
	ReminderCandidates(ctx context.Context, ex db.Executor, after, until time.Time) ([]Reminder, error)
	// ClaimRun chèn khóa của một lần chạy việc hằng ngày; false khi khóa đã có (đã chạy trước đó).
	ClaimRun(ctx context.Context, ex db.Executor, key string, at time.Time) (bool, error)
	// Queue là mọi mục chờ xử lý của thành viên active, sắp submitted_at rồi item_id; WaitedHours/Overdue chưa điền.
	Queue(ctx context.Context, ex db.Executor, teacherID *uuid.UUID) ([]QueueItem, error)
	// FilterClasses là các lớp không phải nháp theo mã, dùng cho ô lọc lớp.
	FilterClasses(ctx context.Context, ex db.Executor, teacherID *uuid.UUID) ([]Ref, error)
	// FilterLessons là học liệu homework và quiz bài tập của các lớp không phải nháp, theo lớp rồi vị trí trong khóa.
	FilterLessons(ctx context.Context, ex db.Executor, teacherID *uuid.UUID) ([]LessonOptionRow, error)
	// Recent là tối đa limit lần chấm hiện hành của bài nộp và lần xác nhận hiện hành của lượt quiz bài tập, mới nhất
	// trước.
	Recent(ctx context.Context, ex db.Executor, teacherID *uuid.UUID, limit int) ([]Recent, error)
}
