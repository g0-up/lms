package summary

import (
	"time"

	"github.com/google/uuid"
)

type classDTO struct {
	ID     uuid.UUID `json:"id"`
	Code   string    `json:"code"`
	Name   string    `json:"name"`
	Status string    `json:"status"`
}

func toClassDTO(c ClassInfo) classDTO {
	return classDTO{ID: c.ID, Code: c.Code, Name: c.Name, Status: string(c.Status)}
}

type stageRefDTO struct {
	StageVersionID uuid.UUID `json:"stageVersionId"`
	StageID        uuid.UUID `json:"stageId"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	VersionNo      int       `json:"versionNo"`
}

func toStageRefDTO(s StageRef) stageRefDTO {
	return stageRefDTO{StageVersionID: s.StageVersionID, StageID: s.StageID, Code: s.Code, Name: s.Name, VersionNo: int(s.VersionNo)}
}

type statusDTO struct {
	Done    bool   `json:"done"`
	Label   string `json:"label"`
	Tone    string `json:"tone"`
	Detail  string `json:"detail"`
	Overdue bool   `json:"overdue"`
	Rework  bool   `json:"rework"`
}

func toStatusDTO(s ItemStatus) statusDTO {
	return statusDTO{Done: s.Done, Label: s.Label, Tone: string(s.Tone), Detail: s.Detail, Overdue: s.Overdue, Rework: s.Rework}
}

type submissionDTO struct {
	AttemptNo   int       `json:"attemptNo"`
	Status      string    `json:"status"`
	IsLate      bool      `json:"isLate"`
	SubmittedAt time.Time `json:"submittedAt"`
	Total       *int      `json:"total"`
	Max         *int      `json:"max"`
}

type homeworkDTO struct {
	DueAt           *time.Time     `json:"dueAt"`
	SubmissionCount int            `json:"submissionCount"`
	Latest          *submissionDTO `json:"latest"`
}

type scoreDTO struct {
	Score   int `json:"score"`
	Max     int `json:"max"`
	Percent int `json:"percent"`
}

type latestAttemptDTO struct {
	scoreDTO
	ReviewStatus *string `json:"reviewStatus"`
}

type quizDTO struct {
	Mode         string            `json:"mode"`
	PassPercent  *int              `json:"passPercent"`
	Limit        *int              `json:"limit"`
	AttemptsUsed int               `json:"attemptsUsed"`
	Submitted    int               `json:"submitted"`
	Open         bool              `json:"open"`
	Best         *scoreDTO         `json:"best"`
	Latest       *latestAttemptDTO `json:"latest"`
}

// itemDTO là một học liệu trong tổng kết: dữ liệu thô theo loại (web dựng dòng chi tiết như lessonDetail của
// prototype) cùng trạng thái đã tính.
type itemDTO struct {
	LessonID      uuid.UUID    `json:"lessonId"`
	Title         string       `json:"title"`
	Type          string       `json:"type"`
	Required      bool         `json:"required"`
	FirstOpenedAt *time.Time   `json:"firstOpenedAt"`
	CompletedAt   *time.Time   `json:"completedAt"`
	Status        statusDTO    `json:"status"`
	Homework      *homeworkDTO `json:"homework,omitempty"`
	Quiz          *quizDTO     `json:"quiz,omitempty"`
}

func toScoreDTO(s Score) scoreDTO {
	return scoreDTO{Score: s.Score, Max: s.Max, Percent: s.Percent()}
}

func toItemDTO(it Item) itemDTO {
	f := it.Facts
	out := itemDTO{
		LessonID: f.Lesson.ID, Title: f.Lesson.Title, Type: string(f.Lesson.Type), Required: f.Lesson.Required,
		FirstOpenedAt: f.FirstOpenedAt, CompletedAt: f.CompletedAt, Status: toStatusDTO(it.Status),
	}
	if h := f.Homework; h != nil {
		d := &homeworkDTO{DueAt: h.DueAt, SubmissionCount: h.SubmissionCount}
		if l := h.Latest; l != nil {
			d.Latest = &submissionDTO{
				AttemptNo: l.AttemptNo, Status: string(l.Status), IsLate: l.IsLate, SubmittedAt: l.SubmittedAt, Total: l.Total, Max: l.Max,
			}
		}
		out.Homework = d
	}
	if q := f.Quiz; q != nil {
		d := &quizDTO{
			Mode: string(q.Mode), PassPercent: q.PassPercent, Limit: q.Limit, AttemptsUsed: q.AttemptsUsed,
			Submitted: q.Submitted, Open: q.Open,
		}
		if q.Best != nil {
			b := toScoreDTO(*q.Best)
			d.Best = &b
		}
		if q.Latest != nil {
			d.Latest = &latestAttemptDTO{scoreDTO: toScoreDTO(q.Latest.Score)}
			if q.Latest.ReviewStatus != nil {
				rs := string(*q.Latest.ReviewStatus)
				d.Latest.ReviewStatus = &rs
			}
		}
		out.Quiz = d
	}
	return out
}

type unitDTO struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	Position int       `json:"position"`
	Items    []itemDTO `json:"items"`
}

type missingDTO struct {
	LessonID  uuid.UUID `json:"lessonId"`
	Title     string    `json:"title"`
	UnitTitle string    `json:"unitTitle"`
}

type reviewDTO struct {
	Body       string    `json:"body"`
	AuthorName string    `json:"authorName"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Revision   int       `json:"revision"`
}

