// Package audit ghi nhật ký thao tác quản trị vào audit_logs trong cùng transaction với thay đổi nghiệp vụ.
package audit

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/platform/db"
)

// Entry là một dòng nhật ký. Before/After được marshal sang jsonb sau khi bỏ khóa nhạy cảm (xem sanitize);
// nil thành NULL. RequestID rỗng thành NULL (ví dụ thao tác của worker). At rỗng thì lấy từ Clock của Recorder;
// caller đặt At khi nhiều dòng thuộc cùng một thao tác nguyên tử để chúng cùng mốc thời gian.
type Entry struct {
	ActorID    *uuid.UUID
	Action     string // một hằng Action* trong actions.go
	TargetType string // tên thực thể, ví dụ "stage_version", "class", "user"
	TargetID   *uuid.UUID
	Before     any
	After      any
	RequestID  string
	At         time.Time
}

// Recorder ghi Entry bằng Executor của caller để nhật ký và thay đổi nghiệp vụ cùng commit hoặc rollback.
type Recorder interface {
	Record(ctx context.Context, tx db.Executor, e Entry) error
}

var knownActions = map[string]struct{}{
	ActionStageVersionPublished: {}, ActionStageVersionCloned: {}, ActionStageVersionArchived: {}, ActionStageVersionDeleted: {},
	ActionCourseVersionPublished: {}, ActionCourseVersionCloned: {}, ActionCourseVersionArchived: {}, ActionCourseVersionDeleted: {},
	ActionCourseStageVersionApplied: {}, ActionClassCourseVersionChanged: {},
	ActionClassActivated: {}, ActionClassEnded: {}, ActionClassMemberInvited: {}, ActionClassInvitationResent: {}, ActionClassMemberDropped: {},
	ActionUserDisabled: {}, ActionUserEnabled: {},
	ActionStageCreated: {}, ActionCourseCreated: {}, ActionClassCreated: {},
}

// IsKnownAction cho biết action thuộc danh sách trong actions.go.
func IsKnownAction(action string) bool {
	_, ok := knownActions[action]
	return ok
}
