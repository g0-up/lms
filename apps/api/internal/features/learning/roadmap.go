package learning

import (
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
	"lms/api/internal/features/summary"
)

// ClassSummary là thông tin lớp hiển thị trên thẻ lớp và đầu lộ trình.
type ClassSummary struct {
	ID              uuid.UUID
	Code            string
	Name            string
	Status          domain.ClassStatus
	StartDate       time.Time
	EndDate         time.Time
	TeacherName     string
	CourseName      string
	CourseVersionNo int
}

// LessonRef trỏ tới một học liệu (học liệu kế tiếp, trước/sau); StageName rỗng khi không cần.
type LessonRef struct {
	LessonID  uuid.UUID
	Title     string
	StageName string
}

// LessonView là một học liệu trong lộ trình kèm trạng thái của học viên.
type LessonView struct {
	ID              uuid.UUID
	Key             domain.LessonKey
	Title           string
	Type            stages.LessonType
	Position        int
	Required        bool
	State           LessonState
	FirstOpenedAt   *time.Time
	CompletedAt     *time.Time
	DurationSeconds *int
	UnitID          uuid.UUID
	Status          *LessonStatus
}

// LessonStatus là nhãn trạng thái của quiz và homework trên lộ trình (summary tính từ lượt làm, bản nộp, hạn nộp);
// video và markdown không có, vẫn hiện dấu tích như cũ.
type LessonStatus struct {
	Label string
	Tone  string
}

// UnitView là một bài học trong chặng cùng học liệu của nó. Complete khi đã xong mọi học liệu bắt buộc (và có ít nhất
// một); Optional khi bài học không có học liệu bắt buộc nào ("Không bắt buộc").
type UnitView struct {
	ID            uuid.UUID
	Key           domain.LessonKey
	Title         string
	Position      int
	Lessons       []LessonView
	RequiredDone  int
	RequiredTotal int
	Complete      bool
	Optional      bool
}

// StageView là một chặng của phiên bản khóa học (thứ tự theo course_version_stages) cùng bài học theo position.
// Lessons là học liệu phẳng theo position, cùng thứ tự với học liệu đi lần lượt qua Units.
type StageView struct {
	StageID        uuid.UUID
	StageVersionID uuid.UUID
	Code           string
	Name           string
	VersionNo      domain.VersionNo
	Position       int
	Units          []UnitView
	Lessons        []LessonView
	Percent        int
	RequiredDone   int
	RequiredTotal  int
}

// Roadmap là lộ trình của một học viên trong một lớp. Phần trăm chỉ tính học liệu bắt buộc.
type Roadmap struct {
	Class          ClassSummary
	Stages         []StageView
	Percent        int
	RequiredDone   int
	RequiredTotal  int
	NextLesson     *LessonRef
	LastActivityAt *time.Time
	ReadOnly       bool
	ReadOnlyReason string
	SelfReported   bool
}

