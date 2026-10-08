package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func TestParseRole(t *testing.T) {
	for _, s := range []string{"admin", "teacher", "student"} {
		r, err := domain.ParseRole(s)
		require.NoError(t, err)
		require.Equal(t, s, r.String())
	}
	for _, s := range []string{"", "Admin", "root"} {
		_, err := domain.ParseRole(s)
		require.ErrorIs(t, err, domain.ErrInvalidRole, "%q", s)
	}
}

func TestParseMemberStatus(t *testing.T) {
	for _, s := range []string{"active", "dropped", "completed"} {
		st, err := domain.ParseMemberStatus(s)
		require.NoError(t, err)
		require.Equal(t, s, st.String())
	}
	require.Equal(t, domain.MemberStatus("completed"), domain.MemberCompleted)
	_, err := domain.ParseMemberStatus("left")
	require.ErrorIs(t, err, domain.ErrInvalidStatus)
}
