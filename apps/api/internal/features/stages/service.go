package stages

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/media"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
)

// ErrEmbeddedImage: ảnh trong markdown phải upload qua media, không nhúng base64.
var ErrEmbeddedImage = domain.ErrInvalid.WithMsg("Ảnh trong markdown phải tải lên, không nhúng base64.")

// ErrExternalImage: ảnh trong markdown chỉ được trỏ tới media đã upload (/api/v1/media/{id}/content).
var ErrExternalImage = domain.ErrInvalid.WithMsg("Ảnh trong markdown phải tải lên qua hệ thống.")

// ErrLessonKeyImmutable: lesson_key cố định sau khi tạo để đối chiếu học liệu giữa các phiên bản.
var ErrLessonKeyImmutable = domain.ErrInvalid.WithMsg("Không đổi được mã học liệu sau khi tạo.")

// errImageNotUploaded: ảnh trong markdown trỏ tới media không tồn tại, chưa upload xong hoặc không phải ảnh.
func errImageNotUploaded(lessonTitle string) error {
	return domain.ErrInvalid.WithMsg(fmt.Sprintf("Ảnh trong học liệu \"%s\" chưa được tải lên.", lessonTitle))
}

// Actor là admin thực hiện thao tác; RequestID đi vào audit.
type Actor struct {
	ID        uuid.UUID
	RequestID string
}

// Service là use case của chặng, phiên bản chặng và học liệu. Mọi use case ghi chạy trong một transaction mở bằng
// ByIDForUpdate (hoặc khóa dòng stages) để tuần tự hóa với Publish/Archive/Clone.
type Service struct {
	db       db.Executor
	tx       db.Tx
	stages   StageRepo
	versions StageVersionRepo
	media    media.Reader
	render   MarkdownRenderer
	clock    clock.Clock
	audit    audit.Recorder
	ids      ids.Generator
}

// Deps là phụ thuộc của Service; DB dùng cho truy vấn đọc ngoài transaction.
type Deps struct {
	DB       db.Executor
	Tx       db.Tx
	Stages   StageRepo
	Versions StageVersionRepo
	Media    media.Reader
	Render   MarkdownRenderer
	Clock    clock.Clock
	Audit    audit.Recorder
	IDs      ids.Generator
}

// NewService nối phụ thuộc.
func NewService(d Deps) *Service {
	return &Service{
		db: d.DB, tx: d.Tx, stages: d.Stages, versions: d.Versions, media: d.Media, render: d.Render,
		clock: d.Clock, audit: d.Audit, ids: d.IDs,
	}
}

var _ OutdatedReader = (*Service)(nil)

// CreateStageCmd là dữ liệu tạo chặng.
type CreateStageCmd struct {
	Code        string
	Name        string
	Description string
}

// LessonCmd là dữ liệu thêm/sửa học liệu; nil = không gửi (PATCH giữ giá trị cũ).
type LessonCmd struct {
	LessonKey       *string
	Title           *string
	Type            *string
	Required        *bool
	DurationSeconds *int
	VideoMediaID    *uuid.UUID
	MarkdownSource  *string
}

// StageDetail là chặng kèm phiên bản, nơi dùng và khóa học dùng phiên bản cũ (FR-18).
type StageDetail struct {
	Stage    StageRow
	Versions []VersionSummary
	UsedBy   []StageUsedByRow
	Outdated []OutdatedCourse
}

// VersionDetail là phiên bản chặng kèm học liệu và nơi dùng.
type VersionDetail struct {
	Version             *StageVersion
	Stage               StageRow
	ClonedFromVersionNo *domain.VersionNo
	UsedBy              []UsedByRow
}

