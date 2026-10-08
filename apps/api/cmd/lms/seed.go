package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/seed"
)

var (
	seedReset        bool
	seedUploadSample bool
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Nạp dữ liệu mẫu (chỉ dev|e2e); --reset xóa dữ liệu cũ trước",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runSeed(cmd.Context())
	},
}

func init() {
	seedCmd.Flags().BoolVar(&seedReset, "reset", false, "xóa dữ liệu hiện có trước khi seed (chỉ dev|e2e)")
	seedCmd.Flags().BoolVar(&seedUploadSample, "upload-sample", false, "tải video mẫu lên object storage")
}

// runSeed nạp dữ liệu mẫu và in số bản ghi mỗi bảng; không bao giờ in email hay mật khẩu.
func runSeed(parent context.Context) error {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := seed.Options{Reset: seedReset, UploadSample: seedUploadSample}
	// Kiểm tra môi trường và cờ trước khi mở kết nối để production bị từ chối kể cả khi DB không truy cập được.
	if err := seed.Validate(cfg, opts); err != nil {
		return err
	}

	dbx, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = dbx.Close() }()

	res, err := seed.Run(ctx, dbx, cfg, clock.Real{Loc: cfg.Location}, opts)
	if err != nil {
		return err
	}
	if res.AlreadySeeded {
		fmt.Println("đã seed")
		return nil
	}
	for _, c := range res.Counts {
		fmt.Printf("%s: %d\n", c.Table, c.Rows)
	}
	return nil
}
