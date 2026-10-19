package stages

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

// Service là use case của chặng, phiên bản chặng, bài học và học liệu. Mọi use case ghi chạy trong một transaction mở bằng
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

// LessonCmd là dữ liệu thêm/sửa học liệu; nil = không gửi (PATCH giữ giá trị cũ). UnitID bắt buộc khi thêm; khi
// sửa mà khác bài học hiện tại thì học liệu chuyển xuống cuối bài học mới.
type LessonCmd struct {
	UnitID          *uuid.UUID
	LessonKey       *string
	Title           *string
	Type            *string
	Required        *bool
	DurationSeconds *int
	VideoMediaID    *uuid.UUID
	MarkdownSource  *string
	Quiz            *QuizCmd
	Homework        *HomeworkCmd
}

// QuizCmd là cấu hình quiz gửi kèm học liệu; nil = giữ nguyên. Thời gian tính bằng phút như form soạn.
type QuizCmd struct {
	Mode             *string
	MaxAttempts      *int
	TimeLimitMinutes *int
	Shuffle          *bool
	PassPercent      *int
}

// HomeworkCmd là đề bài, ngưỡng đạt và file kèm gửi kèm học liệu; nil = giữ nguyên, AttachmentMediaIDs rỗng khác nil
// là xóa hết file kèm.
type HomeworkCmd struct {
	BriefSource        *string
	PassPercent        *int
	AttachmentMediaIDs []uuid.UUID
}

// QuestionCmd là toàn bộ một câu hỏi người soạn gửi (thêm và sửa đều gửi đủ, như form của prototype). QuestionKey
// chỉ dùng khi thêm; khi sửa phải rỗng hoặc trùng mã cũ.
type QuestionCmd struct {
	QuestionKey       *string
	Type              string
	BodySource        string
	ExplanationSource string
	Points            int
	Options           []OptionCmd
}

// OptionCmd là một phương án; ID nil là phương án mới.
type OptionCmd struct {
	ID      *uuid.UUID
	Body    string
	Correct bool
}

// CriterionCmd là một tiêu chí chấm.
type CriterionCmd struct {
	Description string
	MaxPoints   int
}

// StageDetail là chặng kèm phiên bản, nơi dùng và khóa học dùng phiên bản cũ (FR-18).
type StageDetail struct {
	Stage    StageRow
	Versions []VersionSummary
	UsedBy   []StageUsedByRow
	Outdated []OutdatedCourse
}

// UnitDetail là bài học kèm học liệu của nó, trả cho route soạn bài học.
type UnitDetail struct {
	Unit    *Unit
	Lessons []*Lesson
}

// VersionDetail là phiên bản chặng kèm bài học, học liệu và nơi dùng.
type VersionDetail struct {
	Version             *StageVersion
	Stage               StageRow
	ClonedFromVersionNo *domain.VersionNo
	UsedBy              []UsedByRow
}

// CreateStage tạo chặng cùng bản nháp v1 rỗng (chưa có bài học nào).
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

// AddLesson thêm học liệu vào cuối bài học được chọn của bản nháp.
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
		unitID := uuid.Nil
		if cmd.UnitID != nil {
			unitID = *cmd.UnitID
		}
		if added, err = v.AddLesson(s.ids.New(), unitID, key, deref(cmd.Title), content, required); err != nil {
			return err
		}
		return s.versions.SaveDraft(ctx, tx, v)
	})
	if err != nil {
		return nil, err
	}
	return added, nil
}

