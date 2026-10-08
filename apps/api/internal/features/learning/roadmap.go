package learning

import (
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
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
}

// StageView là một chặng của phiên bản khóa học (thứ tự theo course_version_stages) cùng học liệu theo position.
type StageView struct {
	StageID        uuid.UUID
	StageVersionID uuid.UUID
	Code           string
	Name           string
	VersionNo      domain.VersionNo
	Position       int
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
// phần trăm từng chặng và toàn khóa qua domain.Percent, học liệu kế tiếp là học liệu đầu tiên (bắt buộc hay
// không) chưa hoàn thành theo thứ tự chặng → position; lớp nháp chưa vào học nên không có học liệu kế tiếp.
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
		for _, l := range st.Lessons {
			p := progress[l.ID]
			l.State, l.FirstOpenedAt, l.CompletedAt = p.State(), nil, nil
			if p != nil {
				opened := p.FirstOpenedAt()
				l.FirstOpenedAt, l.CompletedAt = &opened, p.CompletedAt()
				r.LastActivityAt = latest(r.LastActivityAt, l.FirstOpenedAt, l.CompletedAt)
			}
			if l.Required {
				out.RequiredTotal++
				if l.State == StateCompleted {
					out.RequiredDone++
				}
			}
			if r.NextLesson == nil && l.State != StateCompleted && cls.Status != domain.ClassDraft {
				r.NextLesson = &LessonRef{LessonID: l.ID, Title: l.Title, StageName: st.Name}
			}
			out.Lessons = append(out.Lessons, l)
		}
		out.Percent = domain.Percent(out.RequiredDone, out.RequiredTotal)
		r.RequiredDone += out.RequiredDone
		r.RequiredTotal += out.RequiredTotal
		r.Stages = append(r.Stages, out)
	}
	r.Percent = domain.Percent(r.RequiredDone, r.RequiredTotal)
	return r
}

// Locate tìm học liệu trong lộ trình cùng chặng chứa nó và học liệu liền trước/sau theo thứ tự toàn khóa.
func (r Roadmap) Locate(lessonID uuid.UUID) (lesson LessonView, stage StageView, prev, next *LessonRef, ok bool) {
	var last *LessonRef
	for _, st := range r.Stages {
		for _, l := range st.Lessons {
			ref := &LessonRef{LessonID: l.ID, Title: l.Title, StageName: st.Name}
			switch {
			case ok:
				return lesson, stage, prev, ref, true
			case l.ID == lessonID:
				lesson, stage, prev, ok = l, st, last, true
			}
			last = ref
		}
	}
	return lesson, stage, prev, nil, ok
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
