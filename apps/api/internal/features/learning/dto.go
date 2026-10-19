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
	UnitID          string     `json:"unitId"`
	Status          *StatusDTO `json:"status,omitempty"`
}

// StatusDTO là nhãn trạng thái quiz/homework ("Chờ chấm", "Quá hạn", "Tốt nhất 80%"…); tone là ok|info|warn|danger|
// optional. Vắng với video và markdown.
type StatusDTO struct {
	Label string `json:"label"`
	Tone  string `json:"tone"`
}

// RoadmapUnitDTO là một bài học trong chặng; nhãn "Bài i.j" do web ghép từ position của chặng và bài học.
type RoadmapUnitDTO struct {
	ID            string             `json:"id"`
	Title         string             `json:"title"`
	Position      int                `json:"position"`
	RequiredDone  int                `json:"requiredDone"`
	RequiredTotal int                `json:"requiredTotal"`
	Complete      bool               `json:"complete"`
	Optional      bool               `json:"optional"`
	Lessons       []RoadmapLessonDTO `json:"lessons"`
}

// RoadmapStageDTO là một chặng trong lộ trình; id là id chặng (không phải phiên bản chặng). lessons là học liệu
// phẳng theo thứ tự, units nhóm cùng học liệu đó theo bài học.
type RoadmapStageDTO struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Position      int                `json:"position"`
	UnitCount     int                `json:"unitCount"`
	LessonCount   int                `json:"lessonCount"`
	RequiredDone  int                `json:"requiredDone"`
	RequiredTotal int                `json:"requiredTotal"`
	Units         []RoadmapUnitDTO   `json:"units"`
	Lessons       []RoadmapLessonDTO `json:"lessons"`
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
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// UnitRefDTO là bài học chứa học liệu trên trang học (breadcrumb và nhãn "Bài i.j").
type UnitRefDTO struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
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
	Unit            UnitRefDTO  `json:"unit"`
	Status          *StatusDTO  `json:"status,omitempty"`
}

// ContentDTO là {type:"video", mediaId, url, expiresAt}, {type:"markdown", html}, {type:"homework", homework} hoặc
// {type:"quiz", quiz}; trường của loại khác vắng.
type ContentDTO struct {
	Type      string       `json:"type"`
	MediaID   string       `json:"mediaId,omitempty"`
	URL       string       `json:"url,omitempty"`
	ExpiresAt *time.Time   `json:"expiresAt,omitempty"`
	HTML      *string      `json:"html,omitempty"`
	Homework  *HomeworkDTO `json:"homework,omitempty"`
	Quiz      *QuizDTO     `json:"quiz,omitempty"`
}

// QuizDTO là tóm tắt quiz trên trang học; câu hỏi và lượt làm lấy qua /me/classes/{id}/lessons/{lid}/attempts.
type QuizDTO struct {
	Mode             string `json:"mode"`
	QuestionCount    int    `json:"questionCount"`
	MaxScore         int    `json:"maxScore"`
	PassPercent      *int   `json:"passPercent,omitempty"`
	TimeLimitMinutes *int   `json:"timeLimitMinutes,omitempty"`
	MaxAttempts      *int   `json:"maxAttempts,omitempty"`
}

// HomeworkDTO là đề bài (HTML đã sanitize), ngưỡng đạt, tổng điểm tối đa, tiêu chí và file kèm theo thứ tự. File kèm
// tải qua GET /media/{mediaId}/url (ký dạng attachment).
type HomeworkDTO struct {
	BriefHTML   string          `json:"briefHtml"`
	PassPercent int             `json:"passPercent"`
	MaxPoints   int             `json:"maxPoints"`
	Criteria    []CriterionDTO  `json:"criteria"`
	Attachments []AttachmentDTO `json:"attachments"`
}

// CriterionDTO là một tiêu chí chấm.
type CriterionDTO struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	MaxPoints   int    `json:"maxPoints"`
}

