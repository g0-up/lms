package homework

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/grading"
	"lms/api/internal/features/learning"
	"lms/api/internal/features/mailer"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
)

// Service là use case hạn nộp, nộp bài và chấm. Mọi use case ghi chạy trong một transaction; đường ghi lên một
// thành viên luôn khóa class_members trước rồi mới khóa bản nộp.
type Service struct {
	db            db.Executor
	tx            db.Tx
	repo          Repo
	members       classes.MembershipReader
	lessons       LessonReader
	completion    learning.CompletionWriter
	mail          mailer.Enqueuer
	audit         audit.Recorder
	clock         clock.Clock
	ids           ids.Generator
	next          grading.NextFinder
	publicBaseURL string
}

// Deps là phụ thuộc của Service; DB dùng cho truy vấn đọc ngoài transaction. PublicBaseURL dựng link trang học
// liệu trong email kết quả chấm.
type Deps struct {
	DB            db.Executor
	Tx            db.Tx
	Repo          Repo
	Members       classes.MembershipReader
	Lessons       LessonReader
	Completion    learning.CompletionWriter
	Mail          mailer.Enqueuer
	Audit         audit.Recorder
	Clock         clock.Clock
	IDs           ids.Generator
	Next          grading.NextFinder
	PublicBaseURL string
}

// NewService nối phụ thuộc.
func NewService(d Deps) *Service {
	return &Service{
		db: d.DB, tx: d.Tx, repo: d.Repo, members: d.Members, lessons: d.Lessons, completion: d.Completion,
		mail: d.Mail, audit: d.Audit, clock: d.Clock, ids: d.IDs, next: d.Next, publicBaseURL: d.PublicBaseURL,
	}
}

// lessonOf là học liệu homework trong phiên bản khóa học của lớp cùng nội dung đề; học liệu khác lớp hoặc không phải
// homework → ErrLessonNotInClass.
func (s *Service) lessonOf(ctx context.Context, ex db.Executor, courseVersionID, lessonID uuid.UUID) (*learning.LessonRow, stages.HomeworkContent, error) {
	row, err := s.lessons.LessonInCourseVersion(ctx, ex, courseVersionID, lessonID)
	if err != nil {
		return nil, stages.HomeworkContent{}, err
	}
	if row == nil {
		return nil, stages.HomeworkContent{}, ErrLessonNotInClass
	}
	c, ok := row.Content.(stages.HomeworkContent)
	if !ok {
		return nil, stages.HomeworkContent{}, ErrLessonNotInClass
	}
	return row, c, nil
}

func criteriaOf(c stages.HomeworkContent) ([]Criterion, int) {
	out, maxPoints := make([]Criterion, len(c.Criteria)), 0
	for i, cr := range c.Criteria {
		out[i] = Criterion{ID: cr.ID, Description: cr.Description, MaxPoints: cr.MaxPoints}
		maxPoints += cr.MaxPoints
	}
	return out, maxPoints
}

// ---- Hạn nộp ----

// DeadlineView là hạn nộp của một bài tập ở lớp sau khi đổi; DueAt nil là không có hạn.
type DeadlineView struct {
	LessonID uuid.UUID
	DueAt    *time.Time
}

// SetDeadline đặt hoặc đổi hạn nộp; ClearDeadline xóa hạn (bài tập không còn hạn). Chỉ staff phụ trách, lớp nháp
// hoặc đang chạy, học liệu là homework của phiên bản khóa học của lớp. Mỗi lần đổi ghi audit với hạn cũ và mới.
func (s *Service) SetDeadline(ctx context.Context, actor Actor, classID, lessonID uuid.UUID, dueAt time.Time) (DeadlineView, error) {
	due := dueAt
	return s.changeDeadline(ctx, actor, classID, lessonID, &due)
}

// ClearDeadline xem SetDeadline.
func (s *Service) ClearDeadline(ctx context.Context, actor Actor, classID, lessonID uuid.UUID) (DeadlineView, error) {
	return s.changeDeadline(ctx, actor, classID, lessonID, nil)
}

