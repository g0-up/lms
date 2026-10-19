package quizzes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/learning"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
)

// Service là use case làm quiz của học viên. Mọi đường ghi khóa class_members trước rồi mới khóa quiz_attempts;
// lần tự nộp luôn chạy trong transaction riêng, commit trước khi caller đọc hoặc báo lỗi.
type Service struct {
	db       db.Executor
	tx       db.Tx
	repo     Repo
	members  classes.MembershipReader
	progress ProgressWriter
	clock    clock.Clock
	ids      ids.Generator
	shuffle  Shuffler
}

// Deps là phụ thuộc của Service; DB dùng cho truy vấn đọc ngoài transaction.
type Deps struct {
	DB       db.Executor
	Tx       db.Tx
	Repo     Repo
	Members  classes.MembershipReader
	Progress ProgressWriter
	Clock    clock.Clock
	IDs      ids.Generator
	Shuffle  Shuffler
}

// NewService nối phụ thuộc.
func NewService(d Deps) *Service {
	return &Service{
		db: d.DB, tx: d.Tx, repo: d.Repo, members: d.Members, progress: d.Progress, clock: d.Clock, ids: d.IDs,
		shuffle: d.Shuffle,
	}
}

// learner đọc ngữ cảnh học viên trong lớp và yêu cầu thành viên đang học; lớp không tồn tại cũng là
// learning.ErrNotFound để không lộ lớp.
func (s *Service) learner(ctx context.Context, studentID, classID uuid.UUID) (learning.LearningContext, error) {
	mc, err := s.members.MemberContext(ctx, s.db, classID, studentID)
	if errors.Is(err, classes.ErrClassNotFound) {
		return learning.LearningContext{}, learning.ErrNotFound
	}
	if err != nil {
		return learning.LearningContext{}, err
	}
	lc := learning.LearningContext{Member: mc}
	if err := lc.CanView(); err != nil {
		return learning.LearningContext{}, err
	}
	return lc, nil
}

// quizOf là quiz của học liệu trong phiên bản khóa học của lớp; học liệu khác lớp hoặc không phải quiz →
// ErrLessonNotQuiz.
func (s *Service) quizOf(ctx context.Context, courseVersionID, lessonID uuid.UUID) (*stages.QuizContent, error) {
	q, err := s.repo.QuizInCourseVersion(ctx, s.db, courseVersionID, lessonID)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrLessonNotQuiz
	}
	return q, nil
}

// quizOfAttempt là quiz của học liệu của một lượt đã có.
func (s *Service) quizOfAttempt(ctx context.Context, ex db.Executor, lessonID uuid.UUID) (*stages.QuizContent, error) {
	return quizOfLesson(ctx, s.repo, ex, lessonID)
}

// quizOfLesson là quiz của học liệu của một lượt đã có; học liệu mất cấu hình quiz là lỗi dữ liệu.
func quizOfLesson(ctx context.Context, repo Repo, ex db.Executor, lessonID uuid.UUID) (*stages.QuizContent, error) {
	q, err := repo.Quiz(ctx, ex, lessonID)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, fmt.Errorf("quizzes: học liệu %s của lượt không có cấu hình quiz", lessonID)
	}
	return q, nil
}

// lockedGate kiểm lại điều kiện ghi sau khi khóa thành viên: thành viên đang học, lớp đang chạy.
func lockedGate(m LockedMember) error {
	if m.Status != domain.MemberActive {
		return learning.ErrNotFound
	}
	return classGate(m.ClassStatus)
}

// isGateError cho biết lỗi là từ chối do thành viên đã rời hoặc lớp không còn chạy (không phải sự cố).
func isGateError(err error) bool {
	return errors.Is(err, learning.ErrNotFound) || errors.Is(err, ErrClassEnded) || errors.Is(err, ErrClassNotStarted)
}

// ---- Màn bắt đầu ----

// Overview là màn bắt đầu của quiz: cấu hình, số lượt đã dùng (kể cả lượt mở), lượt tốt nhất và có bắt đầu được
// không. Attempts theo attempt_no, gồm cả lượt mở.
type Overview struct {
	Quiz          *stages.QuizContent
	Limit         *int
	Granted       int
	Attempts      []Attempt
	Open          *Attempt
	Best          *Attempt
	CanStart      bool
	BlockedReason string
}

