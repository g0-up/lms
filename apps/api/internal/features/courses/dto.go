package courses

import (
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

// VersionItemDTO là một phiên bản trong danh sách khóa học.
type VersionItemDTO struct {
	ID                 string     `json:"id"`
	VersionNo          int        `json:"versionNo"`
	Status             string     `json:"status"`
	PublishedAt        *time.Time `json:"publishedAt,omitempty"`
	StageCount         int        `json:"stageCount"`
	OutdatedStageCount int        `json:"outdatedStageCount"`
}

// VersionSummaryDTO là một phiên bản trên chi tiết khóa học.
type VersionSummaryDTO struct {
	VersionItemDTO
	ClassCount          int  `json:"classCount"`
	ClonedFromVersionNo *int `json:"clonedFromVersionNo,omitempty"`
}

// ClassUsingDTO là lớp trỏ tới một phiên bản của khóa học.
type ClassUsingDTO struct {
	ClassID   string `json:"classId"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	VersionNo int    `json:"versionNo"`
}

// CourseListItem là một khóa học trên GET /courses.
type CourseListItem struct {
	ID                string           `json:"id"`
	Code              string           `json:"code"`
	Name              string           `json:"name"`
	Description       string           `json:"description,omitempty"`
	LatestVersionNo   int              `json:"latestVersionNo"`
	LatestPublishedNo *int             `json:"latestPublishedNo,omitempty"`
	DraftVersionID    *string          `json:"draftVersionId,omitempty"`
	Versions          []VersionItemDTO `json:"versions"`
	ClassesUsing      []ClassUsingDTO  `json:"classesUsing"`
}

// CourseListDTO là phản hồi GET /courses.
type CourseListDTO struct {
	Items []CourseListItem `json:"items"`
}

// CourseDetailDTO là phản hồi GET/POST /courses.
type CourseDetailDTO struct {
	ID           string              `json:"id"`
	Code         string              `json:"code"`
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	Versions     []VersionSummaryDTO `json:"versions"`
	ClassesUsing []ClassUsingDTO     `json:"classesUsing"`
}

// VersionStageDTO là một chặng của phiên bản khóa học; outdated khi chặng đã có bản phát hành mới hơn.
type VersionStageDTO struct {
	Position          int    `json:"position"`
	StageID           string `json:"stageId"`
	StageCode         string `json:"stageCode"`
	StageName         string `json:"stageName"`
	StageVersionID    string `json:"stageVersionId"`
	StageVersionNo    int    `json:"stageVersionNo"`
	LessonCount       int    `json:"lessonCount"`
	RequiredCount     int    `json:"requiredCount"`
	LatestPublishedNo *int   `json:"latestPublishedNo,omitempty"`
	Outdated          bool   `json:"outdated"`
}

// UsedByClassDTO là lớp tham chiếu phiên bản khóa học.
type UsedByClassDTO struct {
	ClassID     string `json:"classId"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	MemberCount int    `json:"memberCount"`
}

// CourseVersionDTO là phản hồi của các route /course-versions/{vid}.
type CourseVersionDTO struct {
	ID                  string            `json:"id"`
	CourseID            string            `json:"courseId"`
	CourseCode          string            `json:"courseCode"`
	CourseName          string            `json:"courseName"`
	VersionNo           int               `json:"versionNo"`
	Status              string            `json:"status"`
	PublishedAt         *time.Time        `json:"publishedAt,omitempty"`
	ClonedFromVersionNo *int              `json:"clonedFromVersionNo,omitempty"`
	Stages              []VersionStageDTO `json:"stages"`
	Classes             []UsedByClassDTO  `json:"classes"`
	NewerPublished      bool              `json:"newerPublished"`
}

// DeleteVersionDTO là phản hồi DELETE /course-versions/{vid}.
type DeleteVersionDTO struct {
	CourseDeleted bool `json:"courseDeleted"`
}

// ApplyErrorDTO là lý do một khóa học không áp dụng được.
type ApplyErrorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ApplyResultDTO là kết quả áp dụng cho một khóa học.
type ApplyResultDTO struct {
	CourseID     string         `json:"courseId"`
	CourseCode   string         `json:"courseCode"`
	NewVersionNo *int           `json:"newVersionNo,omitempty"`
	Error        *ApplyErrorDTO `json:"error,omitempty"`
}

// ApplyResultsDTO là phản hồi POST /stage-versions/{vid}/apply (200 kể cả khi có khóa học thất bại).
type ApplyResultsDTO struct {
	Results []ApplyResultDTO `json:"results"`
}

// DraftExistsDetails là details của lỗi DRAFT_EXISTS.
type DraftExistsDetails struct {
	DraftVersionID string `json:"draftVersionId"`
	DraftVersionNo int    `json:"draftVersionNo"`
}

// InUseDetails là details của lỗi IN_USE.
type InUseDetails struct {
	UsedBy []UsedByClassDTO `json:"usedBy"`
}

// createCourseRequest không gắn tag binding: kiểm tra nằm ở service để trả 422 với thông điệp spec.
type createCourseRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type setStagesRequest struct {
	StageVersionIDs []uuid.UUID `json:"stageVersionIds"`
}

type applyRequest struct {
	CourseIDs []uuid.UUID `json:"courseIds"`
}

func versionItemDTO(v VersionListRow) VersionItemDTO {
	return VersionItemDTO{
		ID: v.ID.String(), VersionNo: v.VersionNo.Int(), Status: v.Status.String(), PublishedAt: utcPtr(v.PublishedAt),
		StageCount: v.StageCount, OutdatedStageCount: v.OutdatedStageCount,
	}
}

func classUsingDTOs(in []ClassUsingRow) []ClassUsingDTO {
	out := make([]ClassUsingDTO, 0, len(in))
	for _, c := range in {
		out = append(out, ClassUsingDTO{ClassID: c.ClassID.String(), Code: c.Code, Name: c.Name, VersionNo: c.VersionNo.Int()})
	}
	return out
}

// toListItem suy ra latestVersionNo/latestPublishedNo/draftVersionId từ danh sách phiên bản.
func toListItem(r CourseListRow) CourseListItem {
	item := CourseListItem{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, Description: r.Description,
		Versions: make([]VersionItemDTO, 0, len(r.Versions)), ClassesUsing: classUsingDTOs(r.ClassesUsing),
	}
	for _, v := range r.Versions {
		item.Versions = append(item.Versions, versionItemDTO(v))
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

func toListDTO(rows []CourseListRow) CourseListDTO {
	items := make([]CourseListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, toListItem(r))
	}
	return CourseListDTO{Items: items}
}

func toCourseDetailDTO(d CourseDetail) CourseDetailDTO {
	versions := make([]VersionSummaryDTO, 0, len(d.Versions))
	for _, v := range d.Versions {
		versions = append(versions, VersionSummaryDTO{
			VersionItemDTO: versionItemDTO(v), ClassCount: v.ClassCount, ClonedFromVersionNo: noPtr(v.ClonedFromVersionNo),
		})
	}
	return CourseDetailDTO{
		ID: d.Course.ID.String(), Code: d.Course.Code, Name: d.Course.Name, Description: d.Course.Description,
		Versions: versions, ClassesUsing: classUsingDTOs(d.ClassesUsing),
	}
}

func usedByClassDTOs(in []UsedByClassRow) []UsedByClassDTO {
	out := make([]UsedByClassDTO, 0, len(in))
	for _, c := range in {
		out = append(out, UsedByClassDTO{ClassID: c.ClassID.String(), Code: c.Code, Name: c.Name, Status: c.Status, MemberCount: c.MemberCount})
	}
	return out
}

func toVersionDTO(d VersionDetail) CourseVersionDTO {
	v := d.Version
	stages := make([]VersionStageDTO, 0, len(d.Stages))
	for _, s := range d.Stages {
		stages = append(stages, VersionStageDTO{
			Position: s.Position, StageID: s.StageID.String(), StageCode: s.StageCode, StageName: s.StageName,
			StageVersionID: s.StageVersionID.String(), StageVersionNo: s.StageVersionNo.Int(),
			LessonCount: s.LessonCount, RequiredCount: s.RequiredCount, LatestPublishedNo: noPtr(s.LatestPublishedNo),
			Outdated: s.Outdated(),
		})
	}
	return CourseVersionDTO{
		ID: v.ID().String(), CourseID: v.CourseID().String(), CourseCode: d.Course.Code, CourseName: d.Course.Name,
		VersionNo: v.VersionNo().Int(), Status: v.Status().String(), PublishedAt: utcPtr(v.PublishedAt()),
		ClonedFromVersionNo: noPtr(d.ClonedFromVersionNo), Stages: stages, Classes: usedByClassDTOs(d.Classes),
		NewerPublished: d.NewerPublished,
	}
}

func toApplyResultsDTO(in []ApplyResult) ApplyResultsDTO {
	out := make([]ApplyResultDTO, 0, len(in))
	for _, r := range in {
		dto := ApplyResultDTO{CourseID: r.CourseID.String(), CourseCode: r.CourseCode, NewVersionNo: noPtr(r.NewVersionNo)}
		if r.Error != nil {
			dto.Error = &ApplyErrorDTO{Code: r.Error.Code, Message: r.Error.Message}
		}
		out = append(out, dto)
	}
	return ApplyResultsDTO{Results: out}
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