func (s *Service) changeDeadline(ctx context.Context, actor Actor, classID, lessonID uuid.UUID, dueAt *time.Time) (DeadlineView, error) {
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		// Khóa hàng lớp: tuần tự với chuyển trạng thái lớp (kết thúc lớp, đổi phiên bản khóa học).
		cls, err := s.repo.ClassForUpdate(ctx, tx, classID)
		if err != nil {
			return err
		}
		if err := staffCanAccessClass(actor, cls.TeacherID, ErrNotOwnDeadline); err != nil {
			return err
		}
		if cls.Status == domain.ClassEnded {
			return ErrClassEndedDeadline
		}
		row, _, err := s.lessonOf(ctx, tx, cls.CourseVersionID, lessonID)
		if err != nil {
			return err
		}
		old, err := s.repo.Deadline(ctx, tx, classID, lessonID)
		if err != nil {
			return err
		}
		if dueAt == nil {
			if old == nil {
				return nil // không có gì để xóa, không ghi audit
			}
			if err := s.repo.DeleteDeadline(ctx, tx, classID, lessonID); err != nil {
				return err
			}
		} else if err := s.repo.UpsertDeadline(ctx, tx, Deadline{
			ClassID: classID, LessonID: lessonID, DueAt: *dueAt, SetBy: actor.ID, UpdatedAt: s.clock.Now(),
		}); err != nil {
			return err
		}
		payload := func(due *time.Time) map[string]any {
			return map[string]any{"classCode": cls.Code, "lessonId": lessonID, "lessonTitle": row.Title, "dueAt": utcOrNil(due)}
		}
		return s.record(ctx, tx, actor, audit.ActionClassDeadlineChanged, classID, payload(old), payload(dueAt))
	})
	if err != nil {
		return DeadlineView{}, err
	}
	return DeadlineView{LessonID: lessonID, DueAt: dueAt}, nil
}

func utcOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

// ---- Nộp bài ----

// Trạng thái homework của học viên: chưa nộp hoặc trạng thái bản nộp mới nhất.
const statusNotSubmitted = "notSubmitted"

// MySubmissions là homework của học viên trong lớp: hạn nộp, trạng thái hiện hành, có nộp được không (kèm lý do
// khóa) và mọi bản nộp mới nhất trước, mỗi bản kèm mọi lần chấm (Current là kết quả hiện hành).
type MySubmissions struct {
	LessonID     uuid.UUID
	Deadline     *time.Time
	Status       string
	CanSubmit    bool
	LockedReason string
	MaxFiles     int
	Submissions  []SubmissionRecord
}

// MySubmissions đọc homework của học viên; chỉ thành viên đang học của lớp (FORBIDDEN với người ngoài lớp hoặc đã
// rời lớp), lớp nháp hoặc đã kết thúc vẫn xem được nhưng không nộp được.
func (s *Service) MySubmissions(ctx context.Context, studentID, classID, lessonID uuid.UUID) (MySubmissions, error) {
	mc, err := s.members.MemberContext(ctx, s.db, classID, studentID)
	if err != nil {
		return MySubmissions{}, err
	}
	if !mc.IsMember() {
		return MySubmissions{}, ErrNotMember
	}
	if !mc.IsActiveMember() {
		return MySubmissions{}, ErrMemberDropped
	}
	if _, _, err := s.lessonOf(ctx, s.db, mc.CourseVersionID, lessonID); err != nil {
		return MySubmissions{}, err
	}
	return s.mySubmissions(ctx, s.db, classID, lessonID, mc.MemberID, MemberGate(true, mc.MemberStatus, mc.ClassStatus))
}

func (s *Service) mySubmissions(ctx context.Context, ex db.Executor, classID, lessonID, memberID uuid.UUID, gate error) (MySubmissions, error) {
	deadline, err := s.repo.Deadline(ctx, ex, classID, lessonID)
	if err != nil {
		return MySubmissions{}, err
	}
	history, err := s.repo.History(ctx, ex, memberID, lessonID)
	if err != nil {
		return MySubmissions{}, err
	}
	out := MySubmissions{LessonID: lessonID, Deadline: deadline, Status: statusNotSubmitted, MaxFiles: MaxFiles, Submissions: history}
	var latest *SubmissionStatus
	if len(history) > 0 {
		latest = &history[0].Status
		out.Status = string(*latest)
	}
	if gate == nil {
		_, gate = NextStatusOnResubmit(latest)
	}
	// Lý do khóa là thông điệp của lỗi mà lần nộp sẽ nhận.
	var de *domain.Error
	switch {
	case gate == nil:
		out.CanSubmit = true
	case errors.As(gate, &de):
		out.LockedReason = de.Msg
	default:
		return MySubmissions{}, gate
	}
	return out, nil
}

