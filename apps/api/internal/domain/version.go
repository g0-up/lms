package domain

// VersionNo là số thứ tự phiên bản trong một chặng hoặc khóa học, bắt đầu từ 1.
type VersionNo int

// FirstVersionNo là số của phiên bản tạo kèm chặng/khóa học mới.
const FirstVersionNo VersionNo = 1

// ParseVersionNo kiểm n >= 1 (trùng CHECK ck_*_version_no).
func ParseVersionNo(n int) (VersionNo, error) {
	if n < 1 {
		return 0, ErrInvalidVersionNo
	}
	return VersionNo(n), nil
}

// Next trả số phiên bản kế tiếp.
func (n VersionNo) Next() VersionNo { return n + 1 }

// Int trả giá trị int để ghi vào cột version_no.
func (n VersionNo) Int() int { return int(n) }

// VersionStatus là vòng đời của phiên bản chặng và phiên bản khóa học (chuỗi khớp ck_*_versions_status).
type VersionStatus string

// Các trạng thái phiên bản.
const (
	VersionDraft     VersionStatus = "draft"
	VersionPublished VersionStatus = "published"
	VersionArchived  VersionStatus = "archived"
)

// versionTransitions là state machine một chiều draft → published → archived.
var versionTransitions = map[VersionStatus][]VersionStatus{
	VersionDraft:     {VersionPublished},
	VersionPublished: {VersionArchived},
	VersionArchived:  {},
}

var (
	errVersionArchiveNotPublished = ErrInvalidTransition.WithMsg("Chỉ phiên bản đã phát hành mới có thể lưu trữ")
	errVersionPublishedLocked     = ErrInvalidTransition.WithMsg("Phiên bản đã phát hành không thể sửa")
	errVersionPublishNotDraft     = ErrInvalidTransition.WithMsg("Chỉ phát hành được bản nháp")
	errVersionArchivedLocked      = ErrInvalidTransition.WithMsg("Phiên bản đã lưu trữ không thể thay đổi")
)

// ParseVersionStatus đọc trạng thái từ chuỗi của database hoặc request.
func ParseVersionStatus(s string) (VersionStatus, error) {
	st := VersionStatus(s)
	if _, ok := versionTransitions[st]; !ok {
		return "", ErrInvalidStatus
	}
	return st, nil
}

func (s VersionStatus) String() string { return string(s) }

// CanTransitionTo cho biết state machine cho phép s → to.
func (s VersionStatus) CanTransitionTo(to VersionStatus) bool {
	return allowed(versionTransitions[s], to)
}

// Transition trả nil khi s → to hợp lệ, ngược lại lỗi ErrInvalidTransition kèm lý do tiếng Việt.
func (s VersionStatus) Transition(to VersionStatus) error {
	if s.CanTransitionTo(to) {
		return nil
	}
	switch {
	case s == VersionArchived:
		return errVersionArchivedLocked
	case s == VersionDraft && to == VersionArchived:
		return errVersionArchiveNotPublished
	case s == VersionPublished && to == VersionPublished:
		return errVersionPublishNotDraft
	case s == VersionPublished:
		return errVersionPublishedLocked
	default:
		return ErrInvalidTransition
	}
}

// IsMutable: chỉ bản nháp được sửa nội dung hoặc xóa.
func (s VersionStatus) IsMutable() bool { return s == VersionDraft }

func allowed[T comparable](targets []T, to T) bool {
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}
