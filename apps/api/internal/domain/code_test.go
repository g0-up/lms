package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func TestParseCode(t *testing.T) {
	cases := []struct {
		in   string
		kind domain.CodeKind
		ok   bool
	}{
		{"DB", domain.StageCode, true},
		{"GO_1", domain.StageCode, true},
		{" BASIC ", domain.CourseCode, true},
		{"WEB-2", domain.CourseCode, true},
		{strings.Repeat("A", 20), domain.StageCode, true},
		{"db", domain.StageCode, false},
		{"D", domain.StageCode, false},
		{strings.Repeat("A", 21), domain.StageCode, false},
		{"_DB", domain.CourseCode, false},
		{"ĐB", domain.CourseCode, false},
		{"basic01", domain.ClassCode, true},
		{"basic-01", domain.ClassCode, true},
		{strings.Repeat("a", 30), domain.ClassCode, true},
		{"BASIC01", domain.ClassCode, false},
		{"basic_01", domain.ClassCode, false},
		{"b", domain.ClassCode, false},
		{strings.Repeat("a", 31), domain.ClassCode, false},
		{"DB", domain.CodeKind(0), false},
	}
	for _, c := range cases {
		code, err := domain.ParseCode(c.in, c.kind)
		if c.ok {
			require.NoError(t, err, "%q", c.in)
			require.Equal(t, strings.TrimSpace(c.in), code.String())
			continue
		}
		require.ErrorIs(t, err, domain.ErrInvalidCode, "%q", c.in)
		require.Empty(t, code)
	}

	_, err := domain.ParseCode("db", domain.StageCode)
	require.EqualError(t, err, "Mã chỉ gồm chữ in hoa không dấu, số, dấu - hoặc _ (2–20 ký tự)")
	_, err = domain.ParseCode("BASIC01", domain.ClassCode)
	require.EqualError(t, err, "Mã lớp chỉ gồm chữ thường không dấu, số hoặc dấu - (2–30 ký tự)")
}
