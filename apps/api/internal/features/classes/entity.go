// Package classes quản lý lớp học (gắn một phiên bản khóa học đã phát hành), thành viên lớp và lời mời học viên.
// Lớp đi một chiều nháp → đang chạy → đã kết thúc; mời học viên chạy trong một transaction xuyên identity (tạo tài
// khoản, mật khẩu tạm), classes (thành viên, lời mời) và mailer (outbox), giữ advisory lock theo email.
package classes

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/identity"
	"lms/api/internal/platform/apperr"
)

// dateLayout là định dạng ngày của API và cột DATE.
const dateLayout = "2006-01-02"

// Lỗi nghiệp vụ của lớp; message lấy theo prototype/app.js.
var (
	ErrClassFieldsRequired  = domain.ErrInvalid.WithMsg("Nhập mã và tên lớp.")
	ErrCodeTaken            = domain.ErrConflict.WithMsg("Mã lớp đã tồn tại.")
	ErrDatesRequired        = domain.ErrInvalid.WithMsg("Nhập ngày bắt đầu và kết thúc dự kiến.")
	ErrDateFormat           = domain.ErrInvalid.WithMsg("Ngày không hợp lệ, dùng định dạng YYYY-MM-DD.")
	ErrInvalidDates         = domain.ErrInvalid.WithMsg("Ngày kết thúc phải sau ngày bắt đầu.")
	ErrTeacherRequired      = domain.ErrInvalid.WithMsg("Chọn giảng viên phụ trách.")
	ErrTeacherInvalid       = domain.ErrInvalid.WithMsg("Giảng viên không hợp lệ hoặc đã bị vô hiệu hóa.")
	ErrVersionNotPublished  = domain.ErrInvalid.WithMsg("Chọn một phiên bản khóa học đã phát hành.")
	ErrChangeToUnpublished  = domain.ErrInvalid.WithMsg("Chỉ đổi sang phiên bản đã phát hành.")
	ErrClassNotDraft        = domain.ErrInvalidTransition.WithMsg("Chỉ đổi phiên bản khi lớp còn nháp.")
	ErrInvalidTransition    = domain.ErrInvalidTransition.WithMsg("Chỉ chuyển trạng thái một chiều: nháp → đang chạy → đã kết thúc.")
	ErrClassLocked          = domain.ErrInvalidTransition.WithMsg("Lớp đã kết thúc, không sửa được.")
	ErrClassNotFound        = domain.ErrNotFound.WithMsg("Không tìm thấy lớp.")
	ErrMemberNotFound       = domain.ErrNotFound.WithMsg("Không tìm thấy thành viên.")
	ErrNotOwnClass          = domain.ErrForbidden.WithMsg("Bạn chỉ xem được lớp mình phụ trách.")
	ErrMemberAlreadyDropped = domain.ErrInvalidTransition.WithMsg("Học viên đã rời lớp.")
	ErrMemberNotDropped     = domain.ErrInvalidTransition.WithMsg("Học viên đang ở trong lớp.")

	// Lỗi của lời mời (mã HTTP theo hợp đồng web, xem handler).
	ErrClassEnded       = domain.ErrInvalidTransition.WithMsg("Không mời được vào lớp đã kết thúc.")
	ErrInternalEmail    = domain.ErrForbidden.WithMsg("Email này thuộc tài khoản nội bộ, không mời làm học viên được.")
	ErrAccountDisabled  = apperr.New(http.StatusConflict, apperr.CodeAccountDisabled, "Tài khoản đã bị vô hiệu hóa. Kích hoạt lại trước khi mời.")
	ErrAlreadyMember    = domain.ErrConflict.WithMsg("Học viên đã có trong lớp.")
	ErrResendLimit      = apperr.New(http.StatusTooManyRequests, apperr.CodeRateLimited, "Đã gửi lại quá nhiều lần. Thử lại sau.")
	ErrNameRequired     = identity.ErrNameRequired
	ErrAlreadyActivated = identity.ErrAlreadyActivated
)

