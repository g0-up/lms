package config

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testKey là khóa giả 32 byte chỉ dùng trong test.
var testKey = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", outboxKeyBytes)))

func validDevEnv() map[string]string {
	return map[string]string{
		"APP_ENV":            "dev",
		"PUBLIC_BASE_URL":    "http://localhost:5173/",
		"DATABASE_URL":       "postgres://lms:lms@localhost:5432/lms?sslmode=disable",
		"OUTBOX_SECRET_KEY":  testKey,
		"S3_ENDPOINT":        "localhost:9000",
		"S3_PUBLIC_ENDPOINT": "http://localhost:9000",
		"S3_ACCESS_KEY":      "access",
		"S3_SECRET_KEY":      "secret",
	}
}

func validProductionEnv() map[string]string {
	e := validDevEnv()
	e["APP_ENV"] = "production"
	e["PUBLIC_BASE_URL"] = "https://lms.example.vn"
	e["COOKIE_SECURE"] = "true"
	e["S3_USE_SSL"] = "true"
	e["SMTP_USER"] = "mailer"
	return e
}

// setEnv xóa mọi biến của Config khỏi môi trường rồi đặt bộ biến cho test;
// giá trị rỗng trong vars nghĩa là biến vắng mặt hẳn. t.Setenv khôi phục môi trường khi test xong.
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k := range configEnvKeys(t) {
		t.Setenv(k, "")
		require.NoError(t, os.Unsetenv(k))
	}
	for k, v := range vars {
		if v != "" {
			t.Setenv(k, v)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, validDevEnv())

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:5173", cfg.PublicBaseURL, "bỏ dấu / cuối")
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.False(t, cfg.MigrateOnStart)
	assert.Equal(t, 12*time.Hour, cfg.SessionTTL)
	assert.False(t, cfg.CookieSecure)
	assert.Empty(t, cfg.TrustedProxies)
	assert.Equal(t, 8, cfg.PasswordMinLength)
	assert.Equal(t, 72*time.Hour, cfg.TempPasswordTTL)
	assert.Equal(t, 30*time.Minute, cfg.ResetTokenTTL)
	assert.Equal(t, 5, cfg.LoginMaxFailures)
	assert.Equal(t, 15*time.Minute, cfg.LoginLockWindow)
	assert.Equal(t, 300, cfg.RateLimitLoginIPPerMin)
	assert.Equal(t, 30, cfg.RateLimitForgotIPPerMin)
	assert.Equal(t, "localhost", cfg.SMTPHost)
	assert.Equal(t, 1025, cfg.SMTPPort)
	assert.Equal(t, 5*time.Second, cfg.EmailPollInterval)
	assert.Equal(t, "us-east-1", cfg.S3Region)
	assert.Equal(t, "lms-media", cfg.S3Bucket)
	assert.Equal(t, 2*time.Hour, cfg.MediaURLTTL)
	assert.Equal(t, int64(2147483648), cfg.MaxVideoBytes)
	assert.Equal(t, int64(10485760), cfg.MaxImageBytes)
	assert.Equal(t, 7, cfg.StaleDays)
	require.NotNil(t, cfg.Location)
	assert.Equal(t, "Asia/Ho_Chi_Minh", cfg.Location.String())
}

func TestLoadTrustedProxies(t *testing.T) {
	e := validDevEnv()
	e["TRUSTED_PROXIES"] = " 172.16.0.0/12, ,10.0.0.0/8 "
	setEnv(t, e)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"172.16.0.0/12", "10.0.0.0/8"}, cfg.TrustedProxies)
}

