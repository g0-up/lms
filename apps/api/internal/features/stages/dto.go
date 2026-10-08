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

// LessonDTO là học liệu; trường theo loại chỉ có mặt ở loại tương ứng.
type LessonDTO struct {
	ID              string  `json:"id"`
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
	LessonKey       *string    `json:"lessonKey"`
	Title           *string    `json:"title"`
	Type            *string    `json:"type"`
	Required        *bool      `json:"required"`
	DurationSeconds *int       `json:"durationSeconds"`
	MarkdownSource  *string    `json:"markdownSource"`
	VideoMediaID    *uuid.UUID `json:"videoMediaId"`
}

// previewRequest là thân POST /stages/markdown-preview.
type previewRequest struct {
	MarkdownSource string `json:"markdownSource"`
}

// MarkdownPreviewDTO là HTML đã lọc trả cho tab xem trước, giống markdownHtml sau khi phát hành.
type MarkdownPreviewDTO struct {
	HTML string `json:"html"`
}

type reorderRequest struct {
	LessonIDs []uuid.UUID `json:"lessonIds"`
}

func (r lessonRequest) cmd() LessonCmd {
	return LessonCmd{
		LessonKey: r.LessonKey, Title: r.Title, Type: r.Type, Required: r.Required,
		DurationSeconds: r.DurationSeconds, VideoMediaID: r.VideoMediaID, MarkdownSource: r.MarkdownSource,
	}
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
		ID: l.ID().String(), LessonKey: string(l.Key()), Title: l.Title(), Type: string(l.Content().Type()),
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
	}
	return dto
}

func toVersionDTO(d VersionDetail) StageVersionDTO {
	v := d.Version
	lessons := make([]LessonDTO, 0, len(v.Lessons()))
	for _, l := range v.Lessons() {
		lessons = append(lessons, toLessonDTO(l, v.Status()))
	}
	return StageVersionDTO{
		ID: v.ID().String(), StageID: v.StageID().String(), StageCode: d.Stage.Code, StageName: d.Stage.Name,
		VersionNo: v.VersionNo().Int(), Status: v.Status().String(), PublishedAt: utcPtr(v.PublishedAt()),
		ClonedFromVersionNo: noPtr(d.ClonedFromVersionNo), Lessons: lessons, UsedBy: usedByDTOs(d.UsedBy),
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
