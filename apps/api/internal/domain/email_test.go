package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func TestParseEmail(t *testing.T) {
	e, err := domain.ParseEmail("  An.Nguyen@GoUp.vn ")
	require.NoError(t, err)
	require.Equal(t, "an.nguyen@goup.vn", e.String())
	require.Equal(t, "An.Nguyen@GoUp.vn", e.Display())
	require.False(t, e.IsZero())
	require.True(t, domain.Email{}.IsZero())

	long := strings.Repeat("a", 245) + "@goup.vn" // 253 ký tự
	_, err = domain.ParseEmail(long)
	require.NoError(t, err)

	invalid := []string{
		"",
		"   ",
		"an.nguyen.goup.vn",
		"an@nguyen@goup.vn",
		"an nguyen@goup.vn",
		"an@goup",
		"@goup.vn",
		strings.Repeat("a", 247) + "@goup.vn", // 255 ký tự
	}
	for _, s := range invalid {
		_, err := domain.ParseEmail(s)
		require.ErrorIs(t, err, domain.ErrInvalidEmail, "%q", s)
		require.EqualError(t, err, "Email không hợp lệ")
	}
}
