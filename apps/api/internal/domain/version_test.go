package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func TestVersionNo(t *testing.T) {
	n, err := domain.ParseVersionNo(1)
	require.NoError(t, err)
	require.Equal(t, domain.FirstVersionNo, n)
	require.Equal(t, 2, n.Next().Int())

	for _, bad := range []int{0, -1} {
		_, err := domain.ParseVersionNo(bad)
		require.ErrorIs(t, err, domain.ErrInvalidVersionNo)
	}
}

func TestParseVersionStatus(t *testing.T) {
	for _, s := range []string{"draft", "published", "archived"} {
		st, err := domain.ParseVersionStatus(s)
		require.NoError(t, err)
		require.Equal(t, s, st.String())
	}
	for _, s := range []string{"", "Draft", "deleted"} {
		_, err := domain.ParseVersionStatus(s)
		require.ErrorIs(t, err, domain.ErrInvalidStatus, "%q", s)
	}
}

func TestVersionStatusTransitions(t *testing.T) {
	const (
		archiveNotPublished = "Chỉ phiên bản đã phát hành mới có thể lưu trữ"
		publishedLocked     = "Phiên bản đã phát hành không thể sửa"
		publishNotDraft     = "Chỉ phát hành được bản nháp"
		archivedLocked      = "Phiên bản đã lưu trữ không thể thay đổi"
		generic             = "Chuyển trạng thái không hợp lệ"
	)
	d, p, a := domain.VersionDraft, domain.VersionPublished, domain.VersionArchived
	cases := []struct {
		from, to domain.VersionStatus
		ok       bool
		msg      string
	}{
		{d, d, false, generic},
		{d, p, true, ""},
		{d, a, false, archiveNotPublished},
		{p, d, false, publishedLocked},
		{p, p, false, publishNotDraft},
		{p, a, true, ""},
		{a, d, false, archivedLocked},
		{a, p, false, archivedLocked},
		{a, a, false, archivedLocked},
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

	require.False(t, domain.VersionStatus("bogus").CanTransitionTo(p))
	require.ErrorIs(t, domain.VersionStatus("bogus").Transition(p), domain.ErrInvalidTransition)
}

func TestVersionStatusIsMutable(t *testing.T) {
	require.True(t, domain.VersionDraft.IsMutable())
	require.False(t, domain.VersionPublished.IsMutable())
	require.False(t, domain.VersionArchived.IsMutable())
}
