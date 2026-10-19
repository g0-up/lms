package summary

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/features/stages"
)

var now = time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func lessonOf(t stages.LessonType) Lesson {
	return Lesson{ID: uuid.New(), Type: t, Required: true}
}

func practice() Lesson {
	l := lessonOf(stages.LessonQuiz)
	l.QuizMode = stages.QuizPractice
	return l
}

func assessment() Lesson {
	l := lessonOf(stages.LessonQuiz)
	l.QuizMode, l.MaxAttempts, l.PassPercent = stages.QuizAssessment, ptr(2), ptr(70)
	return l
}

func TestStatus(t *testing.T) {
	done := ptr(now.Add(-time.Hour))
	reviewed := func(s ReviewStatus) *ReviewStatus { return &s }
	tests := []struct {
		name  string
		facts ItemFacts
		want  ItemStatus
	}{
		{"video đã tích", ItemFacts{Lesson: lessonOf(stages.LessonVideo), FirstOpenedAt: done, CompletedAt: done},
			ItemStatus{Done: true, Label: "Đã học xong", Tone: ToneOK, Detail: "Đã tích hoàn thành"}},
		{"video đã mở chưa tích", ItemFacts{Lesson: lessonOf(stages.LessonVideo), FirstOpenedAt: done},
			ItemStatus{Label: "Chưa học", Tone: ToneOptional, Detail: "Đã mở, chưa tích hoàn thành"}},
		{"markdown chưa mở", ItemFacts{Lesson: lessonOf(stages.LessonMarkdown)},
			ItemStatus{Label: "Chưa học", Tone: ToneOptional, Detail: "Chưa mở"}},

		{"luyện tập chưa làm", ItemFacts{Lesson: practice(), Quiz: &QuizFacts{Mode: stages.QuizPractice}},
			ItemStatus{Label: "Chưa làm", Tone: ToneOptional, Detail: "Chưa làm"}},
		{"luyện tập đang làm lượt đầu", ItemFacts{Lesson: practice(), Quiz: &QuizFacts{Mode: stages.QuizPractice, AttemptsUsed: 1, Open: true}},
			ItemStatus{Label: "Đang làm", Tone: ToneInfo, Detail: "Chưa làm"}},
		{"luyện tập đã nộp", ItemFacts{Lesson: practice(), CompletedAt: done, Quiz: &QuizFacts{
			Mode: stages.QuizPractice, AttemptsUsed: 2, Submitted: 2, Best: &Score{8, 10}, Latest: &LatestAttempt{Score: Score{3, 4}},
		}}, ItemStatus{Done: true, Label: "Đã làm", Tone: ToneOK, Detail: "2 lượt · Gần nhất 75%"}},

		{"bài tập chưa làm", ItemFacts{Lesson: assessment(), Quiz: &QuizFacts{Mode: stages.QuizAssessment, Limit: ptr(2)}},
			ItemStatus{Label: "Chưa làm", Tone: ToneOptional, Detail: "Chưa làm · 2 lượt"}},
		{"bài tập đang làm lượt đầu", ItemFacts{Lesson: assessment(), Quiz: &QuizFacts{Mode: stages.QuizAssessment, Limit: ptr(2), AttemptsUsed: 1, Open: true}},
			ItemStatus{Label: "Đang làm", Tone: ToneInfo, Detail: "1/2 lượt"}},
		{"bài tập đạt, chờ xác nhận", ItemFacts{Lesson: assessment(), CompletedAt: done, Quiz: &QuizFacts{
			Mode: stages.QuizAssessment, Limit: ptr(2), AttemptsUsed: 1, Submitted: 1, Best: &Score{8, 10},
			Latest: &LatestAttempt{Score: Score{8, 10}, ReviewStatus: reviewed(ReviewPending)},
		}}, ItemStatus{Done: true, Label: "Đạt 80%", Tone: ToneOK, Detail: "Tốt nhất 80% · 1/2 lượt · Chờ xác nhận"}},
		{"bài tập chưa đạt còn lượt", ItemFacts{Lesson: assessment(), Quiz: &QuizFacts{
			Mode: stages.QuizAssessment, Limit: ptr(2), AttemptsUsed: 1, Submitted: 1, Best: &Score{5, 10},
			Latest: &LatestAttempt{Score: Score{5, 10}, ReviewStatus: reviewed(ReviewConfirmed)},
		}}, ItemStatus{Label: "Chưa đạt 50%", Tone: ToneWarn, Detail: "Tốt nhất 50% · 1/2 lượt · Đã xác nhận"}},
		{"bài tập chưa đạt hết lượt", ItemFacts{Lesson: assessment(), Quiz: &QuizFacts{
			Mode: stages.QuizAssessment, Limit: ptr(2), AttemptsUsed: 2, Submitted: 2, Best: &Score{6, 10},
			Latest: &LatestAttempt{Score: Score{4, 10}, ReviewStatus: reviewed(ReviewPending)},
		}}, ItemStatus{Label: "Chưa đạt 60% · hết lượt", Tone: ToneDanger, Detail: "Tốt nhất 60% · 2/2 lượt · Chờ xác nhận"}},
		{"bài tập được cấp thêm lượt thì không còn hết lượt", ItemFacts{Lesson: assessment(), Quiz: &QuizFacts{
			Mode: stages.QuizAssessment, Limit: ptr(3), AttemptsUsed: 2, Submitted: 2, Best: &Score{6, 10},
			Latest: &LatestAttempt{Score: Score{6, 10}},
		}}, ItemStatus{Label: "Chưa đạt 60%", Tone: ToneWarn, Detail: "Tốt nhất 60% · 2/3 lượt"}},

		{"homework chưa đặt hạn", ItemFacts{Lesson: lessonOf(stages.LessonHomework), Homework: &HomeworkFacts{}},
			ItemStatus{Label: "Chưa nộp", Tone: ToneOptional, Detail: "Chưa nộp · chưa đặt hạn"}},
		{"homework còn hạn", ItemFacts{Lesson: lessonOf(stages.LessonHomework), Homework: &HomeworkFacts{DueAt: ptr(now.Add(time.Hour))}},
			ItemStatus{Label: "Chưa nộp", Tone: ToneOptional, Detail: "Chưa nộp"}},
		{"homework đúng bằng hạn chưa quá hạn", ItemFacts{Lesson: lessonOf(stages.LessonHomework), Homework: &HomeworkFacts{DueAt: ptr(now)}},
			ItemStatus{Label: "Chưa nộp", Tone: ToneOptional, Detail: "Chưa nộp"}},
		{"homework quá hạn chưa nộp", ItemFacts{Lesson: lessonOf(stages.LessonHomework), Homework: &HomeworkFacts{DueAt: ptr(now.Add(-time.Nanosecond))}},
			ItemStatus{Label: "Quá hạn", Tone: ToneDanger, Detail: "Chưa nộp", Overdue: true}},
		{"homework chờ chấm, nộp muộn không tính quá hạn", ItemFacts{Lesson: lessonOf(stages.LessonHomework), Homework: &HomeworkFacts{
			DueAt: ptr(now.Add(-48 * time.Hour)), SubmissionCount: 1,
			Latest: &Submission{AttemptNo: 1, Status: SubmissionPending, IsLate: true, SubmittedAt: now.Add(-time.Hour)},
		}}, ItemStatus{Label: "Chờ chấm", Tone: ToneInfo, Detail: "1 lần nộp · nộp muộn"}},
		{"homework cần làm lại", ItemFacts{Lesson: lessonOf(stages.LessonHomework), Homework: &HomeworkFacts{
			SubmissionCount: 2, Latest: &Submission{AttemptNo: 2, Status: SubmissionRework, Total: ptr(9), Max: ptr(20)},
		}}, ItemStatus{Label: "Cần làm lại", Tone: ToneWarn, Detail: "2 lần nộp · 9/20 điểm", Rework: true}},
		{"homework đạt", ItemFacts{Lesson: lessonOf(stages.LessonHomework), CompletedAt: done, Homework: &HomeworkFacts{
			SubmissionCount: 1, Latest: &Submission{AttemptNo: 1, Status: SubmissionPassed, Total: ptr(18), Max: ptr(20)},
		}}, ItemStatus{Done: true, Label: "Đạt", Tone: ToneOK, Detail: "1 lần nộp · 18/20 điểm"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Status(tt.facts, now))
		})
	}
}

