package stages

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

// VersionSummaryDTO là một phiên bản trong danh sách/chi tiết chặng.
type VersionSummaryDTO struct {
	ID                  string     `json:"id"`
	VersionNo           int        `json:"versionNo"`
	Status              string     `json:"status"`
	LessonCount         int        `json:"lessonCount"`
	PublishedAt         *time.Time `json:"publishedAt,omitempty"`
	ClonedFromVersionNo *int       `json:"clonedFromVersionNo,omitempty"`
}

// StageListItem là một chặng trên GET /stages.
type StageListItem struct {
	ID                  string              `json:"id"`
	Code                string              `json:"code"`
	Name                string              `json:"name"`
	Description         string              `json:"description,omitempty"`
	LatestVersionNo     int                 `json:"latestVersionNo"`
	LatestPublishedNo   *int                `json:"latestPublishedNo,omitempty"`
	DraftVersionID      *string             `json:"draftVersionId,omitempty"`
	Versions            []VersionSummaryDTO `json:"versions"`
	UsedByCourseCount   int                 `json:"usedByCourseCount"`
	OutdatedCourseCount int                 `json:"outdatedCourseCount"`
}

// StageListDTO là phản hồi GET /stages.
type StageListDTO struct {
	Items []StageListItem `json:"items"`
}

// UsedByRowDTO là một phiên bản khóa học tham chiếu phiên bản chặng.
type UsedByRowDTO struct {
	CourseID        string   `json:"courseId"`
	CourseCode      string   `json:"courseCode"`
	CourseName      string   `json:"courseName"`
	CourseVersionID string   `json:"courseVersionId"`
	VersionNo       int      `json:"versionNo"`
	Status          string   `json:"status"`
	ClassCodes      []string `json:"classCodes"`
}

// StageUsedByRowDTO là UsedByRowDTO trên GET /stages/{id}, kèm phiên bản chặng đang dùng và cờ đã cũ.
type StageUsedByRowDTO struct {
	UsedByRowDTO
	StageVersionNo int  `json:"stageVersionNo"`
	Outdated       bool `json:"outdated"`
}

// OutdatedCourseRow là khóa học dùng phiên bản chặng cũ (FR-18); canApply=false khi khóa học đã có bản nháp.
type OutdatedCourseRow struct {
	CourseID        string  `json:"courseId"`
	CourseCode      string  `json:"courseCode"`
	CourseName      string  `json:"courseName"`
	CourseVersionID string  `json:"courseVersionId"`
	CourseVersionNo int     `json:"courseVersionNo"`
	UsingVersionNo  int     `json:"usingVersionNo"`
	LatestVersionID string  `json:"latestVersionId"`
	LatestVersionNo int     `json:"latestVersionNo"`
	CanApply        bool    `json:"canApply"`
	BlockedReason   *string `json:"blockedReason,omitempty"`
	DraftVersionID  *string `json:"draftVersionId,omitempty"`
	DraftVersionNo  *int    `json:"draftVersionNo,omitempty"`
}

// StageDetailDTO là phản hồi GET/POST /stages.
type StageDetailDTO struct {
	ID              string              `json:"id"`
	Code            string              `json:"code"`
	Name            string              `json:"name"`
	Description     string              `json:"description,omitempty"`
	Versions        []VersionSummaryDTO `json:"versions"`
	UsedBy          []StageUsedByRowDTO `json:"usedBy"`
	OutdatedCourses []OutdatedCourseRow `json:"outdatedCourses"`
}

// UnitDTO là bài học kèm số học liệu và số học liệu bắt buộc.
type UnitDTO struct {
	ID            string `json:"id"`
	UnitKey       string `json:"unitKey"`
	Title         string `json:"title"`
	Position      int    `json:"position"`
	LessonCount   int    `json:"lessonCount"`
	RequiredCount int    `json:"requiredCount"`
}

// LessonDTO là học liệu; trường theo loại chỉ có mặt ở loại tương ứng.
type LessonDTO struct {
	ID              string  `json:"id"`
	UnitID          string  `json:"unitId"`
	LessonKey       string  `json:"lessonKey"`
	Title           string  `json:"title"`
	Type            string  `json:"type"`
	Required        bool    `json:"required"`
	Position        int     `json:"position"`
	DurationSeconds *int    `json:"durationSeconds,omitempty"`
	MarkdownSource  *string `json:"markdownSource,omitempty"`
	MarkdownHTML    *string `json:"markdownHtml,omitempty"`
	VideoMediaID    *string `json:"videoMediaId,omitempty"`
	VideoFileName   *string `json:"videoFileName,omitempty"`
	// Quiz có đáp án đúng nên LessonDTO chỉ dùng cho route admin; học viên đọc qua learning.
	Quiz     *QuizDTO     `json:"quiz,omitempty"`
	Homework *HomeworkDTO `json:"homework,omitempty"`
}

