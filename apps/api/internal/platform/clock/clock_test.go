package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/clock"
)

func TestRealUsesLocation(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)

	now := clock.Real{Loc: loc}.Now()
	require.Equal(t, loc, now.Location())
	require.WithinDuration(t, time.Now(), now, time.Second)

	require.WithinDuration(t, time.Now(), clock.Real{}.Now(), time.Second)
}

func TestFakeSetAndAdvance(t *testing.T) {
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	f := &clock.Fake{T: start}
	var c clock.Clock = f
	require.Equal(t, start, c.Now())
	require.Equal(t, start, c.Now(), "không tự trôi")

	f.Advance(15 * time.Minute)
	require.Equal(t, start.Add(15*time.Minute), c.Now())
	f.Advance(-time.Hour)
	require.Equal(t, start.Add(-45*time.Minute), c.Now())

	later := start.AddDate(0, 0, 3)
	f.Set(later)
	require.Equal(t, later, c.Now())
}
