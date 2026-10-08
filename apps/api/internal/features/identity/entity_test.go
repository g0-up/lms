package identity

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
)

var entityNow = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

func mustEmail(t *testing.T, s string) domain.Email {
	t.Helper()
	e, err := domain.ParseEmail(s)
	require.NoError(t, err)
	return e
}

// fakeHash là hash giả cho test entity; entity không tự verify nên không cần argon2 thật.
func fakeHash(s string) PasswordHash { return PasswordHashFromPHC("fake:" + s) }

func verifyAs(plain string) func(PasswordHash) bool {
	return func(h PasswordHash) bool { return h.PHC() == "fake:"+plain }
}

func newStudent(t *testing.T) *User {
	t.Helper()
	u, err := NewInvitedStudent(uuid.New(), mustEmail(t, "Minh.Bui@Gmail.com"), "  Bùi Minh  ", fakeHash("tmp"), entityNow, 72*time.Hour)
	require.NoError(t, err)
	return u
}

func TestNewInvitedStudent(t *testing.T) {
	u := newStudent(t)
	assert.Equal(t, "Bùi Minh", u.Name())
	assert.Equal(t, "minh.bui@gmail.com", u.Email().String())
	assert.Equal(t, domain.RoleStudent, u.Role())
	assert.True(t, u.IsStudent())
	assert.Equal(t, domain.UserInvited, u.Status())
	assert.True(t, u.MustChangePassword())
	require.NotNil(t, u.TempPasswordExpiresAt())
	assert.Equal(t, entityNow.Add(72*time.Hour), *u.TempPasswordExpiresAt())
	assert.Equal(t, "fake:tmp", u.PasswordHash().PHC())
	assert.NotEqual(t, uuid.Nil, u.ID())
}

func TestNewUserValidatesInput(t *testing.T) {
	email := mustEmail(t, "a@example.com")
	_, err := NewInvitedStudent(uuid.New(), domain.Email{}, "An", fakeHash("x"), entityNow, time.Hour)
	require.ErrorIs(t, err, domain.ErrInvalidEmail)
	_, err = NewInvitedStudent(uuid.New(), email, "   ", fakeHash("x"), entityNow, time.Hour)
	require.ErrorIs(t, err, ErrNameRequired)
	_, err = NewInvitedStudent(uuid.New(), email, strings.Repeat("ắ", maxNameRunes+1), fakeHash("x"), entityNow, time.Hour)
	require.ErrorIs(t, err, ErrNameTooLong)
	u, err := NewInvitedStudent(uuid.New(), email, strings.Repeat("ắ", maxNameRunes), fakeHash("x"), entityNow, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, maxNameRunes, len([]rune(u.Name())))
}

func TestNewInternalUser(t *testing.T) {
	email := mustEmail(t, "huong.le@goup.vn")
	for _, role := range []domain.Role{domain.RoleAdmin, domain.RoleTeacher} {
		u, err := NewInternalUser(uuid.New(), email, "Lê Hương", role, fakeHash("x"), entityNow)
		require.NoError(t, err)
		assert.Equal(t, role, u.Role())
		assert.Equal(t, domain.UserActive, u.Status())
		assert.False(t, u.MustChangePassword())
		assert.Nil(t, u.TempPasswordExpiresAt())
		assert.False(t, u.IsStudent())
	}
	_, err := NewInternalUser(uuid.New(), email, "Lê Hương", domain.RoleStudent, fakeHash("x"), entityNow)
	require.ErrorIs(t, err, domain.ErrInvalidRole)
	_, err = NewInternalUser(uuid.New(), email, "", domain.RoleTeacher, fakeHash("x"), entityNow)
	require.ErrorIs(t, err, ErrNameRequired)
}

func TestSnapshotRehydrateRoundTrip(t *testing.T) {
	u := newStudent(t)
	u.RecordLogin(entityNow.Add(time.Minute))
	snap := u.Snapshot()
	back := RehydrateUser(snap, u.PasswordHash())
	assert.Equal(t, snap, back.Snapshot())
	assert.Equal(t, u.PasswordHash(), back.PasswordHash())
}

