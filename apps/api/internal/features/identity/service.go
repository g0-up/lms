package identity

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
)

// attemptRetention là thời gian giữ login_attempts trước khi worker dọn.
const attemptRetention = 30 * 24 * time.Hour

// ServiceDeps là phụ thuộc của Service. Rand nil thì dùng crypto/rand.
type ServiceDeps struct {
	DB       db.Executor // thao tác một câu ngoài transaction (đọc phiên, đăng xuất)
	Tx       db.Tx
	Users    UserRepo
	Sessions SessionRepo
	Attempts LoginAttemptRepo
	Resets   ResetTokenRepo
	Mailer   mailer.Enqueuer
	Clock    clock.Clock
	Audit    audit.Recorder
	Cfg      config.Config
	Rand     io.Reader
}

// Service là các use case của identity.
type Service struct {
	ServiceDeps
}

var _ UserProvisioner = (*Service)(nil)

// NewService tạo Service.
func NewService(d ServiceDeps) *Service {
	if d.Rand == nil {
		d.Rand = rand.Reader
	}
	return &Service{ServiceDeps: d}
}

// LoginResult là kết quả đăng nhập thành công; Token chỉ đi vào cookie.
type LoginResult struct {
	Session *Session
	Token   SessionToken
	User    *User
}

// Login đăng nhập trong một transaction giữ advisory lock theo email: kiểm khóa tạm, verify, ghi attempt và tạo
// phiên không bị đua giữa các request song song.
func (s *Service) Login(ctx context.Context, rawEmail, password, ip, userAgent string) (*LoginResult, error) {
	email, err := domain.ParseEmail(rawEmail)
	if err != nil {
		return nil, ErrInvalidEmail
	}
	token, err := NewSessionToken(s.Rand)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	now := s.Clock.Now()
	var (
		res      *LoginResult
		loginErr error // lỗi nghiệp vụ trả sau khi commit để attempt sai vẫn được ghi
	)
	err = s.Tx.Transact(ctx, func(tx db.Executor) error {
		if err := s.Attempts.LockEmail(ctx, tx, email); err != nil {
			return err
		}
		failures, oldest, err := s.Attempts.CountFailures(ctx, tx, email, now.Add(-s.Cfg.LoginLockWindow))
		if err != nil {
			return err
		}
		if failures >= s.Cfg.LoginMaxFailures {
			loginErr = s.tooManyAttempts(now, oldest)
			return nil
		}
		fail := LoginAttempt{EmailNormalized: email.String(), IP: ip, AttemptedAt: now}

		u, err := s.Users.ByEmail(ctx, tx, email)
		if errors.Is(err, ErrUserNotFound) {
			dummyHash().Verify(password) // đồng đều thời gian với email có thật
			loginErr = ErrInvalidCredentials
			return s.Attempts.Record(ctx, tx, fail)
		}
		if err != nil {
			return err
		}
		if err := u.Authenticate(now, func(h PasswordHash) bool { return h.Verify(password) }); err != nil {
			loginErr = err
			if errors.Is(err, ErrInvalidCredentials) {
				return s.Attempts.Record(ctx, tx, fail)
			}
			return nil // disabled / mật khẩu tạm hết hạn không tính là lần sai
		}

		// Khóa hàng user rồi kiểm lại để không đua với DisableUser.
		u, err = s.Users.ByIDForUpdate(ctx, tx, u.ID())
		if err != nil {
			return err
		}
		if u.Status() == domain.UserDisabled {
			loginErr = ErrAccountDisabled
			return nil
		}
		ok := fail
		ok.Succeeded = true
		if err := s.Attempts.Record(ctx, tx, ok); err != nil {
			return err
		}
		u.RecordLogin(now)
		if err := s.Users.Update(ctx, tx, u); err != nil {
			return err
		}
		sess := &Session{
			ID: ids.New(), UserID: u.ID(), TokenHash: token.Hash(), CreatedAt: now,
			ExpiresAt: now.Add(s.Cfg.SessionTTL), LastSeenAt: now, IP: ip, UserAgent: userAgent,
		}
		if err := s.Sessions.Create(ctx, tx, sess); err != nil {
			return err
		}
		res = &LoginResult{Session: sess, Token: token, User: u}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if loginErr != nil {
		return nil, loginErr
	}
	return res, nil
}

// tooManyAttempts dựng lỗi 429 với Retry-After là số giây tới khi lần sai cũ nhất ra khỏi cửa sổ.
func (s *Service) tooManyAttempts(now time.Time, oldest *time.Time) error {
	wait := s.Cfg.LoginLockWindow
	if oldest != nil {
		wait = oldest.Add(s.Cfg.LoginLockWindow).Sub(now)
	}
	secs := max(int(math.Ceil(wait.Seconds())), 1)
	e := apperr.New(http.StatusTooManyRequests, apperr.CodeTooManyAttempts,
		fmt.Sprintf("Bạn đã nhập sai quá nhiều lần. Thử lại sau %d phút.", int(math.Ceil(s.Cfg.LoginLockWindow.Minutes()))))
	e.Details = map[string]string{"retry_after": strconv.Itoa(secs)}
	return e
}

// Authenticate trả phiên và user theo token cookie. Phiên hết hạn bị xóa; user disabled không có phiên.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (*Session, *User, error) {
	if rawToken == "" {
		return nil, nil, ErrSessionNotFound
	}
	sess, u, err := s.Sessions.ByTokenHash(ctx, s.DB, HashToken(rawToken))
	if err != nil {
		return nil, nil, err
	}
	now := s.Clock.Now()
	if sess.IsExpired(now) {
		if err := s.Sessions.DeleteByID(ctx, s.DB, sess.ID); err != nil {
			return nil, nil, err
		}
		return nil, nil, ErrSessionNotFound
	}
	if err := s.Sessions.TouchLastSeen(ctx, s.DB, sess.ID, now, s.Cfg.SessionTTL); err != nil {
		return nil, nil, err
	}
	return sess, u, nil
}

// Logout xóa phiên của token; token rỗng hoặc không tồn tại vẫn thành công.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.Sessions.DeleteByTokenHash(ctx, s.DB, HashToken(rawToken))
}

