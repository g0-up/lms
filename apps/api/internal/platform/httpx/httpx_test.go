package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/httpx"
)

func init() { gin.SetMode(gin.TestMode) }

type inviteReq struct {
	Email    string   `json:"email" binding:"required,email"`
	FullName string   `json:"fullName" binding:"required,min=2,max=120"`
	Role     string   `json:"role" binding:"omitempty,oneof=admin teacher student"`
	ClassID  string   `json:"classId" binding:"omitempty,uuid"`
	Tags     []string `json:"tags" binding:"max=2"`
	Position int      `json:"position" binding:"omitempty,min=1,max=99"`
	Internal string   `json:"-"`
	NoTag    string   `binding:"omitempty,len=3"`
}

func bind(t *testing.T, body string, dst any) error {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/x", strings.NewReader(body))
	return httpx.BindJSON(c, dst)
}

func requireValidation(t *testing.T, err error, details map[string]string) {
	t.Helper()
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae), "%v", err)
	require.Equal(t, http.StatusBadRequest, ae.Status)
	require.Equal(t, apperr.CodeValidationFailed, ae.Code)
	require.Equal(t, details, ae.Details)
}

func TestBindJSONHappyPath(t *testing.T) {
	var req inviteReq
	err := bind(t, `{"email":"an.nguyen@gmail.com","fullName":"Nguyễn Hoàng An","role":"student","tags":["a"]}`, &req)
	require.NoError(t, err)
	require.Equal(t, "Nguyễn Hoàng An", req.FullName)
	require.Equal(t, []string{"a"}, req.Tags)
}

func TestBindJSONRejectsMalformedBodies(t *testing.T) {
	invalid := map[string]string{"body": "JSON không hợp lệ"}
	for name, body := range map[string]string{
		"unknown field": `{"email":"a@b.vn","fullName":"An","isAdmin":true}`,
		"syntax":        `{"email":`,
		"empty":         ``,
		"trailing":      `{"email":"a@b.vn","fullName":"An"} {"x":1}`,
		"wrong type":    `{"email":1,"fullName":"An"}`,
		"array":         `[]`,
	} {
		t.Run(name, func(t *testing.T) {
			var req inviteReq
			requireValidation(t, bind(t, body, &req), invalid)
		})
	}
}

func TestBindJSONRejectsLargeBody(t *testing.T) {
	big := `{"email":"a@b.vn","fullName":"` + strings.Repeat("a", httpx.MaxBodyBytes) + `"}`
	var req inviteReq
	requireValidation(t, bind(t, big, &req), map[string]string{"body": "Dữ liệu quá lớn (tối đa 64KB)"})

	// Giá trị đầu hợp lệ nhưng phần thừa vượt giới hạn vẫn bị chặn.
	padded := `{"email":"a@b.vn","fullName":"An"}` + strings.Repeat(" ", httpx.MaxBodyBytes)
	requireValidation(t, bind(t, padded, &req), map[string]string{"body": "Dữ liệu quá lớn (tối đa 64KB)"})
}

func TestBindJSONNilBody(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.Body = nil
	var req inviteReq
	requireValidation(t, httpx.BindJSON(c, &req), map[string]string{"body": "JSON không hợp lệ"})
}

func TestBindJSONVietnameseFieldErrors(t *testing.T) {
	var req inviteReq
	requireValidation(t, bind(t, `{}`, &req), map[string]string{
		"email":    "Bắt buộc",
		"fullName": "Bắt buộc",
	})

	err := bind(t, `{"email":"khong-phai-email","fullName":"A","role":"root","classId":"abc","tags":["a","b","c"],"position":100,"NoTag":"ab"}`, &req)
	requireValidation(t, err, map[string]string{
		"email":    "Email không hợp lệ",
		"fullName": "Phải có ít nhất 2 ký tự",
		"role":     "Giá trị phải là một trong: admin, teacher, student",
		"classId":  "Mã định danh không hợp lệ",
		"tags":     "Phải có tối đa 2 phần tử",
		"position": "Giá trị phải không quá 99",
		"NoTag":    "Phải có đúng 3 ký tự",
	})

	var req2 inviteReq
	err = bind(t, `{"email":"a@b.vn","fullName":"`+strings.Repeat("a", 121)+`","position":-1}`, &req2)
	requireValidation(t, err, map[string]string{
		"fullName": "Phải có tối đa 120 ký tự",
		"position": "Giá trị phải từ 1",
	})
}

type nested struct {
	Items []struct {
		Title string `json:"title" binding:"required,startswith=A"`
	} `json:"items" binding:"dive"`
}