func toReviewDTO(r Review) reviewDTO {
	return reviewDTO{Body: r.Body, AuthorName: r.AuthorName, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Revision: r.Revision}
}

type stageSummaryDTO struct {
	stageRefDTO
	Units    []unitDTO    `json:"units"`
	Required int          `json:"required"`
	Done     int          `json:"done"`
	Percent  int          `json:"percent"`
	Complete bool         `json:"complete"`
	Missing  []missingDTO `json:"missing"`
	Rework   bool         `json:"rework"`
	Overdue  bool         `json:"overdue"`
	Review   *reviewDTO   `json:"review"`
}

func toStageSummaryDTO(s StageSummary) stageSummaryDTO {
	out := stageSummaryDTO{
		stageRefDTO: toStageRefDTO(s.Stage), Units: make([]unitDTO, len(s.Units)), Required: s.Required, Done: s.Done,
		Percent: s.Percent, Complete: s.Complete, Missing: make([]missingDTO, len(s.Missing)), Rework: s.Rework, Overdue: s.Overdue,
	}
	for i, u := range s.Units {
		items := make([]itemDTO, len(u.Items))
		for j, it := range u.Items {
			items[j] = toItemDTO(it)
		}
		out.Units[i] = unitDTO{ID: u.ID, Title: u.Title, Position: u.Position, Items: items}
	}
	for i, m := range s.Missing {
		out.Missing[i] = missingDTO(m)
	}
	if s.Review != nil {
		r := toReviewDTO(*s.Review)
		out.Review = &r
	}
	return out
}

type assessmentScoreDTO struct {
	LessonID    uuid.UUID `json:"lessonId"`
	Title       string    `json:"title"`
	BestPercent *int      `json:"bestPercent"`
	Passed      bool      `json:"passed"`
}

type homeworkScoreDTO struct {
	LessonID uuid.UUID `json:"lessonId"`
	Title    string    `json:"title"`
	Total    *int      `json:"total"`
	Max      *int      `json:"max"`
	Status   string    `json:"status"`
}

type courseStageDTO struct {
	StageVersionID uuid.UUID            `json:"stageVersionId"`
	Name           string               `json:"name"`
	Complete       bool                 `json:"complete"`
	MissingCount   int                  `json:"missingCount"`
	Assessments    []assessmentScoreDTO `json:"assessments"`
	Homework       []homeworkScoreDTO   `json:"homework"`
}

// courseSummaryDTO là tổng kết toàn khóa; không có trường điểm tổng nào.
type courseSummaryDTO struct {
	StagesComplete int              `json:"stagesComplete"`
	StagesTotal    int              `json:"stagesTotal"`
	Stages         []courseStageDTO `json:"stages"`
}

