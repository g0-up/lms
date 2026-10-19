package quizzes

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
)

func TestReviewContent(t *testing.T) {
	q1, q2, q3 := uuid.New(), uuid.New(), uuid.New()
	a := &Attempt{QuestionOrder: []uuid.UUID{q3, q1, q2}}

	comment, notes, err := reviewContent(a, ReviewInput{
		Comment: "  Làm tốt.  ",
		Notes: []NoteInput{
			{QuestionID: q1.String(), Note: " Xem lại khái niệm. "},
			{QuestionID: q2.String(), Note: "   "},
			{QuestionID: q3.String(), Note: "Đúng hướng."},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Làm tốt.", comment)
	assert.Equal(t, []ReviewNote{{QuestionID: q3, Note: "Đúng hướng."}, {QuestionID: q1, Note: "Xem lại khái niệm."}}, notes,
		"ghi chú theo thứ tự câu học viên thấy, ghi chú rỗng bị bỏ")

	_, notes, err = reviewContent(a, ReviewInput{Comment: "Ổn."})
	require.NoError(t, err)
	assert.Empty(t, notes, "ghi chú từng câu không bắt buộc")

	cases := []struct {
		name string
		in   ReviewInput
		want error
	}{
		{"nhận xét rỗng", ReviewInput{Comment: " \n "}, ErrCommentRequired},
		{"nhận xét quá dài", ReviewInput{Comment: strings.Repeat("ạ", maxCommentRunes+1)}, ErrCommentTooLong},
		{"câu không thuộc lượt", ReviewInput{Comment: "x", Notes: []NoteInput{{QuestionID: uuid.NewString(), Note: "y"}}}, ErrNoteNotInAttempt},
		{"id câu sai dạng", ReviewInput{Comment: "x", Notes: []NoteInput{{QuestionID: "q1", Note: "y"}}}, ErrNoteNotInAttempt},
		{"trùng câu", ReviewInput{Comment: "x", Notes: []NoteInput{{QuestionID: q1.String(), Note: "a"}, {QuestionID: q1.String(), Note: "b"}}}, ErrDuplicateNote},
		{"ghi chú quá dài", ReviewInput{Comment: "x", Notes: []NoteInput{{QuestionID: q1.String(), Note: strings.Repeat("ạ", maxNoteRunes+1)}}}, ErrNoteTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := reviewContent(a, tc.in)
			require.ErrorIs(t, err, tc.want)
			assert.True(t, errors.Is(err, domain.ErrInvalid), "lỗi form là VALIDATION_FAILED")
		})
	}

	_, _, err = reviewContent(a, ReviewInput{Comment: strings.Repeat("ạ", maxCommentRunes)})
	assert.NoError(t, err, "đúng biên 5000 ký tự (tính theo rune) vẫn nhận")
}

func TestNextReviewStatus(t *testing.T) {
	st := func(s ReviewStatus) *ReviewStatus { return &s }
	for _, cur := range []*ReviewStatus{nil, st(ReviewPending), st(ReviewConfirmed)} {
		got, err := nextReviewStatus(cur)
		require.NoError(t, err)
		assert.Equal(t, ReviewConfirmed, got)
	}
	_, err := nextReviewStatus(st(ReviewSuperseded))
	require.ErrorIs(t, err, ErrReviewSuperseded)
	assert.True(t, errors.Is(err, domain.ErrInvalidTransition))
	assert.Equal(t, "Học viên đã nộp lượt mới hơn. Hãy xác nhận lượt mới nhất.", err.Error())
}

func TestReviewAction(t *testing.T) {
	assert.Equal(t, audit.ActionQuizAttemptConfirmed, reviewAction(stages.QuizAssessment, true))
	assert.Equal(t, audit.ActionQuizAttemptCommented, reviewAction(stages.QuizPractice, true))
	assert.Equal(t, audit.ActionQuizAttemptReviewUpdated, reviewAction(stages.QuizAssessment, false))
	assert.Equal(t, audit.ActionQuizAttemptReviewUpdated, reviewAction(stages.QuizPractice, false))
}

func TestShortTitle(t *testing.T) {
	assert.Equal(t, "chỉ mục", shortTitle("Kiểm tra: chỉ mục"))
	assert.Equal(t, "khóa chính", shortTitle("Luyện tập:khóa chính"))
	assert.Equal(t, "SQL nâng cao", shortTitle("Bài tập: SQL nâng cao"))
	assert.Equal(t, "Quiz cuối chặng: Kiểm tra", shortTitle("Quiz cuối chặng: Kiểm tra"), "chỉ bỏ tiền tố ở đầu")
}

func TestQuestionStatus(t *testing.T) {
	yes, no := true, false
	assert.Equal(t, QuestionBlank, questionStatus(Answer{}))
	assert.Equal(t, QuestionBlank, questionStatus(Answer{IsCorrect: &no}), "không chọn gì là bỏ trống dù đã chấm")
	assert.Equal(t, QuestionCorrect, questionStatus(Answer{Selected: []uuid.UUID{uuid.New()}, IsCorrect: &yes}))
	assert.Equal(t, QuestionWrong, questionStatus(Answer{Selected: []uuid.UUID{uuid.New()}, IsCorrect: &no}))
}

func TestMinutesTaken(t *testing.T) {
	start := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *Attempt { s := start.Add(d); return &Attempt{StartedAt: start, SubmittedAt: &s} }
	assert.Equal(t, 1, minutesTaken(at(10*time.Second)), "tối thiểu 1 phút")
	assert.Equal(t, 7, minutesTaken(at(7*time.Minute+29*time.Second)))
	assert.Equal(t, 8, minutesTaken(at(7*time.Minute+30*time.Second)))
	assert.Equal(t, 0, minutesTaken(&Attempt{StartedAt: start}), "lượt chưa nộp")
}

func TestMemberQuizStatus(t *testing.T) {
	now := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	submitted := func(no, score int, passed bool) Attempt {
		return Attempt{No: no, SubmittedAt: &now, Score: &score, MaxScore: 10, Passed: &passed}
	}
	open := Attempt{No: 3, MaxScore: 10}
	two, three := 2, 3

	assert.Equal(t, MemberQuizTodo, memberQuizStatus(stages.QuizPractice, nil, nil))
	assert.Equal(t, MemberQuizInProgress, memberQuizStatus(stages.QuizPractice, []Attempt{{No: 1, MaxScore: 10}}, nil))
	assert.Equal(t, MemberQuizDone, memberQuizStatus(stages.QuizPractice, []Attempt{submitted(1, 2, false), open}, nil))

	assert.Equal(t, MemberQuizTodo, memberQuizStatus(stages.QuizAssessment, nil, &two))
	assert.Equal(t, MemberQuizInProgress, memberQuizStatus(stages.QuizAssessment, []Attempt{{No: 1, MaxScore: 10}}, &two))
	assert.Equal(t, MemberQuizFailed, memberQuizStatus(stages.QuizAssessment, []Attempt{submitted(1, 3, false)}, &two))
	assert.Equal(t, MemberQuizOutOfAttempts,
		memberQuizStatus(stages.QuizAssessment, []Attempt{submitted(1, 3, false), submitted(2, 4, false)}, &two))
	assert.Equal(t, MemberQuizFailed,
		memberQuizStatus(stages.QuizAssessment, []Attempt{submitted(1, 3, false), submitted(2, 4, false)}, &three),
		"được cấp thêm lượt thì chưa hết lượt")
	assert.Equal(t, MemberQuizInProgress,
		memberQuizStatus(stages.QuizAssessment, []Attempt{submitted(1, 3, false), submitted(2, 4, false), open}, &three))
	assert.Equal(t, MemberQuizPassed,
		memberQuizStatus(stages.QuizAssessment, []Attempt{submitted(1, 8, true), submitted(2, 3, false)}, &two),
		"lượt tốt nhất đạt thì đạt dù lượt sau trượt")
}

func TestAvgPercent(t *testing.T) {
	assert.Nil(t, avgPercent(nil))
	assert.Equal(t, 70, *avgPercent([]int{70}))
	assert.Equal(t, 78, *avgPercent([]int{70, 85}), "77.5 làm tròn lên")
	assert.Equal(t, 67, *avgPercent([]int{100, 50, 50}))
}

func TestStaffCanAccessClass(t *testing.T) {
	teacher := uuid.New()
	assert.NoError(t, staffCanAccessClass(Actor{ID: uuid.New(), Role: domain.RoleAdmin}, teacher, ErrNotOwnReview))
	assert.NoError(t, staffCanAccessClass(Actor{ID: teacher, Role: domain.RoleTeacher}, teacher, ErrNotOwnReview))
	assert.ErrorIs(t, staffCanAccessClass(Actor{ID: uuid.New(), Role: domain.RoleTeacher}, teacher, ErrNotOwnReview), ErrNotOwnReview)
	assert.ErrorIs(t, staffCanAccessClass(Actor{ID: uuid.New(), Role: domain.RoleTeacher}, teacher, classes.ErrNotOwnClass),
		classes.ErrNotOwnClass)
	assert.ErrorIs(t, staffCanAccessClass(Actor{ID: teacher, Role: domain.RoleStudent}, teacher, ErrNotOwnReview), domain.ErrForbidden)
}
