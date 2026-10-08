//go:build integration

// Package testdb cấp database Postgres thật (TEST_DATABASE_URL) cho integration test.
//
// Mọi package test tích hợp dùng chung một database lms_test, còn `go test ./...` chạy các package
// song song ở nhiều tiến trình. Open giữ một advisory lock cấp session trên một kết nối riêng tới hết
// tiến trình test, nên các package lần lượt dùng database (package sau chờ package trước thoát)
// thay vì giẫm lên nhau khi migrate down/up hoặc TRUNCATE.
package testdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/platform/db"
	"lms/api/migrations"
)

// lockKey là khóa advisory dành riêng cho việc tuần tự hóa test trên database lms_test.
const lockKey int64 = 0x6c6d735f74657374 // "lms_test"

const setupTimeout = 2 * time.Minute

var (
	once    sync.Once
	shared  *sqlx.DB
	initErr error
	// lockConn giữ advisory lock tới khi tiến trình test thoát; không bao giờ đóng tường minh.
	lockConn *sql.Conn
)

// URL trả TEST_DATABASE_URL; test bị skip khi biến này trống.
func URL(t testing.TB) string {
	t.Helper()
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("integration test cần TEST_DATABASE_URL (database lms_test)")
	}
	return url
}

// Open trả pool dùng chung của tiến trình test sau khi đã giữ khóa database và migrate up tới bản mới nhất.
func Open(t testing.TB) *sqlx.DB {
	t.Helper()
	url := URL(t)
	once.Do(func() { shared, initErr = setup(url) })
	if initErr != nil {
		t.Fatalf("testdb: %v", initErr)
	}
	// Mất kết nối giữ khóa nghĩa là advisory lock đã nhả và package khác có thể đang dùng database.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := lockConn.PingContext(ctx); err != nil {
		t.Fatalf("testdb: mất kết nối giữ khóa database test: %v", err)
	}
	return shared
}

func setup(url string) (*sqlx.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), setupTimeout)
	defer cancel()

	pool, err := db.Open(ctx, url)
	if err != nil {
		return nil, err
	}
	conn, err := pool.Conn(ctx)
	if err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("kết nối giữ khóa: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		_ = conn.Close()
		_ = pool.Close()
		return nil, fmt.Errorf("chờ khóa database test: %w", err)
	}
	lockConn = conn

	if err := db.Migrate(ctx, url, db.Up, 0); err != nil {
		return nil, err
	}
	return pool, nil
}

// Reset xóa dữ liệu mọi bảng của schema public (trừ schema_migrations) và đặt lại sequence.
func Reset(t testing.TB, dbx *sqlx.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var tables []string
	err := dbx.SelectContext(ctx, &tables,
		`SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'schema_migrations' ORDER BY tablename`)
	if err != nil {
		t.Fatalf("testdb: liệt kê bảng: %v", err)
	}
	if len(tables) == 0 {
		return
	}
	quoted := make([]string, len(tables))
	for i, name := range tables {
		quoted[i] = pgx.Identifier{"public", name}.Sanitize()
	}
	if _, err := dbx.ExecContext(ctx, "TRUNCATE "+strings.Join(quoted, ", ")+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("testdb: truncate: %v", err)
	}
}

// Tx mở transaction trên dbx và rollback khi test kết thúc, nên dữ liệu test (kể cả fixture) không lọt ra ngoài.
// Kết quả là db.Executor để truyền thẳng vào repository như một *sqlx.Tx thật.
func Tx(t testing.TB, dbx *sqlx.DB) db.Executor {
	t.Helper()
	// Context của BeginTxx sống cùng transaction nên không gắn timeout: tx phải sống tới t.Cleanup.
	tx, err := dbx.BeginTxx(context.Background(), nil)
	if err != nil {
		t.Fatalf("testdb: begin: %v", err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("testdb: rollback: %v", err)
		}
	})
	return tx
}

// LatestVersion trả version lớn nhất trong các file migration nhúng.
func LatestVersion(t testing.TB) uint {
	t.Helper()
	files, err := fs.Glob(migrations.FS, "*.up.sql")
	if err != nil {
		t.Fatalf("testdb: đọc migration nhúng: %v", err)
	}
	var latest uint
	for _, f := range files {
		prefix, _, _ := strings.Cut(f, "_")
		v, err := strconv.ParseUint(prefix, 10, 64)
		if err != nil {
			t.Fatalf("testdb: tên migration không hợp lệ %q", f)
		}
		latest = max(latest, uint(v))
	}
	return latest
}
