package classes

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/identity"
)

func snapshot(role domain.Role, status domain.UserStatus, mustChange bool, expires *time.Time) *identity.UserSnapshot {
	return &identity.UserSnapshot{
		ID: uuid.New(), Name: "Bùi Minh", Role: role, Status: status, MustChangePassword: mustChange, TempPasswordExpiresAt: expires,
	}
}

func at(d time.Duration) *time.Time {
	t := testNow.Add(d)
	return &t
}

func TestDecideInvite(t *testing.T) {
	active := NewMember(uuid.New(), uuid.New(), uuid.New(), testNow)
	dropped := NewMember(uuid.New(), uuid.New(), uuid.New(), testNow)
	if err := dropped.Drop(testNow); err != nil {
		t.Fatal(err)
	}
	student := snapshot(domain.RoleStudent, domain.UserActive, false, nil)

	tests := []struct {
		name     string
		class    domain.ClassStatus
		existing *identity.UserSnapshot
		member   *ClassMember
		fullName string
		want     InviteDecision
		wantErr  error
	}{
		{"lớp đã kết thúc", domain.ClassEnded, nil, nil, "Học Viên Mới", 0, ErrClassEnded},
		{"email giảng viên", domain.ClassActive, snapshot(domain.RoleTeacher, domain.UserActive, false, nil), nil, "", 0, ErrInternalEmail},
		{"email quản trị", domain.ClassDraft, snapshot(domain.RoleAdmin, domain.UserActive, false, nil), nil, "", 0, ErrInternalEmail},
		{"tài khoản bị vô hiệu hóa", domain.ClassActive, snapshot(domain.RoleStudent, domain.UserDisabled, false, nil), nil, "", 0, ErrAccountDisabled},
		{"vô hiệu hóa dù đã rời lớp", domain.ClassActive, snapshot(domain.RoleStudent, domain.UserDisabled, false, nil), dropped, "", 0, ErrAccountDisabled},
		{"đã trong lớp", domain.ClassActive, student, active, "", 0, ErrAlreadyMember},
		{"đã rời lớp → quay lại", domain.ClassActive, student, dropped, "", DecisionRejoin, nil},
		{"email mới thiếu họ tên", domain.ClassActive, nil, nil, "  ", 0, ErrNameRequired},
		{"email mới", domain.ClassDraft, nil, nil, "Học Viên Mới", DecisionCreateUser, nil},
		{"chưa đổi mật khẩu, mật khẩu tạm còn hạn", domain.ClassActive,
			snapshot(domain.RoleStudent, domain.UserInvited, true, at(time.Hour)), nil, "", DecisionAddInvitedKeep, nil},
		{"chưa đổi mật khẩu, mật khẩu tạm hết hạn", domain.ClassActive,
			snapshot(domain.RoleStudent, domain.UserInvited, true, at(-time.Hour)), nil, "", DecisionAddInvitedRotate, nil},
		{"chưa đổi mật khẩu, hết hạn đúng lúc này", domain.ClassActive,
			snapshot(domain.RoleStudent, domain.UserInvited, true, at(0)), nil, "", DecisionAddInvitedRotate, nil},
		{"học viên đang hoạt động", domain.ClassActive, student, nil, "", DecisionAddActive, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecideInvite(newTestClass(t, tt.class), tt.existing, tt.member, tt.fullName, testNow)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("DecideInvite = %v, %v; muốn %v, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
