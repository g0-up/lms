package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func TestErrorWithMsgKeepsSentinelIdentity(t *testing.T) {
	err := domain.ErrInvalidTransition.WithMsg("Lý do cụ thể")
	require.Equal(t, "Lý do cụ thể", err.Error())
	require.Equal(t, domain.KindTransition, err.Kind)
	require.ErrorIs(t, err, domain.ErrInvalidTransition)
	require.NotErrorIs(t, err, domain.ErrInvalid)

	again := err.WithMsg("Lý do khác")
	require.ErrorIs(t, again, domain.ErrInvalidTransition, "WithMsg lồng nhau vẫn trỏ về sentinel gốc")

	wrapped := fmt.Errorf("service: %w", err)
	require.ErrorIs(t, wrapped, domain.ErrInvalidTransition)
	var de *domain.Error
	require.True(t, errors.As(wrapped, &de))
	require.Equal(t, "Lý do cụ thể", de.Msg)
}

func TestSentinelsAreDistinct(t *testing.T) {
	require.NotErrorIs(t, domain.ErrInvalidEmail, domain.ErrInvalidPassword)
	require.NotErrorIs(t, domain.ErrInvalidPassword.WithMsg("x"), domain.ErrInvalidEmail)
	require.NotErrorIs(t, domain.ErrNotFound, errors.New("Không tìm thấy dữ liệu"))
	require.ErrorIs(t, domain.ErrVersionImmutable, domain.ErrVersionImmutable)
}

func TestSentinelKinds(t *testing.T) {
	cases := map[*domain.Error]domain.Kind{
		domain.ErrInvalid:           domain.KindInvalid,
		domain.ErrInvalidEmail:      domain.KindInvalid,
		domain.ErrInvalidCode:       domain.KindInvalid,
		domain.ErrInvalidLessonKey:  domain.KindInvalid,
		domain.ErrInvalidVersionNo:  domain.KindInvalid,
		domain.ErrInvalidStatus:     domain.KindInvalid,
		domain.ErrInvalidRole:       domain.KindInvalid,
		domain.ErrInvalidPassword:   domain.KindInvalid,
		domain.ErrInvalidTransition: domain.KindTransition,
		domain.ErrNotFound:          domain.KindNotFound,
		domain.ErrConflict:          domain.KindConflict,
		domain.ErrForbidden:         domain.KindForbidden,
		domain.ErrVersionImmutable:  domain.KindImmutable,
		domain.ErrInUse:             domain.KindInUse,
	}
	for err, kind := range cases {
		require.Equal(t, kind, err.Kind, err.Msg)
		require.NotEmpty(t, err.Msg)
	}
	require.Equal(t, "Email không hợp lệ", domain.ErrInvalidEmail.Msg)
	require.Equal(t, "Phiên bản đã phát hành không thể sửa", domain.ErrVersionImmutable.Msg)
	require.Equal(t, "Không thể xóa vì đang được sử dụng", domain.ErrInUse.Msg)
}
