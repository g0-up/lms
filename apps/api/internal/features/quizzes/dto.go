package quizzes

import (
	"slices"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/features/grading"
	"lms/api/internal/features/stages"
)

// Ba kiểu phản hồi lượt (lượt mở, kết quả luyện tập, kết quả đánh giá) là ba kiểu riêng, không nhúng lẫn nhau: chỉ
// kết quả luyện tập có đáp án đúng; lượt mở và kết quả đánh giá không có trường nào mang đúng/sai hay giải thích.

// Trạng thái lượt trong phản hồi, để web chọn màn làm bài hay màn kết quả.
const (
	statusOpen      = "open"
	statusSubmitted = "submitted"
)

// ---- Màn bắt đầu ----

// OverviewDTO là phản hồi GET /me/classes/:id/lessons/:lid/attempts.
type OverviewDTO struct {
	Mode             string              `json:"mode"`
	QuestionCount    int                 `json:"questionCount"`
	MaxScore         int                 `json:"maxScore"`
	PassPercent      *int                `json:"passPercent,omitempty"`
	TimeLimitMinutes *int                `json:"timeLimitMinutes,omitempty"`
	Limit            *int                `json:"limit,omitempty"`
	Used             int                 `json:"used"`
	Granted          int                 `json:"granted"`
	OpenAttemptID    *string             `json:"openAttemptId,omitempty"`
	Best             *BestDTO            `json:"best,omitempty"`
	Attempts         []AttemptSummaryDTO `json:"attempts"`
	CanStart         bool                `json:"canStart"`
	BlockedReason    string              `json:"blockedReason,omitempty"`
}

// BestDTO là lượt tốt nhất (tính vào tiến độ); passed chỉ có ở quiz đánh giá.
type BestDTO struct {
	AttemptID string `json:"attemptId"`
	Percent   int    `json:"percent"`
	Passed    *bool  `json:"passed,omitempty"`
}

// AttemptSummaryDTO là một lượt đã nộp trong lịch sử của màn bắt đầu.
type AttemptSummaryDTO struct {
	ID            string    `json:"id"`
	No            int       `json:"no"`
	SubmittedAt   time.Time `json:"submittedAt"`
	AutoSubmitted bool      `json:"autoSubmitted"`
	Score         int       `json:"score"`
	MaxScore      int       `json:"maxScore"`
	Percent       int       `json:"percent"`
	Passed        *bool     `json:"passed,omitempty"`
	ReviewStatus  *string   `json:"reviewStatus,omitempty"`
}

func toOverviewDTO(o Overview) OverviewDTO {
	out := OverviewDTO{
		Mode: string(o.Quiz.Mode), QuestionCount: len(o.Quiz.Questions), MaxScore: o.Quiz.MaxScore(),
		PassPercent: o.Quiz.PassPercent, TimeLimitMinutes: minutesOf(o.Quiz.TimeLimit), Limit: o.Limit,
		Used: len(o.Attempts), Granted: o.Granted, Attempts: []AttemptSummaryDTO{}, CanStart: o.CanStart,
		BlockedReason: o.BlockedReason,
	}
	if o.Open != nil {
		id := o.Open.ID.String()
		out.OpenAttemptID = &id
	}
	if o.Best != nil {
		out.Best = &BestDTO{AttemptID: o.Best.ID.String(), Percent: o.Best.Percent(), Passed: o.Best.Passed}
	}
	for _, a := range o.Attempts {
		if a.IsOpen() {
			continue
		}
		out.Attempts = append(out.Attempts, AttemptSummaryDTO{
			ID: a.ID.String(), No: a.No, SubmittedAt: a.SubmittedAt.UTC(), AutoSubmitted: a.AutoSubmitted,
			Score: *a.Score, MaxScore: a.MaxScore, Percent: a.Percent(), Passed: a.Passed, ReviewStatus: reviewOf(a),
		})
	}
	return out
}

func minutesOf(d *time.Duration) *int {
	if d == nil {
		return nil
	}
	m := int(*d / time.Minute)
	return &m
}

func reviewOf(a Attempt) *string {
	if a.ReviewStatus == nil {
		return nil
	}
	s := string(*a.ReviewStatus)
	return &s
}

