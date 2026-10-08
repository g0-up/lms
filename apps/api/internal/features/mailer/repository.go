package mailer

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
)

// Message là email cần gửi. Payload chỉ chứa dữ liệu không nhạy cảm (Name, ClassName, ClassCode, LoginURL, Email,
// ExpiresAt, ExpiresMinutes); Secret ("" nếu không có) được niêm phong vào secret_enc, không vào payload hay log.
type Message struct {
	To       domain.Email
	Template TemplateName
	Payload  map[string]any
	Secret   string
}

// Enqueuer ghi email vào outbox trong transaction của caller.
type Enqueuer interface {
	Enqueue(ctx context.Context, ex db.Executor, msg Message) (uuid.UUID, error)
}

// OutboxStatusReader đọc trạng thái gửi của các hàng outbox (feature classes hiển thị trạng thái lời mời).
type OutboxStatusReader interface {
	StatusByIDs(ctx context.Context, ex db.Executor, ids []uuid.UUID) (map[uuid.UUID]DeliveryStatus, error)
}

// OutboxRepo là truy cập bảng email_outbox.
type OutboxRepo interface {
	OutboxStatusReader
	Insert(ctx context.Context, ex db.Executor, m *OutboxMessage) error
	// Claim nhận tối đa limit hàng tới hạn (queued tới run_at, hoặc sending quá lease) và đặt lease 2 phút.
	// Lease hết hạn được claim lại mà không tăng attempts.
	Claim(ctx context.Context, ex db.Executor, limit int) ([]*OutboxMessage, error)
	MarkSent(ctx context.Context, ex db.Executor, id uuid.UUID, now time.Time) error
	// MarkFailedAttempt ghi attempts/status/run_at/last_error/secret_enc đã tính ở OutboxMessage.MarkFailedAttempt.
	MarkFailedAttempt(ctx context.Context, ex db.Executor, m *OutboxMessage) error
	// SupersedeQueued đánh dấu failed ('superseded') các hàng còn queued của toEmail thuộc templates và xóa bí mật;
	// classes gọi trong transaction xoay mật khẩu tạm để email cũ mang mật khẩu hết hiệu lực không được gửi.
	SupersedeQueued(ctx context.Context, ex db.Executor, toEmail string, templates []string, now time.Time) (int64, error)
}

// RenderedMail là email đã render, sẵn sàng gửi.
type RenderedMail struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Sender gửi một email (SMTP thật, log, hoặc fake trong test).
type Sender interface {
	Send(ctx context.Context, m RenderedMail) error
}
