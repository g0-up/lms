package grading

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/db"
)

// DigestHour là giờ địa phương sớm nhất trong ngày gửi email tổng hợp chấm trễ.
const DigestHour = 8

// ReminderLead là khoảng trước hạn nộp mà học viên chưa nộp được nhắc.
const ReminderLead = 24 * time.Hour

// Reminder là một cặp (thành viên, homework) cần nhắc hạn nộp.
type Reminder struct {
	MemberID     uuid.UUID
	StudentName  string
	StudentEmail string
	Class        Ref
	Lesson       Ref // Name là tiêu đề học liệu
	DueAt        time.Time
}

// Jobs là việc hằng ngày của hàng chờ chấm, chạy trong Cleanups của worker (mỗi giờ): email tổng hợp chấm trễ cho
// giảng viên (FR-66) và email nhắc hạn nộp cho học viên (FR-68). Mỗi email chèn khóa vào scheduled_job_runs rồi mới
// enqueue, trong cùng một transaction, nên mỗi khóa chỉ được enqueue một lần dù worker khởi động lại hay chạy nhiều
// replica. Không ghi audit: outbox đủ để tra cứu email tự động.
type Jobs struct {
	tx      db.Tx
	repo    Repo
	mail    mailer.Enqueuer
	loc     *time.Location
	baseURL string
	log     *slog.Logger
}

var _ mailer.Cleanup = (*Jobs)(nil)

// JobsDeps là phụ thuộc của Jobs. Location là múi giờ nghiệp vụ (Asia/Ho_Chi_Minh) để xác định ngày, giờ gửi tổng hợp
// và giờ hiển thị hạn nộp; nil là UTC.
type JobsDeps struct {
	Tx            db.Tx
	Repo          Repo
	Mail          mailer.Enqueuer
	Location      *time.Location
	PublicBaseURL string
	Logger        *slog.Logger
}

// NewJobs nối phụ thuộc.
func NewJobs(d JobsDeps) *Jobs {
	loc, log := d.Location, d.Logger
	if loc == nil {
		loc = time.UTC
	}
	if log == nil {
		log = slog.Default()
	}
	return &Jobs{
		tx: d.Tx, repo: d.Repo, mail: d.Mail, loc: loc, baseURL: d.PublicBaseURL,
		log: log.With("component", "grading_jobs"),
	}
}

// Cleanup chạy hai việc hằng ngày; lỗi đọc của việc này không chặn việc kia. Lỗi của từng email chỉ được ghi log.
func (j *Jobs) Cleanup(ctx context.Context, now time.Time) error {
	return errors.Join(j.sendDigests(ctx, now), j.sendReminders(ctx, now))
}

// digestDay là ngày địa phương (yyyy-mm-dd) của now; ok=false khi chưa tới DigestHour giờ địa phương.
func digestDay(now time.Time, loc *time.Location) (day string, ok bool) {
	local := now.In(loc)
	if local.Hour() < DigestHour {
		return "", false
	}
	return local.Format(time.DateOnly), true
}

// reminderWindow là khoảng hạn nộp được nhắc tại now: (now, now + ReminderLead]. Hạn đúng now đã qua nên không nhắc.
func reminderWindow(now time.Time) (after, until time.Time) {
	return now, now.Add(ReminderLead)
}

// sendDigests gửi mỗi giảng viên không bị khóa có mục quá hạn một email mỗi ngày địa phương, liệt kê mọi mục quá hạn
// trong hàng chờ của các lớp giảng viên phụ trách (gồm lớp đã kết thúc, như hàng chờ).
func (j *Jobs) sendDigests(ctx context.Context, now time.Time) error {
	day, ok := digestDay(now, j.loc)
	if !ok {
		return nil
	}
	var (
		all      []QueueItem
		teachers []Teacher
	)
	err := j.tx.Transact(ctx, func(tx db.Executor) error {
		var err error
		if all, err = j.repo.Queue(ctx, tx, nil); err != nil {
			return err
		}
		teachers, err = j.repo.Teachers(ctx, tx)
		return err
	})
	if err != nil {
		return err
	}
	overdue := overdueByTeacher(stamp(all, now))
	for _, t := range teachers {
		items := overdue[t.ID]
		if len(items) == 0 {
			continue
		}
		key := fmt.Sprintf("grading_digest:%s:%s", t.ID, day)
		if err := j.once(ctx, key, now, t.Email, mailer.TemplateGradingDigest, j.digestPayload(t, items)); err != nil {
			j.log.ErrorContext(ctx, "grading: enqueue email tổng hợp chấm trễ lỗi", "teacher_id", t.ID, "err", err)
		}
	}
	return nil
}

