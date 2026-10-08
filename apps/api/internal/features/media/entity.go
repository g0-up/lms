// Package media là feature upload video/ảnh lên object storage qua URL ký và cấp URL xem ngắn hạn.
// Bucket private: server chỉ ký URL, file không đi qua API.
package media

import (
	"fmt"
	"mime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/storage"
)

// maxFileNameRunes giới hạn tên file gốc lưu để hiển thị.
const maxFileNameRunes = 255

// Lỗi của feature, thông điệp theo spec.
var (
	ErrUnsupportedType = domain.ErrInvalid.WithMsg("Định dạng không hỗ trợ.")
	ErrInvalidKind     = domain.ErrInvalid.WithMsg("Loại file không hợp lệ.")
	ErrInvalidFileName = domain.ErrInvalid.WithMsg("Tên file không hợp lệ.")
	ErrInvalidSize     = domain.ErrInvalid.WithMsg("Kích thước file không hợp lệ.")
	ErrNotUploaded     = domain.ErrInvalid.WithMsg("Chưa nhận được file. Tải lên lại.")
	ErrUploadMismatch  = domain.ErrInvalid.WithMsg("File đã tải lên không khớp thông tin khai báo. Tải lên lại.")
	ErrMediaNotFound   = domain.ErrNotFound.WithMsg("Không tìm thấy file.")
	ErrNotReady        = domain.ErrInvalidTransition.WithMsg("File chưa tải lên xong.")
)

// ErrTooLarge là lỗi vượt dung lượng, kèm giới hạn của loại file ("File vượt giới hạn 2 GB.").
func ErrTooLarge(limit int64) error {
	return domain.ErrInvalid.WithMsg(fmt.Sprintf("File vượt giới hạn %s.", humanSize(limit)))
}

// Kind là loại media.
type Kind string

// Các loại media.
const (
	KindVideo Kind = "video"
	KindImage Kind = "image"
)

// ParseKind đọc kind từ request.
func ParseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case KindVideo, KindImage:
		return k, nil
	}
	return "", ErrInvalidKind
}

// Status là trạng thái upload.
type Status string

// Các trạng thái media.
const (
	StatusPending Status = "pending"
	StatusReady   Status = "ready"
)

// allowedTypes là allowlist content type theo loại, kèm đuôi file dùng trong storage key (không lấy từ tên file
// người dùng gửi).
var allowedTypes = map[Kind]map[string]string{
	KindVideo: {"video/mp4": ".mp4"},
	KindImage: {"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif"},
}

// Limits là dung lượng tối đa theo loại (MAX_VIDEO_BYTES, MAX_IMAGE_BYTES).
type Limits struct {
	MaxVideoBytes int64
	MaxImageBytes int64
}

func (l Limits) max(k Kind) int64 {
	if k == KindVideo {
		return l.MaxVideoBytes
	}
	return l.MaxImageBytes
}

// MediaFile là một file trên storage; ready khi server đã Stat thấy object khớp khai báo.
type MediaFile struct {
	id          uuid.UUID
	storageKey  string
	kind        Kind
	fileName    string
	contentType string
	sizeBytes   int64
	status      Status
	uploadedBy  uuid.UUID
	createdAt   time.Time
	readyAt     *time.Time
}

// NewPendingUpload kiểm loại, content type, tên và dung lượng rồi tạo bản ghi pending với storage key
// media/{kind}/{yyyy}/{mm}/{id}{ext}.
func NewPendingUpload(id uuid.UUID, kind Kind, fileName, contentType string, size int64, by uuid.UUID, now time.Time, limits Limits) (*MediaFile, error) {
	types, ok := allowedTypes[kind]
	if !ok {
		return nil, ErrInvalidKind
	}
	ct := normalizeContentType(contentType)
	ext, ok := types[ct]
	if !ok {
		return nil, ErrUnsupportedType
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" || utf8.RuneCountInString(fileName) > maxFileNameRunes || strings.ContainsAny(fileName, "/\\\x00") {
		return nil, ErrInvalidFileName
	}
	if size <= 0 {
		return nil, ErrInvalidSize
	}
	if limit := limits.max(kind); size > limit {
		return nil, ErrTooLarge(limit)
	}
	key := fmt.Sprintf("media/%s/%04d/%02d/%s%s", kind, now.UTC().Year(), int(now.UTC().Month()), id, ext)
	return &MediaFile{
		id: id, storageKey: key, kind: kind, fileName: fileName, contentType: ct, sizeBytes: size,
		status: StatusPending, uploadedBy: by, createdAt: now,
	}, nil
}

// MarkReady xác nhận object trên storage khớp dung lượng và content type đã ký. Gọi lại khi đã ready là no-op.
func (m *MediaFile) MarkReady(stat storage.ObjectStat, now time.Time) error {
	if m.status == StatusReady {
		return nil
	}
	if stat.Size != m.sizeBytes || normalizeContentType(stat.ContentType) != m.contentType {
		return ErrUploadMismatch
	}
	m.status, m.readyAt = StatusReady, &now
	return nil
}

// ID là id media.
func (m *MediaFile) ID() uuid.UUID { return m.id }

// StorageKey là key object trên bucket.
func (m *MediaFile) StorageKey() string { return m.storageKey }

// Kind là loại media.
func (m *MediaFile) Kind() Kind { return m.kind }

// FileName là tên file gốc, chỉ để hiển thị.
func (m *MediaFile) FileName() string { return m.fileName }

// ContentType là content type đã chuẩn hóa.
func (m *MediaFile) ContentType() string { return m.contentType }

// SizeBytes là dung lượng khai báo (và đã khớp khi ready).
func (m *MediaFile) SizeBytes() int64 { return m.sizeBytes }

// Status là trạng thái upload.
func (m *MediaFile) Status() Status { return m.status }

// IsReady cho biết file đã upload xong.
func (m *MediaFile) IsReady() bool { return m.status == StatusReady }

// UploadedBy là admin đã upload.
func (m *MediaFile) UploadedBy() uuid.UUID { return m.uploadedBy }

// CreatedAt là lúc tạo bản ghi.
func (m *MediaFile) CreatedAt() time.Time { return m.createdAt }

// ReadyAt là lúc xác nhận upload, nil khi còn pending.
func (m *MediaFile) ReadyAt() *time.Time { return m.readyAt }

// normalizeContentType bỏ tham số và chữ hoa: "Video/MP4; codecs=x" → "video/mp4".
func normalizeContentType(ct string) string {
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(ct))
	}
	return mt
}

// humanSize in dung lượng tròn theo GB/MB như thông điệp spec ("2 GB", "10 MB").
func humanSize(n int64) string {
	const mib, gib = 1 << 20, 1 << 30
	switch {
	case n >= gib && n%gib == 0:
		return fmt.Sprintf("%d GB", n/gib)
	case n >= mib && n%mib == 0:
		return fmt.Sprintf("%d MB", n/mib)
	default:
		return fmt.Sprintf("%d byte", n)
	}
}
