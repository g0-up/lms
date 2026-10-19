package grading

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/platform/db"
)

// Teacher là giảng viên không bị khóa: có một dòng tồn đọng trên dashboard và nhận email tổng hợp chấm trễ.
type Teacher struct {
	ID    uuid.UUID
	Name  string
	Email string
}

// TeacherBacklog là tồn đọng của một giảng viên (FR-67). Pending và Overdue đếm trên hàng chờ của các lớp giảng viên
// phụ trách; AvgHours là số giờ trung bình từ lúc nộp tới lần xử lý đầu (chấm homework hoặc xác nhận quiz bài tập) do
// chính giảng viên đó làm, trên toàn bộ dữ liệu; nil khi giảng viên chưa xử lý mục nào.
type TeacherBacklog struct {
	TeacherID   uuid.UUID
	TeacherName string
	Pending     int
	Overdue     int
	AvgHours    *float64
}

// Backlog là tồn đọng của mọi giảng viên không bị khóa (theo tên) và tổng số mục quá hạn của toàn hệ thống, đọc bằng
// ex của caller để cùng snapshot với phần còn lại của dashboard. Dùng đúng truy vấn của hàng chờ nên số ở đây không
// lệch với GET /grading/queue.
func (s *Service) Backlog(ctx context.Context, ex db.Executor, now time.Time) ([]TeacherBacklog, int, error) {
	all, err := s.repo.Queue(ctx, ex, nil)
	if err != nil {
		return nil, 0, err
	}
	teachers, err := s.repo.Teachers(ctx, ex)
	if err != nil {
		return nil, 0, err
	}
	avg, err := s.repo.AvgHandledHours(ctx, ex)
	if err != nil {
		return nil, 0, err
	}
	rows, overdue := buildBacklog(stamp(all, now), teachers, avg)
	return rows, overdue, nil
}

// buildBacklog dựng một dòng cho mỗi giảng viên theo thứ tự của teachers và đếm mục quá hạn trên toàn hàng chờ, kể cả
// lớp của giảng viên đã bị khóa. all đã điền Overdue.
func buildBacklog(all []QueueItem, teachers []Teacher, avg map[uuid.UUID]float64) ([]TeacherBacklog, int) {
	type count struct{ pending, overdue int }
	byTeacher := map[uuid.UUID]count{}
	total := 0
	for _, it := range all {
		c := byTeacher[it.TeacherID]
		c.pending++
		if it.Overdue {
			c.overdue++
			total++
		}
		byTeacher[it.TeacherID] = c
	}
	out := make([]TeacherBacklog, len(teachers))
	for i, t := range teachers {
		c := byTeacher[t.ID]
		out[i] = TeacherBacklog{TeacherID: t.ID, TeacherName: t.Name, Pending: c.pending, Overdue: c.overdue}
		if h, ok := avg[t.ID]; ok {
			out[i].AvgHours = &h
		}
	}
	return out, total
}

// overdueByTeacher gom các mục quá hạn theo giảng viên phụ trách lớp, giữ thứ tự cũ nhất trước. all đã điền Overdue.
func overdueByTeacher(all []QueueItem) map[uuid.UUID][]QueueItem {
	out := map[uuid.UUID][]QueueItem{}
	for _, it := range all {
		if it.Overdue {
			out[it.TeacherID] = append(out[it.TeacherID], it)
		}
	}
	return out
}

// stamp điền thời gian chờ của mọi mục theo now.
func stamp(all []QueueItem, now time.Time) []QueueItem {
	for i := range all {
		all[i] = all[i].withClock(now)
	}
	return all
}
