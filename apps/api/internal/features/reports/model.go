// Package reports là read model báo cáo: báo cáo tiến độ lớp cho admin/giảng viên (lọc, sắp xếp, drilldown theo
// học liệu) và dashboard của admin. Feature chỉ đọc, không ghi dữ liệu hay audit.
package reports

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/learning"
	"lms/api/internal/platform/audit"
)

// ClassHeader là đầu báo cáo: lớp, khóa học đang dùng và giảng viên phụ trách.
type ClassHeader struct {
	ID              uuid.UUID
	Code            string
	Name            string
	Status          domain.ClassStatus
	CourseVersionID uuid.UUID
	CourseName      string
	CourseVersionNo int
	TeacherID       uuid.UUID
	TeacherName     string
}

// StageHeader là một cột chặng của báo cáo, theo thứ tự trong phiên bản khóa học.
type StageHeader struct {
	StageID       uuid.UUID
	Code          string
	Name          string
	VersionNo     int
	Position      int
	RequiredTotal int
}

// InviteStatus là lời mời mới nhất của thành viên; Status nil khi bản ghi email đã không còn, sending hiển thị
// như queued.
type InviteStatus struct {
	Kind      string
	Status    *string
	Attempts  int
	LastError *string
}

// StagePercent là tiến độ học liệu bắt buộc của thành viên trong một chặng.
type StagePercent struct {
	StageID       uuid.UUID
	Percent       int
	RequiredDone  int
	RequiredTotal int
}

// ReportRow là một dòng báo cáo (một thành viên lớp). LastActivityAt là mốc mới hơn giữa lần dùng ứng dụng gần
// nhất (users.last_active_at) và lần mở/hoàn thành học liệu gần nhất trong lớp.
type ReportRow struct {
	MemberID           uuid.UUID
	UserID             uuid.UUID
	Name               string
	Email              string
	AccountStatus      domain.UserStatus
	MemberStatus       domain.MemberStatus
	MustChangePassword bool
	Invite             *InviteStatus
	StagePercents      []StagePercent
	Percent            int
	RequiredDone       int
	RequiredTotal      int
	LastLoginAt        *time.Time
	LastActivityAt     *time.Time
}

// SummaryQuery là quy tắc đếm của Summary: thành viên active (hoặc mọi thành viên khi IncludeDropped), không hoạt
// động trước InactiveCutoff, % toàn khóa dưới BelowPercent.
type SummaryQuery struct {
	IncludeDropped bool
	InactiveCutoff time.Time
	BelowPercent   int
}

// Summary là số liệu toàn lớp (không theo bộ lọc dòng, trừ IncludeDropped). InactiveDays/BelowPercent là ngưỡng
// đã dùng để đếm InactiveCount/BelowCount: lấy từ bộ lọc, request không gửi thì dùng mặc định.
type Summary struct {
	MemberCount      int
	ActiveCount      int
	AvgPercent       int
	NotLoggedInCount int
	InactiveCount    int
	BelowCount       int
	InactiveDays     int
	BelowPercent     int
}

// defaultBelowPercent là ngưỡng "% toàn khóa dưới" của Summary khi request không lọc theo % (placeholder 50 của
// thanh lọc).
const defaultBelowPercent = 50

// ClassReport là báo cáo tiến độ lớp.
type ClassReport struct {
	Class        ClassHeader
	Stages       []StageHeader
	Rows         []ReportRow
	Summary      Summary
	Filter       Filter
	SelfReported bool
}

// MemberReport là drilldown một thành viên: từng học liệu của phiên bản khóa học với trạng thái và thời điểm.
type MemberReport struct {
	Class        ClassHeader
	Member       ReportRow
	Stages       []learning.StageView
	SelfReported bool
}

// KPIs là bốn ô số của dashboard.
type KPIs struct {
	Stages   int // số chặng
	Courses  int // số khóa học
	Classes  int // lớp đang chạy
	Students int // học viên đang học: thành viên active của lớp đang chạy, mỗi người một lần
}

// Hints là dòng gợi ý dưới các ô số của dashboard.
type Hints struct {
	DraftClasses    int // lớp nháp
	NotLoggedIn     int // học viên đang học chưa đăng nhập lần nào
	OutdatedCourses int // khóa học đang dùng phiên bản chặng cũ
	FailedInvites   int // lời mời mới nhất của thành viên active gửi thất bại
}

