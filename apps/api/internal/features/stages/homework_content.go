package stages

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

const maxCriterionPoints = 100

// Lỗi nghiệp vụ của homework; message lấy theo prototype/app.js.
var (
	ErrNotHomework           = domain.ErrInvalid.WithMsg("Học liệu không phải homework.")
	ErrCriterionNotFound     = domain.ErrNotFound.WithMsg("Không tìm thấy tiêu chí.")
	ErrCriterionRequired     = domain.ErrInvalid.WithMsg("Nhập nội dung tiêu chí.")
	ErrCriterionPoints       = domain.ErrInvalid.WithMsg("Điểm tối đa của tiêu chí là số nguyên từ 1 đến 100.")
	ErrAttachmentNotDocument = domain.ErrInvalid.WithMsg("File kèm phải là tài liệu đã upload xong.")
	ErrAttachmentDuplicate   = domain.ErrInvalid.WithMsg("File kèm bị trùng.")
)

// errNoBrief, errNoCriteria: lý do chặn phát hành homework, nguyên văn lessonBlockers của prototype.
func errNoBrief(title string) error {
	return domain.ErrInvalid.WithMsg(fmt.Sprintf("\"%s\" chưa có đề bài.", title))
}

func errNoCriteria(title string) error {
	return domain.ErrInvalid.WithMsg(fmt.Sprintf("\"%s\" chưa có tiêu chí chấm.", title))
}

// Criterion là một tiêu chí chấm của rubric.
type Criterion struct {
	ID          uuid.UUID
	Description string
	MaxPoints   int
}

// Attachment là file kèm đề bài (media kind document); FileName và SizeBytes chỉ để hiển thị, đọc từ media_files.
type Attachment struct {
	MediaID   uuid.UUID
	FileName  string
	SizeBytes int64
}

// HomeworkContent là đề bài markdown, ngưỡng đạt, rubric theo thứ tự và file kèm theo thứ tự. Đề bài được để trống
// khi soạn; phát hành mới bắt buộc có đề bài và ít nhất một tiêu chí.
type HomeworkContent struct {
	BriefSource string
	BriefHTML   *string
	PassPercent int
	Criteria    []*Criterion
	Attachments []Attachment
}

// NewHomeworkContent là homework rỗng với ngưỡng đạt mặc định 70% như prototype.
func NewHomeworkContent() HomeworkContent { return HomeworkContent{PassPercent: defaultPassPercent} }

func (HomeworkContent) Type() LessonType { return LessonHomework }

func (c HomeworkContent) Validate() error {
	if c.PassPercent < 1 || c.PassPercent > 100 {
		return ErrPassPercent
	}
	seen := make(map[uuid.UUID]bool, len(c.Attachments))
	for _, a := range c.Attachments {
		if a.MediaID == uuid.Nil {
			return ErrAttachmentNotDocument
		}
		if seen[a.MediaID] {
			return ErrAttachmentDuplicate
		}
		seen[a.MediaID] = true
	}
	return nil
}

// MaxScore là tổng điểm tối đa các tiêu chí.
func (c HomeworkContent) MaxScore() int {
	total := 0
	for _, cr := range c.Criteria {
		total += cr.MaxPoints
	}
	return total
}

// HomeworkSettings là phần người soạn gửi; trường nil giữ giá trị đang có, Attachments rỗng khác nil là xóa hết file kèm.
type HomeworkSettings struct {
	BriefSource *string
	PassPercent *int
	Attachments []Attachment
}

// Configure trả homework với đề bài, ngưỡng đạt và file kèm mới; rubric giữ nguyên. Đổi đề bài thì bỏ HTML cũ để
// lần phát hành sau render lại.
func (c HomeworkContent) Configure(s HomeworkSettings) (HomeworkContent, error) {
	if s.BriefSource != nil {
		if src := strings.TrimSpace(*s.BriefSource); src != c.BriefSource {
			c.BriefSource, c.BriefHTML = src, nil
		}
	}
	if s.PassPercent != nil {
		c.PassPercent = *s.PassPercent
	}
	if s.Attachments != nil {
		c.Attachments = slices.Clone(s.Attachments)
	}
	return c, c.Validate()
}

