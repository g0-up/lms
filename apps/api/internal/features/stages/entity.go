// Package stages quản lý chặng, phiên bản chặng và học liệu. Phiên bản đã phát hành là bất biến: mọi thay đổi học liệu
// đi qua aggregate StageVersion (chặn ở domain) và câu SQL có điều kiện header còn draft (chặn ở repository).
package stages

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

// Lỗi nghiệp vụ của chặng; message lấy theo prototype/app.js.
var (
	ErrStageNameRequired   = domain.ErrInvalid.WithMsg("Nhập mã và tên chặng.")
	ErrLessonTitleRequired = domain.ErrInvalid.WithMsg("Nhập tiêu đề học liệu.")
	ErrVideoRequired       = domain.ErrInvalid.WithMsg("Học liệu video bắt buộc có file video.")
	ErrMarkdownRequired    = domain.ErrInvalid.WithMsg("Học liệu markdown bắt buộc có nội dung.")
	ErrInvalidDuration     = domain.ErrInvalid.WithMsg("Thời lượng không hợp lệ.")
	ErrInvalidLessonType   = domain.ErrInvalid.WithMsg("Loại học liệu không hợp lệ.")
	ErrNoLessons           = domain.ErrInvalid.WithMsg("Chặng cần ít nhất một học liệu trước khi phát hành.")
	ErrReorderMismatch     = domain.ErrInvalid.WithMsg("Danh sách sắp xếp không khớp.")
	ErrPublishNotDraft     = domain.ErrInvalidTransition.WithMsg("Chỉ phát hành được bản nháp.")
	ErrCloneNotPublished   = domain.ErrInvalidTransition.WithMsg("Chỉ nhân bản từ phiên bản đã phát hành.")
	ErrArchiveNotPublished = domain.ErrInvalidTransition.WithMsg("Chỉ lưu trữ phiên bản đã phát hành.")
	ErrDeleteNotDraft      = domain.ErrInvalidTransition.WithMsg("Chỉ xóa được bản nháp.")
	ErrNotRendered         = domain.ErrInvalidTransition.WithMsg("Phiên bản còn học liệu chưa render.")
	ErrVersionImmutable    = domain.ErrVersionImmutable.WithMsg("Phiên bản đã phát hành không thể chỉnh sửa.")
	ErrStageNotFound       = domain.ErrNotFound.WithMsg("Không tìm thấy chặng.")
	ErrVersionNotFound     = domain.ErrNotFound.WithMsg("Không tìm thấy phiên bản chặng.")
	ErrLessonNotFound      = domain.ErrNotFound.WithMsg("Không tìm thấy học liệu.")
	ErrCodeTaken           = domain.ErrConflict.WithMsg("Mã chặng đã tồn tại.")
	ErrLessonKeyTaken      = domain.ErrConflict.WithMsg("Mã học liệu đã tồn tại trong phiên bản.")
)

// ErrDraftExists: chặng đã có bản nháp; UI dùng DraftID để mở bản nháp đó.
type ErrDraftExists struct {
	DraftID uuid.UUID
	No      domain.VersionNo
}

func (e *ErrDraftExists) Error() string {
	return fmt.Sprintf("Chặng đã có bản nháp v%d. Phát hành hoặc xóa bản nháp trước.", e.No)
}

// ErrInUse: phiên bản không phải nháp đang được khóa học tham chiếu nên chỉ lưu trữ được.
type ErrInUse struct {
	UsedBy []UsedByRow
}

func (e *ErrInUse) Error() string {
	names := make([]string, 0, len(e.UsedBy))
	for _, u := range e.UsedBy {
		names = append(names, fmt.Sprintf("%s v%d", u.CourseName, u.VersionNo))
	}
	return "Đang được dùng trong " + strings.Join(names, ", ") + ". Hãy lưu trữ thay vì xóa."
}

// Stage là chặng; mô tả nằm trên phiên bản (stage_versions.description) nên Stage chỉ giữ để tạo bản nháp v1.
type Stage struct {
	id          uuid.UUID
	code        domain.Code
	name        string
	description string
	createdBy   uuid.UUID
	createdAt   time.Time
}

