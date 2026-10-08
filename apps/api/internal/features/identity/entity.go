// Package identity là feature tài khoản: đăng nhập bằng session cookie lưu Postgres, đổi mật khẩu lần đầu,
// quên/đặt lại mật khẩu, khóa đăng nhập theo email và vô hiệu hóa tài khoản. Feature khác chỉ dùng middleware
// (SessionAuth, MustChangePassword, RequireRole, CurrentUser) và interface UserProvisioner.
package identity

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
)

// maxNameRunes khớp ck_users_full_name.
const maxNameRunes = 120

// Lỗi của feature. Lỗi có Kind phù hợp là domain.Error để apperr.FromDomain ánh xạ (feature khác như classes nhận
// được đúng mã); lỗi đăng nhập/mật khẩu cần mã riêng (401, 403 ACCOUNT_DISABLED, 422) nên là *apperr.Error.
// Thông điệp chép nguyên văn spec/prototype.
var (
	ErrInvalidCredentials  = apperr.New(http.StatusUnauthorized, apperr.CodeUnauthenticated, "Email hoặc mật khẩu không đúng.")
	ErrTempPasswordExpired = apperr.New(http.StatusUnauthorized, apperr.CodeTempPasswordExpired, "Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên.")
	ErrAccountDisabled     = apperr.New(http.StatusForbidden, apperr.CodeAccountDisabled, "Tài khoản đã bị vô hiệu hóa. Vui lòng liên hệ quản trị viên.")
	ErrPasswordSameAsTemp  = apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidationFailed, "Mật khẩu mới phải khác mật khẩu tạm.")
	ErrPasswordMismatch    = apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidationFailed, "Hai mật khẩu không khớp.")
	ErrCurrentPassword     = apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidationFailed, "Nhập mật khẩu hiện tại.")
	ErrInvalidEmail        = apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidationFailed, "Email không hợp lệ.")
	ErrResetTokenInvalid   = apperr.New(http.StatusBadRequest, apperr.CodeValidationFailed, "Liên kết đặt lại mật khẩu không hợp lệ hoặc đã hết hạn.")
	ErrPasswordChange      = apperr.New(http.StatusForbidden, apperr.CodePasswordChangeRequired, "Bạn cần đổi mật khẩu trước khi tiếp tục.")
	// ErrPasswordTooShort là gốc của lỗi độ dài; thông điệp thật mang PASSWORD_MIN_LENGTH (xem passwordTooShort).
	ErrPasswordTooShort = apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidationFailed, "Mật khẩu mới quá ngắn.")

	ErrAlreadyActivated = domain.ErrConflict.WithMsg("Học viên đã đổi mật khẩu; không cần gửi lại lời mời.")
	ErrEmailTaken       = domain.ErrConflict.WithMsg("Email đã được sử dụng")
	ErrAdminProtected   = domain.ErrConflict.WithMsg("Không vô hiệu hóa được tài khoản quản trị.")
	ErrUserNotFound     = domain.ErrNotFound.WithMsg("Không tìm thấy người dùng")
	ErrNameRequired     = domain.ErrInvalid.WithMsg("Nhập họ tên học viên.")
	ErrNameTooLong      = domain.ErrInvalid.WithMsg(fmt.Sprintf("Họ tên tối đa %d ký tự.", maxNameRunes))
	errUserDisabled     = domain.ErrInvalidTransition.WithMsg("Tài khoản đã bị vô hiệu hóa")
)

// passwordTooShort là ErrPasswordTooShort với độ dài tối thiểu đang cấu hình.
func passwordTooShort(minLen int) error {
	return apperr.Wrap(ErrPasswordTooShort, http.StatusUnprocessableEntity, apperr.CodeValidationFailed,
		fmt.Sprintf("Mật khẩu mới cần tối thiểu %d ký tự.", minLen))
}

// User là aggregate tài khoản. Field không export; dựng qua NewInvitedStudent, NewInternalUser hoặc RehydrateUser.
// Bảng users không có cột version: ghi theo id trong transaction có SELECT … FOR UPDATE khi cần.
type User struct {
	id           uuid.UUID
	email        domain.Email
	name         string
	role         domain.Role
	status       domain.UserStatus
	password     PasswordHash
	mustChange   bool
	tempExpires  *time.Time
	lastLoginAt  *time.Time
	lastActiveAt *time.Time
	disabledAt   *time.Time
	createdAt    time.Time
	updatedAt    time.Time
}

