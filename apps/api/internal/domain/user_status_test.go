package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func TestParseUserStatus(t *testing.T) {
	for _, s := range []string{"invited", "active", "disabled"} {
		st, err := domain.ParseUserStatus(s)
		require.NoError(t, err)
		require.Equal(t, s, st.String())
	}
	_, err := domain.ParseUserStatus("locked")
	require.ErrorIs(t, err, domain.ErrInvalidStatus)
}

func TestUserStatusTransitions(t *testing.T) {
	const (
		disabled = "Tài khoản đã bị vô hiệu hóa"
		generic  = "Không thể chuyển trạng thái tài khoản như vậy"
	)
	i, a, d := domain.UserInvited, domain.UserActive, domain.UserDisabled
	cases := []struct {
		from, to domain.UserStatus
		ok       bool
		msg      string
	}{
		{i, i, false, generic},
		{i, a, true, ""},
		{i, d, true, ""},
		{a, i, false, generic},
		{a, a, false, generic},
		{a, d, true, ""},
		{d, i, true, ""},
		{d, a, true, ""},
		{d, d, false, disabled},
	}
	for _, c := range cases {
		require.Equal(t, c.ok, c.from.CanTransitionTo(c.to), "%s → %s", c.from, c.to)
		err := c.from.Transition(c.to)
		if c.ok {
			require.NoError(t, err, "%s → %s", c.from, c.to)
			continue
		}
		require.ErrorIs(t, err, domain.ErrInvalidTransition, "%s → %s", c.from, c.to)
		require.EqualError(t, err, c.msg, "%s → %s", c.from, c.to)
	}
}

func TestUserStatusEnable(t *testing.T) {
	st, err := domain.UserDisabled.Enable(true)
	require.NoError(t, err)
	require.Equal(t, domain.UserInvited, st, "chưa đăng nhập lần nào: về invited, vẫn dùng mật khẩu tạm")

	st, err = domain.UserDisabled.Enable(false)
	require.NoError(t, err)
	require.Equal(t, domain.UserActive, st)

	for _, from := range []domain.UserStatus{domain.UserInvited, domain.UserActive} {
		st, err := from.Enable(true)
		require.ErrorIs(t, err, domain.ErrInvalidTransition)
		require.EqualError(t, err, "Tài khoản chưa bị vô hiệu hóa")
		require.Equal(t, from, st)
	}
}
