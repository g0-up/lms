package domain

import (
	"regexp"
	"strings"
)

// CodeKind chọn quy tắc mã: chặng và khóa học dùng chữ in hoa, lớp dùng chữ thường.
type CodeKind int

// Các loại mã.
const (
	StageCode CodeKind = iota + 1
	CourseCode
	ClassCode
)

// Regex trùng CHECK ck_stages_code, ck_courses_code, ck_classes_code của migration.
var (
	upperCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,19}$`)
	classCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,29}$`)
)

var (
	errInvalidUpperCode = ErrInvalidCode.WithMsg("Mã chỉ gồm chữ in hoa không dấu, số, dấu - hoặc _ (2–20 ký tự)")
	errInvalidClassCode = ErrInvalidCode.WithMsg("Mã lớp chỉ gồm chữ thường không dấu, số hoặc dấu - (2–30 ký tự)")
)

// Code là mã chặng, khóa học hoặc lớp đã kiểm tra.
type Code string

// ParseCode trim rồi kiểm s theo quy tắc của kind; không tự đổi chữ hoa/thường để người dùng thấy đúng mã mình nhập.
func ParseCode(s string, kind CodeKind) (Code, error) {
	s = strings.TrimSpace(s)
	switch kind {
	case StageCode, CourseCode:
		if !upperCodePattern.MatchString(s) {
			return "", errInvalidUpperCode
		}
	case ClassCode:
		if !classCodePattern.MatchString(s) {
			return "", errInvalidClassCode
		}
	default:
		return "", ErrInvalidCode
	}
	return Code(s), nil
}

func (c Code) String() string { return string(c) }
