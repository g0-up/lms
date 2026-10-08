package courses

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
)

// CourseRepo lưu khóa học.
type CourseRepo interface {
	Create(ctx context.Context, ex db.Executor, c *Course) error // trùng mã → ErrCodeTaken
	ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*CourseRow, error)
	// Delete chỉ xóa khi khóa học không còn phiên bản nào.
	Delete(ctx context.Context, ex db.Executor, id uuid.UUID) error
	List(ctx context.Context, ex db.Executor, q ListQuery) ([]CourseListRow, error)
	// LockForUpdate khóa dòng courses để tuần tự hóa clone/áp dụng phiên bản chặng trên cùng khóa học.
	LockForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) error
}

// CourseVersionRepo lưu phiên bản khóa học và danh sách chặng. Bất biến: Create luôn chèn header draft, Create và
// SaveDraft chỉ ghi course_version_stages khi header còn draft (SQL có điều kiện, 0 dòng → ErrVersionImmutable);
// TransitionStatus chỉ UPDATE header.
type CourseVersionRepo interface {
	Create(ctx context.Context, ex db.Executor, v *CourseVersion) error // bản nháp thứ hai → ErrDraftExists
	SaveDraft(ctx context.Context, ex db.Executor, v *CourseVersion) error
	TransitionStatus(ctx context.Context, ex db.Executor, id uuid.UUID, from, to domain.VersionStatus, publishedAt *time.Time) error
	Delete(ctx context.Context, ex db.Executor, id uuid.UUID) error // lớp còn tham chiếu → domain.ErrInUse
	ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*CourseVersion, error)
	ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*CourseVersion, error)
	CountByCourse(ctx context.Context, ex db.Executor, courseID uuid.UUID) (int, error)
	ListByCourse(ctx context.Context, ex db.Executor, courseID uuid.UUID) ([]VersionListRow, error)
	// LatestPublished là bản published có version_no lớn nhất; nil nếu khóa học chưa phát hành.
	LatestPublished(ctx context.Context, ex db.Executor, courseID uuid.UUID) (*CourseVersion, error)
	// DraftOf là bản nháp của khóa học; nil nếu không có.
	DraftOf(ctx context.Context, ex db.Executor, courseID uuid.UUID) (*CourseVersion, error)
	// NextVersionNo khóa dòng courses trước khi đọc max(version_no) để hai clone song song không trùng số.
	NextVersionNo(ctx context.Context, ex db.Executor, courseID uuid.UUID) (domain.VersionNo, error)
	UsedByClasses(ctx context.Context, ex db.Executor, versionID uuid.UUID) ([]UsedByClassRow, error)
	// ClassesUsing là lớp trỏ tới bất kỳ phiên bản nào của khóa học.
	ClassesUsing(ctx context.Context, ex db.Executor, courseID uuid.UUID) ([]ClassUsingRow, error)
	// StageRows là danh sách chặng của phiên bản theo position, kèm số học liệu và bản phát hành mới nhất của chặng.
	StageRows(ctx context.Context, ex db.Executor, versionID uuid.UUID) ([]VersionStageRow, error)
}

// StageVersionReader là phần stages cung cấp cho courses (anti-corruption); adapter ở app/deps.go để stages không
// import courses. Id không tồn tại thì vắng mặt trong map.
type StageVersionReader interface {
	Refs(ctx context.Context, ex db.Executor, ids []uuid.UUID) (map[uuid.UUID]StageVersionRef, error)
}

// VersionReader là phần courses cung cấp cho lớp học: chỉ phiên bản đã phát hành mới gắn được vào lớp.
type VersionReader interface {
	// PublishedVersion trả ErrVersionNotFound hoặc ErrNotPublished.
	PublishedVersion(ctx context.Context, ex db.Executor, versionID uuid.UUID) (VersionSummary, error)
}

// VersionSummary là phiên bản khóa học kèm mã, tên khóa học.
type VersionSummary struct {
	ID         uuid.UUID
	CourseID   uuid.UUID
	CourseCode string
	CourseName string
	VersionNo  domain.VersionNo
	Status     domain.VersionStatus
}

// ListQuery lọc danh sách khóa học theo mã hoặc tên.
type ListQuery struct {
	Q string
}

// CourseRow là khóa học kèm mô tả của phiên bản mới nhất.
type CourseRow struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	CreatedAt   time.Time
}

// VersionListRow là một phiên bản trong danh sách/chi tiết khóa học, kèm số đếm.
type VersionListRow struct {
	ID                  uuid.UUID
	CourseID            uuid.UUID
	VersionNo           domain.VersionNo
	Status              domain.VersionStatus
	PublishedAt         *time.Time
	ClonedFromVersionNo *domain.VersionNo
	StageCount          int
	OutdatedStageCount  int
	ClassCount          int
}

// ClassUsingRow là lớp trỏ tới một phiên bản của khóa học.
type ClassUsingRow struct {
	CourseID  uuid.UUID
	ClassID   uuid.UUID
	Code      string
	Name      string
	VersionNo domain.VersionNo
}

// CourseListRow là một khóa học trong danh sách.
type CourseListRow struct {
	CourseRow
	Versions     []VersionListRow
	ClassesUsing []ClassUsingRow
}

// UsedByClassRow là lớp tham chiếu một phiên bản khóa học; MemberCount không tính học viên đã rời lớp.
type UsedByClassRow struct {
	ClassID     uuid.UUID
	Code        string
	Name        string
	Status      string
	MemberCount int
}

// VersionStageRow là một chặng của phiên bản khóa học cho màn hình chi tiết.
type VersionStageRow struct {
	Position          int
	StageID           uuid.UUID
	StageCode         string
	StageName         string
	StageVersionID    uuid.UUID
	StageVersionNo    domain.VersionNo
	LessonCount       int
	RequiredCount     int
	LatestPublishedNo *domain.VersionNo
}

// Outdated: chặng đã có bản phát hành mới hơn phiên bản đang gắn.
func (r VersionStageRow) Outdated() bool {
	return r.LatestPublishedNo != nil && *r.LatestPublishedNo > r.StageVersionNo
}
