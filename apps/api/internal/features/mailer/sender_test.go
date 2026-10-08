package mailer

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/config"
)

func smtpConfig(env string) config.Config {
	return config.Config{
		AppEnv: env, SMTPHost: "127.0.0.1", SMTPPort: 1, MailFrom: "GoUp LMS <no-reply@goup.vn>",
		MailFailPattern: `^fail-`,
	}
}

func TestNewSMTPSenderRequiresHostAndFrom(t *testing.T) {
	cfg := smtpConfig(config.EnvDev)
	cfg.SMTPHost = ""
	_, err := NewSMTPSender(cfg)
	require.Error(t, err)
	cfg = smtpConfig(config.EnvDev)
	cfg.MailFrom = ""
	_, err = NewSMTPSender(cfg)
	require.Error(t, err)
}

func TestMailFailPatternOnlyAppliesInE2E(t *testing.T) {
	e2e, err := NewSMTPSender(smtpConfig(config.EnvE2E))
	require.NoError(t, err)
	require.NotNil(t, e2e.failPattern)
	err = e2e.Send(context.Background(), RenderedMail{To: "fail-an@example.com", Subject: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MAIL_FAIL_PATTERN")

	for _, env := range []string{config.EnvDev, config.EnvProduction} {
		s, err := NewSMTPSender(smtpConfig(env))
		require.NoError(t, err)
		assert.Nil(t, s.failPattern, "%s phải bỏ qua MAIL_FAIL_PATTERN", env)
	}

	bad := smtpConfig(config.EnvE2E)
	bad.MailFailPattern = "("
	_, err = NewSMTPSender(bad)
	require.Error(t, err)
	bad.AppEnv = config.EnvProduction
	_, err = NewSMTPSender(bad)
	require.NoError(t, err, "môi trường khác e2e không đọc biến này nên mẫu hỏng không làm hỏng khởi động")
}

func TestSMTPSenderReportsInvalidAddressesAndDialErrors(t *testing.T) {
	s, err := NewSMTPSender(smtpConfig(config.EnvDev))
	require.NoError(t, err)
	ctx := context.Background()
	require.Error(t, s.Send(ctx, RenderedMail{To: "khong hop le", Subject: "x"}))

	badFrom := smtpConfig(config.EnvDev)
	badFrom.MailFrom = "khong hop le"
	s2, err := NewSMTPSender(badFrom)
	require.NoError(t, err)
	require.Error(t, s2.Send(ctx, RenderedMail{To: "an@example.com"}))

	// cổng 1 trên loopback không có SMTP → lỗi dial, không treo
	err = s.Send(ctx, RenderedMail{To: "an@example.com", Subject: "x", Text: "t", HTML: "<p>h</p>"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "smtp: gửi")
}

func TestLogSenderLogsOnlyRecipientAndSubject(t *testing.T) {
	var buf bytes.Buffer
	s := LogSender{Logger: slog.New(slog.NewJSONHandler(&buf, nil))}
	require.NoError(t, s.Send(context.Background(), RenderedMail{
		To: "an@example.com", Subject: "[GoUp LMS] Đặt lại mật khẩu", Text: "bi-mat-text", HTML: "bi-mat-html",
	}))
	out := buf.String()
	assert.Contains(t, out, "an@example.com")
	assert.Contains(t, out, "Đặt lại mật khẩu")
	assert.NotContains(t, out, "bi-mat-text")
	assert.NotContains(t, out, "bi-mat-html")
}