// CreateStage tạo chặng cùng bản nháp v1 rỗng.
func (s *Service) CreateStage(ctx context.Context, actor Actor, cmd CreateStageCmd) (StageDetail, error) {
	if strings.TrimSpace(cmd.Code) == "" || strings.TrimSpace(cmd.Name) == "" {
		return StageDetail{}, ErrStageNameRequired
	}
	code, err := domain.ParseCode(cmd.Code, domain.StageCode)
	if err != nil {
		return StageDetail{}, err
	}
	stage, err := NewStage(s.ids.New(), code, cmd.Name, cmd.Description, actor.ID, s.clock.Now())
	if err != nil {
		return StageDetail{}, err
	}
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		if err := s.stages.Create(ctx, tx, stage); err != nil {
			return err
		}
		first, err := domain.ParseVersionNo(1)
		if err != nil {
			return err
		}
		v := NewDraftVersion(s.ids.New(), stage.ID(), first, stage.Name(), stage.Description(), actor.ID)
		if err := s.versions.Create(ctx, tx, v); err != nil {
			return err
		}
		return s.record(ctx, tx, actor, audit.ActionStageCreated, "stage", stage.ID(),
			map[string]any{"stageId": stage.ID(), "code": stage.Code().String(), "name": stage.Name()})
	})
	if err != nil {
		return StageDetail{}, err
	}
	return s.GetStage(ctx, stage.ID())
}

// GetStage đọc chặng cho màn hình chi tiết.
func (s *Service) GetStage(ctx context.Context, id uuid.UUID) (StageDetail, error) {
	stage, err := s.stages.ByID(ctx, s.db, id)
	if err != nil {
		return StageDetail{}, err
	}
	versions, err := s.versions.ListByStage(ctx, s.db, id)
	if err != nil {
		return StageDetail{}, err
	}
	usedBy, err := s.versions.UsedByStage(ctx, s.db, id)
	if err != nil {
		return StageDetail{}, err
	}
	outdated, err := s.versions.OutdatedCourses(ctx, s.db, id)
	if err != nil {
		return StageDetail{}, err
	}
	return StageDetail{Stage: *stage, Versions: versions, UsedBy: usedBy, Outdated: outdated}, nil
}

// ListStages lọc chặng theo mã hoặc tên.
func (s *Service) ListStages(ctx context.Context, q ListQuery) ([]StageListRow, error) {
	return s.stages.List(ctx, s.db, q)
}

// GetVersion đọc phiên bản chặng cùng học liệu và nơi dùng.
func (s *Service) GetVersion(ctx context.Context, versionID uuid.UUID) (VersionDetail, error) {
	return s.versionDetail(ctx, s.db, versionID)
}

func (s *Service) versionDetail(ctx context.Context, ex db.Executor, versionID uuid.UUID) (VersionDetail, error) {
	v, err := s.versions.ByID(ctx, ex, versionID)
	if err != nil {
		return VersionDetail{}, err
	}
	stage, err := s.stages.ByID(ctx, ex, v.StageID())
	if err != nil {
		return VersionDetail{}, err
	}
	summaries, err := s.versions.ListByStage(ctx, ex, v.StageID())
	if err != nil {
		return VersionDetail{}, err
	}
	usedBy, err := s.versions.UsedBy(ctx, ex, versionID)
	if err != nil {
		return VersionDetail{}, err
	}
	d := VersionDetail{Version: v, Stage: *stage, UsedBy: usedBy}
	for _, sum := range summaries {
		if sum.ID == versionID {
			d.ClonedFromVersionNo = sum.ClonedFromVersionNo
		}
	}
	return d, nil
}

// AddLesson thêm học liệu vào cuối bản nháp.
func (s *Service) AddLesson(ctx context.Context, _ Actor, versionID uuid.UUID, cmd LessonCmd) (*Lesson, error) {
	var added *Lesson
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		v, err := s.lockDraft(ctx, tx, versionID)
		if err != nil {
			return err
		}
		var key domain.LessonKey
		if cmd.LessonKey != nil && strings.TrimSpace(*cmd.LessonKey) != "" {
			if key, err = domain.ParseLessonKey(strings.TrimSpace(*cmd.LessonKey)); err != nil {
				return err
			}
		}
		content, err := s.contentFactory(ctx, tx, cmd, nil)
		if err != nil {
			return err
		}
		required := cmd.Required == nil || *cmd.Required
		if added, err = v.AddLesson(s.ids.New(), key, deref(cmd.Title), content, required); err != nil {
			return err
		}
		return s.versions.SaveDraft(ctx, tx, v)
	})
	if err != nil {
		return nil, err
	}
	return added, nil
}

