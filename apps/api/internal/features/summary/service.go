package summary

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
)

// StatusReader trả trạng thái từng học liệu của một thành viên, khóa theo lesson id. learning gọi trong executor
// của nó để hiện nhãn trạng thái quiz và homework trên lộ trình.
type StatusReader interface {
	ForMember(ctx context.Context, ex db.Executor, classID, memberID, courseVersionID uuid.UUID, now time.Time) (map[uuid.UUID]ItemStatus, error)
}

// Service là use case tổng kết: đọc trong transaction chỉ đọc để mọi bảng cùng một snapshot, ghi nhận xét trong
// transaction riêng (khóa class_members trước).
type Service struct {
	read    db.Tx
	tx      db.Tx
	repo    Repo
	members classes.MembershipReader
	audit   audit.Recorder
	clock   clock.Clock
	ids     ids.Generator
}

var _ StatusReader = (*Service)(nil)

// Deps là phụ thuộc của Service. Read nên là transaction chỉ đọc REPEATABLE READ; Tx là transaction ghi.
type Deps struct {
	Read    db.Tx
	Tx      db.Tx
	Repo    Repo
	Members classes.MembershipReader
	Audit   audit.Recorder
	Clock   clock.Clock
	IDs     ids.Generator
}

// NewService nối phụ thuộc.
func NewService(d Deps) *Service {
	return &Service{read: d.Read, tx: d.Tx, repo: d.Repo, members: d.Members, audit: d.Audit, clock: d.Clock, ids: d.IDs}
}

// ForMember tính trạng thái mọi học liệu của thành viên memberID trong lớp classID.
func (s *Service) ForMember(ctx context.Context, ex db.Executor, classID, memberID, courseVersionID uuid.UUID, now time.Time) (map[uuid.UUID]ItemStatus, error) {
	lessons, err := s.repo.Lessons(ctx, ex, courseVersionID)
	if err != nil {
		return nil, err
	}
	act, err := s.repo.Activity(ctx, ex, classID, &memberID)
	if err != nil {
		return nil, err
	}
	facts := BuildFacts(lessons, []uuid.UUID{memberID}, act)[memberID]
	out := make(map[uuid.UUID]ItemStatus, len(facts))
	for _, f := range facts {
		out[f.Lesson.ID] = Status(f, now)
	}
	return out, nil
}

// LearnerSummary là tổng kết khóa của học viên studentID trong lớp classID, gồm nhận xét chặng của giảng viên. Mọi
// trạng thái lớp đều xem được; học viên không thuộc lớp hoặc đã rời lớp bị từ chối.
func (s *Service) LearnerSummary(ctx context.Context, studentID, classID uuid.UUID) (LearnerSummary, error) {
	var out LearnerSummary
	err := s.read.Transact(ctx, func(tx db.Executor) error {
		mc, err := s.members.MemberContext(ctx, tx, classID, studentID)
		if err != nil {
			return err
		}
		switch {
		case !mc.IsMember():
			return ErrNotMember
		case !mc.IsActiveMember():
			return ErrMemberDropped
		}
		cls, err := s.repo.Class(ctx, tx, classID)
		if err != nil {
			return err
		}
		stageRefs, facts, reviews, err := s.memberData(ctx, tx, cls, mc.MemberID)
		if err != nil {
			return err
		}
		sums, done, total := BuildCourse(stageRefs, facts, reviewsByStage(reviews), s.clock.Now())
		out = LearnerSummary{
			Class: cls, Stages: sums, CoursePercent: domain.Percent(done, total), RequiredDone: done, RequiredTotal: total,
			Course: Course(sums),
		}
		return nil
	})
	if err != nil {
		return LearnerSummary{}, err
	}
	return out, nil
}

