package stages

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

// Giới hạn của quiz; khớp CHECK của quiz_settings, questions và question_options.
const (
	maxQuizAttempts      = 10
	maxQuizTimeLimit     = 180 * time.Minute
	minQuestionOptions   = 2
	maxQuestionOptions   = 6
	maxOptionBodyLen     = 500
	maxQuestionPoints    = 100
	defaultQuizAttempts  = 2
	defaultQuizTimeLimit = 20 * time.Minute
	defaultPassPercent   = 70
)

// Lỗi nghiệp vụ của quiz; message lấy theo prototype/app.js.
var (
	ErrInvalidQuizMode      = domain.ErrInvalid.WithMsg("Chế độ quiz không hợp lệ.")
	ErrQuizAttempts         = domain.ErrInvalid.WithMsg("Số lượt làm bài từ 1 đến 10.")
	ErrQuizTimeLimit        = domain.ErrInvalid.WithMsg("Thời gian làm bài từ 1 đến 180 phút.")
	ErrPassPercent          = domain.ErrInvalid.WithMsg("Ngưỡng đạt là số nguyên từ 1 đến 100%.")
	ErrPracticeHasLimits    = domain.ErrInvalid.WithMsg("Quiz luyện tập không giới hạn lượt, thời gian hay ngưỡng đạt.")
	ErrNotQuiz              = domain.ErrInvalid.WithMsg("Học liệu không phải quiz.")
	ErrQuestionNotFound     = domain.ErrNotFound.WithMsg("Không tìm thấy câu hỏi.")
	ErrQuestionBodyRequired = domain.ErrInvalid.WithMsg("Nhập nội dung câu hỏi.")
	ErrInvalidQuestionType  = domain.ErrInvalid.WithMsg("Loại câu hỏi không hợp lệ.")
	ErrQuestionPoints       = domain.ErrInvalid.WithMsg("Điểm của câu hỏi là số nguyên từ 1 đến 100.")
	ErrTooFewOptions        = domain.ErrInvalid.WithMsg("Câu hỏi cần ít nhất 2 phương án.")
	ErrTooManyOptions       = domain.ErrInvalid.WithMsg("Câu hỏi có tối đa 6 phương án.")
	ErrOptionBody           = domain.ErrInvalid.WithMsg("Nội dung phương án từ 1 đến 500 ký tự.")
	ErrNoCorrectOption      = domain.ErrInvalid.WithMsg("Đánh dấu ít nhất một đáp án đúng.")
	ErrSingleManyCorrect    = domain.ErrInvalid.WithMsg(`Câu một đáp án chỉ có một phương án đúng. Đổi sang "Nhiều đáp án" nếu cần.`)
	ErrOptionNotInQuestion  = domain.ErrInvalid.WithMsg("Phương án không thuộc câu hỏi này.")
	ErrInvalidQuestionKey   = domain.ErrInvalid.WithMsg("Mã câu hỏi chỉ gồm chữ thường không dấu, số hoặc dấu - (2–40 ký tự).")
	ErrQuestionKeyImmutable = domain.ErrInvalid.WithMsg("Không đổi được mã câu hỏi sau khi tạo.")
	ErrQuestionKeyTaken     = domain.ErrConflict.WithMsg("Mã câu hỏi đã có trong quiz này.")
)

// errNoQuestions, errUnansweredQuestions: lý do chặn phát hành quiz, nguyên văn lessonBlockers của prototype.
func errNoQuestions(title string) error {
	return domain.ErrInvalid.WithMsg(fmt.Sprintf("\"%s\" chưa có câu hỏi.", title))
}

func errUnansweredQuestions(title string, n int) error {
	return domain.ErrInvalid.WithMsg(fmt.Sprintf("\"%s\" có %d câu chưa đánh dấu đáp án đúng.", title, n))
}

// QuizMode: practice là quiz học liệu (không giới hạn, xem đáp án sau khi nộp), assessment là quiz bài tập.
type QuizMode string

const (
	QuizPractice   QuizMode = "practice"
	QuizAssessment QuizMode = "assessment"
)

// ParseQuizMode kiểm chế độ quiz từ client.
func ParseQuizMode(s string) (QuizMode, error) {
	switch m := QuizMode(s); m {
	case QuizPractice, QuizAssessment:
		return m, nil
	default:
		return "", ErrInvalidQuizMode
	}
}

