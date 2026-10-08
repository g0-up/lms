package seed

import (
	"time"
)

// Dữ liệu mẫu chép từ prototype/seed.js: khóa học "BASIC" v1 dựng từ năm chặng đã phát hành, lớp basic01 và basic02
// đang chạy trên nó, basic03 còn nháp. Mọi mốc thời gian tương đối so với thời điểm seed, giống seed.js.
// Khóa chuỗi (u-admin, sv-db-1, ...) chỉ dùng để nối dữ liệu trong package; id thật là uuid v7 sinh lúc seed.

// offset là khoảng cách so với thời điểm seed, tính bằng giờ; âm là quá khứ.
type offset float64

func ago(days, hours float64) offset { return offset(-(days*24 + hours)) }
func ahead(days float64) offset      { return offset(days * 24) }
func opt(o offset) *offset           { return &o }

func (o offset) at(now time.Time) time.Time {
	return now.Add(time.Duration(float64(o) * float64(time.Hour)))
}

// Mã email outbox của lời mời theo seed.js.
const (
	inviteSent   = "sent"
	inviteFailed = "failed"
	inviteQueued = "queued"
)

// failedInviteError là lỗi SMTP của lời mời thất bại trong seed.js.
const failedInviteError = "Mailbox không tồn tại (550 5.1.1)"

// adminKey là người thực hiện mọi lời mời và hành động audit trong seed.js.
const adminKey = "u-admin"

type userSeed struct {
	key, role, name, email, status string
	mustChangePassword             bool
	lastLoginAt, lastActiveAt      *offset
	tempPasswordExpiresAt          *offset
}

var users = []userSeed{
	{key: "u-admin", role: "admin", name: "Trần Minh Quân", email: "quan.tran@goup.vn", status: "active", lastLoginAt: opt(ago(0, 2)), lastActiveAt: opt(ago(0, 1))},
	{key: "u-gv", role: "teacher", name: "Lê Thu Hương", email: "huong.le@goup.vn", status: "active", lastLoginAt: opt(ago(1, 0)), lastActiveAt: opt(ago(1, 0))},
	{key: "u-gv2", role: "teacher", name: "Phạm Quốc Bảo", email: "bao.pham@goup.vn", status: "active", lastLoginAt: opt(ago(3, 0)), lastActiveAt: opt(ago(3, 0))},
	{key: "u-an", role: "student", name: "Nguyễn Hoàng An", email: "an.nguyen@gmail.com", status: "active", lastLoginAt: opt(ago(0, 5)), lastActiveAt: opt(ago(0, 4))},
	{key: "u-bich", role: "student", name: "Trần Thị Bích", email: "bich.tran@gmail.com", status: "active", lastLoginAt: opt(ago(2, 0)), lastActiveAt: opt(ago(2, 0))},
	{key: "u-cuong", role: "student", name: "Lê Văn Cường", email: "cuong.le@outlook.com", status: "active", lastLoginAt: opt(ago(1, 0)), lastActiveAt: opt(ago(1, 0))},
	{key: "u-dung", role: "student", name: "Phạm Minh Dũng", email: "dung.pham@gmail.com", status: "invited", mustChangePassword: true, tempPasswordExpiresAt: opt(ago(55, 0))},
	{key: "u-ha", role: "student", name: "Hoàng Thu Hà", email: "ha.hoang@gmail.com", status: "active", lastLoginAt: opt(ago(4, 0)), lastActiveAt: opt(ago(4, 0))},
	{key: "u-khang", role: "student", name: "Vũ Đức Khang", email: "khang.vu@gmail.com", status: "active", lastLoginAt: opt(ago(21, 0)), lastActiveAt: opt(ago(21, 0))},
	{key: "u-phong", role: "student", name: "Đặng Hải Phong", email: "phong.dang@gmail.com", status: "active", lastLoginAt: opt(ago(0, 9)), lastActiveAt: opt(ago(0, 8))},
	{key: "u-linh", role: "student", name: "Đỗ Ngọc Linh", email: "linh.do@gmail.com", status: "active", lastLoginAt: opt(ago(1, 0)), lastActiveAt: opt(ago(1, 0))},
	{key: "u-minh", role: "student", name: "Bùi Quang Minh", email: "minh.bui@gmail.com", status: "invited", mustChangePassword: true, tempPasswordExpiresAt: opt(ahead(2))},
	{key: "u-nhan", role: "student", name: "Ngô Thanh Nhàn", email: "nhan.ngo@gmail.com", status: "active", lastLoginAt: opt(ago(6, 0)), lastActiveAt: opt(ago(6, 0))},
	{key: "u-quyen", role: "student", name: "Lý Mai Quyên", email: "quyen.ly@gmail.com", status: "invited", mustChangePassword: true, tempPasswordExpiresAt: opt(ahead(1.5))},
	{key: "u-thao", role: "student", name: "Võ Phương Thảo", email: "thao.vo@gmail.com", status: "disabled", lastLoginAt: opt(ago(15, 0)), lastActiveAt: opt(ago(15, 0))},
	{key: "u-son", role: "student", name: "Trịnh Bảo Sơn", email: "son.trinh@gmail.com", status: "invited", mustChangePassword: true, tempPasswordExpiresAt: opt(ahead(2.8))},
	{key: "u-tu", role: "student", name: "Mai Anh Tú", email: "tu.mai@gmail.com", status: "invited", mustChangePassword: true, tempPasswordExpiresAt: opt(ahead(2.9))},
}

