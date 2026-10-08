package domain

// MemberStatus là trạng thái thành viên lớp (chuỗi khớp ck_class_members_status).
// MVP không có endpoint ghi MemberCompleted; giá trị có sẵn để không cần đổi schema sau.
type MemberStatus string

// Các trạng thái thành viên lớp.
const (
	MemberActive    MemberStatus = "active"
	MemberDropped   MemberStatus = "dropped"
	MemberCompleted MemberStatus = "completed"
)

// ParseMemberStatus đọc trạng thái thành viên từ chuỗi.
func ParseMemberStatus(s string) (MemberStatus, error) {
	switch st := MemberStatus(s); st {
	case MemberActive, MemberDropped, MemberCompleted:
		return st, nil
	default:
		return "", ErrInvalidStatus
	}
}

func (s MemberStatus) String() string { return string(s) }
