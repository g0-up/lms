package quizzes

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
)

// Giới hạn của form nhận xét.
const (
	maxCommentRunes = 5000
	maxNoteRunes    = 1000
)

// Lỗi của xác nhận và nhận xét lượt quiz; copy theo prototype/app.js (reviewAttempt, pageReviewAttempt).
var (
	ErrSubmittedNotFound = domain.ErrNotFound.WithMsg("Không tìm thấy lượt làm bài đã nộp.")
	ErrNotOwnReview      = domain.ErrForbidden.WithMsg("Bạn chỉ nhận xét bài của lớp mình phụ trách.")
	ErrReviewSuperseded  = domain.ErrInvalidTransition.WithMsg("Học viên đã nộp lượt mới hơn. Hãy xác nhận lượt mới nhất.")
	ErrCommentRequired   = domain.ErrInvalid.WithMsg("Nhập nhận xét chung cho học viên.")
	ErrCommentTooLong    = domain.ErrInvalid.WithMsg(fmt.Sprintf("Nhận xét tối đa %d ký tự.", maxCommentRunes))
	ErrNoteTooLong       = domain.ErrInvalid.WithMsg(fmt.Sprintf("Ghi chú từng câu tối đa %d ký tự.", maxNoteRunes))
	ErrNoteNotInAttempt  = domain.ErrInvalid.WithMsg("Ghi chú cho câu không thuộc lượt làm bài này.")
	ErrDuplicateNote     = domain.ErrInvalid.WithMsg("Mỗi câu chỉ có một ghi chú.")
)

// Actor là người dùng đã đăng nhập gọi use case của staff; RequestID gắn vào audit.
type Actor struct {
	ID        uuid.UUID
	Role      domain.Role
	RequestID string
}

// staffCanAccessClass: admin mọi lớp, giảng viên chỉ lớp mình phụ trách (denied khi khác lớp), vai trò khác không.
// Cùng quy tắc với homework và reports.
func staffCanAccessClass(actor Actor, teacherID uuid.UUID, denied error) error {
	switch {
	case actor.Role == domain.RoleAdmin:
		return nil
	case actor.Role == domain.RoleTeacher && actor.ID == teacherID:
		return nil
	case actor.Role == domain.RoleTeacher:
		return denied
	}
	return domain.ErrForbidden
}

// quizTitlePrefix là tiền tố loại bài trong tiêu đề học liệu, bỏ đi khi gọi tên quiz trong email (shortTitle).
var quizTitlePrefix = regexp.MustCompile(`^(Bài tập|Luyện tập|Kiểm tra):\s*`)

// shortTitle là tiêu đề quiz bỏ tiền tố "Bài tập:", "Luyện tập:", "Kiểm tra:".
func shortTitle(title string) string { return quizTitlePrefix.ReplaceAllString(title, "") }

// NoteInput là ghi chú một câu như client gửi.
type NoteInput struct {
	QuestionID string
	Note       string
}

// ReviewInput là form nhận xét như client gửi.
type ReviewInput struct {
	Comment string
	Notes   []NoteInput
}

// ReviewNote là ghi chú đã kiểm của một câu.
type ReviewNote struct {
	QuestionID uuid.UUID
	Note       string
}

// reviewContent kiểm form nhận xét với lượt: nhận xét chung bắt buộc, mỗi ghi chú thuộc một câu của lượt, ghi chú rỗng
// bị bỏ. Ghi chú trả theo thứ tự câu học viên thấy.
func reviewContent(a *Attempt, in ReviewInput) (string, []ReviewNote, error) {
	comment := strings.TrimSpace(in.Comment)
	switch {
	case comment == "":
		return "", nil, ErrCommentRequired
	case utf8.RuneCountInString(comment) > maxCommentRunes:
		return "", nil, ErrCommentTooLong
	}
	byQuestion := make(map[uuid.UUID]string, len(in.Notes))
	for _, n := range in.Notes {
		id, err := uuid.Parse(n.QuestionID)
		if err != nil || !a.HasQuestion(id) {
			return "", nil, ErrNoteNotInAttempt
		}
		if _, ok := byQuestion[id]; ok {
			return "", nil, ErrDuplicateNote
		}
		note := strings.TrimSpace(n.Note)
		if utf8.RuneCountInString(note) > maxNoteRunes {
			return "", nil, ErrNoteTooLong
		}
		byQuestion[id] = note
	}
	notes := make([]ReviewNote, 0, len(byQuestion))
	for _, q := range a.QuestionOrder {
		if note := byQuestion[q]; note != "" {
			notes = append(notes, ReviewNote{QuestionID: q, Note: note})
		}
	}
	return comment, notes, nil
}