// QuestionType: single có đúng một đáp án đúng, multi có một hoặc nhiều.
type QuestionType string

const (
	QuestionSingle QuestionType = "single"
	QuestionMulti  QuestionType = "multi"
)

// ParseQuestionType kiểm loại câu hỏi từ client.
func ParseQuestionType(s string) (QuestionType, error) {
	switch t := QuestionType(s); t {
	case QuestionSingle, QuestionMulti:
		return t, nil
	default:
		return "", ErrInvalidQuestionType
	}
}

// ParseQuestionKey kiểm question_key theo cùng quy tắc với lesson_key (ck_questions_key).
func ParseQuestionKey(s string) (domain.LessonKey, error) {
	k, err := domain.ParseLessonKey(s)
	if err != nil {
		return "", ErrInvalidQuestionKey
	}
	return k, nil
}

// Option là một phương án của câu hỏi.
type Option struct {
	ID      uuid.UUID
	Body    string
	Correct bool
}

// Question là câu hỏi trắc nghiệm. Key giữ nguyên khi nhân bản để so thống kê câu hỏi giữa các phiên bản; HTML nil
// nghĩa là chưa render (chỉ render khi phát hành).
type Question struct {
	ID                uuid.UUID
	Key               domain.LessonKey
	Type              QuestionType
	BodySource        string
	BodyHTML          *string
	ExplanationSource string
	ExplanationHTML   *string
	Points            int
	Options           []Option
}

// hasCorrect báo câu hỏi đã có ít nhất một đáp án đúng.
func (q *Question) hasCorrect() bool {
	return slices.ContainsFunc(q.Options, func(o Option) bool { return o.Correct })
}

// QuizContent là cấu hình quiz và danh sách câu hỏi theo thứ tự. Luyện tập: ba giới hạn đều nil. Đánh giá: đủ cả ba.
type QuizContent struct {
	Mode        QuizMode
	MaxAttempts *int
	TimeLimit   *time.Duration
	Shuffle     bool
	PassPercent *int
	Questions   []*Question
}

// NewQuizContent là quiz luyện tập rỗng, đảo thứ tự bật như form của prototype.
func NewQuizContent() QuizContent { return QuizContent{Mode: QuizPractice, Shuffle: true} }

func (QuizContent) Type() LessonType { return LessonQuiz }

// Validate kiểm cấu hình theo chế độ; câu hỏi được kiểm lúc thêm/sửa, còn đủ câu và đáp án là điều kiện phát hành.
func (c QuizContent) Validate() error {
	switch c.Mode {
	case QuizPractice:
		if c.MaxAttempts != nil || c.TimeLimit != nil || c.PassPercent != nil {
			return ErrPracticeHasLimits
		}
	case QuizAssessment:
		if c.MaxAttempts == nil || *c.MaxAttempts < 1 || *c.MaxAttempts > maxQuizAttempts {
			return ErrQuizAttempts
		}
		if t := c.TimeLimit; t == nil || *t < time.Minute || *t > maxQuizTimeLimit || *t%time.Minute != 0 {
			return ErrQuizTimeLimit
		}
		if !validPercent(c.PassPercent) {
			return ErrPassPercent
		}
	default:
		return ErrInvalidQuizMode
	}
	return nil
}

// MaxScore là tổng điểm các câu hỏi.
func (c QuizContent) MaxScore() int {
	total := 0
	for _, q := range c.Questions {
		total += q.Points
	}
	return total
}

// QuizSettings là cấu hình người soạn gửi; trường nil giữ giá trị đang có.
type QuizSettings struct {
	Mode        *QuizMode
	MaxAttempts *int
	TimeLimit   *time.Duration
	Shuffle     *bool
	PassPercent *int
}