// ---- Lượt mở ----

// StartDTO là phản hồi POST /me/classes/:id/lessons/:lid/attempts.
type StartDTO struct {
	Resumed bool           `json:"resumed"`
	Attempt OpenAttemptDTO `json:"attempt"`
}

// OpenAttemptDTO là lượt đang làm: câu hỏi và phương án theo thứ tự đã chốt của lượt, phương án đã chọn; không có
// đáp án, giải thích hay đúng/sai. serverNow để client tính đồng hồ theo độ lệch với server.
type OpenAttemptDTO struct {
	ID         string            `json:"id"`
	Status     string            `json:"status"`
	Mode       string            `json:"mode"`
	No         int               `json:"no"`
	Limit      *int              `json:"limit"`
	DeadlineAt *time.Time        `json:"deadlineAt"`
	ServerNow  time.Time         `json:"serverNow"`
	SavedAt    *time.Time        `json:"savedAt"`
	Questions  []OpenQuestionDTO `json:"questions"`
}

// OpenQuestionDTO là một câu của lượt mở.
type OpenQuestionDTO struct {
	ID                string          `json:"id"`
	Type              string          `json:"type"`
	BodyHTML          string          `json:"bodyHtml"`
	Points            int             `json:"points"`
	Options           []OpenOptionDTO `json:"options"`
	SelectedOptionIDs []string        `json:"selectedOptionIds"`
}

// OpenOptionDTO là một phương án của lượt mở: chỉ id và nội dung.
type OpenOptionDTO struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

func toOpenAttemptDTO(v AttemptView) OpenAttemptDTO {
	a := v.Attempt
	out := OpenAttemptDTO{
		ID: a.ID.String(), Status: statusOpen, Mode: string(v.Quiz.Mode), No: a.No, Limit: v.Limit,
		DeadlineAt: utcPtr(a.DeadlineAt), ServerNow: v.Now.UTC(), SavedAt: utcPtr(v.SavedAt()),
		Questions: make([]OpenQuestionDTO, 0, len(a.QuestionOrder)),
	}
	for _, qid := range a.QuestionOrder {
		q := questionByID(*v.Quiz, qid)
		if q == nil {
			continue
		}
		dto := OpenQuestionDTO{
			ID: q.ID.String(), Type: string(q.Type), BodyHTML: deref(q.BodyHTML), Points: q.Points,
			Options: make([]OpenOptionDTO, 0, len(q.Options)), SelectedOptionIDs: idStrings(v.Answers[q.ID].Selected),
		}
		for _, o := range optionsInOrder(q, a.OptionOrder[q.ID]) {
			dto.Options = append(dto.Options, OpenOptionDTO{ID: o.ID.String(), Body: o.Body})
		}
		out.Questions = append(out.Questions, dto)
	}
	return out
}

// optionsInOrder trả phương án của câu theo thứ tự đã chốt của lượt; phương án không có trong thứ tự (không xảy ra
// với phiên bản đã phát hành) đứng cuối theo thứ tự soạn.
func optionsInOrder(q *stages.Question, order []uuid.UUID) []stages.Option {
	out := make([]stages.Option, 0, len(q.Options))
	for _, id := range order {
		if i := slices.IndexFunc(q.Options, func(o stages.Option) bool { return o.ID == id }); i >= 0 {
			out = append(out, q.Options[i])
		}
	}
	for _, o := range q.Options {
		if !slices.Contains(order, o.ID) {
			out = append(out, o)
		}
	}
	return out
}

// ---- Kết quả luyện tập ----

// PracticeResultDTO là kết quả lượt luyện tập: từng câu theo thứ tự soạn với đáp án đúng, phương án đã chọn, giải
// thích và ghi chú của giảng viên.
type PracticeResultDTO struct {
	ID            string                `json:"id"`
	Status        string                `json:"status"`
	Mode          string                `json:"mode"`
	No            int                   `json:"no"`
	StartedAt     time.Time             `json:"startedAt"`
	SubmittedAt   time.Time             `json:"submittedAt"`
	AutoSubmitted bool                  `json:"autoSubmitted"`
	Score         int                   `json:"score"`
	MaxScore      int                   `json:"maxScore"`
	Percent       int                   `json:"percent"`
	Questions     []PracticeQuestionDTO `json:"questions"`
	Review        *PracticeReviewDTO    `json:"review,omitempty"`
}

