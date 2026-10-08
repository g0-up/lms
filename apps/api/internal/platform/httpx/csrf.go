package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"lms/api/internal/platform/apperr"
)

// CustomHeader và CustomHeaderValue là header web client gửi kèm mọi request thay đổi dữ liệu. Form HTML hay
// thẻ <img> của trang khác không đặt được header tùy ý, nên thiếu header là dấu hiệu CSRF.
const (
	CustomHeader      = "X-Requested-With"
	CustomHeaderValue = "fetch"
)

// RequireCustomHeader là lớp CSRF thứ hai: POST/PUT/PATCH/DELETE thiếu X-Requested-With: fetch → 403 FORBIDDEN.
// Method an toàn (GET, HEAD, OPTIONS) đi qua.
func RequireCustomHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if !strings.EqualFold(c.GetHeader(CustomHeader), CustomHeaderValue) {
				Fail(c, apperr.Forbidden())
				return
			}
		}
		c.Next()
	}
}

// CrossOriginProtection là lớp CSRF thứ nhất: bọc h bằng http.CrossOriginProtection (Sec-Fetch-Site / Origin),
// từ chối request không an toàn đến từ origin khác bằng 403 FORBIDDEN theo envelope lỗi chung. trustedOrigins là
// URL gốc được tin (ví dụ PUBLIC_BASE_URL khi web và API khác origin qua proxy); phần path bị bỏ, rỗng bị bỏ qua.
func CrossOriginProtection(h http.Handler, trustedOrigins ...string) (http.Handler, error) {
	p := http.NewCrossOriginProtection()
	for _, raw := range trustedOrigins {
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("httpx: origin tin cậy không hợp lệ: %q", raw)
		}
		if err := p.AddTrustedOrigin(u.Scheme + "://" + u.Host); err != nil {
			return nil, fmt.Errorf("httpx: origin tin cậy %q: %w", raw, err)
		}
	}
	p.SetDenyHandler(http.HandlerFunc(denyCrossOrigin))
	return p.Handler(h), nil
}

func denyCrossOrigin(w http.ResponseWriter, _ *http.Request) {
	e := apperr.Forbidden()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{Error: ErrorBody{Code: e.Code, Message: e.Message}})
}