// nextReviewStatus là review_status sau khi lưu nhận xét: chờ xác nhận (đánh giá) hoặc chưa nhận xét (luyện tập) thành
// confirmed, đã confirmed giữ nguyên; lượt đã có lượt mới hơn không nhận xét được.
func nextReviewStatus(cur *ReviewStatus) (ReviewStatus, error) {
	if cur != nil && *cur == ReviewSuperseded {
		return "", ErrReviewSuperseded
	}
	return ReviewConfirmed, nil
}

// reviewAction là action audit của một lần lưu nhận xét: lần đầu là xác nhận (đánh giá) hoặc nhận xét (luyện tập), các
// lần sau là sửa nhận xét.
func reviewAction(mode stages.QuizMode, first bool) string {
	switch {
	case !first:
		return audit.ActionQuizAttemptReviewUpdated
	case mode == stages.QuizAssessment:
		return audit.ActionQuizAttemptConfirmed
	default:
		return audit.ActionQuizAttemptCommented
	}
}

// QuestionStatus là kết quả một câu của lượt đã nộp trong trang xem lượt của staff.
type QuestionStatus string

const (
	QuestionCorrect QuestionStatus = "correct"
	QuestionWrong   QuestionStatus = "wrong"
	QuestionBlank   QuestionStatus = "blank"
)

// questionStatus: bỏ trống khi không chọn phương án nào, đúng khi đã chấm đúng, còn lại là sai.
func questionStatus(ans Answer) QuestionStatus {
	switch {
	case len(ans.Selected) == 0:
		return QuestionBlank
	case ans.IsCorrect != nil && *ans.IsCorrect:
		return QuestionCorrect
	default:
		return QuestionWrong
	}
}

// minutesTaken là thời gian làm của lượt đã nộp tính theo phút, làm tròn, tối thiểu 1 như prototype.
func minutesTaken(a *Attempt) int {
	if a.SubmittedAt == nil {
		return 0
	}
	return max(1, int(math.Round(a.SubmittedAt.Sub(a.StartedAt).Minutes())))
}

// Trạng thái của một thành viên với một quiz trong tab Quiz của lớp, theo lessonStatus của prototype.
const (
	MemberQuizTodo          = "todo"
	MemberQuizInProgress    = "inProgress"
	MemberQuizDone          = "done"
	MemberQuizPassed        = "passed"
	MemberQuizFailed        = "failed"
	MemberQuizOutOfAttempts = "outOfAttempts"
)

// memberQuizStatus tính trạng thái từ mọi lượt (theo attempt_no) của thành viên. Luyện tập: đã làm khi có lượt đã nộp.
// Đánh giá: đạt khi lượt tốt nhất đạt; đang làm khi có lượt mở; chưa đạt khi còn lượt, hết lượt khi không còn.
func memberQuizStatus(mode stages.QuizMode, attempts []Attempt, limit *int) string {
	open := slices.ContainsFunc(attempts, func(a Attempt) bool { return a.IsOpen() })
	best := Best(attempts)
	if mode != stages.QuizAssessment {
		switch {
		case best != nil:
			return MemberQuizDone
		case open:
			return MemberQuizInProgress
		default:
			return MemberQuizTodo
		}
	}
	switch {
	case best != nil && best.Passed != nil && *best.Passed:
		return MemberQuizPassed
	case open:
		return MemberQuizInProgress
	case best == nil:
		return MemberQuizTodo
	case limit != nil && len(attempts) >= *limit:
		return MemberQuizOutOfAttempts
	default:
		return MemberQuizFailed
	}
}

// avgPercent là trung bình các phần trăm làm tròn như prototype; rỗng → nil.
func avgPercent(ps []int) *int {
	if len(ps) == 0 {
		return nil
	}
	sum := 0
	for _, p := range ps {
		sum += p
	}
	avg := int(math.Round(float64(sum) / float64(len(ps))))
	return &avg
}