// Course dựng tổng kết toàn khóa từ tổng kết các chặng, không đọc thêm dữ liệu.
func Course(sums []StageSummary) CourseSummary {
	out := CourseSummary{StagesTotal: len(sums), Stages: make([]CourseStage, len(sums))}
	for i, s := range sums {
		cs := CourseStage{
			StageVersionID: s.Stage.StageVersionID, Name: s.Stage.Name, Complete: s.Complete, MissingCount: len(s.Missing),
			Assessments: []AssessmentScore{}, Homework: []HomeworkScore{},
		}
		for _, u := range s.Units {
			for _, it := range u.Items {
				l := it.Facts.Lesson
				switch {
				case l.Type == stages.LessonQuiz && l.QuizMode == stages.QuizAssessment:
					cs.Assessments = append(cs.Assessments, assessmentScore(it))
				case l.Type == stages.LessonHomework:
					cs.Homework = append(cs.Homework, homeworkScore(it))
				}
			}
		}
		if s.Complete {
			out.StagesComplete++
		}
		out.Stages[i] = cs
	}
	return out
}

func assessmentScore(it Item) AssessmentScore {
	out := AssessmentScore{LessonID: it.Facts.Lesson.ID, Title: it.Facts.Lesson.Title, Passed: it.Status.Done}
	if q := it.Facts.Quiz; q != nil && q.Best != nil {
		p := q.Best.Percent()
		out.BestPercent = &p
	}
	return out
}

func homeworkScore(it Item) HomeworkScore {
	out := HomeworkScore{LessonID: it.Facts.Lesson.ID, Title: it.Facts.Lesson.Title, Status: HomeworkNotSubmitted}
	h := it.Facts.Homework
	if h == nil || h.Latest == nil {
		return out
	}
	out.Total, out.Max = h.Latest.Total, h.Latest.Max
	switch h.Latest.Status {
	case SubmissionPassed:
		out.Status = HomeworkPassed
	case SubmissionRework:
		out.Status = HomeworkRework
	default:
		out.Status = HomeworkPending
	}
	return out
}

// ClassMatrix là ma trận học viên đang học × chặng của lớp cho giảng viên phụ trách hoặc admin. Số truy vấn cố
// định, không tăng theo số học viên.
func (s *Service) ClassMatrix(ctx context.Context, actor Actor, classID uuid.UUID) (ClassMatrix, error) {
	var out ClassMatrix
	err := s.read.Transact(ctx, func(tx db.Executor) error {
		cls, err := s.repo.Class(ctx, tx, classID)
		if err != nil {
			return err
		}
		if err := staffCanAccessClass(actor, cls.TeacherID, classes.ErrNotOwnClass); err != nil {
			return err
		}
		stageRefs, err := s.repo.Stages(ctx, tx, cls.CourseVersionID)
		if err != nil {
			return err
		}
		lessons, err := s.repo.Lessons(ctx, tx, cls.CourseVersionID)
		if err != nil {
			return err
		}
		members, err := s.repo.ActiveMembers(ctx, tx, classID)
		if err != nil {
			return err
		}
		act, err := s.repo.Activity(ctx, tx, classID, nil)
		if err != nil {
			return err
		}
		reviews, err := s.repo.Reviews(ctx, tx, classID, nil)
		if err != nil {
			return err
		}
		out = buildMatrix(cls, stageRefs, lessons, members, act, reviews, s.clock.Now())
		return nil
	})
	if err != nil {
		return ClassMatrix{}, err
	}
	return out, nil
}

func buildMatrix(cls ClassInfo, stageRefs []StageRef, lessons []Lesson, members []Member, act Activity, reviews []Review, now time.Time) ClassMatrix {
	memberIDs := make([]uuid.UUID, len(members))
	for i, m := range members {
		memberIDs[i] = m.ID
	}
	facts := BuildFacts(lessons, memberIDs, act)
	type memberStage struct{ member, stage uuid.UUID }
	reviewed := make(map[memberStage]bool, len(reviews))
	for _, r := range reviews {
		reviewed[memberStage{r.MemberID, r.StageVersionID}] = true
	}
	out := ClassMatrix{Class: cls, Stages: stageRefs, Rows: make([]MatrixRow, len(members))}
	for i, m := range members {
		sums, done, total := BuildCourse(stageRefs, facts[m.ID], nil, now)
		row := MatrixRow{Member: m, CoursePercent: domain.Percent(done, total), Cells: make([]MatrixCell, len(sums))}
		for j, st := range sums {
			row.Cells[j] = MatrixCell{
				StageVersionID: st.Stage.StageVersionID, Complete: st.Complete, Done: st.Done, Required: st.Required,
				Missing: len(st.Missing), Rework: st.Rework, Overdue: st.Overdue,
				HasReview: reviewed[memberStage{m.ID, st.Stage.StageVersionID}],
			}
		}
		out.Rows[i] = row
	}
	return out
}

