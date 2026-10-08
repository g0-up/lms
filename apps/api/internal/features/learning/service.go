package learning

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/media"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
)

// Service là use case học của học viên. Quyền xét bằng LearningContext dựng từ classes.MembershipReader; học viên
// chỉ có một vai trò nên mọi use case nhận id học viên đã qua RequireRole(student).
type Service struct {
	db        db.Executor
	tx        db.Tx
	progress  ProgressRepo
	structure CourseStructureRepo
	myClasses MyClassesRepo
	members   classes.MembershipReader
	media     media.URLSigner
	clock     clock.Clock
}

// Deps là phụ thuộc của Service; DB dùng cho truy vấn đọc và câu ghi đơn ngoài transaction.
type Deps struct {
	DB        db.Executor
	Tx        db.Tx
	Progress  ProgressRepo
	Structure CourseStructureRepo
	MyClasses MyClassesRepo
	Members   classes.MembershipReader
	Media     media.URLSigner
	Clock     clock.Clock
}

// NewService nối phụ thuộc.
func NewService(d Deps) *Service {
	return &Service{
		db: d.DB, tx: d.Tx, progress: d.Progress, structure: d.Structure, myClasses: d.MyClasses,
		members: d.Members, media: d.Media, clock: d.Clock,
	}
}

// LessonContentView là nội dung trả cho trang học: video có URL ký (MediaID để ký lại qua /media/{id}/url),
// markdown có HTML đã sanitize lúc phát hành.
type LessonContentView struct {
	Type      stages.LessonType
	MediaID   uuid.UUID
	URL       string
	ExpiresAt time.Time
	HTML      string
}

// LessonPage là trang học một học liệu: học liệu kèm trạng thái của học viên, chặng chứa nó, nội dung và học liệu
// liền trước/sau theo thứ tự toàn khóa.
type LessonPage struct {
	Lesson   LessonView
	Stage    StageView
	Content  LessonContentView
	ReadOnly bool
	Prev     *LessonRef
	Next     *LessonRef
}

// Completion là kết quả tích/bỏ tích: completedAt sau thay đổi và tiến độ toàn khóa.
type Completion struct {
	CompletedAt   *time.Time
	Percent       int
	RequiredDone  int
	RequiredTotal int
}

// MyClasses liệt kê lớp học viên đang học (gồm lớp nháp và đã kết thúc, không gồm lớp đã rời).
func (s *Service) MyClasses(ctx context.Context, studentID uuid.UUID) ([]MyClassRow, error) {
	return s.myClasses.ListForStudent(ctx, s.db, studentID)
}

// Roadmap là lộ trình của học viên trong lớp; lớp nháp hoặc đã kết thúc trả lộ trình chỉ đọc.
func (s *Service) Roadmap(ctx context.Context, studentID, classID uuid.UUID) (Roadmap, error) {
	lc, err := s.learningContext(ctx, s.db, studentID, classID)
	if err != nil {
		return Roadmap{}, err
	}
	if err := lc.CanView(); err != nil {
		return Roadmap{}, err
	}
	cls, err := s.myClasses.ClassSummary(ctx, s.db, classID)
	if err != nil {
		return Roadmap{}, err
	}
	return s.roadmapOf(ctx, s.db, cls, lc)
}

// OpenLesson trả nội dung học liệu và ghi lần mở đầu tiên khi lớp đang chạy (lần sau không đổi first_opened_at);
// lớp đã kết thúc xem được mà không ghi, lớp nháp chưa cấp nội dung.
func (s *Service) OpenLesson(ctx context.Context, studentID, classID, lessonID uuid.UUID) (LessonPage, error) {
	lc, err := s.learningContext(ctx, s.db, studentID, classID)
	if err != nil {
		return LessonPage{}, err
	}
	if err := lc.CanOpen(); err != nil {
		return LessonPage{}, err
	}
	row, err := s.lessonOf(ctx, lc, lessonID)
	if err != nil {
		return LessonPage{}, err
	}
	content, err := s.contentOf(ctx, studentID, row)
	if err != nil {
		return LessonPage{}, err
	}
	if lc.CanRecord() == nil {
		if _, err := s.progress.Open(ctx, s.db, OpenLesson(lc.Member.MemberID, lessonID, s.now())); err != nil {
			return LessonPage{}, err
		}
	}
	r, err := s.roadmapOf(ctx, s.db, ClassSummary{ID: classID, Status: lc.Member.ClassStatus}, lc)
	if err != nil {
		return LessonPage{}, err
	}
	lesson, stage, prev, next, ok := r.Locate(lessonID)
	if !ok {
		return LessonPage{}, fmt.Errorf("learning: học liệu %s vắng trong lộ trình lớp %s", lessonID, classID)
	}
	return LessonPage{Lesson: lesson, Stage: stage, Content: content, ReadOnly: r.ReadOnly, Prev: prev, Next: next}, nil
}

