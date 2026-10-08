package classes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
)

// MembershipReader cho feature học (learning) biết học viên có trong lớp không và lớp đang ở trạng thái nào,
// chạy trong tx (hoặc Executor) của caller.
type MembershipReader interface {
	// MemberContext trả ngữ cảnh (lớp, học viên); lớp không tồn tại → ErrClassNotFound; học viên không thuộc lớp →
	// MemberContext với IsMember() = false.
	MemberContext(ctx context.Context, ex db.Executor, classID, userID uuid.UUID) (MemberContext, error)
}

// MemberContext là ngữ cảnh một học viên trong một lớp. MemberID = uuid.Nil khi học viên không thuộc lớp.
type MemberContext struct {
	MemberID        uuid.UUID
	MemberStatus    domain.MemberStatus
	ClassStatus     domain.ClassStatus
	CourseVersionID uuid.UUID
	TeacherID       uuid.UUID
}

// IsMember cho biết học viên có hàng class_members trong lớp (mọi trạng thái).
func (m MemberContext) IsMember() bool { return m.MemberID != uuid.Nil }

// IsActiveMember cho biết học viên đang học trong lớp (chưa rời).
func (m MemberContext) IsActiveMember() bool {
	return m.IsMember() && m.MemberStatus == domain.MemberActive
}

// PGMembershipReader là MembershipReader trên PostgreSQL.
type PGMembershipReader struct{}

var _ MembershipReader = PGMembershipReader{}

func (PGMembershipReader) MemberContext(ctx context.Context, ex db.Executor, classID, userID uuid.UUID) (MemberContext, error) {
	var rec struct {
		MemberID        *uuid.UUID `db:"member_id"`
		MemberStatus    *string    `db:"member_status"`
		ClassStatus     string     `db:"class_status"`
		CourseVersionID uuid.UUID  `db:"course_version_id"`
		TeacherID       uuid.UUID  `db:"teacher_id"`
	}
	err := sqlx.GetContext(ctx, ex, &rec, `SELECT cm.id AS member_id, cm.status AS member_status, c.status AS class_status,
		c.course_version_id, c.teacher_id
		FROM classes c LEFT JOIN class_members cm ON cm.class_id = c.id AND cm.user_id = $2
		WHERE c.id = $1`, classID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return MemberContext{}, ErrClassNotFound
	}
	if err != nil {
		return MemberContext{}, fmt.Errorf("classes: đọc ngữ cảnh thành viên: %w", err)
	}
	cs, err := domain.ParseClassStatus(rec.ClassStatus)
	if err != nil {
		return MemberContext{}, fmt.Errorf("classes: trạng thái lớp %q: %w", rec.ClassStatus, err)
	}
	out := MemberContext{ClassStatus: cs, CourseVersionID: rec.CourseVersionID, TeacherID: rec.TeacherID}
	if rec.MemberID != nil && rec.MemberStatus != nil {
		ms, err := domain.ParseMemberStatus(*rec.MemberStatus)
		if err != nil {
			return MemberContext{}, fmt.Errorf("classes: trạng thái thành viên %q: %w", *rec.MemberStatus, err)
		}
		out.MemberID, out.MemberStatus = *rec.MemberID, ms
	}
	return out, nil
}
