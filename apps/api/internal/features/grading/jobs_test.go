package grading

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
)

// jobsRepo là Repo giả cho Jobs: ClaimRun giữ khóa như unique key của scheduled_job_runs.
type jobsRepo struct {
	Repo
	queue      []QueueItem
	teachers   []Teacher
	reminders  []Reminder
	queueErr   error
	claimed    map[string]bool
	gotWindows [][2]time.Time
}

func (r *jobsRepo) Queue(context.Context, db.Executor, *uuid.UUID) ([]QueueItem, error) {
	return append([]QueueItem(nil), r.queue...), r.queueErr
}

func (r *jobsRepo) Teachers(context.Context, db.Executor) ([]Teacher, error) { return r.teachers, nil }

func (r *jobsRepo) ReminderCandidates(_ context.Context, _ db.Executor, after, until time.Time) ([]Reminder, error) {
	r.gotWindows = append(r.gotWindows, [2]time.Time{after, until})
	return r.reminders, nil
}

func (r *jobsRepo) ClaimRun(_ context.Context, _ db.Executor, key string, _ time.Time) (bool, error) {
	if r.claimed[key] {
		return false, nil
	}
	r.claimed[key] = true
	return true, nil
}

// jobsTx chạy fn và, khi fn lỗi, khôi phục các khóa đã chèn như rollback.
type jobsTx struct{ repo *jobsRepo }

func (t jobsTx) Transact(_ context.Context, fn func(db.Executor) error) error {
	before := maps.Clone(t.repo.claimed)
	if err := fn(nil); err != nil {
		t.repo.claimed = before
		return err
	}
	return nil
}

// outbox là Enqueuer giả; failFor là email người nhận bị lỗi enqueue.
type outbox struct {
	sent    []mailer.Message
	failFor string
}

func (o *outbox) Enqueue(_ context.Context, _ db.Executor, m mailer.Message) (uuid.UUID, error) {
	if m.To.Display() == o.failFor {
		return uuid.Nil, errors.New("outbox lỗi")
	}
	o.sent = append(o.sent, m)
	return uuid.New(), nil
}

func (o *outbox) byTemplate(name mailer.TemplateName) []mailer.Message {
	var out []mailer.Message
	for _, m := range o.sent {
		if m.Template == name {
			out = append(out, m)
		}
	}
	return out
}

func vnLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)
	return loc
}

func newTestJobs(repo *jobsRepo, box *outbox, loc *time.Location) *Jobs {
	if repo.claimed == nil {
		repo.claimed = map[string]bool{}
	}
	return NewJobs(JobsDeps{
		Tx: jobsTx{repo: repo}, Repo: repo, Mail: box, Location: loc, PublicBaseURL: "http://localhost:5173",
	})
}

func overdueItem(teacher uuid.UUID, student string, kind Kind, submitted time.Time) QueueItem {
	return QueueItem{
		Key: QueueKey{Kind: kind, ItemID: uuid.New()}, SubmittedAt: submitted, TeacherID: teacher,
		Student: Ref{ID: uuid.New(), Name: student}, Class: Ref{ID: uuid.New(), Code: "BASIC01", Name: "Backend cơ bản K01"},
		Lesson: Ref{ID: uuid.New(), Name: "Bài tập: Thiết kế CSDL"},
	}
}

