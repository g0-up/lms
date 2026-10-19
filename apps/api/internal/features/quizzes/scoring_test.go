package quizzes

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/features/stages"
)

// question dựng câu hỏi với n phương án; correct là chỉ số các phương án đúng.
func question(typ stages.QuestionType, points, n int, correct ...int) *stages.Question {
	q := &stages.Question{ID: uuid.New(), Type: typ, Points: points}
	for i := range n {
		q.Options = append(q.Options, stages.Option{ID: uuid.New(), Body: "PA", Correct: contains(correct, i)})
	}
	return q
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func opts(q *stages.Question, idx ...int) []uuid.UUID {
	out := make([]uuid.UUID, len(idx))
	for i, j := range idx {
		out[i] = q.Options[j].ID
	}
	return out
}

func TestScoreSingleChoice(t *testing.T) {
	q := question(stages.QuestionSingle, 2, 3, 1)
	quiz := stages.QuizContent{Questions: []*stages.Question{q}}

	score, maxScore, per := Score(quiz, map[uuid.UUID][]uuid.UUID{q.ID: opts(q, 1)})
	assert.Equal(t, 2, score)
	assert.Equal(t, 2, maxScore)
	assert.True(t, per[0].Correct)
	assert.Equal(t, 2, per[0].Points)

	score, _, per = Score(quiz, map[uuid.UUID][]uuid.UUID{q.ID: opts(q, 0)})
	assert.Equal(t, 0, score)
	assert.False(t, per[0].Correct)
	assert.Equal(t, 0, per[0].Points)
}

func TestScoreMultiChoiceNeedsExactSet(t *testing.T) {
	q := question(stages.QuestionMulti, 3, 4, 0, 2)
	quiz := stages.QuizContent{Questions: []*stages.Question{q}}
	cases := []struct {
		name   string
		picked []uuid.UUID
		want   int
	}{
		{"đúng đủ", opts(q, 2, 0), 3},
		{"thiếu một", opts(q, 0), 0},
		{"thừa một", opts(q, 0, 1, 2), 0},
		{"bỏ trống", []uuid.UUID{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			score, maxScore, per := Score(quiz, map[uuid.UUID][]uuid.UUID{q.ID: tc.picked})
			assert.Equal(t, tc.want, score)
			assert.Equal(t, 3, maxScore)
			assert.Equal(t, tc.want > 0, per[0].Correct)
		})
	}
}

func TestScoreBlankQuestionsScoreZeroInAuthoredOrder(t *testing.T) {
	q1 := question(stages.QuestionSingle, 1, 2, 0)
	q2 := question(stages.QuestionSingle, 4, 2, 1)
	quiz := stages.QuizContent{Questions: []*stages.Question{q1, q2}}

	score, maxScore, per := Score(quiz, map[uuid.UUID][]uuid.UUID{q2.ID: opts(q2, 1)})
	assert.Equal(t, 4, score)
	assert.Equal(t, 5, maxScore)
	require.Len(t, per, 2)
	assert.Equal(t, q1.ID, per[0].QuestionID)
	assert.False(t, per[0].Correct)
	assert.Empty(t, per[0].Selected)
	assert.Equal(t, q2.ID, per[1].QuestionID)
}

// Ngưỡng 70% với tổng 10 điểm chia 3 câu lẻ điểm (3, 3, 4): 7/10 đạt đúng biên, 6/10 trượt.
func TestPassThresholdBoundary(t *testing.T) {
	q1 := question(stages.QuestionSingle, 3, 2, 0)
	q2 := question(stages.QuestionSingle, 3, 2, 0)
	q3 := question(stages.QuestionSingle, 4, 2, 0)
	pct := 70
	quiz := stages.QuizContent{Mode: stages.QuizAssessment, PassPercent: &pct, Questions: []*stages.Question{q1, q2, q3}}

	score, maxScore, _ := Score(quiz, map[uuid.UUID][]uuid.UUID{q1.ID: opts(q1, 0), q3.ID: opts(q3, 0)})
	require.Equal(t, 7, score)
	require.Equal(t, 10, maxScore)
	assert.True(t, *Passed(quiz, score, maxScore))

	score, _, _ = Score(quiz, map[uuid.UUID][]uuid.UUID{q1.ID: opts(q1, 0), q2.ID: opts(q2, 0)})
	require.Equal(t, 6, score)
	assert.False(t, *Passed(quiz, score, maxScore))

	// 2/3 = 66,67% không được làm tròn lên ngưỡng 67%.
	pct67 := 67
	quiz.PassPercent = &pct67
	assert.False(t, *Passed(quiz, 2, 3))

	quiz.Mode = stages.QuizPractice
	assert.Nil(t, Passed(quiz, 10, 10))
}

func TestNormalizeSelection(t *testing.T) {
	single := question(stages.QuestionSingle, 1, 3, 0)
	multi := question(stages.QuestionMulti, 1, 3, 0, 1)

	got, err := normalizeSelection(multi, opts(multi, 2, 0, 2))
	require.NoError(t, err)
	assert.Equal(t, opts(multi, 0, 2), got, "bỏ trùng, theo thứ tự soạn")

	got, err = normalizeSelection(single, nil)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{}, got, "rỗng là bỏ chọn")

	_, err = normalizeSelection(single, opts(single, 0, 1))
	assert.ErrorIs(t, err, ErrSingleChoice)

	_, err = normalizeSelection(single, opts(multi, 0))
	assert.ErrorIs(t, err, ErrOptionNotInQuestion)
}

// reverseShuffler đảo ngược thứ tự để kiểm attemptOrder mà không phụ thuộc ngẫu nhiên.
type reverseShuffler struct{}

func (reverseShuffler) Shuffle(n int, swap func(i, j int)) {
	for i := range n / 2 {
		swap(i, n-1-i)
	}
}

func TestAttemptOrder(t *testing.T) {
	q1 := question(stages.QuestionSingle, 1, 3, 0)
	q2 := question(stages.QuestionSingle, 1, 2, 0)
	quiz := stages.QuizContent{Questions: []*stages.Question{q1, q2}}

	questions, options := attemptOrder(quiz, reverseShuffler{})
	assert.Equal(t, []uuid.UUID{q1.ID, q2.ID}, questions, "shuffle tắt giữ thứ tự soạn")
	assert.Equal(t, opts(q1, 0, 1, 2), options[q1.ID])

	quiz.Shuffle = true
	questions, options = attemptOrder(quiz, reverseShuffler{})
	assert.Equal(t, []uuid.UUID{q2.ID, q1.ID}, questions)
	assert.Equal(t, opts(q1, 2, 1, 0), options[q1.ID])
	assert.Equal(t, opts(q2, 1, 0), options[q2.ID])
}