// NewStage kiểm tên chặng; mã đã được domain.ParseCode kiểm trước.
func NewStage(id uuid.UUID, code domain.Code, name, description string, by uuid.UUID, now time.Time) (*Stage, error) {
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return nil, ErrStageNameRequired
	}
	return &Stage{id: id, code: code, name: name, description: strings.TrimSpace(description), createdBy: by, createdAt: now}, nil
}

func (s *Stage) ID() uuid.UUID        { return s.id }
func (s *Stage) Code() domain.Code    { return s.code }
func (s *Stage) Name() string         { return s.name }
func (s *Stage) Description() string  { return s.description }
func (s *Stage) CreatedBy() uuid.UUID { return s.createdBy }
func (s *Stage) CreatedAt() time.Time { return s.createdAt }

// LessonType là loại học liệu; thêm loại mới = thêm struct implement LessonContent và một case trong repository_pg.
type LessonType string

const (
	LessonVideo    LessonType = "video"
	LessonMarkdown LessonType = "markdown"
)

// ParseLessonType kiểm loại học liệu từ client.
func ParseLessonType(s string) (LessonType, error) {
	switch t := LessonType(s); t {
	case LessonVideo, LessonMarkdown:
		return t, nil
	default:
		return "", ErrInvalidLessonType
	}
}

// LessonContent là nội dung riêng theo loại học liệu (Strategy).
type LessonContent interface {
	Type() LessonType
	Validate() error
}

// VideoContent tham chiếu file video đã upload; FileName chỉ để hiển thị (đọc từ media_files), không ghi xuống DB.
type VideoContent struct {
	MediaID         uuid.UUID
	DurationSeconds *int
	FileName        string
}

func (VideoContent) Type() LessonType { return LessonVideo }

func (c VideoContent) Validate() error {
	if c.MediaID == uuid.Nil {
		return ErrVideoRequired
	}
	if c.DurationSeconds != nil && *c.DurationSeconds < 0 {
		return ErrInvalidDuration
	}
	return nil
}

// MarkdownContent giữ nguồn markdown; HTML nil nghĩa là chưa render (chỉ render khi phát hành).
type MarkdownContent struct {
	Source string
	HTML   *string
}

func (MarkdownContent) Type() LessonType { return LessonMarkdown }

func (c MarkdownContent) Validate() error {
	if strings.TrimSpace(c.Source) == "" {
		return ErrMarkdownRequired
	}
	return nil
}

// Lesson là học liệu trong một phiên bản chặng; position bắt đầu từ 1 và liên tục.
type Lesson struct {
	id       uuid.UUID
	key      domain.LessonKey
	title    string
	position int
	required bool
	content  LessonContent
}

func (l *Lesson) ID() uuid.UUID          { return l.id }
func (l *Lesson) Key() domain.LessonKey  { return l.key }
func (l *Lesson) Title() string          { return l.title }
func (l *Lesson) Position() int          { return l.position }
func (l *Lesson) Required() bool         { return l.required }
func (l *Lesson) Content() LessonContent { return l.content }

// StageVersion là gốc aggregate của học liệu; chỉ bản draft được sửa.
type StageVersion struct {
	id           uuid.UUID
	stageID      uuid.UUID
	versionNo    domain.VersionNo
	status       domain.VersionStatus
	title        string
	description  string
	clonedFromID *uuid.UUID
	lessons      []*Lesson
	publishedAt  *time.Time
	archivedAt   *time.Time
	createdBy    uuid.UUID
}

// NewDraftVersion tạo bản nháp rỗng; title là tên chặng tại thời điểm tạo.
func NewDraftVersion(id, stageID uuid.UUID, no domain.VersionNo, title, description string, by uuid.UUID) *StageVersion {
	return &StageVersion{
		id: id, stageID: stageID, versionNo: no, status: domain.VersionDraft,
		title: title, description: description, createdBy: by,
	}
}

