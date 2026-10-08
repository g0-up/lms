// Package storage nói chuyện với object storage S3-compatible (dev MinIO, prod Cloudflare R2) qua một interface chung.
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound: object chưa có trên storage (ví dụ trình duyệt chưa PUT xong).
var ErrNotFound = errors.New("storage: object không tồn tại")

// Storage là phần storage mà feature media dùng; bucket private nên mọi truy cập của trình duyệt đi qua URL ký.
type Storage interface {
	// PresignPut ký URL PUT có kèm Content-Type; headers là các header client phải gửi đúng như đã ký.
	PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (url string, headers map[string]string, err error)
	// PresignGet ký URL GET; storage tự xử lý Range nên video tua được.
	PresignGet(ctx context.Context, key string, ttl time.Duration, responseContentType string) (string, error)
	Stat(ctx context.Context, key string) (ObjectStat, error)
	Delete(ctx context.Context, key string) error
}

// Uploader ghi thẳng object từ server; chỉ seed dùng để nạp video mẫu.
type Uploader interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
}

// ObjectStat là metadata của object đã upload.
type ObjectStat struct {
	Size        int64
	ContentType string
	ETag        string
}