// OutdatedRow là một cặp (khóa học, chặng) mà bản published mới nhất của khóa học đang dùng phiên bản chặng cũ.
type OutdatedRow struct {
	CourseID          uuid.UUID
	CourseCode        string
	CourseName        string
	CourseVersionID   uuid.UUID // bản published mới nhất của khóa học, bản đang dùng chặng cũ
	CourseVersionNo   int
	StageID           uuid.UUID
	StageCode         string
	StageName         string
	CurrentVersionNo  int
	LatestVersionID   uuid.UUID // bản published mới nhất của chặng, nơi admin áp dụng
	LatestPublishedNo int
}

// ClassRow là một lớp trong mục "Lớp học" của dashboard.
type ClassRow struct {
	ID              uuid.UUID
	Code            string
	Name            string
	Status          domain.ClassStatus
	CourseName      string
	CourseVersionNo int
	MemberCount     int
	AvgPercent      int
}

// ActivityTarget là đối tượng của một dòng nhật ký; ID nil khi thao tác không gắn đối tượng cụ thể.
type ActivityTarget struct {
	Type  string
	ID    *uuid.UUID
	Label string
}

// ActivityRow là một dòng "Nhật ký thao tác" đã dựng sẵn nhãn tiếng Việt; before/after gốc không ra ngoài.
type ActivityRow struct {
	ID          uuid.UUID
	At          time.Time
	ActorName   string
	ActionLabel string
	Target      ActivityTarget
	Summary     string
}

// Dashboard là trang tổng quan của admin.
type Dashboard struct {
	KPIs           KPIs
	Hints          Hints
	Outdated       []OutdatedRow
	Classes        []ClassRow
	RecentActivity []ActivityRow
}

// recentActivityLimit là số dòng nhật ký trên dashboard.
const recentActivityLimit = 8

// systemActorName là tên hiển thị của thao tác không có người thực hiện (worker) hoặc người đã không còn.
const systemActorName = "Hệ thống"

// actionLabels là nhãn tiếng Việt của từng action audit, nguyên văn nhật ký của prototype.
var actionLabels = map[string]string{
	audit.ActionStageCreated:              "Tạo chặng",
	audit.ActionStageVersionCloned:        "Nhân bản chặng",
	audit.ActionStageVersionPublished:     "Phát hành chặng",
	audit.ActionStageVersionArchived:      "Lưu trữ chặng",
	audit.ActionStageVersionDeleted:       "Xóa phiên bản chặng",
	audit.ActionCourseCreated:             "Tạo khóa học",
	audit.ActionCourseVersionCloned:       "Nhân bản khóa học",
	audit.ActionCourseVersionPublished:    "Phát hành khóa học",
	audit.ActionCourseVersionArchived:     "Lưu trữ khóa học",
	audit.ActionCourseVersionDeleted:      "Xóa phiên bản khóa học",
	audit.ActionCourseStageVersionApplied: "Áp dụng chặng cho khóa học",
	audit.ActionClassCreated:              "Tạo lớp",
	audit.ActionClassCourseVersionChanged: "Đổi phiên bản khóa học của lớp",
	audit.ActionClassActivated:            "Kích hoạt lớp",
	audit.ActionClassEnded:                "Kết thúc lớp",
	audit.ActionClassMemberInvited:        "Mời học viên",
	audit.ActionClassInvitationResent:     "Gửi lại lời mời",
	audit.ActionClassMemberDropped:        "Gỡ học viên khỏi lớp",
	audit.ActionUserDisabled:              "Vô hiệu hóa tài khoản",
	audit.ActionUserEnabled:               "Kích hoạt lại tài khoản",
}

// actionLabel trả nhãn của action; action chưa có nhãn trả chính nó và ok=false để nơi gọi log cảnh báo.
func actionLabel(action string) (string, bool) {
	if l, ok := actionLabels[action]; ok {
		return l, true
	}
	return action, false
}

// Loại đối tượng audit (audit_logs.target_type) mà nhật ký tra được tên.
const (
	targetClass         = "class"
	targetStage         = "stage"
	targetStageVersion  = "stage_version"
	targetCourse        = "course"
	targetCourseVersion = "course_version"
	targetUser          = "user"
)

// TargetRef là một đối tượng cần tra tên cho nhật ký.
type TargetRef struct {
	Type string
	ID   uuid.UUID
}

// TargetInfo là tên đã tra của một đối tượng: mã lớp, tên chặng/khóa học; VersionNo > 0 với phiên bản.
type TargetInfo struct {
	Name      string
	VersionNo int
}

