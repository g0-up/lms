package homework

import (
	"time"

	"github.com/google/uuid"

	"lms/api/internal/features/grading"
)

// deadlineRequest là body PUT /classes/:id/deadlines/:lid; dueAt theo RFC 3339 (có múi giờ).
type deadlineRequest struct {
	DueAt string `json:"dueAt" binding:"required"`
}

// submitRequest là body POST /me/classes/:id/lessons/:lid/submissions.
type submitRequest struct {
	RepoURL   string      `json:"repoUrl"`
	CommitRef string      `json:"commitRef"`
	MediaIDs  []uuid.UUID `json:"mediaIds"`
}

// scoreRequest là điểm một tiêu chí; points vắng là chưa nhập.
type scoreRequest struct {
	CriterionID uuid.UUID `json:"criterionId"`
	Points      *int      `json:"points"`
	Comment     string    `json:"comment"`
}

// gradeRequest là body POST /submissions/:sid/grades.
type gradeRequest struct {
	Scores  []scoreRequest `json:"scores"`
	Comment string         `json:"comment"`
}

func (r gradeRequest) input() GradeInput {
	scores := make([]ScoreInput, len(r.Scores))
	for i, s := range r.Scores {
		scores[i] = ScoreInput(s)
	}
	return GradeInput{Scores: scores, Comment: r.Comment}
}

// DeadlineDTO là hạn nộp sau khi đổi.
type DeadlineDTO struct {
	LessonID string     `json:"lessonId"`
	DueAt    *time.Time `json:"dueAt"`
}

func toDeadlineDTO(v DeadlineView) DeadlineDTO {
	return DeadlineDTO{LessonID: v.LessonID.String(), DueAt: utc(v.DueAt)}
}

// FileDTO là một file của bản nộp; tải qua GET /media/{mediaId}/url (luôn attachment).
type FileDTO struct {
	MediaID   string `json:"mediaId"`
	FileName  string `json:"fileName"`
	SizeBytes int64  `json:"sizeBytes"`
}

// ScoreDTO là điểm một tiêu chí của một lần chấm.
type ScoreDTO struct {
	CriterionID string `json:"criterionId"`
	Description string `json:"description"`
	Points      int    `json:"points"`
	MaxPoints   int    `json:"maxPoints"`
	Comment     string `json:"comment"`
}

// GradeDTO là một lần chấm.
type GradeDTO struct {
	ID          string     `json:"id"`
	TotalPoints int        `json:"totalPoints"`
	MaxPoints   int        `json:"maxPoints"`
	Result      string     `json:"result"`
	Comment     string     `json:"comment"`
	GradedBy    string     `json:"gradedBy"`
	GradedAt    time.Time  `json:"gradedAt"`
	Scores      []ScoreDTO `json:"scores"`
}

func toGradeDTO(g GradeRecord) GradeDTO {
	scores := make([]ScoreDTO, len(g.Scores))
	for i, s := range g.Scores {
		scores[i] = ScoreDTO{
			CriterionID: s.CriterionID.String(), Description: s.Description, Points: s.Points, MaxPoints: s.MaxPoints, Comment: s.Comment,
		}
	}
	return GradeDTO{
		ID: g.ID.String(), TotalPoints: g.TotalPoints, MaxPoints: g.MaxPoints, Result: string(g.Result), Comment: g.Comment,
		GradedBy: g.GradedBy, GradedAt: g.GradedAt.UTC(), Scores: scores,
	}
}

// SubmissionDTO là một bản nộp; grade là lần chấm hiện hành (null khi chưa chấm). grades (chỉ ở trang chấm) là mọi
// lần chấm, mới nhất trước.
type SubmissionDTO struct {
	ID          string     `json:"id"`
	No          int        `json:"no"`
	SubmittedAt time.Time  `json:"submittedAt"`
	IsLate      bool       `json:"isLate"`
	RepoURL     *string    `json:"repoUrl"`
	CommitRef   *string    `json:"commitRef"`
	Files       []FileDTO  `json:"files"`
	Status      string     `json:"status"`
	Grade       *GradeDTO  `json:"grade"`
	Grades      []GradeDTO `json:"grades,omitempty"`
}

func toSubmissionDTO(s SubmissionRecord, allGrades bool) SubmissionDTO {
	files := make([]FileDTO, len(s.Files))
	for i, f := range s.Files {
		files[i] = FileDTO{MediaID: f.MediaID.String(), FileName: f.FileName, SizeBytes: f.SizeBytes}
	}
	out := SubmissionDTO{
		ID: s.ID.String(), No: s.AttemptNo, SubmittedAt: s.SubmittedAt.UTC(), IsLate: s.IsLate, RepoURL: s.RepoURL,
		CommitRef: s.CommitRef, Files: files, Status: string(s.Status),
	}
	if g := s.Current(); g != nil {
		dto := toGradeDTO(*g)
		out.Grade = &dto
	}
	if allGrades {
		out.Grades = make([]GradeDTO, len(s.Grades))
		for i, g := range s.Grades {
			out.Grades[i] = toGradeDTO(g)
		}
	}
	return out
}