// SetCompletion tích hoặc bỏ tích hoàn thành, idempotent; chỉ khi lớp đang chạy và học liệu đã mở.
func (s *Service) SetCompletion(ctx context.Context, studentID, classID, lessonID uuid.UUID, completed bool) (Completion, error) {
	lc, err := s.learningContext(ctx, s.db, studentID, classID)
	if err != nil {
		return Completion{}, err
	}
	if err := lc.CanRecord(); err != nil {
		return Completion{}, err
	}
	if _, err := s.lessonOf(ctx, lc, lessonID); err != nil {
		return Completion{}, err
	}
	var out Completion
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		p, err := s.progress.GetForUpdate(ctx, tx, lc.Member.MemberID, lessonID)
		if err != nil {
			return err
		}
		if p == nil {
			return ErrNotOpened
		}
		now := s.now()
		if p.SetCompleted(completed, now) {
			if err := s.progress.SetCompleted(ctx, tx, p, now); err != nil {
				return err
			}
		}
		r, err := s.roadmapOf(ctx, tx, ClassSummary{ID: classID, Status: lc.Member.ClassStatus}, lc)
		if err != nil {
			return err
		}
		out = Completion{CompletedAt: p.CompletedAt(), Percent: r.Percent, RequiredDone: r.RequiredDone, RequiredTotal: r.RequiredTotal}
		return nil
	})
	if err != nil {
		return Completion{}, err
	}
	return out, nil
}

// learningContext đọc ngữ cảnh học viên trong lớp; lớp không tồn tại cũng là ErrNotFound để không lộ lớp.
func (s *Service) learningContext(ctx context.Context, ex db.Executor, studentID, classID uuid.UUID) (LearningContext, error) {
	mc, err := s.members.MemberContext(ctx, ex, classID, studentID)
	if errors.Is(err, classes.ErrClassNotFound) {
		return LearningContext{}, ErrNotFound
	}
	if err != nil {
		return LearningContext{}, err
	}
	return LearningContext{Member: mc}, nil
}

// lessonOf kiểm học liệu thuộc phiên bản khóa học của lớp (không chỉ tồn tại) để không ghi tiến độ chéo lớp.
func (s *Service) lessonOf(ctx context.Context, lc LearningContext, lessonID uuid.UUID) (*LessonRow, error) {
	row, err := s.structure.LessonInCourseVersion(ctx, s.db, lc.Member.CourseVersionID, lessonID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrLessonNotInCourse
	}
	return row, nil
}

// contentOf dựng nội dung theo loại học liệu: video ký URL qua media (cùng kiểm quyền với /media/{id}/url),
// markdown trả HTML đã sanitize.
func (s *Service) contentOf(ctx context.Context, studentID uuid.UUID, row *LessonRow) (LessonContentView, error) {
	switch c := row.Content.(type) {
	case stages.VideoContent:
		u, err := s.media.SignedURL(ctx, media.Principal{ID: studentID, Role: domain.RoleStudent}, c.MediaID)
		if err != nil {
			return LessonContentView{}, err
		}
		return LessonContentView{Type: stages.LessonVideo, MediaID: c.MediaID, URL: u.URL, ExpiresAt: u.ExpiresAt}, nil
	case stages.MarkdownContent:
		if c.HTML == nil {
			return LessonContentView{}, fmt.Errorf("learning: học liệu %s chưa có HTML đã phát hành", row.ID)
		}
		return LessonContentView{Type: stages.LessonMarkdown, HTML: *c.HTML}, nil
	default:
		return LessonContentView{}, fmt.Errorf("learning: học liệu %s có nội dung %T không hỗ trợ", row.ID, row.Content)
	}
}

func (s *Service) roadmapOf(ctx context.Context, ex db.Executor, cls ClassSummary, lc LearningContext) (Roadmap, error) {
	structure, err := s.structure.StagesOfCourseVersion(ctx, ex, lc.Member.CourseVersionID)
	if err != nil {
		return Roadmap{}, err
	}
	progress, err := s.progress.ByMember(ctx, ex, lc.Member.MemberID)
	if err != nil {
		return Roadmap{}, err
	}
	return BuildRoadmap(cls, structure, progress), nil
}

// now cắt về micro giây (độ chính xác timestamptz) để giá trị trả về trùng giá trị đọc lại từ DB.
func (s *Service) now() time.Time { return s.clock.Now().UTC().Truncate(time.Microsecond) }
