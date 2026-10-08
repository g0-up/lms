package media

import (
	"context"

	"github.com/google/uuid"

	"lms/api/internal/platform/db"
)

// Repo lưu media_files.
type Repo interface {
	Create(ctx context.Context, ex db.Executor, m *MediaFile) error
	// Update ghi trạng thái upload (pending → ready).
	Update(ctx context.Context, ex db.Executor, m *MediaFile) error
	ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*MediaFile, error) // không có → ErrMediaNotFound
	// CanUserAccess: media gắn với học liệu của lớp mà user dạy, hoặc học (thành viên active, lớp active|ended).
	// Admin được kiểm ở service.
	CanUserAccess(ctx context.Context, ex db.Executor, mediaID, userID uuid.UUID) (bool, error)
}

// Reader là phần media mà feature khác (stages) đọc để kiểm file của học liệu đã ready và đúng loại.
type Reader interface {
	ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*MediaFile, error)
}
