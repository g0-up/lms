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

func homeworkPayload() map[string]any {
	return map[string]any{
		"Name": "An Nguyễn", "ClassName": "Backend cơ bản K01", "ClassCode": "BASIC01",
		"LessonTitle": "Bài tập: Thiết kế CSDL", "TotalPoints": 8, "MaxPoints": 10, "ResultLabel": "Đạt",
		"Comment": "Bảng <users> thiếu khóa ngoại.", "Updated": false,
		"ResultURL": "http://localhost:5173/learn/classes/c1/lessons/l1",
	}
}

func TestRenderHomeworkGraded(t *testing.T) {
	r := testRenderer(t)
	m, err := r.Render(TemplateHomeworkGraded, "an.nguyen@gmail.com", homeworkPayload())
	require.NoError(t, err)
	assert.Equal(t, "[GoUp LMS] Bài đã được chấm: Bài tập: Thiết kế CSDL · Đạt", m.Subject)
	for _, body := range []string{m.Text, m.HTML} {
		assert.Contains(t, body, "An Nguyễn")
		assert.Contains(t, body, "Backend cơ bản K01")
		assert.Contains(t, body, "8/10")
		assert.Contains(t, body, "Đạt")
		assert.Contains(t, body, "http://localhost:5173/learn/classes/c1/lessons/l1")
	}
	assert.Contains(t, m.Text, "Bảng <users> thiếu khóa ngoại.")
	assert.Contains(t, m.HTML, "Bảng &lt;users&gt; thiếu khóa ngoại.", "nhận xét của giảng viên được escape trong HTML")

	data := homeworkPayload()
	data["Updated"], data["ResultLabel"] = true, "Cần làm lại"
	m, err = r.Render(TemplateHomeworkGraded, "an.nguyen@gmail.com", data)
	require.NoError(t, err)
	assert.Equal(t, "[GoUp LMS] Điểm đã được cập nhật: Bài tập: Thiết kế CSDL · Cần làm lại", m.Subject)
	assert.Contains(t, m.Text, "cập nhật điểm")
}

func quizPayload() map[string]any {
	return map[string]any{
		"Name": "Cường Lê", "ClassName": "Backend cơ bản K01", "ClassCode": "BASIC01",
		"QuizTitle": "Kiểm tra cuối chặng Go", "AttemptNo": 2, "Percent": 60, "ResultLabel": "Chưa đạt",
		"Assessment": true, "Updated": false, "Comment": "Ôn lại bài <Cài đặt> trước khi xin thêm lượt.",
		"ResultURL": "http://localhost:5173/learn/classes/c1/lessons/l1?result=a1",
	}
}

func TestRenderQuizReviewed(t *testing.T) {
	r := testRenderer(t)
	m, err := r.Render(TemplateQuizReviewed, "cuong.le@outlook.com", quizPayload())
	require.NoError(t, err)
	assert.Equal(t, "[GoUp LMS] Giảng viên đã xác nhận kết quả: Kiểm tra cuối chặng Go lượt #2 · 60% · Chưa đạt", m.Subject)
	for _, body := range []string{m.Text, m.HTML} {
		assert.Contains(t, body, "Cường Lê")
		assert.Contains(t, body, "Backend cơ bản K01")
		assert.Contains(t, body, "lượt #2")
		assert.Contains(t, body, "60%")
		assert.Contains(t, body, "Chưa đạt")
		assert.Contains(t, body, "http://localhost:5173/learn/classes/c1/lessons/l1?result=a1")
	}
	assert.Contains(t, m.Text, "Ôn lại bài <Cài đặt> trước khi xin thêm lượt.")
	assert.Contains(t, m.HTML, "Ôn lại bài &lt;Cài đặt&gt;", "nhận xét của giảng viên được escape trong HTML")

	data := quizPayload()
	data["Updated"] = true
	m, err = r.Render(TemplateQuizReviewed, "cuong.le@outlook.com", data)
	require.NoError(t, err)
	assert.Equal(t, "[GoUp LMS] Nhận xét đã được cập nhật: Kiểm tra cuối chặng Go lượt #2 · 60% · Chưa đạt", m.Subject)
	assert.Contains(t, m.Text, "cập nhật nhận xét")

	data = quizPayload()
	data["Assessment"], data["QuizTitle"], data["AttemptNo"] = false, "Goroutine và channel", 1
	delete(data, "Percent")
	delete(data, "ResultLabel")
	m, err = r.Render(TemplateQuizReviewed, "cuong.le@outlook.com", data)
	require.NoError(t, err, "quiz học liệu không cần % và kết quả")
	assert.Equal(t, "[GoUp LMS] Giảng viên đã nhận xét: Goroutine và channel lượt #1", m.Subject)
	for _, body := range []string{m.Text, m.HTML} {
		assert.NotContains(t, body, "%")
		assert.NotContains(t, body, "Kết quả:")
	}
}