// QuizDTO là cấu hình và câu hỏi của quiz; giới hạn chỉ có ở chế độ assessment.
type QuizDTO struct {
	Mode             string        `json:"mode"`
	MaxAttempts      *int          `json:"maxAttempts"`
	TimeLimitMinutes *int          `json:"timeLimitMinutes"`
	Shuffle          bool          `json:"shuffle"`
	PassPercent      *int          `json:"passPercent"`
	Questions        []QuestionDTO `json:"questions"`
}

// QuestionDTO là câu hỏi kèm phương án và đáp án đúng; HTML chỉ có khi phiên bản không còn là nháp.
type QuestionDTO struct {
	ID                string      `json:"id"`
	QuestionKey       string      `json:"questionKey"`
	Type              string      `json:"type"`
	BodySource        string      `json:"bodySource"`
	BodyHTML          *string     `json:"bodyHtml,omitempty"`
	ExplanationSource string      `json:"explanationSource"`
	ExplanationHTML   *string     `json:"explanationHtml,omitempty"`
	Points            int         `json:"points"`
	Options           []OptionDTO `json:"options"`
}

// OptionDTO là một phương án.
type OptionDTO struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	IsCorrect bool   `json:"isCorrect"`
}

// HomeworkDTO là đề bài, ngưỡng đạt, rubric và file kèm; maxPoints là tổng điểm tối đa các tiêu chí.
type HomeworkDTO struct {
	BriefSource string          `json:"briefSource"`
	BriefHTML   *string         `json:"briefHtml,omitempty"`
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

// StageVersionDTO là phản hồi của các route /stage-versions/{vid}.
type StageVersionDTO struct {
	ID                  string         `json:"id"`
	StageID             string         `json:"stageId"`
	StageCode           string         `json:"stageCode"`
	StageName           string         `json:"stageName"`
	VersionNo           int            `json:"versionNo"`
	Status              string         `json:"status"`
	PublishedAt         *time.Time     `json:"publishedAt,omitempty"`
	ClonedFromVersionNo *int           `json:"clonedFromVersionNo,omitempty"`
	Units               []UnitDTO      `json:"units"`
	Lessons             []LessonDTO    `json:"lessons"`
	UsedBy              []UsedByRowDTO `json:"usedBy"`
}

// DeleteVersionDTO là phản hồi DELETE /stage-versions/{vid}.
type DeleteVersionDTO struct {
	StageDeleted bool `json:"stageDeleted"`
}

// DraftExistsDetails là details của lỗi DRAFT_EXISTS.
type DraftExistsDetails struct {
	DraftVersionID string `json:"draftVersionId"`
	DraftVersionNo int    `json:"draftVersionNo"`
}

// InUseDetails là details của lỗi IN_USE.
type InUseDetails struct {
	UsedBy []UsedByRowDTO `json:"usedBy"`
}

// createStageRequest không gắn tag binding: kiểm tra nằm ở service để trả 422 với thông điệp spec.
type createStageRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// lessonRequest dùng cho cả POST và PATCH; trường vắng mặt là nil.
type lessonRequest struct {
	UnitID          *uuid.UUID       `json:"unitId"`
	LessonKey       *string          `json:"lessonKey"`
	Title           *string          `json:"title"`
	Type            *string          `json:"type"`
	Required        *bool            `json:"required"`
	DurationSeconds *int             `json:"durationSeconds"`
	MarkdownSource  *string          `json:"markdownSource"`
	VideoMediaID    *uuid.UUID       `json:"videoMediaId"`
	Quiz            *quizRequest     `json:"quiz"`
	Homework        *homeworkRequest `json:"homework"`
}

// quizRequest là cấu hình quiz; kiểm tra nằm ở service/domain để trả 422 với thông điệp spec.
type quizRequest struct {
	Mode             *string `json:"mode"`
	MaxAttempts      *int    `json:"maxAttempts"`
	TimeLimitMinutes *int    `json:"timeLimitMinutes"`
	Shuffle          *bool   `json:"shuffle"`
	PassPercent      *int    `json:"passPercent"`
}

// homeworkRequest là đề bài, ngưỡng đạt và file kèm; attachmentMediaIds rỗng là xóa hết file kèm.
type homeworkRequest struct {
	BriefSource        *string     `json:"briefSource"`
	PassPercent        *int        `json:"passPercent"`
	AttachmentMediaIDs []uuid.UUID `json:"attachmentMediaIds"`
}

// questionRequest là thân POST/PATCH câu hỏi; PATCH cũng gửi đủ câu hỏi.
type questionRequest struct {
	QuestionKey       *string         `json:"questionKey"`
	Type              string          `json:"type"`
	BodySource        string          `json:"bodySource"`
	ExplanationSource string          `json:"explanationSource"`
	Points            int             `json:"points"`
	Options           []optionRequest `json:"options"`
}

type optionRequest struct {
	ID      *uuid.UUID `json:"id"`
	Body    string     `json:"body"`
	Correct bool       `json:"correct"`
}

// criterionRequest là thân POST/PATCH tiêu chí.
type criterionRequest struct {
	Description string `json:"description"`
	MaxPoints   int    `json:"maxPoints"`
}

// reorderQuestionsRequest là thân PUT /questions/order: hoán vị đầy đủ id câu hỏi.
type reorderQuestionsRequest struct {
	QuestionIDs []uuid.UUID `json:"questionIds"`
}

// reorderCriteriaRequest là thân PUT /criteria/order: hoán vị đầy đủ id tiêu chí.
type reorderCriteriaRequest struct {
	CriterionIDs []uuid.UUID `json:"criterionIds"`
}

func (r questionRequest) cmd() QuestionCmd {
	opts := make([]OptionCmd, 0, len(r.Options))
	for _, o := range r.Options {
		opts = append(opts, OptionCmd(o))
	}
	return QuestionCmd{
		QuestionKey: r.QuestionKey, Type: r.Type, BodySource: r.BodySource, ExplanationSource: r.ExplanationSource,
		Points: r.Points, Options: opts,
	}
}

// previewRequest là thân POST /stages/markdown-preview.
type previewRequest struct {
	MarkdownSource string `json:"markdownSource"`
}

// MarkdownPreviewDTO là HTML đã lọc trả cho tab xem trước, giống markdownHtml sau khi phát hành.
type MarkdownPreviewDTO struct {
	HTML string `json:"html"`
}

// unitRequest là thân POST/PATCH bài học.
type unitRequest struct {
	Title string `json:"title"`
}

// reorderUnitsRequest là thân PUT /units/order: hoán vị đầy đủ id bài học.
type reorderUnitsRequest struct {
	UnitIDs []uuid.UUID `json:"unitIds"`
}

// reorderRequest là thân PUT /lessons/order: bố cục đầy đủ, mỗi bài học kèm học liệu theo thứ tự.
type reorderRequest struct {
	Units []unitLessonsRequest `json:"units"`
}

type unitLessonsRequest struct {
	UnitID    uuid.UUID   `json:"unitId"`
	LessonIDs []uuid.UUID `json:"lessonIds"`
}

func (r reorderRequest) layout() []UnitLessons {
	out := make([]UnitLessons, 0, len(r.Units))
	for _, u := range r.Units {
		out = append(out, UnitLessons(u))
	}
	return out
}

func (r lessonRequest) cmd() LessonCmd {
	cmd := LessonCmd{
		UnitID: r.UnitID, LessonKey: r.LessonKey, Title: r.Title, Type: r.Type, Required: r.Required,
		DurationSeconds: r.DurationSeconds, VideoMediaID: r.VideoMediaID, MarkdownSource: r.MarkdownSource,
	}
	if r.Quiz != nil {
		q := QuizCmd(*r.Quiz)
		cmd.Quiz = &q
	}
	if r.Homework != nil {
		h := HomeworkCmd(*r.Homework)
		cmd.Homework = &h
	}
	return cmd
}

// BlockedReason là thông điệp FR-18 khi khóa học đã có bản nháp nên chưa áp được phiên bản chặng mới.
func BlockedReason(draftNo domain.VersionNo) string {
	return fmt.Sprintf("Khóa học đang có bản nháp v%d. Phát hành hoặc xóa bản nháp trước.", draftNo)
}

func versionSummaryDTOs(in []VersionSummary) []VersionSummaryDTO {
	out := make([]VersionSummaryDTO, 0, len(in))
	for _, v := range in {
		out = append(out, VersionSummaryDTO{
			ID: v.ID.String(), VersionNo: v.VersionNo.Int(), Status: v.Status.String(), LessonCount: v.LessonCount,
			PublishedAt: utcPtr(v.PublishedAt), ClonedFromVersionNo: noPtr(v.ClonedFromVersionNo),
		})
	}
	return out
}

// toListItem suy ra latestVersionNo/latestPublishedNo/draftVersionId từ danh sách phiên bản (đã sắp DESC).
func toListItem(r StageListRow) StageListItem {
	item := StageListItem{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, Description: r.Description,
		Versions: versionSummaryDTOs(r.Versions), UsedByCourseCount: r.UsedByCourseCount, OutdatedCourseCount: r.OutdatedCourseCount,
	}
	for _, v := range r.Versions {
		if v.VersionNo.Int() > item.LatestVersionNo {
			item.LatestVersionNo = v.VersionNo.Int()
		}
		switch v.Status {
		case domain.VersionPublished:
			if item.LatestPublishedNo == nil || v.VersionNo.Int() > *item.LatestPublishedNo {
				n := v.VersionNo.Int()
				item.LatestPublishedNo = &n
			}
		case domain.VersionDraft:
			id := v.ID.String()
			item.DraftVersionID = &id
		}
	}
	return item
}

func toListDTO(rows []StageListRow) StageListDTO {
	items := make([]StageListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, toListItem(r))
	}
	return StageListDTO{Items: items}
}

