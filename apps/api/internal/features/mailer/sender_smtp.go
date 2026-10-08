package mailer

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/wneessen/go-mail"

	"lms/api/internal/platform/config"
)

// smtpTimeout giới hạn thời gian kết nối và gửi một email.
const smtpTimeout = 30 * time.Second

// SMTPSender gửi email qua SMTP (Mailpit ở dev). TLS opportunistic; xác thực chỉ khi có SMTP_USER.
type SMTPSender struct {
	host        string
	port        int
	user        string
	password    string
	from        string
	failPattern *regexp.Regexp // chỉ đặt khi APP_ENV=e2e
}

var _ Sender = (*SMTPSender)(nil)

// NewSMTPSender dựng SMTPSender từ cấu hình. MAIL_FAIL_PATTERN chỉ được đọc khi APP_ENV=e2e: người nhận khớp mẫu
// bị từ chối ngay tại client để E2E kiểm được luồng gửi thất bại; môi trường khác bỏ qua biến này.
func NewSMTPSender(cfg config.Config) (*SMTPSender, error) {
	if cfg.SMTPHost == "" {
		return nil, fmt.Errorf("mailer: SMTP_HOST rỗng")
	}
	if cfg.MailFrom == "" {
		return nil, fmt.Errorf("mailer: MAIL_FROM rỗng")
	}
	s := &SMTPSender{host: cfg.SMTPHost, port: cfg.SMTPPort, user: cfg.SMTPUser, password: cfg.SMTPPassword, from: cfg.MailFrom}
	if cfg.IsE2E() && cfg.MailFailPattern != "" {
		re, err := regexp.Compile(cfg.MailFailPattern)
		if err != nil {
			return nil, fmt.Errorf("mailer: MAIL_FAIL_PATTERN không hợp lệ: %w", err)
		}
		s.failPattern = re
	}
	return s, nil
}

// Send dựng message multipart (text + HTML) và gửi trong một kết nối.
func (s *SMTPSender) Send(ctx context.Context, m RenderedMail) error {
	if s.failPattern != nil && s.failPattern.MatchString(m.To) {
		return fmt.Errorf("smtp: người nhận bị từ chối theo MAIL_FAIL_PATTERN")
	}
	msg := mail.NewMsg()
	if err := msg.From(s.from); err != nil {
		return fmt.Errorf("smtp: địa chỉ gửi không hợp lệ: %w", err)
	}
	if err := msg.To(m.To); err != nil {
		return fmt.Errorf("smtp: địa chỉ nhận không hợp lệ: %w", err)
	}
	msg.Subject(m.Subject)
	msg.SetMessageID()
	msg.SetDate()
	msg.SetBodyString(mail.TypeTextPlain, m.Text)
	msg.AddAlternativeString(mail.TypeTextHTML, m.HTML)

	opts := []mail.Option{mail.WithPort(s.port), mail.WithTLSPolicy(mail.TLSOpportunistic), mail.WithTimeout(smtpTimeout)}
	if s.user != "" {
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover), mail.WithUsername(s.user), mail.WithPassword(s.password))
	}
	c, err := mail.NewClient(s.host, opts...)
	if err != nil {
		return fmt.Errorf("smtp: cấu hình client: %w", err)
	}
	if err := c.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("smtp: gửi: %w", err)
	}
	return nil
}
