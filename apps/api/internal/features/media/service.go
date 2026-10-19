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

// MaxUnattachedDocuments là số file bài tập chưa gắn bản nộp tối đa của một học viên trong UnattachedWindow; chặn
// việc tải rác lên storage.
const MaxUnattachedDocuments = 10

// UnattachedWindow là khoảng một file ready chưa nộp còn giữ chỗ trong hạn mức; file pending giữ chỗ trong UploadTTL.
const UnattachedWindow = 24 * time.Hour

// Lỗi upload của học viên.
var (
	ErrStudentUploadKind = domain.ErrForbidden.WithMsg("Học viên chỉ tải lên được file bài tập.")
	ErrNotEnrolled       = domain.ErrForbidden.WithMsg("Bạn cần đang học một lớp để tải file bài tập.")
	ErrTooManyUnattached = domain.ErrInvalidTransition.WithMsg("Bạn có quá nhiều file chưa nộp. Hãy nộp hoặc chờ trước khi tải thêm.")
)

// Config là giới hạn dung lượng và hạn URL xem (MEDIA_URL_TTL). StudentTx chạy transaction cho upload của học viên
// (khóa và đếm file chưa gắn); nil thì chỉ admin upload được.
type Config struct {
	Limits
	URLTTL    time.Duration
	StudentTx db.Tx
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

// UploadTicket là URL PUT đã ký cho một media pending; Headers là các header trình duyệt phải gửi đúng như đã ký
// (Content-Type là type chuẩn server chọn, không phải type của file trên máy).
type UploadTicket struct {
	MediaID   uuid.UUID
	UploadURL string
	Headers   map[string]string
	ExpiresAt time.Time
}

// SignedURL là URL xem đã ký và thời điểm hết hạn để player ký lại trước khi hết.
type SignedURL struct {
	URL       string
	ExpiresAt time.Time
}

// canUpload: admin tải mọi loại; học viên chỉ tải file bài tập khi đã nối StudentTx.
func (s *Service) canUpload(actor Principal) bool {
	return actor.IsAdmin() || (actor.Role == domain.RoleStudent && s.cfg.StudentTx != nil)
}

// InitUpload kiểm khai báo, ký URL PUT (kèm Content-Type và Content-Length) rồi lưu bản ghi pending. Admin tải mọi
// loại. Học viên chỉ tải document, phải đang học một lớp active và có ít hơn MaxUnattachedDocuments file chưa gắn bản
// nộp; kiểm và lưu trong cùng transaction có khóa theo học viên nên hai request song song không vượt giới hạn.
func (s *Service) InitUpload(ctx context.Context, actor Principal, cmd InitUploadCmd) (UploadTicket, error) {
	if !s.canUpload(actor) {
		return UploadTicket{}, domain.ErrForbidden
	}
	kind, err := ParseKind(cmd.Kind)
	if err != nil {
		return UploadTicket{}, err
	}
	if !actor.IsAdmin() && kind != KindDocument {
		return UploadTicket{}, ErrStudentUploadKind
	}
	now := s.clock.Now()
	m, err := NewPendingUpload(s.ids.New(), kind, cmd.FileName, cmd.ContentType, cmd.SizeBytes, actor.ID, now, s.cfg.Limits)
	if err != nil {
		return UploadTicket{}, err
	}
	url, headers, err := s.store.PresignPut(ctx, m.StorageKey(), m.ContentType(), m.SizeBytes(), UploadTTL)
	if err != nil {
		return UploadTicket{}, fmt.Errorf("media: ký URL upload: %w", err)
	}
	if err := s.create(ctx, actor, m); err != nil {
		return UploadTicket{}, err
	}
	return UploadTicket{MediaID: m.ID(), UploadURL: url, Headers: headers, ExpiresAt: now.Add(UploadTTL)}, nil
}

func (s *Service) create(ctx context.Context, actor Principal, m *MediaFile) error {
	if actor.IsAdmin() {
		return s.repo.Create(ctx, s.db, m)
	}
	return s.cfg.StudentTx.Transact(ctx, func(tx db.Executor) error {
		now := s.clock.Now()
		q, err := s.repo.StudentUploadQuota(ctx, tx, actor.ID, QuotaWindow{
			ReadySince: now.Add(-UnattachedWindow), PendingSince: now.Add(-UploadTTL),
		})
		if err != nil {
			return err
		}
		if !q.Enrolled {
			return ErrNotEnrolled
		}
		if q.Unattached >= MaxUnattachedDocuments {
			return ErrTooManyUnattached
		}
		return s.repo.Create(ctx, tx, m)
	})
}

// CompleteUpload Stat object trên storage; chỉ khi object tồn tại và khớp khai báo mới chuyển ready. Admin hoàn tất
// mọi file; học viên chỉ file mình tải lên (file của người khác coi như không tồn tại).
func (s *Service) CompleteUpload(ctx context.Context, actor Principal, id uuid.UUID) (*MediaFile, error) {
	if !s.canUpload(actor) {
		return nil, domain.ErrForbidden
	}
	m, err := s.repo.ByID(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	if !actor.IsAdmin() && m.UploadedBy() != actor.ID {
		return nil, ErrMediaNotFound
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
// dạy hoặc học, file mình tải lên và file trong bản nộp của lớp mình dạy (CanUserAccess). Storage tự xử lý Range nên video tua được. File kèm (document) luôn ký dạng attachment
// với type chuẩn đã lưu, để trình duyệt tải về thay vì hiển thị nội dung người khác upload.
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
	url, err := s.store.PresignGet(ctx, m.StorageKey(), s.cfg.URLTTL, getOptions(m))
	if err != nil {
		return SignedURL{}, fmt.Errorf("media: ký URL xem: %w", err)
	}
	return SignedURL{URL: url, ExpiresAt: now.Add(s.cfg.URLTTL)}, nil
}

// getOptions chọn header phản hồi cho URL xem: video, ảnh hiển thị trực tiếp; document luôn tải về.
func getOptions(m *MediaFile) storage.GetOptions {
	opts := storage.GetOptions{ContentType: m.ContentType()}
	if m.Kind() == KindDocument {
		opts.AttachmentName = m.FileName()
	}
	return opts
}
