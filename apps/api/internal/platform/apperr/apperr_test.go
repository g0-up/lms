package apperr_test

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
)

func TestCodesCoverContract(t *testing.T) {
	contract := []string{
		"VALIDATION_FAILED", "UNAUTHENTICATED", "PASSWORD_CHANGE_REQUIRED", "FORBIDDEN", "NOT_FOUND",
		"CONFLICT", "VERSION_IMMUTABLE", "DRAFT_EXISTS", "IN_USE", "INVALID_TRANSITION",
		"TEMP_PASSWORD_EXPIRED", "ACCOUNT_DISABLED", "TOO_MANY_ATTEMPTS", "RATE_LIMITED",
	}
	for _, code := range contract {
		msg, ok := apperr.DefaultMessages[code]
		require.True(t, ok, code)
		require.NotEmpty(t, msg, code)
		require.Equal(t, msg, apperr.Message(code))
	}
	require.Len(t, apperr.DefaultMessages, len(contract)+2, "chỉ thêm INTERNAL và METHOD_NOT_ALLOWED ngoài hợp đồng")
	require.Equal(t, "Bạn đã nhập sai quá nhiều lần. Thử lại sau 15 phút.", apperr.Message(apperr.CodeTooManyAttempts))
	require.Equal(t, "Lỗi hệ thống, vui lòng thử lại", apperr.Message("UNKNOWN_CODE"))
}

func TestConstructors(t *testing.T) {
	cases := []struct {
		err    *apperr.Error
		status int
		code   string
		msg    string
	}{
		{apperr.Validation(map[string]string{"email": "Bắt buộc"}), 400, apperr.CodeValidationFailed, "Dữ liệu không hợp lệ"},
		{apperr.NotFound("lớp"), 404, apperr.CodeNotFound, "Không tìm thấy lớp"},
		{apperr.Forbidden(), 403, apperr.CodeForbidden, "Bạn không có quyền thực hiện thao tác này"},
		{apperr.Unauthenticated(), 401, apperr.CodeUnauthenticated, "Vui lòng đăng nhập"},
		{apperr.Conflict("Mã đã tồn tại"), 409, apperr.CodeConflict, "Mã đã tồn tại"},
		{apperr.Internal(errors.New("boom")), 500, apperr.CodeInternal, "Lỗi hệ thống, vui lòng thử lại"},
		{apperr.New(http.StatusTooManyRequests, apperr.CodeRateLimited, "x"), 429, apperr.CodeRateLimited, "x"},
	}
	for _, c := range cases {
		require.Equal(t, c.status, c.err.Status, c.code)
		require.Equal(t, c.code, c.err.Code)
		require.Equal(t, c.msg, c.err.Message)
	}
	require.Equal(t, map[string]string{"email": "Bắt buộc"}, cases[0].err.Details)
	require.Equal(t, "NOT_FOUND: Không tìm thấy lớp", cases[1].err.Error())
	require.Equal(t, "INTERNAL: Lỗi hệ thống, vui lòng thử lại: boom", cases[5].err.Error())
}

func TestWrapUnwrapsThroughErrorsIsAs(t *testing.T) {
	cause := fmt.Errorf("repo: %w", sql.ErrNoRows)
	err := apperr.Wrap(cause, 404, apperr.CodeNotFound, "Không tìm thấy lớp")
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.Equal(t, cause, err.Cause())

	outer := fmt.Errorf("service: %w", err)
	var ae *apperr.Error
	require.True(t, errors.As(outer, &ae))
	require.Equal(t, apperr.CodeNotFound, ae.Code)
	require.True(t, apperr.Is(outer, apperr.CodeNotFound))
	require.False(t, apperr.Is(outer, apperr.CodeConflict))
	require.False(t, apperr.Is(errors.New("x"), apperr.CodeNotFound))
	require.False(t, apperr.Is(nil, apperr.CodeNotFound))
}

func TestFromDomainMapsEachKind(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
		msg    string
	}{
		{domain.ErrInvalidEmail, 400, apperr.CodeValidationFailed, "Email không hợp lệ"},
		{domain.ErrInvalidTransition.WithMsg("Chỉ phát hành được bản nháp"), 409, apperr.CodeInvalidTransition, "Chỉ phát hành được bản nháp"},
		{domain.ErrNotFound, 404, apperr.CodeNotFound, "Không tìm thấy dữ liệu"},
		{domain.ErrConflict, 409, apperr.CodeConflict, "Dữ liệu bị trùng"},
		{domain.ErrForbidden, 403, apperr.CodeForbidden, "Bạn không có quyền thực hiện thao tác này"},
		{domain.ErrVersionImmutable, 409, apperr.CodeVersionImmutable, "Phiên bản đã phát hành không thể sửa"},
		{domain.ErrInUse, 409, apperr.CodeInUse, "Không thể xóa vì đang được sử dụng"},
		{&domain.Error{Kind: domain.KindConflict}, 409, apperr.CodeConflict, "Dữ liệu bị trùng"},
		{fmt.Errorf("service: %w", domain.ErrVersionImmutable), 409, apperr.CodeVersionImmutable, "Phiên bản đã phát hành không thể sửa"},
		{&domain.Error{Kind: domain.Kind(99), Msg: "lạ"}, 500, apperr.CodeInternal, "Lỗi hệ thống, vui lòng thử lại"},
		{errors.New("pq: secret detail"), 500, apperr.CodeInternal, "Lỗi hệ thống, vui lòng thử lại"},
	}
	for _, c := range cases {
		got := apperr.FromDomain(c.err)
		require.Equal(t, c.status, got.Status, "%v", c.err)
		require.Equal(t, c.code, got.Code, "%v", c.err)
		require.Equal(t, c.msg, got.Message, "%v", c.err)
		require.ErrorIs(t, got, c.err, "cause được giữ để log")
	}

	require.Nil(t, apperr.FromDomain(nil))
	existing := apperr.Conflict("Mã đã tồn tại")
	require.Same(t, existing, apperr.FromDomain(fmt.Errorf("wrap: %w", existing)))
}
