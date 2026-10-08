// Package apperr là kiểu lỗi ứng dụng duy nhất mang HTTP status, mã lỗi và thông điệp tiếng Việt cho client.
package apperr

import (
	"errors"
	"net/http"

	"lms/api/internal/domain"
)

// Error là lỗi trả cho client. Message và Details an toàn để hiển thị; cause chỉ để log, không bao giờ vào response.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return e.Code + ": " + e.Message + ": " + e.cause.Error()
	}
	return e.Code + ": " + e.Message
}

// Unwrap trả lỗi gốc để errors.Is/As đi xuyên qua.
func (e *Error) Unwrap() error { return e.cause }

// New tạo lỗi không có cause.
func New(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

// Wrap tạo lỗi giữ cause để log và errors.Is.
func Wrap(cause error, status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg, cause: cause}
}

// Validation là 400 VALIDATION_FAILED; details ánh xạ tên field JSON sang thông điệp.
func Validation(details map[string]string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: CodeValidationFailed, Message: Message(CodeValidationFailed), Details: details}
}

// NotFound là 404 "Không tìm thấy <what>".
func NotFound(what string) *Error {
	return New(http.StatusNotFound, CodeNotFound, "Không tìm thấy "+what)
}

// Forbidden là 403 FORBIDDEN.
func Forbidden() *Error {
	return New(http.StatusForbidden, CodeForbidden, Message(CodeForbidden))
}

// Unauthenticated là 401 UNAUTHENTICATED.
func Unauthenticated() *Error {
	return New(http.StatusUnauthorized, CodeUnauthenticated, Message(CodeUnauthenticated))
}

// Conflict là 409 CONFLICT với thông điệp cụ thể.
func Conflict(msg string) *Error {
	return New(http.StatusConflict, CodeConflict, msg)
}

// Internal là 500 INTERNAL; cause chỉ để log.
func Internal(cause error) *Error {
	return Wrap(cause, http.StatusInternalServerError, CodeInternal, Message(CodeInternal))
}

// Cause trả lỗi gốc (có thể nil) để log.
func (e *Error) Cause() error { return e.cause }

var domainKinds = map[domain.Kind]struct {
	status int
	code   string
}{
	domain.KindInvalid:    {http.StatusBadRequest, CodeValidationFailed},
	domain.KindTransition: {http.StatusConflict, CodeInvalidTransition},
	domain.KindNotFound:   {http.StatusNotFound, CodeNotFound},
	domain.KindConflict:   {http.StatusConflict, CodeConflict},
	domain.KindForbidden:  {http.StatusForbidden, CodeForbidden},
	domain.KindImmutable:  {http.StatusConflict, CodeVersionImmutable},
	domain.KindInUse:      {http.StatusConflict, CodeInUse},
}

// FromDomain chuyển lỗi bất kỳ thành *Error: giữ nguyên *Error có sẵn trong chuỗi, ánh xạ domain.Error theo Kind,
// còn lại là Internal. nil trả nil.
func FromDomain(err error) *Error {
	if err == nil {
		return nil
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}
	var de *domain.Error
	if errors.As(err, &de) {
		if m, ok := domainKinds[de.Kind]; ok {
			msg := de.Msg
			if msg == "" {
				msg = Message(m.code)
			}
			return Wrap(err, m.status, m.code, msg)
		}
	}
	return Internal(err)
}

// Is cho biết err (hoặc lỗi bọc trong nó) là *Error có mã code.
func Is(err error, code string) bool {
	var ae *Error
	return errors.As(err, &ae) && ae.Code == code
}
