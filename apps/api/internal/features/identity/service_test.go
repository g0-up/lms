package identity

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/audit"
)

const goodPassword = "matkhau-dung-1"

func invitedStudent(t *testing.T, clkNow time.Time, email string) func(PasswordHash) (*User, error) {
	return func(h PasswordHash) (*User, error) {
		return NewInvitedStudent(uuid.New(), mustEmail(t, email), "Bùi Minh", h, clkNow, 72*time.Hour)
	}
}

func internalUser(t *testing.T, clkNow time.Time, email string, role domain.Role) func(PasswordHash) (*User, error) {
	return func(h PasswordHash) (*User, error) {
		return NewInternalUser(uuid.New(), mustEmail(t, email), "Lê Hương", role, h, clkNow)
	}
}

func requireAppErr(t *testing.T, err error, status int, code string) {
	t.Helper()
	_ = asAppErr(t, err, status, code)
}

// asAppErr như requireAppErr nhưng trả lỗi để kiểm thêm Message/Details.
func asAppErr(t *testing.T, err error, status int, code string) *apperr.Error {
	t.Helper()
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, status, ae.Status)
	assert.Equal(t, code, ae.Code)
	return ae
}

func TestLoginSuccessCreatesSession(t *testing.T) {
	svc, m, clk := newMemService(t)
	u := addUser(t, m, invitedStudent(t, clk.Now(), "minh.bui@gmail.com"), goodPassword)

	res, err := svc.Login(context.Background(), "  Minh.Bui@Gmail.com ", goodPassword, "10.0.0.1", "ua")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), res.User.ID())
	assert.Equal(t, res.Token.Hash(), res.Session.TokenHash)
	assert.Equal(t, clk.Now().Add(12*time.Hour), res.Session.ExpiresAt)
	assert.Equal(t, "10.0.0.1", res.Session.IP)
	require.Len(t, m.sessions, 1)
	require.Len(t, m.attempts, 1)
	assert.True(t, m.attempts[0].Succeeded)
	assert.Equal(t, clk.Now(), *m.users[u.ID()].LastLoginAt)
	assert.True(t, res.User.MustChangePassword(), "đăng nhập bằng mật khẩu tạm vẫn phải đổi mật khẩu")
}

func TestLoginLocksAfterMaxFailures(t *testing.T) {
	svc, m, clk := newMemService(t)
	addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	ctx := context.Background()

	first := clk.Now()
	for i := range 5 {
		_, err := svc.Login(ctx, "huong.le@goup.vn", "sai", "", "")
		requireAppErr(t, err, http.StatusUnauthorized, apperr.CodeUnauthenticated)
		if i == 0 {
			clk.Advance(time.Minute)
		}
	}
	_, err := svc.Login(ctx, "huong.le@goup.vn", goodPassword, "", "")
	ae := asAppErr(t, err, http.StatusTooManyRequests, apperr.CodeTooManyAttempts)
	assert.Equal(t, "Bạn đã nhập sai quá nhiều lần. Thử lại sau 15 phút.", ae.Message)
	// lần sai đầu cách now 1 phút → còn 14 phút
	assert.Equal(t, "840", ae.Details["retry_after"])
	assert.Len(t, m.attempts, 5, "lần bị khóa không ghi attempt")

	// lần sai đầu ra khỏi cửa sổ → còn 4 lần sai, đăng nhập đúng được
	clk.Set(first.Add(15*time.Minute + time.Second))
	_, err = svc.Login(ctx, "huong.le@goup.vn", goodPassword, "", "")
	require.NoError(t, err)
}

func TestLoginUnknownEmailRecordsFailure(t *testing.T) {
	svc, m, _ := newMemService(t)
	_, err := svc.Login(context.Background(), "khongtontai@example.com", "x", "1.2.3.4", "")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	require.Len(t, m.attempts, 1)
	assert.False(t, m.attempts[0].Succeeded)
	assert.Equal(t, "khongtontai@example.com", m.attempts[0].EmailNormalized)
}

func TestLoginInvalidEmail(t *testing.T) {
	svc, m, _ := newMemService(t)
	_, err := svc.Login(context.Background(), "khong-phai-email", "x", "", "")
	requireAppErr(t, err, http.StatusUnprocessableEntity, apperr.CodeValidationFailed)
	assert.Empty(t, m.attempts)
}

