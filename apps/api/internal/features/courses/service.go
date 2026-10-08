package courses

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
)

// Actor là admin thực hiện thao tác; RequestID đi vào audit.
type Actor struct {
	ID        uuid.UUID
	RequestID string
}

// Service là use case của khóa học và phiên bản khóa học. Mọi use case ghi chạy trong một transaction mở bằng
// ByIDForUpdate (hoặc khóa dòng courses) để tuần tự hóa với Publish/Archive/Clone/áp dụng phiên bản chặng.
type Service struct {
	db            db.Executor
	tx            db.Tx
	courses       CourseRepo
	versions      CourseVersionRepo
	stageVersions StageVersionReader
	clock         clock.Clock
	audit         audit.Recorder
	ids           ids.Generator
}

// Deps là phụ thuộc của Service; DB dùng cho truy vấn đọc ngoài transaction.
type Deps struct {
	DB            db.Executor
	Tx            db.Tx
	Courses       CourseRepo
	Versions      CourseVersionRepo
	StageVersions StageVersionReader
	Clock         clock.Clock
	Audit         audit.Recorder
	IDs           ids.Generator
}

// NewService nối phụ thuộc.
func NewService(d Deps) *Service {
	return &Service{
		db: d.DB, tx: d.Tx, courses: d.Courses, versions: d.Versions, stageVersions: d.StageVersions,
		clock: d.Clock, audit: d.Audit, ids: d.IDs,
	}
}

var _ VersionReader = (*Service)(nil)

// CreateCourseCmd là dữ liệu tạo khóa học.
type CreateCourseCmd struct {
	Code        string
	Name        string
	Description string
}

// CourseDetail là khóa học kèm phiên bản (mới nhất trước) và lớp đang dùng.
type CourseDetail struct {
	Course       CourseRow
	Versions     []VersionListRow
	ClassesUsing []ClassUsingRow
}

// VersionDetail là phiên bản khóa học kèm danh sách chặng và lớp đang dùng.
type VersionDetail struct {
	Version             *CourseVersion
	Course              CourseRow
	ClonedFromVersionNo *domain.VersionNo
	Stages              []VersionStageRow
	Classes             []UsedByClassRow
	// NewerPublished: khóa học có bản published với số phiên bản lớn hơn.
	NewerPublished bool
}

// CreateCourse tạo khóa học cùng bản nháp v1 rỗng.
func (s *Service) CreateCourse(ctx context.Context, actor Actor, cmd CreateCourseCmd) (CourseDetail, error) {
	if strings.TrimSpace(cmd.Code) == "" || strings.TrimSpace(cmd.Name) == "" {
		return CourseDetail{}, ErrCourseNameRequired
	}
	code, err := domain.ParseCode(cmd.Code, domain.CourseCode)
	if err != nil {
		return CourseDetail{}, err
	}
	course, err := NewCourse(s.ids.New(), code, cmd.Name, cmd.Description, actor.ID, s.clock.Now())
	if err != nil {
		return CourseDetail{}, err
	}
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		if err := s.courses.Create(ctx, tx, course); err != nil {
			return err
		}
		v := NewDraftCourseVersion(s.ids.New(), course.ID(), domain.FirstVersionNo, course.Name(), course.Description(), nil, actor.ID)
		if err := s.versions.Create(ctx, tx, v); err != nil {
			return err
		}
		return s.record(ctx, tx, actor, audit.ActionCourseCreated, "course", course.ID(),
			map[string]any{"courseId": course.ID(), "code": course.Code().String(), "name": course.Name()})
	})
	if err != nil {
		return CourseDetail{}, err
	}
	return s.GetCourse(ctx, course.ID())
}

// ListCourses lọc khóa học theo mã hoặc tên.
func (s *Service) ListCourses(ctx context.Context, q ListQuery) ([]CourseListRow, error) {
	return s.courses.List(ctx, s.db, q)
}

// GetCourse đọc khóa học cho màn hình chi tiết.
func (s *Service) GetCourse(ctx context.Context, id uuid.UUID) (CourseDetail, error) {
	course, err := s.courses.ByID(ctx, s.db, id)
	if err != nil {
		return CourseDetail{}, err
	}
	versions, err := s.versions.ListByCourse(ctx, s.db, id)
	if err != nil {
		return CourseDetail{}, err
	}
	classes, err := s.versions.ClassesUsing(ctx, s.db, id)
	if err != nil {
		return CourseDetail{}, err
	}
	return CourseDetail{Course: *course, Versions: versions, ClassesUsing: classes}, nil
}

