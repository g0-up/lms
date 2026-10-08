package mailer

import (
	"context"
	"log/slog"
)

// LogSender chỉ ghi log người nhận và tiêu đề (không bao giờ ghi body vì body chứa mật khẩu tạm hoặc link đặt
// lại); dùng khi SMTP_HOST rỗng.
type LogSender struct {
	Logger *slog.Logger
}

var _ Sender = LogSender{}

// Send ghi một dòng log và luôn thành công.
func (s LogSender) Send(ctx context.Context, m RenderedMail) error {
	log := s.Logger
	if log == nil {
		log = slog.Default()
	}
	log.InfoContext(ctx, "mailer: LogSender bỏ qua gửi thật", "to", m.To, "subject", m.Subject)
	return nil
}
