package reports

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/audit"
)

func TestActionLabelsCoverEveryAuditAction(t *testing.T) {
	want := map[string]string{
		"stage.created":                "Tạo chặng",
		"stage_version.cloned":         "Nhân bản chặng",
		"stage_version.published":      "Phát hành chặng",
		"stage_version.archived":       "Lưu trữ chặng",
		"stage_version.deleted":        "Xóa phiên bản chặng",
		"course.created":               "Tạo khóa học",
		"course_version.cloned":        "Nhân bản khóa học",
		"course_version.published":     "Phát hành khóa học",
		"course_version.archived":      "Lưu trữ khóa học",
		"course_version.deleted":       "Xóa phiên bản khóa học",
		"course.stage_version_applied": "Áp dụng chặng cho khóa học",
		"class.created":                "Tạo lớp",
		"class.course_version_changed": "Đổi phiên bản khóa học của lớp",
		"class.activated":              "Kích hoạt lớp",
		"class.ended":                  "Kết thúc lớp",
		"class.member_invited":         "Mời học viên",
		"class.invitation_resent":      "Gửi lại lời mời",
		"class.member_dropped":         "Gỡ học viên khỏi lớp",
		"user.disabled":                "Vô hiệu hóa tài khoản",
		"user.enabled":                 "Kích hoạt lại tài khoản",
	}
	assert.Equal(t, want, actionLabels)
	for action := range actionLabels {
		assert.True(t, audit.IsKnownAction(action), action)
		label, ok := actionLabel(action)
		assert.True(t, ok, action)
		assert.Equal(t, want[action], label)
	}

	label, ok := actionLabel("class.renamed")
	assert.False(t, ok)
	assert.Equal(t, "class.renamed", label)
}

