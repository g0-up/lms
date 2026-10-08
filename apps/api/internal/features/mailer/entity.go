// Package mailer là hàng đợi email outbox: feature khác ghi email_outbox trong transaction nghiệp vụ qua Enqueuer,
// worker `lms worker` claim, render template lúc gửi và gửi qua Sender (SMTP hoặc log). Bí mật (mật khẩu tạm,
// token đặt lại) chỉ nằm ở secret_enc (AES-256-GCM), bị xóa khi gửi xong hoặc thất bại lần cuối.
package mailer

import (
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// TemplateName là tên template email, khớp ck_email_outbox_template.
type TemplateName string

// Các template email.
const (
	TemplateInvite        TemplateName = "invite"
	TemplateAdded         TemplateName = "added"
	TemplateResend        TemplateName = "resend"
	TemplatePasswordReset TemplateName = "password_reset"
)

// Valid cho biết t là một trong 4 template cho phép.
func (t TemplateName) Valid() bool {
	switch t {
	case TemplateInvite, TemplateAdded, TemplateResend, TemplatePasswordReset:
		return true
	}
	return false
}

// Trạng thái hàng outbox, khớp ck_email_outbox_status.
const (
	StatusQueued  = "queued"
	StatusSending = "sending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
)

// MaxAttempts là số lần gửi tối đa của một email.
const MaxAttempts = 3

// maxLastError là độ dài tối đa (ký tự) của last_error.
const maxLastError = 500

// backoff[i] là thời gian chờ sau lần thất bại thứ i+1 (chỉ hai phần tử đầu được dùng khi MaxAttempts = 3).
var backoff = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}

// Lỗi khi enqueue.
var (
	ErrUnknownTemplate = errors.New("mailer: template không hợp lệ")
	ErrSecretInPayload = errors.New("mailer: payload không được chứa bí mật (TempPassword, Link, Token)")
	ErrNoSecretBox     = errors.New("mailer: chưa cấu hình OUTBOX_SECRET_KEY")
)

// OutboxMessage là một hàng email_outbox. Không có body: worker render template lúc gửi.
type OutboxMessage struct {
	ID          uuid.UUID
	ToEmail     string
	Template    TemplateName
	Payload     map[string]any // không chứa bí mật
	SecretEnc   []byte         // nil khi không có bí mật hoặc đã xóa
	Status      string
	Attempts    int
	RunAt       time.Time
	LockedUntil *time.Time
	LastError   *string
	SentAt      *time.Time
	CreatedAt   time.Time
}

// MarkSent đánh dấu đã gửi và xóa bí mật.
func (m *OutboxMessage) MarkSent(now time.Time) {
	m.Status, m.SentAt, m.SecretEnc, m.LockedUntil = StatusSent, &now, nil, nil
}

// MarkFailedAttempt ghi một lần gửi thất bại; đây là chỗ DUY NHẤT tăng Attempts. Đủ MaxAttempts thì failed và xóa
// bí mật (final=true), chưa đủ thì queued lại sau backoff 1m, 5m, 15m.
func (m *OutboxMessage) MarkFailedAttempt(errMsg string, now time.Time) (final bool) {
	m.Attempts++
	msg := truncateRunes(errMsg, maxLastError)
	m.LastError, m.LockedUntil = &msg, nil
	if m.Attempts >= MaxAttempts {
		m.Status, m.SecretEnc = StatusFailed, nil
		return true
	}
	m.Status = StatusQueued
	m.RunAt = now.Add(backoff[min(m.Attempts, len(backoff))-1])
	return false
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

// DeliveryStatus là trạng thái gửi đọc được bởi feature khác (FE gộp sending vào "Đang chờ gửi").
type DeliveryStatus struct {
	Status    string
	Attempts  int
	LastError *string
	SentAt    *time.Time
}