// Submit ghi một lần nộp: chỉ thành viên đang học của lớp active; bài đã Đạt không nộp lại; bản đang chờ chấm bị
// thay (superseded). Kiểm lại trạng thái sau khi khóa class_members nên hai lần nộp đồng thời chạy tuần tự.
func (s *Service) Submit(ctx context.Context, studentID, classID, lessonID uuid.UUID, in SubmitInput) (MySubmissions, error) {
	var out MySubmissions
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		mc, err := s.members.MemberContext(ctx, tx, classID, studentID)
		if err != nil {
			return err
		}
		if err := MemberGate(mc.IsMember(), mc.MemberStatus, mc.ClassStatus); err != nil {
			return err
		}
		if _, _, err := s.lessonOf(ctx, tx, mc.CourseVersionID, lessonID); err != nil {
			return err
		}
		norm, err := in.Normalize()
		if err != nil {
			return err
		}
		locked, err := s.repo.LockMember(ctx, tx, mc.MemberID)
		if err != nil {
			return err
		}
		if err := MemberGate(true, locked.Status, locked.ClassStatus); err != nil {
			return err
		}
		latest, err := s.repo.Latest(ctx, tx, mc.MemberID, lessonID)
		if err != nil {
			return err
		}
		attempt := 1
		var latestStatus *SubmissionStatus
		if latest != nil {
			attempt, latestStatus = latest.AttemptNo+1, &latest.Status
		}
		supersede, err := NextStatusOnResubmit(latestStatus)
		if err != nil {
			return err
		}
		if supersede {
			if err := s.repo.Supersede(ctx, tx, latest.ID); err != nil {
				return err
			}
		}
		if n, err := s.repo.UsableFiles(ctx, tx, studentID, norm.MediaIDs); err != nil {
			return err
		} else if n != len(norm.MediaIDs) {
			fe := &FieldErrors{}
			fe.add("mediaIds", ErrFileUnavailable)
			return fe
		}
		deadline, err := s.repo.Deadline(ctx, tx, classID, lessonID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		if err := s.repo.InsertSubmission(ctx, tx, NewSubmission{
			ID: s.ids.New(), MemberID: mc.MemberID, LessonID: lessonID, AttemptNo: attempt,
			RepoURL: optional(norm.RepoURL), CommitRef: optional(norm.CommitRef), SubmittedAt: now,
			IsLate: IsLate(deadline, now), MediaIDs: norm.MediaIDs,
		}); err != nil {
			return err
		}
		out, err = s.mySubmissions(ctx, tx, classID, lessonID, mc.MemberID, nil)
		return err
	})
	if err != nil {
		return MySubmissions{}, err
	}
	return out, nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---- Chấm ----

// SubmissionDetail là trang chấm một bản nộp: bản nộp, học viên, lớp, chặng, đề bài với tiêu chí và ngưỡng, hạn nộp,
// bản nộp mới nhất của (thành viên, bài tập) và toàn bộ lịch sử nộp kèm mọi lần chấm.
type SubmissionDetail struct {
	Submission         SubmissionRecord
	Student            Student
	Class              ClassInfo
	StageID            uuid.UUID
	StageName          string
	Lesson             LessonInfo
	Deadline           *time.Time
	IsLatest           bool
	LatestSubmissionID uuid.UUID
	History            []SubmissionRecord
}

// Student là học viên của bản nộp.
type Student struct {
	MemberID     uuid.UUID
	UserID       uuid.UUID
	Name         string
	Email        string
	MemberStatus domain.MemberStatus
}

// LessonInfo là đề bài homework để chấm.
type LessonInfo struct {
	ID          uuid.UUID
	Title       string
	BriefHTML   *string
	PassPercent int
	MaxPoints   int
	Criteria    []Criterion
}

