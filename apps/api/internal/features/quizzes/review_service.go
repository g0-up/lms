package quizzes

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/grading"
	"lms/api/internal/features/mailer"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
)

// ReviewService là use case của staff với lượt quiz đã nộp: xem lượt kèm đáp án, xác nhận hoặc nhận xét, tổng quan
// quiz của lớp. Chỉ admin hoặc giảng viên phụ trách lớp. Đường ghi khóa class_members trước rồi mới khóa lượt, cùng
// thứ tự với nộp bài, nên xác nhận và nộp lượt mới của cùng thành viên chạy tuần tự.
type ReviewService struct {
	db            db.Executor
	tx            db.Tx
	repo          Repo
	mail          mailer.Enqueuer
	audit         audit.Recorder
	clock         clock.Clock
	ids           ids.Generator
	next          grading.NextFinder
	publicBaseURL string
}

// ReviewDeps là phụ thuộc của ReviewService; DB dùng cho truy vấn đọc ngoài transaction. PublicBaseURL dựng link
// trang kết quả trong email nhận xét.
type ReviewDeps struct {
	DB            db.Executor
	Tx            db.Tx
	Repo          Repo
	Mail          mailer.Enqueuer
	Audit         audit.Recorder
	Clock         clock.Clock
	IDs           ids.Generator
	Next          grading.NextFinder
	PublicBaseURL string
}

// NewReviewService nối phụ thuộc.
func NewReviewService(d ReviewDeps) *ReviewService {
	return &ReviewService{
		db: d.DB, tx: d.Tx, repo: d.Repo, mail: d.Mail, audit: d.Audit, clock: d.Clock, ids: d.IDs, next: d.Next,
		publicBaseURL: d.PublicBaseURL,
	}
}

// ---- Xem lượt đã nộp ----

// AttemptDetail là trang xem một lượt đã nộp của staff. Reviews là các lần nhận xét của lượt (mới nhất trước, dòng
// đầu là hiện hành); History là các lượt đã nộp cùng (thành viên, quiz), mới nhất trước, Current là nhận xét hiện
// hành của từng lượt trong đó.
type AttemptDetail struct {
	Context         AttemptContext
	Quiz            *stages.QuizContent
	Attempt         Attempt
	Answers         map[uuid.UUID]Answer
	Limit           *int
	Reviews         []ReviewRecord
	History         []Attempt
	Current         map[uuid.UUID]ReviewRecord
	LatestAttemptID uuid.UUID
}

// IsLatest cho biết lượt đang xem là lượt nộp mới nhất của học viên ở quiz này.
func (d AttemptDetail) IsLatest() bool { return d.LatestAttemptID == d.Attempt.ID }

// AttemptDetail đọc một lượt đã nộp; lượt đang làm là NOT_FOUND vì staff không xem lượt chưa nộp.
func (s *ReviewService) AttemptDetail(ctx context.Context, actor Actor, attemptID uuid.UUID) (AttemptDetail, error) {
	ac, err := s.repo.SubmittedAttemptContext(ctx, s.db, attemptID)
	if err != nil {
		return AttemptDetail{}, err
	}
	if err := staffCanAccessClass(actor, ac.Class.TeacherID, classes.ErrNotOwnClass); err != nil {
		return AttemptDetail{}, err
	}
	quiz, err := quizOfLesson(ctx, s.repo, s.db, ac.LessonID)
	if err != nil {
		return AttemptDetail{}, err
	}
	attempts, err := s.repo.Attempts(ctx, s.db, ac.MemberID, ac.LessonID)
	if err != nil {
		return AttemptDetail{}, err
	}
	answers, err := s.repo.Answers(ctx, s.db, attemptID)
	if err != nil {
		return AttemptDetail{}, err
	}
	granted, err := s.repo.Granted(ctx, s.db, ac.MemberID, ac.LessonID)
	if err != nil {
		return AttemptDetail{}, err
	}
	reviews, err := s.repo.Reviews(ctx, s.db, ac.MemberID, ac.LessonID)
	if err != nil {
		return AttemptDetail{}, err
	}
	out := AttemptDetail{
		Context: ac, Quiz: quiz, Answers: make(map[uuid.UUID]Answer, len(answers)), Limit: Limit(*quiz, granted),
		Current: make(map[uuid.UUID]ReviewRecord),
	}
	for _, ans := range answers {
		out.Answers[ans.QuestionID] = ans
	}
	found := false
	for i := len(attempts) - 1; i >= 0; i-- {
		a := attempts[i]
		if a.IsOpen() {
			continue
		}
		if a.ID == attemptID {
			out.Attempt, found = a, true
		}
		out.History = append(out.History, a)
	}
	if !found {
		return AttemptDetail{}, fmt.Errorf("quizzes: lượt %s vắng trong các lượt của thành viên", attemptID)
	}
	out.LatestAttemptID = out.History[0].ID
	for _, r := range reviews {
		if r.AttemptID == attemptID {
			out.Reviews = append(out.Reviews, r)
		}
		if _, ok := out.Current[r.AttemptID]; !ok {
			out.Current[r.AttemptID] = r
		}
	}
	return out, nil
}

