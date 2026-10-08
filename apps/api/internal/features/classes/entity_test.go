package classes

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

var testNow = time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)

func day(s string) time.Time {
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		panic(err)
	}
	return d
}

func testDates(t *testing.T) DateRange {
	t.Helper()
	d, err := NewDateRange(day("2026-11-01"), day("2027-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func newTestClass(t *testing.T, status domain.ClassStatus) *Class {
	t.Helper()
	c, err := NewClass(uuid.New(), "basic04", "Lập trình cơ bản 04", PublishedCourseVersion{ID: uuid.New()},
		testDates(t), ActiveTeacher{ID: uuid.New(), Name: "Lê Thu Hương"}, uuid.New(), testNow)
	if err != nil {
		t.Fatal(err)
	}
	c.status = status
	return c
}

func TestParseDate(t *testing.T) {
	if _, err := ParseDate("  "); !errors.Is(err, ErrDatesRequired) {
		t.Fatalf("rỗng: %v", err)
	}
	if _, err := ParseDate("01/11/2026"); !errors.Is(err, ErrDateFormat) {
		t.Fatalf("sai định dạng: %v", err)
	}
	d, err := ParseDate(" 2026-11-01 ")
	if err != nil || FormatDate(d) != "2026-11-01" {
		t.Fatalf("ParseDate = %v, %v", d, err)
	}
}

func TestNewDateRange(t *testing.T) {
	tests := []struct {
		name       string
		start, end time.Time
		want       error
	}{
		{"thiếu ngày bắt đầu", time.Time{}, day("2027-03-01"), ErrDatesRequired},
		{"thiếu ngày kết thúc", day("2026-11-01"), time.Time{}, ErrDatesRequired},
		{"kết thúc trước bắt đầu", day("2026-11-01"), day("2026-10-01"), ErrInvalidDates},
		{"kết thúc trùng bắt đầu", day("2026-11-01"), day("2026-11-01"), ErrInvalidDates},
		{"cùng ngày khác giờ vẫn trùng", day("2026-11-01"), day("2026-11-01").Add(5 * time.Hour), ErrInvalidDates},
		{"hợp lệ", day("2026-11-01"), day("2026-11-02"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewDateRange(tt.start, tt.end); !errors.Is(err, tt.want) {
				t.Fatalf("NewDateRange = %v, muốn %v", err, tt.want)
			}
		})
	}
}

func TestNewClass(t *testing.T) {
	cv, teacher, dates := PublishedCourseVersion{ID: uuid.New()}, ActiveTeacher{ID: uuid.New()}, testDates(t)
	if _, err := NewClass(uuid.New(), "basic04", "  ", cv, dates, teacher, uuid.New(), testNow); !errors.Is(err, ErrClassFieldsRequired) {
		t.Fatalf("thiếu tên: %v", err)
	}
	if _, err := NewClass(uuid.New(), "", "Lớp", cv, dates, teacher, uuid.New(), testNow); !errors.Is(err, ErrClassFieldsRequired) {
		t.Fatalf("thiếu mã: %v", err)
	}
	if _, err := NewClass(uuid.New(), "basic04", "Lớp", PublishedCourseVersion{}, dates, teacher, uuid.New(), testNow); !errors.Is(err, ErrVersionNotPublished) {
		t.Fatalf("thiếu phiên bản: %v", err)
	}
	if _, err := NewClass(uuid.New(), "basic04", "Lớp", cv, dates, ActiveTeacher{}, uuid.New(), testNow); !errors.Is(err, ErrTeacherRequired) {
		t.Fatalf("thiếu giảng viên: %v", err)
	}
	c, err := NewClass(uuid.New(), "basic04", " Lớp 04 ", cv, dates, teacher, uuid.New(), testNow)
	if err != nil || c.Status() != domain.ClassDraft || c.Name() != "Lớp 04" || !c.AcceptsInvitations() || c.IsActive() {
		t.Fatalf("NewClass = %+v, %v", c, err)
	}
}

func TestClassTransitionsOneWay(t *testing.T) {
	tests := []struct {
		name string
		from domain.ClassStatus
		run  func(*Class, time.Time) error
		want error
		to   domain.ClassStatus
	}{
		{"kích hoạt lớp nháp", domain.ClassDraft, (*Class).Activate, nil, domain.ClassActive},
		{"kết thúc lớp đang chạy", domain.ClassActive, (*Class).End, nil, domain.ClassEnded},
		{"kết thúc lớp nháp (nhảy cóc)", domain.ClassDraft, (*Class).End, ErrInvalidTransition, domain.ClassDraft},
		{"kích hoạt lại lớp đang chạy", domain.ClassActive, (*Class).Activate, ErrInvalidTransition, domain.ClassActive},
		{"kích hoạt lớp đã kết thúc", domain.ClassEnded, (*Class).Activate, ErrInvalidTransition, domain.ClassEnded},
		{"kết thúc lần hai", domain.ClassEnded, (*Class).End, ErrInvalidTransition, domain.ClassEnded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClass(t, tt.from)
			err := tt.run(c, testNow)
			if !errors.Is(err, tt.want) || c.Status() != tt.to {
				t.Fatalf("err = %v (muốn %v), status = %s (muốn %s)", err, tt.want, c.Status(), tt.to)
			}
			if err != nil && err.Error() != "Chỉ chuyển trạng thái một chiều: nháp → đang chạy → đã kết thúc." {
				t.Fatalf("message = %q", err.Error())
			}
		})
	}
	c := newTestClass(t, domain.ClassDraft)
	if err := c.Activate(testNow); err != nil || c.ActivatedAt() == nil || !c.IsActive() {
		t.Fatalf("Activate: %v, activatedAt = %v", err, c.ActivatedAt())
	}
	if err := c.End(testNow); err != nil || c.EndedAt() == nil || c.AcceptsInvitations() {
		t.Fatalf("End: %v, endedAt = %v", err, c.EndedAt())
	}
}

func TestChangeCourseVersion(t *testing.T) {
	next := PublishedCourseVersion{ID: uuid.New()}
	for _, st := range []domain.ClassStatus{domain.ClassActive, domain.ClassEnded} {
		if err := newTestClass(t, st).ChangeCourseVersion(next); !errors.Is(err, ErrClassNotDraft) {
			t.Fatalf("%s: %v", st, err)
		}
	}
	c := newTestClass(t, domain.ClassDraft)
	if err := c.ChangeCourseVersion(PublishedCourseVersion{}); !errors.Is(err, ErrChangeToUnpublished) {
		t.Fatalf("bản chưa kiểm: %v", err)
	}
	if err := c.ChangeCourseVersion(next); err != nil || c.CourseVersionID() != next.ID {
		t.Fatalf("đổi phiên bản: %v", err)
	}
}

func TestEndedClassIsReadOnly(t *testing.T) {
	c := newTestClass(t, domain.ClassEnded)
	if err := c.Rename("Tên mới"); !errors.Is(err, ErrClassLocked) {
		t.Fatalf("Rename: %v", err)
	}
	if err := c.Reschedule(testDates(t)); !errors.Is(err, ErrClassLocked) {
		t.Fatalf("Reschedule: %v", err)
	}
	if err := c.AssignTeacher(ActiveTeacher{ID: uuid.New()}); !errors.Is(err, ErrClassLocked) {
		t.Fatalf("AssignTeacher: %v", err)
	}

	active := newTestClass(t, domain.ClassActive)
	teacher := ActiveTeacher{ID: uuid.New()}
	if err := active.Rename(" Tên mới "); err != nil || active.Name() != "Tên mới" {
		t.Fatalf("Rename lớp đang chạy: %v", err)
	}
	if err := active.Rename(" "); !errors.Is(err, ErrClassFieldsRequired) {
		t.Fatalf("Rename rỗng: %v", err)
	}
	if err := active.AssignTeacher(teacher); err != nil || active.TeacherID() != teacher.ID {
		t.Fatalf("AssignTeacher lớp đang chạy: %v", err)
	}
	if err := active.AssignTeacher(ActiveTeacher{}); !errors.Is(err, ErrTeacherRequired) {
		t.Fatalf("AssignTeacher rỗng: %v", err)
	}
}

func TestMemberDropRejoin(t *testing.T) {
	m := NewMember(uuid.New(), uuid.New(), uuid.New(), testNow)
	joined := m.JoinedAt()
	if err := m.Rejoin(); !errors.Is(err, ErrMemberNotDropped) {
		t.Fatalf("Rejoin khi đang học: %v", err)
	}
	later := testNow.Add(time.Hour)
	if err := m.Drop(later); err != nil || m.Status() != domain.MemberDropped || m.DroppedAt() == nil || !m.DroppedAt().Equal(later) {
		t.Fatalf("Drop: %v, %+v", err, m)
	}
	if err := m.Drop(later); !errors.Is(err, ErrMemberAlreadyDropped) {
		t.Fatalf("Drop lần hai: %v", err)
	}
	if err := m.Rejoin(); err != nil || !m.IsActive() || m.DroppedAt() != nil || !m.JoinedAt().Equal(joined) {
		t.Fatalf("Rejoin: %v, %+v", err, m)
	}
}