// SubmissionDetail đọc trang chấm; chỉ staff phụ trách lớp của bản nộp.
func (s *Service) SubmissionDetail(ctx context.Context, actor Actor, submissionID uuid.UUID) (SubmissionDetail, error) {
	sc, err := s.repo.SubmissionContext(ctx, s.db, submissionID)
	if err != nil {
		return SubmissionDetail{}, err
	}
	if err := staffCanAccessClass(actor, sc.Class.TeacherID, classes.ErrNotOwnClass); err != nil {
		return SubmissionDetail{}, err
	}
	row, content, err := s.lessonOf(ctx, s.db, sc.Class.CourseVersionID, sc.LessonID)
	if err != nil {
		return SubmissionDetail{}, err
	}
	deadline, err := s.repo.Deadline(ctx, s.db, sc.Class.ID, sc.LessonID)
	if err != nil {
		return SubmissionDetail{}, err
	}
	history, err := s.repo.History(ctx, s.db, sc.MemberID, sc.LessonID)
	if err != nil {
		return SubmissionDetail{}, err
	}
	criteria, maxPoints := criteriaOf(content)
	out := SubmissionDetail{
		Student: Student{MemberID: sc.MemberID, UserID: sc.UserID, Name: sc.StudentName, Email: sc.StudentEmail, MemberStatus: sc.MemberStatus},
		Class:   sc.Class, StageID: row.StageID, StageName: row.StageName,
		Lesson:   LessonInfo{ID: row.ID, Title: row.Title, BriefHTML: content.BriefHTML, PassPercent: content.PassPercent, MaxPoints: maxPoints, Criteria: criteria},
		Deadline: deadline, History: history,
	}
	found := false
	for _, h := range history {
		if h.ID == submissionID {
			out.Submission, found = h, true
		}
	}
	if !found || len(history) == 0 {
		return SubmissionDetail{}, fmt.Errorf("homework: bản nộp %s vắng trong lịch sử của thành viên", submissionID)
	}
	out.LatestSubmissionID = history[0].ID
	out.IsLatest = out.LatestSubmissionID == submissionID
	return out, nil
}

// GradeInput là form chấm như client gửi.
type GradeInput struct {
	Scores  []ScoreInput
	Comment string
}

// GradeOutcome là kết quả một lần chấm: lần chấm vừa ghi, trạng thái bản nộp sau chấm, có phải sửa điểm không và mục
// kế tiếp trong hàng chờ chấm (chỉ có ở lần chấm đầu; sửa điểm hoặc hết hàng chờ là nil).
type GradeOutcome struct {
	Grade            GradeRecord
	SubmissionStatus SubmissionStatus
	Updated          bool
	Next             *grading.QueueItem
}