// ---- Xác nhận và nhận xét ----

// ReviewOutcome là kết quả một lần lưu nhận xét: lần nhận xét vừa ghi, review_status sau khi lưu, có phải sửa nhận
// xét không và mục kế tiếp trong hàng chờ chấm (chỉ lần xác nhận đầu của lượt đánh giá; còn lại nil).
type ReviewOutcome struct {
	Review       ReviewRecord
	ReviewStatus ReviewStatus
	Updated      bool
	Next         *grading.QueueItem
}

// Review lưu nhận xét cho lượt đã nộp: ghi lần nhận xét mới (lịch sử giữ nguyên), xác nhận lượt nếu chưa, audit và
// đưa email vào outbox, tất cả trong một transaction. Điểm, kết quả Đạt và mốc hoàn thành không đổi.
func (s *ReviewService) Review(ctx context.Context, actor Actor, attemptID uuid.UUID, in ReviewInput) (ReviewOutcome, error) {
	var out ReviewOutcome
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		ac, err := s.repo.SubmittedAttemptContext(ctx, tx, attemptID)
		if err != nil {
			return err
		}
		if err := staffCanAccessClass(actor, ac.Class.TeacherID, ErrNotOwnReview); err != nil {
			return err
		}
		if _, err := s.repo.LockMember(ctx, tx, ac.MemberID); err != nil {
			return err
		}
		a, err := s.repo.AttemptForUpdate(ctx, tx, attemptID)
		if err != nil {
			return err
		}
		status, err := nextReviewStatus(a.ReviewStatus)
		if err != nil {
			return err
		}
		comment, notes, err := reviewContent(a, in)
		if err != nil {
			return err
		}
		quiz, err := quizOfLesson(ctx, s.repo, tx, a.LessonID)
		if err != nil {
			return err
		}
		before, err := s.currentReview(ctx, tx, ac, attemptID)
		if err != nil {
			return err
		}
		reviewID := s.ids.New()
		if err := s.repo.InsertReview(ctx, tx, NewReview{
			ID: reviewID, AttemptID: attemptID, ReviewerID: actor.ID, Comment: comment, Notes: notes, CreatedAt: s.clock.Now(),
		}); err != nil {
			return err
		}
		if a.ReviewStatus == nil || *a.ReviewStatus != status {
			if err := s.repo.ConfirmReview(ctx, tx, attemptID, a.ReviewStatus); err != nil {
				return err
			}
		}
		if err := s.recordReview(ctx, tx, actor, ac, a, quiz.Mode, before, comment, notes); err != nil {
			return err
		}
		if err := s.enqueueReviewed(ctx, tx, ac, a, quiz.Mode, comment, before != nil); err != nil {
			return err
		}
		// Đọc lại lần nhận xét vừa ghi để trả cùng dạng với lịch sử (tên người nhận xét).
		current, err := s.currentReview(ctx, tx, ac, attemptID)
		if err != nil {
			return err
		}
		if current == nil || current.ID != reviewID {
			return fmt.Errorf("quizzes: lần nhận xét %s vắng sau khi ghi", reviewID)
		}
		out = ReviewOutcome{Review: *current, ReviewStatus: status, Updated: before != nil}
		// Mục kế tiếp đọc trong cùng transaction nên lượt vừa xác nhận đã rời hàng chờ; sửa nhận xét và lượt luyện
		// tập không dẫn tiếp.
		if !out.Updated && quiz.Mode == stages.QuizAssessment {
			key := grading.QueueKey{Kind: grading.KindQuiz, ItemID: attemptID}
			if out.Next, err = s.next.Next(ctx, tx, grading.Actor{ID: actor.ID, Role: actor.Role}, key); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ReviewOutcome{}, err
	}
	return out, nil
}