// staffCreatedAt là thời điểm tạo tài khoản admin/giảng viên (seed.js không ghi; trước mọi dữ liệu khác).
var staffCreatedAt = ago(70, 0)

// disabledAt là thời điểm vô hiệu hóa tài khoản disabled, khớp audit user.disabled của seed.js.
var disabledAt = ago(15, 0)

type stageSeed struct{ key, code, name string }

var stages = []stageSeed{
	{"st-db", "DB", "Database"},
	{"st-ds", "DS", "Data structure"},
	{"st-go", "GO", "Golang basic"},
	{"st-re", "RE", "ReactJS basic"},
	{"st-web", "WEB", "HTML CSS JS"},
}

type lessonSeed struct {
	key, typ, title string
	required        bool
	video           string // tên file video (bài video)
	duration        string // "mm:ss" (bài video)
	markdown        *markdownDoc
}

func video(key, title string, required bool, file, duration string) lessonSeed {
	return lessonSeed{key: key, typ: "video", title: title, required: required, video: file, duration: duration}
}

func doc(key, title string, required bool, md *markdownDoc) lessonSeed {
	return lessonSeed{key: key, typ: "markdown", title: title, required: required, markdown: md}
}

type stageVersionSeed struct {
	key, stage  string
	no          int
	publishedAt offset
	lessons     []lessonSeed
}

// Mọi phiên bản chặng trong seed.js đều published, không clone từ bản nào.
var stageVersions = []stageVersionSeed{
	{key: "sv-db-1", stage: "st-db", no: 1, publishedAt: ago(70, 0), lessons: []lessonSeed{
		video("db-intro", "Giới thiệu SQL", true, "db-01-gioi-thieu-sql.mp4", "18:24"),
		doc("db-table", "Thiết kế bảng và khóa", true, &mdTableDesign),
		doc("db-index", "Đọc thêm: chỉ mục", false, &mdIndexes),
	}},
	{key: "sv-ds-1", stage: "st-ds", no: 1, publishedAt: ago(68, 0), lessons: []lessonSeed{
		video("ds-array", "Mảng và danh sách liên kết", true, "ds-01-mang-danh-sach.mp4", "22:10"),
		doc("ds-stack", "Stack và Queue", true, &mdStackQueue),
		video("ds-hash", "Hash table", true, "ds-03-hash-table.mp4", "16:45"),
	}},
	{key: "sv-go-1", stage: "st-go", no: 1, publishedAt: ago(66, 0), lessons: []lessonSeed{
		video("go-setup", "Cài đặt và Hello World", true, "go-01-cai-dat.mp4", "12:02"),
		doc("go-types", "Kiểu dữ liệu và hàm", true, &mdGoTypes),
		video("go-routine", "Goroutine cơ bản", true, "go-03-goroutine.mp4", "25:31"),
		doc("go-exercise", "Bài tập tổng hợp", false, &mdGoExercise),
	}},
	{key: "sv-re-1", stage: "st-re", no: 1, publishedAt: ago(64, 0), lessons: []lessonSeed{
		video("re-component", "Component và props", true, "re-01-component.mp4", "20:15"),
		doc("re-state", "State và sự kiện", true, &mdReactState),
		video("re-hooks", "Hooks", true, "re-03-hooks.mp4", "28:40"),
	}},
	{key: "sv-web-1", stage: "st-web", no: 1, publishedAt: ago(62, 0), lessons: []lessonSeed{
		doc("web-html", "HTML ngữ nghĩa", true, &mdHTMLSemantic),
		video("web-css", "CSS layout", true, "web-02-css-layout.mp4", "24:05"),
		video("web-js", "JavaScript và DOM", true, "web-03-js-dom.mp4", "19:50"),
	}},
}