// GetVersion đọc phiên bản khóa học cùng danh sách chặng và lớp đang dùng.
func (s *Service) GetVersion(ctx context.Context, versionID uuid.UUID) (VersionDetail, error) {
	v, err := s.versions.ByID(ctx, s.db, versionID)
	if err != nil {
		return VersionDetail{}, err
	}
	course, err := s.courses.ByID(ctx, s.db, v.CourseID())
	if err != nil {
		return VersionDetail{}, err
	}
	summaries, err := s.versions.ListByCourse(ctx, s.db, v.CourseID())
	if err != nil {
		return VersionDetail{}, err
	}
	stages, err := s.versions.StageRows(ctx, s.db, versionID)
	if err != nil {
		return VersionDetail{}, err
	}
	classes, err := s.versions.UsedByClasses(ctx, s.db, versionID)
	if err != nil {
		return VersionDetail{}, err
	}
	d := VersionDetail{Version: v, Course: *course, Stages: stages, Classes: classes}
	for _, sum := range summaries {
		if sum.ID == versionID {
			d.ClonedFromVersionNo = sum.ClonedFromVersionNo
		}
		if sum.Status == domain.VersionPublished && sum.VersionNo > v.VersionNo() {
			d.NewerPublished = true
		}
	}
	return d, nil
}

// SetStages thay danh sách chặng của bản nháp theo đúng thứ tự stageVersionIDs.
func (s *Service) SetStages(ctx context.Context, _ Actor, versionID uuid.UUID, stageVersionIDs []uuid.UUID) (VersionDetail, error) {
	return s.mutateVersion(ctx, versionID, func(tx db.Executor) error {
		v, err := s.lockDraft(ctx, tx, versionID)
		if err != nil {
			return err
		}
		found, err := s.stageVersions.Refs(ctx, tx, stageVersionIDs)
		if err != nil {
			return err
		}
		refs := make([]StageVersionRef, 0, len(stageVersionIDs))
		for _, id := range stageVersionIDs {
			ref, ok := found[id]
			if !ok {
				return ErrStageVersionNotFound
			}
			refs = append(refs, ref)
		}
		if err := v.SetStages(refs); err != nil {
			return err
		}
		return s.versions.SaveDraft(ctx, tx, v)
	})
}

// Publish phát hành bản nháp: khóa header → kiểm lại mọi phiên bản chặng trong transaction → chuyển trạng thái
// header (không ghi lại course_version_stages) → audit.
func (s *Service) Publish(ctx context.Context, actor Actor, versionID uuid.UUID) (VersionDetail, error) {
	return s.mutateVersion(ctx, versionID, func(tx db.Executor) error {
		v, err := s.versions.ByIDForUpdate(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if err := s.publish(ctx, tx, v, s.clock.Now()); err != nil {
			return err
		}
		return s.record(ctx, tx, actor, audit.ActionCourseVersionPublished, "course_version", v.ID(),
			map[string]any{"courseId": v.CourseID(), "versionNo": v.VersionNo(), "stageVersionIds": v.StageVersionIDs()})
	})
}

// publish kiểm domain với phiên bản chặng đọc trong tx rồi ghi chuyển trạng thái draft → published tại now.
func (s *Service) publish(ctx context.Context, tx db.Executor, v *CourseVersion, now time.Time) error {
	refs, err := s.stageVersions.Refs(ctx, tx, v.StageVersionIDs())
	if err != nil {
		return err
	}
	lookup := func(id uuid.UUID) (StageVersionRef, error) {
		ref, ok := refs[id]
		if !ok {
			return StageVersionRef{}, ErrStageVersionNotFound
		}
		return ref, nil
	}
	if err := v.Publish(now, lookup); err != nil {
		return err
	}
	return s.versions.TransitionStatus(ctx, tx, v.ID(), domain.VersionDraft, domain.VersionPublished, &now)
}

// Clone tạo bản nháp v(max+1) từ bản đã phát hành với cùng danh sách tham chiếu; mỗi khóa học tối đa một bản nháp.
func (s *Service) Clone(ctx context.Context, actor Actor, versionID uuid.UUID) (VersionDetail, error) {
	var cloneID uuid.UUID
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		src, err := s.versions.ByID(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if src.Status() != domain.VersionPublished {
			return ErrCloneNotPublished
		}
		c, err := s.cloneLocked(ctx, tx, src, actor)
		if err != nil {
			return err
		}
		if err := s.versions.Create(ctx, tx, c); err != nil {
			return err
		}
		cloneID = c.ID()
		return s.recordCloned(ctx, tx, actor, time.Time{}, src, c)
	})
	if err != nil {
		return VersionDetail{}, err
	}
	return s.GetVersion(ctx, cloneID)
}

// cloneLocked khóa dòng courses, chặn khi đã có bản nháp rồi dựng bản nháp v(max+1) trong bộ nhớ.
func (s *Service) cloneLocked(ctx context.Context, tx db.Executor, src *CourseVersion, actor Actor) (*CourseVersion, error) {
	if err := s.courses.LockForUpdate(ctx, tx, src.CourseID()); err != nil {
		return nil, err
	}
	draft, err := s.versions.DraftOf(ctx, tx, src.CourseID())
	if err != nil {
		return nil, err
	}
	if draft != nil {
		return nil, &ErrDraftExists{DraftID: draft.ID(), No: draft.VersionNo()}
	}
	next, err := s.versions.NextVersionNo(ctx, tx, src.CourseID())
	if err != nil {
		return nil, err
	}
	return src.CloneAsDraft(s.ids.New(), next, actor.ID)
}

