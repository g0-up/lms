package db_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/db"
)

// recorder là driver database/sql giả chỉ ghi lại begin/commit/rollback để kiểm luồng của Transact
// mà không cần Postgres; hành vi với dữ liệu thật nằm ở tx_integration_test.go.
type recorder struct {
	mu        sync.Mutex
	events    []string
	beginErr  error
	commitErr error
	rbErr     error
	opts      driver.TxOptions
}

func (r *recorder) add(e string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) Events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

func (r *recorder) Connect(context.Context) (driver.Conn, error) { return &fakeConn{r: r}, nil }
func (r *recorder) Driver() driver.Driver                        { return nil }

type fakeConn struct{ r *recorder }

func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (c *fakeConn) Close() error                        { return nil }
func (c *fakeConn) Begin() (driver.Tx, error)           { return nil, errors.New("use BeginTx") }

func (c *fakeConn) BeginTx(_ context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if c.r.beginErr != nil {
		return nil, c.r.beginErr
	}
	c.r.opts = opts
	c.r.add("begin")
	return &fakeTx{r: c.r}, nil
}

type fakeTx struct{ r *recorder }

func (t *fakeTx) Commit() error {
	t.r.add("commit")
	return t.r.commitErr
}

func (t *fakeTx) Rollback() error {
	t.r.add("rollback")
	return t.r.rbErr
}

func newFakeDB(t *testing.T, r *recorder) *sqlx.DB {
	t.Helper()
	dbx := sqlx.NewDb(sql.OpenDB(r), "pgx")
	t.Cleanup(func() { _ = dbx.Close() })
	return dbx
}

func TestTransactCommitsOnSuccess(t *testing.T) {
	r := &recorder{}
	called := false
	err := db.Transact(context.Background(), newFakeDB(t, r), func(tx db.Executor) error {
		called = true
		require.NotNil(t, tx)
		return nil
	})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, []string{"begin", "commit"}, r.Events())
}

func TestTransactRollsBackAndReturnsFnError(t *testing.T) {
	r := &recorder{}
	want := errors.New("nghiệp vụ lỗi")
	err := db.Transact(context.Background(), newFakeDB(t, r), func(db.Executor) error { return want })
	require.Same(t, want, err, "lỗi của fn trả nguyên, không bọc")
	require.Equal(t, []string{"begin", "rollback"}, r.Events())
}

func TestTransactJoinsRollbackError(t *testing.T) {
	r := &recorder{rbErr: errors.New("connection reset")}
	want := errors.New("nghiệp vụ lỗi")
	err := db.Transact(context.Background(), newFakeDB(t, r), func(db.Executor) error { return want })
	require.ErrorIs(t, err, want)
	require.ErrorContains(t, err, "db: rollback: connection reset")
}

func TestTransactRollsBackAndRepanics(t *testing.T) {
	r := &recorder{}
	require.PanicsWithValue(t, "boom", func() {
		_ = db.Transact(context.Background(), newFakeDB(t, r), func(db.Executor) error { panic("boom") })
	})
	require.Equal(t, []string{"begin", "rollback"}, r.Events())
}

func TestTransactBeginError(t *testing.T) {
	r := &recorder{beginErr: errors.New("pool closed")}
	err := db.Transact(context.Background(), newFakeDB(t, r), func(db.Executor) error {
		t.Fatal("fn không được chạy khi begin lỗi")
		return nil
	})
	require.ErrorContains(t, err, "db: begin")
	require.Empty(t, r.Events())
}

func TestTransactMapsCommitError(t *testing.T) {
	r := &recorder{commitErr: &pgconn.PgError{Code: "40001"}}
	err := db.Transact(context.Background(), newFakeDB(t, r), func(db.Executor) error { return nil })
	require.True(t, apperr.Is(err, apperr.CodeConflict), "lỗi commit (constraint deferred, serialization) đi qua pgerr.Map")
	require.Equal(t, []string{"begin", "commit"}, r.Events())
}

func TestTransactWithOptionsPassesIsolation(t *testing.T) {
	r := &recorder{}
	err := db.TransactWithOptions(context.Background(), newFakeDB(t, r),
		&sql.TxOptions{Isolation: sql.LevelSerializable}, func(db.Executor) error { return nil })
	require.NoError(t, err)
	require.Equal(t, driver.IsolationLevel(sql.LevelSerializable), r.opts.Isolation)
}