// seedVideoSizeBytes là kích thước giữ chỗ của bản ghi video mẫu: file thật không có trong object storage
// (video mẫu chỉ được tải lên khi chạy seed với cờ upload-sample), còn cột size_bytes bắt buộc > 0.
const seedVideoSizeBytes = 1 << 20

type courseSeed struct{ key, code, name string }

var courses = []courseSeed{
	{"co-basic", "BASIC", "Lập trình cơ bản"},
}

type courseVersionSeed struct {
	key, course   string
	no            int
	publishedAt   offset
	stageVersions []string // theo thứ tự position
}

var courseVersions = []courseVersionSeed{
	{key: "cv-basic-1", course: "co-basic", no: 1, publishedAt: ago(60, 0),
		stageVersions: []string{"sv-db-1", "sv-ds-1", "sv-go-1", "sv-re-1", "sv-web-1"}},
}

type classSeed struct {
	key, code, name, courseVersion, status, teacher string
	startDate, endDate, createdAt                   offset
}

var classes = []classSeed{
	{key: "cl-basic01", code: "basic01", name: "Lập trình cơ bản – khóa 1", courseVersion: "cv-basic-1", status: "active",
		startDate: ago(58, 0), endDate: ahead(55), teacher: "u-gv", createdAt: ago(60, 0)},
	{key: "cl-basic02", code: "basic02", name: "Lập trình cơ bản – khóa 2", courseVersion: "cv-basic-1", status: "active",
		startDate: ago(20, 0), endDate: ahead(95), teacher: "u-gv2", createdAt: ago(24, 0)},
	{key: "cl-basic03", code: "basic03", name: "Lập trình cơ bản – khóa 3", courseVersion: "cv-basic-1", status: "draft",
		startDate: ahead(28), endDate: ahead(140), teacher: "u-gv", createdAt: ago(3, 0)},
}

// enrollment là một lần enroll() của seed.js: thành viên lớp, lời mời kèm email outbox, và tiến độ done()/opened().
type enrollment struct {
	class, user  string
	invite       string  // inviteSent | inviteFailed | inviteQueued
	invitedDays  float64 // số ngày trước thời điểm seed
	memberStatus string
	done         []string // bài đã hoàn thành, theo thứ tự trong seed.js
	doneLastDays float64
	opened       []string // bài đã mở nhưng chưa hoàn thành
	openedDays   float64
}

