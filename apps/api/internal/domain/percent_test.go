package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func TestPercent(t *testing.T) {
	cases := []struct{ done, total, want int }{
		{0, 0, 0},
		{1, 3, 33},
		{2, 3, 67},
		{1, 2, 50},
		{3, 3, 100},
		{5, 3, 100},
		{1, 8, 13}, // 12.5 làm tròn ra xa 0 như round(numeric) của Postgres
		{0, 5, 0},
		{-1, 5, 0},
		{3, -1, 0},
	}
	for _, c := range cases {
		require.Equal(t, c.want, domain.Percent(c.done, c.total), "Percent(%d, %d)", c.done, c.total)
	}
}
