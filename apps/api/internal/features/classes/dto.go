package classes

import (
	"time"
)

// createClassRequest là body POST /classes. Không dùng binding:"required" để thiếu field trả 422 với message nghiệp vụ.
type createClassRequest struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	CourseVersionID string `json:"courseVersionId"`
	StartDate       string `json:"startDate"`
	EndDate         string `json:"endDate"`
	TeacherID       string `json:"teacherId"`
}

// updateClassRequest là body PATCH /classes/{id}; field vắng = giữ nguyên.
type updateClassRequest struct {
	Name            *string `json:"name"`
	StartDate       *string `json:"startDate"`
	EndDate         *string `json:"endDate"`
	TeacherID       *string `json:"teacherId"`
	CourseVersionID *string `json:"courseVersionId"`
}

// inviteRequest là body POST /classes/{id}/invitations.
type inviteRequest struct {
	Email    string `json:"email"`
	FullName string `json:"fullName"`
}

// ClassListItem là một lớp trên GET /classes.
type ClassListItem struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	CourseName      string `json:"courseName"`
	CourseVersionNo int    `json:"courseVersionNo"`
	TeacherName     string `json:"teacherName"`
	StartDate       string `json:"startDate"`
	EndDate         string `json:"endDate"`
	MemberCount     int    `json:"memberCount"`
}

// TeachingClassItem là một lớp trên GET /teach/classes. AvgPercent null khi chưa có số liệu tiến độ.
type TeachingClassItem struct {
	ID                string `json:"id"`
	Code              string `json:"code"`
	Name              string `json:"name"`
	Status            string `json:"status"`
	StartDate         string `json:"startDate"`
	EndDate           string `json:"endDate"`
	CourseName        string `json:"courseName"`
	CourseVersionNo   int    `json:"courseVersionNo"`
	MemberCount       int    `json:"memberCount"`
	AvgPercent        *int   `json:"avgPercent"`
	NotLoggedIn       int    `json:"notLoggedIn"`
	InactiveOver7Days int    `json:"inactiveOver7Days"`
}

// ClassTeacherDTO là giảng viên phụ trách trên chi tiết lớp.
type ClassTeacherDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ClassCourseVersionDTO là phiên bản khóa học của lớp.
type ClassCourseVersionDTO struct {
	ID          string `json:"id"`
	CourseID    string `json:"courseId"`
	CourseName  string `json:"courseName"`
	VersionNo   int    `json:"versionNo"`
	StageCount  int    `json:"stageCount"`
	LessonCount int    `json:"lessonCount"`
}

// ClassDetailDTO là chi tiết lớp (GET/POST/PATCH /classes, activate, end). activatedAt/endedAt null khi chưa có.
type ClassDetailDTO struct {
	ID            string                `json:"id"`
	Code          string                `json:"code"`
	Name          string                `json:"name"`
	Status        string                `json:"status"`
	StartDate     string                `json:"startDate"`
	EndDate       string                `json:"endDate"`
	Teacher       ClassTeacherDTO       `json:"teacher"`
	CourseVersion ClassCourseVersionDTO `json:"courseVersion"`
	MemberCount   int                   `json:"memberCount"`
	ActivatedAt   *time.Time            `json:"activatedAt"`
	EndedAt       *time.Time            `json:"endedAt"`
}

// MemberDTO là một thành viên lớp. Field invite* vắng khi thành viên chưa có lời mời; tempPasswordExpiresAt chỉ có
// khi học viên chưa đổi mật khẩu. Không có field nào chứa mật khẩu.
type MemberDTO struct {
	ID                    string     `json:"id"`
	UserID                string     `json:"userId"`
	Email                 string     `json:"email"`
	FullName              string     `json:"fullName"`
	AccountStatus         string     `json:"accountStatus"`
	MemberStatus          string     `json:"memberStatus"`
	TempPasswordExpiresAt *time.Time `json:"tempPasswordExpiresAt,omitempty"`
	InviteStatus          *string    `json:"inviteStatus,omitempty"`
	InviteKind            *string    `json:"inviteKind,omitempty"`
	InviteAttempts        *int       `json:"inviteAttempts,omitempty"`
	InviteLastError       *string    `json:"inviteLastError,omitempty"`
	InvitedAt             *time.Time `json:"invitedAt,omitempty"`
	LastLoginAt           *time.Time `json:"lastLoginAt,omitempty"`
	LastActiveAt          *time.Time `json:"lastActiveAt,omitempty"`
	JoinedAt              time.Time  `json:"joinedAt"`
	DroppedAt             *time.Time `json:"droppedAt,omitempty"`
}

