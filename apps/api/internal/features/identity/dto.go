package identity

import (
	"time"

	"github.com/google/uuid"
)

// MeDTO là người dùng đang đăng nhập (GET /auth/me, POST /auth/login, POST /auth/change-password). Không có field
// nào chứa hash hay mật khẩu.
type MeDTO struct {
	ID                    uuid.UUID  `json:"id"`
	Name                  string     `json:"name"`
	Email                 string     `json:"email"`
	Role                  string     `json:"role"`
	Status                string     `json:"status"`
	MustChangePassword    bool       `json:"mustChangePassword"`
	TempPasswordExpiresAt *time.Time `json:"tempPasswordExpiresAt,omitempty"` // chỉ khi còn phải đổi mật khẩu tạm
}

// NewMeDTO dựng MeDTO từ user.
func NewMeDTO(u *User) MeDTO {
	d := MeDTO{
		ID: u.ID(), Name: u.Name(), Email: u.Email().Display(), Role: string(u.Role()), Status: string(u.Status()),
		MustChangePassword: u.MustChangePassword(),
	}
	if u.MustChangePassword() {
		d.TempPasswordExpiresAt = u.TempPasswordExpiresAt()
	}
	return d
}

// UserDTO là user trong danh sách quản trị và kết quả vô hiệu hóa/kích hoạt.
type UserDTO struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	Status string    `json:"status"`
}

// NewUserDTO dựng UserDTO từ user.
func NewUserDTO(u *User) UserDTO {
	return UserDTO{ID: u.ID(), Name: u.Name(), Email: u.Email().Display(), Role: string(u.Role()), Status: string(u.Status())}
}

type loginResponse struct {
	User MeDTO `json:"user"`
}

type listUsersResponse struct {
	Items []UserDTO `json:"items"`
}

type messageResponse struct {
	Message string `json:"message"`
}
