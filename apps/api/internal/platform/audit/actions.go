package audit

// Action ghi vào audit_logs.action: đúng 20 giá trị của hợp đồng API, dạng quá khứ, chấm phân cách.
// actions_test.go chặn thêm action ngoài danh sách; nhãn tiếng Việt thuộc màn hình nhật ký.
const (
	ActionStageVersionPublished     = "stage_version.published"
	ActionStageVersionCloned        = "stage_version.cloned"
	ActionStageVersionArchived      = "stage_version.archived"
	ActionStageVersionDeleted       = "stage_version.deleted"
	ActionCourseVersionPublished    = "course_version.published"
	ActionCourseVersionCloned       = "course_version.cloned"
	ActionCourseVersionArchived     = "course_version.archived"
	ActionCourseVersionDeleted      = "course_version.deleted"
	ActionCourseStageVersionApplied = "course.stage_version_applied"
	ActionClassCourseVersionChanged = "class.course_version_changed"
	ActionClassActivated            = "class.activated"
	ActionClassEnded                = "class.ended"
	ActionClassMemberInvited        = "class.member_invited"
	ActionClassInvitationResent     = "class.invitation_resent"
	ActionClassMemberDropped        = "class.member_dropped"
	ActionUserDisabled              = "user.disabled"
	ActionUserEnabled               = "user.enabled"
	ActionStageCreated              = "stage.created"
	ActionCourseCreated             = "course.created"
	ActionClassCreated              = "class.created"
)