// ListResponse là envelope danh sách {items: [...]}; items không bao giờ null.
type ListResponse[T any] struct {
	Items []T `json:"items"`
}

// InviteResponse là kết quả POST /classes/{id}/invitations.
type InviteResponse struct {
	Kind   string    `json:"kind"`
	Member MemberDTO `json:"member"`
}

// MemberResponse là kết quả POST …/resend.
type MemberResponse struct {
	Member MemberDTO `json:"member"`
}

func toClassListItems(rows []ClassListRow) ListResponse[ClassListItem] {
	items := make([]ClassListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, ClassListItem{
			ID: r.ID.String(), Code: r.Code, Name: r.Name, Status: r.Status.String(), CourseName: r.CourseName,
			CourseVersionNo: r.CourseVersionNo, TeacherName: r.TeacherName,
			StartDate: FormatDate(r.StartDate), EndDate: FormatDate(r.EndDate), MemberCount: r.MemberCount,
		})
	}
	return ListResponse[ClassListItem]{Items: items}
}

func toTeachingItems(rows []TeachingClassRow) ListResponse[TeachingClassItem] {
	items := make([]TeachingClassItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, TeachingClassItem{
			ID: r.ID.String(), Code: r.Code, Name: r.Name, Status: r.Status.String(),
			StartDate: FormatDate(r.StartDate), EndDate: FormatDate(r.EndDate),
			CourseName: r.CourseName, CourseVersionNo: r.CourseVersionNo, MemberCount: r.MemberCount,
			AvgPercent: r.AvgPercent, NotLoggedIn: r.NotLoggedIn, InactiveOver7Days: r.InactiveCount,
		})
	}
	return ListResponse[TeachingClassItem]{Items: items}
}

func toClassDetailDTO(d *ClassDetail) ClassDetailDTO {
	return ClassDetailDTO{
		ID: d.ID.String(), Code: d.Code, Name: d.Name, Status: d.Status.String(),
		StartDate: FormatDate(d.StartDate), EndDate: FormatDate(d.EndDate),
		Teacher: ClassTeacherDTO{ID: d.TeacherID.String(), Name: d.TeacherName},
		CourseVersion: ClassCourseVersionDTO{
			ID: d.CourseVersionID.String(), CourseID: d.CourseID.String(), CourseName: d.CourseName, VersionNo: d.VersionNo,
			StageCount: d.StageCount, LessonCount: d.LessonCount,
		},
		MemberCount: d.MemberCount, ActivatedAt: utcPtr(d.ActivatedAt), EndedAt: utcPtr(d.EndedAt),
	}
}

func toMemberDTO(r MemberRow) MemberDTO {
	dto := MemberDTO{
		ID: r.ID.String(), UserID: r.UserID.String(), Email: r.Email, FullName: r.FullName,
		AccountStatus: r.AccountStatus.String(), MemberStatus: r.MemberStatus.String(),
		InviteStatus: r.InviteStatus, InviteAttempts: r.InviteAttempts, InviteLastError: r.InviteLastError,
		InvitedAt: utcPtr(r.InvitedAt), LastLoginAt: utcPtr(r.LastLoginAt), LastActiveAt: utcPtr(r.LastActiveAt),
		JoinedAt: r.JoinedAt.UTC(), DroppedAt: utcPtr(r.DroppedAt),
	}
	if r.MustChangePassword {
		dto.TempPasswordExpiresAt = utcPtr(r.TempPasswordExpiresAt)
	}
	if r.InviteKind != nil {
		k := string(*r.InviteKind)
		dto.InviteKind = &k
	}
	return dto
}

func toMemberItems(rows []MemberRow) ListResponse[MemberDTO] {
	items := make([]MemberDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, toMemberDTO(r))
	}
	return ListResponse[MemberDTO]{Items: items}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