func TestLoginStatusErrorsDoNotCountAsFailures(t *testing.T) {
	svc, m, clk := newMemService(t)
	expired := addUser(t, m, invitedStudent(t, clk.Now().Add(-73*time.Hour), "dung.pham@gmail.com"), goodPassword)
	disabled := addUser(t, m, invitedStudent(t, clk.Now(), "thao.vo@gmail.com"), goodPassword)
	d, _ := m.get(disabled.ID())
	require.NoError(t, d.Disable(clk.Now()))
	m.put(d)
	ctx := context.Background()

	_, err := svc.Login(ctx, expired.Email().String(), goodPassword, "", "")
	requireAppErr(t, err, http.StatusUnauthorized, apperr.CodeTempPasswordExpired)
	_, err = svc.Login(ctx, disabled.Email().String(), goodPassword, "", "")
	requireAppErr(t, err, http.StatusForbidden, apperr.CodeAccountDisabled)
	_, err = svc.Login(ctx, disabled.Email().String(), "sai", "", "")
	requireAppErr(t, err, http.StatusUnauthorized, apperr.CodeUnauthenticated)

	require.Len(t, m.attempts, 1, "chỉ lần sai mật khẩu được ghi")
	assert.Empty(t, m.sessions)
}

func TestTooManyAttemptsRetryAfterIsAtLeastOneSecond(t *testing.T) {
	svc, _, clk := newMemService(t)
	now := clk.Now()
	old := now.Add(-15 * time.Minute)
	ae := asAppErr(t, svc.tooManyAttempts(now, &old), http.StatusTooManyRequests, apperr.CodeTooManyAttempts)
	assert.Equal(t, "1", ae.Details["retry_after"])
	ae = asAppErr(t, svc.tooManyAttempts(now, nil), http.StatusTooManyRequests, apperr.CodeTooManyAttempts)
	assert.Equal(t, "900", ae.Details["retry_after"])
}

func TestAuthenticateSlidesAndExpiresSession(t *testing.T) {
	svc, m, clk := newMemService(t)
	addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	ctx := context.Background()
	res, err := svc.Login(ctx, "huong.le@goup.vn", goodPassword, "", "")
	require.NoError(t, err)

	_, _, err = svc.Authenticate(ctx, "")
	require.ErrorIs(t, err, ErrSessionNotFound)
	_, _, err = svc.Authenticate(ctx, "token-la")
	require.ErrorIs(t, err, ErrSessionNotFound)

	clk.Advance(11 * time.Hour)
	sess, u, err := svc.Authenticate(ctx, res.Token.Reveal())
	require.NoError(t, err)
	assert.Equal(t, res.Session.ID, sess.ID)
	assert.Equal(t, "huong.le@goup.vn", u.Email().String())
	assert.Equal(t, clk.Now().Add(12*time.Hour), m.sessions[sess.ID].ExpiresAt, "gia hạn trượt")

	clk.Advance(12 * time.Hour)
	_, _, err = svc.Authenticate(ctx, res.Token.Reveal())
	require.ErrorIs(t, err, ErrSessionNotFound)
	assert.Empty(t, m.sessions, "phiên hết hạn bị xóa")
}

func TestLogoutDeletesSessionAndToleratesMissingToken(t *testing.T) {
	svc, m, clk := newMemService(t)
	addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	ctx := context.Background()
	res, err := svc.Login(ctx, "huong.le@goup.vn", goodPassword, "", "")
	require.NoError(t, err)

	require.NoError(t, svc.Logout(ctx, ""))
	require.NoError(t, svc.Logout(ctx, "khong-ton-tai"))
	require.Len(t, m.sessions, 1)
	require.NoError(t, svc.Logout(ctx, res.Token.Reveal()))
	assert.Empty(t, m.sessions)
}

func TestChangePasswordValidationOrder(t *testing.T) {
	svc, m, clk := newMemService(t)
	active := addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	ctx := context.Background()
	ptr := func(s string) *string { return &s }

	cases := []struct {
		name string
		cmd  ChangePasswordCmd
		want string
		code int
	}{
		{"không khớp", ChangePasswordCmd{CurrentPassword: ptr(goodPassword), NewPassword: "matkhaumoi1", ConfirmPassword: "khac"}, "Hai mật khẩu không khớp.", 422},
		{"quá ngắn", ChangePasswordCmd{CurrentPassword: ptr(goodPassword), NewPassword: "ngan1", ConfirmPassword: "ngan1"}, "Mật khẩu mới cần tối thiểu 8 ký tự.", 422},
		{"thiếu mật khẩu hiện tại", ChangePasswordCmd{NewPassword: "matkhaumoi1", ConfirmPassword: "matkhaumoi1"}, "Nhập mật khẩu hiện tại.", 422},
		{"mật khẩu hiện tại rỗng", ChangePasswordCmd{CurrentPassword: ptr(""), NewPassword: "matkhaumoi1", ConfirmPassword: "matkhaumoi1"}, "Nhập mật khẩu hiện tại.", 422},
		{"sai mật khẩu hiện tại", ChangePasswordCmd{CurrentPassword: ptr("sai"), NewPassword: "matkhaumoi1", ConfirmPassword: "matkhaumoi1"}, "Email hoặc mật khẩu không đúng.", 401},
		{"trùng mật khẩu cũ", ChangePasswordCmd{CurrentPassword: ptr(goodPassword), NewPassword: goodPassword, ConfirmPassword: goodPassword}, "Mật khẩu mới phải khác mật khẩu tạm.", 422},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.ChangePassword(ctx, active, uuid.New(), c.cmd)
			var ae *apperr.Error
			require.ErrorAs(t, err, &ae)
			assert.Equal(t, c.code, ae.Status)
			assert.Equal(t, c.want, ae.Message)
		})
	}
	assert.Equal(t, active.PasswordHash(), m.hashes[active.ID()], "lỗi kiểm tra không đổi mật khẩu")
}

