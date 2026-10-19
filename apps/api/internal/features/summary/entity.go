// Package summary là read model tiến độ theo học viên: trạng thái từng học liệu, tổng kết chặng, tổng kết khóa và
// ma trận lớp, cùng nhận xét tổng kết chặng của giảng viên. Package chỉ đọc bảng bằng SQL riêng, không import
// homework, quizzes hay learning; learning gọi StatusReader để hiện nhãn trạng thái trên lộ trình. Hoàn thành chỉ
// lấy từ lesson_progress.completed_at nên % ở đây khớp learning và reports.
package summary

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
)

// maxReviewRunes là độ dài tối đa của nhận xét tổng kết chặng, như nhận xét chấm bài.
const maxReviewRunes = 5000

// Lỗi của feature, thông điệp theo prototype và spec.
var (
	ErrReviewRequired  = domain.ErrInvalid.WithMsg("Nhập nhận xét tổng kết chặng.")
	ErrReviewTooLong   = domain.ErrInvalid.WithMsg(fmt.Sprintf("Nhận xét tối đa %d ký tự.", maxReviewRunes))
	ErrStageNotInClass = domain.ErrNotFound.WithMsg("Chặng không thuộc khóa học của lớp này.")
	ErrNotMember       = domain.ErrForbidden.WithMsg("Bạn không học lớp này.")
	ErrMemberDropped   = domain.ErrForbidden.WithMsg("Bạn không còn học lớp này.")
	ErrNotOwnReview    = domain.ErrForbidden.WithMsg("Bạn chỉ nhận xét học viên của lớp mình phụ trách.")
	ErrConcurrentEdit  = domain.ErrConflict.WithMsg("Nhận xét vừa được người khác lưu. Tải lại rồi thử lại.")
)

// Tone là sắc thái hiển thị của nhãn trạng thái, cùng bộ giá trị với badge của web.
type Tone string

// Các sắc thái: xong, đang xử lý, cần chú ý, nguy cấp, chưa bắt đầu.
const (
	ToneOK       Tone = "ok"
	ToneInfo     Tone = "info"
	ToneWarn     Tone = "warn"
	ToneDanger   Tone = "danger"
	ToneOptional Tone = "optional"
)

// SubmissionStatus là trạng thái bài nộp homework, cùng giá trị cột submissions.status.
type SubmissionStatus string

// Các trạng thái bài nộp.
const (
	SubmissionPending    SubmissionStatus = "pending"
	SubmissionPassed     SubmissionStatus = "passed"
	SubmissionRework     SubmissionStatus = "rework"
	SubmissionSuperseded SubmissionStatus = "superseded"
)

// ReviewStatus là trạng thái xác nhận của lượt quiz, cùng giá trị cột quiz_attempts.review_status.
type ReviewStatus string

// Các trạng thái xác nhận lượt quiz.
const (
	ReviewPending    ReviewStatus = "pending"
	ReviewConfirmed  ReviewStatus = "confirmed"
	ReviewSuperseded ReviewStatus = "superseded"
)

// Label là nhãn trên giao diện theo thuật ngữ quiz của spec.
func (s ReviewStatus) Label() string {
	switch s {
	case ReviewPending:
		return "Chờ xác nhận"
	case ReviewConfirmed:
		return "Đã xác nhận"
	case ReviewSuperseded:
		return "Đã có lượt mới hơn"
	}
	return ""
}

// Actor là người gọi use case; RequestID ghi vào audit.
type Actor struct {
	ID        uuid.UUID
	Role      domain.Role
	RequestID string
}

// staffCanAccessClass: admin mọi lớp, giảng viên chỉ lớp mình phụ trách (denied khi khác lớp), vai trò khác không.
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

// ClassInfo là lớp cùng giảng viên phụ trách và phiên bản khóa học.
type ClassInfo struct {
	ID              uuid.UUID
	Code            string
	Name            string
	Status          domain.ClassStatus
	TeacherID       uuid.UUID
	CourseVersionID uuid.UUID
}

