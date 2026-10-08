package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"lms/api/internal/platform/db"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Chạy migration SQL nhúng: up | down [--steps N] | version",
}

var migrateDownSteps int

func init() {
	up := &cobra.Command{
		Use:   "up",
		Short: "Áp mọi migration chưa chạy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMigrate(cmd.Context(), db.Up, 0)
		},
	}
	down := &cobra.Command{
		Use:   "down",
		Short: "Hoàn tác migration (mặc định tất cả; --steps N chỉ N bước)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if migrateDownSteps < 0 {
				return fmt.Errorf("--steps phải >= 0")
			}
			return runMigrate(cmd.Context(), db.Down, migrateDownSteps)
		},
	}
	down.Flags().IntVar(&migrateDownSteps, "steps", 0, "số migration cần hoàn tác (0 = tất cả)")
	version := &cobra.Command{
		Use:   "version",
		Short: "In version schema hiện tại",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printVersion(cmd.Context())
		},
	}
	migrateCmd.AddCommand(up, down, version)
}

func runMigrate(parent context.Context, direction string, steps int) error {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := db.Migrate(ctx, cfg.DatabaseURL, direction, steps); err != nil {
		return err
	}
	return printVersion(ctx)
}

// printVersion in `version N (dirty=false)`; database chưa có migration nào in `no migration` và không lỗi.
func printVersion(ctx context.Context) error {
	v, dirty, err := db.Version(ctx, cfg.DatabaseURL)
	if errors.Is(err, db.ErrNilVersion) {
		fmt.Println("no migration")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("version %d (dirty=%t)\n", v, dirty)
	return nil
}
