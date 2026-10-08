package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"lms/api/internal/platform/config"
)

// cfg được nạp một lần trong PersistentPreRunE trước khi bất kỳ lệnh con nào chạy.
var cfg config.Config

var rootCmd = &cobra.Command{
	Use:           "lms",
	Short:         "GoUp LMS: API, migration, seed, worker",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(*cobra.Command, []string) error {
		loaded, err := config.Load()
		if err != nil {
			return fmt.Errorf("cấu hình không hợp lệ: %w", err)
		}
		cfg = loaded
		slog.SetDefault(newLogger(cfg.LogLevel))
		return nil
	},
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(serveCmd, migrateCmd, seedCmd, workerCmd, healthcheckCmd)
}

// Execute chạy CLI; lỗi được ghi thành một dòng log JSON để cùng định dạng với log vận hành.
func Execute() error {
	slog.SetDefault(newLogger("info"))
	err := rootCmd.Execute()
	if err != nil {
		slog.Error(err.Error())
	}
	return err
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
