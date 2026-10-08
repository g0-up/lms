package classes

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
)

// ClassRepo là truy cập bảng classes. Không tìm thấy → ErrClassNotFound.
type ClassRepo interface {
	Create(ctx context.Context, ex db.Executor, c *Class) error // mã trùng → ErrCodeTaken; end <= start → ErrInvalidDates
	// Update ghi lại lớp với điều kiện trạng thái trong DB vẫn là from (DB không có trigger nên đây là lớp chặn
	// cuối của state machine); lệch → ErrInvalidTransition.
	Update(ctx context.Context, ex db.Executor, c *Class, from domain.ClassStatus, now time.Time) error
	ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*Class, error)
	ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*Class, error)
	// Detail đọc lớp kèm giảng viên, phiên bản khóa học (số chặng, số học liệu), sĩ số và mốc kích hoạt/kết thúc.
	Detail(ctx context.Context, ex db.Executor, id uuid.UUID) (*ClassDetail, error)
	List(ctx context.Context, ex db.Executor, q ListQuery) ([]ClassListRow, error)
}

// MemberRepo là truy cập bảng class_members. Không tìm thấy → ErrMemberNotFound (trừ ByClassAndUser).
type MemberRepo interface {
	Create(ctx context.Context, ex db.Executor, m *ClassMember) error // trùng (class_id, user_id) → ErrAlreadyMember
	// Update ghi trạng thái với điều kiện trạng thái trong DB vẫn là from; lệch → lỗi chuyển trạng thái tương ứng.
	Update(ctx context.Context, ex db.Executor, m *ClassMember, from domain.MemberStatus) error
	ByID(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (*ClassMember, error)
	// ByIDForUpdate khóa hàng thành viên tới hết tx; gửi lại lời mời đếm giới hạn dưới khóa này.
	ByIDForUpdate(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (*ClassMember, error)
	ByClassAndUser(ctx context.Context, ex db.Executor, classID, userID uuid.UUID) (*ClassMember, error) // không có → nil, nil
	// ListRows trả thành viên kèm tài khoản và lời mời mới nhất (trạng thái gửi email); dropped chỉ khi includeDropped.
	ListRows(ctx context.Context, ex db.Executor, classID uuid.UUID, includeDropped bool) ([]MemberRow, error)
	RowByID(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (*MemberRow, error)
}

// InvitationRepo là truy cập bảng invitations (append-only).
type InvitationRepo interface {
	Create(ctx context.Context, ex db.Executor, inv *Invitation) error
	// CountResendSince đếm lời mời kind resend của (lớp, học viên) từ since.
	CountResendSince(ctx context.Context, ex db.Executor, classID, userID uuid.UUID, since time.Time) (int, error)
}

// ListQuery lọc danh sách lớp. StaleCutoff là mốc "không hoạt động" (now − StaleDays).
type ListQuery struct {
	Status      *domain.ClassStatus
	TeacherID   *uuid.UUID
	Q           string
	StaleCutoff time.Time
}

// ClassListRow là một dòng danh sách lớp kèm số liệu thành viên.
type ClassListRow struct {
	ID              uuid.UUID
	Code            string
	Name            string
	Status          domain.ClassStatus
	StartDate       time.Time
	EndDate         time.Time
	TeacherID       uuid.UUID
	TeacherName     string
	CourseName      string
	CourseVersionNo int
	MemberCount     int // thành viên active
	NotLoggedIn     int // thành viên active có tài khoản invited
	InactiveCount   int // thành viên active không hoạt động từ StaleCutoff (chỉ tính khi lớp active)
}

// ClassDetail là read model chi tiết lớp.
type ClassDetail struct {
	ID              uuid.UUID
	Code            string
	Name            string
	Status          domain.ClassStatus
	StartDate       time.Time
	EndDate         time.Time
	TeacherID       uuid.UUID
	TeacherName     string
	CourseVersionID uuid.UUID
	CourseID        uuid.UUID
	CourseName      string
	VersionNo       int
	StageCount      int
	LessonCount     int
	MemberCount     int
	ActivatedAt     *time.Time
	EndedAt         *time.Time
}

// MemberRow là một thành viên kèm tài khoản và lời mời mới nhất. Invite* nil khi thành viên chưa có lời mời.
type MemberRow struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	Email                 string
	FullName              string
	AccountStatus         domain.UserStatus
	MustChangePassword    bool
	TempPasswordExpiresAt *time.Time
	MemberStatus          domain.MemberStatus
	JoinedAt              time.Time
	DroppedAt             *time.Time
	LastLoginAt           *time.Time
	LastActiveAt          *time.Time
	InviteKind            *InvitationKind
	InvitedAt             *time.Time
	InviteStatus          *string // queued (gồm sending) | sent | failed
	InviteAttempts        *int
	InviteLastError       *string
}
