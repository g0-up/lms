//go:build integration

package quizzes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	msgNotOwnGrant     = "Bạn chỉ cấp thêm lượt cho lớp mình phụ trách."
	msgGrantNotActive  = "Lớp không ở trạng thái đang chạy"
	msgGrantNotAssess  = "Chỉ cấp thêm lượt cho quiz bài tập."
	msgGrantRange      = "Số lượt cấp thêm từ 1 đến 3."
	msgGrantReason     = "Nhập lý do cấp thêm lượt. Lý do được lưu vào lịch sử."
	msgNoAttemptsLeft2 = "Bạn đã dùng hết 2 lượt làm bài."
)

func grantURL(classID, memberID uuid.UUID) string {
	return "/api/v1/classes/" + classID.String() + "/members/" + memberID.String() + "/grants"
}

// grantBody là thân POST cấp thêm lượt; extra là any để gửi được cả giá trị thiếu (nil) lẫn sai kiểu.
func grantBody(lessonID any, extra any, reason string) string {
	body := map[string]any{"lessonId": fmt.Sprint(lessonID), "reason": reason}
	if extra != nil {
		body["extraAttempts"] = extra
	}
	raw, _ := json.Marshal(body)
	return string(raw)
}

func (e *itEnv) grant(email string, classID, memberID uuid.UUID, body string) GrantDTO {
	e.t.Helper()
	return decode[GrantDTO](e.t, e.must(email, http.MethodPost, grantURL(classID, memberID), body, http.StatusCreated))
}

func (e *itEnv) fullName(userID uuid.UUID) string {
	e.t.Helper()
	var name string
	if err := e.dbx.Get(&name, `SELECT full_name FROM users WHERE id = $1`, userID); err != nil {
		e.t.Fatal(err)
	}
	return name
}

// useAllAssessmentAttempts nộp cả hai lượt của quiz đánh giá cho An (bỏ trống) để An hết lượt.
func (e *itEnv) useAllAssessmentAttempts() {
	e.t.Helper()
	for range 2 {
		a := e.start(anEmail, assessmentID)
		e.submit(anEmail, a.ID)
		e.clk.Advance(time.Minute)
	}
	e.expectError(anEmail, http.MethodPost, attemptsURL(basic01ID, assessmentID), "", http.StatusConflict,
		"INVALID_TRANSITION", msgNoAttemptsLeft2)
}

// quizItem là quiz lessonID trong tổng quan quiz của basic01.
func (e *itEnv) quizItem(lessonID uuid.UUID) QuizItemDTO {
	e.t.Helper()
	v := decode[ClassQuizzesDTO](e.t, e.must(huongEmail, http.MethodGet, classQuizzesURL(basic01ID), "", http.StatusOK))
	for _, it := range v.Items {
		if it.Lesson.ID == lessonID.String() {
			return it
		}
	}
	e.t.Fatalf("tổng quan quiz không có %s", lessonID)
	return QuizItemDTO{}
}

func memberOf(t *testing.T, it QuizItemDTO, memberID uuid.UUID) MemberQuizDTO {
	t.Helper()
	for _, m := range it.Members {
		if m.MemberID == memberID.String() {
			return m
		}
	}
	t.Fatalf("quiz %s không có thành viên %s", it.Lesson.Title, memberID)
	return MemberQuizDTO{}
}

