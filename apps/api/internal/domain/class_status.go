package domain

// ClassStatus là vòng đời lớp (chuỗi khớp ck_classes_status): draft → active → ended, một chiều.
type ClassStatus string

// Các trạng thái lớp.
const (
	ClassDraft  ClassStatus = "draft"
	ClassActive ClassStatus = "active"
	ClassEnded  ClassStatus = "ended"
)

var classTransitions = map[ClassStatus][]ClassStatus{
	ClassDraft:  {ClassActive},
	ClassActive: {ClassEnded},
	ClassEnded:  {},
}

var (
	errClassEndedLocked     = ErrInvalidTransition.WithMsg("Lớp đã kết thúc không thể thay đổi")
	errClassActivateNoDraft = ErrInvalidTransition.WithMsg("Chỉ lớp nháp mới có thể kích hoạt")
	errClassOneWay          = ErrInvalidTransition.WithMsg("Chỉ chuyển trạng thái một chiều: nháp → đang chạy → đã kết thúc.")
)

// ParseClassStatus đọc trạng thái lớp từ chuỗi.
func ParseClassStatus(s string) (ClassStatus, error) {
	st := ClassStatus(s)
	if _, ok := classTransitions[st]; !ok {
		return "", ErrInvalidStatus
	}
	return st, nil
}

func (s ClassStatus) String() string { return string(s) }

// CanTransitionTo cho biết state machine cho phép s → to.
func (s ClassStatus) CanTransitionTo(to ClassStatus) bool {
	return allowed(classTransitions[s], to)
}

// Transition trả nil khi s → to hợp lệ, ngược lại lỗi ErrInvalidTransition kèm lý do tiếng Việt.
func (s ClassStatus) Transition(to ClassStatus) error {
	if s.CanTransitionTo(to) {
		return nil
	}
	switch {
	case s == ClassEnded:
		return errClassEndedLocked
	case to == ClassActive:
		return errClassActivateNoDraft
	default:
		return errClassOneWay
	}
}