func toUsedByDTO(u UsedByRow) UsedByRowDTO {
	codes := u.ClassCodes
	if codes == nil {
		codes = []string{}
	}
	return UsedByRowDTO{
		CourseID: u.CourseID.String(), CourseCode: u.CourseCode, CourseName: u.CourseName,
		CourseVersionID: u.CourseVersionID.String(), VersionNo: u.VersionNo.Int(), Status: u.Status.String(), ClassCodes: codes,
	}
}

func usedByDTOs(in []UsedByRow) []UsedByRowDTO {
	out := make([]UsedByRowDTO, 0, len(in))
	for _, u := range in {
		out = append(out, toUsedByDTO(u))
	}
	return out
}

// ToOutdatedRow chuyển một khóa học FR-18 sang DTO; dashboard dùng lại nên xuất ra.
func ToOutdatedRow(o OutdatedCourse) OutdatedCourseRow {
	row := OutdatedCourseRow{
		CourseID: o.CourseID.String(), CourseCode: o.CourseCode, CourseName: o.CourseName,
		CourseVersionID: o.CourseVersionID.String(), CourseVersionNo: o.CourseVersionNo.Int(),
		UsingVersionNo: o.UsingVersionNo.Int(), LatestVersionID: o.LatestVersionID.String(), LatestVersionNo: o.LatestVersionNo.Int(),
		CanApply: o.DraftVersionID == nil,
	}
	if o.DraftVersionID != nil {
		id := o.DraftVersionID.String()
		row.DraftVersionID = &id
	}
	if o.DraftVersionNo != nil {
		n := o.DraftVersionNo.Int()
		row.DraftVersionNo = &n
		reason := BlockedReason(*o.DraftVersionNo)
		row.BlockedReason = &reason
	}
	return row
}

