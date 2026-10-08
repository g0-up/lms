package mailer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/secretbox"
)

// claimBatch là số hàng tối đa mỗi lần claim.
const claimBatch = 10

// cleanupEvery là chu kỳ chạy các tác vụ dọn dẹp định kỳ của worker.
const cleanupEvery = time.Hour

// errSecretOpen là last_error khi không mở được secret_enc (mất hoặc đổi OUTBOX_SECRET_KEY).
const errSecretOpen = "secretbox: open"

// Cleanup là tác vụ dọn dẹp định kỳ chạy kèm worker (ví dụ xóa login_attempts cũ).
type Cleanup interface {
	Cleanup(ctx context.Context, now time.Time) error
}

// WorkerConfig là phụ thuộc của Worker.
type WorkerConfig struct {
	Tx            db.Tx
	Repo          OutboxRepo
	Renderer      *Renderer
	Sender        Sender
	Box           *secretbox.Box // nil: hàng có secret_enc thất bại với errSecretOpen
	Clock         clock.Clock
	PublicBaseURL string
	PollInterval  time.Duration
	Cleanups      []Cleanup
	Logger        *slog.Logger
}

// Worker claim email tới hạn, render lúc gửi và gửi tuần tự. Gửi at-least-once: worker chết giữa chừng thì hàng
// được claim lại khi lease 2 phút hết hạn.
type Worker struct {
	cfg WorkerConfig
	log *slog.Logger
}

// NewWorker kiểm phụ thuộc bắt buộc và tạo Worker.
func NewWorker(cfg WorkerConfig) (*Worker, error) {
	if cfg.Tx == nil || cfg.Repo == nil || cfg.Renderer == nil || cfg.Sender == nil || cfg.Clock == nil {
		return nil, errors.New("mailer: worker thiếu Tx, Repo, Renderer, Sender hoặc Clock")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Worker{cfg: cfg, log: log.With("component", "mailer_worker")}, nil
}

// Run chạy RunOnce mỗi PollInterval và Cleanups mỗi giờ (lần đầu ngay khi khởi động) tới khi ctx bị hủy.
func (w *Worker) Run(ctx context.Context) error {
	tick := time.NewTicker(w.cfg.PollInterval)
	defer tick.Stop()
	var lastCleanup time.Time
	for {
		if now := w.cfg.Clock.Now(); lastCleanup.IsZero() || now.Sub(lastCleanup) >= cleanupEvery {
			w.cleanup(ctx, now)
			lastCleanup = now
		}
		if _, err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
			w.log.ErrorContext(ctx, "mailer: lượt gửi lỗi", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

func (w *Worker) cleanup(ctx context.Context, now time.Time) {
	for _, c := range w.cfg.Cleanups {
		if err := c.Cleanup(ctx, now); err != nil && ctx.Err() == nil {
			w.log.ErrorContext(ctx, "mailer: dọn dẹp lỗi", "err", err)
		}
	}
}

// RunOnce claim tối đa claimBatch hàng trong một transaction ngắn rồi gửi từng hàng ngoài transaction; mỗi kết quả
// ghi trong transaction riêng. Trả số hàng đã xử lý (gửi được hoặc ghi thất bại).
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	var claimed []*OutboxMessage
	err := w.cfg.Tx.Transact(ctx, func(tx db.Executor) error {
		var err error
		claimed, err = w.cfg.Repo.Claim(ctx, tx, claimBatch)
		return err
	})
	if err != nil {
		return 0, err
	}
	done := 0
	for _, m := range claimed {
		if ctx.Err() != nil {
			// Hàng còn lại giữ lease và được claim lại sau 2 phút.
			return done, ctx.Err()
		}
		if err := w.process(ctx, m); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

// process gửi một hàng và ghi kết quả. Lỗi trả về chỉ là lỗi ghi DB; lỗi gửi được ghi vào hàng.
func (w *Worker) process(ctx context.Context, m *OutboxMessage) error {
	log := w.log.With("outbox_id", m.ID, "template", string(m.Template))
	sendErr := w.send(ctx, m)
	now := w.cfg.Clock.Now()
	if sendErr == nil {
		if err := w.cfg.Tx.Transact(ctx, func(tx db.Executor) error {
			return w.cfg.Repo.MarkSent(ctx, tx, m.ID, now)
		}); err != nil {
			return err
		}
		m.MarkSent(now)
		log.InfoContext(ctx, "mailer: đã gửi")
		return nil
	}
	final := m.MarkFailedAttempt(sendErr.Error(), now)
	if err := w.cfg.Tx.Transact(ctx, func(tx db.Executor) error {
		return w.cfg.Repo.MarkFailedAttempt(ctx, tx, m)
	}); err != nil {
		return err
	}
	log.WarnContext(ctx, "mailer: gửi thất bại", "attempts", m.Attempts, "final", final, "err", *m.LastError)
	return nil
}

// send mở bí mật, render và gửi. Lỗi trả về không chứa body hay bí mật vì được lưu vào last_error.
func (w *Worker) send(ctx context.Context, m *OutboxMessage) error {
	data := make(map[string]any, len(m.Payload)+2)
	for k, v := range m.Payload {
		data[k] = v
	}
	if len(m.SecretEnc) > 0 {
		if w.cfg.Box == nil {
			return errors.New(errSecretOpen)
		}
		plain, err := w.cfg.Box.Open(m.SecretEnc)
		if err != nil {
			return errors.New(errSecretOpen)
		}
		secret := string(plain)
		switch m.Template {
		case TemplateInvite, TemplateResend:
			data["TempPassword"] = secret
		case TemplatePasswordReset:
			data["Link"] = w.cfg.PublicBaseURL + "/reset-password#token=" + secret
		case TemplateAdded:
			// added không mang bí mật; bỏ qua nếu có.
		}
	}
	if needsSecret(m.Template) && len(m.SecretEnc) == 0 {
		return fmt.Errorf("mailer: template %s thiếu secret_enc", m.Template)
	}
	// Lỗi template (missingkey=error) chỉ nêu tên khóa, không nêu giá trị.
	mail, err := w.cfg.Renderer.Render(m.Template, m.ToEmail, data)
	if err != nil {
		return err
	}
	return w.cfg.Sender.Send(ctx, mail)
}

func needsSecret(t TemplateName) bool {
	return t == TemplateInvite || t == TemplateResend || t == TemplatePasswordReset
}