func rawJSON(t *testing.T, v map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestDescribeActivity(t *testing.T) {
	var (
		actorID   = uuid.New()
		studentID = uuid.New()
		classID   = uuid.New()
		stageID   = uuid.New()
		svOld     = uuid.New()
		svNew     = uuid.New()
		courseID  = uuid.New()
		cvOld     = uuid.New()
		cvNew     = uuid.New()
		goneID    = uuid.New()
	)
	lk := activityLookups{
		Targets: map[TargetRef]TargetInfo{
			{targetClass, classID}:       {Name: "basic01"},
			{targetStage, stageID}:       {Name: "Database"},
			{targetStageVersion, svOld}:  {Name: "Database", VersionNo: 1},
			{targetStageVersion, svNew}:  {Name: "Database", VersionNo: 2},
			{targetCourse, courseID}:     {Name: "Lập trình cơ bản"},
			{targetCourseVersion, cvOld}: {Name: "Lập trình cơ bản", VersionNo: 1},
			{targetCourseVersion, cvNew}: {Name: "Lập trình cơ bản", VersionNo: 2},
		},
		Users: map[uuid.UUID]UserRef{
			actorID:   {Name: "Quân Trần", Email: "quan.tran@goup.vn"},
			studentID: {Name: "An Nguyễn", Email: "an.nguyen@gmail.com"},
		},
	}
	id := func(u uuid.UUID) *uuid.UUID { return &u }

	cases := []struct {
		name                    string
		entry                   audit.LoggedEntry
		wantTarget, wantSummary string
	}{
		{
			"tạo chặng",
			audit.LoggedEntry{Action: audit.ActionStageCreated, TargetType: targetStage, TargetID: id(stageID),
				After: rawJSON(t, map[string]any{"stageId": stageID, "code": "DB", "name": "Database"})},
			"Database", "Database (DB)",
		},
		{
			"nhân bản chặng",
			audit.LoggedEntry{Action: audit.ActionStageVersionCloned, TargetType: targetStageVersion, TargetID: id(svNew),
				After: rawJSON(t, map[string]any{"stageId": stageID, "fromVersionNo": 1, "toVersionNo": 2})},
			"Database v2", "Database v1 → v2",
		},
		{
			"phát hành khóa học",
			audit.LoggedEntry{Action: audit.ActionCourseVersionPublished, TargetType: targetCourseVersion, TargetID: id(cvNew),
				After: rawJSON(t, map[string]any{"courseId": courseID, "versionNo": 2})},
			"Lập trình cơ bản v2", "Lập trình cơ bản v2",
		},
		{
			"xóa phiên bản chặng đã không còn",
			audit.LoggedEntry{Action: audit.ActionStageVersionDeleted, TargetType: targetStageVersion, TargetID: id(goneID),
				After: rawJSON(t, map[string]any{"stageId": stageID, "versionNo": 3})},
			"Database v3", "Database v3",
		},
		{
			"áp dụng chặng cho khóa học",
			audit.LoggedEntry{Action: audit.ActionCourseStageVersionApplied, TargetType: targetCourse, TargetID: id(courseID),
				After: rawJSON(t, map[string]any{"courseId": courseID, "fromVersionNo": 1, "toVersionNo": 2, "stageId": stageID,
					"fromStageVersionId": svOld, "toStageVersionId": svNew})},
			"Lập trình cơ bản", "Database v2 → Lập trình cơ bản v2",
		},
		{
			"đổi phiên bản khóa học của lớp",
			audit.LoggedEntry{Action: audit.ActionClassCourseVersionChanged, TargetType: targetClass, TargetID: id(classID),
				Before: rawJSON(t, map[string]any{"courseVersionId": cvOld}), After: rawJSON(t, map[string]any{"courseVersionId": cvNew})},
			"basic01", "basic01: v1 → v2",
		},
		{
			"mời một học viên",
			audit.LoggedEntry{Action: audit.ActionClassMemberInvited, TargetType: targetClass, TargetID: id(classID),
				After: rawJSON(t, map[string]any{"classId": classID, "userId": studentID, "kind": "invite"})},
			"basic01", "basic01: mời an.nguyen@gmail.com",
		},
		{
			"mời nhiều học viên (seed)",
			audit.LoggedEntry{Action: audit.ActionClassMemberInvited, TargetType: targetClass, TargetID: id(classID),
				After: rawJSON(t, map[string]any{"classId": classID, "classCode": "basic01", "count": 5})},
			"basic01", "basic01: mời 5 học viên",
		},
		{
			"gửi lại lời mời",
			audit.LoggedEntry{Action: audit.ActionClassInvitationResent, TargetType: targetClass, TargetID: id(classID),
				After: rawJSON(t, map[string]any{"userId": studentID})},
			"basic01", "basic01: gửi lại cho an.nguyen@gmail.com",
		},
		{
			"gỡ học viên",
			audit.LoggedEntry{Action: audit.ActionClassMemberDropped, TargetType: targetClass, TargetID: id(classID),
				After: rawJSON(t, map[string]any{"userId": studentID, "status": "dropped"})},
			"basic01", "basic01: gỡ an.nguyen@gmail.com",
		},
		{
			"kích hoạt lớp",
			audit.LoggedEntry{Action: audit.ActionClassActivated, TargetType: targetClass, TargetID: id(classID),
				After: rawJSON(t, map[string]any{"status": "active"})},
			"basic01", "basic01",
		},
		{
			"vô hiệu hóa tài khoản",
			audit.LoggedEntry{Action: audit.ActionUserDisabled, TargetType: targetUser, TargetID: id(studentID),
				After: rawJSON(t, map[string]any{"status": "disabled"})},
			"an.nguyen@gmail.com", "an.nguyen@gmail.com",
		},
		{
			"action lạ dùng nhãn đối tượng",
			audit.LoggedEntry{Action: "class.renamed", TargetType: targetClass, TargetID: id(classID)},
			"basic01", "basic01",
		},
		{
			"payload hỏng vẫn dựng được",
			audit.LoggedEntry{Action: audit.ActionClassMemberInvited, TargetType: targetClass, TargetID: id(classID),
				After: json.RawMessage(`[1,2]`)},
			"basic01", "basic01",
		},
	}
	for _, tc := range cases {
		tc.entry.ID, tc.entry.At, tc.entry.ActorID = uuid.New(), time.Now(), &actorID
		row := describeActivity(tc.entry, lk)
		assert.Equal(t, tc.wantTarget, row.Target.Label, tc.name)
		assert.Equal(t, tc.wantSummary, row.Summary, tc.name)
		assert.Equal(t, "Quân Trần", row.ActorName, tc.name)
		assert.Equal(t, tc.entry.TargetType, row.Target.Type, tc.name)
	}

	sys := describeActivity(audit.LoggedEntry{ID: uuid.New(), Action: audit.ActionClassEnded, TargetType: targetClass, TargetID: id(classID)}, lk)
	assert.Equal(t, "Hệ thống", sys.ActorName)
	assert.Equal(t, "Kết thúc lớp", sys.ActionLabel)
}

func TestActivityRefs(t *testing.T) {
	actor, student, classID, stageID, sv, courseID, cv := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	entries := []audit.LoggedEntry{
		{ActorID: &actor, Action: audit.ActionClassMemberInvited, TargetType: targetClass, TargetID: &classID,
			After: rawJSON(t, map[string]any{"userId": student})},
		{ActorID: &actor, Action: audit.ActionCourseStageVersionApplied, TargetType: targetCourse, TargetID: &courseID,
			After: rawJSON(t, map[string]any{"courseId": courseID, "stageId": stageID, "toStageVersionId": sv})},
		{Action: audit.ActionClassCourseVersionChanged, TargetType: targetClass, TargetID: &classID,
			Before: rawJSON(t, map[string]any{"courseVersionId": cv})},
		{ActorID: &actor, Action: audit.ActionUserDisabled, TargetType: targetUser, TargetID: &student},
	}
	refs, users := activityRefs(entries)
	assert.ElementsMatch(t, []TargetRef{
		{targetClass, classID}, {targetCourse, courseID}, {targetStage, stageID}, {targetStageVersion, sv}, {targetCourseVersion, cv},
	}, refs)
	assert.ElementsMatch(t, []uuid.UUID{actor, student}, users)
}