func toStageDetailDTO(d StageDetail) StageDetailDTO {
	usedBy := make([]StageUsedByRowDTO, 0, len(d.UsedBy))
	for _, u := range d.UsedBy {
		usedBy = append(usedBy, StageUsedByRowDTO{UsedByRowDTO: toUsedByDTO(u.UsedByRow), StageVersionNo: u.StageVersionNo.Int(), Outdated: u.Outdated})
	}
	outdated := make([]OutdatedCourseRow, 0, len(d.Outdated))
	for _, o := range d.Outdated {
		outdated = append(outdated, ToOutdatedRow(o))
	}
	return StageDetailDTO{
		ID: d.Stage.ID.String(), Code: d.Stage.Code, Name: d.Stage.Name, Description: d.Stage.Description,
		Versions: versionSummaryDTOs(d.Versions), UsedBy: usedBy, OutdatedCourses: outdated,
	}
}

// toLessonDTO: markdownHtml chỉ trả khi phiên bản không còn là nháp (HTML của nháp chưa render hoặc đã cũ so với source).
func toLessonDTO(l *Lesson, status domain.VersionStatus) LessonDTO {
	dto := LessonDTO{
		ID: l.ID().String(), UnitID: l.UnitID().String(), LessonKey: string(l.Key()), Title: l.Title(), Type: string(l.Content().Type()),
		Required: l.Required(), Position: l.Position(),
	}
	switch c := l.Content().(type) {
	case VideoContent:
		id, name := c.MediaID.String(), c.FileName
		dto.VideoMediaID, dto.VideoFileName = &id, &name
		if c.DurationSeconds != nil {
			d := *c.DurationSeconds
			dto.DurationSeconds = &d
		}
	case MarkdownContent:
		src := c.Source
		dto.MarkdownSource = &src
		if status != domain.VersionDraft && c.HTML != nil {
			html := *c.HTML
			dto.MarkdownHTML = &html
		}
	case QuizContent:
		dto.Quiz = toQuizDTO(c, status)
	case HomeworkContent:
		dto.Homework = toHomeworkDTO(c, status)
	}
	return dto
}