var enrollments = []enrollment{
	// basic01 — tuần thứ tám
	{class: "cl-basic01", user: "u-an", invite: inviteSent, invitedDays: 58, memberStatus: "active",
		done:         []string{"db-intro", "db-table", "db-index", "ds-array", "ds-stack", "ds-hash", "go-setup", "go-types", "go-routine", "re-component", "re-state"},
		doneLastDays: 0.2, opened: []string{"re-hooks"}, openedDays: 0.17},
	{class: "cl-basic01", user: "u-bich", invite: inviteSent, invitedDays: 58, memberStatus: "active",
		done:         []string{"db-intro", "db-table", "ds-array", "ds-stack", "ds-hash", "go-setup", "go-types"},
		doneLastDays: 2, opened: []string{"go-routine"}, openedDays: 2},
	{class: "cl-basic01", user: "u-cuong", invite: inviteSent, invitedDays: 58, memberStatus: "active",
		done:         []string{"db-intro", "db-table", "ds-array", "ds-stack", "ds-hash", "go-setup", "go-types", "go-routine", "go-exercise", "re-component", "re-state", "re-hooks", "web-html", "web-css"},
		doneLastDays: 1, opened: []string{"web-js"}, openedDays: 1},
	// Chưa đăng nhập lần nào; mật khẩu tạm đã hết hạn.
	{class: "cl-basic01", user: "u-dung", invite: inviteSent, invitedDays: 58, memberStatus: "active"},
	{class: "cl-basic01", user: "u-ha", invite: inviteSent, invitedDays: 57, memberStatus: "active",
		done: []string{"db-intro", "db-table", "ds-array", "ds-stack"}, doneLastDays: 4, opened: []string{"ds-hash"}, openedDays: 4},
	{class: "cl-basic01", user: "u-khang", invite: inviteSent, invitedDays: 57, memberStatus: "active",
		done: []string{"db-intro", "db-table"}, doneLastDays: 21},
	{class: "cl-basic01", user: "u-thao", invite: inviteSent, invitedDays: 57, memberStatus: "dropped",
		done: []string{"db-intro"}, doneLastDays: 15},

	// basic02 — tuần thứ ba
	{class: "cl-basic02", user: "u-phong", invite: inviteSent, invitedDays: 22, memberStatus: "active",
		done:         []string{"db-intro", "db-table", "ds-array", "ds-stack", "ds-hash", "go-setup"},
		doneLastDays: 0.3, opened: []string{"go-types"}, openedDays: 0.3},
	{class: "cl-basic02", user: "u-linh", invite: inviteSent, invitedDays: 22, memberStatus: "active",
		done: []string{"db-intro", "db-table", "ds-array"}, doneLastDays: 1},
	{class: "cl-basic02", user: "u-minh", invite: inviteFailed, invitedDays: 22, memberStatus: "active"},
	{class: "cl-basic02", user: "u-nhan", invite: inviteSent, invitedDays: 22, memberStatus: "active",
		done: []string{"db-intro"}, doneLastDays: 6, opened: []string{"db-table"}, openedDays: 6},
	{class: "cl-basic02", user: "u-quyen", invite: inviteSent, invitedDays: 1.5, memberStatus: "active"},
	// Phong vào basic01 muộn, là lớp thứ hai của anh (tài khoản đã có được mời lại).
	{class: "cl-basic01", user: "u-phong", invite: inviteSent, invitedDays: 12, memberStatus: "active",
		done: []string{"db-intro", "db-table", "ds-array", "ds-stack"}, doneLastDays: 2},

	// basic03 — nháp, đã gửi hai lời mời
	{class: "cl-basic03", user: "u-son", invite: inviteSent, invitedDays: 0.2, memberStatus: "active"},
	{class: "cl-basic03", user: "u-tu", invite: inviteQueued, invitedDays: 0.1, memberStatus: "active"},
}

// droppedAt là thời điểm thành viên rời lớp (seed.js không ghi; trùng lúc tài khoản bị vô hiệu hóa).
var droppedAt = disabledAt

// doneStepDays là khoảng cách giữa hai bài hoàn thành liên tiếp trong done() của seed.js.
const doneStepDays = 1.3

// auditSeed là một dòng audit của seed.js với action theo tập chuẩn của ứng dụng.
// members (nếu có) là học viên được mời/gửi lại lời mời trong lớp đích.
type auditSeed struct {
	days               float64
	action, targetType string
	target             string
	before, after      map[string]any
	members            []string
	invitationKind     string
}

var audits = []auditSeed{
	{days: 70, action: "stage_version.published", targetType: "stage_version", target: "sv-db-1",
		before: map[string]any{"status": "draft"}, after: map[string]any{"status": "published"}},
	{days: 60, action: "course_version.published", targetType: "course_version", target: "cv-basic-1",
		before: map[string]any{"status": "draft"}, after: map[string]any{"status": "published"}},
	{days: 58, action: "class.activated", targetType: "class", target: "cl-basic01",
		before: map[string]any{"status": "draft"}, after: map[string]any{"status": "active"}},
	{days: 22, action: "class.member_invited", targetType: "class", target: "cl-basic02",
		members: []string{"u-phong", "u-linh", "u-minh", "u-nhan", "u-quyen"}, invitationKind: "invite"},
	{days: 20, action: "class.activated", targetType: "class", target: "cl-basic02",
		before: map[string]any{"status": "draft"}, after: map[string]any{"status": "active"}},
	{days: 15, action: "user.disabled", targetType: "user", target: "u-thao",
		before: map[string]any{"status": "active"}, after: map[string]any{"status": "disabled"}},
	{days: 12, action: "class.member_invited", targetType: "class", target: "cl-basic01",
		members: []string{"u-phong"}, invitationKind: "invite"},
	{days: 3, action: "class.created", targetType: "class", target: "cl-basic03",
		after: map[string]any{"status": "draft"}},
	{days: 1.5, action: "class.invitation_resent", targetType: "class", target: "cl-basic02",
		members: []string{"u-quyen"}, invitationKind: "resend"},
	{days: 0.2, action: "class.member_invited", targetType: "class", target: "cl-basic03",
		members: []string{"u-son", "u-tu"}, invitationKind: "invite"},
}
