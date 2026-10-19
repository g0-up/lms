//go:build integration

package grading_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/features/grading"
	"lms/api/internal/platform/db"
)

// addTeacher chèn một giảng viên chưa phụ trách lớp nào; disabled là tài khoản đã bị khóa.
func (e *itEnv) addTeacher(name, email string, disabled bool) uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	status, disabledAt := "active", (*time.Time)(nil)
	if disabled {
		now := e.clk.Now()
		status, disabledAt = "disabled", &now
	}
	e.exec(`INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash, disabled_at)
		VALUES ($1, $2, $2, $3, 'teacher', $4, 'x', $5)`, id, email, name, status, disabledAt)
	return id
}

type submitted struct {
	ID          uuid.UUID `db:"id"`
	SubmittedAt time.Time `db:"submitted_at"`
}

func (e *itEnv) submissionOf(classCode, email, key string) submitted {
	e.t.Helper()
	var s submitted
	require.NoError(e.t, e.dbx.Get(&s, `SELECT id, submitted_at FROM submissions
		WHERE class_member_id = $1 AND lesson_id = $2 ORDER BY attempt_no LIMIT 1`,
		e.memberID(classCode, email), e.lessonID(key)))
	return s
}

func (e *itEnv) grade(sub submitted, by uuid.UUID, after time.Duration) {
	e.t.Helper()
	e.exec(`INSERT INTO submission_grades (id, submission_id, graded_by, total_points, max_points, result, comment, graded_at)
		VALUES ($1, $2, $3, 8, 10, 'passed', 'Ổn.', $4)`, uuid.New(), sub.ID, by, sub.SubmittedAt.Add(after))
}

func (e *itEnv) review(at submitted, by uuid.UUID, after time.Duration) {
	e.t.Helper()
	e.exec(`INSERT INTO quiz_attempt_reviews (id, attempt_id, reviewer_id, comment, created_at) VALUES ($1, $2, $3, 'Ổn.', $4)`,
		uuid.New(), at.ID, by, at.SubmittedAt.Add(after))
}

func (e *itEnv) backlog() ([]grading.TeacherBacklog, int) {
	e.t.Helper()
	var (
		rows    []grading.TeacherBacklog
		overdue int
	)
	err := readTx{e.dbx}.Transact(context.Background(), func(tx db.Executor) error {
		var err error
		rows, overdue, err = e.svc.Backlog(context.Background(), tx, e.clk.Now())
		return err
	})
	require.NoError(e.t, err)
	return rows, overdue
}

func TestBacklogAvgHoursOnFixedData(t *testing.T) {
	e := newITAt(t, jobsNow)
	e.exec(`DELETE FROM submission_grades`)
	e.exec(`DELETE FROM quiz_attempt_reviews`)
	adminID, huongID, baoID := e.userID(adminEmail), e.userID(huongEmail), e.userID(baoEmail)
	e.addTeacher("Mai Văn Mới", "moi.mai@goup.vn", false)
	e.addTeacher("Bùi Đã Nghỉ", "nghi.bui@goup.vn", true)

	// Hương chấm đầu hai bài (10 giờ, 20 giờ); lần sửa điểm của Admin không tính.
	bich := e.submissionOf("basic01", bichEmail, "ds-hw")
	e.grade(bich, huongID, 10*time.Hour)
	e.grade(bich, adminID, 50*time.Hour)
	e.grade(e.submissionOf("basic01", phongEmail, "ds-hw"), huongID, 20*time.Hour)
	// Bảo chấm đầu bài của Phong ở basic02 sau 6 giờ; lần sửa của Hương không tính cho Hương.
	phong2 := e.submissionOf("basic02", phongEmail, "ds-hw")
	e.grade(phong2, baoID, 6*time.Hour)
	e.grade(phong2, huongID, 100*time.Hour)
	// Hương xác nhận đầu lượt quiz bài tập của Hà sau 2 giờ; nhận xét quiz luyện tập của Bảo không tính.
	var assessment, practice submitted
	require.NoError(t, e.dbx.Get(&assessment, `SELECT a.id, a.submitted_at FROM quiz_attempts a
		JOIN quiz_settings qs ON qs.lesson_id = a.lesson_id AND qs.mode = 'assessment'
		WHERE a.class_member_id = $1 AND a.lesson_id = $2 AND a.submitted_at IS NOT NULL ORDER BY a.attempt_no LIMIT 1`,
		e.memberID("basic01", haEmail), e.lessonID("db-test")))
	e.review(assessment, huongID, 2*time.Hour)
	e.review(assessment, adminID, 40*time.Hour)
	require.NoError(t, e.dbx.Get(&practice, `SELECT a.id, a.submitted_at FROM quiz_attempts a
		JOIN quiz_settings qs ON qs.lesson_id = a.lesson_id AND qs.mode = 'practice'
		WHERE a.submitted_at IS NOT NULL ORDER BY a.id LIMIT 1`))
	e.review(practice, baoID, 300*time.Hour)

	rows, overdue := e.backlog()
	byName := map[string]grading.TeacherBacklog{}
	names := make([]string, len(rows))
	for i, r := range rows {
		byName[r.TeacherName], names[i] = r, r.TeacherName
	}
	assert.Equal(t, []string{"Lê Thu Hương", "Mai Văn Mới", "Phạm Quốc Bảo"}, names,
		"mọi giảng viên không bị khóa, theo tên")

	require.NotNil(t, byName["Lê Thu Hương"].AvgHours)
	assert.InDelta(t, 32.0/3, *byName["Lê Thu Hương"].AvgHours, 1e-9, "(10 + 20 + 2) / 3")
	require.NotNil(t, byName["Phạm Quốc Bảo"].AvgHours)
	assert.InDelta(t, 6.0, *byName["Phạm Quốc Bảo"].AvgHours, 1e-9)
	assert.Nil(t, byName["Mai Văn Mới"].AvgHours, "chưa xử lý mục nào")
	assert.Zero(t, byName["Mai Văn Mới"].Pending)

	for _, email := range []string{huongEmail, baoEmail} {
		q, err := e.svc.Queue(context.Background(), e.actor(email), grading.Filter{})
		require.NoError(t, err)
		var row grading.TeacherBacklog
		for _, r := range rows {
			if r.TeacherID == e.userID(email) {
				row = r
			}
		}
		assert.Equal(t, q.Total, row.Pending, "pending khớp hàng chờ của %s", email)
		assert.Equal(t, q.OverdueCount, row.Overdue, "overdue khớp hàng chờ của %s", email)
	}
	assert.Equal(t, e.overdueOf(adminEmail), overdue, "tổng quá hạn khớp hàng chờ Admin")
}