// Grade chấm hoặc sửa điểm bản nộp mới nhất: ghi lần chấm mới (lịch sử giữ nguyên), đặt trạng thái bản nộp theo
// kết quả, ghi hoặc xóa mốc hoàn thành học liệu, audit và đưa email kết quả vào outbox, tất cả trong một transaction.
func (s *Service) Grade(ctx context.Context, actor Actor, submissionID uuid.UUID, in GradeInput) (GradeOutcome, error) {
	var out GradeOutcome
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		sc, err := s.repo.SubmissionContext(ctx, tx, submissionID)
		if err != nil {
			return err
		}
		if err := staffCanAccessClass(actor, sc.Class.TeacherID, ErrNotOwnGrade); err != nil {
			return err
		}
		if _, err := s.repo.LockMember(ctx, tx, sc.MemberID); err != nil {
			return err
		}
		status, err := s.repo.LockSubmission(ctx, tx, submissionID)
		if err != nil {
			return err
		}
		latest, err := s.repo.Latest(ctx, tx, sc.MemberID, sc.LessonID)
		if err != nil {
			return err
		}
		if status == StatusSuperseded || latest == nil || latest.ID != submissionID {
			return ErrNotLatest
		}
		row, content, err := s.lessonOf(ctx, tx, sc.Class.CourseVersionID, sc.LessonID)
		if err != nil {
			return err
		}
		criteria, _ := criteriaOf(content)
		g, err := NewGrade(criteria, content.PassPercent, in.Scores, in.Comment)
		if err != nil {
			return err
		}
		history, err := s.repo.History(ctx, tx, sc.MemberID, sc.LessonID)
		if err != nil {
			return err
		}
		previous := history[0].Current()
		now := s.clock.Now()
		gradeID := s.ids.New()
		if err := s.repo.InsertGrade(ctx, tx, NewGradeRow{ID: gradeID, SubmissionID: submissionID, GradedBy: actor.ID, Grade: g, GradedAt: now}); err != nil {
			return err
		}
		if err := s.repo.SetResult(ctx, tx, submissionID, g.Result); err != nil {
			return err
		}
		if g.Result == ResultPassed {
			err = s.completion.MarkCompleted(ctx, tx, sc.MemberID, sc.LessonID, now)
		} else {
			err = s.completion.ClearCompleted(ctx, tx, sc.MemberID, sc.LessonID, now)
		}
		if err != nil {
			return err
		}
		if err := s.recordGrade(ctx, tx, actor, sc, row.Title, previous, g); err != nil {
			return err
		}
		if err := s.enqueueResult(ctx, tx, sc, row.Title, g, previous != nil); err != nil {
			return err
		}
		// Đọc lại lần chấm vừa ghi để trả cùng dạng với lịch sử (tên người chấm, mô tả tiêu chí).
		after, err := s.repo.History(ctx, tx, sc.MemberID, sc.LessonID)
		if err != nil {
			return err
		}
		current := after[0].Current()
		if current == nil || current.ID != gradeID {
			return fmt.Errorf("homework: lần chấm %s vắng sau khi ghi", gradeID)
		}
		out = GradeOutcome{Grade: *current, SubmissionStatus: SubmissionStatus(g.Result), Updated: previous != nil}
		// Mục kế tiếp đọc trong cùng transaction nên bản vừa chấm đã rời hàng chờ; sửa điểm không dẫn tiếp.
		if !out.Updated {
			key := grading.QueueKey{Kind: grading.KindHomework, ItemID: submissionID}
			if out.Next, err = s.next.Next(ctx, tx, grading.Actor{ID: actor.ID, Role: actor.Role}, key); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return GradeOutcome{}, err
	}
	return out, nil
}

func (s *Service) recordGrade(ctx context.Context, tx db.Executor, actor Actor, sc SubmissionContext, lessonTitle string, previous *GradeRecord, g Grade) error {
	base := func() map[string]any {
		return map[string]any{
			"classCode": sc.Class.Code, "lessonId": sc.LessonID, "lessonTitle": lessonTitle, "memberId": sc.MemberID,
			"studentName": sc.StudentName, "submissionId": sc.ID, "attemptNo": sc.AttemptNo,
		}
	}
	after := base()
	scores := make([]map[string]any, len(g.Scores))
	for i, sc := range g.Scores {
		scores[i] = map[string]any{"criterionId": sc.CriterionID, "points": sc.Points}
	}
	after["totalPoints"], after["maxPoints"], after["result"], after["scores"] = g.TotalPoints, g.MaxPoints, g.Result, scores
	if previous == nil {
		return s.record(ctx, tx, actor, audit.ActionHomeworkGraded, sc.Class.ID, nil, after)
	}
	before := base()
	old := make([]map[string]any, len(previous.Scores))
	for i, sc := range previous.Scores {
		old[i] = map[string]any{"criterionId": sc.CriterionID, "points": sc.Points}
	}
	before["totalPoints"], before["maxPoints"], before["result"], before["scores"] = previous.TotalPoints, previous.MaxPoints, previous.Result, old
	return s.record(ctx, tx, actor, audit.ActionHomeworkGradeUpdated, sc.Class.ID, before, after)
}

// enqueueResult đưa email kết quả vào outbox; link trang học liệu ở khóa ResultURL (mailer từ chối khóa Link).
func (s *Service) enqueueResult(ctx context.Context, tx db.Executor, sc SubmissionContext, lessonTitle string, g Grade, updated bool) error {
	to, err := domain.ParseEmail(sc.StudentEmail)
	if err != nil {
		return fmt.Errorf("homework: email học viên %s: %w", sc.UserID, err)
	}
	payload := map[string]any{
		"Name": sc.StudentName, "ClassName": sc.Class.Name, "ClassCode": sc.Class.Code, "LessonTitle": lessonTitle,
		"TotalPoints": g.TotalPoints, "MaxPoints": g.MaxPoints, "ResultLabel": g.Result.Label(), "Comment": g.Comment,
		"Updated":   updated,
		"ResultURL": fmt.Sprintf("%s/learn/classes/%s/lessons/%s", s.publicBaseURL, sc.Class.ID, sc.LessonID),
	}
	_, err = s.mail.Enqueue(ctx, tx, mailer.Message{To: to, Template: mailer.TemplateHomeworkGraded, Payload: payload})
	return err
}

