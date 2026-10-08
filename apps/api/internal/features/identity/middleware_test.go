package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/middleware"
)

// testRouter dựng /api/v1 như router thật: header CSRF, route identity và một route nghiệp vụ /stages sau
// SessionAuth + MustChangePassword.
func testRouter(t *testing.T) (*gin.Engine, *Service, *memStore, *clock.Fake) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc, m, clk := newMemService(t)
	svc.Cfg.AppEnv = "e2e" // tắt limiter IP để test không phụ thuộc thứ tự
	engine := gin.New()
	api := engine.Group("/api/v1", httpx.RequireCustomHeader())
	auth := NewMiddleware(svc, false)
	NewHandler(svc, svc.Cfg).Register(api, auth)
	authed := api.Group("", auth.SessionAuth(), auth.MustChangePassword())
	authed.GET("/stages", func(c *gin.Context) {
		uid, _ := c.Get(middleware.UserIDKey)
		c.JSON(http.StatusOK, gin.H{"user": uid})
	})
	authed.GET("/teacher-only", RequireRole(domain.RoleTeacher, domain.RoleAdmin), func(c *gin.Context) { c.Status(http.StatusOK) })
	return engine, svc, m, clk
}

type apiResp struct {
	*httptest.ResponseRecorder
}

func (r apiResp) errorCode(t *testing.T) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &body), r.Body.String())
	return body.Error.Code
}

func do(engine http.Handler, method, path, body string, cookies ...*http.Cookie) apiResp {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set(httpx.CustomHeader, httpx.CustomHeaderValue)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return apiResp{w}
}

func sessionCookie(t *testing.T, r apiResp) *http.Cookie {
	t.Helper()
	for _, c := range r.Result().Cookies() {
		if c.Name == cookieNameInsecure {
			return c
		}
	}
	t.Fatalf("không có cookie phiên: %v", r.Header().Values("Set-Cookie"))
	return nil
}

func TestCookieName(t *testing.T) {
	assert.Equal(t, "__Host-sid", CookieName(true))
	assert.Equal(t, "sid", CookieName(false))
}

func TestLoginSetsSessionCookieAndMeWorks(t *testing.T) {
	engine, _, m, clk := testRouter(t)
	addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)

	r := do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"huong.le@goup.vn","password":"`+goodPassword+`"}`)
	require.Equal(t, http.StatusOK, r.Code, r.Body.String())
	raw := r.Header().Get("Set-Cookie")
	assert.Contains(t, raw, "HttpOnly")
	assert.Contains(t, raw, "SameSite=Lax")
	assert.Contains(t, raw, "Path=/")
	assert.NotContains(t, raw, "Max-Age")
	assert.NotContains(t, raw, "Secure")
	assert.NotContains(t, r.Body.String(), "password")
	var body loginResponse
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &body))
	assert.Equal(t, "huong.le@goup.vn", body.User.Email)
	assert.Nil(t, body.User.TempPasswordExpiresAt)

	cookie := sessionCookie(t, r)
	me := do(engine, http.MethodGet, "/api/v1/auth/me", "", cookie)
	require.Equal(t, http.StatusOK, me.Code)
	assert.Contains(t, me.Body.String(), `"role":"teacher"`)

	assert.Equal(t, http.StatusOK, do(engine, http.MethodGet, "/api/v1/stages", "", cookie).Code)
	assert.Equal(t, http.StatusOK, do(engine, http.MethodGet, "/api/v1/teacher-only", "", cookie).Code)
}

func TestSecureCookieUsesHostPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, m, clk := newMemService(t)
	svc.Cfg.AppEnv, svc.Cfg.CookieSecure = "e2e", true
	addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	engine := gin.New()
	NewHandler(svc, svc.Cfg).Register(engine.Group("/api/v1"), NewMiddleware(svc, true))

	r := do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"huong.le@goup.vn","password":"`+goodPassword+`"}`)
	require.Equal(t, http.StatusOK, r.Code)
	raw := r.Header().Get("Set-Cookie")
	assert.True(t, strings.HasPrefix(raw, "__Host-sid="), raw)
	assert.Contains(t, raw, "Secure")
}

func TestSessionAuthRejectsMissingAndUnknownCookie(t *testing.T) {
	engine, _, _, _ := testRouter(t)
	r := do(engine, http.MethodGet, "/api/v1/auth/me", "")
	assert.Equal(t, http.StatusUnauthorized, r.Code)
	assert.Equal(t, apperr.CodeUnauthenticated, r.errorCode(t))
	r = do(engine, http.MethodGet, "/api/v1/stages", "", &http.Cookie{Name: "sid", Value: "khong-ton-tai"})
	assert.Equal(t, http.StatusUnauthorized, r.Code)
}

