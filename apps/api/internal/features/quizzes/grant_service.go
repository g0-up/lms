package quizzes

import (
	"context"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/db"
)

// Grant cấp thêm lượt quiz bài tập cho thành viên memberID của lớp classID: chỉ admin hoặc giảng viên phụ trách, lớp
// đang chạy, thành viên đang học. Khóa class_members trước như bắt đầu và nộp lượt, nên cấp lượt và bắt đầu lượt
// của cùng thành viên chạy tuần tự và giới hạn lượt không bị vượt. Học viên đã đạt vẫn cấp được (ẩn nút chỉ là UX).
func (s *ReviewService) Grant(ctx context.Context, actor Actor, classID, memberID uuid.UUID, in GrantInput) (Grant, error) {
	var out Grant
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		cls, err := s.repo.Class(ctx, tx, classID)
		if err != nil {
			return err
		}
		if err := staffCanAccessClass(actor, cls.TeacherID, ErrNotOwnGrant); err != nil {
			return err
		}
		if cls.Status != domain.ClassActive {
			return ErrGrantClassNotActive
		}
		m, err := s.repo.LockClassMember(ctx, tx, classID, memberID)
		if err != nil {
			return err
		}
		switch {
		case m.ClassStatus != domain.ClassActive:
			return ErrGrantClassNotActive
		case m.Status != domain.MemberActive:
			return classes.ErrMemberAlreadyDropped
		}
		lesson, err := s.assessmentLesson(ctx, tx, cls.CourseVersionID, in.LessonID)
		if err != nil {
			return err
		}
		extra, reason, err := grantContent(in)
		if err != nil {
			return err
		}
		g, err := s.repo.InsertGrant(ctx, tx, NewGrant{
			ID: s.ids.New(), MemberID: m.ID, LessonID: lesson.LessonID, ExtraAttempts: extra, Reason: reason,
			GrantedBy: actor.ID, GrantedAt: s.clock.Now(),
		})
		if err != nil {
			return err
		}
		actorID := actor.ID
		if err := s.audit.Record(ctx, tx, audit.Entry{
			ActorID: &actorID, Action: audit.ActionQuizAttemptsGranted, TargetType: "class", TargetID: &cls.ID,
			After: map[string]any{
				"classCode": cls.Code, "memberId": m.ID, "studentName": m.Name, "lessonId": lesson.LessonID,
				"lessonTitle": lesson.Title, "extraAttempts": extra, "reason": reason,
			},
			RequestID: actor.RequestID,
		}); err != nil {
			return err
		}
		out = g
		return nil
	})
	if err != nil {
		return Grant{}, err
	}
	return out, nil
}

// assessmentLesson là quiz bài tập lessonID trong phiên bản khóa học của lớp; học liệu khác phiên bản, không phải
// quiz hoặc là quiz luyện tập → ErrGrantNotAssessment.
func (s *ReviewService) assessmentLesson(ctx context.Context, ex db.Executor, courseVersionID, lessonID uuid.UUID) (QuizLesson, error) {
	lessons, err := s.repo.QuizLessons(ctx, ex, courseVersionID)
	if err != nil {
		return QuizLesson{}, err
	}
	for _, l := range lessons {
		if l.LessonID == lessonID && l.Quiz.Mode == stages.QuizAssessment {
			return l, nil
		}
	}
	return QuizLesson{}, ErrGrantNotAssessment
}
