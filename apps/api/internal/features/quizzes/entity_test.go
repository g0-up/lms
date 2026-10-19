package quizzes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
)

func TestPastGrace(t *testing.T) {
	deadline := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	a := Attempt{DeadlineAt: &deadline}
	assert.False(t, a.PastGrace(deadline.Add(SubmitGrace)), "đúng biên ân hạn vẫn được nhận")
	assert.True(t, a.PastGrace(deadline.Add(SubmitGrace+time.Nanosecond)))
	assert.False(t, (&Attempt{}).PastGrace(deadline.Add(time.Hour)), "không giới hạn thời gian thì không bao giờ quá hạn")
}

func TestLimit(t *testing.T) {
	two := 2
	assessment := stages.QuizContent{Mode: stages.QuizAssessment, MaxAttempts: &two}
	assert.Equal(t, 2, *Limit(assessment, 0))
	assert.Equal(t, 3, *Limit(assessment, 1))
	assert.Nil(t, Limit(stages.QuizContent{Mode: stages.QuizPractice, MaxAttempts: &two}, 1))
}

func TestBestPicksHighestRatioEarlierOnTie(t *testing.T) {
	score := func(s, m int) Attempt { return Attempt{Score: &s, MaxScore: m} }
	attempts := []Attempt{score(3, 10), score(7, 10), {MaxScore: 10}, score(14, 20)}
	attempts[0].No, attempts[1].No, attempts[2].No, attempts[3].No = 1, 2, 3, 4

	best := Best(attempts)
	assert.Equal(t, 2, best.No, "7/10 bằng 14/20 thì lượt sớm hơn thắng; lượt mở bị bỏ qua")
	assert.Nil(t, Best([]Attempt{{MaxScore: 10}}))
}

func TestBlockedReason(t *testing.T) {
	two := 2
	assert.Equal(t, "Lớp đã kết thúc", blockedReason(domain.ClassEnded, nil, 0, false))
	assert.Equal(t, "Lớp chưa bắt đầu", blockedReason(domain.ClassDraft, nil, 0, false))
	assert.Equal(t, "Bạn đã dùng hết 2 lượt", blockedReason(domain.ClassActive, &two, 2, false))
	assert.Empty(t, blockedReason(domain.ClassActive, &two, 2, true), "còn lượt mở thì vẫn tiếp tục được")
	assert.Empty(t, blockedReason(domain.ClassActive, &two, 1, false))
	assert.Empty(t, blockedReason(domain.ClassActive, nil, 50, false))
}