func TestSessionAuthRejectsDisabledUserImmediately(t *testing.T) {
	engine, svc, m, clk := testRouter(t)
	admin := addUser(t, m, internalUser(t, clk.Now(), "quan.tran@goup.vn", domain.RoleAdmin), goodPassword)
	teacher := addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	r := do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"huong.le@goup.vn","password":"`+goodPassword+`"}`)
	cookie := sessionCookie(t, r)

	_, err := svc.DisableUser(context.Background(), admin, teacher.ID(), "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, do(engine, http.MethodGet, "/api/v1/auth/me", "", cookie).Code)
}

func TestMustChangePasswordAllowlist(t *testing.T) {
	engine, _, m, clk := testRouter(t)
	addUser(t, m, invitedStudent(t, clk.Now(), "minh.bui@gmail.com"), goodPassword)
	r := do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"minh.bui@gmail.com","password":"`+goodPassword+`"}`)
	require.Equal(t, http.StatusOK, r.Code)
	var body loginResponse
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &body))
	assert.True(t, body.User.MustChangePassword)
	require.NotNil(t, body.User.TempPasswordExpiresAt)
	cookie := sessionCookie(t, r)

	blocked := do(engine, http.MethodGet, "/api/v1/stages", "", cookie)
	assert.Equal(t, http.StatusForbidden, blocked.Code)
	assert.Equal(t, apperr.CodePasswordChangeRequired, blocked.errorCode(t))
	assert.Equal(t, http.StatusForbidden, do(engine, http.MethodGet, "/api/v1/users?role=teacher", "", cookie).Code)

	assert.Equal(t, http.StatusOK, do(engine, http.MethodGet, "/api/v1/auth/me", "", cookie).Code)
	changed := do(engine, http.MethodPost, "/api/v1/auth/change-password", `{"newPassword":"matkhaumoi1","confirmPassword":"matkhaumoi1"}`, cookie)
	require.Equal(t, http.StatusOK, changed.Code, changed.Body.String())
	assert.Contains(t, changed.Body.String(), `"mustChangePassword":false`)
	assert.Contains(t, changed.Body.String(), `"status":"active"`)
	assert.NotContains(t, changed.Body.String(), "tempPasswordExpiresAt")

	assert.Equal(t, http.StatusOK, do(engine, http.MethodGet, "/api/v1/stages", "", cookie).Code)
	assert.Equal(t, http.StatusForbidden, do(engine, http.MethodGet, "/api/v1/teacher-only", "", cookie).Code)
}

func TestLogoutAlwaysNoContentAndClearsCookie(t *testing.T) {
	engine, _, m, clk := testRouter(t)
	addUser(t, m, invitedStudent(t, clk.Now(), "minh.bui@gmail.com"), goodPassword)
	cookie := sessionCookie(t, do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"minh.bui@gmail.com","password":"`+goodPassword+`"}`))

	r := do(engine, http.MethodPost, "/api/v1/auth/logout", "", cookie)
	assert.Equal(t, http.StatusNoContent, r.Code)
	assert.Contains(t, r.Header().Get("Set-Cookie"), "Max-Age=0")
	assert.Empty(t, m.sessions)
	assert.Equal(t, http.StatusUnauthorized, do(engine, http.MethodGet, "/api/v1/auth/me", "", cookie).Code)

	assert.Equal(t, http.StatusNoContent, do(engine, http.MethodPost, "/api/v1/auth/logout", "").Code)
}

func TestMutationWithoutCustomHeaderIsForbidden(t *testing.T) {
	engine, _, _, _ := testRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestForgotPasswordResponseIsUniform(t *testing.T) {
	engine, _, m, clk := testRouter(t)
	addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)

	var bodies []string
	for _, email := range []string{"huong.le@goup.vn", "khongtontai@example.com", "khong-phai-email"} {
		r := do(engine, http.MethodPost, "/api/v1/auth/forgot-password", `{"email":"`+email+`"}`)
		assert.Equal(t, http.StatusAccepted, r.Code, email)
		bodies = append(bodies, r.Body.String())
	}
	assert.Equal(t, bodies[0], bodies[1])
	assert.Equal(t, bodies[0], bodies[2])
	assert.Contains(t, bodies[0], forgotMessage)
	assert.Len(t, m.mails, 1)

	assert.Equal(t, http.StatusBadRequest, do(engine, http.MethodPost, "/api/v1/auth/forgot-password", `{"email":`).Code)
}

func TestResetPasswordEndpoint(t *testing.T) {
	engine, _, m, clk := testRouter(t)
	addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	require.Equal(t, http.StatusAccepted, do(engine, http.MethodPost, "/api/v1/auth/forgot-password", `{"email":"huong.le@goup.vn"}`).Code)
	token := m.mails[0].Secret

	body := `{"token":"` + token + `","newPassword":"matkhaumoi1","confirmPassword":"matkhaumoi1"}`
	assert.Equal(t, http.StatusNoContent, do(engine, http.MethodPost, "/api/v1/auth/reset-password", body).Code)
	again := do(engine, http.MethodPost, "/api/v1/auth/reset-password", body)
	assert.Equal(t, http.StatusBadRequest, again.Code)
	assert.Equal(t, apperr.CodeValidationFailed, again.errorCode(t))

	r := do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"huong.le@goup.vn","password":"matkhaumoi1"}`)
	assert.Equal(t, http.StatusOK, r.Code)
}