func (s *Service) record(ctx context.Context, tx db.Executor, actor Actor, action string, classID uuid.UUID, before, after any) error {
	actorID := actor.ID
	return s.audit.Record(ctx, tx, audit.Entry{
		ActorID: &actorID, Action: action, TargetType: "class", TargetID: &classID, Before: before, After: after, RequestID: actor.RequestID,
	})
}

// ---- Tổng quan homework của lớp ----

// ClassHomework là mọi homework của phiên bản khóa học của lớp, mỗi bài kèm thống kê và trạng thái từng thành viên
// đang học.
type ClassHomework struct {
	Class ClassInfo
	Items []HomeworkItem
}

// HomeworkItem là một homework trong tổng quan lớp.
type HomeworkItem struct {
	Lesson  HomeworkLesson
	Stats   HomeworkStats
	Members []MemberHomework
}

// HomeworkStats đếm theo bản nộp mới nhất của thành viên đang học; OverdueNotSubmitted là số thành viên chưa nộp
// khi đã quá hạn.
type HomeworkStats struct {
	Submitted           int
	Pending             int
	Passed              int
	Rework              int
	OverdueNotSubmitted int
}

// MemberHomework là trạng thái một thành viên với một homework; WaitingDays là số ngày bản đang chờ chấm đã chờ.
type MemberHomework struct {
	MemberID    uuid.UUID
	Name        string
	Status      string
	Latest      *LatestRow
	WaitingDays *int
}

// ClassHomework đọc tổng quan homework của lớp bằng ba truy vấn tập hợp; chỉ staff phụ trách.
func (s *Service) ClassHomework(ctx context.Context, actor Actor, classID uuid.UUID) (ClassHomework, error) {
	cls, err := s.repo.Class(ctx, s.db, classID)
	if err != nil {
		return ClassHomework{}, err
	}
	if err := staffCanAccessClass(actor, cls.TeacherID, classes.ErrNotOwnClass); err != nil {
		return ClassHomework{}, err
	}
	lessons, err := s.repo.HomeworkLessons(ctx, s.db, classID, cls.CourseVersionID)
	if err != nil {
		return ClassHomework{}, err
	}
	members, err := s.repo.ActiveMembers(ctx, s.db, classID)
	if err != nil {
		return ClassHomework{}, err
	}
	rows, err := s.repo.LatestByClass(ctx, s.db, classID)
	if err != nil {
		return ClassHomework{}, err
	}
	type key struct{ member, lesson uuid.UUID }
	latest := make(map[key]LatestRow, len(rows))
	for _, r := range rows {
		latest[key{r.MemberID, r.LessonID}] = r
	}
	now := s.clock.Now()
	out := ClassHomework{Class: cls, Items: make([]HomeworkItem, len(lessons))}
	for i, l := range lessons {
		item := HomeworkItem{Lesson: l, Members: make([]MemberHomework, len(members))}
		overdue := l.DueAt != nil && now.After(*l.DueAt)
		for j, m := range members {
			mh := MemberHomework{MemberID: m.MemberID, Name: m.Name, Status: statusNotSubmitted}
			if r, ok := latest[key{m.MemberID, l.LessonID}]; ok {
				mh.Latest, mh.Status = &r, string(r.Status)
				item.Stats.Submitted++
				switch r.Status {
				case StatusPending:
					item.Stats.Pending++
					days := int(now.Sub(r.SubmittedAt) / (24 * time.Hour))
					mh.WaitingDays = &days
				case StatusPassed:
					item.Stats.Passed++
				case StatusRework:
					item.Stats.Rework++
				}
			} else if overdue {
				item.Stats.OverdueNotSubmitted++
			}
			item.Members[j] = mh
		}
		out.Items[i] = item
	}
	return out, nil
}
