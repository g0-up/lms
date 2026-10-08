package ids_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/ids"
)

func TestNewIsVersion7AndUnique(t *testing.T) {
	seen := make(map[uuid.UUID]struct{}, 1000)
	for range 1000 {
		id := ids.New()
		require.Equal(t, uuid.Version(7), id.Version())
		_, dup := seen[id]
		require.False(t, dup)
		seen[id] = struct{}{}
	}
}

func TestNewIsMonotonic(t *testing.T) {
	prev := ids.New()
	for range 1000 {
		next := ids.New()
		require.Negative(t, bytes.Compare(prev[:], next[:]), "uuid v7 sinh liên tiếp phải tăng dần")
		require.Less(t, prev.String(), next.String(), "thứ tự chuỗi trùng thứ tự byte")
		prev = next
	}
}

func TestParse(t *testing.T) {
	id := ids.New()
	got, err := ids.Parse(id.String())
	require.NoError(t, err)
	require.Equal(t, id, got)

	upper, err := ids.Parse("0199A000-0000-7000-8000-00000000F001")
	require.NoError(t, err)
	require.Equal(t, "0199a000-0000-7000-8000-00000000f001", upper.String())

	for _, s := range []string{
		"",
		"abc",
		"urn:uuid:" + id.String(),
		"{" + id.String() + "}",
		strings.ReplaceAll(id.String(), "-", ""),
		"0199a000-0000-7000-8000-00000000f00g",
		"0199a000_0000_7000_8000_00000000f001",
	} {
		_, err := ids.Parse(s)
		require.ErrorIs(t, err, ids.ErrInvalid, "%q", s)
	}
}

func TestV7GeneratorReturnsVersion7(t *testing.T) {
	var g ids.Generator = ids.V7{}
	require.Equal(t, uuid.Version(7), g.New().Version())
}
