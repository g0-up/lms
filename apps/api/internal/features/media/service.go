package media

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/storage"
)

// UploadTTL là hạn của URL PUT; video lớn vẫn kịp vì trình duyệt PUT thẳng lên storage.
const UploadTTL = 15 * time.Minute

// Principal là người dùng đã đăng nhập gọi use case.
type Principal struct {
	ID   uuid.UUID
	Role domain.Role
}

// IsAdmin cho biết người gọi là quản trị viên.
func (p Principal) IsAdmin() bool { return p.Role == domain.RoleAdmin }

// Config là giới hạn dung lượng và hạn URL xem (MEDIA_URL_TTL).
type Config struct {
	Limits
	URLTTL time.Duration
}

// Service là use case upload và cấp URL xem.
type Service struct {
	db    db.Executor
	repo  Repo
	store storage.Storage
	clock clock.Clock
	ids   ids.Generator
	cfg   Config
}

// NewService nối phụ thuộc; dbx dùng cho các thao tác một câu lệnh (không cần transaction).
func NewService(dbx db.Executor, repo Repo, store storage.Storage, clk clock.Clock, gen ids.Generator, cfg Config) *Service {
	return &Service{db: dbx, repo: repo, store: store, clock: clk, ids: gen, cfg: cfg}
}

// InitUploadCmd là thông tin file trình duyệt khai báo trước khi PUT.
type InitUploadCmd struct {
	Kind        string
	FileName    string
	ContentType string
	SizeBytes   int64
}

// UploadTicket là URL PUT đã ký cho một media pending.
type UploadTicket struct {
	MediaID   uuid.UUID
	UploadURL string
	ExpiresAt time.Time
}

// SignedURL là URL xem đã ký và thời điểm hết hạn để player ký lại trước khi hết.
type SignedURL struct {
	URL       string
	ExpiresAt time.Time
}

// InitUpload kiểm khai báo, ký URL PUT (kèm Content-Type và Content-Length) rồi lưu bản ghi pending. Chỉ admin.
func (s *Service) InitUpload(ctx context.Context, actor Principal, cmd InitUploadCmd) (UploadTicket, error) {
	if !actor.IsAdmin() {
		return UploadTicket{}, domain.ErrForbidden
	}
	kind, err := ParseKind(cmd.Kind)
	if err != nil {
		return UploadTicket{}, err
	}
	now := s.clock.Now()
	m, err := NewPendingUpload(s.ids.New(), kind, cmd.FileName, cmd.ContentType, cmd.SizeBytes, actor.ID, now, s.cfg.Limits)
	if err != nil {
		return UploadTicket{}, err
	}
	url, _, err := s.store.PresignPut(ctx, m.StorageKey(), m.ContentType(), m.SizeBytes(), UploadTTL)
	if err != nil {
		return UploadTicket{}, fmt.Errorf("media: ký URL upload: %w", err)
	}
	if err := s.repo.Create(ctx, s.db, m); err != nil {
		return UploadTicket{}, err
	}
	return UploadTicket{MediaID: m.ID(), UploadURL: url, ExpiresAt: now.Add(UploadTTL)}, nil
}

// CompleteUpload Stat object trên storage; chỉ khi object tồn tại và khớp khai báo mới chuyển ready. Chỉ admin.
func (s *Service) CompleteUpload(ctx context.Context, actor Principal, id uuid.UUID) (*MediaFile, error) {
	if !actor.IsAdmin() {
		return nil, domain.ErrForbidden
	}
	m, err := s.repo.ByID(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	if m.IsReady() {
		return m, nil
	}
	stat, err := s.store.Stat(ctx, m.StorageKey())
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotUploaded
	}
	if err != nil {
		return nil, fmt.Errorf("media: kiểm file đã upload: %w", err)
	}
	if err := m.MarkReady(stat, s.clock.Now()); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, s.db, m); err != nil {
		return nil, err
	}
	return m, nil
}

// SignedURL ký URL xem hạn MEDIA_URL_TTL. Admin xem mọi file; người khác chỉ xem file gắn với học liệu của lớp mình
// dạy hoặc học (CanUserAccess). Storage tự xử lý Range nên video tua được.
func (s *Service) SignedURL(ctx context.Context, user Principal, id uuid.UUID) (SignedURL, error) {
	m, err := s.repo.ByID(ctx, s.db, id)
	if err != nil {
		return SignedURL{}, err
	}
	if !user.IsAdmin() {
		ok, err := s.repo.CanUserAccess(ctx, s.db, id, user.ID)
		if err != nil {
			return SignedURL{}, err
		}
		if !ok {
			return SignedURL{}, domain.ErrForbidden
		}
	}
	if !m.IsReady() {
		return SignedURL{}, ErrNotReady
	}
	now := s.clock.Now()
	url, err := s.store.PresignGet(ctx, m.StorageKey(), s.cfg.URLTTL, m.ContentType())
	if err != nil {
		return SignedURL{}, fmt.Errorf("media: ký URL xem: %w", err)
	}
	return SignedURL{URL: url, ExpiresAt: now.Add(s.cfg.URLTTL)}, nil
}
