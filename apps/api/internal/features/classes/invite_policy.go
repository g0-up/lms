package classes

import (
	"strings"
	"time"

	"lms/api/internal/domain"
	"lms/api/internal/features/identity"
)

// InviteDecision là nhánh xử lý một lời mời FR-01 sau khi qua hết điều kiện chặn.
type InviteDecision int

// Các nhánh mời.
const (
	// DecisionCreateUser: email chưa có tài khoản → tạo học viên invited + mật khẩu tạm, email invite.
	DecisionCreateUser InviteDecision = iota + 1
	// DecisionAddActive: học viên đã đăng nhập → thêm vào lớp, email added.
	DecisionAddActive
	// DecisionAddInvitedKeep: học viên chưa đổi mật khẩu, mật khẩu tạm còn hạn → thêm, email added, không xoay.
	DecisionAddInvitedKeep
	// DecisionAddInvitedRotate: học viên chưa đổi mật khẩu, mật khẩu tạm đã hết hạn → xoay, email invite.
	DecisionAddInvitedRotate
	// DecisionRejoin: thành viên đã rời lớp → kích hoạt lại, giữ tiến độ cũ.
	DecisionRejoin
)

func (d InviteDecision) String() string {
	switch d {
	case DecisionCreateUser:
		return "create_user"
	case DecisionAddActive:
		return "add_active"
	case DecisionAddInvitedKeep:
		return "add_invited_keep"
	case DecisionAddInvitedRotate:
		return "add_invited_rotate"
	case DecisionRejoin:
		return "rejoin"
	default:
		return "unknown"
	}
}

// DecideInvite áp quy tắc FR-01 theo đúng thứ tự prototype: lớp đã kết thúc, email nội bộ, tài khoản bị vô hiệu
// hóa, đã là thành viên, thành viên đã rời, email mới, học viên chưa đổi mật khẩu (còn/hết hạn), học viên active.
// existing và member nil khi chưa có.
func DecideInvite(cls *Class, existing *identity.UserSnapshot, member *ClassMember, fullName string, now time.Time) (InviteDecision, error) {
	if !cls.AcceptsInvitations() {
		return 0, ErrClassEnded
	}
	if existing != nil && existing.Role != domain.RoleStudent {
		return 0, ErrInternalEmail
	}
	if existing != nil && existing.Status == domain.UserDisabled {
		return 0, ErrAccountDisabled
	}
	if member != nil && member.Status() == domain.MemberDropped {
		return DecisionRejoin, nil
	}
	if member != nil {
		return 0, ErrAlreadyMember
	}
	if existing == nil {
		if strings.TrimSpace(fullName) == "" {
			return 0, ErrNameRequired
		}
		return DecisionCreateUser, nil
	}
	if existing.MustChangePassword {
		if tempPasswordValid(existing, now) {
			return DecisionAddInvitedKeep, nil
		}
		return DecisionAddInvitedRotate, nil
	}
	return DecisionAddActive, nil
}

// tempPasswordValid cho biết mật khẩu tạm còn dùng được tại now (cùng mốc với User.Authenticate).
func tempPasswordValid(u *identity.UserSnapshot, now time.Time) bool {
	return u.TempPasswordExpiresAt != nil && now.Before(*u.TempPasswordExpiresAt)
}
