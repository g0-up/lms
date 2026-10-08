package learning

import (
	"time"

	"lms/api/internal/features/stages"
)

// dateLayout là định dạng ngày bắt đầu/kết thúc lớp trả cho client.
const dateLayout = "2006-01-02"

// ClassDTO là thông tin lớp trên thẻ "Lớp của tôi" và đầu lộ trình.
type ClassDTO struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	StartDate       string `json:"startDate"`
	EndDate         string `json:"endDate"`
	TeacherName     string `json:"teacherName"`
	CourseName      string `json:"courseName"`
	CourseVersionNo int    `json:"courseVersionNo"`
}

// NextLessonDTO là học liệu kế tiếp nên học.
type NextLessonDTO struct {
	LessonID  string `json:"lessonId"`
	Title     string `json:"title"`
	StageName string `json:"stageName"`
}

// MyClassItemDTO là một lớp trong GET /me/classes.
type MyClassItemDTO struct {
	ClassDTO
	Percent        int            `json:"percent"`
	RequiredDone   int            `json:"requiredDone"`
	RequiredTotal  int            `json:"requiredTotal"`
	NextLesson     *NextLessonDTO `json:"nextLesson,omitempty"`
	ReadOnlyReason string         `json:"readOnlyReason,omitempty"`
}

// MyClassesDTO là phản hồi GET /me/classes.
type MyClassesDTO struct {
	Items []MyClassItemDTO `json:"items"`
}

// RoadmapLessonDTO là một học liệu trong lộ trình; firstOpenedAt/completedAt vắng khi chưa mở/chưa hoàn thành.
type RoadmapLessonDTO struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Type            string     `json:"type"`
	Required        bool       `json:"required"`
	Position        int        `json:"position"`
	DurationSeconds *int       `json:"durationSeconds,omitempty"`
	FirstOpenedAt   *time.Time `json:"firstOpenedAt,omitempty"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
}

// RoadmapStageDTO là một chặng trong lộ trình; id là id chặng (không phải phiên bản chặng).
type RoadmapStageDTO struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Position int                `json:"position"`
	Lessons  []RoadmapLessonDTO `json:"lessons"`
}

// RoadmapDTO là phản hồi GET /me/classes/{id}.
type RoadmapDTO struct {
	Class          ClassDTO          `json:"class"`
	Percent        int               `json:"percent"`
	RequiredDone   int               `json:"requiredDone"`
	RequiredTotal  int               `json:"requiredTotal"`
	ReadOnly       bool              `json:"readOnly"`
	ReadOnlyReason string            `json:"readOnlyReason,omitempty"`
	NextLesson     *NextLessonDTO    `json:"nextLesson,omitempty"`
	Stages         []RoadmapStageDTO `json:"stages"`
	SelfReported   bool              `json:"selfReported"`
}

// StageRefDTO là chặng chứa học liệu trên trang học.
type StageRefDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LessonDTO là học liệu trên trang học.
type LessonDTO struct {
	ID              string      `json:"id"`
	Title           string      `json:"title"`
	Type            string      `json:"type"`
	Required        bool        `json:"required"`
	Position        int         `json:"position"`
	DurationSeconds *int        `json:"durationSeconds,omitempty"`
	Stage           StageRefDTO `json:"stage"`
}

// ContentDTO là {type:"video", mediaId, url, expiresAt} hoặc {type:"markdown", html}; trường của loại kia vắng.
type ContentDTO struct {
	Type      string     `json:"type"`
	MediaID   string     `json:"mediaId,omitempty"`
	URL       string     `json:"url,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	HTML      *string    `json:"html,omitempty"`
}

