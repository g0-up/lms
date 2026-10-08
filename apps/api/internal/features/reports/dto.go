package reports

import (
	"time"

	"lms/api/internal/features/learning"
)

// TeacherDTO là giảng viên phụ trách lớp.
type TeacherDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ClassHeaderDTO là đầu báo cáo lớp.
type ClassHeaderDTO struct {
	ID              string     `json:"id"`
	Code            string     `json:"code"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	CourseName      string     `json:"courseName"`
	CourseVersionNo int        `json:"courseVersionNo"`
	Teacher         TeacherDTO `json:"teacher"`
}

// StageHeaderDTO là một cột chặng của báo cáo.
type StageHeaderDTO struct {
	StageID       string `json:"stageId"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	VersionNo     int    `json:"versionNo"`
	Position      int    `json:"position"`
	RequiredTotal int    `json:"requiredTotal"`
}

// InviteDTO là lời mời mới nhất; status null khi không còn bản ghi email, lastError null khi chưa lỗi.
type InviteDTO struct {
	Kind      string  `json:"kind"`
	Status    *string `json:"status"`
	Attempts  int     `json:"attempts"`
	LastError *string `json:"lastError"`
}

// StagePercentDTO là tiến độ một chặng của dòng báo cáo.
type StagePercentDTO struct {
	StageID       string `json:"stageId"`
	Percent       int    `json:"percent"`
	RequiredDone  int    `json:"requiredDone"`
	RequiredTotal int    `json:"requiredTotal"`
}

// ReportRowDTO là một dòng báo cáo; invite vắng khi thành viên chưa có lời mời, lastLoginAt/lastActivityAt null
// khi chưa có.
type ReportRowDTO struct {
	MemberID           string            `json:"memberId"`
	UserID             string            `json:"userId"`
	Name               string            `json:"name"`
	Email              string            `json:"email"`
	AccountStatus      string            `json:"accountStatus"`
	MemberStatus       string            `json:"memberStatus"`
	MustChangePassword bool              `json:"mustChangePassword"`
	Invite             *InviteDTO        `json:"invite,omitempty"`
	StagePercents      []StagePercentDTO `json:"stagePercents"`
	Percent            int               `json:"percent"`
	RequiredDone       int               `json:"requiredDone"`
	RequiredTotal      int               `json:"requiredTotal"`
	LastLoginAt        *time.Time        `json:"lastLoginAt"`
	LastActivityAt     *time.Time        `json:"lastActivityAt"`
}

// SummaryDTO là số liệu toàn lớp; inactiveDays/belowPercent là ngưỡng đã dùng để đếm inactiveCount/belowCount.
type SummaryDTO struct {
	MemberCount      int `json:"memberCount"`
	ActiveCount      int `json:"activeCount"`
	AvgPercent       int `json:"avgPercent"`
	NotLoggedInCount int `json:"notLoggedInCount"`
	InactiveCount    int `json:"inactiveCount"`
	BelowCount       int `json:"belowCount"`
	InactiveDays     int `json:"inactiveDays"`
	BelowPercent     int `json:"belowPercent"`
}

// FilterDTO là bộ lọc đã áp dụng; inactiveDays/belowPercent null khi không lọc.
type FilterDTO struct {
	NotLoggedIn    bool   `json:"notLoggedIn"`
	InactiveDays   *int   `json:"inactiveDays"`
	BelowPercent   *int   `json:"belowPercent"`
	IncludeDropped bool   `json:"includeDropped"`
	Sort           string `json:"sort"`
}

// ClassReportDTO là phản hồi GET /classes/{id}/report.
type ClassReportDTO struct {
	Class        ClassHeaderDTO   `json:"class"`
	Stages       []StageHeaderDTO `json:"stages"`
	Rows         []ReportRowDTO   `json:"rows"`
	Summary      SummaryDTO       `json:"summary"`
	Filter       FilterDTO        `json:"filter"`
	SelfReported bool             `json:"selfReported"`
}

// LessonProgressDTO là một học liệu trong drilldown; firstOpenedAt/completedAt null khi chưa mở/chưa hoàn thành.
type LessonProgressDTO struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Type          string     `json:"type"`
	Required      bool       `json:"required"`
	Position      int        `json:"position"`
	State         string     `json:"state"`
	FirstOpenedAt *time.Time `json:"firstOpenedAt"`
	CompletedAt   *time.Time `json:"completedAt"`
}

// StageProgressDTO là một chặng trong drilldown.
type StageProgressDTO struct {
	StageID   string              `json:"stageId"`
	Code      string              `json:"code"`
	Name      string              `json:"name"`
	VersionNo int                 `json:"versionNo"`
	Percent   int                 `json:"percent"`
	Lessons   []LessonProgressDTO `json:"lessons"`
}

