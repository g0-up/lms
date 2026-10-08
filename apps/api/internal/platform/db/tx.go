package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"lms/api/internal/platform/db/pgerr"
)

// Executor là phần chung của *sqlx.DB và *sqlx.Tx. Repository nhận Executor nên không cần biết đang chạy trong
// transaction hay không.
type Executor interface {
	sqlx.ExtContext
	sqlx.PreparerContext
}

var (
	_ Executor = (*sqlx.DB)(nil)
	_ Executor = (*sqlx.Tx)(nil)
)

// Transact chạy fn trong một transaction mức cô lập mặc định (READ COMMITTED). Xem TransactWithOptions.
func Transact(ctx context.Context, dbx *sqlx.DB, fn func(tx Executor) error) error {
	return TransactWithOptions(ctx, dbx, nil, fn)
}

// TransactWithOptions mở transaction với opts (ví dụ Serializable khi clone phiên bản) rồi chạy fn:
//   - fn trả lỗi: rollback và trả nguyên lỗi của fn (ghép lỗi rollback bằng errors.Join nếu có);
//   - fn panic: rollback rồi panic lại với giá trị cũ;
//   - fn thành công: commit. Lỗi commit đi qua pgerr.Map vì constraint DEFERRABLE chỉ bị kiểm lúc commit.
func TransactWithOptions(ctx context.Context, dbx *sqlx.DB, opts *sql.TxOptions, fn func(tx Executor) error) (err error) {
	tx, err := dbx.BeginTxx(ctx, opts)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("db: rollback: %w", rbErr))
		}
		return err
	}
	committed = true
	if err := tx.Commit(); err != nil {
		return pgerr.Map(fmt.Errorf("db: commit: %w", err))
	}
	return nil
}

// Tx là unit of work mà service nhận qua constructor: service không giữ *sqlx.DB, unit test thay bằng fake.
type Tx interface {
	Transact(ctx context.Context, fn func(tx Executor) error) error
}

// TxRunner là Tx thật trên một *sqlx.DB, mức cô lập mặc định.
type TxRunner struct{ DB *sqlx.DB }

var _ Tx = TxRunner{}

// Transact chạy fn qua Transact của package.
func (r TxRunner) Transact(ctx context.Context, fn func(tx Executor) error) error {
	return Transact(ctx, r.DB, fn)
}
