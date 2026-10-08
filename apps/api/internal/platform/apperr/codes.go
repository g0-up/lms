package apperr

// Mã lỗi nghiệp vụ trả trong envelope {"error":{"code",...}}; đúng danh sách 14 mã của hợp đồng API.
const (
	CodeValidationFailed       = "VALIDATION_FAILED"
	CodeUnauthenticated        = "UNAUTHENTICATED"
	CodePasswordChangeRequired = "PASSWORD_CHANGE_REQUIRED"
	CodeForbidden              = "FORBIDDEN"
	CodeNotFound               = "NOT_FOUND"
	CodeConflict               = "CONFLICT"
	CodeVersionImmutable       = "VERSION_IMMUTABLE"
	CodeDraftExists            = "DRAFT_EXISTS"
	CodeInUse                  = "IN_USE"
	CodeInvalidTransition      = "INVALID_TRANSITION"
	CodeTempPasswordExpired    = "TEMP_PASSWORD_EXPIRED"
	CodeAccountDisabled        = "ACCOUNT_DISABLED"
	CodeTooManyAttempts        = "TOO_MANY_ATTEMPTS"
	CodeRateLimited            = "RATE_LIMITED"
)

// Mã ở tầng vận chuyển, không phải lỗi nghiệp vụ: lỗi hệ thống và sai method HTTP.
const (
	CodeInternal         = "INTERNAL"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
)

// DefaultMessages là thông điệp tiếng Việt mặc định của từng mã; nơi gọi có thể thay bằng thông điệp cụ thể hơn.
var DefaultMessages = map[string]string{ //nolint:gosec // thông điệp hiển thị cho người dùng; khóa chứa chữ "Password" làm G101 báo nhầm
	CodeValidationFailed:       "Dữ liệu không hợp lệ",
	CodeUnauthenticated:        "Vui lòng đăng nhập",
	CodePasswordChangeRequired: "Bạn cần đổi mật khẩu trước khi tiếp tục",
	CodeForbidden:              "Bạn không có quyền thực hiện thao tác này",
	CodeNotFound:               "Không tìm thấy dữ liệu",
	CodeConflict:               "Dữ liệu bị trùng",
	CodeVersionImmutable:       "Phiên bản đã phát hành không thể sửa",
	CodeDraftExists:            "Đã có bản nháp, hãy tiếp tục chỉnh sửa bản nháp đó",
	CodeInUse:                  "Không thể xóa vì đang được sử dụng",
	CodeInvalidTransition:      "Chuyển trạng thái không hợp lệ",
	CodeTempPasswordExpired:    "Mật khẩu tạm đã hết hạn, liên hệ quản trị để được cấp lại",
	CodeAccountDisabled:        "Tài khoản đã bị vô hiệu hóa",
	CodeTooManyAttempts:        "Bạn đã nhập sai quá nhiều lần. Thử lại sau 15 phút.",
	CodeRateLimited:            "Bạn thao tác quá nhanh, thử lại sau",
	CodeInternal:               "Lỗi hệ thống, vui lòng thử lại",
	CodeMethodNotAllowed:       "Phương thức không được hỗ trợ",
}

// Message trả thông điệp mặc định của code, hoặc thông điệp lỗi hệ thống khi code lạ.
func Message(code string) string {
	if m, ok := DefaultMessages[code]; ok {
		return m
	}
	return DefaultMessages[CodeInternal]
}
