package stages

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
)

// StageRepo lưu chặng.
type StageRepo interface {
	Create(ctx context.Context, ex db.Executor, s *Stage) error // trùng mã → ErrCodeTaken
	ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*StageRow, error)
	// Delete chỉ xóa khi chặng không còn phiên bản nào.
	Delete(ctx context.Context, ex db.Executor, id uuid.UUID) error
	// LockForUpdate khóa dòng stages để tuần tự hóa clone/tạo phiên bản của cùng chặng.
	LockForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) error
	List(ctx context.Context, ex db.Executor, q ListQuery) ([]StageListRow, error)
}

// StageVersionRepo lưu phiên bản chặng và học liệu. Bất biến: Create/SaveDraft chỉ ghi khi header còn draft (SQL có
// điều kiện, 0 dòng → ErrVersionImmutable); TransitionStatus chỉ UPDATE header.
type StageVersionRepo interface {
	Create(ctx context.Context, ex db.Executor, v *StageVersion) error // bản nháp thứ hai → ErrDraftExists
	SaveDraft(ctx context.Context, ex db.Executor, v *StageVersion) error
	TransitionStatus(ctx context.Context, ex db.Executor, id uuid.UUID, from, to domain.VersionStatus, publishedAt *time.Time) error
	Delete(ctx context.Context, ex db.Executor, id uuid.UUID) error
	ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*StageVersion, error)
	ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*StageVersion, error)
	ListByStage(ctx context.Context, ex db.Executor, stageID uuid.UUID) ([]VersionSummary, error)
	CountByStage(ctx context.Context, ex db.Executor, stageID uuid.UUID) (int, error)
	// NextVersionNo khóa dòng stages trước khi đọc max(version_no) để hai clone song song không trùng số.
	NextVersionNo(ctx context.Context, ex db.Executor, stageID uuid.UUID) (domain.VersionNo, error)
	DraftOf(ctx context.Context, ex db.Executor, stageID uuid.UUID) (*VersionSummary, error)
	UsedBy(ctx context.Context, ex db.Executor, versionID uuid.UUID) ([]UsedByRow, error)
	UsedByStage(ctx context.Context, ex db.Executor, stageID uuid.UUID) ([]StageUsedByRow, error)
	OutdatedCourses(ctx context.Context, ex db.Executor, stageID uuid.UUID) ([]OutdatedCourse, error)
	AllOutdated(ctx context.Context, ex db.Executor) ([]OutdatedCourse, error)
}

// OutdatedReader là phần FR-18 stages cung cấp cho dashboard; nơi dùng không viết SQL riêng.
type OutdatedReader interface {
	OutdatedCourses(ctx context.Context, stageID uuid.UUID) ([]OutdatedCourse, error)
	AllOutdated(ctx context.Context) ([]OutdatedCourse, error)
}

// ListQuery lọc danh sách chặng theo mã hoặc tên.
type ListQuery struct {
	Q string
}

// StageRow là chặng kèm mô tả của phiên bản mới nhất.
type StageRow struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	CreatedAt   time.Time
}

// VersionSummary là một dòng phiên bản trong danh sách, kèm số học liệu.
type VersionSummary struct {
	ID                  uuid.UUID
	StageID             uuid.UUID
	VersionNo           domain.VersionNo
	Status              domain.VersionStatus
	PublishedAt         *time.Time
	ClonedFromVersionNo *domain.VersionNo
	LessonCount         int
}

// StageListRow là một chặng trong danh sách với các số đếm cho màn hình chặng.
type StageListRow struct {
	StageRow
	Versions            []VersionSummary
	UsedByCourseCount   int
	OutdatedCourseCount int
}

// UsedByRow là một phiên bản khóa học tham chiếu phiên bản chặng.
type UsedByRow struct {
	CourseID        uuid.UUID
	CourseCode      string
	CourseName      string
	CourseVersionID uuid.UUID
	VersionNo       domain.VersionNo
	Status          domain.VersionStatus
	ClassCodes      []string
}

// StageUsedByRow là UsedByRow trên màn hình chặng: kèm phiên bản chặng đang dùng và cờ đã cũ.
type StageUsedByRow struct {
	UsedByRow
	StageVersionNo domain.VersionNo
	Outdated       bool
}

// OutdatedCourse là khóa học mà phiên bản phát hành mới nhất còn dùng phiên bản chặng cũ hơn bản phát hành mới nhất.
type OutdatedCourse struct {
	CourseID        uuid.UUID
	CourseCode      string
	CourseName      string
	CourseVersionID uuid.UUID
	CourseVersionNo domain.VersionNo
	StageID         uuid.UUID
	StageCode       string
	StageName       string
	UsingVersionNo  domain.VersionNo
	LatestVersionID uuid.UUID
	LatestVersionNo domain.VersionNo
	DraftVersionID  *uuid.UUID
	DraftVersionNo  *domain.VersionNo
}

// VersionRef là phiên bản chặng nhìn từ feature khác (courses ghép chặng vào khóa học và áp phiên bản mới).
type VersionRef struct {
	ID          uuid.UUID
	StageID     uuid.UUID
	StageCode   string
	StageName   string
	VersionNo   domain.VersionNo
	Status      domain.VersionStatus
	LessonCount int
}

// VersionRefReader là phần stages cung cấp cho courses; adapter ở app nối vào interface khai báo bên courses để
// stages không import courses.
type VersionRefReader interface {
	// Refs trả phiên bản theo id; id không tồn tại không có trong map.
	Refs(ctx context.Context, ex db.Executor, ids []uuid.UUID) (map[uuid.UUID]VersionRef, error)
	// LatestPublishedOfStage trả bản phát hành mới nhất; chặng chưa phát hành bản nào → ErrVersionNotFound.
	LatestPublishedOfStage(ctx context.Context, ex db.Executor, stageID uuid.UUID) (VersionRef, error)
}
