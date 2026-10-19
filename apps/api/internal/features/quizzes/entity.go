// Package quizzes là feature làm quiz của học viên: bắt đầu hoặc tiếp tục lượt, lưu từng câu, nộp, chấm ở server và
// tự nộp lượt quá giờ. Đáp án đúng không rời server trước khi nộp; với quiz đánh giá thì không bao giờ rời server.
package quizzes

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
)

// SubmitGrace là ân hạn sau giờ hết hạn: lưu câu và nộp tay vẫn được nhận trong khoảng này, quá ân hạn thì lượt
// được tự nộp với các câu đã lưu.
const SubmitGrace = 30 * time.Second

// closeBatch là số lượt quá giờ tối đa CloseExpired đóng mỗi lần chạy.
const closeBatch = 100

// Lỗi nghiệp vụ; copy theo prototype/app.js (learnerGate, startAttempt, saveAnswer, submitAttempt).
var (
	ErrLessonNotQuiz        = domain.ErrNotFound.WithMsg("Bài quiz không thuộc lớp này.")
	ErrAttemptNotFound      = domain.ErrNotFound.WithMsg("Không tìm thấy lượt làm bài.")
	ErrQuestionNotInAttempt = domain.ErrNotFound.WithMsg("Câu hỏi không thuộc lượt làm bài này.")
	ErrOptionNotInQuestion  = domain.ErrInvalid.WithMsg("Phương án không thuộc câu hỏi này.")
	ErrSingleChoice         = domain.ErrInvalid.WithMsg("Câu một đáp án chỉ chọn được một phương án.")
	ErrOptionIDsRequired    = domain.ErrInvalid.WithMsg("Gửi danh sách phương án đã chọn; danh sách rỗng là bỏ chọn.")
	ErrClassNotStarted      = domain.ErrInvalidTransition.WithMsg("Lớp chưa bắt đầu.")
	ErrClassEnded           = domain.ErrInvalidTransition.WithMsg("Lớp đã kết thúc, chỉ xem được học liệu.")
	ErrAlreadySubmitted     = domain.ErrInvalidTransition.WithMsg("Lượt làm bài này đã được nộp.")
	ErrTimeUpNotSaved       = domain.ErrInvalidTransition.WithMsg("Đã hết giờ làm bài; câu trả lời không được lưu.")
	ErrTimeUpAutoSubmitted  = domain.ErrInvalidTransition.WithMsg("Đã hết giờ làm bài; hệ thống đã tự nộp với các câu đã lưu.")
	ErrConcurrentChange     = domain.ErrConflict.WithMsg("Xung đột dữ liệu, vui lòng thử lại")
)

// errNoAttemptsLeft: học viên đã dùng hết lượt của quiz đánh giá.
func errNoAttemptsLeft(limit int) error {
	return domain.ErrInvalidTransition.WithMsg(fmt.Sprintf("Bạn đã dùng hết %d lượt làm bài.", limit))
}

// ReviewStatus là trạng thái xác nhận của lượt quiz đánh giá đã nộp; lượt luyện tập không có.
type ReviewStatus string

const (
	ReviewPending    ReviewStatus = "pending"
	ReviewConfirmed  ReviewStatus = "confirmed"
	ReviewSuperseded ReviewStatus = "superseded"
)

// ParseReviewStatus đọc review_status từ database.
func ParseReviewStatus(s string) (ReviewStatus, error) {
	switch r := ReviewStatus(s); r {
	case ReviewPending, ReviewConfirmed, ReviewSuperseded:
		return r, nil
	default:
		return "", fmt.Errorf("quizzes: review_status %q không hợp lệ", s)
	}
}

// Attempt là một lượt làm quiz. Thứ tự câu và phương án chốt lúc bắt đầu; Score, Passed và ReviewStatus chỉ có sau
// khi nộp.
type Attempt struct {
	ID            uuid.UUID
	MemberID      uuid.UUID
	LessonID      uuid.UUID
	No            int
	QuestionOrder []uuid.UUID
	OptionOrder   map[uuid.UUID][]uuid.UUID
	StartedAt     time.Time
	DeadlineAt    *time.Time
	SubmittedAt   *time.Time
	AutoSubmitted bool
	Score         *int
	MaxScore      int
	Passed        *bool
	ReviewStatus  *ReviewStatus
}