// currentReview là nhận xét hiện hành của lượt; chưa có → nil.
func (s *ReviewService) currentReview(ctx context.Context, ex db.Executor, ac AttemptContext, attemptID uuid.UUID) (*ReviewRecord, error) {
	reviews, err := s.repo.Reviews(ctx, ex, ac.MemberID, ac.LessonID)
	if err != nil {
		return nil, err
	}
	for i := range reviews {
		if reviews[i].AttemptID == attemptID {
			return &reviews[i], nil
		}
	}
	return nil, nil
}

func (s *ReviewService) recordReview(ctx context.Context, tx db.Executor, actor Actor, ac AttemptContext, a *Attempt, mode stages.QuizMode, previous *ReviewRecord, comment string, notes []ReviewNote) error {
	base := func(comment string, notes []ReviewNote) map[string]any {
		list := make([]map[string]any, len(notes))
		for i, n := range notes {
			list[i] = map[string]any{"questionId": n.QuestionID, "note": n.Note}
		}
		return map[string]any{
			"classCode": ac.Class.Code, "lessonId": ac.LessonID, "lessonTitle": ac.LessonTitle, "memberId": ac.MemberID,
			"studentName": ac.StudentName, "attemptId": a.ID, "attemptNo": a.No, "comment": comment, "notes": list,
		}
	}
	var before any
	if previous != nil {
		before = base(previous.Comment, previous.Notes)
	}
	actorID := actor.ID
	return s.audit.Record(ctx, tx, audit.Entry{
		ActorID: &actorID, Action: reviewAction(mode, previous == nil), TargetType: "class", TargetID: &ac.Class.ID,
		Before: before, After: base(comment, notes), RequestID: actor.RequestID,
	})
}

// enqueueReviewed đưa email nhận xét vào outbox. Payload chỉ có tên quiz, số lượt, %, kết quả và nhận xét chung:
// không có nội dung câu, phương án, giải thích hay ghi chú từng câu. Link trang kết quả ở khóa ResultURL (mailer từ
// chối khóa Link).
func (s *ReviewService) enqueueReviewed(ctx context.Context, tx db.Executor, ac AttemptContext, a *Attempt, mode stages.QuizMode, comment string, updated bool) error {
	to, err := domain.ParseEmail(ac.StudentEmail)
	if err != nil {
		return fmt.Errorf("quizzes: email học viên %s: %w", ac.UserID, err)
	}
	assessment := mode == stages.QuizAssessment
	resultLabel := ""
	if assessment && a.Passed != nil {
		resultLabel = "Chưa đạt"
		if *a.Passed {
			resultLabel = "Đạt"
		}
	}
	payload := map[string]any{
		"Name": ac.StudentName, "ClassName": ac.Class.Name, "ClassCode": ac.Class.Code, "QuizTitle": shortTitle(ac.LessonTitle),
		"AttemptNo": a.No, "Percent": a.Percent(), "ResultLabel": resultLabel, "Assessment": assessment, "Updated": updated,
		"Comment":   comment,
		"ResultURL": fmt.Sprintf("%s/learn/classes/%s/lessons/%s?result=%s", s.publicBaseURL, ac.Class.ID, ac.LessonID, a.ID),
	}
	_, err = s.mail.Enqueue(ctx, tx, mailer.Message{To: to, Template: mailer.TemplateQuizReviewed, Payload: payload})
	return err
}

// ---- Tổng quan quiz của lớp ----

// ClassQuizzes là mọi quiz của phiên bản khóa học của lớp, mỗi quiz kèm thống kê và trạng thái từng thành viên đang
// học.
type ClassQuizzes struct {
	Class ClassInfo
	Items []QuizItem
}

// QuizItem là một quiz trong tổng quan lớp; Grants là lịch sử cấp thêm lượt, mới nhất trước.
type QuizItem struct {
	Lesson    QuizLesson
	Stats     QuizStats
	Members   []MemberQuiz
	Grants    []Grant
	Questions QuestionStats
}

// QuizStats tính trên thành viên đang học, theo renderQuizTab của prototype: Attempted là số thành viên đã nộp ít
// nhất một lượt, Passed là số thành viên đã đạt (đánh giá) hoặc đã làm (luyện tập), Pending là số thành viên có lượt
// chờ xác nhận, AvgBestPercent là trung bình lượt tốt nhất của các thành viên đã nộp, SubmittedAttempts là tổng số
// lượt đã nộp.
type QuizStats struct {
	Attempted         int
	Members           int
	Passed            int
	Pending           int
	AvgBestPercent    *int
	SubmittedAttempts int
}

