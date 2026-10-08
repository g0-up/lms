package storage

import (
	"context"
	"crypto/md5" //nolint:gosec // ETag giả lập theo kiểu S3, không dùng cho bảo mật
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"
)

// MemStorage là Storage trong bộ nhớ cho unit test; URL ký là URL giả, không trỏ tới server thật.
type MemStorage struct {
	mu      sync.Mutex
	objects map[string]ObjectStat
}

var (
	_ Storage  = (*MemStorage)(nil)
	_ Uploader = (*MemStorage)(nil)
)

// NewMemStorage tạo storage rỗng.
func NewMemStorage() *MemStorage {
	return &MemStorage{objects: map[string]ObjectStat{}}
}

func (s *MemStorage) PresignPut(_ context.Context, key, contentType string, size int64, ttl time.Duration) (string, map[string]string, error) {
	q := url.Values{"size": {fmt.Sprint(size)}, "ttl": {ttl.String()}, "X-Amz-Signature": {"mem"}}
	return "https://storage.mem/put/" + url.PathEscape(key) + "?" + q.Encode(), map[string]string{"Content-Type": contentType}, nil
}

func (s *MemStorage) PresignGet(_ context.Context, key string, ttl time.Duration, responseContentType string) (string, error) {
	q := url.Values{"ttl": {ttl.String()}, "response-content-type": {responseContentType}, "X-Amz-Signature": {"mem"}}
	return "https://storage.mem/get/" + url.PathEscape(key) + "?" + q.Encode(), nil
}

func (s *MemStorage) Stat(_ context.Context, key string) (ObjectStat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.objects[key]
	if !ok {
		return ObjectStat{}, ErrNotFound
	}
	return st, nil
}

func (s *MemStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

// Put giả lập trình duyệt PUT lên URL ký.
func (s *MemStorage) Put(_ context.Context, key string, r io.Reader, _ int64, contentType string) error {
	h := md5.New() //nolint:gosec // xem ghi chú import
	n, err := io.Copy(h, r)
	if err != nil {
		return fmt.Errorf("storage: đọc dữ liệu: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = ObjectStat{Size: n, ContentType: contentType, ETag: hex.EncodeToString(h.Sum(nil))}
	return nil
}