// Overview đọc màn bắt đầu; lượt mở đã quá ân hạn của chính học viên được tự nộp (transaction riêng) trước khi đọc.
func (s *Service) Overview(ctx context.Context, studentID, classID, lessonID uuid.UUID) (Overview, error) {
	lc, err := s.learner(ctx, studentID, classID)
	if err != nil {
		return Overview{}, err
	}
	quiz, err := s.quizOf(ctx, lc.Member.CourseVersionID, lessonID)
	if err != nil {
		return Overview{}, err
	}
	if lc.CanRecord() == nil {
		if err := s.closeOwnExpired(ctx, lc.Member.MemberID, lessonID); err != nil {
			return Overview{}, err
		}
	}
	attempts, err := s.repo.Attempts(ctx, s.db, lc.Member.MemberID, lessonID)
	if err != nil {
		return Overview{}, err
	}
	granted, err := s.repo.Granted(ctx, s.db, lc.Member.MemberID, lessonID)
	if err != nil {
		return Overview{}, err
	}
	out := Overview{Quiz: quiz, Limit: Limit(*quiz, granted), Granted: granted, Attempts: attempts, Best: Best(attempts)}
	for i := range attempts {
		if attempts[i].IsOpen() {
			out.Open = &attempts[i]
		}
	}
	out.BlockedReason = blockedReason(lc.Member.ClassStatus, out.Limit, len(attempts), out.Open != nil)
	out.CanStart = out.BlockedReason == ""
	return out, nil
}

// closeOwnExpired tự nộp lượt mở đã quá ân hạn của (thành viên, quiz) trong transaction riêng, commit trước khi
// caller đọc. Lớp vừa kết thúc hoặc thành viên vừa rời giữa chừng thì để lượt mở như cũ.
func (s *Service) closeOwnExpired(ctx context.Context, memberID, lessonID uuid.UUID) error {
	attempts, err := s.repo.Attempts(ctx, s.db, memberID, lessonID)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	for i := range attempts {
		if a := &attempts[i]; a.IsOpen() && a.PastGrace(now) {
			if _, err := s.closeOne(ctx, a.ID, now); err != nil && !isGateError(err) {
				return err
			}
		}
	}
	return nil
}

// closeOne chạy autoOrSubmit trong transaction riêng.
func (s *Service) closeOne(ctx context.Context, attemptID uuid.UUID, now time.Time) (Outcome, error) {
	var out Outcome
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		var err error
		out, err = s.autoOrSubmit(ctx, tx, attemptID, now)
		return err
	})
	return out, err
}

// ---- Bắt đầu hoặc tiếp tục lượt ----

// Started là kết quả bắt đầu: Resumed khi trả lại lượt đang mở thay vì tạo lượt mới.
type Started struct {
	Resumed bool
	View    AttemptView
}

// errStaleOpen: lượt mở thấy trong transaction bắt đầu đã quá ân hạn (đồng hồ vượt qua giữa hai transaction); đóng
// lượt đó ở transaction riêng rồi thử lại.
var errStaleOpen = errors.New("quizzes: lượt mở vừa quá ân hạn")

// Start bắt đầu lượt mới hoặc trả lượt đang mở. Chỉ thành viên đang học ở lớp đang chạy; quiz đánh giá giới hạn
// maxAttempts cộng lượt cấp thêm. Lần mở đầu tiên của học liệu được ghi như khi mở trang học liệu.
func (s *Service) Start(ctx context.Context, studentID, classID, lessonID uuid.UUID) (Started, error) {
	lc, err := s.learner(ctx, studentID, classID)
	if err != nil {
		return Started{}, err
	}
	if err := classGate(lc.Member.ClassStatus); err != nil {
		return Started{}, err
	}
	quiz, err := s.quizOf(ctx, lc.Member.CourseVersionID, lessonID)
	if err != nil {
		return Started{}, err
	}
	memberID := lc.Member.MemberID
	for range 2 {
		if err := s.closeOwnExpired(ctx, memberID, lessonID); err != nil {
			return Started{}, err
		}
		var (
			attemptID uuid.UUID
			resumed   bool
		)
		err := s.tx.Transact(ctx, func(tx db.Executor) error {
			var err error
			attemptID, resumed, err = s.start(ctx, tx, memberID, lessonID, quiz)
			return err
		})
		if errors.Is(err, errStaleOpen) {
			continue
		}
		if err != nil {
			return Started{}, err
		}
		v, err := s.view(ctx, attemptID)
		if err != nil {
			return Started{}, err
		}
		return Started{Resumed: resumed, View: v}, nil
	}
	return Started{}, ErrConcurrentChange
}