// ChangePasswordCmd là yêu cầu đổi mật khẩu. CurrentPassword bắt buộc khi tài khoản active, bỏ qua khi invited.
type ChangePasswordCmd struct {
	CurrentPassword *string
	NewPassword     string
	ConfirmPassword string
}

// ChangePassword đổi mật khẩu của user đang đăng nhập và thu hồi các phiên khác cùng mọi token đặt lại còn mở.
func (s *Service) ChangePassword(ctx context.Context, user *User, sessionID uuid.UUID, cmd ChangePasswordCmd) (*User, error) {
	if err := s.validateNewPassword(cmd.NewPassword, cmd.ConfirmPassword); err != nil {
		return nil, err
	}
	if user.Status() == domain.UserActive {
		if cmd.CurrentPassword == nil || *cmd.CurrentPassword == "" {
			return nil, ErrCurrentPassword
		}
		if !user.PasswordHash().Verify(*cmd.CurrentPassword) {
			return nil, ErrInvalidCredentials
		}
	}
	if user.PasswordHash().Verify(cmd.NewPassword) {
		return nil, ErrPasswordSameAsTemp
	}
	hash, err := NewPasswordHash(cmd.NewPassword)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	now := s.Clock.Now()
	var out *User
	err = s.Tx.Transact(ctx, func(tx db.Executor) error {
		u, err := s.Users.ByIDForUpdate(ctx, tx, user.ID())
		if err != nil {
			return err
		}
		if err := u.ChangePassword(hash, now); err != nil {
			return err
		}
		if err := s.Users.Update(ctx, tx, u); err != nil {
			return err
		}
		if err := s.Sessions.DeleteAllForUser(ctx, tx, u.ID(), &sessionID); err != nil {
			return err
		}
		if err := s.Resets.InvalidateForUser(ctx, tx, u.ID(), now); err != nil {
			return err
		}
		out = u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) validateNewPassword(newPassword, confirm string) error {
	if newPassword != confirm {
		return ErrPasswordMismatch
	}
	if utf8.RuneCountInString(newPassword) < s.Cfg.PasswordMinLength {
		return passwordTooShort(s.Cfg.PasswordMinLength)
	}
	var de *domain.Error
	if err := domain.ValidatePassword(newPassword, s.Cfg.PasswordMinLength); errors.As(err, &de) {
		return apperr.Wrap(err, http.StatusUnprocessableEntity, apperr.CodeValidationFailed, de.Msg)
	} else if err != nil {
		return err
	}
	return nil
}

// ForgotPassword gửi email đặt lại nếu email thuộc tài khoản chưa bị vô hiệu hóa. Mọi nhánh đều sinh token và mở
// một transaction để thời gian xử lý không lộ email có tồn tại hay không; handler luôn trả cùng một phản hồi.
func (s *Service) ForgotPassword(ctx context.Context, rawEmail string) error {
	token, err := NewResetToken(s.Rand)
	if err != nil {
		return apperr.Internal(err)
	}
	email, parseErr := domain.ParseEmail(rawEmail)
	now := s.Clock.Now()
	return s.Tx.Transact(ctx, func(tx db.Executor) error {
		if parseErr != nil {
			return nil
		}
		u, err := s.Users.ByEmail(ctx, tx, email)
		if errors.Is(err, ErrUserNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if u.Status() == domain.UserDisabled {
			return nil
		}
		if err := s.Resets.InvalidateForUser(ctx, tx, u.ID(), now); err != nil {
			return err
		}
		ttl := s.Cfg.ResetTokenTTL
		if err := s.Resets.Create(ctx, tx, &PasswordResetToken{
			ID: ids.New(), UserID: u.ID(), TokenHash: token.Hash(), ExpiresAt: now.Add(ttl), CreatedAt: now,
		}); err != nil {
			return err
		}
		_, err = s.Mailer.Enqueue(ctx, tx, mailer.Message{
			To:       u.Email(),
			Template: mailer.TemplatePasswordReset,
			Payload:  map[string]any{"Name": u.Name(), "ExpiresMinutes": int(math.Round(ttl.Minutes()))},
			Secret:   token.Reveal(),
		})
		return err
	})
}

// ResetPasswordCmd là yêu cầu đặt lại mật khẩu bằng token trong liên kết email.
type ResetPasswordCmd struct {
	Token           string
	NewPassword     string
	ConfirmPassword string
}

// ResetPassword tiêu thụ token và đặt mật khẩu mới trong một transaction; mọi phiên của user bị thu hồi.
func (s *Service) ResetPassword(ctx context.Context, cmd ResetPasswordCmd) error {
	if err := s.validateNewPassword(cmd.NewPassword, cmd.ConfirmPassword); err != nil {
		return err
	}
	if cmd.Token == "" {
		return ErrResetTokenInvalid
	}
	hash, err := NewPasswordHash(cmd.NewPassword)
	if err != nil {
		return apperr.Internal(err)
	}
	now := s.Clock.Now()
	return s.Tx.Transact(ctx, func(tx db.Executor) error {
		userID, err := s.Resets.Consume(ctx, tx, HashToken(cmd.Token), now)
		if err != nil {
			return err
		}
		u, err := s.Users.ByIDForUpdate(ctx, tx, userID)
		if err != nil {
			return err
		}
		if err := u.ResetPassword(hash, now); err != nil {
			return err // disabled: rollback, token không bị tiêu thụ
		}
		if err := s.Users.Update(ctx, tx, u); err != nil {
			return err
		}
		if err := s.Resets.InvalidateForUser(ctx, tx, u.ID(), now); err != nil {
			return err
		}
		return s.Sessions.DeleteAllForUser(ctx, tx, u.ID(), nil)
	})
}

// DisableUser vô hiệu hóa học viên hoặc giảng viên: hủy mọi phiên, vô hiệu token đặt lại, ghi audit user.disabled.
func (s *Service) DisableUser(ctx context.Context, actor *User, userID uuid.UUID, requestID string) (*User, error) {
	return s.setEnabled(ctx, actor, userID, requestID, false)
}

// EnableUser kích hoạt lại tài khoản và ghi audit user.enabled.
func (s *Service) EnableUser(ctx context.Context, actor *User, userID uuid.UUID, requestID string) (*User, error) {
	return s.setEnabled(ctx, actor, userID, requestID, true)
}

func (s *Service) setEnabled(ctx context.Context, actor *User, userID uuid.UUID, requestID string, enable bool) (*User, error) {
	now := s.Clock.Now()
	var out *User
	err := s.Tx.Transact(ctx, func(tx db.Executor) error {
		u, err := s.Users.ByIDForUpdate(ctx, tx, userID)
		if err != nil {
			return err
		}
		if u.Role() == domain.RoleAdmin {
			return ErrAdminProtected
		}
		before := u.Status()
		action := audit.ActionUserEnabled
		if enable {
			err = u.Enable(now)
		} else {
			action = audit.ActionUserDisabled
			err = u.Disable(now)
		}
		if err != nil {
			return err
		}
		if err := s.Users.Update(ctx, tx, u); err != nil {
			return err
		}
		if !enable {
			if err := s.Sessions.DeleteAllForUser(ctx, tx, u.ID(), nil); err != nil {
				return err
			}
			if err := s.Resets.InvalidateForUser(ctx, tx, u.ID(), now); err != nil {
				return err
			}
		}
		actorID, targetID := actor.ID(), u.ID()
		if err := s.Audit.Record(ctx, tx, audit.Entry{
			ActorID: &actorID, Action: action, TargetType: "user", TargetID: &targetID,
			Before: map[string]any{"status": before}, After: map[string]any{"status": u.Status()}, RequestID: requestID,
		}); err != nil {
			return err
		}
		out = u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListUsers liệt kê user theo vai trò và trạng thái (nil = mọi trạng thái).
func (s *Service) ListUsers(ctx context.Context, role domain.Role, status *domain.UserStatus) ([]*User, error) {
	return s.Users.ListByRole(ctx, s.DB, role, status)
}

// ProvisionStudent trả học viên có sẵn theo email hoặc tạo học viên invited với mật khẩu tạm mới.
func (s *Service) ProvisionStudent(ctx context.Context, ex db.Executor, email domain.Email, name string, now time.Time) (*User, TemporaryPassword, bool, error) {
	u, err := s.Users.ByEmail(ctx, ex, email)
	if err == nil {
		return u, TemporaryPassword{}, false, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, TemporaryPassword{}, false, err
	}
	tmp, hash, err := s.newTemporaryPassword()
	if err != nil {
		return nil, TemporaryPassword{}, false, err
	}
	u, err = NewInvitedStudent(ids.New(), email, name, hash, now, s.Cfg.TempPasswordTTL)
	if err != nil {
		return nil, TemporaryPassword{}, false, err
	}
	if err := s.Users.Create(ctx, ex, u); err != nil {
		return nil, TemporaryPassword{}, false, err
	}
	return u, tmp, true, nil
}

// RotateTemporaryPassword cấp mật khẩu tạm mới (hạn TempPasswordTTL) cho học viên chưa đổi mật khẩu lần đầu.
func (s *Service) RotateTemporaryPassword(ctx context.Context, ex db.Executor, u *User, now time.Time) (TemporaryPassword, error) {
	tmp, hash, err := s.newTemporaryPassword()
	if err != nil {
		return TemporaryPassword{}, err
	}
	if err := u.Reinvite(hash, now, s.Cfg.TempPasswordTTL); err != nil {
		return TemporaryPassword{}, err
	}
	if err := s.Users.Update(ctx, ex, u); err != nil {
		return TemporaryPassword{}, err
	}
	return tmp, nil
}

func (s *Service) newTemporaryPassword() (TemporaryPassword, PasswordHash, error) {
	tmp, err := GenerateTemporaryPassword(s.Rand, TemporaryPasswordLength)
	if err != nil {
		return TemporaryPassword{}, PasswordHash{}, err
	}
	hash, err := NewPasswordHash(tmp.Reveal())
	if err != nil {
		return TemporaryPassword{}, PasswordHash{}, err
	}
	return tmp, hash, nil
}

// Cleanup xóa login_attempts quá 30 ngày và phiên đã hết hạn; worker gọi mỗi giờ.
func (s *Service) Cleanup(ctx context.Context, now time.Time) error {
	if _, err := s.Attempts.DeleteOlderThan(ctx, s.DB, now.Add(-attemptRetention)); err != nil {
		return err
	}
	_, err := s.Sessions.DeleteExpired(ctx, s.DB, now)
	return err
}
