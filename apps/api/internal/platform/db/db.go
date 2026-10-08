// Package db mở pool sqlx trên driver pgx v5 (database/sql) và chạy migration nhúng.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	maxOpenConns    = 20
	connMaxIdleTime = 5 * time.Minute
)

// Open tạo pool *sqlx.DB cho ứng dụng và ping để báo lỗi kết nối ngay khi khởi động.
// Không bao giờ đưa pool này cho golang-migrate (xem Migrate).
func Open(ctx context.Context, dsn string) (*sqlx.DB, error) {
	connCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		// Không bọc err gốc vì thông điệp của pgx có thể chứa DSN kèm mật khẩu.
		return nil, fmt.Errorf("db: DATABASE_URL không hợp lệ")
	}
	sqlDB := stdlib.OpenDB(*connCfg)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	dbx := sqlx.NewDb(sqlDB, "pgx")
	if err := dbx.PingContext(ctx); err != nil {
		_ = dbx.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return dbx, nil
}
