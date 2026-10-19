//go:build integration

package quizzes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/features/classes"
	"lms/api/internal/features/mailer"
)

const (
	adminEmail = "quan.tran@goup.vn"
	baoEmail   = "bao.pham@goup.vn"

	msgSuperseded = "Học viên đã nộp lượt mới hơn. Hãy xác nhận lượt mới nhất."
)

func reviewURL(id string) string          { return "/api/v1/quiz-attempts/" + id + "/reviews" }
func staffAttemptURL(id string) string    { return "/api/v1/quiz-attempts/" + id }
func classQuizzesURL(id uuid.UUID) string { return "/api/v1/classes/" + id.String() + "/quizzes" }

// reviewBody là thân POST nhận xét; notes là cặp (câu, ghi chú).
func reviewBody(comment string, notes ...any) string {
	list := make([]map[string]string, 0, len(notes)/2)
	for i := 0; i+1 < len(notes); i += 2 {
		list = append(list, map[string]string{"questionId": fmt.Sprint(notes[i]), "note": fmt.Sprint(notes[i+1])})
	}
	raw, _ := json.Marshal(map[string]any{"comment": comment, "notes": list})
	return string(raw)
}

func (e *itEnv) postReview(email, attemptID, body string) ReviewOutcomeDTO {
	e.t.Helper()
	return decode[ReviewOutcomeDTO](e.t, e.must(email, http.MethodPost, reviewURL(attemptID), body, http.StatusCreated))
}

// outboxMail là email nhận xét trong outbox, render như worker gửi (payload JSON → template).
type outboxMail struct {
	To      string
	Payload map[string]any
	mailer.RenderedMail
}

// reviewedMails đọc mọi email quiz_reviewed theo thứ tự đưa vào outbox và render từng email.
func (e *itEnv) reviewedMails() []outboxMail {
	e.t.Helper()
	var rows []struct {
		To      string `db:"to_email"`
		Payload string `db:"payload"`
	}
	if err := e.dbx.Select(&rows, `SELECT to_email, payload::text AS payload FROM email_outbox
		WHERE template = $1 ORDER BY created_at, id`, string(mailer.TemplateQuizReviewed)); err != nil {
		e.t.Fatal(err)
	}
	r, err := mailer.NewRenderer(nil)
	if err != nil {
		e.t.Fatal(err)
	}
	out := make([]outboxMail, len(rows))
	for i, row := range rows {
		var payload map[string]any
		if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
			e.t.Fatal(err)
		}
		rendered, err := r.Render(mailer.TemplateQuizReviewed, row.To, payload)
		if err != nil {
			e.t.Fatalf("render quiz_reviewed: %v", err)
		}
		out[i] = outboxMail{To: row.To, Payload: payload, RenderedMail: rendered}
	}
	return out
}

// assertMailHidesQuiz kiểm email không mang nội dung câu, phương án, giải thích hay ghi chú từng câu, và payload không
// có khóa bí mật hay khóa link chung.
func assertMailHidesQuiz(t *testing.T, m outboxMail, notes ...string) {
	t.Helper()
	banned := append([]string{"Giải thích", "Truy vấn khoảng", "Tìm toàn văn", "SELECT", "INSERT", "WHERE a = 1",
		"Chỉ mục B-tree", "Chỉ mục làm chậm", "Định danh duy nhất", "PRIMARY KEY", "Khóa chính dùng"}, notes...)
	for _, b := range banned {
		if strings.Contains(m.Subject+m.Text+m.HTML, b) {
			t.Errorf("email lộ %q:\n%s\n%s", b, m.Subject, m.Text)
		}
	}
	for _, k := range []string{"Link", "Token", "TempPassword"} {
		if _, ok := m.Payload[k]; ok {
			t.Errorf("payload có khóa %q: %v", k, m.Payload)
		}
	}
}

// auditRow là một bản ghi audit nhận xét quiz.
type auditRow struct {
	Action    string  `db:"action"`
	Before    *string `db:"before"`
	After     string  `db:"after"`
	ActorID   string  `db:"actor_id"`
	TargetID  string  `db:"target_id"`
	RequestID *string `db:"request_id"`
}

