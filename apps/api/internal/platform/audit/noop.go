package audit

import (
	"context"

	"lms/api/internal/platform/db"
)

// Noop bỏ qua mọi Entry; dùng trong unit test không quan tâm nhật ký.
type Noop struct{}

var _ Recorder = Noop{}

// Record không làm gì.
func (Noop) Record(context.Context, db.Executor, Entry) error { return nil }