func TestDigestStartsAtEightLocalOncePerDay(t *testing.T) {
	loc := vnLoc(t)
	huong, bao := Teacher{ID: uuid.New(), Name: "Lê Thu Hương", Email: "huong.le@goup.vn"},
		Teacher{ID: uuid.New(), Name: "Phạm Quốc Bảo", Email: "bao.pham@goup.vn"}
	clk := &clock.Fake{T: time.Date(2026, 10, 19, 7, 59, 0, 0, loc)}
	repo := &jobsRepo{
		teachers: []Teacher{bao, huong},
		queue: []QueueItem{
			overdueItem(huong.ID, "Bích Trần", KindHomework, clk.Now().Add(-108*time.Hour)),
			overdueItem(huong.ID, "Hà Phạm", KindQuiz, clk.Now().Add(-84*time.Hour)),
			overdueItem(huong.ID, "Cường Lê", KindHomework, clk.Now().Add(-2*time.Hour)),
			overdueItem(bao.ID, "Phong Võ", KindHomework, clk.Now().Add(time.Minute-GradingSLA)),
		},
	}
	box := &outbox{}
	jobs := newTestJobs(repo, box, loc)

	require.NoError(t, jobs.Cleanup(context.Background(), clk.Now()))
	assert.Empty(t, box.byTemplate(mailer.TemplateGradingDigest), "07:59 giờ Việt Nam chưa gửi tổng hợp")

	clk.Advance(time.Minute)
	require.NoError(t, jobs.Cleanup(context.Background(), clk.Now()))
	digests := box.byTemplate(mailer.TemplateGradingDigest)
	require.Len(t, digests, 1, "08:00 gửi; Bảo chỉ có mục chờ đúng 72 giờ nên chưa quá hạn")
	m := digests[0]
	assert.Equal(t, "huong.le@goup.vn", m.To.Display())
	assert.True(t, repo.claimed["grading_digest:"+huong.ID.String()+":2026-10-19"])
	assert.Equal(t, "Lê Thu Hương", m.Payload["Name"])
	assert.Equal(t, 2, m.Payload["Count"])
	assert.Equal(t, "http://localhost:5173/teach/grading?overdue=1", m.Payload["QueueURL"])
	assert.NotContains(t, m.Payload, "Link")
	items, ok := m.Payload["Items"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, items, 2)
	assert.Equal(t, map[string]any{
		"StudentName": "Bích Trần", "KindLabel": "bài tập", "ClassCode": "BASIC01", "ClassName": "Backend cơ bản K01",
		"LessonTitle": "Bài tập: Thiết kế CSDL", "Waited": "4,5 ngày",
	}, items[0])
	assert.Equal(t, "quiz", items[1]["KindLabel"])
	assert.Equal(t, "3,5 ngày", items[1]["Waited"])

	require.NoError(t, jobs.Cleanup(context.Background(), clk.Now()))
	assert.Len(t, box.byTemplate(mailer.TemplateGradingDigest), 1, "chạy lại cùng giờ không gửi lại")

	clk.Advance(15 * time.Hour)
	require.NoError(t, jobs.Cleanup(context.Background(), clk.Now()))
	digests = box.byTemplate(mailer.TemplateGradingDigest)
	require.Len(t, digests, 2, "23:00 cùng ngày: Hương không nhận lại, Bảo nhận vì mục vừa quá hạn sau 08:00")
	assert.Equal(t, "bao.pham@goup.vn", digests[1].To.Display())
	assert.True(t, repo.claimed["grading_digest:"+bao.ID.String()+":2026-10-19"])

	clk.Advance(time.Hour)
	require.NoError(t, jobs.Cleanup(context.Background(), clk.Now()))
	assert.Len(t, box.byTemplate(mailer.TemplateGradingDigest), 2, "00:00 ngày mới vẫn trước 8 giờ")

	clk.Advance(8 * time.Hour)
	require.NoError(t, jobs.Cleanup(context.Background(), clk.Now()))
	digests = box.byTemplate(mailer.TemplateGradingDigest)
	require.Len(t, digests, 4, "08:00 ngày sau gửi lại cho cả Hương và Bảo vì mục vẫn quá hạn")
	assert.True(t, repo.claimed["grading_digest:"+huong.ID.String()+":2026-10-20"])
	assert.True(t, repo.claimed["grading_digest:"+bao.ID.String()+":2026-10-20"])
}

func TestDigestDayUsesLocalCalendar(t *testing.T) {
	loc := vnLoc(t)
	_, ok := digestDay(time.Date(2026, 10, 19, 0, 59, 0, 0, time.UTC), loc)
	assert.False(t, ok, "00:59 UTC là 07:59 giờ Việt Nam")
	day, ok := digestDay(time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC), loc)
	assert.True(t, ok)
	assert.Equal(t, "2026-10-19", day)
	day, ok = digestDay(time.Date(2026, 10, 19, 16, 59, 0, 0, time.UTC), loc)
	assert.True(t, ok)
	assert.Equal(t, "2026-10-19", day, "16:59 UTC là 23:59 cùng ngày ở Việt Nam")
	_, ok = digestDay(time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC), loc)
	assert.False(t, ok, "23:30 UTC là 06:30 sáng hôm sau ở Việt Nam")
}

func TestReminderWindowIncludesExactlyOneDayAheadButNotNow(t *testing.T) {
	loc := vnLoc(t)
	clk := &clock.Fake{T: time.Date(2026, 10, 19, 6, 0, 0, 0, loc)}
	repo := &jobsRepo{}
	jobs := newTestJobs(repo, &outbox{}, loc)
	require.NoError(t, jobs.Cleanup(context.Background(), clk.Now()))
	require.Len(t, repo.gotWindows, 1)
	after, until := repo.gotWindows[0][0], repo.gotWindows[0][1]
	assert.True(t, after.Equal(clk.Now()), "hạn đúng bằng now nằm ở biên mở, không được nhắc")
	assert.True(t, until.Equal(clk.Now().Add(24*time.Hour)), "hạn đúng now+24h nằm ở biên đóng, được nhắc")
}