// UpdateLesson sửa học liệu trong bản nháp; trường không gửi giữ nguyên. unitId khác bài học hiện tại thì chuyển học
// liệu xuống cuối bài học mới.
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
		if cmd.UnitID != nil {
			if err := v.MoveLesson(lessonID, *cmd.UnitID); err != nil {
				return err
			}
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

// ReorderLessons áp bố cục đầy đủ {bài học, học liệu theo thứ tự} cho bản nháp; học liệu có thể đổi bài học.
func (s *Service) ReorderLessons(ctx context.Context, _ Actor, versionID uuid.UUID, layout []UnitLessons) (VersionDetail, error) {
	return s.mutateDraft(ctx, versionID, func(v *StageVersion) error { return v.ReorderLessons(layout) })
}

// AddUnit thêm bài học vào cuối bản nháp.
func (s *Service) AddUnit(ctx context.Context, _ Actor, versionID uuid.UUID, title string) (UnitDetail, error) {
	var added *Unit
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		v, err := s.lockDraft(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if added, err = v.AddUnit(s.ids.New(), title); err != nil {
			return err
		}
		return s.versions.SaveDraft(ctx, tx, v)
	})
	if err != nil {
		return UnitDetail{}, err
	}
	return UnitDetail{Unit: added}, nil
}

// RenameUnit đổi tên bài học trong bản nháp.
func (s *Service) RenameUnit(ctx context.Context, _ Actor, versionID, unitID uuid.UUID, title string) (UnitDetail, error) {
	var d UnitDetail
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		v, err := s.lockDraft(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if err := v.RenameUnit(unitID, title); err != nil {
			return err
		}
		if err := s.versions.SaveDraft(ctx, tx, v); err != nil {
			return err
		}
		u, err := v.Unit(unitID)
		if err != nil {
			return err
		}
		d = UnitDetail{Unit: u, Lessons: v.LessonsOf(unitID)}
		return nil
	})
	if err != nil {
		return UnitDetail{}, err
	}
	return d, nil
}

// RemoveUnit xóa bài học cùng học liệu trong đó khỏi bản nháp; trả số học liệu bị xóa.
func (s *Service) RemoveUnit(ctx context.Context, _ Actor, versionID, unitID uuid.UUID) (int, error) {
	var removed int
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		v, err := s.lockDraft(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if removed, err = v.RemoveUnit(unitID); err != nil {
			return err
		}
		return s.versions.SaveDraft(ctx, tx, v)
	})
	if err != nil {
		return 0, err
	}
	return removed, nil
}