// Configure trả quiz với cấu hình mới và giữ nguyên câu hỏi, nên đổi giữa luyện tập và đánh giá không mất câu hỏi.
// Luyện tập bỏ mọi giới hạn; đánh giá lấy giới hạn gửi lên, rồi giới hạn đang có, rồi mặc định 2 lượt, 20 phút, 70%.
func (c QuizContent) Configure(s QuizSettings) (QuizContent, error) {
	if s.Mode != nil {
		c.Mode = *s.Mode
	}
	if s.Shuffle != nil {
		c.Shuffle = *s.Shuffle
	}
	if c.Mode != QuizAssessment {
		c.MaxAttempts, c.TimeLimit, c.PassPercent = nil, nil, nil
		return c, c.Validate()
	}
	c.MaxAttempts = firstOf(s.MaxAttempts, c.MaxAttempts, defaultQuizAttempts)
	c.TimeLimit = firstOf(s.TimeLimit, c.TimeLimit, defaultQuizTimeLimit)
	c.PassPercent = firstOf(s.PassPercent, c.PassPercent, defaultPassPercent)
	return c, c.Validate()
}

// firstOf trả bản sao của giá trị đầu tiên khác nil, không có thì fallback.
func firstOf[T any](sent, current *T, fallback T) *T {
	v := fallback
	switch {
	case sent != nil:
		v = *sent
	case current != nil:
		v = *current
	}
	return &v
}

func validPercent(p *int) bool { return p != nil && *p >= 1 && *p <= 100 }

// QuestionInput là dữ liệu một câu hỏi từ người soạn. Key chỉ dùng khi thêm (rỗng = sinh tự động); phương án có ID
// uuid.Nil là phương án mới, ID khác phải là phương án đang có của chính câu hỏi đó.
type QuestionInput struct {
	Key               domain.LessonKey
	Type              QuestionType
	BodySource        string
	ExplanationSource string
	Points            int
	Options           []Option
}

// AddQuestion thêm câu hỏi vào cuối quiz của bản nháp.
func (v *StageVersion) AddQuestion(lessonID, id uuid.UUID, in QuestionInput, newID func() uuid.UUID) (*Question, error) {
	l, c, err := v.quizLesson(lessonID)
	if err != nil {
		return nil, err
	}
	taken := func(k domain.LessonKey) bool {
		return slices.ContainsFunc(c.Questions, func(q *Question) bool { return q.Key == k })
	}
	key := in.Key
	if key == "" {
		key = uniqueKey("", "cau-hoi", taken)
	} else if taken(key) {
		return nil, ErrQuestionKeyTaken
	}
	q, err := buildQuestion(id, key, in, nil, newID)
	if err != nil {
		return nil, err
	}
	c.Questions = append(slices.Clip(c.Questions), q)
	l.content = c
	return q, nil
}

// UpdateQuestion thay toàn bộ nội dung, điểm và phương án của câu hỏi; id và question_key giữ nguyên.
func (v *StageVersion) UpdateQuestion(lessonID, questionID uuid.UUID, in QuestionInput, newID func() uuid.UUID) (*Question, error) {
	l, c, err := v.quizLesson(lessonID)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(c.Questions, func(q *Question) bool { return q.ID == questionID })
	if i < 0 {
		return nil, ErrQuestionNotFound
	}
	prev := c.Questions[i]
	if in.Key != "" && in.Key != prev.Key {
		return nil, ErrQuestionKeyImmutable
	}
	q, err := buildQuestion(prev.ID, prev.Key, in, prev, newID)
	if err != nil {
		return nil, err
	}
	c.Questions = slices.Clone(c.Questions)
	c.Questions[i] = q
	l.content = c
	return q, nil
}