// IsOpen cho biết lượt chưa nộp.
func (a *Attempt) IsOpen() bool { return a.SubmittedAt == nil }

// PastGrace đúng khi lượt có giờ hết hạn và now đã qua giờ đó cộng ân hạn.
func (a *Attempt) PastGrace(now time.Time) bool {
	return a.DeadlineAt != nil && now.After(a.DeadlineAt.Add(SubmitGrace))
}

// HasQuestion cho biết câu thuộc lượt (theo thứ tự đã chốt).
func (a *Attempt) HasQuestion(id uuid.UUID) bool {
	for _, q := range a.QuestionOrder {
		if q == id {
			return true
		}
	}
	return false
}

// Percent là điểm của lượt đã nộp theo phần trăm, làm tròn như báo cáo.
func (a *Attempt) Percent() int {
	if a.Score == nil {
		return 0
	}
	return domain.Percent(*a.Score, a.MaxScore)
}

// Outcome là kết quả của autoOrSubmit; không phải lỗi để transaction vẫn commit lần tự nộp.
type Outcome int

const (
	// OutcomeSubmitted: nộp trước ân hạn, submitted_at là lúc nộp.
	OutcomeSubmitted Outcome = iota + 1
	// OutcomeAlreadySubmitted: lượt đã được nộp ở đường khác; không ghi gì.
	OutcomeAlreadySubmitted
	// OutcomeExpired: quá ân hạn, lượt được tự nộp với submitted_at = deadline_at.
	OutcomeExpired
)

// Limit là số lượt tối đa: quiz đánh giá được maxAttempts cộng lượt cấp thêm, luyện tập không giới hạn (nil).
func Limit(q stages.QuizContent, granted int) *int {
	if q.Mode != stages.QuizAssessment || q.MaxAttempts == nil {
		return nil
	}
	n := *q.MaxAttempts + granted
	return &n
}

// Passed là kết quả Đạt của quiz đánh giá (score*100 >= ngưỡng*max, không làm tròn); luyện tập trả nil.
func Passed(q stages.QuizContent, score, maxScore int) *bool {
	if q.Mode != stages.QuizAssessment || q.PassPercent == nil {
		return nil
	}
	ok := score*100 >= *q.PassPercent*maxScore
	return &ok
}

// Best là lượt đã nộp có tỉ lệ điểm cao nhất (bằng nhau thì lượt sớm hơn), như quizBest của prototype; nil khi
// chưa nộp lượt nào. attempts theo thứ tự attempt_no.
func Best(attempts []Attempt) *Attempt {
	var best *Attempt
	for i := range attempts {
		a := &attempts[i]
		if a.Score == nil {
			continue
		}
		// So score/max bằng nhân chéo để không sai số dấu phẩy động.
		if best == nil || *a.Score*best.MaxScore > *best.Score*a.MaxScore {
			best = a
		}
	}
	return best
}

// classGate là điều kiện ghi của lượt làm bài: lớp đang chạy. Thành viên đang học kiểm riêng (learning.CanView).
func classGate(st domain.ClassStatus) error {
	switch st {
	case domain.ClassActive:
		return nil
	case domain.ClassDraft:
		return ErrClassNotStarted
	default:
		return ErrClassEnded
	}
}

// blockedReason là lý do không bắt đầu được lượt mới, nguyên văn quizView của prototype; "" khi bắt đầu hoặc tiếp
// tục được. Đã đạt không chặn làm thêm lượt.
func blockedReason(class domain.ClassStatus, limit *int, used int, hasOpen bool) string {
	switch {
	case class == domain.ClassEnded:
		return "Lớp đã kết thúc"
	case class != domain.ClassActive:
		return "Lớp chưa bắt đầu"
	case limit != nil && used >= *limit && !hasOpen:
		return fmt.Sprintf("Bạn đã dùng hết %d lượt", *limit)
	default:
		return ""
	}
}