func toCourseSummaryDTO(c CourseSummary) courseSummaryDTO {
	out := courseSummaryDTO{StagesComplete: c.StagesComplete, StagesTotal: c.StagesTotal, Stages: make([]courseStageDTO, len(c.Stages))}
	for i, s := range c.Stages {
		cs := courseStageDTO{
			StageVersionID: s.StageVersionID, Name: s.Name, Complete: s.Complete, MissingCount: s.MissingCount,
			Assessments: make([]assessmentScoreDTO, len(s.Assessments)), Homework: make([]homeworkScoreDTO, len(s.Homework)),
		}
		for j, a := range s.Assessments {
			cs.Assessments[j] = assessmentScoreDTO(a)
		}
		for j, h := range s.Homework {
			cs.Homework[j] = homeworkScoreDTO{LessonID: h.LessonID, Title: h.Title, Total: h.Total, Max: h.Max, Status: string(h.Status)}
		}
		out.Stages[i] = cs
	}
	return out
}

type learnerSummaryDTO struct {
	Class         classDTO          `json:"class"`
	Stages        []stageSummaryDTO `json:"stages"`
	CoursePercent int               `json:"coursePercent"`
	RequiredDone  int               `json:"requiredDone"`
	RequiredTotal int               `json:"requiredTotal"`
	Course        courseSummaryDTO  `json:"course"`
}

func toLearnerSummaryDTO(s LearnerSummary) learnerSummaryDTO {
	out := learnerSummaryDTO{
		Class: toClassDTO(s.Class), Stages: make([]stageSummaryDTO, len(s.Stages)),
		CoursePercent: s.CoursePercent, RequiredDone: s.RequiredDone, RequiredTotal: s.RequiredTotal,
		Course: toCourseSummaryDTO(s.Course),
	}
	for i, st := range s.Stages {
		out.Stages[i] = toStageSummaryDTO(st)
	}
	return out
}

type matrixCellDTO struct {
	StageVersionID uuid.UUID `json:"stageVersionId"`
	Complete       bool      `json:"complete"`
	Done           int       `json:"done"`
	Required       int       `json:"required"`
	Missing        int       `json:"missing"`
	Rework         bool      `json:"rework"`
	Overdue        bool      `json:"overdue"`
	HasReview      bool      `json:"hasReview"`
}

type matrixRowDTO struct {
	MemberID      uuid.UUID       `json:"memberId"`
	Name          string          `json:"name"`
	Email         string          `json:"email"`
	CoursePercent int             `json:"coursePercent"`
	Cells         []matrixCellDTO `json:"cells"`
}

type classMatrixDTO struct {
	Class  classDTO       `json:"class"`
	Stages []stageRefDTO  `json:"stages"`
	Rows   []matrixRowDTO `json:"rows"`
}

func toClassMatrixDTO(m ClassMatrix) classMatrixDTO {
	out := classMatrixDTO{Class: toClassDTO(m.Class), Stages: make([]stageRefDTO, len(m.Stages)), Rows: make([]matrixRowDTO, len(m.Rows))}
	for i, s := range m.Stages {
		out.Stages[i] = toStageRefDTO(s)
	}
	for i, r := range m.Rows {
		cells := make([]matrixCellDTO, len(r.Cells))
		for j, c := range r.Cells {
			cells[j] = matrixCellDTO(c)
		}
		out.Rows[i] = matrixRowDTO{MemberID: r.Member.ID, Name: r.Member.Name, Email: r.Member.Email, CoursePercent: r.CoursePercent, Cells: cells}
	}
	return out
}

type memberDTO struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Email  string    `json:"email"`
	Status string    `json:"status"`
}

type memberStageDTO struct {
	Class   classDTO        `json:"class"`
	Member  memberDTO       `json:"member"`
	Summary stageSummaryDTO `json:"summary"`
}

func toMemberStageDTO(m MemberStage) memberStageDTO {
	return memberStageDTO{
		Class:   toClassDTO(m.Class),
		Member:  memberDTO{ID: m.Member.ID, Name: m.Member.Name, Email: m.Member.Email, Status: string(m.Member.Status)},
		Summary: toStageSummaryDTO(m.Summary),
	}
}

type reviewRequest struct {
	Body string `json:"body"`
}