func (e *itEnv) quizAudits() []auditRow {
	e.t.Helper()
	var out []auditRow
	if err := e.dbx.Select(&out, `SELECT action, before::text AS before, after::text AS after, actor_id::text AS actor_id,
		target_id::text AS target_id, request_id FROM audit_logs WHERE action LIKE 'quiz_attempt.%' ORDER BY at, id`); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// Vòng đời xác nhận lượt đánh giá: lượt cũ bị thay không xác nhận được, xác nhận dẫn tới mục kế tiếp, sửa nhận xét giữ
// lịch sử, điểm và hoàn thành không đổi, email và audit đúng.
func TestAssessmentReviewLifecycle(t *testing.T) {
	e := newIT(t)

	first := e.start(anEmail, assessmentID)
	e.save(anEmail, first.ID, aq1, ao1a) // 3/10: trượt
	e.submit(anEmail, first.ID)
	e.clk.Advance(time.Hour)
	second := e.start(anEmail, assessmentID)
	e.save(anEmail, second.ID, aq1, ao1a)
	e.save(anEmail, second.ID, aq3, ao3a, ao3b) // 7/10: đạt
	e.submit(anEmail, second.ID)
	e.clk.Advance(time.Hour)
	khang := e.start(khangEmail, assessmentID)
	e.submit(khangEmail, khang.ID)
	e.clk.Advance(time.Hour)

	if got := queueIDs(e.quizQueue()); !slices.Equal(got, sorted(second.ID, khang.ID)) {
		t.Fatalf("hàng chờ trước khi xác nhận = %v", got)
	}

	e.expectError(huongEmail, http.MethodPost, reviewURL(first.ID), reviewBody("Lượt cũ."), http.StatusConflict,
		"INVALID_TRANSITION", msgSuperseded)
	if got := e.count(`SELECT count(*) FROM quiz_attempt_reviews`); got != 0 {
		t.Fatalf("lượt bị thay không được ghi nhận xét: %d", got)
	}

	rowBefore := e.attemptRow(second.ID)
	completedBefore, _ := e.completedAt(memberAnID, assessmentID)
	const (
		comment = "Đạt ngưỡng, cần xem lại chi phí ghi của chỉ mục."
		note    = "Xem lại chi phí ghi của chỉ mục."
	)
	out := e.postReview(huongEmail, second.ID, reviewBody(comment, aq2, note, aq1, "   "))
	if out.ReviewStatus != "confirmed" || out.Updated || out.Review.Comment != comment ||
		out.Review.Reviewer.ID != huongID.String() || out.Review.Reviewer.Name != "Lê Thu Hương" ||
		len(out.Review.Notes) != 1 || out.Review.Notes[0].QuestionID != aq2.String() || out.Review.Notes[0].Note != note {
		t.Fatalf("xác nhận = %+v", out)
	}
	if out.NextQueueItem == nil || out.NextQueueItem.ItemID != khang.ID || out.NextQueueItem.Kind != "quiz" {
		t.Fatalf("mục kế tiếp = %+v, muốn lượt của Khang", out.NextQueueItem)
	}
	if got := queueIDs(e.quizQueue()); !slices.Equal(got, []string{khang.ID}) {
		t.Fatalf("lượt đã xác nhận phải rời hàng chờ: %v", got)
	}
	if got := e.reviewStatuses(memberAnID, assessmentID); !slices.Equal(got, []string{"superseded", "confirmed"}) {
		t.Fatalf("review_status = %v", got)
	}
	rowAfter := e.attemptRow(second.ID)
	if *rowAfter.Score != *rowBefore.Score || *rowAfter.Passed != *rowBefore.Passed ||
		!rowAfter.SubmittedAt.Equal(*rowBefore.SubmittedAt) || rowAfter.AutoSubmitted != rowBefore.AutoSubmitted {
		t.Fatalf("xác nhận không được đổi điểm hay kết quả: %+v → %+v", rowBefore, rowAfter)
	}
	if completed, _ := e.completedAt(memberAnID, assessmentID); completed == nil || !completed.Equal(*completedBefore) {
		t.Fatalf("xác nhận không được đổi mốc hoàn thành: %v → %v", completedBefore, completed)
	}

	audits := e.quizAudits()
	if len(audits) != 1 || audits[0].Action != "quiz_attempt.confirmed" || audits[0].Before != nil ||
		audits[0].ActorID != huongID.String() || audits[0].TargetID != basic01ID.String() ||
		!strings.Contains(audits[0].After, comment) || !strings.Contains(audits[0].After, note) ||
		audits[0].RequestID == nil || *audits[0].RequestID == "" {
		t.Fatalf("audit xác nhận = %+v", audits)
	}
	mails := e.reviewedMails()
	if len(mails) != 1 || mails[0].To != anEmail ||
		mails[0].Subject != "[GoUp LMS] Giảng viên đã xác nhận kết quả: chỉ mục lượt #2 · 70% · Đạt" ||
		!strings.Contains(mails[0].Text, comment) {
		t.Fatalf("email xác nhận = %+v", mails)
	}
	if url, _ := mails[0].Payload["ResultURL"].(string); url != fmt.Sprintf("http://localhost:5173/learn/classes/%s/lessons/%s?result=%s",
		basic01ID, assessmentID, second.ID) {
		t.Fatalf("ResultURL = %q", url)
	}
	assertMailHidesQuiz(t, mails[0], note)

	// Sửa nhận xét: thêm một lần nhận xét (lịch sử giữ nguyên), không dẫn tiếp.
	e.clk.Advance(10 * time.Minute)
	const (
		edited     = "Đạt ngưỡng. Lượt sau chú ý câu về chỉ mục kết hợp."
		editedNote = "Tiền tố trái quyết định chỉ mục có dùng được không."
	)
	out = e.postReview(huongEmail, second.ID, reviewBody(edited, aq3, editedNote))
	if out.ReviewStatus != "confirmed" || !out.Updated || out.NextQueueItem != nil || out.Review.Comment != edited {
		t.Fatalf("sửa nhận xét = %+v", out)
	}
	if got := e.count(`SELECT count(*) FROM quiz_attempt_reviews WHERE attempt_id = $1`, second.ID); got != 2 {
		t.Fatalf("số lần nhận xét = %d, muốn 2", got)
	}
	if got := e.reviewStatuses(memberAnID, assessmentID); !slices.Equal(got, []string{"superseded", "confirmed"}) {
		t.Fatalf("review_status sau khi sửa = %v", got)
	}
	audits = e.quizAudits()
	if len(audits) != 2 || audits[1].Action != "quiz_attempt.review_updated" || audits[1].Before == nil ||
		!strings.Contains(*audits[1].Before, comment) || !strings.Contains(*audits[1].Before, note) ||
		!strings.Contains(audits[1].After, edited) || strings.Contains(audits[1].After, comment) {
		t.Fatalf("audit sửa nhận xét = %+v", audits)
	}
	mails = e.reviewedMails()
	if len(mails) != 2 || !strings.Contains(mails[1].Subject, "Nhận xét đã được cập nhật") ||
		!strings.Contains(mails[1].Text, edited) {
		t.Fatalf("email sửa nhận xét = %+v", mails)
	}
	assertMailHidesQuiz(t, mails[1], note, editedNote)

	// Trang xem lượt của staff: đáp án đầy đủ, nhận xét mới nhất trước, lịch sử lượt kèm nhận xét hiện hành.
	raw := e.must(huongEmail, http.MethodGet, staffAttemptURL(second.ID), "", http.StatusOK)
	d := decode[AttemptDetailDTO](t, raw)
	if d.ReviewStatus == nil || *d.ReviewStatus != "confirmed" || !d.IsLatest || d.Score != 7 || d.Percent != 70 ||
		d.CorrectCount != 2 || d.QuestionCount != 3 || len(d.Reviews) != 2 || d.Reviews[0].Comment != edited ||
		d.Reviews[1].Comment != comment || len(d.Attempts) != 2 || d.Attempts[0].ID != second.ID ||
		d.Attempts[0].Review == nil || d.Attempts[0].Review.Comment != edited || d.Attempts[1].Review != nil ||
		d.Questions[0].ID != aq3.String() || d.Questions[1].Status != "blank" || len(d.Questions[0].Options) != 3 {
		t.Fatalf("trang xem lượt = %s", raw)
	}
	assertGolden(t, "review-assessment.json", raw)
	old := decode[AttemptDetailDTO](t, e.must(huongEmail, http.MethodGet, staffAttemptURL(first.ID), "", http.StatusOK))
	if old.IsLatest || old.LatestAttemptID != second.ID || old.ReviewStatus == nil || *old.ReviewStatus != "superseded" ||
		len(old.Reviews) != 0 {
		t.Fatalf("trang xem lượt cũ = %+v", old)
	}

	// Học viên thấy nhận xét hiện hành kèm ghi chú, đánh số theo thứ tự học viên thấy (trùng số câu trên trang của
	// giảng viên, nơi aq3 đứng đầu), vẫn không thấy đáp án.
	raw = e.must(anEmail, http.MethodGet, attemptURL(second.ID), "", http.StatusOK)
	assertOnlyKeys(t, raw, assessmentKeys)
	assertHidesAnswers(t, raw, false)
	res := decode[AssessmentResultDTO](t, raw)
	if res.Review == nil || !res.Review.Updated || res.Review.Comment != edited || len(res.Review.Notes) != 1 ||
		res.Review.Notes[0].QuestionID != aq3.String() || res.Review.Notes[0].Position != 1 ||
		res.ReviewStatus == nil || *res.ReviewStatus != "confirmed" {
		t.Fatalf("kết quả học viên sau nhận xét = %s", raw)
	}
	assertGolden(t, "attempt-result-assessment-reviewed.json", raw)
}

// Lượt luyện tập: nhận xét được nhưng không bao giờ vào hàng chờ và không dẫn tiếp.
func TestPracticeReviewIsNeverQueued(t *testing.T) {
	e := newIT(t)
	a := e.start(anEmail, practiceID)
	e.save(anEmail, a.ID, pq1, po1a)
	e.save(anEmail, a.ID, pq2, po2a)
	e.submit(anEmail, a.ID)
	e.clk.Advance(time.Hour)
	if got := e.reviewStatuses(memberAnID, practiceID); !slices.Equal(got, []string{"-"}) {
		t.Fatalf("review_status luyện tập = %v", got)
	}
	if q := e.quizQueue(); len(q) != 0 {
		t.Fatalf("luyện tập không vào hàng chờ: %+v", q)
	}
	rowBefore := e.attemptRow(a.ID)

	const comment = "Câu ràng buộc còn thiếu một đáp án."
	out := e.postReview(huongEmail, a.ID, reviewBody(comment, pq2, "UNIQUE cũng chặn trùng."))
	if out.Updated || out.NextQueueItem != nil || out.Review.Comment != comment || len(out.Review.Notes) != 1 {
		t.Fatalf("nhận xét luyện tập = %+v", out)
	}
	if q := e.quizQueue(); len(q) != 0 {
		t.Fatalf("lượt luyện tập đã nhận xét không vào hàng chờ: %+v", q)
	}
	if rowAfter := e.attemptRow(a.ID); *rowAfter.Score != *rowBefore.Score || rowAfter.Passed != nil {
		t.Fatalf("nhận xét không đổi điểm: %+v → %+v", rowBefore, rowAfter)
	}
	if audits := e.quizAudits(); len(audits) != 1 || audits[0].Action != "quiz_attempt.commented" || audits[0].Before != nil {
		t.Fatalf("audit nhận xét luyện tập = %+v", audits)
	}
	mails := e.reviewedMails()
	if len(mails) != 1 || mails[0].Subject != "[GoUp LMS] Giảng viên đã nhận xét: khóa chính lượt #1" {
		t.Fatalf("email nhận xét luyện tập = %+v", mails)
	}
	assertMailHidesQuiz(t, mails[0], "UNIQUE cũng chặn trùng.")

	raw := e.must(huongEmail, http.MethodGet, staffAttemptURL(a.ID), "", http.StatusOK)
	assertGolden(t, "review-practice.json", raw)

	raw = e.must(anEmail, http.MethodGet, attemptURL(a.ID), "", http.StatusOK)
	assertOnlyKeys(t, raw, practiceKeys)
	res := decode[PracticeResultDTO](t, raw)
	if res.Review == nil || res.Review.Comment != comment || res.Review.Updated || res.Questions[1].Note == nil ||
		*res.Questions[1].Note != "UNIQUE cũng chặn trùng." {
		t.Fatalf("kết quả luyện tập sau nhận xét = %s", raw)
	}

	out = e.postReview(huongEmail, a.ID, reviewBody("Đã sửa nhận xét."))
	if !out.Updated || out.NextQueueItem != nil {
		t.Fatalf("sửa nhận xét luyện tập = %+v", out)
	}
	if audits := e.quizAudits(); len(audits) != 2 || audits[1].Action != "quiz_attempt.review_updated" || audits[1].Before == nil {
		t.Fatalf("audit sửa nhận xét luyện tập = %+v", audits)
	}
}

// Chỉ admin hoặc giảng viên phụ trách lớp xem và nhận xét được; học viên và giảng viên lớp khác bị chặn, lượt đang
// làm không xem được.
func TestReviewAccess(t *testing.T) {
	e := newIT(t)
	a := e.start(anEmail, assessmentID)
	e.submit(anEmail, a.ID)
	open := e.start(khangEmail, assessmentID)
	e.clk.Advance(time.Hour)

	notOwnClass := classes.ErrNotOwnClass.Error()
	e.expectError(baoEmail, http.MethodGet, staffAttemptURL(a.ID), "", http.StatusForbidden, "FORBIDDEN", notOwnClass)
	e.expectError(baoEmail, http.MethodPost, reviewURL(a.ID), reviewBody("Không phải lớp mình."), http.StatusForbidden,
		"FORBIDDEN", "Bạn chỉ nhận xét bài của lớp mình phụ trách.")
	e.expectError(baoEmail, http.MethodGet, classQuizzesURL(basic01ID), "", http.StatusForbidden, "FORBIDDEN", notOwnClass)
	e.expectError(anEmail, http.MethodGet, staffAttemptURL(a.ID), "", http.StatusForbidden, "FORBIDDEN", "")
	e.expectError(anEmail, http.MethodPost, reviewURL(a.ID), reviewBody("Tự nhận xét."), http.StatusForbidden, "FORBIDDEN", "")
	e.expectError(anEmail, http.MethodGet, classQuizzesURL(basic01ID), "", http.StatusForbidden, "FORBIDDEN", "")
	if got := e.serve("", http.MethodGet, staffAttemptURL(a.ID), ""); got.code != http.StatusUnauthorized {
		t.Fatalf("chưa đăng nhập = %d", got.code)
	}

	const notSubmitted = "Không tìm thấy lượt làm bài đã nộp."
	e.expectError(huongEmail, http.MethodGet, staffAttemptURL(open.ID), "", http.StatusNotFound, "NOT_FOUND", notSubmitted)
	e.expectError(huongEmail, http.MethodPost, reviewURL(open.ID), reviewBody("Chưa nộp."), http.StatusNotFound, "NOT_FOUND",
		notSubmitted)
	e.expectError(huongEmail, http.MethodGet, staffAttemptURL(uuid.Must(uuid.NewV7()).String()), "", http.StatusNotFound,
		"NOT_FOUND", notSubmitted)
	e.expectError(huongEmail, http.MethodGet, staffAttemptURL("khong-phai-uuid"), "", http.StatusNotFound, "NOT_FOUND", "")
	e.expectError(huongEmail, http.MethodGet, classQuizzesURL(uuid.Must(uuid.NewV7())), "", http.StatusNotFound, "NOT_FOUND", "")
	if got := e.count(`SELECT count(*) FROM quiz_attempt_reviews`) + e.count(`SELECT count(*) FROM email_outbox`) +
		len(e.quizAudits()); got != 0 {
		t.Fatalf("yêu cầu bị chặn không được ghi gì: %d", got)
	}

	// Admin xem và xác nhận được mọi lớp.
	e.must(adminEmail, http.MethodGet, staffAttemptURL(a.ID), "", http.StatusOK)
	e.must(adminEmail, http.MethodGet, classQuizzesURL(basic01ID), "", http.StatusOK)
	out := e.postReview(adminEmail, a.ID, reviewBody("Admin xác nhận."))
	if out.ReviewStatus != "confirmed" || out.Review.Reviewer.ID != adminID.String() || out.NextQueueItem != nil {
		t.Fatalf("admin xác nhận = %+v", out)
	}
	// Giảng viên phụ trách sửa được nhận xét của admin.
	if out = e.postReview(huongEmail, a.ID, reviewBody("Hương bổ sung.")); !out.Updated {
		t.Fatalf("giảng viên sửa nhận xét của admin = %+v", out)
	}
}

func TestReviewValidation(t *testing.T) {
	e := newIT(t)
	a := e.start(anEmail, assessmentID)
	e.submit(anEmail, a.ID)

	cases := []struct {
		name, body string
		status     int
		msg        string
	}{
		{"nhận xét rỗng", reviewBody("  "), http.StatusUnprocessableEntity, "Nhập nhận xét chung cho học viên."},
		{"nhận xét quá dài", reviewBody(strings.Repeat("ạ", 5001)), http.StatusUnprocessableEntity, "Nhận xét tối đa 5000 ký tự."},
		{"ghi chú quá dài", reviewBody("x", aq1, strings.Repeat("ạ", 1001)), http.StatusUnprocessableEntity,
			"Ghi chú từng câu tối đa 1000 ký tự."},
		{"ghi chú câu của quiz khác", reviewBody("x", pq1, "y"), http.StatusUnprocessableEntity,
			"Ghi chú cho câu không thuộc lượt làm bài này."},
		{"trùng câu", reviewBody("x", aq1, "a", aq1, "b"), http.StatusUnprocessableEntity, "Mỗi câu chỉ có một ghi chú."},
		{"JSON hỏng", `{"comment":`, http.StatusBadRequest, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e.expectError(huongEmail, http.MethodPost, reviewURL(a.ID), tc.body, tc.status, "VALIDATION_FAILED", tc.msg)
		})
	}
	if got := e.count(`SELECT count(*) FROM quiz_attempt_reviews`) + e.count(`SELECT count(*) FROM email_outbox`) +
		len(e.quizAudits()); got != 0 {
		t.Fatalf("yêu cầu lỗi không được ghi gì: %d", got)
	}
	if got := e.reviewStatuses(memberAnID, assessmentID); !slices.Equal(got, []string{"pending"}) {
		t.Fatalf("review_status = %v", got)
	}
}

