package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func TestParseLessonKey(t *testing.T) {
	for _, s := range []string{"db-intro", "go1", " web-html ", strings.Repeat("a", 40)} {
		k, err := domain.ParseLessonKey(s)
		require.NoError(t, err, "%q", s)
		require.Equal(t, strings.TrimSpace(s), k.String())
	}
	for _, s := range []string{"", "a", "-intro", "DB-intro", "db_intro", "giới-thiệu", strings.Repeat("a", 41)} {
		_, err := domain.ParseLessonKey(s)
		require.ErrorIs(t, err, domain.ErrInvalidLessonKey, "%q", s)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Giới thiệu SQL":                   "gioi-thieu-sql",
		"Thiết kế bảng và khóa":            "thiet-ke-bang-va-khoa",
		"Đọc thêm: chỉ mục":                "doc-them-chi-muc",
		"  Stack   và   Queue  ":           "stack-va-queue",
		"Cài đặt và Hello World!":          "cai-dat-va-hello-world",
		"ỨNG DỤNG ĐẦU TIÊN":                "ung-dung-dau-tien",
		"Giói thiệu (NFD)":              "gioi-thieu-nfd",
		"***":                              "",
		strings.Repeat("Bài học dài ", 10): "bai-hoc-dai-bai-hoc-dai-bai-hoc-dai-bai",
		"Học Go 1.27 – phần 2":             "hoc-go-1-27-phan-2",
	}
	for in, want := range cases {
		got := domain.Slugify(in)
		require.Equal(t, want, got, "%q", in)
		require.LessOrEqual(t, len(got), 40)
	}
	k, err := domain.ParseLessonKey(domain.Slugify("Giới thiệu SQL"))
	require.NoError(t, err)
	require.Equal(t, domain.LessonKey("gioi-thieu-sql"), k)
}