// Member là một thành viên của lớp cùng tên và email học viên.
type Member struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Name   string
	Email  string
	Status domain.MemberStatus
}

// StageRef là một chặng của phiên bản khóa học theo thứ tự course_version_stages.
type StageRef struct {
	StageVersionID uuid.UUID
	StageID        uuid.UUID
	Code           string
	Name           string
	VersionNo      domain.VersionNo
	Position       int
}

// Lesson là một học liệu của phiên bản khóa học cùng bài học chứa nó và cấu hình quiz (nếu là quiz).
type Lesson struct {
	ID             uuid.UUID
	StageVersionID uuid.UUID
	UnitID         uuid.UUID
	UnitTitle      string
	UnitPosition   int
	Title          string
	Type           stages.LessonType
	Position       int
	Required       bool
	QuizMode       stages.QuizMode
	MaxAttempts    *int
	PassPercent    *int
}

// Submission là bài nộp mới nhất của một (thành viên, homework) cùng lần chấm hiện hành (nếu đã chấm).
type Submission struct {
	AttemptNo   int
	Status      SubmissionStatus
	IsLate      bool
	SubmittedAt time.Time
	Total       *int
	Max         *int
}

// HomeworkFacts là dữ liệu thô homework của một thành viên: hạn nộp của lớp, số lần nộp và bản nộp mới nhất.
type HomeworkFacts struct {
	DueAt           *time.Time
	SubmissionCount int
	Latest          *Submission
}

// Score là điểm một lượt quiz đã nộp.
type Score struct {
	Score int
	Max   int
}

// Percent là phần trăm điểm qua domain.Percent, như quizzes.
func (s Score) Percent() int { return domain.Percent(s.Score, s.Max) }

// LatestAttempt là lượt quiz đã nộp gần nhất cùng trạng thái xác nhận (nil khi không cần xác nhận).
type LatestAttempt struct {
	Score
	ReviewStatus *ReviewStatus
}

// QuizFacts là dữ liệu thô quiz của một thành viên. AttemptsUsed đếm mọi lượt kể cả lượt đang làm (cùng cách
// quizzes tính lượt còn lại); Limit nil là không giới hạn (luyện tập); Best là lượt tỉ lệ điểm cao nhất, bằng nhau
// thì lượt sớm hơn.
type QuizFacts struct {
	Mode         stages.QuizMode
	PassPercent  *int
	Limit        *int
	AttemptsUsed int
	Submitted    int
	Open         bool
	Best         *Score
	Latest       *LatestAttempt
}

// ItemFacts là dữ liệu thô của một (thành viên, học liệu). Homework chỉ có ở học liệu homework, Quiz chỉ ở quiz.
type ItemFacts struct {
	Lesson        Lesson
	FirstOpenedAt *time.Time
	CompletedAt   *time.Time
	Homework      *HomeworkFacts
	Quiz          *QuizFacts
}

// ItemStatus là trạng thái hiển thị của một học liệu. Done chỉ lấy từ completed_at; Overdue và Rework là cờ suy
// ra cho bộ lọc ma trận, không lưu.
type ItemStatus struct {
	Done    bool
	Label   string
	Tone    Tone
	Detail  string
	Overdue bool
	Rework  bool
}

// Item là một học liệu trong tổng kết cùng trạng thái của nó.
type Item struct {
	Facts  ItemFacts
	Status ItemStatus
}

// UnitItems là một bài học trong tổng kết chặng cùng học liệu theo thứ tự.
type UnitItems struct {
	ID       uuid.UUID
	Title    string
	Position int
	Items    []Item
}

// MissingItem là học liệu bắt buộc chưa hoàn thành.
type MissingItem struct {
	LessonID  uuid.UUID
	Title     string
	UnitTitle string
}