// Label là "Tên" hoặc "Tên vN" với phiên bản.
func (t TargetInfo) Label() string {
	if t.VersionNo > 0 {
		return fmt.Sprintf("%s v%d", t.Name, t.VersionNo)
	}
	return t.Name
}

// UserRef là tên và email của user mà nhật ký nhắc tới (người thao tác, học viên được mời, tài khoản bị khóa).
type UserRef struct {
	Name  string
	Email string
}

// activityLookups là kết quả tra tên theo lô cho một trang nhật ký.
type activityLookups struct {
	Targets map[TargetRef]TargetInfo
	Users   map[uuid.UUID]UserRef
}

// activityPayload là các khóa before/after mà feature (và seed) ghi; khóa vắng để giá trị rỗng.
type activityPayload struct {
	Code             string     `json:"code"`
	Name             string     `json:"name"`
	ClassCode        string     `json:"classCode"`
	Email            string     `json:"email"`
	Count            int        `json:"count"`
	VersionNo        int        `json:"versionNo"`
	FromVersionNo    int        `json:"fromVersionNo"`
	ToVersionNo      int        `json:"toVersionNo"`
	StageID          *uuid.UUID `json:"stageId"`
	CourseID         *uuid.UUID `json:"courseId"`
	UserID           *uuid.UUID `json:"userId"`
	CourseVersionID  *uuid.UUID `json:"courseVersionId"`
	ToStageVersionID *uuid.UUID `json:"toStageVersionId"`
}

// parsePayload đọc jsonb before/after; NULL hoặc JSON không phải object cho payload rỗng (nhật ký vẫn hiển thị
// được bằng tên đã tra).
func parsePayload(raw json.RawMessage) activityPayload {
	var p activityPayload
	if len(raw) == 0 {
		return p
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return activityPayload{}
	}
	return p
}

// activityRefs liệt kê đối tượng và user cần tra để dựng các dòng nhật ký entries.
func activityRefs(entries []audit.LoggedEntry) ([]TargetRef, []uuid.UUID) {
	var refs []TargetRef
	var users []uuid.UUID
	seenRef := map[TargetRef]bool{}
	seenUser := map[uuid.UUID]bool{}
	addRef := func(typ string, id *uuid.UUID) {
		if id == nil {
			return
		}
		r := TargetRef{Type: typ, ID: *id}
		if !seenRef[r] {
			seenRef[r] = true
			refs = append(refs, r)
		}
	}
	addUser := func(id *uuid.UUID) {
		if id != nil && !seenUser[*id] {
			seenUser[*id] = true
			users = append(users, *id)
		}
	}
	for _, e := range entries {
		addUser(e.ActorID)
		before, after := parsePayload(e.Before), parsePayload(e.After)
		switch e.TargetType {
		case targetUser:
			addUser(e.TargetID)
		case targetClass, targetStage, targetStageVersion, targetCourse, targetCourseVersion:
			addRef(e.TargetType, e.TargetID)
		}
		addUser(after.UserID)
		addRef(targetStage, after.StageID)
		addRef(targetCourse, after.CourseID)
		addRef(targetStageVersion, after.ToStageVersionID)
		addRef(targetCourseVersion, before.CourseVersionID)
		addRef(targetCourseVersion, after.CourseVersionID)
	}
	return refs, users
}

// describeActivity dựng một dòng nhật ký từ bản ghi audit và kết quả tra tên.
func describeActivity(e audit.LoggedEntry, lk activityLookups) ActivityRow {
	label, _ := actionLabel(e.Action)
	actor := systemActorName
	if e.ActorID != nil {
		if u, ok := lk.Users[*e.ActorID]; ok {
			actor = u.Name
		}
	}
	before, after := parsePayload(e.Before), parsePayload(e.After)
	target := targetLabel(e, after, lk)
	return ActivityRow{
		ID: e.ID, At: e.At, ActorName: actor, ActionLabel: label,
		Target:  ActivityTarget{Type: e.TargetType, ID: e.TargetID, Label: target},
		Summary: summarize(e.Action, before, after, target, lk),
	}
}

// lookupTarget tra tên đối tượng (typ, id); id nil hoặc không còn tồn tại → ok=false.
func (lk activityLookups) lookupTarget(typ string, id *uuid.UUID) (TargetInfo, bool) {
	if id == nil {
		return TargetInfo{}, false
	}
	t, ok := lk.Targets[TargetRef{Type: typ, ID: *id}]
	return t, ok
}

