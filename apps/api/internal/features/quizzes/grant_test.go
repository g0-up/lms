package quizzes

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrantContent(t *testing.T) {
	n := func(v int) *int { return &v }
	cases := []struct {
		name  string
		in    GrantInput
		extra int
		why   string
		err   error
	}{
		{"một lượt, lý do được trim", GrantInput{ExtraAttempts: n(1), Reason: "  Mất kết nối  "}, 1, "Mất kết nối", nil},
		{"ba lượt", GrantInput{ExtraAttempts: n(3), Reason: "Lỗi đề"}, 3, "Lỗi đề", nil},
		{"lý do đúng 500 ký tự", GrantInput{ExtraAttempts: n(2), Reason: strings.Repeat("ă", 500)}, 2, strings.Repeat("ă", 500), nil},
		{"thiếu số lượt", GrantInput{Reason: "Lỗi đề"}, 0, "", ErrGrantAttemptsRange},
		{"0 lượt", GrantInput{ExtraAttempts: n(0), Reason: "Lỗi đề"}, 0, "", ErrGrantAttemptsRange},
		{"4 lượt", GrantInput{ExtraAttempts: n(4), Reason: "Lỗi đề"}, 0, "", ErrGrantAttemptsRange},
		{"số lượt âm", GrantInput{ExtraAttempts: n(-1), Reason: "Lỗi đề"}, 0, "", ErrGrantAttemptsRange},
		{"lý do rỗng", GrantInput{ExtraAttempts: n(1)}, 0, "", ErrGrantReasonRequired},
		{"lý do chỉ có khoảng trắng", GrantInput{ExtraAttempts: n(1), Reason: " \n\t "}, 0, "", ErrGrantReasonRequired},
		{"lý do 501 ký tự", GrantInput{ExtraAttempts: n(1), Reason: strings.Repeat("ă", 501)}, 0, "", ErrGrantReasonTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extra, reason, err := grantContent(tc.in)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.extra, extra)
			assert.Equal(t, tc.why, reason)
		})
	}
	assert.Equal(t, "Số lượt cấp thêm từ 1 đến 3.", ErrGrantAttemptsRange.Error())
}

func TestQuestionText(t *testing.T) {
	h := func(s string) *string { return &s }
	assert.Equal(t, "Chỉ mục B-tree phù hợp với?", questionText(h("<p>Chỉ mục B-tree phù hợp với?</p>"), "nguồn"))
	assert.Equal(t, "Chọn a & b khi x < 3 Đoạn hai dòng mới",
		questionText(h("<p>Chọn <strong>a</strong> &amp; <code>b</code> khi x &lt; 3</p><p>Đoạn hai<br>dòng mới</p>"), ""),
		"bỏ thẻ, giải mã thực thể, khối liền nhau cách nhau một khoảng trắng")
	assert.Equal(t, "Câu từ nguồn markdown", questionText(nil, "  Câu  từ\nnguồn markdown "), "chưa render thì dùng nguồn")

	long := strings.Repeat("ab ", 60) // 180 ký tự
	got := questionText(h("<p>"+long+"</p>"), "")
	assert.LessOrEqual(t, utf8.RuneCountInString(got), questionTextRunes)
	assert.True(t, strings.HasSuffix(got, "…"))
	assert.False(t, strings.HasSuffix(strings.TrimSuffix(got, "…"), " "), "không để khoảng trắng trước dấu …")

	exact := strings.Repeat("ữ", questionTextRunes)
	assert.Equal(t, exact, questionText(nil, exact), "đúng 120 ký tự thì giữ nguyên")
	cut := questionText(nil, exact+"x")
	assert.Equal(t, questionTextRunes, utf8.RuneCountInString(cut))
	assert.Equal(t, strings.Repeat("ữ", questionTextRunes-1)+"…", cut)
}

func TestQuestionStats(t *testing.T) {
	q1, q2, q3 := uuid.New(), uuid.New(), uuid.New()
	body := "<p>Câu</p>"
	rows := []QuestionTally{
		{QuestionID: q1, BodyHTML: &body, Correct: 2},
		{QuestionID: q2, BodySource: "Câu hai", Correct: 1},
		{QuestionID: q3, BodySource: "Câu ba", Correct: 0},
	}

	got := questionStats(rows, 3)
	assert.Equal(t, 3, got.Attempts)
	require.Len(t, got.Rows, 3)
	want := []struct {
		id      uuid.UUID
		pos     int
		text    string
		percent int
	}{{q1, 1, "Câu", 67}, {q2, 2, "Câu hai", 33}, {q3, 3, "Câu ba", 0}}
	for i, w := range want {
		r := got.Rows[i]
		assert.Equal(t, w.id, r.QuestionID)
		assert.Equal(t, w.pos, r.Position)
		assert.Equal(t, w.text, r.BodyText)
		require.NotNil(t, r.CorrectPercent, "có lượt thì có tỉ lệ, kể cả 0%%")
		assert.Equal(t, w.percent, *r.CorrectPercent)
	}

	none := questionStats(rows, 0)
	assert.Zero(t, none.Attempts)
	for _, r := range none.Rows {
		assert.Nil(t, r.CorrectPercent, "chưa có lượt thì không có tỉ lệ")
	}
	assert.Empty(t, questionStats(nil, 5).Rows)
}