// AddCriterion thêm tiêu chí vào cuối rubric của bản nháp.
func (v *StageVersion) AddCriterion(lessonID, id uuid.UUID, description string, maxPoints int) (*Criterion, error) {
	l, c, err := v.homeworkLesson(lessonID)
	if err != nil {
		return nil, err
	}
	cr, err := buildCriterion(id, description, maxPoints)
	if err != nil {
		return nil, err
	}
	c.Criteria = append(slices.Clip(c.Criteria), cr)
	l.content = c
	return cr, nil
}

// UpdateCriterion thay nội dung và điểm tối đa của tiêu chí; id giữ nguyên.
func (v *StageVersion) UpdateCriterion(lessonID, criterionID uuid.UUID, description string, maxPoints int) (*Criterion, error) {
	l, c, err := v.homeworkLesson(lessonID)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(c.Criteria, func(cr *Criterion) bool { return cr.ID == criterionID })
	if i < 0 {
		return nil, ErrCriterionNotFound
	}
	cr, err := buildCriterion(criterionID, description, maxPoints)
	if err != nil {
		return nil, err
	}
	c.Criteria = slices.Clone(c.Criteria)
	c.Criteria[i] = cr
	l.content = c
	return cr, nil
}

// RemoveCriterion xóa tiêu chí khỏi rubric của bản nháp.
func (v *StageVersion) RemoveCriterion(lessonID, criterionID uuid.UUID) error {
	l, c, err := v.homeworkLesson(lessonID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(c.Criteria, func(cr *Criterion) bool { return cr.ID == criterionID }) {
		return ErrCriterionNotFound
	}
	c.Criteria = slices.DeleteFunc(slices.Clone(c.Criteria), func(cr *Criterion) bool { return cr.ID == criterionID })
	l.content = c
	return nil
}

// ReorderCriteria sắp lại rubric; danh sách phải là hoán vị đầy đủ của tiêu chí hiện có.
func (v *StageVersion) ReorderCriteria(lessonID uuid.UUID, idsInOrder []uuid.UUID) error {
	l, c, err := v.homeworkLesson(lessonID)
	if err != nil {
		return err
	}
	ordered, err := reorderByID(c.Criteria, idsInOrder, func(cr *Criterion) uuid.UUID { return cr.ID })
	if err != nil {
		return err
	}
	c.Criteria = ordered
	l.content = c
	return nil
}

// homeworkLesson kiểm bản nháp trước rồi mới tìm học liệu, để lỗi trên bản đã phát hành luôn là ErrVersionImmutable.
func (v *StageVersion) homeworkLesson(lessonID uuid.UUID) (*Lesson, HomeworkContent, error) {
	if err := v.ensureMutable(); err != nil {
		return nil, HomeworkContent{}, err
	}
	l, err := v.Lesson(lessonID)
	if err != nil {
		return nil, HomeworkContent{}, err
	}
	c, ok := l.content.(HomeworkContent)
	if !ok {
		return nil, HomeworkContent{}, ErrNotHomework
	}
	return l, c, nil
}

func buildCriterion(id uuid.UUID, description string, maxPoints int) (*Criterion, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil, ErrCriterionRequired
	}
	if maxPoints < 1 || maxPoints > maxCriterionPoints {
		return nil, ErrCriterionPoints
	}
	return &Criterion{ID: id, Description: description, MaxPoints: maxPoints}, nil
}

// copyCriteria sao chép sâu rubric; newID khác nil thì tiêu chí nhận id mới (nhân bản).
func copyCriteria(in []*Criterion, newID func() uuid.UUID) []*Criterion {
	if in == nil {
		return nil
	}
	out := make([]*Criterion, 0, len(in))
	for _, cr := range in {
		c := *cr
		if newID != nil {
			c.ID = newID()
		}
		out = append(out, &c)
	}
	return out
}