// Hoàn thành chỉ lấy từ completed_at: bản nộp Đạt mà chưa có mốc hoàn thành vẫn không tính xong, để % ở mọi màn
// hình cùng một nguồn.
func TestStatusDoneOnlyFromCompletedAt(t *testing.T) {
	f := ItemFacts{Lesson: lessonOf(stages.LessonHomework), Homework: &HomeworkFacts{
		SubmissionCount: 1, Latest: &Submission{AttemptNo: 1, Status: SubmissionPassed},
	}}
	assert.False(t, Status(f, now).Done)
	f.CompletedAt = ptr(now)
	assert.True(t, Status(f, now).Done)
}

func TestBuildStage(t *testing.T) {
	stage := StageRef{StageVersionID: uuid.New(), Name: "Cấu trúc dữ liệu"}
	unitA, unitB := uuid.New(), uuid.New()
	video := lessonOf(stages.LessonVideo)
	video.UnitID, video.UnitTitle, video.Title = unitA, "Mảng", "Video mảng"
	optional := lessonOf(stages.LessonMarkdown)
	optional.UnitID, optional.UnitTitle, optional.Required = unitA, "Mảng", false
	hw := lessonOf(stages.LessonHomework)
	hw.UnitID, hw.UnitTitle, hw.Title = unitB, "Kiểm tra cuối chặng", "Bài tập stack"
	review := &Review{Body: "Tốt"}

	t.Run("còn thiếu và cờ", func(t *testing.T) {
		s := BuildStage(stage, []ItemFacts{
			{Lesson: video, CompletedAt: ptr(now)},
			{Lesson: optional},
			{Lesson: hw, Homework: &HomeworkFacts{SubmissionCount: 1, Latest: &Submission{AttemptNo: 1, Status: SubmissionRework}}},
		}, review, now)
		require.Len(t, s.Units, 2)
		assert.Len(t, s.Units[0].Items, 2)
		assert.Equal(t, "Kiểm tra cuối chặng", s.Units[1].Title)
		assert.Equal(t, 2, s.Required)
		assert.Equal(t, 1, s.Done)
		assert.Equal(t, 50, s.Percent)
		assert.False(t, s.Complete)
		assert.Equal(t, []MissingItem{{LessonID: hw.ID, Title: "Bài tập stack", UnitTitle: "Kiểm tra cuối chặng"}}, s.Missing)
		assert.True(t, s.Rework)
		assert.False(t, s.Overdue)
		assert.Same(t, review, s.Review)
	})

	t.Run("xong mọi học liệu bắt buộc thì hoàn thành, học liệu không bắt buộc không tính", func(t *testing.T) {
		s := BuildStage(stage, []ItemFacts{
			{Lesson: video, CompletedAt: ptr(now)},
			{Lesson: optional},
			{Lesson: hw, CompletedAt: ptr(now), Homework: &HomeworkFacts{SubmissionCount: 1, Latest: &Submission{Status: SubmissionPassed}}},
		}, nil, now)
		assert.True(t, s.Complete)
		assert.Equal(t, 100, s.Percent)
		assert.Empty(t, s.Missing)
		assert.Nil(t, s.Review)
	})

	t.Run("chặng không có học liệu bắt buộc không hoàn thành và 0%", func(t *testing.T) {
		s := BuildStage(stage, []ItemFacts{{Lesson: optional, CompletedAt: ptr(now)}}, nil, now)
		assert.False(t, s.Complete)
		assert.Zero(t, s.Percent)
		assert.Zero(t, s.Required)

		empty := BuildStage(stage, nil, nil, now)
		assert.False(t, empty.Complete)
		assert.Empty(t, empty.Units)
		assert.NotNil(t, empty.Missing)
	})

	t.Run("homework quá hạn bật cờ chặng", func(t *testing.T) {
		s := BuildStage(stage, []ItemFacts{{Lesson: hw, Homework: &HomeworkFacts{DueAt: ptr(now.Add(-time.Hour))}}}, nil, now)
		assert.True(t, s.Overdue)
	})
}

