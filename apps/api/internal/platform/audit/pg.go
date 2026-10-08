package audit

import (
	"context"
	"errors"
	"fmt"

	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
)

// PG ghi audit_logs bằng Executor của caller; id là uuid v7, at lấy từ Entry.At hoặc Clock nếu rỗng.
type PG struct {
	Clock clock.Clock
}

var _ Recorder = PG{}

const insertAudit = `INSERT INTO audit_logs (id, actor_id, action, target_type, target_id, before, after, request_id, at)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9)`

// Record kiểm action thuộc actions.go và target_type khác rỗng rồi INSERT. Lỗi Postgres được bọc bằng %w để caller
// rollback transaction nghiệp vụ và vẫn đưa được qua pgerr.Map.
func (p PG) Record(ctx context.Context, tx db.Executor, e Entry) error {
	if !IsKnownAction(e.Action) {
		return fmt.Errorf("audit: action không thuộc danh sách: %q", e.Action)
	}
	if e.TargetType == "" {
		return errors.New("audit: thiếu target_type")
	}
	if p.Clock == nil {
		return errors.New("audit: PG thiếu Clock")
	}
	before, err := jsonb(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonb(e.After)
	if err != nil {
		return err
	}
	at := e.At
	if at.IsZero() {
		at = p.Clock.Now()
	}
	var requestID any
	if e.RequestID != "" {
		requestID = e.RequestID
	}
	if _, err := tx.ExecContext(ctx, insertAudit,
		ids.New(), e.ActorID, e.Action, e.TargetType, e.TargetID, before, after, requestID, at,
	); err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}
