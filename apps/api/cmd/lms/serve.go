package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"lms/api/internal/app"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/storage"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Chạy HTTP API (migrate trước nếu MIGRATE_ON_START=true)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return serve(ctx)
	},
}

func serve(ctx context.Context) error {
	if cfg.MigrateOnStart {
		if err := db.Migrate(ctx, cfg.DatabaseURL, db.Up, 0); err != nil {
			return err
		}
		slog.Info("migrate up xong khi khởi động")
	}

	dbx, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = dbx.Close() }()

	clk := clock.Real{Loc: cfg.Location}
	secrets, err := mailer.NewSecretBox(cfg.OutboxSecretKey)
	if err != nil {
		return err
	}
	store, err := storage.NewMinIO(storage.ConfigFrom(cfg))
	if err != nil {
		return err
	}
	engine := app.New(app.Deps{Cfg: cfg, DB: dbx, Clock: clk, Audit: audit.PG{Clock: clk}, Logger: slog.Default(), Secrets: secrets, Storage: store})
	// Lớp CSRF thứ nhất bọc toàn bộ engine; lớp thứ hai (X-Requested-With) gắn trên nhóm /api/v1.
	handler, err := httpx.CrossOriginProtection(engine, cfg.PublicBaseURL)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("http: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	return <-errCh
}
