package db

import (
	"context"
	"errors"
	"fmt"
)

// ErrNoRowsAffected báo câu lệnh có điều kiện không chạm đúng một dòng.
var ErrNoRowsAffected = errors.New("db: no rows affected")

// ExecAffectOne chạy câu lệnh và trả ErrNoRowsAffected khi RowsAffected() != 1; lỗi thực thi trả nguyên để caller
// đưa qua pgerr.Map. Repository của aggregate có phiên bản bắt buộc dùng cho mọi UPDATE/DELETE/INSERT...SELECT có
// điều kiện trạng thái (ví dụ `WHERE id = $1 AND status = 'draft'`), vì database không có trigger giữ bất biến;
// caller dịch ErrNoRowsAffected thành domain.ErrVersionImmutable hoặc domain.ErrInvalidTransition.
func ExecAffectOne(ctx context.Context, ex Executor, query string, args ...any) error {
	res, err := ex.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: rows affected: %w", err)
	}
	switch {
	case n == 0:
		return ErrNoRowsAffected
	case n > 1:
		return fmt.Errorf("%w: chạm %d dòng thay vì 1", ErrNoRowsAffected, n)
	}
	return nil
}
