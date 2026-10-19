package quizzes

import (
	"math/rand/v2"
	"slices"

	"github.com/google/uuid"

	"lms/api/internal/features/stages"
)

// QuestionResult là kết quả chấm một câu: phương án đã chọn, đúng hay sai và điểm được.
type QuestionResult struct {
	QuestionID uuid.UUID
	Selected   []uuid.UUID
	Correct    bool
	Points     int
}

// Score chấm toàn bộ quiz theo thứ tự soạn. Câu được điểm khi tập phương án chọn bằng đúng tập đáp án đúng, không có
// điểm một phần; câu bỏ trống được 0 điểm. answers theo id câu hỏi.
func Score(q stages.QuizContent, answers map[uuid.UUID][]uuid.UUID) (score, maxScore int, per []QuestionResult) {
	per = make([]QuestionResult, len(q.Questions))
	for i, question := range q.Questions {
		selected := answers[question.ID]
		ok := len(selected) > 0 && sameSet(selected, correctOf(question))
		r := QuestionResult{QuestionID: question.ID, Selected: selected, Correct: ok}
		if ok {
			r.Points = question.Points
		}
		per[i] = r
		score += r.Points
		maxScore += question.Points
	}
	return score, maxScore, per
}

func correctOf(q *stages.Question) []uuid.UUID {
	var out []uuid.UUID
	for _, o := range q.Options {
		if o.Correct {
			out = append(out, o.ID)
		}
	}
	return out
}

// sameSet so hai tập id (bỏ qua thứ tự và phần tử lặp).
func sameSet(a, b []uuid.UUID) bool {
	in := func(xs []uuid.UUID) map[uuid.UUID]bool {
		m := make(map[uuid.UUID]bool, len(xs))
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	ma, mb := in(a), in(b)
	if len(ma) != len(mb) {
		return false
	}
	for x := range ma {
		if !mb[x] {
			return false
		}
	}
	return true
}

// normalizeSelection kiểm phương án học viên chọn cho một câu: mọi phương án thuộc câu, câu một đáp án chọn tối đa
// một. Trả tập đã bỏ trùng theo thứ tự soạn; rỗng là bỏ chọn.
func normalizeSelection(q *stages.Question, ids []uuid.UUID) ([]uuid.UUID, error) {
	for _, id := range ids {
		if !slices.ContainsFunc(q.Options, func(o stages.Option) bool { return o.ID == id }) {
			return nil, ErrOptionNotInQuestion
		}
	}
	out := []uuid.UUID{}
	for _, o := range q.Options {
		if slices.Contains(ids, o.ID) {
			out = append(out, o.ID)
		}
	}
	if q.Type == stages.QuestionSingle && len(out) > 1 {
		return nil, ErrSingleChoice
	}
	return out, nil
}

// Shuffler xáo thứ tự n phần tử bằng swap; production dùng RandShuffler, test dùng bản cố định.
type Shuffler interface {
	Shuffle(n int, swap func(i, j int))
}

// RandShuffler xáo bằng math/rand/v2 (nguồn ngẫu nhiên toàn cục, an toàn khi gọi song song).
type RandShuffler struct{}

func (RandShuffler) Shuffle(n int, swap func(i, j int)) {
	rand.Shuffle(n, swap) //nolint:gosec // thứ tự xáo gửi kèm lượt cho học viên, không phải bí mật; chỉ cần khác nhau giữa các lượt
}

// attemptOrder chốt thứ tự câu và phương án của một lượt: xáo một lần khi quiz bật shuffle, ngược lại giữ thứ tự soạn.
func attemptOrder(q stages.QuizContent, sh Shuffler) ([]uuid.UUID, map[uuid.UUID][]uuid.UUID) {
	questions := make([]uuid.UUID, len(q.Questions))
	options := make(map[uuid.UUID][]uuid.UUID, len(q.Questions))
	for i, question := range q.Questions {
		questions[i] = question.ID
		opts := make([]uuid.UUID, len(question.Options))
		for j, o := range question.Options {
			opts[j] = o.ID
		}
		if q.Shuffle {
			sh.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
		}
		options[question.ID] = opts
	}
	if q.Shuffle {
		sh.Shuffle(len(questions), func(i, j int) { questions[i], questions[j] = questions[j], questions[i] })
	}
	return questions, options
}

// questionByID tra câu hỏi của quiz theo id.
func questionByID(q stages.QuizContent, id uuid.UUID) *stages.Question {
	for _, question := range q.Questions {
		if question.ID == id {
			return question
		}
	}
	return nil
}