// Học viên hết lượt được cấp thêm thì bắt đầu được lượt mới; giới hạn tăng đúng số lượt cấp, lịch sử cấp lượt mới
// nhất trước và audit có đủ trường.
func TestGrantLetsStudentStartAgain(t *testing.T) {
	e := newIT(t)
	e.useAllAssessmentAttempts()

	g := e.grant(huongEmail, basic01ID, memberAnID, grantBody(assessmentID, 1, "  Mất kết nối khi đang làm bài  "))
	assert.Equal(t, memberAnID.String(), g.MemberID)
	assert.Equal(t, assessmentID.String(), g.LessonID)
	assert.Equal(t, e.fullName(anID), g.StudentName)
	assert.Equal(t, 1, g.ExtraAttempts)
	assert.Equal(t, "Mất kết nối khi đang làm bài", g.Reason, "lý do được trim")
	assert.Equal(t, e.fullName(huongID), g.GrantedByName)
	assert.True(t, g.GrantedAt.Equal(e.clk.Now()), "grantedAt = %v", g.GrantedAt)

	o := e.overview(anEmail, basic01ID, assessmentID)
	require.NotNil(t, o.Limit)
	assert.True(t, o.CanStart)
	assert.Equal(t, 3, *o.Limit)
	assert.Equal(t, 1, o.Granted, "màn bắt đầu biết học viên đã được cấp thêm")

	third := e.start(anEmail, assessmentID)
	assert.Equal(t, 3, third.No)
	require.NotNil(t, third.Limit)
	assert.Equal(t, 3, *third.Limit)
	e.submit(anEmail, third.ID)
	e.expectError(anEmail, http.MethodPost, attemptsURL(basic01ID, assessmentID), "", http.StatusConflict,
		"INVALID_TRANSITION", "Bạn đã dùng hết 3 lượt làm bài.")

	// Admin cấp cho lớp bất kỳ; cấp nhiều lần thì cộng dồn.
	e.clk.Advance(time.Minute)
	g2 := e.grant(adminEmail, basic01ID, memberAnID, grantBody(assessmentID, 2, "Câu 2 sai đáp án"))
	assert.Equal(t, e.fullName(adminID), g2.GrantedByName)

	it := e.quizItem(assessmentID)
	an := memberOf(t, it, memberAnID)
	assert.Equal(t, 3, an.Granted)
	require.NotNil(t, an.Limit)
	assert.Equal(t, 5, *an.Limit)
	assert.Equal(t, 3, an.Used)
	assert.Equal(t, []GrantDTO{g2, g}, it.Grants, "lịch sử cấp lượt, mới nhất trước")
	assert.Empty(t, e.quizItem(practiceID).Grants)

	var logs []struct {
		ActorID    uuid.UUID `db:"actor_id"`
		TargetType string    `db:"target_type"`
		TargetID   uuid.UUID `db:"target_id"`
		Before     *string   `db:"before"`
		After      string    `db:"after"`
		RequestID  *string   `db:"request_id"`
	}
	require.NoError(t, e.dbx.Select(&logs, `SELECT actor_id, target_type, target_id, before::text AS before, after::text AS after,
		request_id FROM audit_logs WHERE action = 'quiz.attempts_granted' ORDER BY at, id`))
	require.Len(t, logs, 2)
	first := logs[0]
	assert.Equal(t, huongID, first.ActorID)
	assert.Equal(t, "class", first.TargetType)
	assert.Equal(t, basic01ID, first.TargetID)
	assert.Nil(t, first.Before)
	require.NotNil(t, first.RequestID)
	assert.NotEmpty(t, *first.RequestID)
	var after map[string]any
	require.NoError(t, json.Unmarshal([]byte(first.After), &after))
	assert.Equal(t, map[string]any{
		"classCode": "basic01", "memberId": memberAnID.String(), "studentName": e.fullName(anID),
		"lessonId": assessmentID.String(), "lessonTitle": "Kiểm tra: chỉ mục", "extraAttempts": float64(1),
		"reason": "Mất kết nối khi đang làm bài",
	}, after)
	assert.Equal(t, adminID, logs[1].ActorID)
}