// AttachmentDTO là một file kèm đề bài.
type AttachmentDTO struct {
	MediaID   string `json:"mediaId"`
	FileName  string `json:"fileName"`
	SizeBytes int64  `json:"sizeBytes"`
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

// LessonPageDTO là phản hồi GET /me/classes/{id}/lessons/{lid}. crossesUnit khi học liệu tiếp thuộc bài học khác;
// nextUnitTitle là tên bài học đó (vắng khi không qua ranh giới).
type LessonPageDTO struct {
	Lesson        LessonDTO      `json:"lesson"`
	Content       ContentDTO     `json:"content"`
	Progress      ProgressDTO    `json:"progress"`
	ReadOnly      bool           `json:"readOnly"`
	Prev          *LessonLinkDTO `json:"prev,omitempty"`
	Next          *LessonLinkDTO `json:"next,omitempty"`
	CrossesUnit   bool           `json:"crossesUnit"`
	NextUnitTitle string         `json:"nextUnitTitle,omitempty"`
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
		s := RoadmapStageDTO{
			ID: st.StageID.String(), Name: st.Name, Position: st.Position, UnitCount: len(st.Units), LessonCount: len(st.Lessons),
			RequiredDone: st.RequiredDone, RequiredTotal: st.RequiredTotal,
			Units: make([]RoadmapUnitDTO, 0, len(st.Units)), Lessons: roadmapLessonDTOs(st.Lessons),
		}
		for _, u := range st.Units {
			s.Units = append(s.Units, RoadmapUnitDTO{
				ID: u.ID.String(), Title: u.Title, Position: u.Position, RequiredDone: u.RequiredDone,
				RequiredTotal: u.RequiredTotal, Complete: u.Complete, Optional: u.Optional, Lessons: roadmapLessonDTOs(u.Lessons),
			})
		}
		out.Stages = append(out.Stages, s)
	}
	return out
}

func statusDTO(s *LessonStatus) *StatusDTO {
	if s == nil {
		return nil
	}
	return &StatusDTO{Label: s.Label, Tone: s.Tone}
}

func roadmapLessonDTOs(lessons []LessonView) []RoadmapLessonDTO {
	out := make([]RoadmapLessonDTO, 0, len(lessons))
	for _, l := range lessons {
		out = append(out, RoadmapLessonDTO{
			ID: l.ID.String(), Title: l.Title, Type: string(l.Type), Required: l.Required, Position: l.Position,
			DurationSeconds: l.DurationSeconds, FirstOpenedAt: utc(l.FirstOpenedAt), CompletedAt: utc(l.CompletedAt),
			UnitID: l.UnitID.String(), Status: statusDTO(l.Status),
		})
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
	case stages.LessonHomework:
		if hw := p.Content.Homework; hw != nil {
			content.Homework = toHomeworkDTO(*hw)
		}
	case stages.LessonQuiz:
		if q := p.Content.Quiz; q != nil {
			content.Quiz = toQuizDTO(*q)
		}
	}
	return LessonPageDTO{
		Lesson: LessonDTO{
			ID: l.ID.String(), Title: l.Title, Type: string(l.Type), Required: l.Required, Position: l.Position,
			DurationSeconds: l.DurationSeconds,
			Stage:           StageRefDTO{ID: p.Stage.StageID.String(), Name: p.Stage.Name, Position: p.Stage.Position},
			Unit:            UnitRefDTO{ID: p.Unit.ID.String(), Title: p.Unit.Title, Position: p.Unit.Position},
			Status:          statusDTO(l.Status),
		},
		Content:  content,
		Progress: ProgressDTO{FirstOpenedAt: utc(l.FirstOpenedAt), CompletedAt: utc(l.CompletedAt)},
		ReadOnly: p.ReadOnly, Prev: lessonLinkDTO(p.Prev), Next: lessonLinkDTO(p.Next),
		CrossesUnit: p.CrossesUnit, NextUnitTitle: p.NextUnitTitle,
	}
}

func toHomeworkDTO(c stages.HomeworkContent) *HomeworkDTO {
	out := &HomeworkDTO{PassPercent: c.PassPercent, MaxPoints: c.MaxScore(), Criteria: []CriterionDTO{}, Attachments: []AttachmentDTO{}}
	if c.BriefHTML != nil {
		out.BriefHTML = *c.BriefHTML
	}
	for _, cr := range c.Criteria {
		out.Criteria = append(out.Criteria, CriterionDTO{ID: cr.ID.String(), Description: cr.Description, MaxPoints: cr.MaxPoints})
	}
	for _, a := range c.Attachments {
		out.Attachments = append(out.Attachments, AttachmentDTO{MediaID: a.MediaID.String(), FileName: a.FileName, SizeBytes: a.SizeBytes})
	}
	return out
}

func toCompletionDTO(c Completion) CompletionDTO {
	return CompletionDTO{CompletedAt: utc(c.CompletedAt), Percent: c.Percent, RequiredDone: c.RequiredDone, RequiredTotal: c.RequiredTotal}
}

func toQuizDTO(q QuizSummary) *QuizDTO {
	out := &QuizDTO{
		Mode: string(q.Mode), QuestionCount: q.QuestionCount, MaxScore: q.MaxScore, PassPercent: q.PassPercent,
		MaxAttempts: q.MaxAttempts,
	}
	if q.TimeLimit != nil {
		m := int(*q.TimeLimit / time.Minute)
		out.TimeLimitMinutes = &m
	}
	return out
}
