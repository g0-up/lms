package domain

import (
	"regexp"
	"strings"
)

// maxEmailLen là độ dài tối đa của địa chỉ email (RFC 5321).
const maxEmailLen = 254

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Email là địa chỉ email đã kiểm tra. String() là dạng chuẩn hóa (cột email_normalized, khóa duy nhất),
// Display() là input đã trim giữ nguyên chữ hoa (cột email).
type Email struct {
	normalized string
	display    string
}

// ParseEmail trim, kiểm định dạng và độ dài, rồi chuẩn hóa về chữ thường.
func ParseEmail(s string) (Email, error) {
	display := strings.TrimSpace(s)
	if len(display) > maxEmailLen || !emailPattern.MatchString(display) {
		return Email{}, ErrInvalidEmail
	}
	return Email{normalized: strings.ToLower(display), display: display}, nil
}

// String trả email chuẩn hóa (chữ thường).
func (e Email) String() string { return e.normalized }

// Display trả email người dùng nhập (đã trim).
func (e Email) Display() string { return e.display }

// IsZero cho biết Email chưa được khởi tạo qua ParseEmail.
func (e Email) IsZero() bool { return e.normalized == "" }