// UserSnapshot là bản sao công khai của User để repository ghi/đọc và feature khác đọc (không có hash).
type UserSnapshot struct {
	ID                    uuid.UUID
	Email                 domain.Email
	Name                  string
	Role                  domain.Role
	Status                domain.UserStatus
	MustChangePassword    bool
	TempPasswordExpiresAt *time.Time
	LastLoginAt           *time.Time
	LastActiveAt          *time.Time
	DisabledAt            *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", ErrNameRequired
	case utf8.RuneCountInString(name) > maxNameRunes:
		return "", ErrNameTooLong
	}
	return name, nil
}

// NewInvitedStudent tạo học viên invited với hash của mật khẩu tạm, hết hạn sau ttl.
func NewInvitedStudent(id uuid.UUID, email domain.Email, name string, hash PasswordHash, now time.Time, ttl time.Duration) (*User, error) {
	u, err := newUser(id, email, name, domain.RoleStudent, hash, now)
	if err != nil {
		return nil, err
	}
	expires := now.Add(ttl)
	u.status, u.mustChange, u.tempExpires = domain.UserInvited, true, &expires
	return u, nil
}

// NewInternalUser tạo admin hoặc giảng viên đã kích hoạt (không có mật khẩu tạm).
func NewInternalUser(id uuid.UUID, email domain.Email, name string, role domain.Role, hash PasswordHash, now time.Time) (*User, error) {
	if role != domain.RoleAdmin && role != domain.RoleTeacher {
		return nil, domain.ErrInvalidRole
	}
	u, err := newUser(id, email, name, role, hash, now)
	if err != nil {
		return nil, err
	}
	u.status = domain.UserActive
	return u, nil
}

func newUser(id uuid.UUID, email domain.Email, name string, role domain.Role, hash PasswordHash, now time.Time) (*User, error) {
	if email.IsZero() {
		return nil, domain.ErrInvalidEmail
	}
	name, err := normalizeName(name)
	if err != nil {
		return nil, err
	}
	return &User{id: id, email: email, name: name, role: role, password: hash, createdAt: now, updatedAt: now}, nil
}

// RehydrateUser dựng lại User từ dữ liệu đã lưu, không kiểm quy tắc tạo mới.
func RehydrateUser(s UserSnapshot, hash PasswordHash) *User {
	return &User{
		id: s.ID, email: s.Email, name: s.Name, role: s.Role, status: s.Status, password: hash,
		mustChange: s.MustChangePassword, tempExpires: s.TempPasswordExpiresAt,
		lastLoginAt: s.LastLoginAt, lastActiveAt: s.LastActiveAt, disabledAt: s.DisabledAt,
		createdAt: s.CreatedAt, updatedAt: s.UpdatedAt,
	}
}

// Snapshot trả bản sao công khai (không có hash).
func (u *User) Snapshot() UserSnapshot {
	return UserSnapshot{
		ID: u.id, Email: u.email, Name: u.name, Role: u.role, Status: u.status,
		MustChangePassword: u.mustChange, TempPasswordExpiresAt: u.tempExpires,
		LastLoginAt: u.lastLoginAt, LastActiveAt: u.lastActiveAt, DisabledAt: u.disabledAt,
		CreatedAt: u.createdAt, UpdatedAt: u.updatedAt,
	}
}

// ID là id người dùng.
func (u *User) ID() uuid.UUID { return u.id }

// Email là email đã chuẩn hóa.
func (u *User) Email() domain.Email { return u.email }

// Name là họ tên.
func (u *User) Name() string { return u.name }

// Role là vai trò.
func (u *User) Role() domain.Role { return u.role }

// Status là trạng thái tài khoản.
func (u *User) Status() domain.UserStatus { return u.status }

// PasswordHash là hash hiện tại; chỉ dùng để verify.
func (u *User) PasswordHash() PasswordHash { return u.password }

// TempPasswordExpiresAt là hạn mật khẩu tạm, nil khi không còn mật khẩu tạm.
func (u *User) TempPasswordExpiresAt() *time.Time { return u.tempExpires }

// IsStudent cho biết user là học viên.
func (u *User) IsStudent() bool { return u.role == domain.RoleStudent }

