// Package config đọc biến môi trường thành Config và từ chối khởi động khi thiếu hoặc sai.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Giá trị hợp lệ của APP_ENV.
const (
	EnvDev        = "dev"
	EnvE2E        = "e2e"
	EnvProduction = "production"
)

// outboxKeyBytes là độ dài khóa AES-256-GCM của OUTBOX_SECRET_KEY sau khi decode base64.
const outboxKeyBytes = 32

// minPasswordLength là sàn bảo mật của PASSWORD_MIN_LENGTH.
const minPasswordLength = 8

// Config là danh sách biến DUY NHẤT của toàn dự án. Phase sau chỉ tham chiếu field, không thêm biến.
type Config struct {
	AppEnv         string `env:"APP_ENV,required"` // allowlist: dev|e2e|production
	HTTPAddr       string `env:"HTTP_ADDR" envDefault:":8080"`
	PublicBaseURL  string `env:"PUBLIC_BASE_URL,required"`
	DatabaseURL    string `env:"DATABASE_URL,required"`
	MigrateOnStart bool   `env:"MIGRATE_ON_START" envDefault:"false"`
	LogLevel       string `env:"LOG_LEVEL" envDefault:"info"`
	AppTZ          string `env:"APP_TZ" envDefault:"Asia/Ho_Chi_Minh"`

	SessionTTL              time.Duration `env:"SESSION_TTL" envDefault:"12h"`
	CookieSecure            bool          `env:"COOKIE_SECURE" envDefault:"false"` // nguồn DUY NHẤT quyết định cờ Secure của cookie
	TrustedProxies          []string      `env:"TRUSTED_PROXIES" envSeparator:","` // mặc định rỗng; prod đặt CIDR mạng Caddy/Traefik
	PasswordMinLength       int           `env:"PASSWORD_MIN_LENGTH" envDefault:"8"`
	TempPasswordTTL         time.Duration `env:"TEMP_PASSWORD_TTL" envDefault:"72h"`
	ResetTokenTTL           time.Duration `env:"RESET_TOKEN_TTL" envDefault:"30m"`
	LoginMaxFailures        int           `env:"LOGIN_MAX_FAILURES" envDefault:"5"`
	LoginLockWindow         time.Duration `env:"LOGIN_LOCK_WINDOW" envDefault:"15m"`
	RateLimitLoginIPPerMin  int           `env:"RATE_LIMIT_LOGIN_IP_PER_MIN" envDefault:"300"`
	RateLimitForgotIPPerMin int           `env:"RATE_LIMIT_FORGOT_IP_PER_MIN" envDefault:"30"`

	SMTPHost          string        `env:"SMTP_HOST" envDefault:"localhost"`
	SMTPPort          int           `env:"SMTP_PORT" envDefault:"1025"`
	SMTPUser          string        `env:"SMTP_USER"`
	SMTPPassword      string        `env:"SMTP_PASSWORD"`
	MailFrom          string        `env:"MAIL_FROM" envDefault:"GoUp LMS <no-reply@localhost>"`
	MailFailPattern   string        `env:"MAIL_FAIL_PATTERN"` // chỉ có hiệu lực khi AppEnv == e2e
	EmailPollInterval time.Duration `env:"EMAIL_POLL_INTERVAL" envDefault:"5s"`
	OutboxSecretKey   string        `env:"OUTBOX_SECRET_KEY,required"` // base64 của 32 byte; AES-256-GCM cho email_outbox.secret_enc

	S3Endpoint       string        `env:"S3_ENDPOINT,required"`             // host[:port] API nội bộ (dev minio:9000, prod <account>.r2.cloudflarestorage.com)
	S3PublicEndpoint string        `env:"S3_PUBLIC_ENDPOINT,required"`      // URL trình duyệt thấy, dùng để ký
	S3Region         string        `env:"S3_REGION" envDefault:"us-east-1"` // R2: auto
	S3Bucket         string        `env:"S3_BUCKET" envDefault:"lms-media"`
	S3AccessKey      string        `env:"S3_ACCESS_KEY,required"`
	S3SecretKey      string        `env:"S3_SECRET_KEY,required"`
	S3UseSSL         bool          `env:"S3_USE_SSL" envDefault:"false"`
	MediaURLTTL      time.Duration `env:"MEDIA_URL_TTL" envDefault:"2h"`
	MaxVideoBytes    int64         `env:"MAX_VIDEO_BYTES" envDefault:"2147483648"`
	MaxImageBytes    int64         `env:"MAX_IMAGE_BYTES" envDefault:"10485760"`

	StaleDays    int            `env:"STALE_DAYS" envDefault:"7"`
	SeedPassword string         `env:"SEED_PASSWORD"` // chỉ lệnh seed, dev/e2e
	Location     *time.Location `env:"-"`
}

// IsProduction cho biết tiến trình chạy ở production.
func (c Config) IsProduction() bool { return c.AppEnv == EnvProduction }

