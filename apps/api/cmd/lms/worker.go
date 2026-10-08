package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"lms/api/internal/features/identity"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Chạy worker nền (outbox email, dọn login_attempts) tới khi nhận SIGTERM",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runWorker(ctx)
	},
}

// runWorker gửi email outbox mỗi EMAIL_POLL_INTERVAL và dọn login_attempts/phiên hết hạn mỗi giờ. SMTP_HOST rỗng
// thì dùng LogSender (chỉ log người nhận và tiêu đề).
func runWorker(ctx context.Context) error {
	dbx, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = dbx.Close() }()

	box, err := mailer.NewSecretBox(cfg.OutboxSecretKey)
	if err != nil {
		return err
	}
	renderer, err := mailer.NewRenderer(cfg.Location)
	if err != nil {
		return err
	}
	var sender mailer.Sender = mailer.LogSender{Logger: slog.Default()}
	if cfg.SMTPHost != "" {
		if sender, err = mailer.NewSMTPSender(cfg); err != nil {
			return err
		}
	}
	clk := clock.Real{Loc: cfg.Location}
	cleanup := identity.NewService(identity.ServiceDeps{
		DB: dbx, Sessions: identity.PGSessionRepo{}, Attempts: identity.PGLoginAttemptRepo{}, Clock: clk, Cfg: cfg,
	})
	w, err := mailer.NewWorker(mailer.WorkerConfig{
		Tx: db.TxRunner{DB: dbx}, Repo: mailer.NewOutboxRepo(), Renderer: renderer, Sender: sender, Box: box,
		Clock: clk, PublicBaseURL: cfg.PublicBaseURL, PollInterval: cfg.EmailPollInterval,
		Cleanups: []mailer.Cleanup{cleanup}, Logger: slog.Default(),
	})
	if err != nil {
		return err
	}
	slog.Info("worker: bắt đầu", "poll_interval", cfg.EmailPollInterval.String(), "smtp", cfg.SMTPHost != "")
	err = w.Run(ctx)
	slog.Info("worker: dừng")
	return err
}