func (s *Service) recordCloned(ctx context.Context, tx db.Executor, actor Actor, at time.Time, src, c *CourseVersion) error {
	return s.recordAt(ctx, tx, actor, at, audit.ActionCourseVersionCloned, "course_version", c.ID(),
		map[string]any{"courseId": src.CourseID(), "fromVersionNo": src.VersionNo(), "toVersionNo": c.VersionNo()})
}

// Archive lưu trữ bản đã phát hành; chỉ đổi header, lớp đang trỏ tới không bị ảnh hưởng.
func (s *Service) Archive(ctx context.Context, actor Actor, versionID uuid.UUID) (VersionDetail, error) {
	return s.mutateVersion(ctx, versionID, func(tx db.Executor) error {
		v, err := s.versions.ByIDForUpdate(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if err := v.Archive(s.clock.Now()); err != nil {
			return err
		}
		if err := s.versions.TransitionStatus(ctx, tx, v.ID(), domain.VersionPublished, domain.VersionArchived, nil); err != nil {
			return err
		}
		return s.record(ctx, tx, actor, audit.ActionCourseVersionArchived, "course_version", v.ID(),
			map[string]any{"courseId": v.CourseID(), "versionNo": v.VersionNo()})
	})
}

// Delete xóa bản nháp. Xóa phiên bản cuối cùng thì xóa luôn khóa học; trả courseDeleted. Bản đã phát hành đang được
// lớp dùng → ErrInUse kèm danh sách lớp.
func (s *Service) Delete(ctx context.Context, actor Actor, versionID uuid.UUID) (bool, error) {
	var courseDeleted bool
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		v, err := s.versions.ByIDForUpdate(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if err := v.CanDelete(); err != nil {
			if v.Status() == domain.VersionPublished {
				usedBy, uerr := s.versions.UsedByClasses(ctx, tx, versionID)
				if uerr != nil {
					return uerr
				}
				if len(usedBy) > 0 {
					return &ErrInUse{UsedBy: usedBy}
				}
			}
			return err
		}
		if err := s.versions.Delete(ctx, tx, versionID); err != nil {
			return err
		}
		left, err := s.versions.CountByCourse(ctx, tx, v.CourseID())
		if err != nil {
			return err
		}
		if left == 0 {
			if err := s.courses.Delete(ctx, tx, v.CourseID()); err != nil {
				return err
			}
			courseDeleted = true
		}
		return s.record(ctx, tx, actor, audit.ActionCourseVersionDeleted, "course_version", v.ID(),
			map[string]any{"courseId": v.CourseID(), "versionNo": v.VersionNo(), "courseDeleted": courseDeleted})
	})
	if err != nil {
		return false, err
	}
	return courseDeleted, nil
}

// PublishedVersion là VersionReader cho lớp học: chỉ phiên bản đã phát hành hợp lệ.
func (s *Service) PublishedVersion(ctx context.Context, ex db.Executor, versionID uuid.UUID) (VersionSummary, error) {
	v, err := s.versions.ByID(ctx, ex, versionID)
	if err != nil {
		return VersionSummary{}, err
	}
	if v.Status() != domain.VersionPublished {
		return VersionSummary{}, ErrNotPublished
	}
	course, err := s.courses.ByID(ctx, ex, v.CourseID())
	if err != nil {
		return VersionSummary{}, err
	}
	return VersionSummary{
		ID: v.ID(), CourseID: course.ID, CourseCode: course.Code, CourseName: course.Name,
		VersionNo: v.VersionNo(), Status: v.Status(),
	}, nil
}

// mutateVersion chạy fn trong transaction rồi đọc lại phiên bản sau commit.
func (s *Service) mutateVersion(ctx context.Context, versionID uuid.UUID, fn func(tx db.Executor) error) (VersionDetail, error) {
	if err := s.tx.Transact(ctx, fn); err != nil {
		return VersionDetail{}, err
	}
	return s.GetVersion(ctx, versionID)
}

// lockDraft khóa header và báo ErrVersionImmutable trước khi kiểm dữ liệu vào, để lỗi trên bản đã phát hành luôn
// là VERSION_IMMUTABLE.
func (s *Service) lockDraft(ctx context.Context, tx db.Executor, versionID uuid.UUID) (*CourseVersion, error) {
	v, err := s.versions.ByIDForUpdate(ctx, tx, versionID)
	if err != nil {
		return nil, err
	}
	if !v.Status().IsMutable() {
		return nil, ErrVersionImmutable
	}
	return v, nil
}

func (s *Service) record(ctx context.Context, tx db.Executor, actor Actor, action, targetType string, targetID uuid.UUID, after any) error {
	return s.recordAt(ctx, tx, actor, time.Time{}, action, targetType, targetID, after)
}

// recordAt ghi audit với mốc at cố định (rỗng = đồng hồ của Recorder), dùng khi một thao tác ghi nhiều dòng.
func (s *Service) recordAt(ctx context.Context, tx db.Executor, actor Actor, at time.Time, action, targetType string, targetID uuid.UUID, after any) error {
	actorID := actor.ID
	return s.audit.Record(ctx, tx, audit.Entry{
		ActorID: &actorID, Action: action, TargetType: targetType, TargetID: &targetID, After: after, RequestID: actor.RequestID, At: at,
	})
}