func toSubmissionDTOs(list []SubmissionRecord, allGrades bool) []SubmissionDTO {
	out := make([]SubmissionDTO, len(list))
	for i, s := range list {
		out[i] = toSubmissionDTO(s, allGrades)
	}
	return out
}

// MySubmissionsDTO là phản hồi GET/POST /me/classes/:id/lessons/:lid/submissions.
type MySubmissionsDTO struct {
	LessonID     string          `json:"lessonId"`
	Deadline     *time.Time      `json:"deadline"`
	Status       string          `json:"status"`
	CanSubmit    bool            `json:"canSubmit"`
	LockedReason *string         `json:"lockedReason"`
	MaxFiles     int             `json:"maxFiles"`
	Submissions  []SubmissionDTO `json:"submissions"`
}

func toMySubmissionsDTO(v MySubmissions) MySubmissionsDTO {
	out := MySubmissionsDTO{
		LessonID: v.LessonID.String(), Deadline: utc(v.Deadline), Status: v.Status, CanSubmit: v.CanSubmit,
		MaxFiles: v.MaxFiles, Submissions: toSubmissionDTOs(v.Submissions, false),
	}
	if v.LockedReason != "" {
		out.LockedReason = &v.LockedReason
	}
	return out
}

// CriterionDTO là một tiêu chí chấm.
type CriterionDTO struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	MaxPoints   int    `json:"maxPoints"`
}

// ClassRefDTO là lớp của bản nộp hoặc tổng quan.
type ClassRefDTO struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func toClassRefDTO(c ClassInfo) ClassRefDTO {
	return ClassRefDTO{ID: c.ID.String(), Code: c.Code, Name: c.Name, Status: string(c.Status)}
}

// StageRefDTO là chặng chứa homework.
type StageRefDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// StudentDTO là học viên của bản nộp.
type StudentDTO struct {
	MemberID     string `json:"memberId"`
	UserID       string `json:"userId"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	MemberStatus string `json:"memberStatus"`
}

// LessonDTO là đề homework để chấm.
type LessonDTO struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	BriefHTML   *string        `json:"briefHtml"`
	PassPercent int            `json:"passPercent"`
	MaxPoints   int            `json:"maxPoints"`
	Criteria    []CriterionDTO `json:"criteria"`
}

// SubmissionDetailDTO là phản hồi GET /submissions/:sid.
type SubmissionDetailDTO struct {
	Submission         SubmissionDTO   `json:"submission"`
	Student            StudentDTO      `json:"student"`
	Class              ClassRefDTO     `json:"class"`
	Stage              StageRefDTO     `json:"stage"`
	Lesson             LessonDTO       `json:"lesson"`
	Deadline           *time.Time      `json:"deadline"`
	IsLatest           bool            `json:"isLatest"`
	LatestSubmissionID string          `json:"latestSubmissionId"`
	History            []SubmissionDTO `json:"history"`
}

func toSubmissionDetailDTO(d SubmissionDetail) SubmissionDetailDTO {
	criteria := make([]CriterionDTO, len(d.Lesson.Criteria))
	for i, c := range d.Lesson.Criteria {
		criteria[i] = CriterionDTO{ID: c.ID.String(), Description: c.Description, MaxPoints: c.MaxPoints}
	}
	return SubmissionDetailDTO{
		Submission: toSubmissionDTO(d.Submission, true),
		Student: StudentDTO{
			MemberID: d.Student.MemberID.String(), UserID: d.Student.UserID.String(), Name: d.Student.Name, Email: d.Student.Email,
			MemberStatus: string(d.Student.MemberStatus),
		},
		Class: toClassRefDTO(d.Class),
		Stage: StageRefDTO{ID: d.StageID.String(), Name: d.StageName},
		Lesson: LessonDTO{
			ID: d.Lesson.ID.String(), Title: d.Lesson.Title, BriefHTML: d.Lesson.BriefHTML, PassPercent: d.Lesson.PassPercent,
			MaxPoints: d.Lesson.MaxPoints, Criteria: criteria,
		},
		Deadline: utc(d.Deadline), IsLatest: d.IsLatest, LatestSubmissionID: d.LatestSubmissionID.String(),
		History: toSubmissionDTOs(d.History, true),
	}
}

// GradeOutcomeDTO là phản hồi POST /submissions/:sid/grades. nextQueueItem là mục kế tiếp trong hàng chờ chấm sau
// lần chấm đầu; sửa điểm hoặc hết hàng chờ là null.
type GradeOutcomeDTO struct {
	Grade            GradeDTO              `json:"grade"`
	SubmissionStatus string                `json:"submissionStatus"`
	Updated          bool                  `json:"updated"`
	NextQueueItem    *grading.QueueItemDTO `json:"nextQueueItem"`
}

func toGradeOutcomeDTO(o GradeOutcome) GradeOutcomeDTO {
	out := GradeOutcomeDTO{Grade: toGradeDTO(o.Grade), SubmissionStatus: string(o.SubmissionStatus), Updated: o.Updated}
	if o.Next != nil {
		next := grading.ToQueueItemDTO(*o.Next)
		out.NextQueueItem = &next
	}
	return out
}

// LatestDTO là bản nộp mới nhất của thành viên trong tổng quan lớp; điểm vắng khi chưa chấm.
type LatestDTO struct {
	SubmissionID string    `json:"submissionId"`
	No           int       `json:"no"`
	SubmittedAt  time.Time `json:"submittedAt"`
	IsLate       bool      `json:"isLate"`
	HasRepo      bool      `json:"hasRepo"`
	FileCount    int       `json:"fileCount"`
	TotalPoints  *int      `json:"totalPoints"`
	MaxPoints    *int      `json:"maxPoints"`
}

// MemberHomeworkDTO là một thành viên trong tổng quan một homework.
type MemberHomeworkDTO struct {
	MemberID    string     `json:"memberId"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Latest      *LatestDTO `json:"latest"`
	WaitingDays *int       `json:"waitingDays"`
}