// MustChangePassword cho biết user còn dùng mật khẩu tạm và phải đổi trước khi dùng API khác.
func (u *User) MustChangePassword() bool { return u.mustChange }

// Reinvite cấp hash mật khẩu tạm mới hạn ttl (gửi lại lời mời). Chỉ khi user chưa đổi mật khẩu lần đầu.
func (u *User) Reinvite(hash PasswordHash, now time.Time, ttl time.Duration) error {
	if u.status == domain.UserDisabled {
		return errUserDisabled
	}
	if !u.mustChange {
		return ErrAlreadyActivated
	}
	expires := now.Add(ttl)
	u.password, u.tempExpires, u.updatedAt = hash, &expires, now
	return nil
}

// Authenticate kiểm đăng nhập theo thứ tự: verify mật khẩu trước (sai luôn là ErrInvalidCredentials, không lộ
// trạng thái), rồi mới tới tài khoản bị vô hiệu hóa, rồi mật khẩu tạm hết hạn. verify nhận hash để entity không
// phụ thuộc thuật toán hash.
func (u *User) Authenticate(now time.Time, verify func(PasswordHash) bool) error {
	if !verify(u.password) {
		return ErrInvalidCredentials
	}
	if u.status == domain.UserDisabled {
		return ErrAccountDisabled
	}
	if u.mustChange && u.tempExpires != nil && !now.Before(*u.tempExpires) {
		return ErrTempPasswordExpired
	}
	return nil
}

// ChangePassword đặt mật khẩu mới: hết mật khẩu tạm, invited → active.
func (u *User) ChangePassword(newHash PasswordHash, now time.Time) error {
	return u.setPassword(newHash, now)
}

// ResetPassword đặt mật khẩu mới qua liên kết quên mật khẩu; tài khoản vô hiệu hóa bị từ chối. Trạng thái chỉ đổi
// invited → active, active giữ nguyên.
func (u *User) ResetPassword(newHash PasswordHash, now time.Time) error {
	return u.setPassword(newHash, now)
}

func (u *User) setPassword(newHash PasswordHash, now time.Time) error {
	if u.status == domain.UserDisabled {
		return ErrAccountDisabled
	}
	if u.status == domain.UserInvited {
		if err := u.status.Transition(domain.UserActive); err != nil {
			return err
		}
		u.status = domain.UserActive
	}
	u.password, u.mustChange, u.tempExpires, u.updatedAt = newHash, false, nil, now
	return nil
}

// Disable chuyển invited|active → disabled. Không cho vô hiệu admin là quy tắc của service.
func (u *User) Disable(now time.Time) error {
	if err := u.status.Transition(domain.UserDisabled); err != nil {
		return err
	}
	u.status, u.disabledAt, u.updatedAt = domain.UserDisabled, &now, now
	return nil
}

// Enable kích hoạt lại: về invited khi user vẫn phải đổi mật khẩu tạm, ngược lại về active.
func (u *User) Enable(now time.Time) error {
	next, err := u.status.Enable(u.mustChange)
	if err != nil {
		return err
	}
	u.status, u.disabledAt, u.updatedAt = next, nil, now
	return nil
}

// RecordLogin ghi thời điểm đăng nhập thành công (cũng là lần hoạt động gần nhất).
func (u *User) RecordLogin(now time.Time) {
	u.lastLoginAt, u.lastActiveAt, u.updatedAt = &now, &now, now
}

// Touch ghi lần hoạt động gần nhất.
func (u *User) Touch(now time.Time) {
	u.lastActiveAt = &now
}

// Session là phiên đăng nhập; DB chỉ lưu SHA-256 của token.
type Session struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TokenHash  []byte
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
	IP         string // rỗng khi không xác định
	UserAgent  string
}

// IsExpired cho biết phiên đã hết hạn tại now.
func (s *Session) IsExpired(now time.Time) bool { return !now.Before(s.ExpiresAt) }

// LoginAttempt là một lần đăng nhập (append-only), dùng để khóa theo email.
type LoginAttempt struct {
	EmailNormalized string
	IP              string
	Succeeded       bool
	AttemptedAt     time.Time
}

// PasswordResetToken là token đặt lại mật khẩu dùng một lần; DB chỉ lưu SHA-256.
type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}