// userEmail là email của user id, hoặc fallback (email seed ghi sẵn trong payload) khi không tra được.
func (lk activityLookups) userEmail(id *uuid.UUID, fallback string) string {
	if id != nil {
		if u, ok := lk.Users[*id]; ok {
			return u.Email
		}
	}
	return fallback
}

// targetLabel là nhãn đối tượng: mã lớp, "Tên chặng vN", email tài khoản… Đối tượng đã bị xóa (phiên bản nháp
// đã xóa) dựng lại từ payload: tên chặng/khóa học theo stageId/courseId và versionNo.
func targetLabel(e audit.LoggedEntry, after activityPayload, lk activityLookups) string {
	if e.TargetType == targetUser {
		return lk.userEmail(e.TargetID, after.Email)
	}
	if t, ok := lk.lookupTarget(e.TargetType, e.TargetID); ok {
		return t.Label()
	}
	switch e.TargetType {
	case targetClass:
		return after.ClassCode
	case targetStage, targetCourse:
		return after.Name
	case targetStageVersion, targetCourseVersion:
		name := subjectName(e.TargetType, after, lk)
		if after.VersionNo > 0 {
			return fmt.Sprintf("%s v%d", name, after.VersionNo)
		}
		return name
	}
	return ""
}

// subjectName là tên chặng/khóa học của một phiên bản: theo đối tượng nếu còn, nếu không theo stageId/courseId.
func subjectName(targetType string, after activityPayload, lk activityLookups) string {
	if targetType == targetStageVersion {
		if t, ok := lk.lookupTarget(targetStage, after.StageID); ok {
			return t.Name
		}
		return ""
	}
	if t, ok := lk.lookupTarget(targetCourse, after.CourseID); ok {
		return t.Name
	}
	return ""
}

// summarize dựng tóm tắt một dòng nhật ký từ before/after đã đọc và nhãn đối tượng, theo mẫu nhật ký prototype:
// "Database (DB)", "Database v1 → v2", "Lập trình cơ bản v2", "Database v2 → Lập trình cơ bản v2",
// "basic01: v1 → v2", "basic01: mời an.nguyen@gmail.com". Action không có mẫu riêng dùng nhãn đối tượng.
func summarize(action string, before, after activityPayload, target string, lk activityLookups) string {
	switch action {
	case audit.ActionStageCreated, audit.ActionCourseCreated:
		if after.Name != "" && after.Code != "" {
			return fmt.Sprintf("%s (%s)", after.Name, after.Code)
		}
	case audit.ActionStageVersionCloned, audit.ActionCourseVersionCloned:
		if after.FromVersionNo > 0 && after.ToVersionNo > 0 {
			return fmt.Sprintf("%s v%d → v%d", versionSubject(action, after, lk), after.FromVersionNo, after.ToVersionNo)
		}
	case audit.ActionCourseStageVersionApplied:
		stage, ok := lk.lookupTarget(targetStageVersion, after.ToStageVersionID)
		if ok && after.ToVersionNo > 0 {
			return fmt.Sprintf("%s → %s v%d", stage.Label(), target, after.ToVersionNo)
		}
	case audit.ActionClassCourseVersionChanged:
		from, okFrom := lk.lookupTarget(targetCourseVersion, before.CourseVersionID)
		to, okTo := lk.lookupTarget(targetCourseVersion, after.CourseVersionID)
		if okFrom && okTo {
			return fmt.Sprintf("%s: v%d → v%d", target, from.VersionNo, to.VersionNo)
		}
	case audit.ActionClassMemberInvited:
		if after.Count > 1 {
			return fmt.Sprintf("%s: mời %d học viên", target, after.Count)
		}
		if email := lk.userEmail(after.UserID, after.Email); email != "" {
			return fmt.Sprintf("%s: mời %s", target, email)
		}
	case audit.ActionClassInvitationResent:
		if email := lk.userEmail(after.UserID, after.Email); email != "" {
			return fmt.Sprintf("%s: gửi lại cho %s", target, email)
		}
	case audit.ActionClassMemberDropped:
		if email := lk.userEmail(after.UserID, after.Email); email != "" {
			return fmt.Sprintf("%s: gỡ %s", target, email)
		}
	}
	return target
}

// versionSubject là tên chặng/khóa học của action nhân bản: target là bản mới ("Database v2") nên lấy tên
// không kèm số phiên bản.
func versionSubject(action string, after activityPayload, lk activityLookups) string {
	typ := targetCourseVersion
	if action == audit.ActionStageVersionCloned {
		typ = targetStageVersion
	}
	return subjectName(typ, after, lk)
}
