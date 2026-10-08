// Package learning là phần học của học viên: lớp của tôi, lộ trình, mở học liệu và tự xác nhận hoàn thành; kèm
// ProgressReader cho báo cáo và trang lớp của giảng viên. Tiến độ tính khi đọc, không lưu cột phần trăm.
package learning

import (
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
)

// Lỗi của feature. Không phải thành viên, đã rời lớp, lớp không tồn tại và học liệu ngoài phiên bản khóa học của
// lớp đều là 404 để không lộ lớp hay học liệu có tồn tại.
var (
	ErrNotFound          = domain.ErrNotFound.WithMsg("Bạn không còn là thành viên đang học của lớp.")
	ErrLessonNotInCourse = domain.ErrNotFound.WithMsg("Học liệu không thuộc phiên bản khóa học của lớp.")
	ErrMemberNotFound    = domain.ErrNotFound.WithMsg("Không tìm thấy thành viên.")
	ErrNotOpened         = domain.ErrConflict.WithMsg("Mở học liệu trước khi tích hoàn thành.")
	ErrClassNotActive    = domain.ErrInvalidTransition.WithMsg("Lớp chưa bắt đầu hoặc đã kết thúc; không ghi nhận tiến độ.")
	// ErrClassNotStarted: lớp nháp chỉ cho xem lộ trình; nội dung (video, ảnh trong bài đọc) chỉ cấp khi lớp đã
	// kích hoạt, cùng quy tắc với quyền xem media.
	ErrClassNotStarted = domain.ErrInvalidTransition.WithMsg("Lớp chưa bắt đầu. Bạn sẽ vào học được khi lớp kích hoạt.")
)

// LessonState là trạng thái một học liệu của một học viên.
type LessonState string

// Các trạng thái học liệu.
const (
	StateNotOpened LessonState = "not_opened"
	StateOpened    LessonState = "opened"
	StateCompleted LessonState = "completed"
)

// Lý do lộ trình chỉ đọc.
const (
	ReadOnlyDraft = "draft"
	ReadOnlyEnded = "ended"
)

// LessonProgress là tiến độ của một thành viên lớp trên một học liệu, định danh bằng (memberID, lessonID).
// Có bản ghi nghĩa là đã mở; completedAt khác nil nghĩa là đã tự xác nhận hoàn thành.
type LessonProgress struct {
	memberID      uuid.UUID
	lessonID      uuid.UUID
	firstOpenedAt time.Time
	completedAt   *time.Time
}

// OpenLesson tạo tiến độ cho lần mở đầu tiên; repository ghi bằng INSERT … ON CONFLICT DO NOTHING nên các lần
// mở sau không đổi first_opened_at.
func OpenLesson(memberID, lessonID uuid.UUID, now time.Time) *LessonProgress {
	return &LessonProgress{memberID: memberID, lessonID: lessonID, firstOpenedAt: now}
}

// RestoreProgress dựng lại tiến độ từ dữ liệu đã lưu.
func RestoreProgress(memberID, lessonID uuid.UUID, firstOpenedAt time.Time, completedAt *time.Time) *LessonProgress {
	return &LessonProgress{memberID: memberID, lessonID: lessonID, firstOpenedAt: firstOpenedAt, completedAt: completedAt}
}

// SetCompleted tích (completed=true) hoặc bỏ tích; idempotent: tích lại khi đã tích giữ nguyên completedAt, bỏ
// tích khi chưa tích không đổi gì. changed cho biết có cần ghi xuống DB. completedAt không sớm hơn firstOpenedAt
// (ck_lesson_progress_order) kể cả khi đồng hồ lệch.
func (p *LessonProgress) SetCompleted(completed bool, now time.Time) (changed bool) {
	if completed {
		if p.completedAt != nil {
			return false
		}
		at := now
		if at.Before(p.firstOpenedAt) {
			at = p.firstOpenedAt
		}
		p.completedAt = &at
		return true
	}
	if p.completedAt == nil {
		return false
	}
	p.completedAt = nil
	return true
}

// State trả trạng thái; nil (chưa có bản ghi) là chưa mở.
func (p *LessonProgress) State() LessonState {
	switch {
	case p == nil:
		return StateNotOpened
	case p.completedAt != nil:
		return StateCompleted
	default:
		return StateOpened
	}
}

func (p *LessonProgress) MemberID() uuid.UUID      { return p.memberID }
func (p *LessonProgress) LessonID() uuid.UUID      { return p.lessonID }
func (p *LessonProgress) FirstOpenedAt() time.Time { return p.firstOpenedAt }
func (p *LessonProgress) CompletedAt() *time.Time  { return p.completedAt }

// LearningContext là điều kiện chung của mọi use case học: học viên phải là thành viên đang học của lớp; chỉ lớp
// đang chạy mới ghi nhận tiến độ.
type LearningContext struct {
	Member classes.MemberContext
}

// CanView: thành viên active ở lớp draft|active|ended → nil; không phải thành viên hoặc đã rời → ErrNotFound.
func (lc LearningContext) CanView() error {
	if !lc.Member.IsActiveMember() {
		return ErrNotFound
	}
	return nil
}

// CanOpen: như CanView, thêm lớp không còn nháp (lớp nháp chưa cấp nội dung học liệu).
func (lc LearningContext) CanOpen() error {
	if err := lc.CanView(); err != nil {
		return err
	}
	if lc.Member.ClassStatus == domain.ClassDraft {
		return ErrClassNotStarted
	}
	return nil
}

// CanRecord: như CanView, thêm lớp đang chạy; lớp nháp hoặc đã kết thúc → ErrClassNotActive.
func (lc LearningContext) CanRecord() error {
	if err := lc.CanView(); err != nil {
		return err
	}
	if lc.Member.ClassStatus != domain.ClassActive {
		return ErrClassNotActive
	}
	return nil
}

// ReadOnlyReason trả "" khi lớp đang chạy, "draft" hoặc "ended" khi lộ trình chỉ đọc.
func (lc LearningContext) ReadOnlyReason() string { return readOnlyReason(lc.Member.ClassStatus) }

func readOnlyReason(st domain.ClassStatus) string {
	switch st {
	case domain.ClassDraft:
		return ReadOnlyDraft
	case domain.ClassEnded:
		return ReadOnlyEnded
	default:
		return ""
	}
}
