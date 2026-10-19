//go:build integration

package grading_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/features/grading"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/db"
)

// jobsNow là 09:00 ngày 19/10/2026 giờ Việt Nam: đã qua giờ gửi tổng hợp. Seed tính mọi mốc tương đối so với nó.
var jobsNow = time.Date(2026, 10, 19, 2, 0, 0, 0, time.UTC)

func vn(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)
	return loc
}

// newJobs dựng Jobs như worker: transaction thường, outbox thật, múi giờ Việt Nam.
func (e *itEnv) newJobs() *grading.Jobs {
	return grading.NewJobs(grading.JobsDeps{
		Tx: db.TxRunner{DB: e.dbx}, Repo: grading.PGRepo{}, Mail: mailer.NewService(mailer.NewOutboxRepo(), nil, e.clk),
		Location: vn(e.t), PublicBaseURL: "http://localhost:5173", Logger: slog.New(slog.DiscardHandler),
	})
}

// runJobs chạy một lượt việc hằng ngày tại đồng hồ hiện tại của env.
func (e *itEnv) runJobs(jobs *grading.Jobs) {
	e.t.Helper()
	require.NoError(e.t, jobs.Cleanup(context.Background(), e.clk.Now()))
}

type outboxRow struct {
	To      string         `db:"to_email"`
	Payload map[string]any `db:"-"`
	Raw     []byte         `db:"payload"`
}

// outbox là các email của template đã enqueue, theo người nhận rồi thứ tự tạo.
func (e *itEnv) outbox(name mailer.TemplateName) []outboxRow {
	e.t.Helper()
	var rows []outboxRow
	require.NoError(e.t, e.dbx.Select(&rows, `SELECT to_email, payload FROM email_outbox WHERE template = $1
		ORDER BY to_email, id`, string(name)))
	for i := range rows {
		require.NoError(e.t, json.Unmarshal(rows[i].Raw, &rows[i].Payload))
	}
	return rows
}

func recipients(rows []outboxRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.To
	}
	return out
}

func (e *itEnv) runKeyExists(key string) bool {
	e.t.Helper()
	var n int
	require.NoError(e.t, e.dbx.Get(&n, `SELECT count(*) FROM scheduled_job_runs WHERE run_key = $1`, key))
	return n == 1
}

// overdueOf là số mục quá hạn trong hàng chờ của giảng viên, đọc qua chính API hàng chờ.
func (e *itEnv) overdueOf(email string) int {
	e.t.Helper()
	q, err := e.svc.Queue(context.Background(), e.actor(email), grading.Filter{})
	require.NoError(e.t, err)
	return q.OverdueCount
}

func TestDigestOncePerTeacherPerLocalDay(t *testing.T) {
	e := newITAt(t, jobsNow)
	jobs := e.newJobs()
	huongOverdue, baoOverdue := e.overdueOf(huongEmail), e.overdueOf(baoEmail)
	require.Positive(t, huongOverdue)
	require.Positive(t, baoOverdue)

	e.runJobs(jobs)
	e.clk.Advance(3 * time.Hour)
	e.runJobs(jobs)
	rows := e.outbox(mailer.TemplateGradingDigest)
	assert.Equal(t, []string{baoEmail, huongEmail}, recipients(rows), "chạy hai lần cùng ngày: mỗi giảng viên một email")
	assert.True(t, e.runKeyExists("grading_digest:"+e.userID(huongEmail).String()+":2026-10-19"))
	assert.EqualValues(t, baoOverdue, rows[0].Payload["Count"])
	assert.EqualValues(t, huongOverdue, rows[1].Payload["Count"], "đếm khớp hàng chờ quá hạn của giảng viên")
	assert.Len(t, rows[1].Payload["Items"], huongOverdue)

	e.clk.Set(jobsNow.Add(24 * time.Hour))
	e.runJobs(jobs)
	rows = e.outbox(mailer.TemplateGradingDigest)
	assert.Equal(t, []string{baoEmail, baoEmail, huongEmail, huongEmail}, recipients(rows),
		"ngày hôm sau vẫn còn mục quá hạn nên mỗi giảng viên nhận email mới")
	assert.True(t, e.runKeyExists("grading_digest:"+e.userID(huongEmail).String()+":2026-10-20"))
}

