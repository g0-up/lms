// Package storage nói chuyện với object storage S3-compatible (dev MinIO, prod Cloudflare R2) qua một interface chung.
package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

// ErrNotFound: object chưa có trên storage (ví dụ trình duyệt chưa PUT xong).
var ErrNotFound = errors.New("storage: object không tồn tại")

// Storage là phần storage mà feature media dùng; bucket private nên mọi truy cập của trình duyệt đi qua URL ký.
type Storage interface {
	// PresignPut ký URL PUT có kèm Content-Type; headers là các header client phải gửi đúng như đã ký.
	PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (url string, headers map[string]string, err error)
	// PresignGet ký URL GET; storage tự xử lý Range nên video tua được.
	PresignGet(ctx context.Context, key string, ttl time.Duration, opts GetOptions) (string, error)
	Stat(ctx context.Context, key string) (ObjectStat, error)
	Delete(ctx context.Context, key string) error
}

// Uploader ghi thẳng object từ server; chỉ seed dùng để nạp video mẫu.
type Uploader interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
}

// GetOptions là header phản hồi ký kèm URL GET. ContentType rỗng thì storage trả type đã lưu; AttachmentName khác
// rỗng thì trình duyệt tải về (Content-Disposition: attachment) thay vì mở trực tiếp.
type GetOptions struct {
	ContentType    string
	AttachmentName string
}

// ContentDisposition dựng giá trị Content-Disposition attachment với tên file mã hóa theo RFC 5987, rỗng khi không
// cần tải về.
func (o GetOptions) ContentDisposition() string {
	if o.AttachmentName == "" {
		return ""
	}
	return "attachment; filename*=UTF-8''" + encodeExtValue(o.AttachmentName)
}

// encodeExtValue percent-encode mọi byte ngoài attr-char của RFC 5987, nên tên file có dấu cách, dấu chấm phẩy hay
// ký tự tiếng Việt không làm vỡ header.
func encodeExtValue(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("!#$&+-.^_`|~", c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

// ObjectStat là metadata của object đã upload.
type ObjectStat struct {
	Size        int64
	ContentType string
	ETag        string
}
