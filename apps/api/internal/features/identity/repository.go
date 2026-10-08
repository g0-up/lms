package identity

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
)

// UserRepo là truy cập bảng users. Không tìm thấy → ErrUserNotFound.
type UserRepo interface {
	Create(ctx context.Context, ex db.Executor, u *User) error // email trùng → ErrEmailTaken
	Update(ctx context.Context, ex db.Executor, u *User) error // UPDATE … WHERE id (không có cột version)
	ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*User, error)
	// ByIDForUpdate khóa hàng (SELECT … FOR UPDATE) trong tx đăng nhập, đổi/đặt lại mật khẩu, vô hiệu hóa.
	ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*User, error)
	ByEmail(ctx context.Context, ex db.Executor, e domain.Email) (*User, error)
	ListByRole(ctx context.Context, ex db.Executor, role domain.Role, status *domain.UserStatus) ([]*User, error)
}

// SessionRepo là truy cập bảng sessions; token chỉ đi dưới dạng SHA-256.
type SessionRepo interface {
	Create(ctx context.Context, ex db.Executor, s *Session) error
	// ByTokenHash trả phiên cùng user (JOIN users, bỏ user disabled); không có → ErrSessionNotFound.
	ByTokenHash(ctx context.Context, ex db.Executor, hash []byte) (*Session, *User, error)
	// TouchLastSeen gia hạn trượt: last_seen_at = now, expires_at = now + ttl; chỉ ghi khi lần trước cách > 1 phút.
	TouchLastSeen(ctx context.Context, ex db.Executor, id uuid.UUID, now time.Time, ttl time.Duration) error
	DeleteByID(ctx context.Context, ex db.Executor, id uuid.UUID) error
	DeleteByTokenHash(ctx context.Context, ex db.Executor, hash []byte) error
	// DeleteAllForUser xóa mọi phiên của user, trừ exceptID nếu khác nil.
	DeleteAllForUser(ctx context.Context, ex db.Executor, userID uuid.UUID, exceptID *uuid.UUID) error
	DeleteExpired(ctx context.Context, ex db.Executor, now time.Time) (int64, error)
}

// LoginAttemptRepo là truy cập bảng login_attempts (khóa đăng nhập theo email).
type LoginAttemptRepo interface {
	// LockEmail lấy pg_advisory_xact_lock theo email chuẩn hóa; gọi đầu tx đăng nhập để đếm và ghi không bị đua.
	LockEmail(ctx context.Context, ex db.Executor, email domain.Email) error
	Record(ctx context.Context, ex db.Executor, a LoginAttempt) error
	// CountFailures đếm lần sai từ since và trả thời điểm lần sai cũ nhất trong khoảng (để tính Retry-After).
	CountFailures(ctx context.Context, ex db.Executor, email domain.Email, since time.Time) (int, *time.Time, error)
	DeleteOlderThan(ctx context.Context, ex db.Executor, cutoff time.Time) (int64, error)
}

// ResetTokenRepo là truy cập bảng password_reset_tokens.
type ResetTokenRepo interface {
	Create(ctx context.Context, ex db.Executor, t *PasswordResetToken) error
	// Consume đánh dấu token đã dùng nếu còn hạn và chưa dùng, trả user_id; không hợp lệ → ErrResetTokenInvalid.
	Consume(ctx context.Context, ex db.Executor, hash []byte, now time.Time) (uuid.UUID, error)
	// InvalidateForUser đánh dấu đã dùng mọi token còn mở của user.
	InvalidateForUser(ctx context.Context, ex db.Executor, userID uuid.UUID, now time.Time) error
}

// UserReader là API đọc user cho feature khác, chạy trong tx (hoặc Executor) của caller. Không tìm thấy → nil, nil.
type UserReader interface {
	SnapshotByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*UserSnapshot, error)
	SnapshotByEmail(ctx context.Context, ex db.Executor, email domain.Email) (*UserSnapshot, error)
	// SnapshotByIDs đọc nhiều user một lần (ví dụ tên người thao tác của nhật ký); id không tồn tại vắng mặt trong map.
	SnapshotByIDs(ctx context.Context, ex db.Executor, ids []uuid.UUID) (map[uuid.UUID]UserSnapshot, error)
	// SnapshotByIDForUpdate và SnapshotByEmailForUpdate khóa hàng user (FOR UPDATE) tới hết tx, để caller xoay mật
	// khẩu tạm không ghi đè một lần đổi mật khẩu hay vô hiệu hóa chạy song song.
	SnapshotByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*UserSnapshot, error)
	SnapshotByEmailForUpdate(ctx context.Context, ex db.Executor, email domain.Email) (*UserSnapshot, error)
	// ActiveTeacher trả giảng viên đang hoạt động và giữ khóa FOR SHARE (chặn vô hiệu hóa song song tới hết tx);
	// id không phải giảng viên active → nil, nil.
	ActiveTeacher(ctx context.Context, ex db.Executor, id uuid.UUID) (*UserSnapshot, error)
}

// UserProvisioner là API nội bộ cho feature classes, chạy trong tx của caller và không ghi audit.
type UserProvisioner interface {
	// ProvisionStudent trả học viên có email đó (created=false, mật khẩu tạm rỗng) hoặc tạo học viên invited với
	// mật khẩu tạm mới (created=true). Caller giữ khóa theo email để không đua.
	ProvisionStudent(ctx context.Context, ex db.Executor, email domain.Email, name string, now time.Time) (*User, TemporaryPassword, bool, error)
	// RotateTemporaryPassword cấp mật khẩu tạm mới cho học viên chưa đổi mật khẩu lần đầu (Reinvite) và lưu hash.
	RotateTemporaryPassword(ctx context.Context, ex db.Executor, u *User, now time.Time) (TemporaryPassword, error)
}