// RemoveQuestion xóa câu hỏi khỏi quiz của bản nháp.
func (v *StageVersion) RemoveQuestion(lessonID, questionID uuid.UUID) error {
	l, c, err := v.quizLesson(lessonID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(c.Questions, func(q *Question) bool { return q.ID == questionID }) {
		return ErrQuestionNotFound
	}
	c.Questions = slices.DeleteFunc(slices.Clone(c.Questions), func(q *Question) bool { return q.ID == questionID })
	l.content = c
	return nil
}

// ReorderQuestions sắp lại câu hỏi; danh sách phải là hoán vị đầy đủ của câu hỏi hiện có.
func (v *StageVersion) ReorderQuestions(lessonID uuid.UUID, idsInOrder []uuid.UUID) error {
	l, c, err := v.quizLesson(lessonID)
	if err != nil {
		return err
	}
	ordered, err := reorderByID(c.Questions, idsInOrder, func(q *Question) uuid.UUID { return q.ID })
	if err != nil {
		return err
	}
	c.Questions = ordered
	l.content = c
	return nil
}

// quizLesson kiểm bản nháp trước rồi mới tìm học liệu, để lỗi trên bản đã phát hành luôn là ErrVersionImmutable.
func (v *StageVersion) quizLesson(lessonID uuid.UUID) (*Lesson, QuizContent, error) {
	if err := v.ensureMutable(); err != nil {
		return nil, QuizContent{}, err
	}
	l, err := v.Lesson(lessonID)
	if err != nil {
		return nil, QuizContent{}, err
	}
	c, ok := l.content.(QuizContent)
	if !ok {
		return nil, QuizContent{}, ErrNotQuiz
	}
	return l, c, nil
}

// buildQuestion kiểm câu hỏi theo saveQuestion của prototype rồi dựng câu hỏi mới (HTML chưa render). prev là câu
// đang sửa (nil khi thêm) để giữ id phương án cũ.
func buildQuestion(id uuid.UUID, key domain.LessonKey, in QuestionInput, prev *Question, newID func() uuid.UUID) (*Question, error) {
	body := strings.TrimSpace(in.BodySource)
	switch {
	case body == "":
		return nil, ErrQuestionBodyRequired
	case in.Type != QuestionSingle && in.Type != QuestionMulti:
		return nil, ErrInvalidQuestionType
	case in.Points < 1 || in.Points > maxQuestionPoints:
		return nil, ErrQuestionPoints
	case len(in.Options) < minQuestionOptions:
		return nil, ErrTooFewOptions
	case len(in.Options) > maxQuestionOptions:
		return nil, ErrTooManyOptions
	}
	known := map[uuid.UUID]bool{}
	if prev != nil {
		for _, o := range prev.Options {
			known[o.ID] = true
		}
	}
	options := make([]Option, 0, len(in.Options))
	correct := 0
	for _, o := range in.Options {
		text := strings.TrimSpace(o.Body)
		if text == "" || utf8.RuneCountInString(text) > maxOptionBodyLen {
			return nil, ErrOptionBody
		}
		oid := o.ID
		if oid == uuid.Nil {
			oid = newID()
		} else if !known[oid] {
			return nil, ErrOptionNotInQuestion
		}
		// Mỗi id cũ chỉ dùng một lần.
		delete(known, oid)
		if o.Correct {
			correct++
		}
		options = append(options, Option{ID: oid, Body: text, Correct: o.Correct})
	}
	if correct == 0 {
		return nil, ErrNoCorrectOption
	}
	if in.Type == QuestionSingle && correct > 1 {
		return nil, ErrSingleManyCorrect
	}
	return &Question{
		ID: id, Key: key, Type: in.Type, BodySource: body, ExplanationSource: strings.TrimSpace(in.ExplanationSource),
		Points: in.Points, Options: options,
	}, nil
}

// reorderByID xếp items theo idsInOrder; danh sách phải là hoán vị đầy đủ, không trùng.
func reorderByID[T any](items []T, idsInOrder []uuid.UUID, idOf func(T) uuid.UUID) ([]T, error) {
	if len(idsInOrder) != len(items) {
		return nil, ErrReorderMismatch
	}
	byID := make(map[uuid.UUID]T, len(items))
	for _, it := range items {
		byID[idOf(it)] = it
	}
	out := make([]T, 0, len(items))
	for _, id := range idsInOrder {
		it, ok := byID[id]
		if !ok {
			return nil, ErrReorderMismatch
		}
		delete(byID, id)
		out = append(out, it)
	}
	return out, nil
}

// copyQuestions sao chép sâu câu hỏi; newID khác nil thì câu hỏi và phương án nhận id mới (nhân bản), key giữ nguyên.
func copyQuestions(in []*Question, newID func() uuid.UUID) []*Question {
	if in == nil {
		return nil
	}
	out := make([]*Question, 0, len(in))
	for _, q := range in {
		c := *q
		c.BodyHTML, c.ExplanationHTML = copyPtr(q.BodyHTML), copyPtr(q.ExplanationHTML)
		c.Options = slices.Clone(q.Options)
		if newID != nil {
			c.ID = newID()
			for i := range c.Options {
				c.Options[i].ID = newID()
			}
		}
		out = append(out, &c)
	}
	return out
}

func copyPtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