// StatsDTO là thống kê một homework của lớp.
type StatsDTO struct {
	Submitted           int `json:"submitted"`
	Pending             int `json:"pending"`
	Passed              int `json:"passed"`
	Rework              int `json:"rework"`
	OverdueNotSubmitted int `json:"overdueNotSubmitted"`
}

// LessonRefDTO là homework trong tổng quan lớp.
type LessonRefDTO struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// HomeworkItemDTO là một homework trong tổng quan lớp.
type HomeworkItemDTO struct {
	Lesson      LessonRefDTO        `json:"lesson"`
	Stage       StageRefDTO         `json:"stage"`
	MaxPoints   int                 `json:"maxPoints"`
	PassPercent int                 `json:"passPercent"`
	Deadline    *time.Time          `json:"deadline"`
	Stats       StatsDTO            `json:"stats"`
	Members     []MemberHomeworkDTO `json:"members"`
}

// ClassHomeworkDTO là phản hồi GET /classes/:id/homework.
type ClassHomeworkDTO struct {
	Class ClassRefDTO       `json:"class"`
	Items []HomeworkItemDTO `json:"items"`
}

func toClassHomeworkDTO(v ClassHomework) ClassHomeworkDTO {
	out := ClassHomeworkDTO{Class: toClassRefDTO(v.Class), Items: make([]HomeworkItemDTO, len(v.Items))}
	for i, it := range v.Items {
		members := make([]MemberHomeworkDTO, len(it.Members))
		for j, m := range it.Members {
			md := MemberHomeworkDTO{MemberID: m.MemberID.String(), Name: m.Name, Status: m.Status, WaitingDays: m.WaitingDays}
			if l := m.Latest; l != nil {
				md.Latest = &LatestDTO{
					SubmissionID: l.SubmissionID.String(), No: l.AttemptNo, SubmittedAt: l.SubmittedAt.UTC(), IsLate: l.IsLate,
					HasRepo: l.HasRepo, FileCount: l.FileCount, TotalPoints: l.TotalPoints, MaxPoints: l.MaxPoints,
				}
			}
			members[j] = md
		}
		out.Items[i] = HomeworkItemDTO{
			Lesson:    LessonRefDTO{ID: it.Lesson.LessonID.String(), Title: it.Lesson.Title},
			Stage:     StageRefDTO{ID: it.Lesson.StageID.String(), Name: it.Lesson.StageName},
			MaxPoints: it.Lesson.MaxPoints, PassPercent: it.Lesson.PassPercent, Deadline: utc(it.Lesson.DueAt),
			Stats: StatsDTO{
				Submitted: it.Stats.Submitted, Pending: it.Stats.Pending, Passed: it.Stats.Passed, Rework: it.Stats.Rework,
				OverdueNotSubmitted: it.Stats.OverdueNotSubmitted,
			},
			Members: members,
		}
	}
	return out
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