// UpdateLesson sửa học liệu trong bản nháp; trường không gửi giữ nguyên.
func (s *Service) UpdateLesson(ctx context.Context, _ Actor, versionID, lessonID uuid.UUID, cmd LessonCmd) (*Lesson, error) {
	var updated *Lesson
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		v, err := s.lockDraft(ctx, tx, versionID)
		if err != nil {
			return err
		}
		l, err := v.Lesson(lessonID)
		if err != nil {
			return err
		}
		if cmd.LessonKey != nil && strings.TrimSpace(*cmd.LessonKey) != string(l.Key()) {
			return ErrLessonKeyImmutable
		}
		content, err := s.contentFactory(ctx, tx, cmd, l.Content())
		if err != nil {
			return err
		}
		title, required := l.Title(), l.Required()
		if cmd.Title != nil {
			title = *cmd.Title
		}
		if cmd.Required != nil {
			required = *cmd.Required
		}
		if err := v.UpdateLesson(lessonID, title, content, required); err != nil {
			return err
		}
		if err := s.versions.SaveDraft(ctx, tx, v); err != nil {
			return err
		}
		updated = l
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// RemoveLesson xóa học liệu khỏi bản nháp và dồn thứ tự.
func (s *Service) RemoveLesson(ctx context.Context, _ Actor, versionID, lessonID uuid.UUID) error {
	return s.tx.Transact(ctx, func(tx db.Executor) error {
		v, err := s.lockDraft(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if err := v.RemoveLesson(lessonID); err != nil {
			return err
		}
		return s.versions.SaveDraft(ctx, tx, v)
	})
}

// ReorderLessons sắp lại toàn bộ học liệu của bản nháp.
func (s *Service) ReorderLessons(ctx context.Context, _ Actor, versionID uuid.UUID, lessonIDs []uuid.UUID) (VersionDetail, error) {
	return s.mutateVersion(ctx, versionID, func(tx db.Executor) error {
		v, err := s.lockDraft(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if err := v.Reorder(lessonIDs); err != nil {
			return err
		}
		return s.versions.SaveDraft(ctx, tx, v)
	})
}

// Publish phát hành bản nháp. Thứ tự bắt buộc: khóa header → render markdown → ghi học liệu (SaveDraft, chỉ khớp
// khi header còn draft) → kiểm domain → chuyển trạng thái header → audit.
func (s *Service) Publish(ctx context.Context, actor Actor, versionID uuid.UUID) (VersionDetail, error) {
	return s.mutateVersion(ctx, versionID, func(tx db.Executor) error {
		v, err := s.versions.ByIDForUpdate(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if v.Status() != domain.VersionDraft {
			return ErrPublishNotDraft
		}
		if err := v.RenderMarkdown(s.render); err != nil {
			return err
		}
		if err := s.checkMarkdownImages(ctx, tx, v); err != nil {
			return err
		}
		if err := s.versions.SaveDraft(ctx, tx, v); err != nil {
			return err
		}
		now := s.clock.Now()
		if err := v.Publish(now); err != nil {
			return err
		}
		if err := s.versions.TransitionStatus(ctx, tx, v.ID(), domain.VersionDraft, domain.VersionPublished, &now); err != nil {
			return err
		}
		return s.record(ctx, tx, actor, audit.ActionStageVersionPublished, "stage_version", v.ID(),
			map[string]any{"stageId": v.StageID(), "versionNo": v.VersionNo(), "lessonCount": len(v.Lessons())})
	})
}

// Clone tạo bản nháp v(max+1) từ bản đã phát hành; mỗi chặng tối đa một bản nháp.
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
		if err := s.stages.LockForUpdate(ctx, tx, src.StageID()); err != nil {
			return err
		}
		draft, err := s.versions.DraftOf(ctx, tx, src.StageID())
		if err != nil {
			return err
		}
		if draft != nil {
			return &ErrDraftExists{DraftID: draft.ID, No: draft.VersionNo}
		}
		next, err := s.versions.NextVersionNo(ctx, tx, src.StageID())
		if err != nil {
			return err
		}
		c, err := src.CloneAsDraft(s.ids.New(), next, actor.ID, s.ids.New)
		if err != nil {
			return err
		}
		if err := s.versions.Create(ctx, tx, c); err != nil {
			return err
		}
		cloneID = c.ID()
		return s.record(ctx, tx, actor, audit.ActionStageVersionCloned, "stage_version", c.ID(),
			map[string]any{"stageId": src.StageID(), "fromVersionNo": src.VersionNo(), "toVersionNo": c.VersionNo()})
	})
	if err != nil {
		return VersionDetail{}, err
	}
	return s.GetVersion(ctx, cloneID)
}

// Archive lưu trữ bản đã phát hành; chỉ đổi header, không chạm học liệu.
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
		return s.record(ctx, tx, actor, audit.ActionStageVersionArchived, "stage_version", v.ID(),
			map[string]any{"stageId": v.StageID(), "versionNo": v.VersionNo()})
	})
}