func toQuizDTO(c QuizContent, status domain.VersionStatus) *QuizDTO {
	dto := &QuizDTO{
		Mode: string(c.Mode), MaxAttempts: copyPtr(c.MaxAttempts), Shuffle: c.Shuffle, PassPercent: copyPtr(c.PassPercent),
		Questions: make([]QuestionDTO, 0, len(c.Questions)),
	}
	if c.TimeLimit != nil {
		m := int(*c.TimeLimit / time.Minute)
		dto.TimeLimitMinutes = &m
	}
	for _, q := range c.Questions {
		qd := QuestionDTO{
			ID: q.ID.String(), QuestionKey: string(q.Key), Type: string(q.Type), BodySource: q.BodySource,
			ExplanationSource: q.ExplanationSource, Points: q.Points, Options: make([]OptionDTO, 0, len(q.Options)),
		}
		if status != domain.VersionDraft {
			qd.BodyHTML, qd.ExplanationHTML = copyPtr(q.BodyHTML), copyPtr(q.ExplanationHTML)
		}
		for _, o := range q.Options {
			qd.Options = append(qd.Options, OptionDTO{ID: o.ID.String(), Body: o.Body, IsCorrect: o.Correct})
		}
		dto.Questions = append(dto.Questions, qd)
	}
	return dto
}

func toHomeworkDTO(c HomeworkContent, status domain.VersionStatus) *HomeworkDTO {
	dto := &HomeworkDTO{
		BriefSource: c.BriefSource, PassPercent: c.PassPercent, MaxPoints: c.MaxScore(),
		Criteria: make([]CriterionDTO, 0, len(c.Criteria)), Attachments: make([]AttachmentDTO, 0, len(c.Attachments)),
	}
	if status != domain.VersionDraft {
		dto.BriefHTML = copyPtr(c.BriefHTML)
	}
	for _, cr := range c.Criteria {
		dto.Criteria = append(dto.Criteria, CriterionDTO{ID: cr.ID.String(), Description: cr.Description, MaxPoints: cr.MaxPoints})
	}
	for _, a := range c.Attachments {
		dto.Attachments = append(dto.Attachments, AttachmentDTO{MediaID: a.MediaID.String(), FileName: a.FileName, SizeBytes: a.SizeBytes})
	}
	return dto
}

func toUnitDTO(d UnitDetail) UnitDTO {
	dto := UnitDTO{
		ID: d.Unit.ID().String(), UnitKey: string(d.Unit.Key()), Title: d.Unit.Title(), Position: d.Unit.Position(),
		LessonCount: len(d.Lessons),
	}
	for _, l := range d.Lessons {
		if l.Required() {
			dto.RequiredCount++
		}
	}
	return dto
}

func toVersionDTO(d VersionDetail) StageVersionDTO {
	v := d.Version
	units := make([]UnitDTO, 0, len(v.Units()))
	for _, u := range v.Units() {
		units = append(units, toUnitDTO(UnitDetail{Unit: u, Lessons: v.LessonsOf(u.ID())}))
	}
	lessons := make([]LessonDTO, 0, len(v.Lessons()))
	for _, l := range v.Lessons() {
		lessons = append(lessons, toLessonDTO(l, v.Status()))
	}
	return StageVersionDTO{
		ID: v.ID().String(), StageID: v.StageID().String(), StageCode: d.Stage.Code, StageName: d.Stage.Name,
		VersionNo: v.VersionNo().Int(), Status: v.Status().String(), PublishedAt: utcPtr(v.PublishedAt()),
		ClonedFromVersionNo: noPtr(d.ClonedFromVersionNo), Units: units, Lessons: lessons, UsedBy: usedByDTOs(d.UsedBy),
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func noPtr(n *domain.VersionNo) *int {
	if n == nil {
		return nil
	}
	i := n.Int()
	return &i
}