func (v *StageVersion) ID() uuid.UUID                { return v.id }
func (v *StageVersion) StageID() uuid.UUID           { return v.stageID }
func (v *StageVersion) VersionNo() domain.VersionNo  { return v.versionNo }
func (v *StageVersion) Status() domain.VersionStatus { return v.status }
func (v *StageVersion) Title() string                { return v.title }
func (v *StageVersion) Description() string          { return v.description }
func (v *StageVersion) ClonedFromID() *uuid.UUID     { return v.clonedFromID }
func (v *StageVersion) PublishedAt() *time.Time      { return v.publishedAt }
func (v *StageVersion) ArchivedAt() *time.Time       { return v.archivedAt }
func (v *StageVersion) CreatedBy() uuid.UUID         { return v.createdBy }

// Lessons trả bản sao danh sách đã sắp theo position.
func (v *StageVersion) Lessons() []*Lesson { return append([]*Lesson(nil), v.lessons...) }

// Lesson tìm học liệu theo id.
func (v *StageVersion) Lesson(id uuid.UUID) (*Lesson, error) {
	for _, l := range v.lessons {
		if l.id == id {
			return l, nil
		}
	}
	return nil, ErrLessonNotFound
}

func (v *StageVersion) ensureMutable() error {
	if !v.status.IsMutable() {
		return ErrVersionImmutable
	}
	return nil
}

// AddLesson thêm học liệu vào cuối bản nháp. key rỗng thì sinh từ tiêu đề và thêm hậu tố cho khỏi trùng.
func (v *StageVersion) AddLesson(id uuid.UUID, key domain.LessonKey, title string, content LessonContent, required bool) (*Lesson, error) {
	if err := v.ensureMutable(); err != nil {
		return nil, err
	}
	title, err := validLesson(title, content)
	if err != nil {
		return nil, err
	}
	if key == "" {
		key = v.uniqueKey(title)
	} else if v.hasKey(key) {
		return nil, ErrLessonKeyTaken
	}
	l := &Lesson{id: id, key: key, title: title, position: len(v.lessons) + 1, required: required, content: content}
	v.lessons = append(v.lessons, l)
	return l, nil
}

// UpdateLesson thay tiêu đề, nội dung và cờ bắt buộc; lesson_key và vị trí giữ nguyên.
func (v *StageVersion) UpdateLesson(id uuid.UUID, title string, content LessonContent, required bool) error {
	if err := v.ensureMutable(); err != nil {
		return err
	}
	l, err := v.Lesson(id)
	if err != nil {
		return err
	}
	title, err = validLesson(title, content)
	if err != nil {
		return err
	}
	l.title, l.content, l.required = title, content, required
	return nil
}

// RemoveLesson xóa học liệu rồi dồn position của các học liệu sau.
func (v *StageVersion) RemoveLesson(id uuid.UUID) error {
	if err := v.ensureMutable(); err != nil {
		return err
	}
	if _, err := v.Lesson(id); err != nil {
		return err
	}
	kept := v.lessons[:0:0]
	for _, l := range v.lessons {
		if l.id != id {
			kept = append(kept, l)
		}
	}
	v.lessons = kept
	v.renumber()
	return nil
}

// Reorder sắp lại theo idsInOrder; danh sách phải là hoán vị đầy đủ của học liệu hiện có.
func (v *StageVersion) Reorder(idsInOrder []uuid.UUID) error {
	if err := v.ensureMutable(); err != nil {
		return err
	}
	if len(idsInOrder) != len(v.lessons) {
		return ErrReorderMismatch
	}
	byID := make(map[uuid.UUID]*Lesson, len(v.lessons))
	for _, l := range v.lessons {
		byID[l.id] = l
	}
	ordered := make([]*Lesson, 0, len(idsInOrder))
	for _, id := range idsInOrder {
		l, ok := byID[id]
		if !ok {
			return ErrReorderMismatch
		}
		delete(byID, id)
		ordered = append(ordered, l)
	}
	v.lessons = ordered
	v.renumber()
	return nil
}

