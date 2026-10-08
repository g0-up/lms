package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

const healthcheckTimeout = 2 * time.Second

// healthcheckCmd dùng cho HEALTHCHECK của image distroless (không có curl/wget).
var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "GET /healthz trên tiến trình serve cục bộ; exit 0 khi 200",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, port, err := net.SplitHostPort(cfg.HTTPAddr)
		if err != nil {
			return fmt.Errorf("healthcheck: HTTP_ADDR không hợp lệ: %w", err)
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), healthcheckTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort("127.0.0.1", port)+"/healthz", nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("healthcheck: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("healthcheck: /healthz trả %d", resp.StatusCode)
		}
		return nil
	},
}