func (s *Service) start(ctx context.Context, tx db.Executor, memberID, lessonID uuid.UUID, quiz *stages.QuizContent) (uuid.UUID, bool, error) {
	// Khóa thành viên tuần tự hóa các lần bắt đầu song song; lần sau thấy lượt lần trước vừa tạo.
	m, err := s.repo.LockMember(ctx, tx, memberID)
	if err != nil {
		return uuid.Nil, false, err
	}
	if err := lockedGate(m); err != nil {
		return uuid.Nil, false, err
	}
	attempts, err := s.repo.Attempts(ctx, tx, memberID, lessonID)
	if err != nil {
		return uuid.Nil, false, err
	}
	now := s.clock.Now()
	maxNo := 0
	for i := range attempts {
		a := &attempts[i]
		if a.IsOpen() {
			if a.PastGrace(now) {
				return uuid.Nil, false, errStaleOpen
			}
			return a.ID, true, nil
		}
		maxNo = max(maxNo, a.No)
	}
	granted, err := s.repo.Granted(ctx, tx, memberID, lessonID)
	if err != nil {
		return uuid.Nil, false, err
	}
	if limit := Limit(*quiz, granted); limit != nil && len(attempts) >= *limit {
		return uuid.Nil, false, errNoAttemptsLeft(*limit)
	}
	questions, options := attemptOrder(*quiz, s.shuffle)
	a := &Attempt{
		ID: s.ids.New(), MemberID: memberID, LessonID: lessonID, No: maxNo + 1, QuestionOrder: questions,
		OptionOrder: options, StartedAt: now, MaxScore: quiz.MaxScore(),
	}
	if quiz.TimeLimit != nil {
		deadline := now.Add(*quiz.TimeLimit)
		a.DeadlineAt = &deadline
	}
	if err := s.repo.InsertAttempt(ctx, tx, a); err != nil {
		return uuid.Nil, false, err
	}
	if _, err := s.progress.Open(ctx, tx, learning.OpenLesson(memberID, lessonID, now)); err != nil {
		return uuid.Nil, false, err
	}
	return a.ID, false, nil
}

// ---- Đọc lượt ----

// AttemptView là một lượt cùng quiz và câu trả lời để dựng DTO lượt mở hoặc kết quả. Limit, Used, Best tính trên
// mọi lượt của (thành viên, quiz); Review là nhận xét hiện hành.
type AttemptView struct {
	Attempt *Attempt
	Quiz    *stages.QuizContent
	Answers map[uuid.UUID]Answer
	Limit   *int
	Used    int
	Best    *Attempt
	Review  *Review
	Now     time.Time
}

// SavedAt là lần lưu câu gần nhất của lượt; chưa lưu câu nào → nil.
func (v AttemptView) SavedAt() *time.Time {
	var out *time.Time
	for _, a := range v.Answers {
		if out == nil || a.SavedAt.After(*out) {
			t := a.SavedAt
			out = &t
		}
	}
	return out
}

// Attempt đọc lượt của chính học viên; lượt của người khác → ErrAttemptNotFound. Lượt mở đã quá ân hạn được tự
// nộp (transaction riêng) rồi trả kết quả.
func (s *Service) Attempt(ctx context.Context, studentID, attemptID uuid.UUID) (AttemptView, error) {
	ref, lc, err := s.ownAttempt(ctx, studentID, attemptID)
	if err != nil {
		return AttemptView{}, err
	}
	if lc.CanRecord() == nil {
		a, err := s.repo.Attempt(ctx, s.db, ref.ID)
		if err != nil {
			return AttemptView{}, err
		}
		now := s.clock.Now()
		if a.IsOpen() && a.PastGrace(now) {
			if _, err := s.closeOne(ctx, a.ID, now); err != nil && !isGateError(err) {
				return AttemptView{}, err
			}
		}
	}
	return s.view(ctx, ref.ID)
}