// PracticeQuestionDTO là một câu trong kết quả luyện tập.
type PracticeQuestionDTO struct {
	ID              string              `json:"id"`
	Type            string              `json:"type"`
	BodyHTML        string              `json:"bodyHtml"`
	Points          int                 `json:"points"`
	PointsAwarded   int                 `json:"pointsAwarded"`
	Correct         bool                `json:"correct"`
	ExplanationHTML *string             `json:"explanationHtml,omitempty"`
	Note            *string             `json:"note,omitempty"`
	Options         []PracticeOptionDTO `json:"options"`
}

// PracticeOptionDTO là một phương án trong kết quả luyện tập.
type PracticeOptionDTO struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	IsCorrect bool   `json:"isCorrect"`
	Selected  bool   `json:"selected"`
}

// PracticeReviewDTO là nhận xét hiện hành của giảng viên; ghi chú từng câu nằm trong câu.
type PracticeReviewDTO struct {
	Comment      string    `json:"comment"`
	ReviewerName string    `json:"reviewerName"`
	ReviewedAt   time.Time `json:"reviewedAt"`
	Updated      bool      `json:"updated"`
}

func toPracticeResultDTO(v AttemptView) PracticeResultDTO {
	a := v.Attempt
	out := PracticeResultDTO{
		ID: a.ID.String(), Status: statusSubmitted, Mode: string(v.Quiz.Mode), No: a.No, StartedAt: a.StartedAt.UTC(),
		SubmittedAt: a.SubmittedAt.UTC(), AutoSubmitted: a.AutoSubmitted, Score: deref(a.Score), MaxScore: a.MaxScore,
		Percent: a.Percent(), Questions: make([]PracticeQuestionDTO, 0, len(v.Quiz.Questions)),
	}
	for _, q := range v.Quiz.Questions {
		ans := v.Answers[q.ID]
		dto := PracticeQuestionDTO{
			ID: q.ID.String(), Type: string(q.Type), BodyHTML: deref(q.BodyHTML), Points: q.Points,
			PointsAwarded: deref(ans.Points), Correct: deref(ans.IsCorrect), ExplanationHTML: q.ExplanationHTML,
			Options: make([]PracticeOptionDTO, 0, len(q.Options)),
		}
		if v.Review != nil {
			if n, ok := v.Review.Notes[q.ID]; ok {
				dto.Note = &n
			}
		}
		for _, o := range q.Options {
			dto.Options = append(dto.Options, PracticeOptionDTO{
				ID: o.ID.String(), Body: o.Body, IsCorrect: o.Correct, Selected: slices.Contains(ans.Selected, o.ID),
			})
		}
		out.Questions = append(out.Questions, dto)
	}
	if r := v.Review; r != nil {
		out.Review = &PracticeReviewDTO{Comment: r.Comment, ReviewerName: r.ReviewerName, ReviewedAt: r.CreatedAt.UTC(), Updated: r.Updated}
	}
	return out
}

// ---- Kết quả đánh giá ----

// AssessmentResultDTO là kết quả lượt quiz đánh giá: chỉ điểm, Đạt và nhận xét. Không có phương án, đáp án hay
// đúng/sai từng câu; ghi chú từng câu chỉ kèm nội dung câu hỏi.
type AssessmentResultDTO struct {
	ID            string               `json:"id"`
	Status        string               `json:"status"`
	Mode          string               `json:"mode"`
	No            int                  `json:"no"`
	StartedAt     time.Time            `json:"startedAt"`
	SubmittedAt   time.Time            `json:"submittedAt"`
	AutoSubmitted bool                 `json:"autoSubmitted"`
	Score         int                  `json:"score"`
	MaxScore      int                  `json:"maxScore"`
	Percent       int                  `json:"percent"`
	Passed        *bool                `json:"passed"`
	PassPercent   *int                 `json:"passPercent"`
	ReviewStatus  *string              `json:"reviewStatus"`
	Best          *AssessmentBestDTO   `json:"best,omitempty"`
	AttemptsLeft  *int                 `json:"attemptsLeft"`
	Review        *AssessmentReviewDTO `json:"review,omitempty"`
}

