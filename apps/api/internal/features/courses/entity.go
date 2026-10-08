// Package courses quản lý khóa học, phiên bản khóa học (danh sách phiên bản chặng có thứ tự) và thao tác áp dụng
// phiên bản chặng mới. Phiên bản đã phát hành là bất biến: chặn ở aggregate CourseVersion (domain) và ở câu SQL có
// điều kiện header còn draft (repository).
package courses

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

// Lỗi nghiệp vụ của khóa học; message lấy theo prototype/app.js.
var (
	ErrCourseNameRequired       = domain.ErrInvalid.WithMsg("Nhập mã và tên khóa học.")
	ErrStageVersionNotPublished = domain.ErrInvalid.WithMsg("Chỉ gắn được phiên bản chặng đã phát hành.")
	ErrNoStages                 = domain.ErrInvalid.WithMsg("Khóa học cần ít nhất một chặng.")
	ErrStageMismatch            = domain.ErrInvalid.WithMsg("Phiên bản chặng thay thế phải thuộc cùng chặng.")
	ErrSourceNotPublished       = domain.ErrInvalid.WithMsg("Phiên bản chặng chưa phát hành.")
	ErrCourseLacksStage         = domain.ErrInvalid.WithMsg("Khóa học không chứa chặng này.")
	ErrNoPublishedVersion       = domain.ErrInvalid.WithMsg("Khóa học chưa có phiên bản phát hành.")
	ErrNotPublished             = domain.ErrInvalid.WithMsg("Chọn một phiên bản khóa học đã phát hành.")
	ErrNoCoursesSelected        = domain.ErrInvalid.WithMsg("Chọn ít nhất một khóa học.")
	ErrDuplicateStage           = domain.ErrConflict.WithMsg("Khóa học đã chứa một phiên bản của chặng này.")
	ErrAlreadyUsingVersion      = domain.ErrConflict.WithMsg("Khóa học đã dùng phiên bản này.")
	ErrCodeTaken                = domain.ErrConflict.WithMsg("Mã khóa học đã tồn tại.")
	ErrPublishNotDraft          = domain.ErrInvalidTransition.WithMsg("Chỉ phát hành được bản nháp.")
	ErrCloneNotPublished        = domain.ErrInvalidTransition.WithMsg("Chỉ nhân bản từ phiên bản đã phát hành.")
	ErrArchiveNotPublished      = domain.ErrInvalidTransition.WithMsg("Chỉ lưu trữ phiên bản đã phát hành.")
	ErrDeleteNotDraft           = domain.ErrInvalidTransition.WithMsg("Chỉ xóa được bản nháp.")
	ErrVersionImmutable         = domain.ErrVersionImmutable.WithMsg("Phiên bản đã phát hành không thể chỉnh sửa.")
	ErrCourseNotFound           = domain.ErrNotFound.WithMsg("Không tìm thấy khóa học.")
	ErrVersionNotFound          = domain.ErrNotFound.WithMsg("Không tìm thấy phiên bản khóa học.")
	ErrStageVersionNotFound     = domain.ErrNotFound.WithMsg("Không tìm thấy phiên bản chặng.")
)

// errStageVersionArchived: một chặng của bản nháp đã bị lưu trữ sau khi gắn nên bản nháp không phát hành được nữa.
func errStageVersionArchived(stageCode string, no domain.VersionNo) error {
	return domain.ErrInvalid.WithMsg(fmt.Sprintf("Phiên bản chặng %s v%d đã lưu trữ, không thể phát hành lại.", stageCode, no))
}

// ErrDraftExists: khóa học đã có bản nháp; UI dùng DraftID để mở bản nháp đó.
type ErrDraftExists struct {
	DraftID uuid.UUID
	No      domain.VersionNo
}

func (e *ErrDraftExists) Error() string {
	return fmt.Sprintf("Khóa học đang có bản nháp v%d. Phát hành hoặc xóa bản nháp trước.", e.No)
}

// ErrInUse: phiên bản đã phát hành đang được lớp tham chiếu nên chỉ lưu trữ được.
type ErrInUse struct {
	UsedBy []UsedByClassRow
}

