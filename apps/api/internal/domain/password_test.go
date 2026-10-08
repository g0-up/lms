package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func TestValidatePassword(t *testing.T) {
	require.NoError(t, domain.ValidatePassword("matkhau8", 8))
	require.NoError(t, domain.ValidatePassword("mậtkhẩu8", 8), "đếm theo ký tự, không theo byte")
	require.NoError(t, domain.ValidatePassword(strings.Repeat("a", 128), 8))

	err := domain.ValidatePassword("short", 8)
	require.ErrorIs(t, err, domain.ErrInvalidPassword)
	require.EqualError(t, err, "Mật khẩu phải có ít nhất 8 ký tự")

	err = domain.ValidatePassword(strings.Repeat("a", 11), 12)
	require.EqualError(t, err, "Mật khẩu phải có ít nhất 12 ký tự")

	err = domain.ValidatePassword(strings.Repeat(" ", 10), 8)
	require.ErrorIs(t, err, domain.ErrInvalidPassword)
	require.EqualError(t, err, "Mật khẩu không được chỉ gồm khoảng trắng")

	err = domain.ValidatePassword(strings.Repeat("a", 129), 8)
	require.ErrorIs(t, err, domain.ErrInvalidPassword)
	require.EqualError(t, err, "Mật khẩu quá dài (tối đa 128 byte)")
}