// MemberReportDTO là phản hồi GET /classes/{id}/report/members/{mid}.
type MemberReportDTO struct {
	Class        ClassHeaderDTO     `json:"class"`
	Member       ReportRowDTO       `json:"member"`
	Stages       []StageProgressDTO `json:"stages"`
	SelfReported bool               `json:"selfReported"`
}

// KPIsDTO là bốn ô số của dashboard.
type KPIsDTO struct {
	Stages   int `json:"stages"`
	Courses  int `json:"courses"`
	Classes  int `json:"classes"`
	Students int `json:"students"`
}

// HintsDTO là gợi ý dưới các ô số.
type HintsDTO struct {
	DraftClasses    int `json:"draftClasses"`
	NotLoggedIn     int `json:"notLoggedIn"`
	OutdatedCourses int `json:"outdatedCourses"`
	FailedInvites   int `json:"failedInvites"`
}

// OutdatedDTO là một khóa học đang dùng phiên bản chặng cũ.
type OutdatedDTO struct {
	CourseID          string `json:"courseId"`
	CourseCode        string `json:"courseCode"`
	CourseName        string `json:"courseName"`
	CourseVersionID   string `json:"courseVersionId"`
	CourseVersionNo   int    `json:"courseVersionNo"`
	StageID           string `json:"stageId"`
	StageCode         string `json:"stageCode"`
	StageName         string `json:"stageName"`
	CurrentVersionNo  int    `json:"currentVersionNo"`
	LatestVersionID   string `json:"latestVersionId"`
	LatestPublishedNo int    `json:"latestPublishedNo"`
}

// ClassRowDTO là một lớp trên dashboard.
type ClassRowDTO struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	CourseName      string `json:"courseName"`
	CourseVersionNo int    `json:"courseVersionNo"`
	MemberCount     int    `json:"memberCount"`
	AvgPercent      int    `json:"avgPercent"`
}

// ActivityTargetDTO là đối tượng của dòng nhật ký; id null khi không gắn đối tượng.
type ActivityTargetDTO struct {
	Type  string  `json:"type"`
	ID    *string `json:"id"`
	Label string  `json:"label"`
}

// ActivityDTO là một dòng nhật ký thao tác.
type ActivityDTO struct {
	ID          string            `json:"id"`
	At          time.Time         `json:"at"`
	ActorName   string            `json:"actorName"`
	ActionLabel string            `json:"actionLabel"`
	Target      ActivityTargetDTO `json:"target"`
	Summary     string            `json:"summary"`
}

// DashboardDTO là phản hồi GET /dashboard.
type DashboardDTO struct {
	KPIs           KPIsDTO       `json:"kpis"`
	Hints          HintsDTO      `json:"hints"`
	Outdated       []OutdatedDTO `json:"outdated"`
	Classes        []ClassRowDTO `json:"classes"`
	RecentActivity []ActivityDTO `json:"recentActivity"`
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}

func classHeaderDTO(h ClassHeader) ClassHeaderDTO {
	return ClassHeaderDTO{
		ID: h.ID.String(), Code: h.Code, Name: h.Name, Status: string(h.Status), CourseName: h.CourseName,
		CourseVersionNo: h.CourseVersionNo, Teacher: TeacherDTO{ID: h.TeacherID.String(), Name: h.TeacherName},
	}
}

func reportRowDTO(r ReportRow) ReportRowDTO {
	out := ReportRowDTO{
		MemberID: r.MemberID.String(), UserID: r.UserID.String(), Name: r.Name, Email: r.Email,
		AccountStatus: string(r.AccountStatus), MemberStatus: string(r.MemberStatus), MustChangePassword: r.MustChangePassword,
		StagePercents: make([]StagePercentDTO, len(r.StagePercents)), Percent: r.Percent, RequiredDone: r.RequiredDone,
		RequiredTotal: r.RequiredTotal, LastLoginAt: utc(r.LastLoginAt), LastActivityAt: utc(r.LastActivityAt),
	}
	if r.Invite != nil {
		out.Invite = &InviteDTO{Kind: r.Invite.Kind, Status: r.Invite.Status, Attempts: r.Invite.Attempts, LastError: r.Invite.LastError}
	}
	for i, sp := range r.StagePercents {
		out.StagePercents[i] = StagePercentDTO{
			StageID: sp.StageID.String(), Percent: sp.Percent, RequiredDone: sp.RequiredDone, RequiredTotal: sp.RequiredTotal,
		}
	}
	return out
}