// ownAttempt đọc chủ lượt và yêu cầu học viên là chủ lượt, vẫn đang học trong lớp của lượt.
func (s *Service) ownAttempt(ctx context.Context, studentID, attemptID uuid.UUID) (AttemptRef, learning.LearningContext, error) {
	ref, err := s.repo.AttemptRef(ctx, s.db, attemptID)
	if err != nil {
		return AttemptRef{}, learning.LearningContext{}, err
	}
	if ref.UserID != studentID {
		return AttemptRef{}, learning.LearningContext{}, ErrAttemptNotFound
	}
	lc, err := s.learner(ctx, studentID, ref.ClassID)
	if err != nil {
		return AttemptRef{}, learning.LearningContext{}, err
	}
	return ref, lc, nil
}

func (s *Service) view(ctx context.Context, attemptID uuid.UUID) (AttemptView, error) {
	a, err := s.repo.Attempt(ctx, s.db, attemptID)
	if err != nil {
		return AttemptView{}, err
	}
	quiz, err := s.quizOfAttempt(ctx, s.db, a.LessonID)
	if err != nil {
		return AttemptView{}, err
	}
	answers, err := s.repo.Answers(ctx, s.db, a.ID)
	if err != nil {
		return AttemptView{}, err
	}
	attempts, err := s.repo.Attempts(ctx, s.db, a.MemberID, a.LessonID)
	if err != nil {
		return AttemptView{}, err
	}
	granted, err := s.repo.Granted(ctx, s.db, a.MemberID, a.LessonID)
	if err != nil {
		return AttemptView{}, err
	}
	v := AttemptView{
		Attempt: a, Quiz: quiz, Answers: make(map[uuid.UUID]Answer, len(answers)), Limit: Limit(*quiz, granted),
		Used: len(attempts), Best: Best(attempts), Now: s.clock.Now(),
	}
	for _, ans := range answers {
		v.Answers[ans.QuestionID] = ans
	}
	if !a.IsOpen() {
		if v.Review, err = s.repo.LatestReview(ctx, s.db, a.ID); err != nil {
			return AttemptView{}, err
		}
	}
	return v, nil
}

// ---- Lưu câu ----

// SaveAnswer lưu phương án đã chọn của một câu (rỗng là bỏ chọn) và trả giờ lưu. Chỉ lượt đang mở, chưa quá ân
// hạn, của thành viên đang học ở lớp đang chạy.
func (s *Service) SaveAnswer(ctx context.Context, studentID, attemptID, questionID uuid.UUID, optionIDs []uuid.UUID) (time.Time, error) {
	ref, lc, err := s.ownAttempt(ctx, studentID, attemptID)
	if err != nil {
		return time.Time{}, err
	}
	if err := classGate(lc.Member.ClassStatus); err != nil {
		return time.Time{}, err
	}
	now := s.clock.Now()
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		m, err := s.repo.LockMember(ctx, tx, ref.MemberID)
		if err != nil {
			return err
		}
		if err := lockedGate(m); err != nil {
			return err
		}
		a, err := s.repo.AttemptForUpdate(ctx, tx, ref.ID)
		if err != nil {
			return err
		}
		if !a.IsOpen() {
			return ErrAlreadySubmitted
		}
		if a.PastGrace(now) {
			return ErrTimeUpNotSaved
		}
		if !a.HasQuestion(questionID) {
			return ErrQuestionNotInAttempt
		}
		quiz, err := s.quizOfAttempt(ctx, tx, a.LessonID)
		if err != nil {
			return err
		}
		question := questionByID(*quiz, questionID)
		if question == nil {
			return ErrQuestionNotInAttempt
		}
		selected, err := normalizeSelection(question, optionIDs)
		if err != nil {
			return err
		}
		return s.repo.UpsertAnswer(ctx, tx, a.ID, questionID, selected, now)
	})
	if err != nil {
		return time.Time{}, err
	}
	return now, nil
}

// ---- Nộp ----

// Submit nộp lượt và trả kết quả. Lượt đã nộp → ErrAlreadySubmitted; quá ân hạn → lượt được tự nộp với các câu đã
// lưu (đã commit) rồi trả ErrTimeUpAutoSubmitted.
func (s *Service) Submit(ctx context.Context, studentID, attemptID uuid.UUID) (AttemptView, error) {
	ref, lc, err := s.ownAttempt(ctx, studentID, attemptID)
	if err != nil {
		return AttemptView{}, err
	}
	if err := classGate(lc.Member.ClassStatus); err != nil {
		return AttemptView{}, err
	}
	out, err := s.closeOne(ctx, ref.ID, s.clock.Now())
	if err != nil {
		return AttemptView{}, err
	}
	switch out {
	case OutcomeAlreadySubmitted:
		return AttemptView{}, ErrAlreadySubmitted
	case OutcomeExpired:
		return AttemptView{}, ErrTimeUpAutoSubmitted
	}
	return s.view(ctx, ref.ID)
}

