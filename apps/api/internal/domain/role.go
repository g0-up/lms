package domain

// Role là vai trò tài khoản (chuỗi khớp ck_users_role). Quyền theo vai trò thuộc feature identity, không ở đây.
type Role string

// Các vai trò.
const (
	RoleAdmin   Role = "admin"
	RoleTeacher Role = "teacher"
	RoleStudent Role = "student"
)

// ParseRole đọc vai trò từ chuỗi.
func ParseRole(s string) (Role, error) {
	switch r := Role(s); r {
	case RoleAdmin, RoleTeacher, RoleStudent:
		return r, nil
	default:
		return "", ErrInvalidRole
	}
}

func (r Role) String() string { return string(r) }