// ReorderUnits sắp lại bài học của bản nháp; học liệu đi theo bài học.
func (s *Service) ReorderUnits(ctx context.Context, _ Actor, versionID uuid.UUID, unitIDs []uuid.UUID) (VersionDetail, error) {
	return s.mutateDraft(ctx, versionID, func(v *StageVersion) error { return v.ReorderUnits(unitIDs) })
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

// AddQuestion thêm câu hỏi vào cuối quiz của bản nháp; trả học liệu sau khi ghi.
func (s *Service) AddQuestion(ctx context.Context, _ Actor, versionID, lessonID uuid.UUID, cmd QuestionCmd) (*Lesson, error) {
	return s.mutateLesson(ctx, versionID, lessonID, func(v *StageVersion) error {
		in, err := cmd.input()
		if err != nil {
			return err
		}
		_, err = v.AddQuestion(lessonID, s.ids.New(), in, s.ids.New)
		return err
	})
}

// UpdateQuestion thay nội dung, điểm và phương án của câu hỏi trong bản nháp.
func (s *Service) UpdateQuestion(ctx context.Context, _ Actor, versionID, lessonID, questionID uuid.UUID, cmd QuestionCmd) (*Lesson, error) {
	return s.mutateLesson(ctx, versionID, lessonID, func(v *StageVersion) error {
		in, err := cmd.input()
		if err != nil {
			return err
		}
		_, err = v.UpdateQuestion(lessonID, questionID, in, s.ids.New)
		return err
	})
}

// RemoveQuestion xóa câu hỏi khỏi quiz của bản nháp.
func (s *Service) RemoveQuestion(ctx context.Context, _ Actor, versionID, lessonID, questionID uuid.UUID) error {
	_, err := s.mutateLesson(ctx, versionID, lessonID, func(v *StageVersion) error {
		return v.RemoveQuestion(lessonID, questionID)
	})
	return err
}

// ReorderQuestions sắp lại câu hỏi của quiz trong bản nháp.
func (s *Service) ReorderQuestions(ctx context.Context, _ Actor, versionID, lessonID uuid.UUID, questionIDs []uuid.UUID) (*Lesson, error) {
	return s.mutateLesson(ctx, versionID, lessonID, func(v *StageVersion) error {
		return v.ReorderQuestions(lessonID, questionIDs)
	})
}

// AddCriterion thêm tiêu chí vào cuối rubric của homework trong bản nháp.
func (s *Service) AddCriterion(ctx context.Context, _ Actor, versionID, lessonID uuid.UUID, cmd CriterionCmd) (*Lesson, error) {
	return s.mutateLesson(ctx, versionID, lessonID, func(v *StageVersion) error {
		_, err := v.AddCriterion(lessonID, s.ids.New(), cmd.Description, cmd.MaxPoints)
		return err
	})
}

// UpdateCriterion sửa nội dung và điểm tối đa của tiêu chí trong bản nháp.
func (s *Service) UpdateCriterion(ctx context.Context, _ Actor, versionID, lessonID, criterionID uuid.UUID, cmd CriterionCmd) (*Lesson, error) {
	return s.mutateLesson(ctx, versionID, lessonID, func(v *StageVersion) error {
		_, err := v.UpdateCriterion(lessonID, criterionID, cmd.Description, cmd.MaxPoints)
		return err
	})
}

// RemoveCriterion xóa tiêu chí khỏi rubric của bản nháp.
func (s *Service) RemoveCriterion(ctx context.Context, _ Actor, versionID, lessonID, criterionID uuid.UUID) error {
	_, err := s.mutateLesson(ctx, versionID, lessonID, func(v *StageVersion) error {
		return v.RemoveCriterion(lessonID, criterionID)
	})
	return err
}

// ReorderCriteria sắp lại rubric của homework trong bản nháp.
func (s *Service) ReorderCriteria(ctx context.Context, _ Actor, versionID, lessonID uuid.UUID, criterionIDs []uuid.UUID) (*Lesson, error) {
	return s.mutateLesson(ctx, versionID, lessonID, func(v *StageVersion) error {
		return v.ReorderCriteria(lessonID, criterionIDs)
	})
}

// input kiểm mã, loại và nguồn markdown của câu hỏi rồi dựng QuestionInput cho aggregate.
func (c QuestionCmd) input() (QuestionInput, error) {
	var in QuestionInput
	if c.QuestionKey != nil && strings.TrimSpace(*c.QuestionKey) != "" {
		key, err := ParseQuestionKey(strings.TrimSpace(*c.QuestionKey))
		if err != nil {
			return in, err
		}
		in.Key = key
	}
	typ, err := ParseQuestionType(c.Type)
	if err != nil {
		return in, err
	}
	for _, src := range []string{c.BodySource, c.ExplanationSource} {
		if err := validateMarkdownSource(src); err != nil {
			return in, err
		}
	}
	in.Type, in.BodySource, in.ExplanationSource, in.Points = typ, c.BodySource, c.ExplanationSource, c.Points
	in.Options = make([]Option, 0, len(c.Options))
	for _, o := range c.Options {
		opt := Option{Body: o.Body, Correct: o.Correct}
		if o.ID != nil {
			opt.ID = *o.ID
		}
		in.Options = append(in.Options, opt)
	}
	return in, nil
}

// mutateLesson khóa bản nháp, áp fn rồi ghi lại; trả học liệu lessonID sau khi ghi.
func (s *Service) mutateLesson(ctx context.Context, versionID, lessonID uuid.UUID, fn func(v *StageVersion) error) (*Lesson, error) {
	var l *Lesson
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		v, err := s.lockDraft(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if err := fn(v); err != nil {
			return err
		}
		if err := s.versions.SaveDraft(ctx, tx, v); err != nil {
			return err
		}
		l, err = v.Lesson(lessonID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return l, nil
}

// mutateVersion chạy fn trong transaction rồi đọc lại phiên bản sau commit.
func (s *Service) mutateVersion(ctx context.Context, versionID uuid.UUID, fn func(tx db.Executor) error) (VersionDetail, error) {
	if err := s.tx.Transact(ctx, fn); err != nil {
		return VersionDetail{}, err
	}
	return s.GetVersion(ctx, versionID)
}

// mutateDraft khóa bản nháp, áp fn lên aggregate, ghi lại rồi đọc phiên bản sau commit.
func (s *Service) mutateDraft(ctx context.Context, versionID uuid.UUID, fn func(v *StageVersion) error) (VersionDetail, error) {
	return s.mutateVersion(ctx, versionID, func(tx db.Executor) error {
		v, err := s.lockDraft(ctx, tx, versionID)
		if err != nil {
			return err
		}
		if err := fn(v); err != nil {
			return err
		}
		return s.versions.SaveDraft(ctx, tx, v)
	})
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
	case LessonQuiz:
		// Quiz → quiz giữ câu hỏi, chỉ đổi cấu hình; loại khác chuyển sang quiz thì bắt đầu từ quiz rỗng.
		base, ok := current.(QuizContent)
		if !ok {
			base = NewQuizContent()
		}
		settings, err := cmd.Quiz.settings()
		if err != nil {
			return nil, err
		}
		return base.Configure(settings)
	case LessonHomework:
		base, ok := current.(HomeworkContent)
		if !ok {
			base = NewHomeworkContent()
		}
		settings, err := s.homeworkSettings(ctx, tx, cmd.Homework)
		if err != nil {
			return nil, err
		}
		return base.Configure(settings)
	default:
		return nil, ErrInvalidLessonType
	}
}

// settings đổi cấu hình quiz từ lệnh sang domain; phút ngoài 1–180 bị chặn trước khi nhân để không tràn số.
func (c *QuizCmd) settings() (QuizSettings, error) {
	var out QuizSettings
	if c == nil {
		return out, nil
	}
	if c.Mode != nil {
		mode, err := ParseQuizMode(*c.Mode)
		if err != nil {
			return out, err
		}
		out.Mode = &mode
	}
	if c.TimeLimitMinutes != nil {
		m := *c.TimeLimitMinutes
		if m < 1 || time.Duration(m)*time.Minute > maxQuizTimeLimit {
			return out, ErrQuizTimeLimit
		}
		d := time.Duration(m) * time.Minute
		out.TimeLimit = &d
	}
	out.MaxAttempts, out.Shuffle, out.PassPercent = c.MaxAttempts, c.Shuffle, c.PassPercent
	return out, nil
}

// homeworkSettings kiểm nguồn đề bài và đổi id file kèm thành tài liệu đã upload xong (lấy tên, dung lượng hiển thị).
func (s *Service) homeworkSettings(ctx context.Context, tx db.Executor, c *HomeworkCmd) (HomeworkSettings, error) {
	var out HomeworkSettings
	if c == nil {
		return out, nil
	}
	if c.BriefSource != nil {
		if err := validateMarkdownSource(*c.BriefSource); err != nil {
			return out, err
		}
	}
	out.BriefSource, out.PassPercent = c.BriefSource, c.PassPercent
	if c.AttachmentMediaIDs != nil {
		out.Attachments = make([]Attachment, 0, len(c.AttachmentMediaIDs))
		for _, id := range c.AttachmentMediaIDs {
			m, err := s.media.ByID(ctx, tx, id)
			if errors.Is(err, media.ErrMediaNotFound) {
				return out, ErrAttachmentNotDocument
			}
			if err != nil {
				return out, err
			}
			if !m.IsReady() || m.Kind() != media.KindDocument {
				return out, ErrAttachmentNotDocument
			}
			out.Attachments = append(out.Attachments, Attachment{MediaID: id, FileName: m.FileName(), SizeBytes: m.SizeBytes()})
		}
	}
	return out, nil
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

// checkMarkdownImages bảo đảm mọi ảnh còn lại trong HTML đã render (bài đọc, câu hỏi, lời giải, đề bài) là ảnh đã
// upload xong, trước khi lesson_media tham chiếu tới chúng.
func (s *Service) checkMarkdownImages(ctx context.Context, tx db.Executor, v *StageVersion) error {
	for _, l := range v.Lessons() {
		var ids []uuid.UUID
		for _, html := range renderedHTML(l.Content()) {
			ids = append(ids, mediaIDsInHTML(html)...)
		}
		for _, id := range ids {
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