func TestBindJSONNestedPathAndFallback(t *testing.T) {
	var req nested
	err := bind(t, `{"items":[{"title":"Bài 1"},{"title":""}]}`, &req)
	requireValidation(t, err, map[string]string{
		"items[0].title": "Giá trị không hợp lệ",
		"items[1].title": "Bắt buộc",
	})
}

func TestBindJSONNonStructSkipsValidation(t *testing.T) {
	var m map[string]any
	require.NoError(t, bind(t, `{"a":1}`, &m))
	require.Equal(t, map[string]any{"a": float64(1)}, m)
}

func serve(t *testing.T, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(httpx.RequestIDKey, "req-1"); c.Next() })
	r.GET("/x/:id", h)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x/abc", nil))
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]map[string]any {
	t.Helper()
	var body map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body
}

func TestFailWritesEnvelope(t *testing.T) {
	rec := serve(t, func(c *gin.Context) {
		httpx.Fail(c, apperr.Validation(map[string]string{"email": "Bắt buộc"}))
		require.True(t, c.IsAborted(), "Fail dừng các handler phía sau")
	})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, map[string]map[string]any{"error": {
		"code": "VALIDATION_FAILED", "message": "Dữ liệu không hợp lệ", "details": map[string]any{"email": "Bắt buộc"},
	}}, decode(t, rec))

	rec = serve(t, func(c *gin.Context) { httpx.Fail(c, domain.ErrVersionImmutable) })
	require.Equal(t, http.StatusConflict, rec.Code)
	body := decode(t, rec)
	require.Equal(t, "VERSION_IMMUTABLE", body["error"]["code"])
	require.Equal(t, "Phiên bản đã phát hành không thể sửa", body["error"]["message"])
	require.NotContains(t, body["error"], "details", "details bỏ khi rỗng")
}

func TestFailHidesInternalErrorAndLogsRequestID(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	rec := serve(t, func(c *gin.Context) { httpx.Fail(c, errors.New("pq: password=s3cr3t")) })
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.NotContains(t, rec.Body.String(), "s3cr3t")
	body := decode(t, rec)
	require.Equal(t, "INTERNAL", body["error"]["code"])
	require.Equal(t, "Lỗi hệ thống, vui lòng thử lại", body["error"]["message"])

	require.Contains(t, logs.String(), `"level":"ERROR"`)
	require.Contains(t, logs.String(), `"request_id":"req-1"`)

	rec = serve(t, func(c *gin.Context) { httpx.Fail(c, nil) })
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestFailOnCanceledRequestIsNotAnError(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := gin.New()
	r.GET("/x", func(c *gin.Context) { httpx.Fail(c, fmt.Errorf("query lessons: %w", context.Canceled)) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil).WithContext(ctx))

	require.Equal(t, httpx.StatusClientClosedRequest, rec.Code)
	require.Empty(t, rec.Body.String())
	require.NotContains(t, logs.String(), `"level":"ERROR"`)

	// Request còn sống: lỗi hủy context từ nơi khác vẫn là 500 có log.
	rec = serve(t, func(c *gin.Context) { httpx.Fail(c, context.Canceled) })
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, logs.String(), `"level":"ERROR"`)
}

func TestFailRateLimitedSetsRetryAfter(t *testing.T) {
	rec := serve(t, func(c *gin.Context) {
		e := apperr.New(http.StatusTooManyRequests, apperr.CodeRateLimited, apperr.Message(apperr.CodeRateLimited))
		e.Details = map[string]string{"retry_after": "17"}
		httpx.Fail(c, e)
	})
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "17", rec.Header().Get("Retry-After"))
}

func TestOKAndNoContent(t *testing.T) {
	rec := serve(t, func(c *gin.Context) { httpx.OK(c, http.StatusCreated, gin.H{"id": "1"}) })
	require.Equal(t, http.StatusCreated, rec.Code)
	require.JSONEq(t, `{"id":"1"}`, rec.Body.String())

	rec = serve(t, httpx.NoContent)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String())
}

func TestUUIDParam(t *testing.T) {
	rec := serve(t, func(c *gin.Context) {
		if _, err := httpx.UUIDParam(c, "id"); err != nil {
			httpx.Fail(c, err)
			return
		}
		c.Status(http.StatusOK)
	})
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "NOT_FOUND", decode(t, rec)["error"]["code"])

	id := uuid.Must(uuid.NewV7())
	r := gin.New()
	var got uuid.UUID
	r.GET("/x/:id", func(c *gin.Context) {
		var err error
		got, err = httpx.UUIDParam(c, "id")
		require.NoError(t, err)
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x/"+id.String(), nil))
	require.Equal(t, id, got)
}
