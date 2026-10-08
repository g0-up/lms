package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func TestParseClassStatus(t *testing.T) {
	for _, s := range []string{"draft", "active", "ended"} {
		st, err := domain.ParseClassStatus(s)
		require.NoError(t, err)
		require.Equal(t, s, st.String())
	}
	for _, s := range []string{"", "closed", "Active"} {
		_, err := domain.ParseClassStatus(s)
		require.ErrorIs(t, err, domain.ErrInvalidStatus, "%q", s)
	}
}

func TestClassStatusTransitions(t *testing.T) {
	const (
		endedLocked = "Lớp đã kết thúc không thể thay đổi"
		onlyDraft   = "Chỉ lớp nháp mới có thể kích hoạt"
		oneWay      = "Chỉ chuyển trạng thái một chiều: nháp → đang chạy → đã kết thúc."
	)
	d, a, e := domain.ClassDraft, domain.ClassActive, domain.ClassEnded
	cases := []struct {
		from, to domain.ClassStatus
		ok       bool
		msg      string
	}{
		{d, d, false, oneWay},
		{d, a, true, ""},
		{d, e, false, oneWay},
		{a, d, false, oneWay},
		{a, a, false, onlyDraft},
		{a, e, true, ""},
		{e, d, false, endedLocked},
		{e, a, false, endedLocked},
		{e, e, false, endedLocked},
	}
	for _, c := range cases {
		require.Equal(t, c.ok, c.from.CanTransitionTo(c.to), "%s → %s", c.from, c.to)
		err := c.from.Transition(c.to)
		if c.ok {
			require.NoError(t, err, "%s → %s", c.from, c.to)
			continue
		}
		require.ErrorIs(t, err, domain.ErrInvalidTransition, "%s → %s", c.from, c.to)
		require.EqualError(t, err, c.msg, "%s → %s", c.from, c.to)
	}
}