// IsE2E cho biết tiến trình chạy trong stack E2E.
func (c Config) IsE2E() bool { return c.AppEnv == EnvE2E }

// SeedResetAllowed: chỉ dev và e2e được xóa dữ liệu bằng `lms seed --reset`.
func (c Config) SeedResetAllowed() bool { return c.AppEnv == EnvDev || c.AppEnv == EnvE2E }

// Load đọc env, chuẩn hóa và kiểm tra; trả lỗi gộp mọi vi phạm để vận hành sửa một lần.
func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return cfg, fmt.Errorf("đọc env: %w", withEnvKeys(err))
	}
	cfg.normalize()
	return cfg, cfg.validate()
}

// withEnvKeys đổi lỗi parse theo tên field Go thành tên biến env để vận hành biết cần sửa biến nào.
// Chỉ field không phải chuỗi (số, bool, duration) mới có lỗi parse nên không lộ giá trị bí mật.
func withEnvKeys(err error) error {
	var agg env.AggregateError
	if !errors.As(err, &agg) {
		return err
	}
	t := reflect.TypeFor[Config]()
	out := make([]error, 0, len(agg.Errors))
	for _, e := range agg.Errors {
		var pe env.ParseError
		if errors.As(e, &pe) {
			if f, ok := t.FieldByName(pe.Name); ok {
				key, _, _ := strings.Cut(f.Tag.Get("env"), ",")
				out = append(out, fmt.Errorf("%s không hợp lệ: %w", key, pe.Err))
				continue
			}
		}
		out = append(out, e)
	}
	return errors.Join(out...)
}

func (c *Config) normalize() {
	c.AppEnv = strings.TrimSpace(c.AppEnv)
	c.PublicBaseURL = strings.TrimRight(strings.TrimSpace(c.PublicBaseURL), "/")
	c.LogLevel = strings.ToLower(strings.TrimSpace(c.LogLevel))
	c.OutboxSecretKey = strings.TrimSpace(c.OutboxSecretKey)
	proxies := c.TrustedProxies[:0]
	for _, p := range c.TrustedProxies {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	c.TrustedProxies = proxies
}

func (c *Config) validate() error {
	var errs []error

	// Tag `required` của caarlos0/env chỉ kiểm biến có mặt; `KEY=` trong file .env vẫn lọt nên kiểm rỗng ở đây.
	for _, f := range []struct{ key, val string }{
		{"DATABASE_URL", c.DatabaseURL},
		{"S3_ENDPOINT", c.S3Endpoint},
		{"S3_PUBLIC_ENDPOINT", c.S3PublicEndpoint},
		{"S3_ACCESS_KEY", c.S3AccessKey},
		{"S3_SECRET_KEY", c.S3SecretKey},
	} {
		if strings.TrimSpace(f.val) == "" {
			errs = append(errs, fmt.Errorf("%s không được để trống", f.key))
		}
	}

	switch c.AppEnv {
	case EnvDev, EnvE2E, EnvProduction:
	default:
		errs = append(errs, errors.New("APP_ENV phải là dev, e2e hoặc production"))
	}

	u, err := url.Parse(c.PublicBaseURL)
	publicURLValid := err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
	if !publicURLValid {
		errs = append(errs, errors.New("PUBLIC_BASE_URL phải là URL http(s) tuyệt đối"))
	}

	loc, err := time.LoadLocation(c.AppTZ)
	if err != nil {
		errs = append(errs, fmt.Errorf("APP_TZ không hợp lệ: %w", err))
	}
	c.Location = loc

	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL không hợp lệ: %q", c.LogLevel))
	}

	if c.PasswordMinLength < minPasswordLength {
		errs = append(errs, fmt.Errorf("PASSWORD_MIN_LENGTH phải >= %d", minPasswordLength))
	}

	// Không đưa giá trị khóa vào thông điệp lỗi để log không lộ secret.
	key, err := base64.StdEncoding.DecodeString(c.OutboxSecretKey)
	if err != nil || len(key) != outboxKeyBytes {
		errs = append(errs, fmt.Errorf("OUTBOX_SECRET_KEY phải là base64 của đúng %d byte (openssl rand -base64 32)", outboxKeyBytes))
	}

	if c.IsProduction() {
		if publicURLValid && u.Scheme != "https" {
			errs = append(errs, errors.New("production: PUBLIC_BASE_URL phải dùng https"))
		}
		if !c.CookieSecure {
			errs = append(errs, errors.New("production: COOKIE_SECURE phải là true"))
		}
		if !c.S3UseSSL {
			errs = append(errs, errors.New("production: S3_USE_SSL phải là true"))
		}
		if strings.TrimSpace(c.SMTPUser) == "" {
			errs = append(errs, errors.New("production: SMTP_USER là bắt buộc"))
		}
	}

	return errors.Join(errs...)
}