func TestBuildCourseSumsRequiredAcrossStages(t *testing.T) {
	s1, s2 := StageRef{StageVersionID: uuid.New()}, StageRef{StageVersionID: uuid.New()}
	a, b, c := lessonOf(stages.LessonVideo), lessonOf(stages.LessonVideo), lessonOf(stages.LessonVideo)
	a.StageVersionID, b.StageVersionID, c.StageVersionID = s1.StageVersionID, s1.StageVersionID, s2.StageVersionID
	review := &Review{StageVersionID: s2.StageVersionID}

	sums, done, total := BuildCourse([]StageRef{s1, s2}, []ItemFacts{
		{Lesson: a, CompletedAt: ptr(now)}, {Lesson: b}, {Lesson: c, CompletedAt: ptr(now)},
	}, map[uuid.UUID]*Review{s2.StageVersionID: review}, now)
	require.Len(t, sums, 2)
	assert.Equal(t, 2, done)
	assert.Equal(t, 3, total)
	assert.False(t, sums[0].Complete)
	assert.True(t, sums[1].Complete)
	assert.Nil(t, sums[0].Review)
	assert.Same(t, review, sums[1].Review)
}

func TestBuildFacts(t *testing.T) {
	m1, m2 := uuid.New(), uuid.New()
	video, hw, quiz, prac := lessonOf(stages.LessonVideo), lessonOf(stages.LessonHomework), assessment(), practice()
	lessons := []Lesson{video, hw, quiz, prac}
	pending := ReviewPending
	act := Activity{
		Progress:  []ProgressRow{{MemberID: m1, LessonID: video.ID, FirstOpenedAt: ptr(now), CompletedAt: ptr(now)}},
		Deadlines: map[uuid.UUID]time.Time{hw.ID: now},
		Submissions: []SubmissionRow{{MemberID: m1, LessonID: hw.ID, Count: 2, Latest: Submission{
			AttemptNo: 2, Status: SubmissionPending,
		}}},
		Attempts: []AttemptRow{
			{MemberID: m1, LessonID: quiz.ID, AttemptNo: 1, Score: ptr(7), Max: 10},
			{MemberID: m1, LessonID: quiz.ID, AttemptNo: 2, Score: ptr(14), Max: 20, ReviewStatus: &pending},
			{MemberID: m1, LessonID: quiz.ID, AttemptNo: 3, Max: 10},
			{MemberID: m1, LessonID: prac.ID, AttemptNo: 1, Score: ptr(1), Max: 4},
		},
		Grants: []GrantRow{{MemberID: m1, LessonID: quiz.ID, Extra: 1}, {MemberID: m1, LessonID: quiz.ID, Extra: 2}},
	}
	facts := BuildFacts(lessons, []uuid.UUID{m1, m2}, act)

	f1 := facts[m1]
	require.Len(t, f1, 4)
	assert.Equal(t, ptr(now), f1[0].CompletedAt)
	assert.Nil(t, f1[0].Homework)
	assert.Nil(t, f1[0].Quiz)

	require.NotNil(t, f1[1].Homework)
	assert.Equal(t, ptr(now), f1[1].Homework.DueAt)
	assert.Equal(t, 2, f1[1].Homework.SubmissionCount)
	assert.Equal(t, 2, f1[1].Homework.Latest.AttemptNo)

	q := f1[2].Quiz
	require.NotNil(t, q)
	assert.Equal(t, 3, q.AttemptsUsed, "lượt đang làm cũng tính là đã dùng")
	assert.Equal(t, 2, q.Submitted)
	assert.True(t, q.Open)
	assert.Equal(t, ptr(5), q.Limit, "max_attempts 2 cộng 3 lượt cấp thêm")
	assert.Equal(t, &Score{7, 10}, q.Best, "7/10 bằng 14/20 thì lượt sớm hơn thắng")
	assert.Equal(t, &LatestAttempt{Score: Score{14, 20}, ReviewStatus: &pending}, q.Latest, "lượt đã nộp gần nhất, bỏ qua lượt đang làm")

	assert.Nil(t, f1[3].Quiz.Limit, "luyện tập không giới hạn lượt")
	assert.Equal(t, 1, f1[3].Quiz.Submitted)

	f2 := facts[m2]
	require.Len(t, f2, 4)
	assert.Nil(t, f2[0].CompletedAt)
	assert.Equal(t, ptr(now), f2[1].Homework.DueAt, "hạn nộp theo lớp nên mọi thành viên cùng hạn")
	assert.Nil(t, f2[1].Homework.Latest)
	assert.Equal(t, &QuizFacts{Mode: stages.QuizAssessment, PassPercent: ptr(70), Limit: ptr(2)}, f2[2].Quiz)
}

func TestNormalizeReview(t *testing.T) {
	body, err := NormalizeReview("  Tốt  \n")
	require.NoError(t, err)
	assert.Equal(t, "Tốt", body)

	_, err = NormalizeReview(" \n\t ")
	assert.ErrorIs(t, err, ErrReviewRequired)

	long := make([]rune, maxReviewRunes)
	for i := range long {
		long[i] = 'ữ'
	}
	_, err = NormalizeReview(string(long))
	require.NoError(t, err, "đúng %d ký tự (nhiều byte) vẫn hợp lệ", maxReviewRunes)
	_, err = NormalizeReview(string(long) + "a")
	assert.ErrorIs(t, err, ErrReviewTooLong)
}
