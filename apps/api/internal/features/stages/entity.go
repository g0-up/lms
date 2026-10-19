// Package stages quản lý chặng, phiên bản chặng, bài học và học liệu. Phiên bản đã phát hành là bất biến: mọi thay đổi
// bài học và học liệu đi qua aggregate StageVersion (chặn ở domain) và câu SQL có điều kiện header còn draft (chặn ở
// repository).
package stages

import (
	"cmp"
	"fmt"
	"slices"
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
	LessonQuiz     LessonType = "quiz"
	LessonHomework LessonType = "homework"
)

// ParseLessonType kiểm loại học liệu từ client.
func ParseLessonType(s string) (LessonType, error) {
	switch t := LessonType(s); t {
	case LessonVideo, LessonMarkdown, LessonQuiz, LessonHomework:
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

// Lesson là học liệu thuộc một bài học của phiên bản chặng; position là thứ tự phẳng trong cả phiên bản, bắt đầu từ 1
// và liên tục theo (vị trí bài học, vị trí trong bài học).
type Lesson struct {
	id       uuid.UUID
	unitID   uuid.UUID
	key      domain.LessonKey
	title    string
	position int
	required bool
	content  LessonContent
}

func (l *Lesson) ID() uuid.UUID          { return l.id }
func (l *Lesson) UnitID() uuid.UUID      { return l.unitID }
func (l *Lesson) Key() domain.LessonKey  { return l.key }
func (l *Lesson) Title() string          { return l.title }
func (l *Lesson) Position() int          { return l.position }
func (l *Lesson) Required() bool         { return l.required }
func (l *Lesson) Content() LessonContent { return l.content }

// StageVersion là gốc aggregate của bài học và học liệu; chỉ bản draft được sửa. Bất biến: units và lessons luôn xếp
// theo position, lessons nhóm liền theo thứ tự bài học; renumber() khôi phục bất biến sau mọi thao tác.
type StageVersion struct {
	id           uuid.UUID
	stageID      uuid.UUID
	versionNo    domain.VersionNo
	status       domain.VersionStatus
	title        string
	description  string
	clonedFromID *uuid.UUID
	units        []*Unit
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

// AddLesson thêm học liệu vào cuối bài học unitID. key rỗng thì sinh từ tiêu đề và thêm hậu tố cho khỏi trùng.
func (v *StageVersion) AddLesson(id, unitID uuid.UUID, key domain.LessonKey, title string, content LessonContent, required bool) (*Lesson, error) {
	if err := v.ensureMutable(); err != nil {
		return nil, err
	}
	if _, err := v.Unit(unitID); err != nil {
		return nil, ErrLessonUnitRequired
	}
	title, err := validLesson(title, content)
	if err != nil {
		return nil, err
	}
	if key == "" {
		key = uniqueKey(title, "hoc-lieu", v.hasKey)
	} else if v.hasKey(key) {
		return nil, ErrLessonKeyTaken
	}
	l := &Lesson{id: id, unitID: unitID, key: key, title: title, required: required, content: content}
	v.lessons = append(v.lessons, l)
	v.renumber()
	return l, nil
}

// UpdateLesson thay tiêu đề, nội dung và cờ bắt buộc; lesson_key và vị trí giữ nguyên (chuyển bài học bằng MoveLesson).
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
	v.lessons = slices.DeleteFunc(v.lessons, func(l *Lesson) bool { return l.id == id })
	v.renumber()
	return nil
}

// RenderMarkdown điền HTML cho mọi nguồn markdown của bản nháp (học liệu markdown, nội dung và giải thích câu hỏi,
// đề bài homework); gọi ngay trước Publish trong cùng giao dịch.
func (v *StageVersion) RenderMarkdown(render MarkdownRenderer) error {
	if err := v.ensureMutable(); err != nil {
		return err
	}
	for _, l := range v.lessons {
		c, err := renderContent(l.content, render)
		if err != nil {
			return err
		}
		l.content = c
	}
	return nil
}

// Publish chỉ đổi trạng thái và thời điểm phát hành. Kiểm theo thứ tự của prototype: có bài học, không bài học nào
// rỗng, rồi học liệu hợp lệ, đủ nội dung (quiz có câu hỏi và đáp án đúng, homework có đề bài và rubric) và markdown
// đã render.
func (v *StageVersion) Publish(now time.Time) error {
	if v.status.Transition(domain.VersionPublished) != nil {
		return ErrPublishNotDraft
	}
	if len(v.units) == 0 {
		return ErrNoUnits
	}
	for _, u := range v.units {
		if len(v.LessonsOf(u.id)) == 0 {
			return ErrEmptyUnit(u.title)
		}
	}
	for _, l := range v.lessons {
		if err := l.content.Validate(); err != nil {
			return err
		}
		if err := publishBlocker(l); err != nil {
			return err
		}
		if !rendered(l.content) {
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

// CloneAsDraft sao chép sâu bản đã phát hành thành bản nháp mới: bài học, học liệu, câu hỏi, phương án và tiêu chí có
// id mới nhưng giữ unit_key, lesson_key, question_key, nội dung, tham chiếu media (không copy file), cờ bắt buộc và
// thứ tự; học liệu trỏ sang bài học mới.
func (v *StageVersion) CloneAsDraft(newID uuid.UUID, nextNo domain.VersionNo, by uuid.UUID, newIDs func() uuid.UUID) (*StageVersion, error) {
	if v.status != domain.VersionPublished {
		return nil, ErrCloneNotPublished
	}
	from := v.id
	c := NewDraftVersion(newID, v.stageID, nextNo, v.title, v.description, by)
	c.clonedFromID = &from
	unitIDs := make(map[uuid.UUID]uuid.UUID, len(v.units))
	c.units = make([]*Unit, 0, len(v.units))
	for _, u := range v.units {
		nu := &Unit{id: newIDs(), key: u.key, title: u.title, position: u.position}
		unitIDs[u.id] = nu.id
		c.units = append(c.units, nu)
	}
	c.lessons = make([]*Lesson, 0, len(v.lessons))
	for _, l := range v.lessons {
		c.lessons = append(c.lessons, &Lesson{
			id: newIDs(), unitID: unitIDs[l.unitID], key: l.key, title: l.title, position: l.position, required: l.required,
			content: cloneContent(l.content, newIDs),
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

// renumber khôi phục bất biến: bài học đánh số 1..m theo thứ tự hiện có, học liệu xếp ổn định theo vị trí bài học
// (giữ thứ tự tương đối trong bài học) rồi đánh số phẳng 1..n.
func (v *StageVersion) renumber() {
	unitPos := make(map[uuid.UUID]int, len(v.units))
	for i, u := range v.units {
		u.position = i + 1
		unitPos[u.id] = u.position
	}
	slices.SortStableFunc(v.lessons, func(a, b *Lesson) int { return cmp.Compare(unitPos[a.unitID], unitPos[b.unitID]) })
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

// uniqueKey sinh lesson_key/unit_key từ tiêu đề; tiêu đề không ra slug hợp lệ thì dùng fallback. taken báo key đã
// có trong phiên bản để thêm hậu tố -2, -3…
func uniqueKey(title string, fallback domain.LessonKey, taken func(domain.LessonKey) bool) domain.LessonKey {
	base, err := domain.ParseLessonKey(domain.Slugify(title))
	if err != nil {
		base = fallback
	}
	key := base
	for n := 2; taken(key); n++ {
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

// copyContent sao chép sâu nội dung, giữ nguyên id con.
func copyContent(c LessonContent) LessonContent { return cloneContent(c, nil) }

// cloneContent sao chép sâu nội dung; newID khác nil thì câu hỏi, phương án và tiêu chí nhận id mới.
func cloneContent(c LessonContent, newID func() uuid.UUID) LessonContent {
	switch t := c.(type) {
	case VideoContent:
		t.DurationSeconds = copyPtr(t.DurationSeconds)
		return t
	case MarkdownContent:
		t.HTML = copyPtr(t.HTML)
		return t
	case QuizContent:
		t.MaxAttempts, t.TimeLimit, t.PassPercent = copyPtr(t.MaxAttempts), copyPtr(t.TimeLimit), copyPtr(t.PassPercent)
		t.Questions = copyQuestions(t.Questions, newID)
		return t
	case HomeworkContent:
		t.BriefHTML = copyPtr(t.BriefHTML)
		t.Criteria = copyCriteria(t.Criteria, newID)
		t.Attachments = slices.Clone(t.Attachments)
		return t
	default:
		return c
	}
}

// renderContent trả bản sao nội dung đã render mọi nguồn markdown; nội dung không có markdown giữ nguyên.
func renderContent(c LessonContent, render MarkdownRenderer) (LessonContent, error) {
	var err error
	one := func(src string) *string {
		if err != nil {
			return nil
		}
		var html string
		html, err = render.Render(src)
		return &html
	}
	switch t := c.(type) {
	case MarkdownContent:
		t.HTML = one(t.Source)
		return t, err
	case QuizContent:
		t.Questions = copyQuestions(t.Questions, nil)
		for _, q := range t.Questions {
			q.BodyHTML, q.ExplanationHTML = one(q.BodySource), one(q.ExplanationSource)
		}
		return t, err
	case HomeworkContent:
		t.BriefHTML = one(t.BriefSource)
		return t, err
	default:
		return c, nil
	}
}

// rendered báo mọi nguồn markdown của nội dung đã có HTML.
func rendered(c LessonContent) bool {
	switch t := c.(type) {
	case MarkdownContent:
		return t.HTML != nil
	case QuizContent:
		return !slices.ContainsFunc(t.Questions, func(q *Question) bool { return q.BodyHTML == nil || q.ExplanationHTML == nil })
	case HomeworkContent:
		return t.BriefHTML != nil
	default:
		return true
	}
}

// renderedHTML trả mọi HTML đã render của nội dung, để kiểm ảnh và ghi lesson_media.
func renderedHTML(c LessonContent) []string {
	var out []string
	add := func(h *string) {
		if h != nil {
			out = append(out, *h)
		}
	}
	switch t := c.(type) {
	case MarkdownContent:
		add(t.HTML)
	case QuizContent:
		for _, q := range t.Questions {
			add(q.BodyHTML)
			add(q.ExplanationHTML)
		}
	case HomeworkContent:
		add(t.BriefHTML)
	}
	return out
}

// publishBlocker trả lý do học liệu chưa đủ nội dung để phát hành, theo lessonBlockers của prototype.
func publishBlocker(l *Lesson) error {
	switch t := l.content.(type) {
	case QuizContent:
		if len(t.Questions) == 0 {
			return errNoQuestions(l.title)
		}
		bad := 0
		for _, q := range t.Questions {
			if !q.hasCorrect() {
				bad++
			}
		}
		if bad > 0 {
			return errUnansweredQuestions(l.title, bad)
		}
	case HomeworkContent:
		if strings.TrimSpace(t.BriefSource) == "" {
			return errNoBrief(l.title)
		}
		if len(t.Criteria) == 0 {
			return errNoCriteria(l.title)
		}
	}
	return nil
}