func TestLoadValidation(t *testing.T) {
	cases := []struct {
		name    string
		base    func() map[string]string
		set     map[string]string
		wantErr string
	}{
		{name: "dev hợp lệ", base: validDevEnv},
		{name: "e2e hợp lệ", base: validDevEnv, set: map[string]string{"APP_ENV": "e2e"}},
		{name: "production hợp lệ", base: validProductionEnv},
		{name: "thiếu APP_ENV", base: validDevEnv, set: map[string]string{"APP_ENV": ""}, wantErr: "APP_ENV"},
		{name: "APP_ENV ngoài allowlist", base: validDevEnv, set: map[string]string{"APP_ENV": "staging"}, wantErr: "APP_ENV phải là dev, e2e hoặc production"},
		{name: "thiếu DATABASE_URL", base: validDevEnv, set: map[string]string{"DATABASE_URL": ""}, wantErr: "DATABASE_URL"},
		{name: "thiếu OUTBOX_SECRET_KEY", base: validDevEnv, set: map[string]string{"OUTBOX_SECRET_KEY": ""}, wantErr: "OUTBOX_SECRET_KEY"},
		{name: "thiếu S3_ACCESS_KEY", base: validDevEnv, set: map[string]string{"S3_ACCESS_KEY": ""}, wantErr: "S3_ACCESS_KEY"},
		{name: "PUBLIC_BASE_URL tương đối", base: validDevEnv, set: map[string]string{"PUBLIC_BASE_URL": "/app"}, wantErr: "PUBLIC_BASE_URL phải là URL http(s) tuyệt đối"},
		{name: "PUBLIC_BASE_URL sai scheme", base: validDevEnv, set: map[string]string{"PUBLIC_BASE_URL": "ftp://x"}, wantErr: "PUBLIC_BASE_URL phải là URL http(s) tuyệt đối"},
		{name: "PASSWORD_MIN_LENGTH dưới sàn", base: validDevEnv, set: map[string]string{"PASSWORD_MIN_LENGTH": "7"}, wantErr: "PASSWORD_MIN_LENGTH"},
		{name: "OUTBOX_SECRET_KEY không phải base64", base: validDevEnv, set: map[string]string{"OUTBOX_SECRET_KEY": "not base64!"}, wantErr: "OUTBOX_SECRET_KEY phải là base64"},
		{name: "OUTBOX_SECRET_KEY sai độ dài", base: validDevEnv, set: map[string]string{"OUTBOX_SECRET_KEY": base64.StdEncoding.EncodeToString([]byte("short"))}, wantErr: "OUTBOX_SECRET_KEY phải là base64"},
		{name: "APP_TZ sai", base: validDevEnv, set: map[string]string{"APP_TZ": "Mars/Base"}, wantErr: "APP_TZ"},
		{name: "LOG_LEVEL sai", base: validDevEnv, set: map[string]string{"LOG_LEVEL": "verbose"}, wantErr: "LOG_LEVEL"},
		{name: "SESSION_TTL sai định dạng", base: validDevEnv, set: map[string]string{"SESSION_TTL": "12 giờ"}, wantErr: "SESSION_TTL"},
		{name: "production cần https", base: validProductionEnv, set: map[string]string{"PUBLIC_BASE_URL": "http://lms.example.vn"}, wantErr: "production: PUBLIC_BASE_URL phải dùng https"},
		{name: "production cần COOKIE_SECURE", base: validProductionEnv, set: map[string]string{"COOKIE_SECURE": "false"}, wantErr: "production: COOKIE_SECURE"},
		{name: "production cần S3_USE_SSL", base: validProductionEnv, set: map[string]string{"S3_USE_SSL": "false"}, wantErr: "production: S3_USE_SSL"},
		{name: "production cần SMTP_USER", base: validProductionEnv, set: map[string]string{"SMTP_USER": ""}, wantErr: "production: SMTP_USER"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := tc.base()
			for k, v := range tc.set {
				e[k] = v
			}
			setEnv(t, e)

			_, err := Load()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestLoadRejectsRequiredPresentButEmpty(t *testing.T) {
	for _, key := range []string{"APP_ENV", "PUBLIC_BASE_URL", "DATABASE_URL", "OUTBOX_SECRET_KEY", "S3_ENDPOINT", "S3_PUBLIC_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY"} {
		t.Run(key, func(t *testing.T) {
			setEnv(t, validDevEnv())
			t.Setenv(key, "") // có mặt nhưng rỗng, như dòng `KEY=` trong .env

			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), key)
		})
	}
}

func TestValidationErrorNeverContainsSecret(t *testing.T) {
	e := validDevEnv()
	e["OUTBOX_SECRET_KEY"] = "c2VjcmV0LXZhbHVlLXRoYXQtbXVzdC1ub3QtbGVhaw=="
	setEnv(t, e)

	_, err := Load()
	require.Error(t, err)
	assert.NotContains(t, err.Error(), e["OUTBOX_SECRET_KEY"])
}

func TestHelpers(t *testing.T) {
	cases := []struct {
		env                      string
		production, e2e, resetOK bool
	}{
		{env: EnvDev, resetOK: true},
		{env: EnvE2E, e2e: true, resetOK: true},
		{env: EnvProduction, production: true},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			c := Config{AppEnv: tc.env}
			assert.Equal(t, tc.production, c.IsProduction())
			assert.Equal(t, tc.e2e, c.IsE2E())
			assert.Equal(t, tc.resetOK, c.SeedResetAllowed())
		})
	}
}
