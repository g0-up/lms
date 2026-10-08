package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/platform/db"
)

// MaxLatest là số dòng tối đa một lần Latest trả.
const MaxLatest = 100

// LoggedEntry là một dòng audit_logs đã ghi. Before/After giữ nguyên jsonb (nil khi NULL); nơi đọc tự dựng phần
// hiển thị, không trả thẳng ra API.
type LoggedEntry struct {
	ID         uuid.UUID
	At         time.Time
	ActorID    *uuid.UUID
	Action     string
	TargetType string
	TargetID   *uuid.UUID
	Before     json.RawMessage
	After      json.RawMessage
	RequestID  string
}

// Reader đọc nhật ký thao tác bằng Executor của caller.
type Reader interface {
	// Latest trả tối đa limit dòng mới nhất (at giảm dần, cùng at thì id giảm dần); limit ngoài 1..MaxLatest là lỗi.
	Latest(ctx context.Context, ex db.Executor, limit int) ([]LoggedEntry, error)
}

// PGReader là Reader trên Postgres.
type PGReader struct{}

var _ Reader = PGReader{}

type loggedRow struct {
	ID         uuid.UUID  `db:"id"`
	At         time.Time  `db:"at"`
	ActorID    *uuid.UUID `db:"actor_id"`
	Action     string     `db:"action"`
	TargetType string     `db:"target_type"`
	TargetID   *uuid.UUID `db:"target_id"`
	Before     []byte     `db:"before"`
	After      []byte     `db:"after"`
	RequestID  *string    `db:"request_id"`
}

// Latest đọc theo index ix_audit_logs_at.
func (PGReader) Latest(ctx context.Context, ex db.Executor, limit int) ([]LoggedEntry, error) {
	if limit < 1 || limit > MaxLatest {
		return nil, errors.New("audit: limit ngoài khoảng cho phép")
	}
	var rows []loggedRow
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT id, at, actor_id, action, target_type, target_id, before, after, request_id
		FROM audit_logs ORDER BY at DESC, id DESC LIMIT $1`, limit); err != nil {
		return nil, fmt.Errorf("audit: đọc nhật ký mới nhất: %w", err)
	}
	out := make([]LoggedEntry, 0, len(rows))
	for _, r := range rows {
		e := LoggedEntry{
			ID: r.ID, At: r.At, ActorID: r.ActorID, Action: r.Action, TargetType: r.TargetType, TargetID: r.TargetID,
			Before: rawOrNil(r.Before), After: rawOrNil(r.After),
		}
		if r.RequestID != nil {
			e.RequestID = *r.RequestID
		}
		out = append(out, e)
	}
	return out, nil
}

func rawOrNil(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}
