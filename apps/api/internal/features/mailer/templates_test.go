package mailer

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRenderer(t *testing.T) *Renderer {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)
	r, err := NewRenderer(loc)
	require.NoError(t, err)
	return r
}

func classPayload() map[string]any {
	return map[string]any{
		"Name": "Bùi Minh", "ClassName": "Backend cơ bản K01", "ClassCode": "BASIC01",
		"LoginURL": "http://localhost:5173/login", "Email": "minh.bui@gmail.com",
		"ExpiresAt": "2026-10-08T02:00:00Z",
	}
}

func TestRenderInviteAndResend(t *testing.T) {
	r := testRenderer(t)
	for name, subject := range map[TemplateName]string{
		TemplateInvite: "[GoUp LMS] Lời mời vào lớp BASIC01",
		TemplateResend: "[GoUp LMS] Mật khẩu tạm mới cho lớp BASIC01",
	} {
		data := classPayload()
		data["TempPassword"] = "Ab3dEf7hJk9m"
		m, err := r.Render(name, "minh.bui@gmail.com", data)
		require.NoError(t, err, name)
		assert.Equal(t, "minh.bui@gmail.com", m.To)
		assert.Equal(t, subject, m.Subject)
		for _, body := range []string{m.Text, m.HTML} {
			assert.Contains(t, body, "Bùi Minh", name)
			assert.Contains(t, body, "Backend cơ bản K01", name)
			assert.Contains(t, body, "Ab3dEf7hJk9m", name)
			assert.Contains(t, body, "http://localhost:5173/login", name)
			assert.Contains(t, body, "minh.bui@gmail.com", name)
			assert.Contains(t, body, "09:00 ngày 08/10/2026", name, "hạn hiển thị theo giờ Việt Nam")
			assert.Contains(t, body, AppName, name)
		}
	}
}

func TestRenderAdded(t *testing.T) {
	r := testRenderer(t)
	data := classPayload()
	delete(data, "Email")
	delete(data, "ExpiresAt")
	m, err := r.Render(TemplateAdded, "an.nguyen@gmail.com", data)
	require.NoError(t, err)
	assert.Equal(t, "[GoUp LMS] Bạn đã được thêm vào lớp BASIC01", m.Subject)
	assert.Contains(t, m.Text, "Backend cơ bản K01")
	assert.Contains(t, m.HTML, "http://localhost:5173/login")
	assert.NotContains(t, strings.ToLower(m.Text), "mật khẩu tạm:")
	assert.NotContains(t, m.Text, "email mời trước đó", "tài khoản đã kích hoạt không cần nhắc mật khẩu tạm")

	data["UsePreviousTempPassword"] = true
	m, err = r.Render(TemplateAdded, "an.nguyen@gmail.com", data)
	require.NoError(t, err)
	for _, body := range []string{m.Text, m.HTML} {
		assert.Contains(t, body, "mật khẩu tạm trong email mời trước đó", "tài khoản còn mật khẩu tạm được nhắc dùng lại")
		assert.Contains(t, body, "http://localhost:5173/login")
	}
	assert.NotContains(t, strings.ToLower(m.Text), "mật khẩu tạm:")
}

func TestRenderPasswordReset(t *testing.T) {
	r := testRenderer(t)
	link := "http://localhost:5173/reset-password#token=abc_DEF-123"
	m, err := r.Render(TemplatePasswordReset, "huong.le@goup.vn", map[string]any{
		"Name": "Lê Hương", "ExpiresMinutes": 30, "Link": link,
	})
	require.NoError(t, err)
	assert.Equal(t, "[GoUp LMS] Đặt lại mật khẩu", m.Subject)
	assert.Contains(t, m.Text, link)
	assert.Contains(t, m.HTML, link)
	assert.Contains(t, m.Text, "30 phút")
}

func TestRenderEscapesHTML(t *testing.T) {
	r := testRenderer(t)
	data := classPayload()
	data["Name"] = `<script>alert("x")</script>`
	data["TempPassword"] = "Ab3dEf7hJk9m"
	m, err := r.Render(TemplateInvite, "minh.bui@gmail.com", data)
	require.NoError(t, err)
	assert.NotContains(t, m.HTML, "<script>")
	assert.Contains(t, m.HTML, "&lt;script&gt;")
}

func TestRenderMissingKeyFailsWithoutLeakingValues(t *testing.T) {
	r := testRenderer(t)
	data := classPayload()
	_, err := r.Render(TemplateInvite, "minh.bui@gmail.com", data)
	require.Error(t, err, "thiếu TempPassword phải lỗi thay vì gửi email rỗng mật khẩu")
	assert.Contains(t, err.Error(), "TempPassword")
	assert.NotContains(t, err.Error(), "minh.bui@gmail.com")

	_, err = r.Render("unknown", "x@example.com", data)
	require.ErrorIs(t, err, ErrUnknownTemplate)
}

func TestEveryTemplateHasSubjectAndBodies(t *testing.T) {
	r := testRenderer(t)
	for _, name := range []TemplateName{TemplateInvite, TemplateAdded, TemplateResend, TemplatePasswordReset} {
		assert.True(t, name.Valid())
		assert.Contains(t, r.html, name)
		assert.Contains(t, r.text, name)
		assert.Contains(t, r.subject, name)
	}
	assert.False(t, TemplateName("account_disabled").Valid())
}

func TestFormatTime(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)
	at := time.Date(2026, 10, 8, 2, 0, 0, 0, time.UTC)
	assert.Equal(t, "09:00 ngày 08/10/2026", formatTime(at, loc))
	assert.Equal(t, "09:00 ngày 08/10/2026", formatTime(&at, loc))
	assert.Equal(t, "09:00 ngày 08/10/2026", formatTime("2026-10-08T09:00:00+07:00", loc))
	assert.Equal(t, "", formatTime((*time.Time)(nil), loc))
	assert.Equal(t, "khong-phai-time", formatTime("khong-phai-time", loc))
	assert.Equal(t, "42", formatTime(42, loc))

	r, err := NewRenderer(nil)
	require.NoError(t, err)
	assert.NotNil(t, r)
}