func TestAuthenticateChecksPasswordFirst(t *testing.T) {
	expired := newStudent(t)
	afterExpiry := entityNow.Add(73 * time.Hour)

	disabled := newStudent(t)
	require.NoError(t, disabled.Disable(entityNow))

	cases := []struct {
		name string
		user *User
		now  time.Time
		pass string
		want error
	}{
		{"đúng mật khẩu, còn hạn", newStudent(t), entityNow, "tmp", nil},
		{"sai mật khẩu", newStudent(t), entityNow, "sai", ErrInvalidCredentials},
		{"tạm hết hạn đúng lúc hết hạn", newStudent(t), entityNow.Add(72 * time.Hour), "tmp", ErrTempPasswordExpired},
		{"tạm hết hạn", expired, afterExpiry, "tmp", ErrTempPasswordExpired},
		{"tạm hết hạn nhưng sai mật khẩu không lộ trạng thái", expired, afterExpiry, "sai", ErrInvalidCredentials},
		{"disabled đúng mật khẩu", disabled, entityNow, "tmp", ErrAccountDisabled},
		{"disabled sai mật khẩu không lộ trạng thái", disabled, entityNow, "sai", ErrInvalidCredentials},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.user.Authenticate(c.now, verifyAs(c.pass))
			if c.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, c.want)
		})
	}
}

func TestAuthenticateActiveUserIgnoresTempExpiry(t *testing.T) {
	u := newStudent(t)
	require.NoError(t, u.ChangePassword(fakeHash("moi"), entityNow))
	require.NoError(t, u.Authenticate(entityNow.Add(1000*time.Hour), verifyAs("moi")))
}

func TestChangePasswordActivatesInvitedUser(t *testing.T) {
	u := newStudent(t)
	later := entityNow.Add(time.Hour)
	require.NoError(t, u.ChangePassword(fakeHash("moi"), later))
	assert.Equal(t, domain.UserActive, u.Status())
	assert.False(t, u.MustChangePassword())
	assert.Nil(t, u.TempPasswordExpiresAt())
	assert.Equal(t, "fake:moi", u.PasswordHash().PHC())
	assert.Equal(t, later, u.Snapshot().UpdatedAt)

	// active đổi tiếp vẫn active
	require.NoError(t, u.ChangePassword(fakeHash("moi2"), later))
	assert.Equal(t, domain.UserActive, u.Status())
}

func TestResetPassword(t *testing.T) {
	u := newStudent(t)
	require.NoError(t, u.ResetPassword(fakeHash("moi"), entityNow))
	assert.Equal(t, domain.UserActive, u.Status())
	assert.False(t, u.MustChangePassword())

	d := newStudent(t)
	require.NoError(t, d.Disable(entityNow))
	err := d.ResetPassword(fakeHash("moi"), entityNow)
	require.ErrorIs(t, err, ErrAccountDisabled)
	assert.Equal(t, "fake:tmp", d.PasswordHash().PHC(), "tài khoản disabled không được đổi mật khẩu")
	require.ErrorIs(t, d.ChangePassword(fakeHash("moi"), entityNow), ErrAccountDisabled)
}

func TestDisableEnableTransitions(t *testing.T) {
	invited := newStudent(t)
	require.NoError(t, invited.Disable(entityNow))
	assert.Equal(t, domain.UserDisabled, invited.Status())
	require.NotNil(t, invited.Snapshot().DisabledAt)

	err := invited.Disable(entityNow)
	require.ErrorIs(t, err, domain.ErrInvalidTransition)

	// chưa đổi mật khẩu tạm → về invited
	require.NoError(t, invited.Enable(entityNow))
	assert.Equal(t, domain.UserInvited, invited.Status())
	assert.Nil(t, invited.Snapshot().DisabledAt)
	require.ErrorIs(t, invited.Enable(entityNow), domain.ErrInvalidTransition)

	active := newStudent(t)
	require.NoError(t, active.ChangePassword(fakeHash("moi"), entityNow))
	require.NoError(t, active.Disable(entityNow))
	require.NoError(t, active.Enable(entityNow))
	assert.Equal(t, domain.UserActive, active.Status())
}