// RenderMarkdown điền HTML cho mọi học liệu markdown của bản nháp; gọi ngay trước Publish trong cùng giao dịch.
func (v *StageVersion) RenderMarkdown(render MarkdownRenderer) error {
	if err := v.ensureMutable(); err != nil {
		return err
	}
	for _, l := range v.lessons {
		mc, ok := l.content.(MarkdownContent)
		if !ok {
			continue
		}
		html, err := render.Render(mc.Source)
		if err != nil {
			return err
		}
		mc.HTML = &html
		l.content = mc
	}
	return nil
}

// Publish chỉ đổi trạng thái và thời điểm phát hành; học liệu phải hợp lệ và markdown đã render.
func (v *StageVersion) Publish(now time.Time) error {
	if v.status.Transition(domain.VersionPublished) != nil {
		return ErrPublishNotDraft
	}
	if len(v.lessons) == 0 {
		return ErrNoLessons
	}
	for _, l := range v.lessons {
		if err := l.content.Validate(); err != nil {
			return err
		}
		if mc, ok := l.content.(MarkdownContent); ok && mc.HTML == nil {
			return ErrNotRendered
		}
	}
	v.status, v.publishedAt = domain.VersionPublished, &now
	return nil
}

// Archive chỉ đổi trạng thái; học liệu không bị chạm.
func (v *StageVersion) Archive(now time.Time) error {
	if v.status.Transition(domain.VersionArchived) != nil {
		return ErrArchiveNotPublished
	}
	v.status, v.archivedAt = domain.VersionArchived, &now
	return nil
}

// CloneAsDraft sao chép sâu bản đã phát hành thành bản nháp mới: học liệu có id mới nhưng giữ lesson_key, nội dung,
// tham chiếu media (không copy file), cờ bắt buộc và thứ tự.
func (v *StageVersion) CloneAsDraft(newID uuid.UUID, nextNo domain.VersionNo, by uuid.UUID, lessonIDs func() uuid.UUID) (*StageVersion, error) {
	if v.status != domain.VersionPublished {
		return nil, ErrCloneNotPublished
	}
	from := v.id
	c := NewDraftVersion(newID, v.stageID, nextNo, v.title, v.description, by)
	c.clonedFromID = &from
	c.lessons = make([]*Lesson, 0, len(v.lessons))
	for _, l := range v.lessons {
		c.lessons = append(c.lessons, &Lesson{
			id: lessonIDs(), key: l.key, title: l.title, position: l.position, required: l.required, content: copyContent(l.content),
		})
	}
	return c, nil
}

// CanDelete: chỉ bản nháp xóa được.
func (v *StageVersion) CanDelete() error {
	if v.status != domain.VersionDraft {
		return ErrDeleteNotDraft
	}
	return nil
}

func (v *StageVersion) renumber() {
	for i, l := range v.lessons {
		l.position = i + 1
	}
}

func (v *StageVersion) hasKey(key domain.LessonKey) bool {
	for _, l := range v.lessons {
		if l.key == key {
			return true
		}
	}
	return false
}

// uniqueKey sinh lesson_key từ tiêu đề; tiêu đề không ra slug hợp lệ thì dùng "hoc-lieu".
func (v *StageVersion) uniqueKey(title string) domain.LessonKey {
	base, err := domain.ParseLessonKey(domain.Slugify(title))
	if err != nil {
		base = "hoc-lieu"
	}
	key := base
	for n := 2; v.hasKey(key); n++ {
		suffix := "-" + strconv.Itoa(n)
		head := string(base)
		if len(head)+len(suffix) > 40 {
			head = strings.TrimRight(head[:40-len(suffix)], "-")
		}
		key = domain.LessonKey(head + suffix)
	}
	return key
}

func validLesson(title string, content LessonContent) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", ErrLessonTitleRequired
	}
	if content == nil {
		return "", ErrInvalidLessonType
	}
	if err := content.Validate(); err != nil {
		return "", err
	}
	return title, nil
}

func copyContent(c LessonContent) LessonContent {
	switch t := c.(type) {
	case VideoContent:
		if t.DurationSeconds != nil {
			d := *t.DurationSeconds
			t.DurationSeconds = &d
		}
		return t
	case MarkdownContent:
		if t.HTML != nil {
			h := *t.HTML
			t.HTML = &h
		}
		return t
	default:
		return c
	}
}