func TestChangePasswordInvitedSkipsCurrentAndRevokesOtherSessions(t *testing.T) {
	svc, m, clk := newMemService(t)
	u := addUser(t, m, invitedStudent(t, clk.Now(), "minh.bui@gmail.com"), goodPassword)
	ctx := context.Background()
	keep, err := svc.Login(ctx, "minh.bui@gmail.com", goodPassword, "", "")
	require.NoError(t, err)
	_, err = svc.Login(ctx, "minh.bui@gmail.com", goodPassword, "", "")
	require.NoError(t, err)
	require.NoError(t, memResets{m}.Create(ctx, nil, &PasswordResetToken{ID: uuid.New(), UserID: u.ID(), TokenHash: []byte("h"), ExpiresAt: clk.Now().Add(time.Hour)}))
	wrong := "bi-bo-qua"

	out, err := svc.ChangePassword(ctx, keep.User, keep.Session.ID, ChangePasswordCmd{
		CurrentPassword: &wrong, NewPassword: "matkhaumoi1", ConfirmPassword: "matkhaumoi1",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.UserActive, out.Status())
	assert.False(t, out.MustChangePassword())
	assert.Nil(t, out.TempPasswordExpiresAt())
	assert.True(t, m.hashes[u.ID()].Verify("matkhaumoi1"))
	require.Len(t, m.sessions, 1)
	_, ok := m.sessions[keep.Session.ID]
	assert.True(t, ok, "giữ phiên hiện tại")
	require.NotNil(t, m.resets[0].UsedAt, "token đặt lại còn mở bị vô hiệu")
}

func TestValidateNewPasswordUsesDomainRules(t *testing.T) {
	svc, _, _ := newMemService(t)
	require.NoError(t, svc.validateNewPassword("matkhaumoi1", "matkhaumoi1"))
	blank := strings.Repeat(" ", 10)
	ae := asAppErr(t, svc.validateNewPassword(blank, blank), http.StatusUnprocessableEntity, apperr.CodeValidationFailed)
	assert.Equal(t, "Mật khẩu không được chỉ gồm khoảng trắng", ae.Message)
	long := strings.Repeat("a", 129)
	ae = asAppErr(t, svc.validateNewPassword(long, long), http.StatusUnprocessableEntity, apperr.CodeValidationFailed)
	require.ErrorIs(t, ae, domain.ErrInvalidPassword)
}

func TestForgotPasswordEnqueuesOnlyForEnabledUsers(t *testing.T) {
	svc, m, clk := newMemService(t)
	u := addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	d := addUser(t, m, invitedStudent(t, clk.Now(), "thao.vo@gmail.com"), goodPassword)
	du, _ := m.get(d.ID())
	require.NoError(t, du.Disable(clk.Now()))
	m.put(du)
	ctx := context.Background()

	for _, email := range []string{"khong-phai-email", "khongtontai@example.com", "thao.vo@gmail.com"} {
		require.NoError(t, svc.ForgotPassword(ctx, email), email)
	}
	assert.Empty(t, m.mails)
	assert.Empty(t, m.resets)

	require.NoError(t, svc.ForgotPassword(ctx, "HUONG.LE@goup.vn"))
	require.NoError(t, svc.ForgotPassword(ctx, "huong.le@goup.vn"))
	require.Len(t, m.mails, 2)
	msg := m.mails[1]
	assert.Equal(t, mailer.TemplatePasswordReset, msg.Template)
	assert.Equal(t, u.Email(), msg.To)
	assert.Equal(t, map[string]any{"Name": "Lê Hương", "ExpiresMinutes": 30}, msg.Payload)
	assert.NotEmpty(t, msg.Secret)
	require.Len(t, m.resets, 2)
	assert.NotNil(t, m.resets[0].UsedAt, "token cũ bị vô hiệu khi xin token mới")
	assert.Equal(t, HashToken(msg.Secret), m.resets[1].TokenHash, "DB chỉ lưu hash của token gửi đi")
	assert.Equal(t, clk.Now().Add(30*time.Minute), m.resets[1].ExpiresAt)
}

func TestResetPasswordRoundTrip(t *testing.T) {
	svc, m, clk := newMemService(t)
	u := addUser(t, m, invitedStudent(t, clk.Now(), "minh.bui@gmail.com"), goodPassword)
	ctx := context.Background()
	_, err := svc.Login(ctx, "minh.bui@gmail.com", goodPassword, "", "")
	require.NoError(t, err)
	require.NoError(t, svc.ForgotPassword(ctx, "minh.bui@gmail.com"))
	token := m.mails[0].Secret

	cmd := ResetPasswordCmd{Token: token, NewPassword: "matkhaumoi1", ConfirmPassword: "matkhaumoi1"}
	require.NoError(t, svc.ResetPassword(ctx, cmd))
	got, _ := m.get(u.ID())
	assert.Equal(t, domain.UserActive, got.Status())
	assert.False(t, got.MustChangePassword())
	assert.True(t, got.PasswordHash().Verify("matkhaumoi1"))
	assert.Empty(t, m.sessions, "đặt lại mật khẩu thu hồi mọi phiên")

	requireAppErr(t, svc.ResetPassword(ctx, cmd), http.StatusBadRequest, apperr.CodeValidationFailed)
}

func TestResetPasswordRejectsBadInput(t *testing.T) {
	svc, _, _ := newMemService(t)
	ctx := context.Background()
	requireAppErr(t, svc.ResetPassword(ctx, ResetPasswordCmd{Token: "x", NewPassword: "matkhaumoi1", ConfirmPassword: "khac"}),
		http.StatusUnprocessableEntity, apperr.CodeValidationFailed)
	requireAppErr(t, svc.ResetPassword(ctx, ResetPasswordCmd{Token: "", NewPassword: "matkhaumoi1", ConfirmPassword: "matkhaumoi1"}),
		http.StatusBadRequest, apperr.CodeValidationFailed)
	requireAppErr(t, svc.ResetPassword(ctx, ResetPasswordCmd{Token: "khong-ton-tai", NewPassword: "matkhaumoi1", ConfirmPassword: "matkhaumoi1"}),
		http.StatusBadRequest, apperr.CodeValidationFailed)
}

func TestResetPasswordExpiredToken(t *testing.T) {
	svc, m, clk := newMemService(t)
	addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	ctx := context.Background()
	require.NoError(t, svc.ForgotPassword(ctx, "huong.le@goup.vn"))
	clk.Advance(31 * time.Minute)
	err := svc.ResetPassword(ctx, ResetPasswordCmd{Token: m.mails[0].Secret, NewPassword: "matkhaumoi1", ConfirmPassword: "matkhaumoi1"})
	require.ErrorIs(t, err, ErrResetTokenInvalid)
}

func TestDisableEnableUser(t *testing.T) {
	svc, m, clk := newMemService(t)
	admin := addUser(t, m, internalUser(t, clk.Now(), "quan.tran@goup.vn", domain.RoleAdmin), goodPassword)
	teacher := addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	ctx := context.Background()
	_, err := svc.Login(ctx, "huong.le@goup.vn", goodPassword, "", "")
	require.NoError(t, err)
	require.NoError(t, svc.ForgotPassword(ctx, "huong.le@goup.vn"))

	out, err := svc.DisableUser(ctx, admin, teacher.ID(), "req-1")
	require.NoError(t, err)
	assert.Equal(t, domain.UserDisabled, out.Status())
	assert.Empty(t, m.sessions, "vô hiệu hóa thu hồi mọi phiên")
	assert.NotNil(t, m.resets[0].UsedAt, "vô hiệu hóa vô hiệu token đặt lại")
	require.Len(t, m.audits, 1)
	e := m.audits[0]
	assert.Equal(t, audit.ActionUserDisabled, e.Action)
	assert.Equal(t, admin.ID(), *e.ActorID)
	assert.Equal(t, teacher.ID(), *e.TargetID)
	assert.Equal(t, "user", e.TargetType)
	assert.Equal(t, "req-1", e.RequestID)
	assert.Equal(t, map[string]any{"status": domain.UserActive}, e.Before)
	assert.Equal(t, map[string]any{"status": domain.UserDisabled}, e.After)

	_, err = svc.DisableUser(ctx, admin, teacher.ID(), "")
	require.ErrorIs(t, err, domain.ErrInvalidTransition)

	out, err = svc.EnableUser(ctx, admin, teacher.ID(), "req-2")
	require.NoError(t, err)
	assert.Equal(t, domain.UserActive, out.Status())
	require.Len(t, m.audits, 2)
	assert.Equal(t, audit.ActionUserEnabled, m.audits[1].Action)

	_, err = svc.DisableUser(ctx, admin, admin.ID(), "")
	require.ErrorIs(t, err, ErrAdminProtected)
	_, err = svc.EnableUser(ctx, admin, uuid.New(), "")
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestListUsersFiltersByRoleAndStatus(t *testing.T) {
	svc, m, clk := newMemService(t)
	addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	addUser(t, m, invitedStudent(t, clk.Now(), "minh.bui@gmail.com"), goodPassword)
	m.mu.Lock()
	m.users[uuid.New()] = UserSnapshot{Email: mustEmail(t, "bao.pham@goup.vn"), Role: domain.RoleTeacher, Status: domain.UserDisabled}
	m.mu.Unlock()
	ctx := context.Background()

	all, err := svc.ListUsers(ctx, domain.RoleTeacher, nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)
	active := domain.UserActive
	act, err := svc.ListUsers(ctx, domain.RoleTeacher, &active)
	require.NoError(t, err)
	require.Len(t, act, 1)
	assert.Equal(t, "huong.le@goup.vn", act[0].Email().String())
}

func TestProvisionStudentAndRotate(t *testing.T) {
	svc, m, clk := newMemService(t)
	ctx := context.Background()
	email := mustEmail(t, "son.trinh@gmail.com")

	u, tmp, created, err := svc.ProvisionStudent(ctx, nil, email, "Trịnh Sơn", clk.Now())
	require.NoError(t, err)
	assert.True(t, created)
	assert.Len(t, tmp.Reveal(), TemporaryPasswordLength)
	assert.Equal(t, domain.UserInvited, u.Status())
	assert.True(t, m.hashes[u.ID()].Verify(tmp.Reveal()))

	again, tmp2, created, err := svc.ProvisionStudent(ctx, nil, email, "Tên khác", clk.Now())
	require.NoError(t, err)
	assert.False(t, created)
	assert.True(t, tmp2.IsZero())
	assert.Equal(t, u.ID(), again.ID())

	_, _, _, err = svc.ProvisionStudent(ctx, nil, mustEmail(t, "moi@gmail.com"), " ", clk.Now())
	require.ErrorIs(t, err, ErrNameRequired)

	clk.Advance(time.Hour)
	rotated, err := svc.RotateTemporaryPassword(ctx, nil, again, clk.Now())
	require.NoError(t, err)
	assert.NotEqual(t, tmp.Reveal(), rotated.Reveal())
	assert.True(t, m.hashes[u.ID()].Verify(rotated.Reveal()))
	assert.False(t, m.hashes[u.ID()].Verify(tmp.Reveal()))
	assert.Equal(t, clk.Now().Add(72*time.Hour), *m.users[u.ID()].TempPasswordExpiresAt)

	require.NoError(t, again.ChangePassword(fakeHash("x"), clk.Now()))
	_, err = svc.RotateTemporaryPassword(ctx, nil, again, clk.Now())
	require.ErrorIs(t, err, ErrAlreadyActivated)
}

func TestCleanupDeletesOldAttemptsAndExpiredSessions(t *testing.T) {
	svc, m, clk := newMemService(t)
	now := clk.Now()
	m.attempts = []LoginAttempt{
		{EmailNormalized: "a@x.vn", AttemptedAt: now.Add(-31 * 24 * time.Hour)},
		{EmailNormalized: "a@x.vn", AttemptedAt: now.Add(-29 * 24 * time.Hour)},
	}
	old, fresh := uuid.New(), uuid.New()
	m.sessions[old] = Session{ID: old, ExpiresAt: now.Add(-time.Minute)}
	m.sessions[fresh] = Session{ID: fresh, ExpiresAt: now.Add(time.Minute)}

	require.NoError(t, svc.Cleanup(context.Background(), now))
	assert.Len(t, m.attempts, 1)
	assert.Len(t, m.sessions, 1)
	_, ok := m.sessions[fresh]
	assert.True(t, ok)
}

func TestServiceUsesCryptoRandByDefault(t *testing.T) {
	svc := NewService(ServiceDeps{})
	require.NotNil(t, svc.Rand)
	_, err := NewSessionToken(svc.Rand)
	require.NoError(t, err)
}