// autoOrSubmit là đường chấm duy nhất (nộp tay, tự nộp khi đọc, CloseExpired): khóa thành viên rồi khóa lượt, chấm
// các câu đã lưu và ghi kết quả. Trả Outcome thay vì lỗi để caller commit được lần tự nộp; lỗi chỉ dành cho từ
// chối điều kiện (thành viên rời, lớp không chạy) và sự cố.
func (s *Service) autoOrSubmit(ctx context.Context, tx db.Executor, attemptID uuid.UUID, now time.Time) (Outcome, error) {
	ref, err := s.repo.AttemptRef(ctx, tx, attemptID)
	if err != nil {
		return 0, err
	}
	m, err := s.repo.LockMember(ctx, tx, ref.MemberID)
	if err != nil {
		return 0, err
	}
	if err := lockedGate(m); err != nil {
		return 0, err
	}
	a, err := s.repo.AttemptForUpdate(ctx, tx, attemptID)
	if err != nil {
		return 0, err
	}
	if !a.IsOpen() {
		return OutcomeAlreadySubmitted, nil
	}
	quiz, err := s.quizOfAttempt(ctx, tx, a.LessonID)
	if err != nil {
		return 0, err
	}
	saved, err := s.repo.Answers(ctx, tx, a.ID)
	if err != nil {
		return 0, err
	}
	answers := make(map[uuid.UUID][]uuid.UUID, len(saved))
	for _, ans := range saved {
		answers[ans.QuestionID] = ans.Selected
	}
	outcome, at := OutcomeSubmitted, now
	if a.PastGrace(now) {
		outcome, at = OutcomeExpired, *a.DeadlineAt
		a.AutoSubmitted = true
	}
	score, maxScore, per := Score(*quiz, answers)
	if err := s.repo.GradeAnswers(ctx, tx, a.ID, per, at); err != nil {
		return 0, err
	}
	a.SubmittedAt, a.Score, a.MaxScore, a.Passed = &at, &score, maxScore, Passed(*quiz, score, maxScore)
	if quiz.Mode == stages.QuizAssessment {
		// Mỗi (thành viên, quiz) chỉ một lượt chờ xác nhận: lượt pending cũ nhường chỗ cho lượt vừa nộp.
		if err := s.repo.SupersedePending(ctx, tx, a.MemberID, a.LessonID); err != nil {
			return 0, err
		}
		st := ReviewPending
		a.ReviewStatus = &st
	}
	if err := s.repo.Finish(ctx, tx, a); err != nil {
		return 0, err
	}
	// Hoàn thành: luyện tập ngay lần nộp đầu, đánh giá khi lượt đạt; không bao giờ xóa vì lượt tốt nhất được tính.
	if quiz.Mode == stages.QuizPractice || (a.Passed != nil && *a.Passed) {
		if err := s.progress.MarkCompleted(ctx, tx, a.MemberID, a.LessonID, at); err != nil {
			return 0, err
		}
	}
	return outcome, nil
}

// ---- Tự nộp lượt quá giờ ----

// CloseExpired tự nộp tối đa closeBatch lượt mở đã quá ân hạn, mỗi lượt một transaction. Lỗi của một lượt không
// chặn các lượt khác; trả số lượt đã đóng và các lỗi gộp lại.
func (s *Service) CloseExpired(ctx context.Context, now time.Time) (int, error) {
	idList, err := s.repo.ExpiredOpen(ctx, s.db, now.Add(-SubmitGrace), closeBatch)
	if err != nil {
		return 0, err
	}
	var (
		closed int
		errs   []error
	)
	for _, id := range idList {
		out, err := s.closeOne(ctx, id, now)
		switch {
		case err == nil && out == OutcomeExpired:
			closed++
		case err != nil && !isGateError(err):
			errs = append(errs, fmt.Errorf("quizzes: tự nộp lượt %s: %w", id, err))
		}
	}
	return closed, errors.Join(errs...)
}

// Cleanup cho worker chạy CloseExpired theo lịch dọn dẹp định kỳ.
func (s *Service) Cleanup(ctx context.Context, now time.Time) error {
	_, err := s.CloseExpired(ctx, now)
	return err
}
