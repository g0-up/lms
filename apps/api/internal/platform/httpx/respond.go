// Package httpx gom phần I/O HTTP dùng chung của handler: đọc JSON có kiểm tra, trả dữ liệu và trả lỗi theo
// envelope {"error":{"code","message","details"}}.
package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"lms/api/internal/platform/apperr"
)

// RequestIDKey là khóa gin.Context chứa request id do middleware.RequestID đặt.
const RequestIDKey = "request_id"

// StatusClientClosedRequest là status ghi log khi client hủy request trước khi có phản hồi (quy ước của nginx);
// client không còn nhận được nó.
const StatusClientClosedRequest = 499

// ErrorBody là phần "error" của envelope lỗi.
type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// ErrorEnvelope là body JSON của mọi phản hồi lỗi.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// OK trả data dạng JSON với status.
func OK(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

// NoContent trả 204 không body.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
	c.Writer.WriteHeaderNow()
}

// Fail trả lỗi theo envelope và dừng chuỗi handler. *apperr.Error và domain.Error giữ status/mã/thông điệp của
// chúng; lỗi khác thành 500 INTERNAL không lộ thông điệp gốc, kèm log ERROR có request_id để tra (lỗi 5xx không
// có cause, như apperr.Internal(nil) sau khi Recover đã log stack, không log lại). 429 có
// Details["retry_after"] (giây) thì đặt thêm header Retry-After. Lỗi 5xx khi client đã hủy request (điều hướng hủy
// query) là hệ quả của việc hủy, không phải sự cố: trả 499 không body và không log ERROR.
func Fail(c *gin.Context, err error) {
	if err == nil {
		err = errors.New("httpx: Fail gọi với lỗi nil")
	}
	ae := apperr.FromDomain(err)
	if ae.Status >= http.StatusInternalServerError && errors.Is(c.Request.Context().Err(), context.Canceled) {
		c.AbortWithStatus(StatusClientClosedRequest)
		return
	}
	if ae.Status >= http.StatusInternalServerError && ae.Cause() != nil {
		slog.Default().ErrorContext(c.Request.Context(), "request failed",
			slog.String("request_id", c.GetString(RequestIDKey)),
			slog.String("code", ae.Code),
			slog.Any("err", err),
		)
	}
	if ae.Status == http.StatusTooManyRequests {
		if ra := ae.Details["retry_after"]; ra != "" {
			c.Header("Retry-After", ra)
		}
	}
	c.AbortWithStatusJSON(ae.Status, ErrorEnvelope{Error: ErrorBody{Code: ae.Code, Message: ae.Message, Details: ae.Details}})
}