func toClassReportDTO(r ClassReport) ClassReportDTO {
	out := ClassReportDTO{
		Class:  classHeaderDTO(r.Class),
		Stages: make([]StageHeaderDTO, len(r.Stages)),
		Rows:   make([]ReportRowDTO, len(r.Rows)),
		Summary: SummaryDTO{
			MemberCount: r.Summary.MemberCount, ActiveCount: r.Summary.ActiveCount, AvgPercent: r.Summary.AvgPercent,
			NotLoggedInCount: r.Summary.NotLoggedInCount, InactiveCount: r.Summary.InactiveCount,
			BelowCount: r.Summary.BelowCount, InactiveDays: r.Summary.InactiveDays, BelowPercent: r.Summary.BelowPercent,
		},
		Filter: FilterDTO{
			NotLoggedIn: r.Filter.NotLoggedIn, InactiveDays: r.Filter.InactiveDays, BelowPercent: r.Filter.BelowPercent,
			IncludeDropped: r.Filter.IncludeDropped, Sort: string(r.Filter.Sort),
		},
		SelfReported: r.SelfReported,
	}
	for i, s := range r.Stages {
		out.Stages[i] = StageHeaderDTO{
			StageID: s.StageID.String(), Code: s.Code, Name: s.Name, VersionNo: s.VersionNo, Position: s.Position,
			RequiredTotal: s.RequiredTotal,
		}
	}
	for i, row := range r.Rows {
		out.Rows[i] = reportRowDTO(row)
	}
	return out
}

func stageProgressDTO(st learning.StageView) StageProgressDTO {
	out := StageProgressDTO{
		StageID: st.StageID.String(), Code: st.Code, Name: st.Name, VersionNo: int(st.VersionNo), Percent: st.Percent,
		Lessons: make([]LessonProgressDTO, len(st.Lessons)),
	}
	for i, l := range st.Lessons {
		out.Lessons[i] = LessonProgressDTO{
			ID: l.ID.String(), Title: l.Title, Type: string(l.Type), Required: l.Required, Position: l.Position,
			State: string(l.State), FirstOpenedAt: utc(l.FirstOpenedAt), CompletedAt: utc(l.CompletedAt),
		}
	}
	return out
}

func toMemberReportDTO(r MemberReport) MemberReportDTO {
	out := MemberReportDTO{
		Class: classHeaderDTO(r.Class), Member: reportRowDTO(r.Member),
		Stages: make([]StageProgressDTO, len(r.Stages)), SelfReported: r.SelfReported,
	}
	for i, st := range r.Stages {
		out.Stages[i] = stageProgressDTO(st)
	}
	return out
}

func toDashboardDTO(d Dashboard) DashboardDTO {
	out := DashboardDTO{
		KPIs: KPIsDTO{Stages: d.KPIs.Stages, Courses: d.KPIs.Courses, Classes: d.KPIs.Classes, Students: d.KPIs.Students},
		Hints: HintsDTO{
			DraftClasses: d.Hints.DraftClasses, NotLoggedIn: d.Hints.NotLoggedIn,
			OutdatedCourses: d.Hints.OutdatedCourses, FailedInvites: d.Hints.FailedInvites,
		},
		Outdated:       make([]OutdatedDTO, len(d.Outdated)),
		Classes:        make([]ClassRowDTO, len(d.Classes)),
		RecentActivity: make([]ActivityDTO, len(d.RecentActivity)),
	}
	for i, o := range d.Outdated {
		out.Outdated[i] = OutdatedDTO{
			CourseID: o.CourseID.String(), CourseCode: o.CourseCode, CourseName: o.CourseName,
			CourseVersionID: o.CourseVersionID.String(), CourseVersionNo: o.CourseVersionNo,
			StageID: o.StageID.String(), StageCode: o.StageCode, StageName: o.StageName,
			CurrentVersionNo: o.CurrentVersionNo, LatestVersionID: o.LatestVersionID.String(),
			LatestPublishedNo: o.LatestPublishedNo,
		}
	}
	for i, c := range d.Classes {
		out.Classes[i] = ClassRowDTO{
			ID: c.ID.String(), Code: c.Code, Name: c.Name, Status: string(c.Status), CourseName: c.CourseName,
			CourseVersionNo: c.CourseVersionNo, MemberCount: c.MemberCount, AvgPercent: c.AvgPercent,
		}
	}
	for i, a := range d.RecentActivity {
		var id *string
		if a.Target.ID != nil {
			s := a.Target.ID.String()
			id = &s
		}
		out.RecentActivity[i] = ActivityDTO{
			ID: a.ID.String(), At: a.At.UTC(), ActorName: a.ActorName, ActionLabel: a.ActionLabel,
			Target: ActivityTargetDTO{Type: a.Target.Type, ID: id, Label: a.Target.Label}, Summary: a.Summary,
		}
	}
	return out
}
