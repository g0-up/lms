//go:build integration

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/testdb"
)

type auditRow struct {
	ActorID    *uuid.UUID `db:"actor_id"`
	Action     string     `db:"action"`
	TargetType string     `db:"target_type"`
	TargetID   *uuid.UUID `db:"target_id"`
	Before     []byte     `db:"before"`
	After      []byte     `db:"after"`
	RequestID  *string    `db:"request_id"`
	At         time.Time  `db:"at"`
}

func TestPGRecordInCallerTransaction(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	at := time.Date(2026, 10, 5, 9, 30, 0, 0, time.UTC)
	rec := audit.PG{Clock: &clock.Fake{T: at}}
	actor := uuid.MustParse(testdb.AdminQuanTranID)
	target := uuid.MustParse(testdb.StudentAnNguyenID)

	t.Run("written and read back", func(t *testing.T) {
		tx := testdb.Tx(t, dbx)
		testdb.Fixture(t, tx, "admin_quan_tran")
		testdb.Fixture(t, tx, "student_an_nguyen")

		err := rec.Record(ctx, tx, audit.Entry{
			ActorID:    &actor,
			Action:     audit.ActionUserDisabled,
			TargetType: "user",
			TargetID:   &target,
			Before:     map[string]any{"status": "active", "password_hash": "$argon2id$secret"},
			After:      map[string]any{"status": "disabled"},
			RequestID:  "req-42",
		})
		require.NoError(t, err)

		var row auditRow
		require.NoError(t, sqlxGet(ctx, tx, &row))
		require.Equal(t, actor, *row.ActorID)
		require.Equal(t, "user.disabled", row.Action)
		require.Equal(t, "user", row.TargetType)
		require.Equal(t, target, *row.TargetID)
		require.JSONEq(t, `{"status":"active"}`, string(row.Before))
		require.JSONEq(t, `{"status":"disabled"}`, string(row.After))
		require.Equal(t, "req-42", *row.RequestID)
		require.True(t, at.Equal(row.At))
	})

	t.Run("nil payload and request id become NULL", func(t *testing.T) {
		tx := testdb.Tx(t, dbx)
		err := rec.Record(ctx, tx, audit.Entry{Action: audit.ActionClassEnded, TargetType: "class"})
		require.NoError(t, err)

		var row auditRow
		require.NoError(t, sqlxGet(ctx, tx, &row))
		require.Nil(t, row.ActorID)
		require.Nil(t, row.TargetID)
		require.Nil(t, row.Before)
		require.Nil(t, row.After)
		require.Nil(t, row.RequestID)
	})

	t.Run("explicit At wins over the clock", func(t *testing.T) {
		tx := testdb.Tx(t, dbx)
		shared := at.Add(-time.Hour)
		require.NoError(t, rec.Record(ctx, tx, audit.Entry{Action: audit.ActionClassEnded, TargetType: "class", At: shared}))

		var row auditRow
		require.NoError(t, sqlxGet(ctx, tx, &row))
		require.True(t, shared.Equal(row.At))
	})

	t.Run("rolled back with the caller transaction", func(t *testing.T) {
		tx, err := dbx.BeginTxx(ctx, nil)
		require.NoError(t, err)
		require.NoError(t, rec.Record(ctx, tx, audit.Entry{Action: audit.ActionStageCreated, TargetType: "stage"}))
		require.NoError(t, tx.Rollback())

		var n int
		require.NoError(t, dbx.GetContext(ctx, &n, `SELECT count(*) FROM audit_logs`))
		require.Zero(t, n)
	})
}

func sqlxGet(ctx context.Context, ex interface {
	QueryRowxContext(context.Context, string, ...any) *sqlx.Row
}, dst *auditRow) error {
	return ex.QueryRowxContext(ctx, `SELECT actor_id, action, target_type, target_id, before, after, request_id, at
		FROM audit_logs ORDER BY id DESC LIMIT 1`).StructScan(dst)
}
