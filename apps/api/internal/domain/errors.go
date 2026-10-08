// Package domain là shared kernel: value object và quy tắc nghiệp vụ dùng chung giữa các feature.
// Gói chỉ import thư viện chuẩn và không biết gì về HTTP hay database; tầng platform/apperr ánh xạ Kind sang mã HTTP.
package domain

// Kind phân loại lỗi nghiệp vụ để tầng HTTP chọn status và mã lỗi mà không cần biết từng lỗi cụ thể.
type Kind int

// Các loại lỗi nghiệp vụ.
const (
	KindInvalid    Kind = iota + 1 // dữ liệu vào sai quy tắc
	KindTransition                 // chuyển trạng thái không hợp lệ
	KindNotFound                   // không tìm thấy
	KindConflict                   // trùng hoặc xung đột với dữ liệu hiện có
	KindForbidden                  // không có quyền
	KindImmutable                  // phiên bản đã phát hành không sửa được
	KindInUse                      // đang được tham chiếu nên không xóa được
)

// Error là lỗi nghiệp vụ mang thông điệp tiếng Việt hiển thị được cho người dùng.
//
// Lỗi tạo bằng WithMsg giữ tham chiếu tới sentinel gốc, nên errors.Is(err, ErrInvalidTransition) vẫn đúng
// khi thông điệp cụ thể khác thông điệp mặc định của sentinel.
type Error struct {
	Kind Kind
	Msg  string
	base *Error
}

func (e *Error) Error() string { return e.Msg }

// Is so khớp chính sentinel hoặc sentinel gốc mà lỗi được tạo ra từ đó.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e == t || (e.base != nil && e.base == t)
}

// WithMsg tạo lỗi cùng Kind với thông điệp cụ thể; errors.Is(lỗi mới, e) vẫn đúng.
func (e *Error) WithMsg(msg string) *Error {
	base := e
	if e.base != nil {
		base = e.base
	}
	return &Error{Kind: e.Kind, Msg: msg, base: base}
}

func newError(kind Kind, msg string) *Error { return &Error{Kind: kind, Msg: msg} }

// Sentinel dùng chung. Feature tạo lỗi cụ thể bằng WithMsg hoặc trả thẳng sentinel.
var (
	ErrInvalid           = newError(KindInvalid, "Dữ liệu không hợp lệ")
	ErrInvalidEmail      = newError(KindInvalid, "Email không hợp lệ")
	ErrInvalidCode       = newError(KindInvalid, "Mã không hợp lệ")
	ErrInvalidLessonKey  = newError(KindInvalid, "Mã học liệu chỉ gồm chữ thường không dấu, số hoặc dấu - (2–40 ký tự)")
	ErrInvalidVersionNo  = newError(KindInvalid, "Số phiên bản phải từ 1 trở lên")
	ErrInvalidStatus     = newError(KindInvalid, "Trạng thái không hợp lệ")
	ErrInvalidRole       = newError(KindInvalid, "Vai trò không hợp lệ")
	ErrInvalidPassword   = newError(KindInvalid, "Mật khẩu không hợp lệ")
	ErrInvalidTransition = newError(KindTransition, "Chuyển trạng thái không hợp lệ")
	ErrNotFound          = newError(KindNotFound, "Không tìm thấy dữ liệu")
	ErrConflict          = newError(KindConflict, "Dữ liệu bị trùng")
	ErrForbidden         = newError(KindForbidden, "Bạn không có quyền thực hiện thao tác này")
	ErrVersionImmutable  = newError(KindImmutable, "Phiên bản đã phát hành không thể sửa")
	ErrInUse             = newError(KindInUse, "Không thể xóa vì đang được sử dụng")
)