// MemberStage là tổng kết chặng stageVersionID của thành viên memberID kèm nhận xét, cho giảng viên phụ trách hoặc
// admin. Thành viên phải thuộc lớp classID và chặng phải thuộc phiên bản khóa học của lớp, sai thì NOT_FOUND.
func (s *Service) MemberStage(ctx context.Context, actor Actor, classID, memberID, stageVersionID uuid.UUID) (MemberStage, error) {
	var out MemberStage
	err := s.read.Transact(ctx, func(tx db.Executor) error {
		cls, err := s.repo.Class(ctx, tx, classID)
		if err != nil {
			return err
		}
		if err := staffCanAccessClass(actor, cls.TeacherID, classes.ErrNotOwnClass); err != nil {
			return err
		}
		m, err := s.repo.Member(ctx, tx, classID, memberID)
		if err != nil {
			return err
		}
		stageRefs, facts, reviews, err := s.memberData(ctx, tx, cls, m.ID)
		if err != nil {
			return err
		}
		stage, ok := findStage(stageRefs, stageVersionID)
		if !ok {
			return ErrStageNotInClass
		}
		var inStage []ItemFacts
		for _, f := range facts {
			if f.Lesson.StageVersionID == stageVersionID {
				inStage = append(inStage, f)
			}
		}
		out = MemberStage{
			Class: cls, Member: m,
			Summary: BuildStage(stage, inStage, reviewsByStage(reviews)[stageVersionID], s.clock.Now()),
		}
		return nil
	})
	if err != nil {
		return MemberStage{}, err
	}
	return out, nil
}

// memberData nạp chặng, dữ liệu thô mọi học liệu và nhận xét của một thành viên.
func (s *Service) memberData(ctx context.Context, tx db.Executor, cls ClassInfo, memberID uuid.UUID) ([]StageRef, []ItemFacts, []Review, error) {
	stageRefs, err := s.repo.Stages(ctx, tx, cls.CourseVersionID)
	if err != nil {
		return nil, nil, nil, err
	}
	lessons, err := s.repo.Lessons(ctx, tx, cls.CourseVersionID)
	if err != nil {
		return nil, nil, nil, err
	}
	act, err := s.repo.Activity(ctx, tx, cls.ID, &memberID)
	if err != nil {
		return nil, nil, nil, err
	}
	reviews, err := s.repo.Reviews(ctx, tx, cls.ID, &memberID)
	if err != nil {
		return nil, nil, nil, err
	}
	return stageRefs, BuildFacts(lessons, []uuid.UUID{memberID}, act)[memberID], reviews, nil
}

func reviewsByStage(reviews []Review) map[uuid.UUID]*Review {
	out := make(map[uuid.UUID]*Review, len(reviews))
	for i := range reviews {
		out[reviews[i].StageVersionID] = &reviews[i]
	}
	return out
}

func findStage(stageRefs []StageRef, stageVersionID uuid.UUID) (StageRef, bool) {
	for _, st := range stageRefs {
		if st.StageVersionID == stageVersionID {
			return st, true
		}
	}
	return StageRef{}, false
}