func TestReminderPayloadAndKeyFollowDeadline(t *testing.T) {
	loc := vnLoc(t)
	now := time.Date(2026, 10, 19, 9, 0, 0, 0, loc)
	r := Reminder{
		MemberID: uuid.New(), StudentName: "An Nguyễn", StudentEmail: "an.nguyen@gmail.com",
		Class:  Ref{ID: uuid.New(), Code: "BASIC01", Name: "Backend cơ bản K01"},
		Lesson: Ref{ID: uuid.New(), Name: "Bài tập: Goroutine"},
		DueAt:  time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC),
	}
	repo := &jobsRepo{reminders: []Reminder{r}}
	box := &outbox{}
	jobs := newTestJobs(repo, box, loc)

	require.NoError(t, jobs.Cleanup(context.Background(), now))
	require.NoError(t, jobs.Cleanup(context.Background(), now.Add(time.Hour)))
	sent := box.byTemplate(mailer.TemplateDeadlineReminder)
	require.Len(t, sent, 1, "cùng hạn chỉ nhắc một lần")
	assert.True(t, repo.claimed["deadline_reminder:"+r.MemberID.String()+":"+r.Lesson.ID.String()+":2026-10-20T10:00:00Z"])
	assert.Equal(t, "an.nguyen@gmail.com", sent[0].To.Display())
	assert.Equal(t, map[string]any{
		"Name": "An Nguyễn", "ClassCode": "BASIC01", "ClassName": "Backend cơ bản K01",
		"LessonTitle": "Bài tập: Goroutine", "DueLabel": "20/10/2026 17:00",
		"LessonURL": "http://localhost:5173/learn/classes/" + r.Class.ID.String() + "/lessons/" + r.Lesson.ID.String(),
	}, sent[0].Payload)

	repo.reminders[0].DueAt = r.DueAt.Add(2 * time.Hour)
	require.NoError(t, jobs.Cleanup(context.Background(), now.Add(2*time.Hour)))
	sent = box.byTemplate(mailer.TemplateDeadlineReminder)
	require.Len(t, sent, 2, "đổi hạn thì nhắc lại theo hạn mới")
	assert.Equal(t, "20/10/2026 19:00", sent[1].Payload["DueLabel"])
}

func TestEmailFailureDoesNotBlockOthersAndRetriesLater(t *testing.T) {
	loc := vnLoc(t)
	now := time.Date(2026, 10, 19, 9, 0, 0, 0, loc)
	huong, bao := Teacher{ID: uuid.New(), Name: "Lê Thu Hương", Email: "huong.le@goup.vn"},
		Teacher{ID: uuid.New(), Name: "Phạm Quốc Bảo", Email: "bao.pham@goup.vn"}
	repo := &jobsRepo{
		teachers: []Teacher{huong, bao},
		queue: []QueueItem{
			overdueItem(huong.ID, "Bích Trần", KindHomework, now.Add(-100*time.Hour)),
			overdueItem(bao.ID, "Phong Võ", KindHomework, now.Add(-100*time.Hour)),
		},
	}
	box := &outbox{failFor: "huong.le@goup.vn"}
	jobs := newTestJobs(repo, box, loc)

	require.NoError(t, jobs.Cleanup(context.Background(), now), "lỗi của một email chỉ ghi log")
	digests := box.byTemplate(mailer.TemplateGradingDigest)
	require.Len(t, digests, 1)
	assert.Equal(t, "bao.pham@goup.vn", digests[0].To.Display())
	assert.False(t, repo.claimed["grading_digest:"+huong.ID.String()+":2026-10-19"], "enqueue lỗi thì rollback khóa")

	box.failFor = ""
	require.NoError(t, jobs.Cleanup(context.Background(), now.Add(time.Hour)))
	digests = box.byTemplate(mailer.TemplateGradingDigest)
	require.Len(t, digests, 2, "lần chạy sau gửi bù cho Hương, không gửi lại cho Bảo")
	assert.Equal(t, "huong.le@goup.vn", digests[1].To.Display())
}

func TestReadFailureOfDigestStillSendsReminders(t *testing.T) {
	loc := vnLoc(t)
	readErr := errors.New("db lỗi")
	repo := &jobsRepo{
		queueErr: readErr,
		reminders: []Reminder{{
			MemberID: uuid.New(), StudentName: "An Nguyễn", StudentEmail: "an.nguyen@gmail.com",
			DueAt: time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC),
		}},
	}
	box := &outbox{}
	jobs := newTestJobs(repo, box, loc)
	err := jobs.Cleanup(context.Background(), time.Date(2026, 10, 19, 9, 0, 0, 0, loc))
	require.ErrorIs(t, err, readErr)
	assert.Len(t, box.byTemplate(mailer.TemplateDeadlineReminder), 1)
}

func TestWaitedLabel(t *testing.T) {
	assert.Equal(t, "4,5 ngày", waitedLabel(108))
	assert.Equal(t, "3,5 ngày", waitedLabel(84))
	assert.Equal(t, "3,0 ngày", waitedLabel(73))
	assert.Equal(t, "20,0 ngày", waitedLabel(480))
}