// AssessmentBestDTO là tỉ lệ của lượt tốt nhất.
type AssessmentBestDTO struct {
	Percent int `json:"percent"`
}

// AssessmentReviewDTO là nhận xét hiện hành của giảng viên cho lượt đánh giá.
type AssessmentReviewDTO struct {
	Comment      string              `json:"comment"`
	ReviewerName string              `json:"reviewerName"`
	ReviewedAt   time.Time           `json:"reviewedAt"`
	Updated      bool                `json:"updated"`
	Notes        []AssessmentNoteDTO `json:"notes"`
}

// AssessmentNoteDTO là ghi chú của giảng viên cho một câu; position là số thứ tự câu theo thứ tự học viên thấy (từ 1),
// trùng số câu trên trang xác nhận của giảng viên.
type AssessmentNoteDTO struct {
	QuestionID string `json:"questionId"`
	Position   int    `json:"position"`
	BodyHTML   string `json:"bodyHtml"`
	Note       string `json:"note"`
}

func toAssessmentResultDTO(v AttemptView) AssessmentResultDTO {
	a := v.Attempt
	out := AssessmentResultDTO{
		ID: a.ID.String(), Status: statusSubmitted, Mode: string(v.Quiz.Mode), No: a.No, StartedAt: a.StartedAt.UTC(),
		SubmittedAt: a.SubmittedAt.UTC(), AutoSubmitted: a.AutoSubmitted, Score: deref(a.Score), MaxScore: a.MaxScore,
		Percent: a.Percent(), Passed: a.Passed, PassPercent: v.Quiz.PassPercent, ReviewStatus: reviewOf(*a),
	}
	if v.Best != nil {
		out.Best = &AssessmentBestDTO{Percent: v.Best.Percent()}
	}
	if v.Limit != nil {
		left := max(0, *v.Limit-v.Used)
		out.AttemptsLeft = &left
	}
	if r := v.Review; r != nil {
		rv := &AssessmentReviewDTO{
			Comment: r.Comment, ReviewerName: r.ReviewerName, ReviewedAt: r.CreatedAt.UTC(), Updated: r.Updated,
			Notes: []AssessmentNoteDTO{},
		}
		for i, qid := range a.QuestionOrder {
			q := questionByID(*v.Quiz, qid)
			if q == nil {
				continue
			}
			if n, ok := r.Notes[q.ID]; ok {
				rv.Notes = append(rv.Notes, AssessmentNoteDTO{QuestionID: q.ID.String(), Position: i + 1, BodyHTML: deref(q.BodyHTML), Note: n})
			}
		}
		out.Review = rv
	}
	return out
}

// toAttemptDTO chọn kiểu phản hồi theo trạng thái lượt và chế độ quiz.
func toAttemptDTO(v AttemptView) any {
	switch {
	case v.Attempt.IsOpen():
		return toOpenAttemptDTO(v)
	case v.Quiz.Mode == stages.QuizAssessment:
		return toAssessmentResultDTO(v)
	default:
		return toPracticeResultDTO(v)
	}
}

// ---- Xem lượt và nhận xét của staff ----

// Kiểu phản hồi của staff tách khỏi kiểu của học viên: staff thấy đáp án đúng và phương án đã chọn của mọi lượt.

