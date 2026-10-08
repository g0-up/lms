package domain

import (
	"regexp"
	"strings"
	"unicode"
)

// maxLessonKeyLen trùng CHECK ck_lessons_key.
const maxLessonKeyLen = 40

var lessonKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)

// LessonKey nối một học liệu giữa các phiên bản của cùng chặng (tiến độ đi theo khóa này).
type LessonKey string

// ParseLessonKey trim rồi kiểm s theo quy tắc của cột lessons.lesson_key.
func ParseLessonKey(s string) (LessonKey, error) {
	s = strings.TrimSpace(s)
	if !lessonKeyPattern.MatchString(s) {
		return "", ErrInvalidLessonKey
	}
	return LessonKey(s), nil
}

func (k LessonKey) String() string { return string(k) }

// vietnameseBase đổi chữ có dấu (đã hạ chữ thường) về chữ không dấu.
var vietnameseBase = func() map[rune]rune {
	groups := map[rune]string{
		'a': "àáạảãâầấậẩẫăằắặẳẵ",
		'e': "èéẹẻẽêềếệểễ",
		'i': "ìíịỉĩ",
		'o': "òóọỏõôồốộổỗơờớợởỡ",
		'u': "ùúụủũưừứựửữ",
		'y': "ỳýỵỷỹ",
		'd': "đ",
	}
	m := make(map[rune]rune, 80)
	for base, chars := range groups {
		for _, r := range chars {
			m[r] = base
		}
	}
	return m
}()

// Slugify sinh lesson_key gợi ý từ tiêu đề tiếng Việt: bỏ dấu, chữ thường, ký tự khác chữ/số thành "-",
// cắt còn tối đa 40 ký tự. Kết quả có thể rỗng hoặc quá ngắn (tiêu đề toàn ký hiệu); người gọi vẫn kiểm bằng
// ParseLessonKey trước khi lưu.
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if base, ok := vietnameseBase[r]; ok {
			r = base
		}
		switch {
		case unicode.Is(unicode.Mn, r):
			// Dấu kết hợp của chuỗi dạng NFD: bỏ, chữ gốc đã được ghi.
			continue
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	slug := b.String()
	if len(slug) > maxLessonKeyLen {
		slug = strings.TrimRight(slug[:maxLessonKeyLen], "-")
	}
	return slug
}