func TestReinvite(t *testing.T) {
	u := newStudent(t)
	later := entityNow.Add(100 * time.Hour)
	require.NoError(t, u.Reinvite(fakeHash("tmp2"), later, 72*time.Hour))
	assert.Equal(t, "fake:tmp2", u.PasswordHash().PHC())
	assert.Equal(t, later.Add(72*time.Hour), *u.TempPasswordExpiresAt())
	assert.Equal(t, domain.UserInvited, u.Status())

	activated := newStudent(t)
	require.NoError(t, activated.ChangePassword(fakeHash("moi"), entityNow))
	require.ErrorIs(t, activated.Reinvite(fakeHash("tmp2"), later, time.Hour), ErrAlreadyActivated)
	require.ErrorIs(t, activated.Reinvite(fakeHash("tmp2"), later, time.Hour), domain.ErrConflict)

	disabled := newStudent(t)
	require.NoError(t, disabled.Disable(entityNow))
	require.ErrorIs(t, disabled.Reinvite(fakeHash("tmp2"), later, time.Hour), domain.ErrInvalidTransition)
}

func TestRecordLoginAndTouch(t *testing.T) {
	u := newStudent(t)
	at := entityNow.Add(time.Minute)
	u.RecordLogin(at)
	s := u.Snapshot()
	require.NotNil(t, s.LastLoginAt)
	require.NotNil(t, s.LastActiveAt)
	assert.Equal(t, at, *s.LastLoginAt)

	later := at.Add(time.Hour)
	u.Touch(later)
	assert.Equal(t, later, *u.Snapshot().LastActiveAt)
	assert.Equal(t, at, *u.Snapshot().LastLoginAt)
}

func TestSessionIsExpired(t *testing.T) {
	s := &Session{ExpiresAt: entityNow}
	assert.False(t, s.IsExpired(entityNow.Add(-time.Second)))
	assert.True(t, s.IsExpired(entityNow))
	assert.True(t, s.IsExpired(entityNow.Add(time.Second)))
}

func TestErrorsMapToContractCodes(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{ErrInvalidCredentials, http.StatusUnauthorized, apperr.CodeUnauthenticated},
		{ErrTempPasswordExpired, http.StatusUnauthorized, apperr.CodeTempPasswordExpired},
		{ErrAccountDisabled, http.StatusForbidden, apperr.CodeAccountDisabled},
		{ErrPasswordChange, http.StatusForbidden, apperr.CodePasswordChangeRequired},
		{ErrResetTokenInvalid, http.StatusBadRequest, apperr.CodeValidationFailed},
		{ErrPasswordMismatch, http.StatusUnprocessableEntity, apperr.CodeValidationFailed},
		{passwordTooShort(8), http.StatusUnprocessableEntity, apperr.CodeValidationFailed},
		{ErrEmailTaken, http.StatusConflict, apperr.CodeConflict},
		{ErrAdminProtected, http.StatusConflict, apperr.CodeConflict},
		{ErrUserNotFound, http.StatusNotFound, apperr.CodeNotFound},
		{errUserDisabled, http.StatusConflict, apperr.CodeInvalidTransition},
	}
	for _, c := range cases {
		var ae *apperr.Error
		if !errors.As(c.err, &ae) {
			ae = apperr.FromDomain(c.err)
		}
		require.NotNil(t, ae, c.err.Error())
		assert.Equal(t, c.status, ae.Status, c.err.Error())
		assert.Equal(t, c.code, ae.Code, c.err.Error())
	}
	short := passwordTooShort(8)
	require.ErrorIs(t, short, ErrPasswordTooShort)
	assert.Contains(t, short.Error(), "tối thiểu 8 ký tự")
}
