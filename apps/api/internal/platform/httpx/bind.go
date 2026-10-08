package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"lms/api/internal/platform/apperr"
)

// MaxBodyBytes là giới hạn body JSON của mọi request.
const MaxBodyBytes = 64 << 10

var (
	validateOnce sync.Once
	validate     *validator.Validate
)

// validatorInstance dựng validator một lần: đọc rule từ tag `binding` (cùng quy ước của gin) và báo lỗi theo tên
// field JSON thay vì tên field Go.
func validatorInstance() *validator.Validate {
	validateOnce.Do(func() {
		v := validator.New(validator.WithRequiredStructEnabled())
		v.SetTagName("binding")
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			switch name {
			case "-":
				return ""
			case "":
				return f.Name
			default:
				return name
			}
		})
		validate = v
	})
	return validate
}

// BindJSON đọc body JSON (tối đa 64KB, từ chối field lạ, đúng một giá trị JSON) vào dst rồi kiểm tag `binding`.
// Lỗi luôn là *apperr.Error 400 VALIDATION_FAILED: lỗi cú pháp ở Details["body"], lỗi rule ở Details[<field json>].
// Không log body.
func BindJSON(c *gin.Context, dst any) error {
	if c.Request.Body == nil {
		return invalidBody("JSON không hợp lệ")
	}
	body := http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return decodeError(err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return decodeError(err)
		}
		return invalidBody("JSON không hợp lệ")
	}
	return validateStruct(dst)
}

func decodeError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return invalidBody("Dữ liệu quá lớn (tối đa 64KB)")
	}
	return invalidBody("JSON không hợp lệ")
}

func invalidBody(msg string) error {
	return apperr.Validation(map[string]string{"body": msg})
}

func validateStruct(dst any) error {
	t := reflect.TypeOf(dst)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	err := validatorInstance().Struct(dst)
	if err == nil {
		return nil
	}
	var fieldErrs validator.ValidationErrors
	if !errors.As(err, &fieldErrs) {
		return apperr.Internal(err)
	}
	details := make(map[string]string, len(fieldErrs))
	for _, fe := range fieldErrs {
		key := fieldPath(fe.Namespace())
		if _, seen := details[key]; !seen {
			details[key] = viMessage(fe)
		}
	}
	return apperr.Validation(details)
}

// fieldPath bỏ tên struct gốc khỏi namespace của validator: "createReq.items[0].title" → "items[0].title".
func fieldPath(namespace string) string {
	_, rest, found := strings.Cut(namespace, ".")
	if !found {
		return namespace
	}
	return rest
}
