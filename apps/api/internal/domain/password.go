package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// maxPasswordBytes giới hạn đầu vào của hàm hash để chặn payload lớn làm tốn CPU.
const maxPasswordBytes = 128

var (
	errPasswordBlank   = ErrInvalidPassword.WithMsg("Mật khẩu không được chỉ gồm khoảng trắng")
	errPasswordTooLong = ErrInvalidPassword.WithMsg(fmt.Sprintf("Mật khẩu quá dài (tối đa %d byte)", maxPasswordBytes))
)

// ValidatePassword kiểm mật khẩu mới: ít nhất minLen ký tự, không toàn khoảng trắng, tối đa 128 byte.
func ValidatePassword(s string, minLen int) error {
	if utf8.RuneCountInString(s) < minLen {
		return ErrInvalidPassword.WithMsg(fmt.Sprintf("Mật khẩu phải có ít nhất %d ký tự", minLen))
	}
	if len(s) > maxPasswordBytes {
		return errPasswordTooLong
	}
	if strings.TrimSpace(s) == "" {
		return errPasswordBlank
	}
	return nil
}