// StaffStudentDTO là học viên của lượt.
type StaffStudentDTO struct {
	MemberID     string `json:"memberId"`
	UserID       string `json:"userId"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	MemberStatus string `json:"memberStatus"`
}

// ClassRefDTO là lớp của lượt hoặc của tổng quan.
type ClassRefDTO struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func toClassRefDTO(c ClassInfo) ClassRefDTO {
	return ClassRefDTO{ID: c.ID.String(), Code: c.Code, Name: c.Name, Status: string(c.Status)}
}

// StageRefDTO là chặng chứa quiz.
type StageRefDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LessonRefDTO là học liệu quiz.
type LessonRefDTO struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// AttemptDetailDTO là phản hồi GET /quiz-attempts/:aid. minutes là thời gian làm (phút, tối thiểu 1); correctCount
// trên questionCount là số câu đúng; reviews mới nhất trước, dòng đầu là nhận xét hiện hành; attempts là các lượt đã
// nộp cùng quiz của học viên, mới nhất trước.
type AttemptDetailDTO struct {
	ID               string              `json:"id"`
	Student          StaffStudentDTO     `json:"student"`
	Class            ClassRefDTO         `json:"class"`
	Stage            StageRefDTO         `json:"stage"`
	Lesson           LessonRefDTO        `json:"lesson"`
	Mode             string              `json:"mode"`
	No               int                 `json:"no"`
	Limit            *int                `json:"limit"`
	StartedAt        time.Time           `json:"startedAt"`
	SubmittedAt      time.Time           `json:"submittedAt"`
	AutoSubmitted    bool                `json:"autoSubmitted"`
	Minutes          int                 `json:"minutes"`
	TimeLimitMinutes *int                `json:"timeLimitMinutes"`
	Score            int                 `json:"score"`
	MaxScore         int                 `json:"maxScore"`
	Percent          int                 `json:"percent"`
	CorrectCount     int                 `json:"correctCount"`
	QuestionCount    int                 `json:"questionCount"`
	Passed           *bool               `json:"passed"`
	PassPercent      *int                `json:"passPercent"`
	ReviewStatus     *string             `json:"reviewStatus"`
	IsLatest         bool                `json:"isLatest"`
	LatestAttemptID  string              `json:"latestAttemptId"`
	Questions        []ReviewQuestionDTO `json:"questions"`
	Reviews          []ReviewDTO         `json:"reviews"`
	Attempts         []StaffAttemptDTO   `json:"attempts"`
}

// ReviewQuestionDTO là một câu của lượt theo thứ tự học viên thấy; status là correct, wrong hoặc blank.
type ReviewQuestionDTO struct {
	ID            string              `json:"id"`
	Type          string              `json:"type"`
	BodyHTML      string              `json:"bodyHtml"`
	Points        int                 `json:"points"`
	PointsAwarded int                 `json:"pointsAwarded"`
	Status        string              `json:"status"`
	Options       []PracticeOptionDTO `json:"options"`
}

// ReviewDTO là một lần nhận xét của giảng viên.
type ReviewDTO struct {
	ID        string          `json:"id"`
	Reviewer  ReviewerDTO     `json:"reviewer"`
	CreatedAt time.Time       `json:"createdAt"`
	Comment   string          `json:"comment"`
	Notes     []ReviewNoteDTO `json:"notes"`
}

// ReviewerDTO là người nhận xét.
type ReviewerDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ReviewNoteDTO là ghi chú của giảng viên cho một câu.
type ReviewNoteDTO struct {
	QuestionID string `json:"questionId"`
	Note       string `json:"note"`
}

// StaffAttemptDTO là một lượt đã nộp trong lịch sử của trang xem lượt, kèm nhận xét hiện hành.
type StaffAttemptDTO struct {
	ID            string     `json:"id"`
	No            int        `json:"no"`
	SubmittedAt   time.Time  `json:"submittedAt"`
	AutoSubmitted bool       `json:"autoSubmitted"`
	Score         int        `json:"score"`
	MaxScore      int        `json:"maxScore"`
	Percent       int        `json:"percent"`
	Passed        *bool      `json:"passed"`
	ReviewStatus  *string    `json:"reviewStatus"`
	Review        *ReviewDTO `json:"review"`
}

func toAttemptDetailDTO(d AttemptDetail) AttemptDetailDTO {
	a, ac := d.Attempt, d.Context
	out := AttemptDetailDTO{
		ID: a.ID.String(),
		Student: StaffStudentDTO{
			MemberID: ac.MemberID.String(), UserID: ac.UserID.String(), Name: ac.StudentName, Email: ac.StudentEmail,
			MemberStatus: string(ac.MemberStatus),
		},
		Class: toClassRefDTO(ac.Class), Stage: StageRefDTO{ID: ac.StageID.String(), Name: ac.StageName},
		Lesson: LessonRefDTO{ID: ac.LessonID.String(), Title: ac.LessonTitle}, Mode: string(d.Quiz.Mode), No: a.No,
		Limit: d.Limit, StartedAt: a.StartedAt.UTC(), SubmittedAt: a.SubmittedAt.UTC(), AutoSubmitted: a.AutoSubmitted,
		Minutes: minutesTaken(&a), TimeLimitMinutes: minutesOf(d.Quiz.TimeLimit), Score: deref(a.Score),
		MaxScore: a.MaxScore, Percent: a.Percent(), QuestionCount: len(a.QuestionOrder), Passed: a.Passed,
		PassPercent: d.Quiz.PassPercent, ReviewStatus: reviewOf(a), IsLatest: d.IsLatest(),
		LatestAttemptID: d.LatestAttemptID.String(), Questions: make([]ReviewQuestionDTO, 0, len(a.QuestionOrder)),
		Reviews: make([]ReviewDTO, len(d.Reviews)), Attempts: make([]StaffAttemptDTO, len(d.History)),
	}
	for _, qid := range a.QuestionOrder {
		q := questionByID(*d.Quiz, qid)
		if q == nil {
			continue
		}
		ans := d.Answers[q.ID]
		st := questionStatus(ans)
		if st == QuestionCorrect {
			out.CorrectCount++
		}
		dto := ReviewQuestionDTO{
			ID: q.ID.String(), Type: string(q.Type), BodyHTML: deref(q.BodyHTML), Points: q.Points,
			PointsAwarded: deref(ans.Points), Status: string(st), Options: make([]PracticeOptionDTO, 0, len(q.Options)),
		}
		for _, o := range optionsInOrder(q, a.OptionOrder[q.ID]) {
			dto.Options = append(dto.Options, PracticeOptionDTO{
				ID: o.ID.String(), Body: o.Body, IsCorrect: o.Correct, Selected: slices.Contains(ans.Selected, o.ID),
			})
		}
		out.Questions = append(out.Questions, dto)
	}
	for i, r := range d.Reviews {
		out.Reviews[i] = toReviewDTO(r)
	}
	for i, h := range d.History {
		out.Attempts[i] = StaffAttemptDTO{
			ID: h.ID.String(), No: h.No, SubmittedAt: h.SubmittedAt.UTC(), AutoSubmitted: h.AutoSubmitted,
			Score: deref(h.Score), MaxScore: h.MaxScore, Percent: h.Percent(), Passed: h.Passed, ReviewStatus: reviewOf(h),
		}
		if r, ok := d.Current[h.ID]; ok {
			rv := toReviewDTO(r)
			out.Attempts[i].Review = &rv
		}
	}
	return out
}

func toReviewDTO(r ReviewRecord) ReviewDTO {
	out := ReviewDTO{
		ID: r.ID.String(), Reviewer: ReviewerDTO{ID: r.ReviewerID.String(), Name: r.ReviewerName},
		CreatedAt: r.CreatedAt.UTC(), Comment: r.Comment, Notes: make([]ReviewNoteDTO, len(r.Notes)),
	}
	for i, n := range r.Notes {
		out.Notes[i] = ReviewNoteDTO{QuestionID: n.QuestionID.String(), Note: n.Note}
	}
	return out
}

// reviewRequest là body POST /quiz-attempts/:aid/reviews.
type reviewRequest struct {
	Comment string              `json:"comment"`
	Notes   []reviewNoteRequest `json:"notes"`
}

type reviewNoteRequest struct {
	QuestionID string `json:"questionId"`
	Note       string `json:"note"`
}

func (r reviewRequest) input() ReviewInput {
	in := ReviewInput{Comment: r.Comment, Notes: make([]NoteInput, len(r.Notes))}
	for i, n := range r.Notes {
		in.Notes[i] = NoteInput(n)
	}
	return in
}

// ReviewOutcomeDTO là phản hồi POST /quiz-attempts/:aid/reviews. nextQueueItem là mục kế tiếp trong hàng chờ chấm
// sau lần xác nhận đầu của lượt đánh giá; nhận xét luyện tập, sửa nhận xét hoặc hết hàng chờ là null.
type ReviewOutcomeDTO struct {
	Review        ReviewDTO             `json:"review"`
	ReviewStatus  string                `json:"reviewStatus"`
	Updated       bool                  `json:"updated"`
	NextQueueItem *grading.QueueItemDTO `json:"nextQueueItem"`
}

func toReviewOutcomeDTO(o ReviewOutcome) ReviewOutcomeDTO {
	out := ReviewOutcomeDTO{Review: toReviewDTO(o.Review), ReviewStatus: string(o.ReviewStatus), Updated: o.Updated}
	if o.Next != nil {
		next := grading.ToQueueItemDTO(*o.Next)
		out.NextQueueItem = &next
	}
	return out
}

// ---- Tổng quan quiz của lớp ----

// ClassQuizzesDTO là phản hồi GET /classes/:id/quizzes.
type ClassQuizzesDTO struct {
	Class ClassRefDTO   `json:"class"`
	Items []QuizItemDTO `json:"items"`
}

// QuizItemDTO là một quiz của lớp. grants là các lần cấp thêm lượt cho học viên của lớp ở quiz này.
type QuizItemDTO struct {
	Lesson           LessonRefDTO     `json:"lesson"`
	Stage            StageRefDTO      `json:"stage"`
	Mode             string           `json:"mode"`
	MaxAttempts      *int             `json:"maxAttempts,omitempty"`
	TimeLimitMinutes *int             `json:"timeLimitMinutes,omitempty"`
	PassPercent      *int             `json:"passPercent,omitempty"`
	QuestionCount    int              `json:"questionCount"`
	Stats            QuizStatsDTO     `json:"stats"`
	Members          []MemberQuizDTO  `json:"members"`
	Grants           []GrantDTO       `json:"grants"`
	QuestionStats    QuestionStatsDTO `json:"questionStats"`
}

// GrantDTO là một lần cấp thêm lượt; cũng là phản hồi POST /classes/:id/members/:mid/grants.
type GrantDTO struct {
	ID            string    `json:"id"`
	MemberID      string    `json:"memberId"`
	LessonID      string    `json:"lessonId"`
	StudentName   string    `json:"studentName"`
	ExtraAttempts int       `json:"extraAttempts"`
	Reason        string    `json:"reason"`
	GrantedByName string    `json:"grantedByName"`
	GrantedAt     time.Time `json:"grantedAt"`
}

func toGrantDTO(g Grant) GrantDTO {
	return GrantDTO{
		ID: g.ID.String(), MemberID: g.MemberID.String(), LessonID: g.LessonID.String(), StudentName: g.StudentName,
		ExtraAttempts: g.ExtraAttempts, Reason: g.Reason, GrantedByName: g.GrantedByName, GrantedAt: g.GrantedAt.UTC(),
	}
}

// QuestionStatsDTO là tỉ lệ trả lời đúng từng câu của một quiz; correctPercent null khi chưa có lượt nào.
type QuestionStatsDTO struct {
	Attempts int               `json:"attempts"`
	Rows     []QuestionStatDTO `json:"rows"`
}

// QuestionStatDTO là một câu trong thống kê; bodyText là văn bản thuần đã cắt, không render HTML.
type QuestionStatDTO struct {
	QuestionID     string `json:"questionId"`
	Position       int    `json:"position"`
	BodyText       string `json:"bodyText"`
	CorrectPercent *int   `json:"correctPercent"`
}

func toQuestionStatsDTO(q QuestionStats) QuestionStatsDTO {
	out := QuestionStatsDTO{Attempts: q.Attempts, Rows: make([]QuestionStatDTO, len(q.Rows))}
	for i, r := range q.Rows {
		out.Rows[i] = QuestionStatDTO{QuestionID: r.QuestionID.String(), Position: r.Position, BodyText: r.BodyText, CorrectPercent: r.CorrectPercent}
	}
	return out
}

// grantRequest là thân POST cấp thêm lượt; extraAttempts là con trỏ để phân biệt thiếu trường với 0. lessonId sai
// định dạng không thể là quiz của lớp nên trả ErrGrantNotAssessment.
type grantRequest struct {
	LessonID      string `json:"lessonId"`
	ExtraAttempts *int   `json:"extraAttempts"`
	Reason        string `json:"reason"`
}

func (r grantRequest) input() (GrantInput, error) {
	id, err := uuid.Parse(r.LessonID)
	if err != nil {
		return GrantInput{}, ErrGrantNotAssessment
	}
	return GrantInput{LessonID: id, ExtraAttempts: r.ExtraAttempts, Reason: r.Reason}, nil
}

// QuizStatsDTO là thống kê của một quiz trên thành viên đang học.
type QuizStatsDTO struct {
	Attempted         int  `json:"attempted"`
	Members           int  `json:"members"`
	Passed            int  `json:"passed"`
	Pending           int  `json:"pending"`
	AvgBestPercent    *int `json:"avgBestPercent"`
	SubmittedAttempts int  `json:"submittedAttempts"`
}

// MemberQuizDTO là trạng thái một thành viên với một quiz.
type MemberQuizDTO struct {
	MemberID    string         `json:"memberId"`
	Name        string         `json:"name"`
	Used        int            `json:"used"`
	Limit       *int           `json:"limit"`
	Granted     int            `json:"granted"`
	BestPercent *int           `json:"bestPercent"`
	Status      string         `json:"status"`
	Latest      *LatestQuizDTO `json:"latest"`
}

// LatestQuizDTO là lượt đã nộp mới nhất của thành viên.
type LatestQuizDTO struct {
	AttemptID    string    `json:"attemptId"`
	No           int       `json:"no"`
	Percent      int       `json:"percent"`
	SubmittedAt  time.Time `json:"submittedAt"`
	ReviewStatus *string   `json:"reviewStatus"`
}

func toClassQuizzesDTO(v ClassQuizzes) ClassQuizzesDTO {
	out := ClassQuizzesDTO{Class: toClassRefDTO(v.Class), Items: make([]QuizItemDTO, len(v.Items))}
	for i, it := range v.Items {
		l := it.Lesson
		members := make([]MemberQuizDTO, len(it.Members))
		for j, m := range it.Members {
			md := MemberQuizDTO{
				MemberID: m.MemberID.String(), Name: m.Name, Used: m.Used, Limit: m.Limit, Granted: m.Granted,
				BestPercent: m.BestPercent, Status: m.Status,
			}
			if a := m.Latest; a != nil {
				md.Latest = &LatestQuizDTO{
					AttemptID: a.ID.String(), No: a.No, Percent: a.Percent(), SubmittedAt: a.SubmittedAt.UTC(),
					ReviewStatus: reviewOf(*a),
				}
			}
			members[j] = md
		}
		out.Items[i] = QuizItemDTO{
			Lesson: LessonRefDTO{ID: l.LessonID.String(), Title: l.Title}, Stage: StageRefDTO{ID: l.StageID.String(), Name: l.StageName},
			Mode: string(l.Quiz.Mode), MaxAttempts: l.Quiz.MaxAttempts, TimeLimitMinutes: minutesOf(l.Quiz.TimeLimit),
			PassPercent: l.Quiz.PassPercent, QuestionCount: l.QuestionCount,
			Stats: QuizStatsDTO{
				Attempted: it.Stats.Attempted, Members: it.Stats.Members, Passed: it.Stats.Passed, Pending: it.Stats.Pending,
				AvgBestPercent: it.Stats.AvgBestPercent, SubmittedAttempts: it.Stats.SubmittedAttempts,
			},
			Members: members, Grants: make([]GrantDTO, len(it.Grants)), QuestionStats: toQuestionStatsDTO(it.Questions),
		}
		for j, g := range it.Grants {
			out.Items[i].Grants[j] = toGrantDTO(g)
		}
	}
	return out
}

// ---- Lưu câu ----

// saveAnswerRequest: optionIds bắt buộc (con trỏ để phân biệt mảng rỗng, tức bỏ chọn, với vắng mặt).
type saveAnswerRequest struct {
	OptionIDs *[]string `json:"optionIds"`
}

// SavedDTO là phản hồi PUT /me/attempts/:aid/answers/:qid.
type SavedDTO struct {
	SavedAt time.Time `json:"savedAt"`
}

// parseOptionIDs đọc id phương án; id sai định dạng không thể thuộc câu nên trả ErrOptionNotInQuestion.
func parseOptionIDs(raw []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, ErrOptionNotInQuestion
		}
		out = append(out, id)
	}
	return out, nil
}

// ---- Tiện ích ----

func idStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