// Delete xóa bản nháp (học liệu CASCADE). Xóa phiên bản cuối cùng thì xóa luôn chặng; trả stageDeleted.
// Bản đã phát hành đang được khóa học dùng → ErrInUse kèm nơi dùng.
func (s *Service) Delete(ctx context.Context, actor Actor, versionID uuid.UUID) (bool, error) {
	var stageDeleted bool
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		v, err := s.versions.ByIDForUpdate(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if err := v.CanDelete(); err != nil {
			if v.Status() == domain.VersionPublished {
				usedBy, uerr := s.versions.UsedBy(ctx, tx, versionID)
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
		left, err := s.versions.CountByStage(ctx, tx, v.StageID())
		if err != nil {
			return err
		}
		if left == 0 {
			if err := s.stages.Delete(ctx, tx, v.StageID()); err != nil {
				return err
			}
			stageDeleted = true
		}
		return s.record(ctx, tx, actor, audit.ActionStageVersionDeleted, "stage_version", v.ID(),
			map[string]any{"stageId": v.StageID(), "versionNo": v.VersionNo(), "stageDeleted": stageDeleted})
	})
	if err != nil {
		return false, err
	}
	return stageDeleted, nil
}

// OutdatedCourses là FR-18 của một chặng (OutdatedReader).
func (s *Service) OutdatedCourses(ctx context.Context, stageID uuid.UUID) ([]OutdatedCourse, error) {
	return s.versions.OutdatedCourses(ctx, s.db, stageID)
}

// AllOutdated là FR-18 toàn hệ thống cho dashboard (OutdatedReader).
func (s *Service) AllOutdated(ctx context.Context) ([]OutdatedCourse, error) {
	return s.versions.AllOutdated(ctx, s.db)
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
func (s *Service) lockDraft(ctx context.Context, tx db.Executor, versionID uuid.UUID) (*StageVersion, error) {
	v, err := s.versions.ByIDForUpdate(ctx, tx, versionID)
	if err != nil {
		return nil, err
	}
	if !v.Status().IsMutable() {
		return nil, ErrVersionImmutable
	}
	return v, nil
}

// contentFactory dựng LessonContent từ lệnh; current là nội dung đang có (nil khi thêm mới). Đổi loại học liệu thì
// dựng mới hoàn toàn từ lệnh.
func (s *Service) contentFactory(ctx context.Context, tx db.Executor, cmd LessonCmd, current LessonContent) (LessonContent, error) {
	var typ LessonType
	switch {
	case cmd.Type != nil:
		t, err := ParseLessonType(*cmd.Type)
		if err != nil {
			return nil, err
		}
		typ = t
	case current != nil:
		typ = current.Type()
	default:
		return nil, ErrInvalidLessonType
	}

	switch typ {
	case LessonVideo:
		c, _ := current.(VideoContent)
		if cmd.VideoMediaID != nil {
			c.MediaID = *cmd.VideoMediaID
			fileName, err := s.videoFileName(ctx, tx, c.MediaID)
			if err != nil {
				return nil, err
			}
			c.FileName = fileName
		}
		if cmd.DurationSeconds != nil {
			d := *cmd.DurationSeconds
			c.DurationSeconds = &d
		}
		return c, nil
	case LessonMarkdown:
		if cmd.MarkdownSource != nil {
			// Luật ảnh chỉ áp cho nguồn mới gửi lên: bài cũ còn ảnh ngoài vẫn đổi được tiêu đề/bắt buộc.
			if err := validateMarkdownSource(*cmd.MarkdownSource); err != nil {
				return nil, err
			}
			return MarkdownContent{Source: *cmd.MarkdownSource}, nil
		}
		c, _ := current.(MarkdownContent)
		// Nội dung đang có vẫn bị kiểm base64 như trước khi có luật ảnh ngoài.
		if strings.Contains(c.Source, "data:image/") {
			return nil, ErrEmbeddedImage
		}
		return c, nil
	default:
		return nil, ErrInvalidLessonType
	}
}

// validateMarkdownSource chặn ảnh nhúng base64 rồi ảnh không phải media nội bộ trong nguồn markdown.
func validateMarkdownSource(src string) error {
	if strings.Contains(src, "data:image/") {
		return ErrEmbeddedImage
	}
	if len(foreignImages(src)) > 0 {
		return ErrExternalImage
	}
	return nil
}

// PreviewMarkdown render nguồn markdown bằng đúng renderer lúc phát hành để admin xem trước; không đọc DB và áp cùng
// luật ảnh như khi lưu.
func (s *Service) PreviewMarkdown(source string) (string, error) {
	if err := validateMarkdownSource(source); err != nil {
		return "", err
	}
	return s.render.Render(source)
}

// videoFileName kiểm media là video đã upload xong và trả tên file để hiển thị.
func (s *Service) videoFileName(ctx context.Context, tx db.Executor, id uuid.UUID) (string, error) {
	if id == uuid.Nil {
		return "", ErrVideoRequired
	}
	m, err := s.media.ByID(ctx, tx, id)
	if errors.Is(err, media.ErrMediaNotFound) {
		return "", ErrVideoRequired
	}
	if err != nil {
		return "", err
	}
	if !m.IsReady() || m.Kind() != media.KindVideo {
		return "", ErrVideoRequired
	}
	return m.FileName(), nil
}

// checkMarkdownImages bảo đảm mọi ảnh còn lại trong HTML đã render là ảnh đã upload xong, trước khi lesson_media
// tham chiếu tới chúng.
func (s *Service) checkMarkdownImages(ctx context.Context, tx db.Executor, v *StageVersion) error {
	for _, l := range v.Lessons() {
		mc, ok := l.Content().(MarkdownContent)
		if !ok || mc.HTML == nil {
			continue
		}
		for _, id := range mediaIDsInHTML(*mc.HTML) {
			m, err := s.media.ByID(ctx, tx, id)
			if errors.Is(err, media.ErrMediaNotFound) {
				return errImageNotUploaded(l.Title())
			}
			if err != nil {
				return err
			}
			if !m.IsReady() || m.Kind() != media.KindImage {
				return errImageNotUploaded(l.Title())
			}
		}
	}
	return nil
}

func (s *Service) record(ctx context.Context, tx db.Executor, actor Actor, action, targetType string, targetID uuid.UUID, after any) error {
	actorID := actor.ID
	return s.audit.Record(ctx, tx, audit.Entry{
		ActorID: &actorID, Action: action, TargetType: targetType, TargetID: &targetID, After: after, RequestID: actor.RequestID,
	})
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