func TestDigestSkipsTeacherWithoutOverdueAndDisabledTeacher(t *testing.T) {
	e := newITAt(t, jobsNow)
	jobs := e.newJobs()
	// Phong rời basic02 thì bài 3,5 ngày của Phong rời hàng chờ; các mục còn lại của Bảo đều chưa quá 3 ngày.
	e.exec(`UPDATE class_members SET status = 'dropped', dropped_at = $2 WHERE id = $1`,
		e.memberID("basic02", phongEmail), e.clk.Now())
	require.Zero(t, e.overdueOf(baoEmail))

	e.runJobs(jobs)
	assert.Equal(t, []string{huongEmail}, recipients(e.outbox(mailer.TemplateGradingDigest)),
		"giảng viên không có mục quá hạn không nhận email")

	e.exec(`UPDATE users SET status = 'disabled', disabled_at = $2 WHERE id = $1`, e.userID(huongEmail), e.clk.Now())
	e.clk.Advance(24 * time.Hour)
	e.runJobs(jobs)
	require.Positive(t, e.overdueOf(adminEmail), "lớp của Hương vẫn còn mục quá hạn")
	assert.Equal(t, []string{huongEmail}, recipients(e.outbox(mailer.TemplateGradingDigest)),
		"giảng viên bị khóa không nhận email")
	assert.False(t, e.runKeyExists("grading_digest:"+e.userID(huongEmail).String()+":2026-10-20"))
}

// reminder là (người nhận, mã lớp, học liệu, hạn hiển thị) của một email nhắc hạn.
type reminder struct{ to, class, lesson, due string }

func reminders(rows []outboxRow) []reminder {
	out := make([]reminder, len(rows))
	for i, r := range rows {
		out[i] = reminder{r.To, r.Payload["ClassCode"].(string), r.Payload["LessonTitle"].(string),
			r.Payload["DueLabel"].(string)}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].to != out[j].to {
			return out[i].to < out[j].to
		}
		return out[i].due < out[j].due
	})
	return out
}

func (e *itEnv) setDue(classCode, key string, due time.Time) {
	e.t.Helper()
	e.exec(`UPDATE class_lesson_deadlines SET due_at = $3 WHERE class_id = $1 AND lesson_id = $2`,
		e.classID(classCode), e.lessonID(key), due)
}

func TestReminderTargetsOnlyUnsubmittedActiveMembers(t *testing.T) {
	e := newITAt(t, jobsNow)
	jobs := e.newJobs()
	loc := vn(t)
	// basic01 go-hw hạn sau 0,8 ngày: Cường đã nộp, Thảo đã rời lớp. Khóa thêm tài khoản Khang.
	e.exec(`UPDATE users SET status = 'disabled', disabled_at = $1 WHERE email = 'khang.vu@gmail.com'`, e.clk.Now())
	// basic02 ds-hw dời vào trong 24 giờ nhưng lớp đã kết thúc. basic02 go-hw không có hạn nộp.
	e.setDue("basic02", "ds-hw", e.clk.Now().Add(10*time.Hour))
	e.exec(`UPDATE classes SET status = 'ended' WHERE code = 'basic02'`)

	e.runJobs(jobs)
	e.clk.Advance(time.Hour)
	e.runJobs(jobs)
	var due time.Time
	require.NoError(t, e.dbx.Get(&due, `SELECT due_at FROM class_lesson_deadlines WHERE class_id = $1 AND lesson_id = $2`,
		e.classID("basic01"), e.lessonID("go-hw")))
	label := due.In(loc).Format("02/01/2006 15:04")
	title := "Bài tập: Đọc CSV và thống kê"
	want := []reminder{
		{anEmail, "basic01", title, label},
		{bichEmail, "basic01", title, label},
		{"dung.pham@gmail.com", "basic01", title, label},
		{haEmail, "basic01", title, label},
		{phongEmail, "basic01", title, label},
	}
	assert.Equal(t, want, reminders(e.outbox(mailer.TemplateDeadlineReminder)),
		"chạy hai lần chỉ nhắc một lần; không nhắc người đã nộp, đã rời lớp, bị khóa, lớp đã kết thúc")
	assert.True(t, e.runKeyExists("deadline_reminder:"+e.memberID("basic01", anEmail).String()+":"+
		e.lessonID("go-hw").String()+":"+due.UTC().Format(time.RFC3339)))

	newDue := due.Add(2 * time.Hour)
	e.setDue("basic01", "go-hw", newDue)
	e.runJobs(jobs)
	rows := reminders(e.outbox(mailer.TemplateDeadlineReminder))
	require.Len(t, rows, 2*len(want), "đổi hạn thì mỗi học viên được nhắc lại theo hạn mới")
	assert.Equal(t, reminder{anEmail, "basic01", title, newDue.In(loc).Format("02/01/2006 15:04")}, rows[1])
}

