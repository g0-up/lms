package mailer

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/secretbox"
)

// secretPayloadKeys là khóa payload bị cấm: bí mật chỉ đi qua Message.Secret.
var secretPayloadKeys = []string{"TempPassword", "Link", "Token"}

// Service là Enqueuer: chỉ kiểm, niêm phong bí mật và ghi outbox; không render (worker render lúc gửi).
type Service struct {
	repo  OutboxRepo
	box   *secretbox.Box
	clock clock.Clock
}

var _ Enqueuer = (*Service)(nil)

// NewService tạo Service. box nil thì Enqueue trả ErrNoSecretBox (ví dụ router dựng trong unit test).
func NewService(repo OutboxRepo, box *secretbox.Box, clk clock.Clock) *Service {
	return &Service{repo: repo, box: box, clock: clk}
}

// Enqueue ghi msg vào email_outbox (status queued, run_at = now) bằng ex của caller và trả id hàng outbox.
func (s *Service) Enqueue(ctx context.Context, ex db.Executor, msg Message) (uuid.UUID, error) {
	if !msg.Template.Valid() {
		return uuid.Nil, fmt.Errorf("%w: %q", ErrUnknownTemplate, msg.Template)
	}
	if msg.To.IsZero() {
		return uuid.Nil, fmt.Errorf("mailer: thiếu người nhận")
	}
	for _, k := range secretPayloadKeys {
		if _, ok := msg.Payload[k]; ok {
			return uuid.Nil, fmt.Errorf("%w: %s", ErrSecretInPayload, k)
		}
	}
	var sealed []byte
	if msg.Secret != "" {
		if s.box == nil {
			return uuid.Nil, ErrNoSecretBox
		}
		var err error
		if sealed, err = s.box.Seal([]byte(msg.Secret)); err != nil {
			return uuid.Nil, err
		}
	}
	payload := msg.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	m := &OutboxMessage{
		ID: ids.New(), ToEmail: msg.To.Display(), Template: msg.Template, Payload: payload, SecretEnc: sealed,
		Status: StatusQueued, RunAt: s.clock.Now(),
	}
	if err := s.repo.Insert(ctx, ex, m); err != nil {
		return uuid.Nil, err
	}
	return m.ID, nil
}