// BuildRoadmap gắn tiến độ vào cấu trúc khóa học (hàm thuần, không đổi stages đầu vào): trạng thái từng học liệu,
// tiến độ bắt buộc từng bài học, phần trăm từng chặng và toàn khóa qua domain.Percent, học liệu kế tiếp là học liệu
// đầu tiên (bắt buộc hay không) chưa hoàn thành theo thứ tự chặng → position; lớp nháp chưa vào học nên không có học
// liệu kế tiếp. Học liệu của bài học dựng lại từ Lessons phẳng để mang trạng thái mới.
func BuildRoadmap(cls ClassSummary, structure []StageView, progress map[uuid.UUID]*LessonProgress) Roadmap {
	reason := readOnlyReason(cls.Status)
	r := Roadmap{
		Class: cls, Stages: make([]StageView, 0, len(structure)),
		ReadOnly: reason != "", ReadOnlyReason: reason, SelfReported: true,
	}
	for _, st := range structure {
		out := st
		out.Lessons = make([]LessonView, 0, len(st.Lessons))
		out.RequiredDone, out.RequiredTotal = 0, 0
		out.Units = make([]UnitView, 0, len(st.Units))
		unitAt := make(map[uuid.UUID]int, len(st.Units))
		for i, u := range st.Units {
			u.Lessons, u.RequiredDone, u.RequiredTotal = make([]LessonView, 0, len(u.Lessons)), 0, 0
			unitAt[u.ID] = i
			out.Units = append(out.Units, u)
		}
		for _, l := range st.Lessons {
			p := progress[l.ID]
			l.State, l.FirstOpenedAt, l.CompletedAt = p.State(), nil, nil
			if p != nil {
				opened := p.FirstOpenedAt()
				l.FirstOpenedAt, l.CompletedAt = &opened, p.CompletedAt()
				r.LastActivityAt = latest(r.LastActivityAt, l.FirstOpenedAt, l.CompletedAt)
			}
			done := l.State == StateCompleted
			if l.Required {
				out.RequiredTotal++
				if done {
					out.RequiredDone++
				}
			}
			if i, ok := unitAt[l.UnitID]; ok {
				u := &out.Units[i]
				u.Lessons = append(u.Lessons, l)
				if l.Required {
					u.RequiredTotal++
					if done {
						u.RequiredDone++
					}
				}
			}
			if r.NextLesson == nil && l.State != StateCompleted && cls.Status != domain.ClassDraft {
				r.NextLesson = &LessonRef{LessonID: l.ID, Title: l.Title, StageName: st.Name}
			}
			out.Lessons = append(out.Lessons, l)
		}
		for i := range out.Units {
			u := &out.Units[i]
			u.Optional = u.RequiredTotal == 0
			u.Complete = !u.Optional && u.RequiredDone == u.RequiredTotal
		}
		out.Percent = domain.Percent(out.RequiredDone, out.RequiredTotal)
		r.RequiredDone += out.RequiredDone
		r.RequiredTotal += out.RequiredTotal
		r.Stages = append(r.Stages, out)
	}
	r.Percent = domain.Percent(r.RequiredDone, r.RequiredTotal)
	return r
}

// WithStatuses gắn nhãn trạng thái summary cho học liệu quiz và homework của lộ trình (cả danh sách phẳng của chặng
// lẫn danh sách theo bài học); không đổi phần trăm, vốn chỉ tính từ completed_at.
func (r Roadmap) WithStatuses(statuses map[uuid.UUID]summary.ItemStatus) Roadmap {
	apply := func(ls []LessonView) []LessonView {
		out := make([]LessonView, len(ls))
		for i, l := range ls {
			if st, ok := statuses[l.ID]; ok && (l.Type == stages.LessonQuiz || l.Type == stages.LessonHomework) {
				l.Status = &LessonStatus{Label: st.Label, Tone: string(st.Tone)}
			}
			out[i] = l
		}
		return out
	}
	out := r
	out.Stages = make([]StageView, len(r.Stages))
	for i, st := range r.Stages {
		st.Lessons = apply(st.Lessons)
		units := make([]UnitView, len(st.Units))
		for j, u := range st.Units {
			u.Lessons = apply(u.Lessons)
			units[j] = u
		}
		st.Units = units
		out.Stages[i] = st
	}
	return out
}

// Location là vị trí một học liệu trong lộ trình: chặng, bài học chứa nó và học liệu liền trước/sau theo thứ tự toàn
// khóa. CrossesUnit khi học liệu tiếp thuộc bài học khác (kể cả bài học đầu của chặng sau); NextUnitTitle là tên bài
// học đó cho nút "Sang bài học tiếp".
type Location struct {
	Lesson        LessonView
	Stage         StageView
	Unit          UnitView
	Prev, Next    *LessonRef
	CrossesUnit   bool
	NextUnitTitle string
}

// Locate tìm học liệu trong lộ trình; ok=false khi học liệu không thuộc lộ trình.
func (r Roadmap) Locate(lessonID uuid.UUID) (loc Location, ok bool) {
	var last *LessonRef
	for _, st := range r.Stages {
		for _, u := range st.Units {
			for _, l := range u.Lessons {
				ref := &LessonRef{LessonID: l.ID, Title: l.Title, StageName: st.Name}
				switch {
				case ok:
					loc.Next, loc.CrossesUnit = ref, u.ID != loc.Unit.ID
					if loc.CrossesUnit {
						loc.NextUnitTitle = u.Title
					}
					return loc, true
				case l.ID == lessonID:
					loc, ok = Location{Lesson: l, Stage: st, Unit: u, Prev: last}, true
				}
				last = ref
			}
		}
	}
	return loc, ok
}

func latest(cur *time.Time, ts ...*time.Time) *time.Time {
	for _, t := range ts {
		if t != nil && (cur == nil || t.After(*cur)) {
			v := *t
			cur = &v
		}
	}
	return cur
}