// Review là nhận xét tổng kết chặng của giảng viên cho một thành viên. AuthorName là người lưu gần nhất; Revision là
// số lần lưu.
type Review struct {
	ID             uuid.UUID
	MemberID       uuid.UUID
	StageVersionID uuid.UUID
	Body           string
	AuthorID       uuid.UUID
	AuthorName     string
	Revision       int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// StageSummary là tổng kết một chặng của một thành viên. Complete khi chặng có học liệu bắt buộc và đã xong hết;
// chặng không có học liệu bắt buộc không bao giờ Complete (domain.Percent trả 0 khi tổng bằng 0, nên % và trạng thái
// không mâu thuẫn).
type StageSummary struct {
	Stage    StageRef
	Units    []UnitItems
	Required int
	Done     int
	Percent  int
	Complete bool
	Missing  []MissingItem
	Rework   bool
	Overdue  bool
	Review   *Review
}

// LearnerSummary là tổng kết khóa của học viên: mọi chặng cùng % toàn khóa theo học liệu bắt buộc, và bản gọn toàn
// khóa theo chặng.
type LearnerSummary struct {
	Class         ClassInfo
	Stages        []StageSummary
	CoursePercent int
	RequiredDone  int
	RequiredTotal int
	Course        CourseSummary
}

// CourseSummary là tổng kết toàn khóa: số chặng hoàn thành trên tổng số chặng cùng điểm từng chặng. Cố ý không có
// điểm tổng toàn khóa.
type CourseSummary struct {
	StagesComplete int
	StagesTotal    int
	Stages         []CourseStage
}

// CourseStage là một chặng trong tổng kết toàn khóa: số học liệu bắt buộc còn thiếu, điểm tốt nhất từng quiz bài
// tập và điểm bản nộp mới nhất từng homework, theo thứ tự học liệu.
type CourseStage struct {
	StageVersionID uuid.UUID
	Name           string
	Complete       bool
	MissingCount   int
	Assessments    []AssessmentScore
	Homework       []HomeworkScore
}

// AssessmentScore là điểm một quiz bài tập: BestPercent nil khi chưa nộp lượt nào; Passed theo hoàn thành của học
// liệu.
type AssessmentScore struct {
	LessonID    uuid.UUID
	Title       string
	BestPercent *int
	Passed      bool
}

// HomeworkStatus là trạng thái homework trong tổng kết toàn khóa: trạng thái bản nộp mới nhất, hoặc chưa nộp.
type HomeworkStatus string

// Các trạng thái homework của tổng kết toàn khóa.
const (
	HomeworkNotSubmitted HomeworkStatus = "not_submitted"
	HomeworkPending      HomeworkStatus = "pending"
	HomeworkPassed       HomeworkStatus = "passed"
	HomeworkRework       HomeworkStatus = "rework"
)

// HomeworkScore là điểm bản nộp mới nhất của một homework; Total và Max nil khi chưa chấm.
type HomeworkScore struct {
	LessonID uuid.UUID
	Title    string
	Total    *int
	Max      *int
	Status   HomeworkStatus
}

// MatrixCell là một ô (thành viên, chặng) của ma trận lớp; Missing là số học liệu bắt buộc còn thiếu.
type MatrixCell struct {
	StageVersionID uuid.UUID
	Complete       bool
	Done           int
	Required       int
	Missing        int
	Rework         bool
	Overdue        bool
	HasReview      bool
}

// MatrixRow là một thành viên đang học cùng % toàn khóa và một ô mỗi chặng.
type MatrixRow struct {
	Member        Member
	CoursePercent int
	Cells         []MatrixCell
}

// ClassMatrix là ma trận học viên × chặng của lớp. Bộ lọc FR-82 chạy ở client trên dữ liệu đầy đủ.
type ClassMatrix struct {
	Class  ClassInfo
	Stages []StageRef
	Rows   []MatrixRow
}

// MemberStage là tổng kết một chặng của một thành viên cho drawer của giảng viên.
type MemberStage struct {
	Class   ClassInfo
	Member  Member
	Summary StageSummary
}
