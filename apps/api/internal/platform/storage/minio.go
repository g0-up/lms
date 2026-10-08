package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"lms/api/internal/platform/config"
)

// Config là cấu hình kết nối S3. Endpoint là host[:port] API nội bộ; PublicEndpoint là URL trình duyệt thấy, dùng để
// ký vì chữ ký gắn với host.
type Config struct {
	Endpoint       string
	PublicEndpoint string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	UseSSL         bool
	// Transport thay http.Transport mặc định (test dùng để chứng minh presign không mở kết nối).
	Transport http.RoundTripper
}

// ConfigFrom lấy cấu hình S3 từ config ứng dụng.
func ConfigFrom(c config.Config) Config {
	return Config{
		Endpoint: c.S3Endpoint, PublicEndpoint: c.S3PublicEndpoint, Region: c.S3Region, Bucket: c.S3Bucket,
		AccessKey: c.S3AccessKey, SecretKey: c.S3SecretKey, UseSSL: c.S3UseSSL,
	}
}

// MinIO là Storage trên minio-go v7, dùng được cho MinIO và R2 (chỉ khác endpoint/region).
type MinIO struct {
	internal *minio.Client // gọi API từ server: Stat, Delete, Put
	public   *minio.Client // chỉ để ký URL cho trình duyệt
	bucket   string
}

var (
	_ Storage  = (*MinIO)(nil)
	_ Uploader = (*MinIO)(nil)
)

// NewMinIO tạo hai client; cả hai đặt Region cố định để presign không gọi GetBucketLocation qua mạng.
func NewMinIO(cfg Config) (*MinIO, error) {
	if cfg.Bucket == "" || cfg.Region == "" {
		return nil, errors.New("storage: thiếu bucket hoặc region")
	}
	internal, err := newClient(cfg, cfg.Endpoint, cfg.UseSSL)
	if err != nil {
		return nil, fmt.Errorf("storage: client nội bộ: %w", err)
	}
	pubHost, pubSecure, err := parsePublicEndpoint(cfg.PublicEndpoint)
	if err != nil {
		return nil, err
	}
	public, err := newClient(cfg, pubHost, pubSecure)
	if err != nil {
		return nil, fmt.Errorf("storage: client ký URL: %w", err)
	}
	return &MinIO{internal: internal, public: public, bucket: cfg.Bucket}, nil
}

func newClient(cfg Config, endpoint string, secure bool) (*minio.Client, error) {
	return minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       secure,
		Region:       cfg.Region,
		BucketLookup: minio.BucketLookupPath,
		Transport:    cfg.Transport,
	})
}

// parsePublicEndpoint tách URL công khai (http://localhost:9000) thành host và cờ https.
func parsePublicEndpoint(raw string) (host string, secure bool, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || strings.Trim(u.Path, "/") != "" {
		return "", false, errors.New("storage: S3_PUBLIC_ENDPOINT phải có dạng http(s)://host[:port]")
	}
	return u.Host, u.Scheme == "https", nil
}

func (m *MinIO) PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (string, map[string]string, error) {
	h := http.Header{}
	h.Set("Content-Type", contentType)
	h.Set("Content-Length", strconv.FormatInt(size, 10))
	u, err := m.public.PresignHeader(ctx, http.MethodPut, m.bucket, key, ttl, nil, h)
	if err != nil {
		return "", nil, fmt.Errorf("storage: ký URL upload: %w", err)
	}
	// Content-Length do trình duyệt tự đặt theo file; client chỉ cần gửi đúng Content-Type.
	return u.String(), map[string]string{"Content-Type": contentType}, nil
}

func (m *MinIO) PresignGet(ctx context.Context, key string, ttl time.Duration, responseContentType string) (string, error) {
	params := url.Values{}
	if responseContentType != "" {
		params.Set("response-content-type", responseContentType)
	}
	u, err := m.public.PresignedGetObject(ctx, m.bucket, key, ttl, params)
	if err != nil {
		return "", fmt.Errorf("storage: ký URL xem: %w", err)
	}
	return u.String(), nil
}

func (m *MinIO) Stat(ctx context.Context, key string) (ObjectStat, error) {
	info, err := m.internal.StatObject(ctx, m.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return ObjectStat{}, ErrNotFound
		}
		return ObjectStat{}, fmt.Errorf("storage: stat: %w", err)
	}
	return ObjectStat{Size: info.Size, ContentType: info.ContentType, ETag: info.ETag}, nil
}

func (m *MinIO) Delete(ctx context.Context, key string) error {
	if err := m.internal.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{}); err != nil && !isNotFound(err) {
		return fmt.Errorf("storage: xóa: %w", err)
	}
	return nil
}

func (m *MinIO) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if _, err := m.internal.PutObject(ctx, m.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("storage: upload: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.StatusCode == http.StatusNotFound || resp.Code == minio.NoSuchKey
}
