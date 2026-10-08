package domain

// UserStatus là trạng thái tài khoản (chuỗi khớp ck_users_status).
type UserStatus string

// Các trạng thái tài khoản.
const (
	UserInvited  UserStatus = "invited"  // chưa đăng nhập, còn dùng mật khẩu tạm
	UserActive   UserStatus = "active"   // đã đổi mật khẩu lần đầu
	UserDisabled UserStatus = "disabled" // bị quản trị vô hiệu hóa
)

// userTransitions: invited → active khi đổi mật khẩu lần đầu; invited|active → disabled;
// disabled → invited|active khi kích hoạt lại (Enable chọn đích).
var userTransitions = map[UserStatus][]UserStatus{
	UserInvited:  {UserActive, UserDisabled},
	UserActive:   {UserDisabled},
	UserDisabled: {UserInvited, UserActive},
}

var (
	errUserDisabled   = ErrInvalidTransition.WithMsg("Tài khoản đã bị vô hiệu hóa")
	errUserNotDisable = ErrInvalidTransition.WithMsg("Tài khoản chưa bị vô hiệu hóa")
	errUserTransition = ErrInvalidTransition.WithMsg("Không thể chuyển trạng thái tài khoản như vậy")
)

// ParseUserStatus đọc trạng thái tài khoản từ chuỗi.
func ParseUserStatus(s string) (UserStatus, error) {
	st := UserStatus(s)
	if _, ok := userTransitions[st]; !ok {
		return "", ErrInvalidStatus
	}
	return st, nil
}

func (s UserStatus) String() string { return string(s) }

// CanTransitionTo cho biết state machine cho phép s → to.
func (s UserStatus) CanTransitionTo(to UserStatus) bool {
	return allowed(userTransitions[s], to)
}

// Transition trả nil khi s → to hợp lệ, ngược lại lỗi ErrInvalidTransition kèm lý do tiếng Việt.
func (s UserStatus) Transition(to UserStatus) error {
	if s.CanTransitionTo(to) {
		return nil
	}
	if s == UserDisabled {
		return errUserDisabled
	}
	return errUserTransition
}

// Enable trả trạng thái sau khi kích hoạt lại tài khoản bị vô hiệu hóa: về invited khi người dùng chưa đăng nhập
// lần nào (vẫn phải dùng mật khẩu tạm), ngược lại về active.
func (s UserStatus) Enable(neverLoggedIn bool) (UserStatus, error) {
	if s != UserDisabled {
		return s, errUserNotDisable
	}
	if neverLoggedIn {
		return UserInvited, nil
	}
	return UserActive, nil
}