func (e *ErrInUse) Error() string {
	codes := make([]string, 0, len(e.UsedBy))
	for _, u := range e.UsedBy {
		codes = append(codes, u.Code)
	}
	return "Đang được dùng bởi lớp " + strings.Join(codes, ", ") + ". Hãy lưu trữ thay vì xóa."
}

// Course là khóa học; mô tả nằm trên phiên bản (course_versions.description) nên Course chỉ giữ để tạo bản nháp v1.
type Course struct {
	id          uuid.UUID
	code        domain.Code
	name        string
	description string
	createdBy   uuid.UUID
	createdAt   time.Time
}

// NewCourse kiểm tên khóa học; mã đã được domain.ParseCode kiểm trước.
func NewCourse(id uuid.UUID, code domain.Code, name, description string, by uuid.UUID, now time.Time) (*Course, error) {
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return nil, ErrCourseNameRequired
	}
	return &Course{id: id, code: code, name: name, description: strings.TrimSpace(description), createdBy: by, createdAt: now}, nil
}

func (c *Course) ID() uuid.UUID        { return c.id }
func (c *Course) Code() domain.Code    { return c.code }
func (c *Course) Name() string         { return c.name }
func (c *Course) Description() string  { return c.description }
func (c *Course) CreatedBy() uuid.UUID { return c.createdBy }
func (c *Course) CreatedAt() time.Time { return c.createdAt }

// CourseVersionStage là một phiên bản chặng trong phiên bản khóa học (value object). StageID denormalize để kiểm trùng
// chặng trong bộ nhớ; FK ghép fk_cvs_stage_version bảo đảm khớp stage_versions.stage_id.
type CourseVersionStage struct {
	StageID        uuid.UUID
	StageVersionID uuid.UUID
	Position       int
}

// StageVersionRef là phiên bản chặng đọc từ stages qua StageVersionReader; không phải entity của courses.
type StageVersionRef struct {
	ID        uuid.UUID
	StageID   uuid.UUID
	StageCode string
	VersionNo domain.VersionNo
	Status    domain.VersionStatus
}

// CourseVersion là gốc aggregate của danh sách chặng; chỉ bản draft được sửa.
type CourseVersion struct {
	id           uuid.UUID
	courseID     uuid.UUID
	versionNo    domain.VersionNo
	status       domain.VersionStatus
	title        string
	description  string
	clonedFromID *uuid.UUID
	stages       []CourseVersionStage
	publishedAt  *time.Time
	archivedAt   *time.Time
	createdBy    uuid.UUID
}

// NewDraftCourseVersion tạo bản nháp rỗng; title là tên khóa học tại thời điểm tạo.
func NewDraftCourseVersion(id, courseID uuid.UUID, no domain.VersionNo, title, description string, clonedFrom *uuid.UUID, by uuid.UUID) *CourseVersion {
	return &CourseVersion{
		id: id, courseID: courseID, versionNo: no, status: domain.VersionDraft,
		title: title, description: description, clonedFromID: clonedFrom, createdBy: by,
	}
}

func (v *CourseVersion) ID() uuid.UUID                { return v.id }
func (v *CourseVersion) CourseID() uuid.UUID          { return v.courseID }
func (v *CourseVersion) VersionNo() domain.VersionNo  { return v.versionNo }
func (v *CourseVersion) Status() domain.VersionStatus { return v.status }
func (v *CourseVersion) Title() string                { return v.title }
func (v *CourseVersion) Description() string          { return v.description }
func (v *CourseVersion) ClonedFromID() *uuid.UUID     { return v.clonedFromID }
func (v *CourseVersion) PublishedAt() *time.Time      { return v.publishedAt }
func (v *CourseVersion) ArchivedAt() *time.Time       { return v.archivedAt }
func (v *CourseVersion) CreatedBy() uuid.UUID         { return v.createdBy }

// Stages trả bản sao danh sách đã sắp theo position.
func (v *CourseVersion) Stages() []CourseVersionStage {
	return append([]CourseVersionStage(nil), v.stages...)
}

// StageVersionIDs là id phiên bản chặng theo thứ tự position.
func (v *CourseVersion) StageVersionIDs() []uuid.UUID {
	out := make([]uuid.UUID, 0, len(v.stages))
	for _, s := range v.stages {
		out = append(out, s.StageVersionID)
	}
	return out
}