// NormalizeReview cắt khoảng trắng hai đầu và kiểm nhận xét: bắt buộc, tối đa maxReviewRunes ký tự.
func NormalizeReview(body string) (string, error) {
	body = strings.TrimSpace(body)
	switch {
	case body == "":
		return "", ErrReviewRequired
	case utf8.RuneCountInString(body) > maxReviewRunes:
		return "", ErrReviewTooLong
	}
	return body, nil
}

// reviewAudit là before/after của audit stage_review.saved: nhãn lớp, chặng và học viên để nhật ký tóm tắt
// "{stageName} · {studentName}", cùng nội dung nhận xét.
type reviewAudit struct {
	ClassCode      string    `json:"classCode"`
	StageVersionID uuid.UUID `json:"stageVersionId"`
	StageName      string    `json:"stageName"`
	MemberID       uuid.UUID `json:"memberId"`
	StudentName    string    `json:"studentName"`
	Body           string    `json:"body"`
	Revision       int       `json:"revision"`
	AuthorID       uuid.UUID `json:"authorId"`
}

// SaveReview tạo hoặc sửa nhận xét tổng kết chặng của thành viên memberID: lần đầu revision 1, mỗi lần sửa
// revision + 1 và người viết là người sửa. Phạm vi như MemberStage; audit stage_review.saved ghi cùng transaction,
// before là nội dung cũ khi sửa. Trạng thái chặng không chặn gì.
func (s *Service) SaveReview(ctx context.Context, actor Actor, classID, memberID, stageVersionID uuid.UUID, body string) (Review, error) {
	body, err := NormalizeReview(body)
	if err != nil {
		return Review{}, err
	}
	var out Review
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		cls, err := s.repo.Class(ctx, tx, classID)
		if err != nil {
			return err
		}
		if err := staffCanAccessClass(actor, cls.TeacherID, ErrNotOwnReview); err != nil {
			return err
		}
		m, err := s.repo.LockMember(ctx, tx, classID, memberID)
		if err != nil {
			return err
		}
		stageRefs, err := s.repo.Stages(ctx, tx, cls.CourseVersionID)
		if err != nil {
			return err
		}
		stage, ok := findStage(stageRefs, stageVersionID)
		if !ok {
			return ErrStageNotInClass
		}
		prev, err := s.repo.Review(ctx, tx, m.ID, stageVersionID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		r := Review{
			MemberID: m.ID, StageVersionID: stageVersionID, Body: body, AuthorID: actor.ID,
			Revision: 1, CreatedAt: now, UpdatedAt: now,
		}
		var before any
		if prev == nil {
			r.ID = s.ids.New()
			if err := s.repo.InsertReview(ctx, tx, r); err != nil {
				return err
			}
		} else {
			r.ID, r.CreatedAt, r.Revision = prev.ID, prev.CreatedAt, prev.Revision+1
			if err := s.repo.UpdateReview(ctx, tx, r, prev.Revision); err != nil {
				return err
			}
			before = reviewAudit{
				ClassCode: cls.Code, StageVersionID: stageVersionID, StageName: stage.Name, MemberID: m.ID,
				StudentName: m.Name, Body: prev.Body, Revision: prev.Revision, AuthorID: prev.AuthorID,
			}
		}
		after := reviewAudit{
			ClassCode: cls.Code, StageVersionID: stageVersionID, StageName: stage.Name, MemberID: m.ID,
			StudentName: m.Name, Body: r.Body, Revision: r.Revision, AuthorID: r.AuthorID,
		}
		actorID := actor.ID
		if err := s.audit.Record(ctx, tx, audit.Entry{
			ActorID: &actorID, Action: audit.ActionStageReviewSaved, TargetType: "class", TargetID: &cls.ID,
			Before: before, After: after, RequestID: actor.RequestID, At: now,
		}); err != nil {
			return err
		}
		saved, err := s.repo.Review(ctx, tx, m.ID, stageVersionID)
		if err != nil {
			return err
		}
		if saved == nil {
			return errors.New("summary: nhận xét vừa lưu không đọc lại được")
		}
		out = *saved
		return nil
	})
	if err != nil {
		return Review{}, err
	}
	return out, nil
}