func digestPayload() map[string]any {
	return map[string]any{
		"Name": "Lê Thu Hương", "Count": 2, "QueueURL": "http://localhost:5173/teach/grading?overdue=1",
		"Items": []any{
			map[string]any{
				"StudentName": "Bích Trần", "KindLabel": "bài tập", "ClassCode": "BASIC01", "ClassName": "Backend cơ bản K01",
				"LessonTitle": "Bài tập: Thiết kế <CSDL>", "Waited": "4,5 ngày",
			},
			map[string]any{
				"StudentName": "Hà Phạm", "KindLabel": "quiz", "ClassCode": "BASIC01", "ClassName": "Backend cơ bản K01",
				"LessonTitle": "Kiểm tra cơ sở dữ liệu", "Waited": "3,5 ngày",
			},
		},
	}
}

func TestRenderGradingDigest(t *testing.T) {
	r := testRenderer(t)
	m, err := r.Render(TemplateGradingDigest, "huong.le@goup.vn", digestPayload())
	require.NoError(t, err)
	assert.Equal(t, "[GoUp LMS] 2 bài chờ chấm hoặc xác nhận quá 3 ngày: Bích Trần (bài tập), Hà Phạm (quiz)", m.Subject)
	for _, body := range []string{m.Text, m.HTML} {
		assert.Contains(t, body, "Lê Thu Hương")
		assert.Contains(t, body, "Bích Trần")
		assert.Contains(t, body, "Hà Phạm")
		assert.Contains(t, body, "Backend cơ bản K01")
		assert.Contains(t, body, "Kiểm tra cơ sở dữ liệu")
		assert.Contains(t, body, "4,5 ngày")
		assert.Contains(t, body, "3,5 ngày")
		assert.Contains(t, body, "http://localhost:5173/teach/grading?overdue=1")
	}
	assert.Contains(t, m.Text, "Bài tập: Thiết kế <CSDL>")
	assert.Contains(t, m.HTML, "Bài tập: Thiết kế &lt;CSDL&gt;", "tiêu đề học liệu được escape trong HTML")
}

func reminderPayload() map[string]any {
	return map[string]any{
		"Name": "An Nguyễn", "ClassCode": "BASIC01", "ClassName": "Backend cơ bản K01",
		"LessonTitle": "Bài tập: Goroutine", "DueLabel": "20/10/2026 17:00",
		"LessonURL": "http://localhost:5173/learn/classes/c1/lessons/l1",
	}
}

func TestRenderDeadlineReminder(t *testing.T) {
	r := testRenderer(t)
	m, err := r.Render(TemplateDeadlineReminder, "an.nguyen@gmail.com", reminderPayload())
	require.NoError(t, err)
	assert.Equal(t, "[GoUp LMS] Nhắc hạn nộp: Bài tập: Goroutine · hạn 20/10/2026 17:00", m.Subject)
	for _, body := range []string{m.Text, m.HTML} {
		assert.Contains(t, body, "An Nguyễn")
		assert.Contains(t, body, "Bài tập: Goroutine")
		assert.Contains(t, body, "Backend cơ bản K01")
		assert.Contains(t, body, "20/10/2026 17:00")
		assert.Contains(t, body, "http://localhost:5173/learn/classes/c1/lessons/l1")
	}

	data := reminderPayload()
	delete(data, "LessonURL")
	_, err = r.Render(TemplateDeadlineReminder, "an.nguyen@gmail.com", data)
	require.Error(t, err, "thiếu link phải lỗi thay vì gửi email không có đường dẫn")
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
	for _, name := range []TemplateName{TemplateInvite, TemplateAdded, TemplateResend, TemplatePasswordReset, TemplateHomeworkGraded,
		TemplateQuizReviewed, TemplateGradingDigest, TemplateDeadlineReminder} {
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