// StageVersionFor tìm phiên bản của chặng stageID trong phiên bản khóa học.
func (v *CourseVersion) StageVersionFor(stageID uuid.UUID) (CourseVersionStage, bool) {
	for _, s := range v.stages {
		if s.StageID == stageID {
			return s, true
		}
	}
	return CourseVersionStage{}, false
}

// SetStages thay toàn bộ danh sách chặng của bản nháp theo thứ tự refs: mỗi ref phải đã phát hành, không hai phiên
// bản của cùng một chặng; position = vị trí trong refs + 1.
func (v *CourseVersion) SetStages(refs []StageVersionRef) error {
	if !v.status.IsMutable() {
		return ErrVersionImmutable
	}
	seen := make(map[uuid.UUID]bool, len(refs))
	stages := make([]CourseVersionStage, 0, len(refs))
	for i, r := range refs {
		if seen[r.StageID] {
			return ErrDuplicateStage
		}
		seen[r.StageID] = true
		if r.Status != domain.VersionPublished {
			return ErrStageVersionNotPublished
		}
		stages = append(stages, CourseVersionStage{StageID: r.StageID, StageVersionID: r.ID, Position: i + 1})
	}
	v.stages = stages
	return nil
}

// ReplaceStageVersion đổi phiên bản chặng old sang next (cùng chặng, next đã phát hành) và giữ nguyên position.
func (v *CourseVersion) ReplaceStageVersion(old, next StageVersionRef) error {
	if !v.status.IsMutable() {
		return ErrVersionImmutable
	}
	if old.StageID != next.StageID {
		return ErrStageMismatch
	}
	if next.Status != domain.VersionPublished {
		return ErrStageVersionNotPublished
	}
	for i, s := range v.stages {
		if s.StageVersionID == old.ID {
			v.stages[i].StageVersionID = next.ID
			return nil
		}
	}
	return ErrCourseLacksStage
}

// Publish kiểm lại mọi phiên bản chặng tại thời điểm phát hành (lookup đọc trong cùng transaction) rồi chỉ đổi trạng
// thái và thời điểm phát hành trong bộ nhớ; repository ghi bằng TransitionStatus, không chạm danh sách chặng.
func (v *CourseVersion) Publish(now time.Time, lookup func(uuid.UUID) (StageVersionRef, error)) error {
	if v.status.Transition(domain.VersionPublished) != nil {
		return ErrPublishNotDraft
	}
	if len(v.stages) == 0 {
		return ErrNoStages
	}
	for _, s := range v.stages {
		ref, err := lookup(s.StageVersionID)
		if err != nil {
			return err
		}
		switch ref.Status {
		case domain.VersionPublished:
		case domain.VersionArchived:
			return errStageVersionArchived(ref.StageCode, ref.VersionNo)
		default:
			return ErrStageVersionNotPublished
		}
	}
	v.status, v.publishedAt = domain.VersionPublished, &now
	return nil
}

// Archive chỉ đổi trạng thái; danh sách chặng không bị chạm.
func (v *CourseVersion) Archive(now time.Time) error {
	if v.status.Transition(domain.VersionArchived) != nil {
		return ErrArchiveNotPublished
	}
	v.status, v.archivedAt = domain.VersionArchived, &now
	return nil
}

// CloneAsDraft nhân bản nông bản đã phát hành: bản nháp mới trỏ lại đúng các phiên bản chặng cũ, cùng thứ tự.
func (v *CourseVersion) CloneAsDraft(newID uuid.UUID, nextNo domain.VersionNo, by uuid.UUID) (*CourseVersion, error) {
	if v.status != domain.VersionPublished {
		return nil, ErrCloneNotPublished
	}
	from := v.id
	c := NewDraftCourseVersion(newID, v.courseID, nextNo, v.title, v.description, &from, by)
	c.stages = v.Stages()
	return c, nil
}

// CanDelete: chỉ bản nháp xóa được.
func (v *CourseVersion) CanDelete() error {
	if v.status != domain.VersionDraft {
		return ErrDeleteNotDraft
	}
	return nil
}
