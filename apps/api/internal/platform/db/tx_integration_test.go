//go:build integration

package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
	"lms/api/internal/platform/testdb"
)

const insertMedia = `INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by)
VALUES ($1, 'image', $2, 'a.png', 'image/png', 10, 'ready', $3)`

func countMedia(t *testing.T, ex db.Executor, id string) int {
	t.Helper()
	var n int
	require.NoError(t, ex.QueryRowxContext(context.Background(), `SELECT count(*) FROM media_files WHERE id = $1`, id).Scan(&n))
	return n
}

func TestTransactAgainstPostgres(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	testdb.Fixture(t, dbx, "admin_quan_tran")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const failedID = "0199a000-0000-7000-8000-00000000f001"
	want := errors.New("nghiệp vụ lỗi")
	err := db.Transact(ctx, dbx, func(tx db.Executor) error {
		_, err := tx.ExecContext(ctx, insertMedia, failedID, "test/failed.png", testdb.AdminQuanTranID)
		require.NoError(t, err)
		return want
	})
	require.ErrorIs(t, err, want)
	require.Zero(t, countMedia(t, dbx, failedID), "fn lỗi → rollback")

	const panicID = "0199a000-0000-7000-8000-00000000f002"
	require.Panics(t, func() {
		_ = db.Transact(ctx, dbx, func(tx db.Executor) error {
			_, err := tx.ExecContext(ctx, insertMedia, panicID, "test/panic.png", testdb.AdminQuanTranID)
			require.NoError(t, err)
			panic("boom")
		})
	})
	require.Zero(t, countMedia(t, dbx, panicID), "panic → rollback")

	const okID = "0199a000-0000-7000-8000-00000000f003"
	err = db.Transact(ctx, dbx, func(tx db.Executor) error {
		_, err := tx.ExecContext(ctx, insertMedia, okID, "test/ok.png", testdb.AdminQuanTranID)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, 1, countMedia(t, dbx, okID), "thành công → commit")
}

// Constraint DEFERRABLE chỉ bị kiểm lúc COMMIT; Transact phải dịch lỗi đó qua pgerr.Map như lỗi của câu lệnh.
func TestTransactMapsDeferredConstraintAtCommit(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	testdb.Fixture(t, dbx, "stage_db_draft")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := db.Transact(ctx, dbx, func(tx db.Executor) error {
		if _, err := tx.ExecContext(ctx, "SET CONSTRAINTS "+pgerr.UqLessonsVersionPosition+" DEFERRED"); err != nil {
			return err
		}
		return db.ExecAffectOne(ctx, tx, `UPDATE lessons SET position = 1 WHERE id = $1`, testdb.LessonDBIndexV2ID)
	})
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae), "%v", err)
	require.Equal(t, apperr.CodeConflict, ae.Code)
	require.Equal(t, "Dữ liệu bị trùng trong phiên bản", ae.Message)

	var pos int
	require.NoError(t, dbx.QueryRowxContext(ctx, `SELECT position FROM lessons WHERE id = $1`, testdb.LessonDBIndexV2ID).Scan(&pos))
	require.Equal(t, 2, pos, "commit thất bại không để lại thay đổi")
}

func TestTxRunnerCommitsThroughTransact(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	testdb.Fixture(t, dbx, "admin_quan_tran")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const id = "0199a000-0000-7000-8000-00000000f010"
	var tx db.Tx = db.TxRunner{DB: dbx}
	err := tx.Transact(ctx, func(ex db.Executor) error {
		_, err := ex.ExecContext(ctx, insertMedia, id, "test/runner.png", testdb.AdminQuanTranID)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, 1, countMedia(t, dbx, id))
}