// ParseDate đọc ngày dạng YYYY-MM-DD (UTC, 00:00). Chuỗi rỗng → ErrDatesRequired.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, ErrDatesRequired
	}
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, ErrDateFormat
	}
	return d, nil
}

// FormatDate in ngày dạng YYYY-MM-DD.
func FormatDate(t time.Time) string { return t.Format(dateLayout) }

// DateRange là khoảng ngày dự kiến của lớp: End sau Start (trùng ck_classes_dates).
type DateRange struct {
	Start, End time.Time
}

// NewDateRange kiểm có đủ hai ngày và ngày kết thúc sau ngày bắt đầu; giờ trong ngày bị bỏ.
func NewDateRange(start, end time.Time) (DateRange, error) {
	if start.IsZero() || end.IsZero() {
		return DateRange{}, ErrDatesRequired
	}
	start, end = truncateDay(start), truncateDay(end)
	if !end.After(start) {
		return DateRange{}, ErrInvalidDates
	}
	return DateRange{Start: start, End: end}, nil
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// PublishedCourseVersion là phiên bản khóa học đã được kiểm là published (service dựng từ courses.VersionReader),
// nên Class không thể gắn phiên bản chưa kiểm.
type PublishedCourseVersion struct {
	ID         uuid.UUID
	CourseName string
	VersionNo  domain.VersionNo
}

// ActiveTeacher là giảng viên đã được kiểm role=teacher, status=active (service dựng từ identity.UserReader).
type ActiveTeacher struct {
	ID   uuid.UUID
	Name string
}

// Class là aggregate lớp học. activatedAt/endedAt không có cột riêng: đọc lại từ audit class.activated/class.ended.
type Class struct {
	id              uuid.UUID
	code            domain.Code
	name            string
	courseVersionID uuid.UUID
	status          domain.ClassStatus
	dates           DateRange
	teacherID       uuid.UUID
	createdBy       uuid.UUID
	createdAt       time.Time
	activatedAt     *time.Time
	endedAt         *time.Time
}

// NewClass tạo lớp nháp; mã đã được domain.ParseCode(ClassCode) kiểm, phiên bản và giảng viên là VO đã kiểm.
func NewClass(id uuid.UUID, code domain.Code, name string, cv PublishedCourseVersion, dates DateRange, teacher ActiveTeacher, by uuid.UUID, now time.Time) (*Class, error) {
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return nil, ErrClassFieldsRequired
	}
	if cv.ID == uuid.Nil {
		return nil, ErrVersionNotPublished
	}
	if teacher.ID == uuid.Nil {
		return nil, ErrTeacherRequired
	}
	return &Class{
		id: id, code: code, name: name, courseVersionID: cv.ID, status: domain.ClassDraft, dates: dates,
		teacherID: teacher.ID, createdBy: by, createdAt: now,
	}, nil
}

func (c *Class) ID() uuid.UUID              { return c.id }
func (c *Class) Code() domain.Code          { return c.code }
func (c *Class) Name() string               { return c.name }
func (c *Class) CourseVersionID() uuid.UUID { return c.courseVersionID }
func (c *Class) Status() domain.ClassStatus { return c.status }
func (c *Class) Dates() DateRange           { return c.dates }
func (c *Class) TeacherID() uuid.UUID       { return c.teacherID }
func (c *Class) CreatedBy() uuid.UUID       { return c.createdBy }
func (c *Class) CreatedAt() time.Time       { return c.createdAt }
func (c *Class) ActivatedAt() *time.Time    { return c.activatedAt }
func (c *Class) EndedAt() *time.Time        { return c.endedAt }
func (c *Class) IsActive() bool             { return c.status == domain.ClassActive }
func (c *Class) AcceptsInvitations() bool   { return c.status != domain.ClassEnded }

// editable chặn mọi thay đổi trên lớp đã kết thúc (ended chỉ xem).
func (c *Class) editable() error {
	if c.status == domain.ClassEnded {
		return ErrClassLocked
	}
	return nil
}

// Rename đổi tên lớp (mọi trạng thái trừ ended).
func (c *Class) Rename(name string) error {
	if err := c.editable(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrClassFieldsRequired
	}
	c.name = name
	return nil
}