// Mỗi trường hợp từ chối có đúng copy và không ghi grant hay audit nào.
func TestGrantRejections(t *testing.T) {
	e := newIT(t)
	valid := func(lessonID uuid.UUID) string { return grantBody(lessonID, 1, "Mất kết nối") }
	cases := []struct {
		name          string
		email         string
		class, member uuid.UUID
		body          string
		status        int
		code, msg     string
	}{
		{"giảng viên lớp khác", baoEmail, basic01ID, memberAnID, valid(assessmentID), http.StatusForbidden, "FORBIDDEN", msgNotOwnGrant},
		{"học viên", anEmail, basic01ID, memberAnID, valid(assessmentID), http.StatusForbidden, "FORBIDDEN", ""},
		{"lớp không có", huongEmail, uuid.Must(uuid.NewV7()), memberAnID, valid(assessmentID), http.StatusNotFound, "NOT_FOUND", "Không tìm thấy lớp."},
		{"lớp đã kết thúc", huongEmail, basic00ID, memberAn00ID, valid(assessmentID), http.StatusConflict, "INVALID_TRANSITION", msgGrantNotActive},
		{"lớp nháp", huongEmail, basic03ID, memberAn03ID, valid(assessmentID), http.StatusConflict, "INVALID_TRANSITION", msgGrantNotActive},
		{"thành viên lớp khác qua id lớp mình", huongEmail, basic01ID, memberLinhID, valid(assessmentID), http.StatusNotFound, "NOT_FOUND", "Không tìm thấy thành viên."},
		{"thành viên không có", huongEmail, basic01ID, uuid.Must(uuid.NewV7()), valid(assessmentID), http.StatusNotFound, "NOT_FOUND", "Không tìm thấy thành viên."},
		{"học viên đã rời lớp", huongEmail, basic01ID, memberThaoID, valid(assessmentID), http.StatusConflict, "INVALID_TRANSITION", "Học viên đã rời lớp."},
		{"quiz luyện tập", huongEmail, basic01ID, memberAnID, valid(practiceID), http.StatusUnprocessableEntity, "VALIDATION_FAILED", msgGrantNotAssess},
		{"quiz của khóa khác", baoEmail, go01ID, memberLinhID, valid(assessmentID), http.StatusUnprocessableEntity, "VALIDATION_FAILED", msgGrantNotAssess},
		{"học liệu không có", huongEmail, basic01ID, memberAnID, valid(uuid.Must(uuid.NewV7())), http.StatusUnprocessableEntity, "VALIDATION_FAILED", msgGrantNotAssess},
		{"lessonId sai định dạng", huongEmail, basic01ID, memberAnID, grantBody("abc", 1, "Mất kết nối"), http.StatusUnprocessableEntity, "VALIDATION_FAILED", msgGrantNotAssess},
		{"0 lượt", huongEmail, basic01ID, memberAnID, grantBody(assessmentID, 0, "Mất kết nối"), http.StatusUnprocessableEntity, "VALIDATION_FAILED", msgGrantRange},
		{"4 lượt", huongEmail, basic01ID, memberAnID, grantBody(assessmentID, 4, "Mất kết nối"), http.StatusUnprocessableEntity, "VALIDATION_FAILED", msgGrantRange},
		{"thiếu số lượt", huongEmail, basic01ID, memberAnID, grantBody(assessmentID, nil, "Mất kết nối"), http.StatusUnprocessableEntity, "VALIDATION_FAILED", msgGrantRange},
		{"lý do rỗng", huongEmail, basic01ID, memberAnID, grantBody(assessmentID, 1, ""), http.StatusUnprocessableEntity, "VALIDATION_FAILED", msgGrantReason},
		{"lý do chỉ có khoảng trắng", huongEmail, basic01ID, memberAnID, grantBody(assessmentID, 1, "   "), http.StatusUnprocessableEntity, "VALIDATION_FAILED", msgGrantReason},
		{"lý do quá 500 ký tự", huongEmail, basic01ID, memberAnID, grantBody(assessmentID, 1, strings.Repeat("a", 501)), http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Lý do tối đa 500 ký tự."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e.t = t
			e.expectError(tc.email, http.MethodPost, grantURL(tc.class, tc.member), tc.body, tc.status, tc.code, tc.msg)
		})
	}
	assert.Zero(t, e.count(`SELECT count(*) FROM quiz_attempt_grants`), "không có grant nào được ghi")
	assert.Zero(t, e.count(`SELECT count(*) FROM audit_logs WHERE action = 'quiz.attempts_granted'`))
}

// Cấp lượt và hai lần bắt đầu lượt của cùng học viên xếp hàng sau khóa class_members (hàng đợi khóa của Postgres
// theo thứ tự đến): chạy tuần tự, không deadlock và số lượt không vượt giới hạn ở cả hai thứ tự.
func TestGrantRacesStart(t *testing.T) {
	cases := []struct {
		name       string
		grantFirst bool
		attempts   int
	}{
		{"cấp lượt trước rồi hai lần bắt đầu", true, 3},
		{"hai lần bắt đầu trước rồi cấp lượt", false, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newIT(t)
			e.useAllAssessmentAttempts()
			staff, student := e.as(huongEmail), e.as(anEmail)

			release := e.holdMemberLock(memberAnID)
			var (
				wg     sync.WaitGroup
				grant  apiResp
				starts [2]apiResp
			)
			queueGrant := func() {
				wg.Go(func() {
					grant = e.serve(staff, http.MethodPost, grantURL(basic01ID, memberAnID), grantBody(assessmentID, 1, "Mất kết nối"))
				})
			}
			waiting := 0 // xếp từng yêu cầu vào hàng đợi khóa theo đúng thứ tự của case
			if tc.grantFirst {
				queueGrant()
				waiting++
				e.waitForLockWaiters(waiting)
			}
			for i := range starts {
				wg.Go(func() { starts[i] = e.serve(student, http.MethodPost, attemptsURL(basic01ID, assessmentID), "") })
				waiting++
				e.waitForLockWaiters(waiting)
			}
			if !tc.grantFirst {
				queueGrant()
			}
			e.waitForLockWaiters(3)
			release()
			wg.Wait()

			require.Equal(t, http.StatusCreated, grant.code, "%s", grant.body)
			if tc.grantFirst {
				require.Equal(t, http.StatusCreated, starts[0].code, "%s", starts[0].body)
				require.Equal(t, http.StatusOK, starts[1].code, "lần bắt đầu sau tiếp tục lượt vừa mở: %s", starts[1].body)
				assert.Equal(t, decode[StartDTO](t, starts[0].body).Attempt.ID, decode[StartDTO](t, starts[1].body).Attempt.ID)
			} else {
				for _, st := range starts {
					require.Equal(t, http.StatusConflict, st.code, "%s", st.body)
					assert.Equal(t, msgNoAttemptsLeft2, errorOf(t, st).Error.Message)
				}
			}
			assert.Equal(t, tc.attempts, e.count(`SELECT count(*) FROM quiz_attempts WHERE class_member_id = $1 AND lesson_id = $2`,
				memberAnID, assessmentID), "không vượt giới hạn 2 + 1")
			o := e.overview(anEmail, basic01ID, assessmentID)
			assert.Equal(t, tc.attempts, o.Used)
			require.NotNil(t, o.Limit)
			assert.Equal(t, 3, *o.Limit)
		})
	}
}