// sendReminders nhắc mỗi học viên chưa nộp một lần cho mỗi hạn nộp; khóa chứa dueAt nên đổi hạn thì được nhắc lại
// theo hạn mới.
func (j *Jobs) sendReminders(ctx context.Context, now time.Time) error {
	after, until := reminderWindow(now)
	var list []Reminder
	err := j.tx.Transact(ctx, func(tx db.Executor) error {
		var err error
		list, err = j.repo.ReminderCandidates(ctx, tx, after, until)
		return err
	})
	if err != nil {
		return err
	}
	for _, r := range list {
		key := fmt.Sprintf("deadline_reminder:%s:%s:%s", r.MemberID, r.Lesson.ID, r.DueAt.UTC().Format(time.RFC3339))
		if err := j.once(ctx, key, now, r.StudentEmail, mailer.TemplateDeadlineReminder, j.reminderPayload(r)); err != nil {
			j.log.ErrorContext(ctx, "grading: enqueue email nhắc hạn nộp lỗi", "member_id", r.MemberID,
				"lesson_id", r.Lesson.ID, "err", err)
		}
	}
	return nil
}

// once chèn khóa key và, nếu chèn được, enqueue email trong cùng transaction: lỗi enqueue làm rollback khóa để lần
// chạy sau thử lại.
func (j *Jobs) once(ctx context.Context, key string, now time.Time, to string, tpl mailer.TemplateName,
	payload map[string]any) error {
	email, err := domain.ParseEmail(to)
	if err != nil {
		return err
	}
	return j.tx.Transact(ctx, func(tx db.Executor) error {
		claimed, err := j.repo.ClaimRun(ctx, tx, key, now)
		if err != nil || !claimed {
			return err
		}
		_, err = j.mail.Enqueue(ctx, tx, mailer.Message{To: email, Template: tpl, Payload: payload})
		return err
	})
}

// digestPayload là dữ liệu email tổng hợp: từng mục (học viên, loại, lớp, học liệu, đã chờ bao lâu) cũ nhất trước và
// link hàng chờ đã lọc quá hạn ở khóa QueueURL (mailer từ chối khóa Link).
func (j *Jobs) digestPayload(t Teacher, items []QueueItem) map[string]any {
	rows := make([]map[string]any, len(items))
	for i, it := range items {
		rows[i] = map[string]any{
			"StudentName": it.Student.Name, "KindLabel": kindLabel(it.Key.Kind), "ClassCode": it.Class.Code,
			"ClassName": it.Class.Name, "LessonTitle": it.Lesson.Name, "Waited": waitedLabel(it.WaitedHours),
		}
	}
	return map[string]any{
		"Name": t.Name, "Count": len(items), "Items": rows, "QueueURL": j.baseURL + "/teach/grading?overdue=1",
	}
}

// reminderPayload là dữ liệu email nhắc hạn nộp; hạn hiển thị theo giờ địa phương, link trang homework ở khóa
// LessonURL.
func (j *Jobs) reminderPayload(r Reminder) map[string]any {
	return map[string]any{
		"Name": r.StudentName, "ClassCode": r.Class.Code, "ClassName": r.Class.Name, "LessonTitle": r.Lesson.Name,
		"DueLabel":  r.DueAt.In(j.loc).Format("02/01/2006 15:04"),
		"LessonURL": fmt.Sprintf("%s/learn/classes/%s/lessons/%s", j.baseURL, r.Class.ID, r.Lesson.ID),
	}
}

// kindLabel là nhãn loại mục trong email tổng hợp, theo prototype.
func kindLabel(k Kind) string {
	if k == KindQuiz {
		return "quiz"
	}
	return "bài tập"
}

// waitedLabel hiển thị thời gian chờ theo ngày, một chữ số thập phân với dấu phẩy ("4,5 ngày"); mục trong email
// tổng hợp luôn chờ quá 3 ngày.
func waitedLabel(hours int) string {
	return strings.Replace(fmt.Sprintf("%.1f ngày", float64(hours)/24), ".", ",", 1)
}
