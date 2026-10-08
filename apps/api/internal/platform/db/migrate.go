package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // đăng ký driver "pgx" cho database/sql

	"lms/api/migrations"
)

// Hướng migration hợp lệ của Migrate.
const (
	Up   = "up"
	Down = "down"
)

// ErrNilVersion: database chưa áp migration nào (bảng schema_migrations rỗng). Không phải lỗi vận hành.
var ErrNilVersion = migrate.ErrNilVersion

// Migrate chạy migration nhúng theo direction. steps = 0 nghĩa là chạy hết (up tới mới nhất, down về rỗng);
// steps > 0 chỉ chạy đúng số bước đó. "Không có gì để chạy" không bị coi là lỗi.
//
// Hàm tự mở một *sql.DB riêng: driver pgx5 của golang-migrate dùng chung handle nhận vào và Close() đóng luôn nó,
// nên truyền pool của ứng dụng sẽ làm pool bị đóng. golang-migrate giữ advisory lock nên hai tiến trình
// migrate cùng lúc không giẫm nhau.
func Migrate(ctx context.Context, dsn string, direction string, steps int) error {
	if steps < 0 {
		return fmt.Errorf("migrate: steps phải >= 0, nhận %d", steps)
	}
	if direction != Up && direction != Down {
		return fmt.Errorf("migrate: hướng không hợp lệ %q (up|down)", direction)
	}
	m, err := newMigrator(ctx, dsn)
	if err != nil {
		return err
	}
	defer closeMigrator(m)

	// Ctrl-C/SIGTERM: dừng sau migration đang chạy thay vì bỏ ngang giữa chừng.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			select {
			case m.GracefulStop <- true:
			default:
			}
		case <-done:
		}
	}()

	switch {
	case direction == Up && steps == 0:
		err = m.Up()
	case direction == Up:
		err = m.Steps(steps)
	case steps == 0:
		err = m.Down()
	default:
		err = m.Steps(-steps)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate %s: %w", direction, err)
	}
	return ctx.Err()
}

// Version trả version hiện tại và cờ dirty; trả ErrNilVersion khi chưa có migration nào.
func Version(ctx context.Context, dsn string) (uint, bool, error) {
	m, err := newMigrator(ctx, dsn)
	if err != nil {
		return 0, false, err
	}
	defer closeMigrator(m)

	v, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, ErrNilVersion
		}
		return 0, false, fmt.Errorf("migrate version: %w", err)
	}
	return v, dirty, nil
}

func newMigrator(ctx context.Context, dsn string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: nguồn SQL nhúng: %w", err)
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("migrate: DATABASE_URL không hợp lệ")
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate: ping: %w", err)
	}
	drv, err := pgxv5.WithInstance(sqlDB, &pgxv5.Config{})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate: driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", drv)
	if err != nil {
		_ = drv.Close()
		return nil, fmt.Errorf("migrate: khởi tạo: %w", err)
	}
	return m, nil
}

// closeMigrator đóng nguồn iofs và driver (kéo theo *sql.DB riêng của migrate).
func closeMigrator(m *migrate.Migrate) {
	_, _ = m.Close()
}
