package media

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/platform/db"
)

// Repo lưu media_files.
type Repo interface {
	Create(ctx context.Context, ex db.Executor, m *MediaFile) error
	// Update ghi trạng thái upload (pending → ready).
	Update(ctx context.Context, ex db.Executor, m *MediaFile) error
	ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*MediaFile, error) // không có → ErrMediaNotFound
	// CanUserAccess: media gắn với học liệu của lớp mà user dạy, hoặc học (thành viên active, lớp active|ended); media
	// do chính user tải lên; media trong bản nộp của lớp mà user dạy. Admin được kiểm ở service.
	CanUserAccess(ctx context.Context, ex db.Executor, mediaID, userID uuid.UUID) (bool, error)
	// StudentUploadQuota khóa theo học viên đến hết transaction ex (upload song song của cùng học viên chờ nhau) rồi
	// đọc quyền tải và số document chưa gắn bản nộp còn tính vào hạn mức (xem QuotaWindow). ex phải là transaction.
	StudentUploadQuota(ctx context.Context, ex db.Executor, userID uuid.UUID, w QuotaWindow) (StudentQuota, error)
}

// QuotaWindow là mốc tạo file còn tính vào hạn mức: file ready tạo từ ReadySince, file pending tạo từ PendingSince
// (URL PUT còn hạn). File cũ hơn không giữ chỗ, nên học viên rời trang bỏ dở upload không bị khóa mãi.
type QuotaWindow struct {
	ReadySince   time.Time
	PendingSince time.Time
}

// StudentQuota là điều kiện upload của học viên: Enrolled khi đang là thành viên active của ít nhất một lớp active;
// Unattached là số document chưa thuộc bản nộp nào và còn trong QuotaWindow.
type StudentQuota struct {
	Enrolled   bool `db:"enrolled"`
	Unattached int  `db:"unattached"`
}

// Reader là phần media mà feature khác (stages) đọc để kiểm file của học liệu đã ready và đúng loại.
type Reader interface {
	ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*MediaFile, error)
}
