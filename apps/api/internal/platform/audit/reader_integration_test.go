//go:build integration

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/testdb"
)

func TestPGReaderLatest(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx := testdb.Tx(t, dbx)
	testdb.Fixture(t, tx, "admin_quan_tran")
	actor := uuid.MustParse(testdb.AdminQuanTranID)
	base := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	clk := &clock.Fake{T: base}
	rec := audit.PG{Clock: clk}
	for i, action := range []string{audit.ActionClassCreated, audit.ActionClassActivated, audit.ActionUserDisabled} {
		clk.T = base.Add(time.Duration(i) * time.Minute)
		require.NoError(t, rec.Record(ctx, tx, audit.Entry{
			ActorID: &actor, Action: action, TargetType: "class", TargetID: &actor,
			Before: map[string]any{"status": "draft"}, After: map[string]any{"status": "active"}, RequestID: "req-1",
		}))
	}
	require.NoError(t, rec.Record(ctx, tx, audit.Entry{Action: audit.ActionClassEnded, TargetType: "class"}))

	got, err := audit.PGReader{}.Latest(ctx, tx, 3)
	require.NoError(t, err)
	require.Len(t, got, 3)
	// Dòng ghi sau cùng mang clk.T của lần ghi thứ ba nên cùng at; id v7 lớn hơn đứng trước.
	require.Equal(t, audit.ActionClassEnded, got[0].Action)
	require.Nil(t, got[0].ActorID)
	require.Nil(t, got[0].Before)
	require.Equal(t, audit.ActionUserDisabled, got[1].Action)
	require.Equal(t, audit.ActionClassActivated, got[2].Action)
	require.Equal(t, actor, *got[1].ActorID)
	require.JSONEq(t, `{"status":"active"}`, string(got[1].After))
	require.Equal(t, "req-1", got[1].RequestID)
	require.True(t, base.Add(2*time.Minute).Equal(got[1].At))

	for _, bad := range []int{0, -1, audit.MaxLatest + 1} {
		_, err := audit.PGReader{}.Latest(ctx, tx, bad)
		require.Error(t, err, "limit %d", bad)
	}
}