// MemberQuiz là trạng thái một thành viên với một quiz; Used đếm cả lượt đang mở, Latest là lượt đã nộp mới nhất.
type MemberQuiz struct {
	MemberID    uuid.UUID
	Name        string
	Used        int
	Limit       *int
	Granted     int
	BestPercent *int
	Status      string
	Latest      *Attempt
}

// ClassQuizzes đọc tổng quan quiz của lớp bằng năm truy vấn tập hợp; chỉ staff phụ trách. Tỉ lệ đúng từng câu lấy mẫu
// số là số lượt đã nộp của thành viên đang học (kể cả lượt bị thay thế), như questionRates của prototype.
func (s *ReviewService) ClassQuizzes(ctx context.Context, actor Actor, classID uuid.UUID) (ClassQuizzes, error) {
	cls, err := s.repo.Class(ctx, s.db, classID)
	if err != nil {
		return ClassQuizzes{}, err
	}
	if err := staffCanAccessClass(actor, cls.TeacherID, classes.ErrNotOwnClass); err != nil {
		return ClassQuizzes{}, err
	}
	lessons, err := s.repo.QuizLessons(ctx, s.db, cls.CourseVersionID)
	if err != nil {
		return ClassQuizzes{}, err
	}
	members, err := s.repo.ActiveMembers(ctx, s.db, classID)
	if err != nil {
		return ClassQuizzes{}, err
	}
	attempts, err := s.repo.ClassAttempts(ctx, s.db, classID)
	if err != nil {
		return ClassQuizzes{}, err
	}
	grants, err := s.repo.ClassGrants(ctx, s.db, classID)
	if err != nil {
		return ClassQuizzes{}, err
	}
	correct, err := s.repo.QuestionTallies(ctx, s.db, classID, cls.CourseVersionID)
	if err != nil {
		return ClassQuizzes{}, err
	}
	type key struct{ member, lesson uuid.UUID }
	attemptsOf := make(map[key][]Attempt)
	for _, a := range attempts {
		k := key{a.MemberID, a.LessonID}
		attemptsOf[k] = append(attemptsOf[k], a)
	}
	grantedOf := make(map[key]int)
	grantsOf := make(map[uuid.UUID][]Grant)
	for _, g := range grants {
		grantedOf[key{g.MemberID, g.LessonID}] += g.ExtraAttempts
		grantsOf[g.LessonID] = append(grantsOf[g.LessonID], g)
	}
	correctOf := make(map[uuid.UUID][]QuestionTally)
	for _, q := range correct {
		correctOf[q.LessonID] = append(correctOf[q.LessonID], q)
	}
	out := ClassQuizzes{Class: cls, Items: make([]QuizItem, len(lessons))}
	for i, l := range lessons {
		item := QuizItem{
			Lesson: l, Stats: QuizStats{Members: len(members)}, Members: make([]MemberQuiz, len(members)),
			Grants: grantsOf[l.LessonID],
		}
		var bests []int
		for j, m := range members {
			k := key{m.MemberID, l.LessonID}
			list := attemptsOf[k]
			mq := MemberQuiz{
				MemberID: m.MemberID, Name: m.Name, Used: len(list), Granted: grantedOf[k],
				Limit: Limit(l.Quiz, grantedOf[k]),
			}
			mq.Status = memberQuizStatus(l.Quiz.Mode, list, mq.Limit)
			pending := false
			for n := range list {
				a := &list[n]
				if a.IsOpen() {
					continue
				}
				item.Stats.SubmittedAttempts++
				mq.Latest = a
				pending = pending || (a.ReviewStatus != nil && *a.ReviewStatus == ReviewPending)
			}
			if best := Best(list); best != nil {
				p := best.Percent()
				mq.BestPercent = &p
				bests = append(bests, p)
				item.Stats.Attempted++
			}
			if mq.Status == MemberQuizPassed || mq.Status == MemberQuizDone {
				item.Stats.Passed++
			}
			if pending {
				item.Stats.Pending++
			}
			item.Members[j] = mq
		}
		item.Stats.AvgBestPercent = avgPercent(bests)
		item.Questions = questionStats(correctOf[l.LessonID], item.Stats.SubmittedAttempts)
		out.Items[i] = item
	}
	return out, nil
}
