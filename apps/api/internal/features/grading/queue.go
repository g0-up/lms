// Package grading là read model hàng chờ chấm: bài homework chờ chấm và lượt quiz bài tập chờ xác nhận của các lớp
// actor phụ trách, cũ nhất trước. Package chỉ đọc bảng bằng SQL, không import homework hay quizzes; hai feature đó
// gọi ngược vào đây qua NextFinder để lấy mục kế tiếp sau khi chấm hoặc xác nhận.
package grading

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
)

// GradingSLA là thời gian chờ tối đa của một mục trước khi bị coi là quá hạn xử lý (FR-65). Báo cáo tồn đọng và
// email nhắc dùng lại hằng này.
const GradingSLA = 72 * time.Hour

// Giới hạn của danh sách đã chấm gần đây.
const (
	DefaultRecentLimit = 6
	MaxRecentLimit     = 50
)

// ErrInvalidFilter là lỗi của mọi tham số lọc sai; hợp đồng API ghi 422 như bộ lọc của báo cáo.
var ErrInvalidFilter = apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidationFailed, "Tham số lọc không hợp lệ.")

// ErrStaffOnly: chỉ admin và giảng viên có hàng chờ chấm.
var ErrStaffOnly = domain.ErrForbidden.WithMsg("Chỉ Admin và giảng viên xem được hàng chờ chấm.")

// Kind là loại mục trong hàng chờ.
type Kind string

// Các loại mục.
const (
	KindHomework Kind = "homework" // bài nộp homework chờ chấm
	KindQuiz     Kind = "quiz"     // lượt quiz bài tập chờ xác nhận
)

// ParseKind đọc tham số kind; rỗng là mọi loại.
func ParseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case "", KindHomework, KindQuiz:
		return k, nil
	default:
		return "", ErrInvalidFilter
	}
}

// Actor là người xem hàng chờ.
type Actor struct {
	ID   uuid.UUID
	Role domain.Role
}

// scope trả teacher_id cần lọc: nil với admin (mọi lớp), id của chính giảng viên, vai trò khác ErrStaffOnly.
func (a Actor) scope() (*uuid.UUID, error) {
	switch a.Role {
	case domain.RoleAdmin:
		return nil, nil
	case domain.RoleTeacher:
		id := a.ID
		return &id, nil
	default:
		return nil, ErrStaffOnly
	}
}

// QueueKey định danh một mục: loại và id bài nộp hoặc id lượt quiz.
type QueueKey struct {
	Kind   Kind
	ItemID uuid.UUID
}

// Ref là cặp id và tên hiển thị dùng trong dòng hàng chờ.
type Ref struct {
	ID   uuid.UUID
	Code string
	Name string
}

// QueueItem là một mục chờ xử lý. Score/MaxScore chỉ có ở quiz; IsLate chỉ có nghĩa ở homework, AutoSubmitted chỉ ở
// quiz. WaitedHours và Overdue tính theo thời điểm đọc.
type QueueItem struct {
	Key           QueueKey
	SubmittedAt   time.Time
	No            int
	IsLate        bool
	AutoSubmitted bool
	Score         *int
	MaxScore      *int
	TeacherID     uuid.UUID
	Student       Ref // ID là users.id, Name là họ tên
	MemberID      uuid.UUID
	Class         Ref
	Lesson        Ref // Name là tiêu đề học liệu
	Stage         Ref // Code và Name của chặng
	WaitedHours   int
	Overdue       bool
}

// withClock điền WaitedHours (làm tròn xuống, không âm) và Overdue: chờ quá đúng GradingSLA mới là quá hạn.
func (q QueueItem) withClock(now time.Time) QueueItem {
	waited := max(now.Sub(q.SubmittedAt), 0)
	q.WaitedHours = int(waited / time.Hour)
	q.Overdue = waited > GradingSLA
	return q
}

// ScorePercent là % tổng của lượt quiz; homework trả nil.
func (q QueueItem) ScorePercent() *int {
	if q.Score == nil || q.MaxScore == nil {
		return nil
	}
	p := domain.Percent(*q.Score, *q.MaxScore)
	return &p
}

// Filter là bộ lọc của hàng chờ; giá trị rỗng là không lọc.
type Filter struct {
	Kind        Kind
	ClassID     *uuid.UUID
	LessonID    *uuid.UUID
	OverdueOnly bool
}

// Match: mục khớp mọi điều kiện của bộ lọc.
func (f Filter) Match(q QueueItem) bool {
	return (f.Kind == "" || q.Key.Kind == f.Kind) &&
		(f.ClassID == nil || q.Class.ID == *f.ClassID) &&
		(f.LessonID == nil || q.Lesson.ID == *f.LessonID) &&
		(!f.OverdueOnly || q.Overdue)
}

// Queue là kết quả của trang hàng chờ. Total, QuizPending và OverdueCount đếm trên toàn bộ hàng chờ của actor (không
// theo bộ lọc), Items đã lọc.
type Queue struct {
	Items        []QueueItem
	Total        int
	QuizPending  int
	OverdueCount int
	Classes      []Ref
	Lessons      []LessonOption
}

// LessonOption là một học liệu có thể vào hàng chờ (homework hoặc quiz bài tập), dùng cho ô lọc.
type LessonOption struct {
	ID    uuid.UUID
	Title string
	Kind  Kind
}

// LessonOptionRow là một học liệu có thể vào hàng chờ của một lớp, theo thứ tự lớp rồi vị trí trong khóa.
type LessonOptionRow struct {
	ClassID uuid.UUID
	LessonOption
}

// buildQueue lọc và đếm all (đã sắp cũ nhất trước). Ô lọc học liệu thu hẹp theo lớp và loại đang chọn, mỗi học liệu
// một lần dù nhiều lớp dùng chung phiên bản khóa.
func buildQueue(all []QueueItem, f Filter, classes []Ref, lessons []LessonOptionRow) Queue {
	q := Queue{Items: []QueueItem{}, Total: len(all), Classes: classes, Lessons: []LessonOption{}}
	for _, it := range all {
		if it.Key.Kind == KindQuiz {
			q.QuizPending++
		}
		if it.Overdue {
			q.OverdueCount++
		}
		if f.Match(it) {
			q.Items = append(q.Items, it)
		}
	}
	seen := map[uuid.UUID]bool{}
	for _, l := range lessons {
		if seen[l.ID] || (f.ClassID != nil && l.ClassID != *f.ClassID) || (f.Kind != "" && l.Kind != f.Kind) {
			continue
		}
		seen[l.ID] = true
		q.Lessons = append(q.Lessons, l.LessonOption)
	}
	return q
}

// next trả mục cũ nhất của all khác after; hết mục trả nil.
func next(all []QueueItem, after QueueKey) *QueueItem {
	for i := range all {
		if all[i].Key != after {
			it := all[i]
			return &it
		}
	}
	return nil
}

// Recent là một lần chấm homework hoặc một lần xác nhận quiz bài tập gần đây. Points/MaxPoints là điểm homework hoặc
// điểm lượt quiz; Passed là Đạt theo lần chấm hoặc theo ngưỡng của quiz.
type Recent struct {
	Key       QueueKey
	At        time.Time
	By        Ref
	Student   Ref
	Class     Ref
	Lesson    Ref
	No        int
	Points    int
	MaxPoints int
	Passed    bool
}