func TestLoginErrorResponses(t *testing.T) {
	engine, _, m, clk := testRouter(t)
	addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)

	bad := do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"x","password":"y"}`)
	assert.Equal(t, http.StatusUnprocessableEntity, bad.Code)
	for range 5 {
		r := do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"huong.le@goup.vn","password":"sai"}`)
		require.Equal(t, http.StatusUnauthorized, r.Code)
		assert.Empty(t, r.Header().Get("Set-Cookie"))
	}
	clk.Advance(time.Minute)
	locked := do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"huong.le@goup.vn","password":"`+goodPassword+`"}`)
	assert.Equal(t, http.StatusTooManyRequests, locked.Code)
	assert.Equal(t, "840", locked.Header().Get("Retry-After"))
	assert.Equal(t, apperr.CodeTooManyAttempts, locked.errorCode(t))
}

func TestAdminUserEndpoints(t *testing.T) {
	engine, _, m, clk := testRouter(t)
	addUser(t, m, internalUser(t, clk.Now(), "quan.tran@goup.vn", domain.RoleAdmin), goodPassword)
	teacher := addUser(t, m, internalUser(t, clk.Now(), "huong.le@goup.vn", domain.RoleTeacher), goodPassword)
	admin := sessionCookie(t, do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"quan.tran@goup.vn","password":"`+goodPassword+`"}`))
	asTeacher := sessionCookie(t, do(engine, http.MethodPost, "/api/v1/auth/login", `{"email":"huong.le@goup.vn","password":"`+goodPassword+`"}`))

	list := do(engine, http.MethodGet, "/api/v1/users?role=teacher&status=active", "", admin)
	require.Equal(t, http.StatusOK, list.Code)
	assert.Contains(t, list.Body.String(), teacher.ID().String())
	assert.NotContains(t, list.Body.String(), "password")
	assert.Equal(t, http.StatusBadRequest, do(engine, http.MethodGet, "/api/v1/users?role=boss", "", admin).Code)
	assert.Equal(t, http.StatusBadRequest, do(engine, http.MethodGet, "/api/v1/users?role=teacher&status=x", "", admin).Code)

	forbidden := do(engine, http.MethodGet, "/api/v1/users?role=teacher", "", asTeacher)
	assert.Equal(t, http.StatusForbidden, forbidden.Code)
	assert.Equal(t, apperr.CodeForbidden, forbidden.errorCode(t))

	path := "/api/v1/users/" + teacher.ID().String()
	r := do(engine, http.MethodPost, path+"/disable", "", admin)
	require.Equal(t, http.StatusOK, r.Code, r.Body.String())
	assert.Contains(t, r.Body.String(), `"status":"disabled"`)
	assert.Equal(t, http.StatusConflict, do(engine, http.MethodPost, path+"/disable", "", admin).Code)
	assert.Equal(t, http.StatusOK, do(engine, http.MethodPost, path+"/enable", "", admin).Code)
	assert.Equal(t, http.StatusNotFound, do(engine, http.MethodPost, "/api/v1/users/"+uuid.NewString()+"/enable", "", admin).Code)
	assert.Equal(t, http.StatusNotFound, do(engine, http.MethodPost, "/api/v1/users/not-a-uuid/enable", "", admin).Code)
}

func TestRequireRoleWithoutSessionIsUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/x", RequireRole(domain.RoleAdmin), func(c *gin.Context) { c.Status(http.StatusOK) })
	m := &Middleware{}
	engine.GET("/y", m.MustChangePassword(), func(c *gin.Context) { c.Status(http.StatusOK) })
	assert.Equal(t, http.StatusUnauthorized, do(engine, http.MethodGet, "/x", "").Code)
	assert.Equal(t, http.StatusUnauthorized, do(engine, http.MethodGet, "/y", "").Code)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, ok := CurrentUser(c)
	assert.False(t, ok)
	_, ok = CurrentSessionID(c)
	assert.False(t, ok)
}