// Thống kê câu hỏi: mẫu số là mọi lượt đã nộp (kể cả lượt bị thay thế) của thành viên đang học; câu chỉ toàn bỏ
// trống có 0%; quiz chưa có lượt có tỉ lệ null.
func TestQuestionStatsOfClass(t *testing.T) {
	e := newIT(t)

	before := e.quizItem(assessmentID).QuestionStats
	assert.Zero(t, before.Attempts)
	require.Len(t, before.Rows, 3)
	for i, r := range before.Rows {
		assert.Equal(t, i+1, r.Position)
		assert.Nil(t, r.CorrectPercent, "quiz chưa có lượt thì tỉ lệ null")
	}
	assert.Equal(t, "Chỉ mục B-tree phù hợp với?", before.Rows[0].BodyText, "văn bản thuần, không còn thẻ <p>")

	// An: lượt 1 đúng câu 1, lượt 2 bỏ trống (lượt 1 bị thay thế vẫn tính). Khang: đúng câu 1 và 2. Câu 3 không ai
	// trả lời: 0%.
	a1 := e.start(anEmail, assessmentID)
	e.save(anEmail, a1.ID, aq1, ao1a)
	e.submit(anEmail, a1.ID)
	a2 := e.start(anEmail, assessmentID)
	e.submit(anEmail, a2.ID)
	k := e.start(khangEmail, assessmentID)
	e.save(khangEmail, k.ID, aq1, ao1a)
	e.save(khangEmail, k.ID, aq2, ao2b)
	e.submit(khangEmail, k.ID)
	open := e.start(khangEmail, practiceID) // lượt đang mở không tính
	e.save(khangEmail, open.ID, pq1, po1a)

	assertGolden(t, "question-stats.json", questionStatsOnly(t,
		e.must(huongEmail, http.MethodGet, classQuizzesURL(basic01ID), "", http.StatusOK)))
	percents := func(s QuestionStatsDTO) []any {
		out := make([]any, len(s.Rows))
		for i, r := range s.Rows {
			if r.CorrectPercent != nil {
				out[i] = *r.CorrectPercent
			}
		}
		return out
	}
	got := e.quizItem(assessmentID).QuestionStats
	assert.Equal(t, 3, got.Attempts)
	assert.Equal(t, []any{67, 33, 0}, percents(got))
	practice := e.quizItem(practiceID).QuestionStats
	assert.Zero(t, practice.Attempts)
	assert.Equal(t, []any{nil, nil, nil}, percents(practice))

	e.exec(`UPDATE class_members SET status = 'dropped', dropped_at = now() WHERE id = $1`, memberKhangID)
	got = e.quizItem(assessmentID).QuestionStats
	assert.Equal(t, 2, got.Attempts, "lượt của học viên đã rời lớp không tính")
	assert.Equal(t, []any{50, 0, 0}, percents(got))
}

// questionStatsOnly giữ lại tiêu đề và questionStats của từng quiz trong golden thống kê câu hỏi.
func questionStatsOnly(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v ClassQuizzesDTO
	require.NoError(t, json.Unmarshal(raw, &v))
	type item struct {
		Lesson        string           `json:"lesson"`
		QuestionStats QuestionStatsDTO `json:"questionStats"`
	}
	out := make([]item, len(v.Items))
	for i, it := range v.Items {
		out[i] = item{Lesson: it.Lesson.Title, QuestionStats: it.QuestionStats}
	}
	b, err := json.Marshal(out)
	require.NoError(t, err)
	return b
}