// Xác nhận lượt 1 đúng lúc học viên nộp lượt 2: cả hai cùng khóa thành viên trước rồi mới khóa lượt nên không deadlock;
// bên nào lấy khóa trước quyết định kết quả và không bao giờ xác nhận lượt đã bị thay. Thứ tự lấy khóa được ép bằng
// hàng đợi khóa FIFO của Postgres.
func TestReviewRacesSubmit(t *testing.T) {
	for _, confirmFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("xác nhận trước=%v", confirmFirst), func(t *testing.T) {
			e := newIT(t)
			first := e.start(anEmail, assessmentID)
			e.save(anEmail, first.ID, aq1, ao1a)
			e.submit(anEmail, first.ID)
			e.clk.Advance(time.Hour)
			second := e.start(anEmail, assessmentID)
			student, teacher := e.as(anEmail), e.as(huongEmail)

			release := e.holdMemberLock(memberAnID)
			var (
				wg             sync.WaitGroup
				review, submit apiResp
			)
			order := []func(){
				func() {
					review = e.serve(teacher, http.MethodPost, reviewURL(first.ID), reviewBody("Xác nhận lượt 1."))
				},
				func() { submit = e.serve(student, http.MethodPost, submitURL(second.ID), "") },
			}
			if !confirmFirst {
				slices.Reverse(order)
			}
			wg.Go(order[0])
			e.waitForLockWaiters(1)
			wg.Go(order[1])
			e.waitForLockWaiters(2)
			release()
			wg.Wait()

			if submit.code != http.StatusOK {
				t.Fatalf("nộp = %d: %s", submit.code, submit.body)
			}
			if confirmFirst {
				if review.code != http.StatusCreated {
					t.Fatalf("xác nhận trước = %d: %s", review.code, review.body)
				}
				// Lượt đã xác nhận giữ nguyên khi có lượt mới; chỉ lượt pending mới bị thay.
				if got := e.reviewStatuses(memberAnID, assessmentID); !slices.Equal(got, []string{"confirmed", "pending"}) {
					t.Fatalf("review_status = %v", got)
				}
				if got := e.count(`SELECT count(*) FROM quiz_attempt_reviews WHERE attempt_id = $1`, first.ID); got != 1 {
					t.Fatalf("số lần nhận xét = %d", got)
				}
			} else {
				if env := errorOf(t, review); review.code != http.StatusConflict || env.Error.Message != msgSuperseded {
					t.Fatalf("xác nhận sau khi nộp lượt mới = %d: %s", review.code, review.body)
				}
				if got := e.reviewStatuses(memberAnID, assessmentID); !slices.Equal(got, []string{"superseded", "pending"}) {
					t.Fatalf("review_status = %v", got)
				}
				if got := e.count(`SELECT count(*) FROM quiz_attempt_reviews`) + len(e.reviewedMails()); got != 0 {
					t.Fatalf("lượt bị thay không được nhận xét: %d", got)
				}
			}
			if got := queueIDs(e.quizQueue()); !slices.Equal(got, []string{second.ID}) {
				t.Fatalf("hàng chờ = %v", got)
			}
		})
	}
}