// ProgressDTO là tiến độ của học viên trên học liệu; firstOpenedAt null khi lớp đã kết thúc và chưa từng mở.
type ProgressDTO struct {
	FirstOpenedAt *time.Time `json:"firstOpenedAt"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}

// LessonLinkDTO là học liệu liền trước/sau.
type LessonLinkDTO struct {
	LessonID string `json:"lessonId"`
	Title    string `json:"title"`
}

// LessonPageDTO là phản hồi GET /me/classes/{id}/lessons/{lid}.
type LessonPageDTO struct {
	Lesson   LessonDTO      `json:"lesson"`
	Content  ContentDTO     `json:"content"`
	Progress ProgressDTO    `json:"progress"`
	ReadOnly bool           `json:"readOnly"`
	Prev     *LessonLinkDTO `json:"prev,omitempty"`
	Next     *LessonLinkDTO `json:"next,omitempty"`
}

// CompletionDTO là phản hồi PUT …/completion; completedAt null sau khi bỏ tích.
type CompletionDTO struct {
	CompletedAt   *time.Time `json:"completedAt"`
	Percent       int        `json:"percent"`
	RequiredDone  int        `json:"requiredDone"`
	RequiredTotal int        `json:"requiredTotal"`
}

// completionRequest: completed bắt buộc (con trỏ để phân biệt false với vắng mặt).
type completionRequest struct {
	Completed *bool `json:"completed" binding:"required"`
}

func classDTO(c ClassSummary) ClassDTO {
	return ClassDTO{
		ID: c.ID.String(), Code: c.Code, Name: c.Name, Status: string(c.Status),
		StartDate: c.StartDate.Format(dateLayout), EndDate: c.EndDate.Format(dateLayout),
		TeacherName: c.TeacherName, CourseName: c.CourseName, CourseVersionNo: c.CourseVersionNo,
	}
}

func nextLessonDTO(r *LessonRef) *NextLessonDTO {
	if r == nil {
		return nil
	}
	return &NextLessonDTO{LessonID: r.LessonID.String(), Title: r.Title, StageName: r.StageName}
}

func lessonLinkDTO(r *LessonRef) *LessonLinkDTO {
	if r == nil {
		return nil
	}
	return &LessonLinkDTO{LessonID: r.LessonID.String(), Title: r.Title}
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}

func toMyClassesDTO(rows []MyClassRow) MyClassesDTO {
	items := make([]MyClassItemDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, MyClassItemDTO{
			ClassDTO: classDTO(r.Class), Percent: r.Percent, RequiredDone: r.RequiredDone, RequiredTotal: r.RequiredTotal,
			NextLesson: nextLessonDTO(r.NextLesson), ReadOnlyReason: readOnlyReason(r.Class.Status),
		})
	}
	return MyClassesDTO{Items: items}
}

func toRoadmapDTO(r Roadmap) RoadmapDTO {
	out := RoadmapDTO{
		Class: classDTO(r.Class), Percent: r.Percent, RequiredDone: r.RequiredDone, RequiredTotal: r.RequiredTotal,
		ReadOnly: r.ReadOnly, ReadOnlyReason: r.ReadOnlyReason, NextLesson: nextLessonDTO(r.NextLesson),
		Stages: make([]RoadmapStageDTO, 0, len(r.Stages)), SelfReported: r.SelfReported,
	}
	for _, st := range r.Stages {
		s := RoadmapStageDTO{ID: st.StageID.String(), Name: st.Name, Position: st.Position, Lessons: make([]RoadmapLessonDTO, 0, len(st.Lessons))}
		for _, l := range st.Lessons {
			s.Lessons = append(s.Lessons, RoadmapLessonDTO{
				ID: l.ID.String(), Title: l.Title, Type: string(l.Type), Required: l.Required, Position: l.Position,
				DurationSeconds: l.DurationSeconds, FirstOpenedAt: utc(l.FirstOpenedAt), CompletedAt: utc(l.CompletedAt),
			})
		}
		out.Stages = append(out.Stages, s)
	}
	return out
}

func toLessonPageDTO(p LessonPage) LessonPageDTO {
	l := p.Lesson
	content := ContentDTO{Type: string(p.Content.Type)}
	switch p.Content.Type {
	case stages.LessonVideo:
		exp := p.Content.ExpiresAt
		content.MediaID, content.URL, content.ExpiresAt = p.Content.MediaID.String(), p.Content.URL, utc(&exp)
	case stages.LessonMarkdown:
		html := p.Content.HTML
		content.HTML = &html
	}
	return LessonPageDTO{
		Lesson: LessonDTO{
			ID: l.ID.String(), Title: l.Title, Type: string(l.Type), Required: l.Required, Position: l.Position,
			DurationSeconds: l.DurationSeconds, Stage: StageRefDTO{ID: p.Stage.StageID.String(), Name: p.Stage.Name},
		},
		Content:  content,
		Progress: ProgressDTO{FirstOpenedAt: utc(l.FirstOpenedAt), CompletedAt: utc(l.CompletedAt)},
		ReadOnly: p.ReadOnly, Prev: lessonLinkDTO(p.Prev), Next: lessonLinkDTO(p.Next),
	}
}

func toCompletionDTO(c Completion) CompletionDTO {
	return CompletionDTO{CompletedAt: utc(c.CompletedAt), Percent: c.Percent, RequiredDone: c.RequiredDone, RequiredTotal: c.RequiredTotal}
}
