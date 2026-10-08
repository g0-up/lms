package httpx

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// viMessage dịch một lỗi rule của validator sang tiếng Việt. Tag chưa có bản dịch dùng thông điệp chung để không
// bao giờ lộ tiếng Anh ra giao diện.
func viMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required", "required_if", "required_unless", "required_with", "required_without":
		return "Bắt buộc"
	case "email":
		return "Email không hợp lệ"
	case "uuid", "uuid4", "uuid7":
		return "Mã định danh không hợp lệ"
	case "oneof":
		return "Giá trị phải là một trong: " + strings.Join(strings.Fields(fe.Param()), ", ")
	case "min", "gte":
		return boundMessage(fe, "ít nhất")
	case "max", "lte":
		return boundMessage(fe, "tối đa")
	case "len":
		return boundMessage(fe, "đúng")
	default:
		return "Giá trị không hợp lệ"
	}
}

// boundMessage viết thông điệp giới hạn theo kiểu field: độ dài chuỗi, số phần tử, hoặc giá trị số.
func boundMessage(fe validator.FieldError, bound string) string {
	switch fe.Kind() {
	case reflect.String:
		return "Phải có " + bound + " " + fe.Param() + " ký tự"
	case reflect.Slice, reflect.Array, reflect.Map:
		return "Phải có " + bound + " " + fe.Param() + " phần tử"
	default:
		return "Giá trị phải " + boundVerb(bound) + " " + fe.Param()
	}
}

func boundVerb(bound string) string {
	switch bound {
	case "ít nhất":
		return "từ"
	case "tối đa":
		return "không quá"
	default:
		return "bằng"
	}
}