// Reschedule đổi ngày dự kiến (mọi trạng thái trừ ended).
func (c *Class) Reschedule(d DateRange) error {
	if err := c.editable(); err != nil {
		return err
	}
	c.dates = d
	return nil
}

// AssignTeacher đổi giảng viên phụ trách (mọi trạng thái trừ ended).
func (c *Class) AssignTeacher(t ActiveTeacher) error {
	if err := c.editable(); err != nil {
		return err
	}
	if t.ID == uuid.Nil {
		return ErrTeacherRequired
	}
	c.teacherID = t.ID
	return nil
}

// ChangeCourseVersion đổi phiên bản khóa học; chỉ khi lớp còn nháp.
func (c *Class) ChangeCourseVersion(cv PublishedCourseVersion) error {
	if c.status != domain.ClassDraft {
		return ErrClassNotDraft
	}
	if cv.ID == uuid.Nil {
		return ErrChangeToUnpublished
	}
	c.courseVersionID = cv.ID
	return nil
}

// Activate chuyển nháp → đang chạy.
func (c *Class) Activate(now time.Time) error {
	if err := c.transition(domain.ClassActive); err != nil {
		return err
	}
	c.activatedAt = &now
	return nil
}

// End chuyển đang chạy → đã kết thúc.
func (c *Class) End(now time.Time) error {
	if err := c.transition(domain.ClassEnded); err != nil {
		return err
	}
	c.endedAt = &now
	return nil
}

func (c *Class) transition(to domain.ClassStatus) error {
	if !c.status.CanTransitionTo(to) {
		return ErrInvalidTransition
	}
	c.status = to
	return nil
}

// ClassMember là thành viên lớp. MVP không có use case ghi MemberCompleted.
type ClassMember struct {
	id        uuid.UUID
	classID   uuid.UUID
	userID    uuid.UUID
	status    domain.MemberStatus
	joinedAt  time.Time
	droppedAt *time.Time
}

// NewMember tạo thành viên đang học.
func NewMember(id, classID, userID uuid.UUID, now time.Time) *ClassMember {
	return &ClassMember{id: id, classID: classID, userID: userID, status: domain.MemberActive, joinedAt: now}
}

func (m *ClassMember) ID() uuid.UUID               { return m.id }
func (m *ClassMember) ClassID() uuid.UUID          { return m.classID }
func (m *ClassMember) UserID() uuid.UUID           { return m.userID }
func (m *ClassMember) Status() domain.MemberStatus { return m.status }
func (m *ClassMember) JoinedAt() time.Time         { return m.joinedAt }
func (m *ClassMember) DroppedAt() *time.Time       { return m.droppedAt }
func (m *ClassMember) IsActive() bool              { return m.status == domain.MemberActive }

// Drop gỡ thành viên khỏi lớp (active → dropped); tiến độ học vẫn gắn với thành viên này.
func (m *ClassMember) Drop(now time.Time) error {
	if m.status != domain.MemberActive {
		return ErrMemberAlreadyDropped
	}
	m.status, m.droppedAt = domain.MemberDropped, &now
	return nil
}

// Rejoin đưa thành viên đã rời quay lại (dropped → active); joinedAt giữ nguyên để tiến độ cũ vẫn liền mạch.
func (m *ClassMember) Rejoin() error {
	if m.status != domain.MemberDropped {
		return ErrMemberNotDropped
	}
	m.status, m.droppedAt = domain.MemberActive, nil
	return nil
}

// InvitationKind là loại lời mời, khớp ck_invitations_kind và tên template email.
type InvitationKind string

// Các loại lời mời.
const (
	InvitationInvite InvitationKind = "invite"
	InvitationAdded  InvitationKind = "added"
	InvitationResend InvitationKind = "resend"
)

// Invitation là một lần gửi lời mời (log append-only), trỏ tới hàng email_outbox.
type Invitation struct {
	ID            uuid.UUID
	ClassID       uuid.UUID
	UserID        uuid.UUID
	Kind          InvitationKind
	EmailOutboxID uuid.UUID
	InvitedBy     uuid.UUID
	CreatedAt     time.Time
}