func TestReminderWindowBoundaries(t *testing.T) {
	e := newITAt(t, jobsNow)
	now := e.clk.Now()
	// basic01 go-hw hạn đúng now+24h được nhắc; basic02 ds-hw hạn đúng now đã qua, không nhắc dù còn học viên chưa nộp.
	e.setDue("basic01", "go-hw", now.Add(24*time.Hour))
	e.setDue("basic02", "ds-hw", now)
	e.runJobs(e.newJobs())
	classes := map[string]bool{}
	for _, r := range reminders(e.outbox(mailer.TemplateDeadlineReminder)) {
		classes[r.class] = true
	}
	assert.Equal(t, map[string]bool{"basic01": true}, classes)

	e.setDue("basic02", "ds-hw", now.Add(24*time.Hour+time.Second))
	e.runJobs(e.newJobs())
	for _, r := range reminders(e.outbox(mailer.TemplateDeadlineReminder)) {
		assert.Equal(t, "basic01", r.class, "hạn sau now+24h chưa được nhắc")
	}
}

func TestConcurrentRunsEnqueueEachKeyOnce(t *testing.T) {
	e := newITAt(t, jobsNow)
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		jobs := e.newJobs()
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = jobs.Cleanup(context.Background(), jobsNow)
		}()
	}
	close(start)
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, []string{baoEmail, huongEmail}, recipients(e.outbox(mailer.TemplateGradingDigest)),
		"khóa chính scheduled_job_runs: mỗi giảng viên đúng một dòng outbox")
	got := recipients(e.outbox(mailer.TemplateDeadlineReminder))
	assert.Len(t, got, 6, "mỗi học viên chưa nộp go-hw đúng một dòng outbox")
	seen := map[string]bool{}
	for _, to := range got {
		assert.False(t, seen[to], "trùng email nhắc cho %s", to)
		seen[to] = true
	}
}

func TestJobPayloadsRenderAfterOutboxRoundTrip(t *testing.T) {
	e := newITAt(t, jobsNow)
	e.runJobs(e.newJobs())
	r, err := mailer.NewRenderer(vn(t))
	require.NoError(t, err)

	digests := e.outbox(mailer.TemplateGradingDigest)
	require.NotEmpty(t, digests)
	for _, row := range digests {
		assert.NotContains(t, row.Payload, "Link")
		m, err := r.Render(mailer.TemplateGradingDigest, row.To, row.Payload)
		require.NoError(t, err)
		assert.Contains(t, m.Subject, "bài chờ chấm hoặc xác nhận quá 3 ngày: ")
		assert.Contains(t, m.Text, "http://localhost:5173/teach/grading?overdue=1")
		assert.Contains(t, m.HTML, `href="http://localhost:5173/teach/grading?overdue=1"`)
	}
	huong := digests[1]
	require.Equal(t, huongEmail, huong.To)
	m, err := r.Render(mailer.TemplateGradingDigest, huong.To, huong.Payload)
	require.NoError(t, err)
	assert.Contains(t, m.Subject, "Trần Thị Bích (bài tập)")
	assert.Contains(t, m.Subject, "Hoàng Thu Hà (quiz)")
	assert.Contains(t, m.Text, "đã chờ 4,5 ngày")

	reminders := e.outbox(mailer.TemplateDeadlineReminder)
	require.NotEmpty(t, reminders)
	lessonURL := "http://localhost:5173/learn/classes/" + e.classID("basic01").String() + "/lessons/" +
		e.lessonID("go-hw").String()
	for _, row := range reminders {
		assert.NotContains(t, row.Payload, "Link")
		m, err := r.Render(mailer.TemplateDeadlineReminder, row.To, row.Payload)
		require.NoError(t, err)
		assert.Contains(t, m.Subject, "[GoUp LMS] Nhắc hạn nộp: Bài tập: Đọc CSV và thống kê · hạn ")
		assert.Contains(t, m.Text, lessonURL)
		assert.Contains(t, m.HTML, `href="`+lessonURL+`"`)
	}
}
