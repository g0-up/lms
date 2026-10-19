//go:build integration

package seed

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/features/identity"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/testdb"
)

const testSeedPassword = "Seed-Password-Test-1"

// testOutboxKey là khóa AES-256 (base64 của 32 byte) chỉ dùng trong test.
const testOutboxKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func testConfig(t *testing.T) config.Config {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)
	return config.Config{
		AppEnv:          config.EnvDev,
		PublicBaseURL:   "http://localhost:5173",
		TempPasswordTTL: 72 * time.Hour,
		SeedPassword:    testSeedPassword,
		OutboxSecretKey: testOutboxKey,
		Location:        loc,
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

// expectedCounts là số bản ghi mỗi bảng suy ra từ data.go. Bản nháp chép bài học, học liệu và nội dung của bản
// gốc nhưng dùng chung media.
func expectedCounts() map[string]int {
	c, w, qw := contentCounts(), workCounts(), quizWorkCounts()
	d := draftCounts()
	return map[string]int{
		"users":                  len(users),
		"media_files":            videoLessonCount() + c.documents + w.files,
		"stages":                 len(stages),
		"stage_versions":         len(stageVersions) + len(stageDrafts),
		"stage_units":            unitCount() + d.units,
		"lessons":                lessonCount() + d.lessons,
		"quiz_settings":          c.quizzes,
		"questions":              c.questions,
		"question_options":       c.options,
		"homework_settings":      c.homeworks,
		"rubric_criteria":        c.criteria,
		"homework_attachments":   c.attachments,
		"lesson_media":           videoLessonCount() + c.attachments + d.videos,
		"courses":                len(courses),
		"course_versions":        len(courseVersions),
		"course_version_stages":  len(courseVersions[0].stageVersions),
		"classes":                len(classes),
		"class_members":          len(enrollments),
		"email_outbox":           len(enrollments),
		"invitations":            len(enrollments),
		"lesson_progress":        progressCount() + w.progress + qw.progress,
		"class_lesson_deadlines": len(deadlines),
		"submissions":            len(submissions),
		"submission_files":       w.files,
		"submission_grades":      w.grades,
		"grade_criterion_scores": w.scores,
		"quiz_attempts":          len(quizAttempts),
		"quiz_attempt_answers":   qw.answers,
		"quiz_attempt_reviews":   qw.reviews,
		"quiz_review_notes":      qw.notes,
		"stage_reviews":          len(stageReviews),
		"audit_logs":             len(audits) + len(stageDrafts) + len(deadlines) + w.grades + qw.reviews + len(stageReviews),
	}
}

type seededDrafts struct{ units, lessons, videos int }

// draftCounts đếm bài học, học liệu và bài video mà các bản nháp chép từ bản gốc.
func draftCounts() seededDrafts {
	var d seededDrafts
	for _, draft := range stageDrafts {
		from := stageVersionByKey(draft.from)
		d.units += len(from.units)
		d.lessons += len(from.lessons)
		for _, l := range from.lessons {
			if l.typ == "video" {
				d.videos++
			}
		}
	}
	return d
}

type seededWork struct{ files, grades, scores, progress int }

// workCounts đếm bài nộp trong data.go; progress là số cặp (thành viên, homework) đã nộp, mỗi lần chấm có một audit.
func workCounts() seededWork {
	var w seededWork
	pairs := map[string]bool{}
	for _, sub := range submissions {
		w.files += len(sub.files)
		w.grades += len(sub.grades)
		for _, g := range sub.grades {
			w.scores += len(g.points)
		}
		pairs[memberKey(sub.class, sub.user)+"/"+sub.lesson] = true
	}
	w.progress = len(pairs)
	return w
}

type seededQuizWork struct{ answers, progress, reviews, notes int }

// quizWorkCounts đếm lượt quiz trong data.go: mỗi câu đã lưu một dòng trả lời, mỗi cặp (thành viên, quiz) một dòng tiến
// độ, mỗi lần nhận xét một dòng review kèm một audit.
func quizWorkCounts() seededQuizWork {
	var w seededQuizWork
	pairs := map[string]bool{}
	for _, a := range quizAttempts {
		w.answers += len(a.mask)
		if a.review != nil {
			w.reviews++
			w.notes += len(a.review.notes)
		}
		pairs[memberKey(a.class, a.user)+"/"+a.lesson] = true
	}
	w.progress = len(pairs)
	return w
}

type seededContent struct {
	quizzes, questions, options, homeworks, criteria, attachments, documents int
}

// contentCounts đếm nội dung quiz/homework trong data.go, gồm bản chép trong bản nháp; documents là số file kèm
// khác tên.
func contentCounts() seededContent {
	var c seededContent
	docs := map[string]bool{}
	versions := slices.Clone(stageVersions)
	for _, d := range stageDrafts {
		versions = append(versions, stageVersionByKey(d.from))
	}
	for _, sv := range versions {
		for _, l := range sv.lessons {
			if q := l.quiz; q != nil {
				c.quizzes++
				c.questions += len(q.questions)
				for _, qs := range q.questions {
					c.options += len(qs.options)
				}
			}
			if hw := l.homework; hw != nil {
				c.homeworks++
				c.criteria += len(hw.criteria)
				c.attachments += len(hw.attachments)
				for _, name := range hw.attachments {
					docs[name] = true
				}
			}
		}
	}
	c.documents = len(docs)
	return c
}

func tableCounts(t *testing.T, dbx *sqlx.DB) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, table := range insertOrder {
		var n int
		// Tên bảng lấy từ danh sách hằng insertOrder, không từ đầu vào ngoài.
		require.NoError(t, dbx.GetContext(testContext(t), &n, "SELECT count(*) FROM "+table))
		out[table] = n
	}
	return out
}

func resultCounts(res Result) map[string]int {
	out := map[string]int{}
	for _, c := range res.Counts {
		out[c.Table] = c.Rows
	}
	return out
}

func adminID(t *testing.T, dbx *sqlx.DB) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, dbx.GetContext(testContext(t), &id, `SELECT id FROM users WHERE email_normalized = $1`, markerEmail))
	return id
}

// openEmpty trả database test rỗng và dọn lại khi test xong: seed có học liệu quiz/homework và media document, mà
// migration 0011 down (round trip ở package migrations chạy ngay sau) chỉ chạy được trên database không có chúng.
func openEmpty(t *testing.T) *sqlx.DB {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	t.Cleanup(func() { testdb.Reset(t, dbx) })
	return dbx
}

func TestRunSeedsPrototypeDatasetIdempotently(t *testing.T) {
	dbx := openEmpty(t)
	ctx := testContext(t)
	cfg := testConfig(t)
	clk := fixedClock{time.Now()}

	res, err := Run(ctx, dbx, cfg, clk, Options{})
	require.NoError(t, err)
	assert.False(t, res.AlreadySeeded)
	want := expectedCounts()
	assert.Equal(t, want, resultCounts(res))
	assert.Equal(t, want, tableCounts(t, dbx))
	assert.Equal(t, 17, want["users"])
	assert.Equal(t, 30+9, want["lessons"], "30 học liệu đã phát hành và 9 học liệu của bản nháp Go v2")

	again, err := Run(ctx, dbx, cfg, clk, Options{})
	require.NoError(t, err)
	assert.True(t, again.AlreadySeeded)
	assert.Empty(t, again.Counts)
	assert.Equal(t, want, tableCounts(t, dbx), "chạy lần hai không đổi dữ liệu")
}

func TestRunResetReplacesData(t *testing.T) {
	dbx := openEmpty(t)
	ctx := testContext(t)
	cfg := testConfig(t)
	clk := fixedClock{time.Now()}

	_, err := Run(ctx, dbx, cfg, clk, Options{})
	require.NoError(t, err)
	before := adminID(t, dbx)

	res, err := Run(ctx, dbx, cfg, clk, Options{Reset: true})
	require.NoError(t, err)
	assert.False(t, res.AlreadySeeded)
	assert.Equal(t, expectedCounts(), tableCounts(t, dbx))
	assert.NotEqual(t, before, adminID(t, dbx), "reset xóa dữ liệu cũ rồi seed lại với id mới")
}

func TestRunRejectsInvalidInputWithoutWriting(t *testing.T) {
	dbx := openEmpty(t)
	ctx := testContext(t)
	clk := fixedClock{time.Now()}

	prod := testConfig(t)
	prod.AppEnv = config.EnvProduction
	_, err := Run(ctx, dbx, prod, clk, Options{Reset: true})
	require.ErrorIs(t, err, ErrProduction)

	noPassword := testConfig(t)
	noPassword.SeedPassword = ""
	_, err = Run(ctx, dbx, noPassword, clk, Options{})
	require.ErrorIs(t, err, ErrSeedPassword)

	var n int
	require.NoError(t, dbx.GetContext(ctx, &n, `SELECT count(*) FROM users`))
	assert.Zero(t, n)
}

func TestSeededDataSatisfiesApplicationInvariants(t *testing.T) {
	dbx := openEmpty(t)
	ctx := testContext(t)
	cfg := testConfig(t)
	now := time.Now()
	_, err := Run(ctx, dbx, cfg, fixedClock{now}, Options{})
	require.NoError(t, err)

	count := func(q string, args ...any) int {
		t.Helper()
		var n int
		require.NoError(t, dbx.GetContext(ctx, &n, q, args...), q)
		return n
	}

	t.Run("published markdown lessons are rendered", func(t *testing.T) {
		assert.Zero(t, count(`SELECT count(*) FROM lessons l JOIN stage_versions sv ON sv.id = l.stage_version_id
			WHERE sv.status = 'published' AND l.type = 'markdown' AND l.markdown_html IS NULL`))
		assert.Equal(t, 8, count(`SELECT count(*) FROM lessons l JOIN stage_versions sv ON sv.id = l.stage_version_id
			WHERE sv.status = 'published' AND l.type = 'markdown'`))
		// Bản nháp Go v2 chép nguyên HTML đã render của ba bài markdown Go v1.
		assert.Equal(t, 3, count(`SELECT count(*) FROM lessons l JOIN stage_versions sv ON sv.id = l.stage_version_id
			WHERE sv.status = 'draft' AND l.type = 'markdown' AND l.markdown_html IS NOT NULL`))
	})

	t.Run("lessons follow their units in flat position order", func(t *testing.T) {
		var layout []string
		require.NoError(t, dbx.SelectContext(ctx, &layout, `
			SELECT u.position || '.' || u.title || ':' || string_agg(l.lesson_key, ',' ORDER BY l.position)
			  FROM stage_units u JOIN lessons l ON l.unit_id = u.id
			  JOIN stage_versions sv ON sv.id = u.stage_version_id JOIN stages s ON s.id = sv.stage_id
			 WHERE s.code = 'GO' AND sv.status = 'published' GROUP BY u.id ORDER BY u.position`))
		assert.Equal(t, []string{
			"1.Cài đặt và Hello World:go-setup,go-hello-read,go-hello-practice", "2.Kiểu dữ liệu và hàm:go-types,go-exercise",
			"3.Goroutine và channel:go-routine,go-practice", "4.Kiểm tra cuối chặng:go-hw,go-quiz",
		}, layout)
		// Thứ tự phẳng tăng theo (bài học, thứ tự trong bài học) ở mọi phiên bản.
		assert.Zero(t, count(`SELECT count(*) FROM (
			SELECT l.position, row_number() OVER (PARTITION BY l.stage_version_id ORDER BY u.position, l.position) AS rn
			  FROM lessons l JOIN stage_units u ON u.id = l.unit_id) x WHERE x.position <> x.rn`))
	})

	t.Run("every video lesson has duration and lesson media", func(t *testing.T) {
		assert.Zero(t, count(`SELECT count(*) FROM lessons l WHERE l.type = 'video'
			AND (l.duration_seconds IS NULL OR NOT EXISTS (SELECT 1 FROM lesson_media lm WHERE lm.lesson_id = l.id AND lm.media_id = l.video_media_id))`))
		assert.Equal(t, 1104, count(`SELECT duration_seconds FROM lessons WHERE lesson_key = 'db-intro'`))
		assert.Equal(t, 9, count(`SELECT count(*) FROM media_files WHERE status = 'ready' AND storage_key LIKE 'seed/%.mp4'`))
	})

	t.Run("quiz and homework content is ready to publish", func(t *testing.T) {
		c := contentCounts()
		// 11 quiz và 2 homework đã phát hành, cộng ba quiz và một homework (ba tiêu chí) chép sang bản nháp Go v2.
		require.Equal(t, seededContent{quizzes: 14, questions: c.questions, options: c.options, homeworks: 3, criteria: 9,
			attachments: 1, documents: 1}, c)
		// Quiz assessment có đủ cấu hình, practice để NULL theo ck_quiz_settings_rules.
		assert.Equal(t, 6, count(`SELECT count(*) FROM quiz_settings WHERE mode = 'assessment'
			AND max_attempts IS NOT NULL AND time_limit_seconds IS NOT NULL AND pass_percent IS NOT NULL`))
		assert.Equal(t, 8, count(`SELECT count(*) FROM quiz_settings WHERE mode = 'practice'
			AND max_attempts IS NULL AND time_limit_seconds IS NULL AND pass_percent IS NULL`))
		assert.Equal(t, 600, count(`SELECT qs.time_limit_seconds FROM quiz_settings qs JOIN lessons l ON l.id = qs.lesson_id
			WHERE l.lesson_key = 'db-test'`))
		// Bản nháp giữ nguyên cấu hình quiz của bản gốc.
		assert.Equal(t, 900, count(`SELECT qs.time_limit_seconds FROM quiz_settings qs JOIN lessons l ON l.id = qs.lesson_id
			JOIN stage_versions sv ON sv.id = l.stage_version_id WHERE l.lesson_key = 'go-quiz' AND sv.status = 'draft'`))
		// Phiên bản đã phát hành không còn câu hỏi hay đề bài chưa render.
		assert.Zero(t, count(`SELECT count(*) FROM questions WHERE body_html IS NULL
			OR (explanation_source <> '' AND explanation_html IS NULL)`))
		assert.Zero(t, count(`SELECT count(*) FROM homework_settings WHERE brief_html IS NULL OR brief_source = ''`))
		// Mỗi câu hỏi có đáp án đúng; câu single đúng một đáp án.
		assert.Zero(t, count(`SELECT count(*) FROM questions q WHERE NOT EXISTS
			(SELECT 1 FROM question_options o WHERE o.question_id = q.id AND o.is_correct)`))
		assert.Zero(t, count(`SELECT count(*) FROM questions q WHERE q.type = 'single' AND
			(SELECT count(*) FROM question_options o WHERE o.question_id = q.id AND o.is_correct) <> 1`))
		// File kèm là document đã upload xong và nằm trong lesson_media của homework.
		assert.Zero(t, count(`SELECT count(*) FROM homework_attachments a JOIN media_files m ON m.id = a.media_id
			WHERE m.kind <> 'document' OR m.status <> 'ready'
			   OR NOT EXISTS (SELECT 1 FROM lesson_media lm WHERE lm.lesson_id = a.lesson_id AND lm.media_id = a.media_id)`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM media_files WHERE kind = 'document'
			AND original_name = 'ds-stack-queue-starter.zip' AND content_type = 'application/zip'`))
	})

	t.Run("password hash matches SEED_PASSWORD", func(t *testing.T) {
		var hashes []string
		require.NoError(t, dbx.SelectContext(ctx, &hashes, `SELECT DISTINCT password_hash FROM users`))
		require.Len(t, hashes, 1, "mọi user dùng cùng hash của SEED_PASSWORD")
		assert.True(t, identity.PasswordHashFromPHC(hashes[0]).Verify(testSeedPassword))
	})

	t.Run("user states follow the prototype", func(t *testing.T) {
		assert.Equal(t, 5, count(`SELECT count(*) FROM users WHERE status = 'invited' AND must_change_password AND temp_password_expires_at IS NOT NULL`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM users WHERE status = 'disabled' AND disabled_at IS NOT NULL`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM class_members WHERE status = 'dropped' AND dropped_at IS NOT NULL`))
	})

	t.Run("outbox rows follow the worker contract", func(t *testing.T) {
		assert.Zero(t, count(`SELECT count(*) FROM email_outbox WHERE secret_enc IS NOT NULL AND status <> 'queued'`))
		var sealed [][]byte
		require.NoError(t, dbx.SelectContext(ctx, &sealed, `SELECT secret_enc FROM email_outbox WHERE status = 'queued'`))
		require.Len(t, sealed, 1)
		box, err := mailer.NewSecretBox(testOutboxKey)
		require.NoError(t, err)
		plain, err := box.Open(sealed[0])
		require.NoError(t, err, "hàng queued phải có secret_enc mở được để worker gửi")
		assert.Equal(t, testSeedPassword, string(plain))
		assert.Zero(t, count(`SELECT count(*) FROM email_outbox WHERE payload::text ILIKE '%password%'`))
		assert.Equal(t, 13, count(`SELECT count(*) FROM email_outbox WHERE status = 'sent' AND sent_at IS NOT NULL AND template = 'invite'`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM email_outbox WHERE status = 'failed' AND attempts = 3 AND last_error IS NOT NULL`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM email_outbox WHERE status = 'queued' AND attempts = 0`))
		assert.Equal(t, len(enrollments), count(`SELECT count(*) FROM invitations i JOIN email_outbox o ON o.id = i.email_outbox_id
			JOIN users u ON u.id = i.user_id WHERE o.to_email = u.email AND i.kind = 'invite'`))
	})

	t.Run("classes and course reference fixed versions", func(t *testing.T) {
		assert.Equal(t, 2, count(`SELECT count(*) FROM classes WHERE status = 'active'`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM classes WHERE status = 'draft' AND code = 'basic03'`))
		var start string
		require.NoError(t, dbx.GetContext(ctx, &start, `SELECT start_date::text FROM classes WHERE code = 'basic01'`))
		assert.Equal(t, ago(58, 0).at(now).In(cfg.Location).Format(time.DateOnly), start)
		assert.Equal(t, 5, count(`SELECT count(*) FROM course_version_stages`))
	})

	t.Run("audit uses canonical actions without secrets", func(t *testing.T) {
		assert.Zero(t, count(`SELECT count(*) FROM audit_logs WHERE action NOT IN (
			'stage_version.published', 'course_version.published', 'class.activated', 'class.member_invited',
			'user.disabled', 'class.created', 'class.invitation_resent', 'class.homework_deadline_changed',
			'homework.graded', 'homework.grade_updated', 'quiz_attempt.confirmed', 'quiz_attempt.commented',
			'stage_review.saved', 'stage_version.cloned')`))
		assert.Zero(t, count(`SELECT count(*) FROM audit_logs WHERE coalesce(after::text, '') ILIKE '%password%' OR coalesce(before::text, '') ILIKE '%password%'`))
		assert.Zero(t, count(`SELECT count(*) FROM audit_logs WHERE actor_id IS NULL OR target_id IS NULL`))
	})

	t.Run("homework work follows the service rules", func(t *testing.T) {
		var byStatus []string
		require.NoError(t, dbx.SelectContext(ctx, &byStatus,
			`SELECT status || ':' || count(*) FROM submissions GROUP BY status ORDER BY status`))
		assert.Equal(t, []string{"passed:2", "pending:5", "rework:2", "superseded:1"}, byStatus)
		// Muộn đúng theo hạn của lớp: An nộp lại, Bích và Phong ở basic01 sau hạn 20 ngày trước.
		assert.Equal(t, 3, count(`SELECT count(*) FROM submissions WHERE is_late`))
		assert.Zero(t, count(`SELECT count(*) FROM submissions s JOIN class_members m ON m.id = s.class_member_id
			LEFT JOIN class_lesson_deadlines d ON d.class_id = m.class_id AND d.lesson_id = s.lesson_id
			WHERE s.is_late <> coalesce(s.submitted_at > d.due_at, false)`))
		// Trạng thái bản đã chấm là kết quả lần chấm mới nhất; có đúng một bản được sửa điểm.
		assert.Zero(t, count(`SELECT count(*) FROM submissions s JOIN LATERAL (SELECT result FROM submission_grades g
			WHERE g.submission_id = s.id ORDER BY graded_at DESC LIMIT 1) g ON true WHERE s.status <> g.result`))
		assert.Zero(t, count(`SELECT count(*) FROM submissions s WHERE s.status IN ('passed', 'rework')
			AND NOT EXISTS (SELECT 1 FROM submission_grades g WHERE g.submission_id = s.id)`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM (SELECT submission_id FROM submission_grades GROUP BY submission_id HAVING count(*) > 1) x`))
		// Tổng điểm khớp điểm từng tiêu chí, điểm tối đa khớp rubric, kết quả theo ngưỡng đạt.
		assert.Zero(t, count(`SELECT count(*) FROM submission_grades g JOIN submissions s ON s.id = g.submission_id
			JOIN homework_settings h ON h.lesson_id = s.lesson_id
			WHERE g.total_points <> (SELECT sum(points) FROM grade_criterion_scores c WHERE c.grade_id = g.id)
			   OR g.max_points <> (SELECT sum(max_points) FROM rubric_criteria r WHERE r.lesson_id = s.lesson_id)
			   OR (g.result = 'passed') <> (g.total_points * 100 >= h.pass_percent * g.max_points)`))
		// Hoàn thành homework đúng khi bản nộp mới nhất Đạt.
		assert.Zero(t, count(`SELECT count(*) FROM lesson_progress p JOIN lessons l ON l.id = p.lesson_id AND l.type = 'homework'
			WHERE (p.completed_at IS NOT NULL) <> EXISTS (SELECT 1 FROM submissions s WHERE s.class_member_id = p.class_member_id
			  AND s.lesson_id = p.lesson_id AND s.status = 'passed')`))
		assert.Equal(t, 2, count(`SELECT count(*) FROM lesson_progress p JOIN lessons l ON l.id = p.lesson_id
			WHERE l.type = 'homework' AND p.completed_at IS NOT NULL`))
		// File nộp là document của chính học viên, đã sẵn sàng.
		assert.Zero(t, count(`SELECT count(*) FROM submission_files f JOIN media_files mf ON mf.id = f.media_id
			JOIN submissions s ON s.id = f.submission_id JOIN class_members m ON m.id = s.class_member_id
			WHERE mf.kind <> 'document' OR mf.status <> 'ready' OR mf.uploaded_by <> m.user_id
			   OR mf.storage_key <> 'seed/submissions/' || s.id || '/' || mf.original_name`))
		var actions []string
		require.NoError(t, dbx.SelectContext(ctx, &actions, `SELECT action || ':' || count(*) FROM audit_logs
			WHERE action IN ('class.homework_deadline_changed', 'homework.graded', 'homework.grade_updated') GROUP BY action ORDER BY action`))
		assert.Equal(t, []string{"class.homework_deadline_changed:3", "homework.grade_updated:1", "homework.graded:4"}, actions)
		assert.Zero(t, count(`SELECT count(*) FROM audit_logs WHERE action = 'homework.grade_updated' AND before IS NULL`))
	})

	t.Run("quiz attempts follow the service rules", func(t *testing.T) {
		var byStatus []string
		require.NoError(t, dbx.SelectContext(ctx, &byStatus, `SELECT qs.mode || ':' || coalesce(a.review_status, '-') || ':' || count(*)
			FROM quiz_attempts a JOIN quiz_settings qs ON qs.lesson_id = a.lesson_id
			WHERE a.submitted_at IS NOT NULL GROUP BY qs.mode, a.review_status ORDER BY 1`))
		assert.Equal(t, []string{"assessment:confirmed:10", "assessment:pending:6", "assessment:superseded:1", "practice:-:13",
			"practice:confirmed:2"}, byStatus)
		// Lượt đã xác nhận (hoặc đã nhận xét) có đúng một lần nhận xét sau lúc nộp, do giảng viên lớp hoặc admin, kèm
		// một audit; lượt khác không có nhận xét. Ghi chú thuộc câu của lượt.
		assert.Zero(t, count(`SELECT count(*) FROM quiz_attempts a WHERE (a.review_status = 'confirmed')
			<> EXISTS (SELECT 1 FROM quiz_attempt_reviews r WHERE r.attempt_id = a.id)`))
		assert.Zero(t, count(`SELECT count(*) FROM quiz_attempt_reviews r JOIN quiz_attempts a ON a.id = r.attempt_id
			JOIN class_members m ON m.id = a.class_member_id JOIN classes c ON c.id = m.class_id JOIN users u ON u.id = r.reviewer_id
			WHERE r.created_at <= a.submitted_at OR r.created_at > $1 OR (u.role <> 'admin' AND u.id <> c.teacher_id)`, now))
		assert.Zero(t, count(`SELECT count(*) FROM quiz_review_notes n JOIN quiz_attempt_reviews r ON r.id = n.review_id
			JOIN quiz_attempts a ON a.id = r.attempt_id WHERE NOT n.question_id = ANY (a.question_order)`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM quiz_attempt_reviews r JOIN users u ON u.id = r.reviewer_id
			JOIN quiz_attempts a ON a.id = r.attempt_id JOIN class_members m ON m.id = a.class_member_id
			JOIN classes c ON c.id = m.class_id WHERE u.email = 'bao.pham@goup.vn' AND c.code = 'basic02'`))
		var actions []string
		require.NoError(t, dbx.SelectContext(ctx, &actions, `SELECT action || ':' || count(*) FROM audit_logs
			WHERE action LIKE 'quiz_attempt.%' GROUP BY action ORDER BY action`))
		assert.Equal(t, []string{"quiz_attempt.commented:2", "quiz_attempt.confirmed:10"}, actions)
		assert.Zero(t, count(`SELECT count(*) FROM audit_logs l WHERE l.action LIKE 'quiz_attempt.%' AND (l.before IS NOT NULL
			OR NOT EXISTS (SELECT 1 FROM quiz_attempt_reviews r WHERE r.id IS NOT NULL AND r.attempt_id = (l.after->>'attemptId')::uuid
			  AND r.reviewer_id = l.actor_id AND r.created_at = l.at AND r.comment = l.after->>'comment'))`))
		// Lượt đang làm duy nhất là của Bích, đã lưu hai câu chưa chấm.
		assert.Equal(t, 1, count(`SELECT count(*) FROM quiz_attempts a JOIN class_members m ON m.id = a.class_member_id
			JOIN users u ON u.id = m.user_id WHERE a.submitted_at IS NULL AND u.email = 'bich.tran@gmail.com'
			AND (SELECT count(*) FROM quiz_attempt_answers x WHERE x.attempt_id = a.id AND x.is_correct IS NULL AND x.points_awarded IS NULL) = 2`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM quiz_attempts WHERE submitted_at IS NULL`))
		// Điểm là tổng điểm từng câu, điểm tối đa là tổng điểm câu hỏi, Đạt theo ngưỡng (chỉ assessment).
		assert.Zero(t, count(`SELECT count(*) FROM quiz_attempts a JOIN quiz_settings qs ON qs.lesson_id = a.lesson_id
			WHERE a.submitted_at IS NOT NULL AND (
			      a.score <> (SELECT sum(points_awarded) FROM quiz_attempt_answers x WHERE x.attempt_id = a.id)
			   OR a.max_score <> (SELECT sum(points) FROM questions q WHERE q.lesson_id = a.lesson_id)
			   OR (SELECT count(*) FROM quiz_attempt_answers x WHERE x.attempt_id = a.id)
			      <> (SELECT count(*) FROM questions q WHERE q.lesson_id = a.lesson_id)
			   OR a.passed IS DISTINCT FROM CASE WHEN qs.mode = 'assessment' THEN a.score * 100 >= qs.pass_percent * a.max_score END)`))
		// Câu đúng khi tập phương án chọn bằng tập đáp án đúng; câu lưu trong lúc làm, lượt nộp trước hạn.
		assert.Zero(t, count(`SELECT count(*) FROM quiz_attempt_answers x JOIN quiz_attempts a ON a.id = x.attempt_id
			WHERE a.submitted_at IS NOT NULL AND x.is_correct <> (x.selected_option_ids::uuid[] @> (SELECT array_agg(o.id)
			  FROM question_options o WHERE o.question_id = x.question_id AND o.is_correct) AND x.selected_option_ids::uuid[] <@
			  (SELECT array_agg(o.id) FROM question_options o WHERE o.question_id = x.question_id AND o.is_correct))`))
		assert.Zero(t, count(`SELECT count(*) FROM quiz_attempt_answers x JOIN quiz_attempts a ON a.id = x.attempt_id
			WHERE x.saved_at < a.started_at OR x.saved_at > coalesce(a.submitted_at, $1)`, now))
		assert.Zero(t, count(`SELECT count(*) FROM quiz_attempts WHERE submitted_at > deadline_at OR started_at > $1
			OR cardinality(question_order) <> (SELECT count(*) FROM questions q WHERE q.lesson_id = quiz_attempts.lesson_id)`, now))
		// Hoàn thành quiz: practice khi đã nộp một lượt, assessment khi có lượt Đạt; mỗi lượt có dòng tiến độ.
		assert.Zero(t, count(`SELECT count(*) FROM lesson_progress p JOIN quiz_settings qs ON qs.lesson_id = p.lesson_id
			WHERE (p.completed_at IS NOT NULL) <> EXISTS (SELECT 1 FROM quiz_attempts a WHERE a.class_member_id = p.class_member_id
			  AND a.lesson_id = p.lesson_id AND a.submitted_at IS NOT NULL AND (qs.mode = 'practice' OR a.passed))`))
		assert.Zero(t, count(`SELECT count(*) FROM quiz_attempts a WHERE NOT EXISTS (SELECT 1 FROM lesson_progress p
			WHERE p.class_member_id = a.class_member_id AND p.lesson_id = a.lesson_id AND p.first_opened_at <= a.started_at)`))
		// Lượt tự nộp duy nhất của Khang: hệ thống nộp đúng lúc hết giờ, giảng viên đã xác nhận sau đó.
		assert.Equal(t, 1, count(`SELECT count(*) FROM quiz_attempts WHERE auto_submitted`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM quiz_attempts a JOIN class_members m ON m.id = a.class_member_id
			JOIN users u ON u.id = m.user_id JOIN lessons l ON l.id = a.lesson_id JOIN quiz_settings qs ON qs.lesson_id = a.lesson_id
			WHERE a.auto_submitted AND u.email = 'khang.vu@gmail.com' AND l.lesson_key = 'db-test' AND qs.mode = 'assessment'
			  AND a.submitted_at = a.deadline_at AND a.deadline_at = a.started_at + make_interval(secs => qs.time_limit_seconds)
			  AND a.review_status = 'confirmed'`))
	})

	t.Run("stage reviews follow the service rules", func(t *testing.T) {
		// Nhận xét viết một lần (revision 1) bởi giảng viên lớp hoặc admin, cho chặng thuộc khóa học của lớp, kèm đúng
		// một audit stage_review.saved cùng nội dung và thời điểm.
		assert.Equal(t, 2, count(`SELECT count(*) FROM stage_reviews r JOIN class_members m ON m.id = r.class_member_id
			JOIN classes c ON c.id = m.class_id JOIN users u ON u.id = r.author_id
			JOIN course_version_stages cvs ON cvs.course_version_id = c.course_version_id AND cvs.stage_version_id = r.stage_version_id
			WHERE r.revision = 1 AND r.created_at = r.updated_at AND r.created_at <= $1 AND (u.role = 'admin' OR u.id = c.teacher_id)
			  AND r.body = btrim(r.body)`, now))
		assert.Equal(t, 2, count(`SELECT count(*) FROM audit_logs l JOIN stage_reviews r ON r.class_member_id = (l.after->>'memberId')::uuid
			AND r.stage_version_id = (l.after->>'stageVersionId')::uuid
			JOIN class_members m ON m.id = r.class_member_id
			WHERE l.action = 'stage_review.saved' AND l.before IS NULL AND l.target_id = m.class_id AND l.actor_id = r.author_id
			  AND l.at = r.created_at AND l.after->>'body' = r.body AND (l.after->>'revision')::int = 1
			  AND l.after->>'stageName' = 'Data structure'`))
	})

	t.Run("go v2 draft is a faithful clone of go v1", func(t *testing.T) {
		// Bản nháp duy nhất: chưa phát hành, nhân bản từ Go v1, không thuộc khóa học nào, kèm audit nhân bản.
		assert.Equal(t, 1, count(`SELECT count(*) FROM stage_versions WHERE status = 'draft'`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM stage_versions d JOIN stage_versions src ON src.id = d.cloned_from_id
			JOIN stages s ON s.id = d.stage_id
			WHERE d.status = 'draft' AND d.version_no = 2 AND d.published_at IS NULL AND d.title = src.title
			  AND s.code = 'GO' AND src.stage_id = d.stage_id AND src.version_no = 1 AND src.status = 'published'
			  AND d.created_at > src.published_at`))
		assert.Zero(t, count(`SELECT count(*) FROM course_version_stages cvs JOIN stage_versions sv ON sv.id = cvs.stage_version_id
			WHERE sv.status = 'draft'`))
		assert.Equal(t, 1, count(`SELECT count(*) FROM audit_logs l JOIN stage_versions d ON d.id = l.target_id
			WHERE l.action = 'stage_version.cloned' AND l.target_type = 'stage_version' AND l.before IS NULL
			  AND l.at = d.created_at AND l.after->>'stageId' = d.stage_id::text
			  AND (l.after->>'fromVersionNo')::int = 1 AND (l.after->>'toVersionNo')::int = 2`))
		// Bài học và học liệu giữ khóa, thứ tự, loại và nội dung của bản gốc; video dùng chung media.
		layout := func(status string) []string {
			var out []string
			require.NoError(t, dbx.SelectContext(ctx, &out, `
				SELECT u.unit_key || '/' || u.position || '/' || l.lesson_key || '/' || l.position || '/' || l.type || '/'
				       || l.required || '/' || coalesce(md5(l.markdown_html), '-') || '/' || coalesce(l.video_media_id::text, '-')
				       || '/' || (SELECT count(*) FROM questions q WHERE q.lesson_id = l.id)
				       || '/' || (SELECT count(*) FROM rubric_criteria r WHERE r.lesson_id = l.id)
				  FROM lessons l JOIN stage_units u ON u.id = l.unit_id
				  JOIN stage_versions sv ON sv.id = l.stage_version_id JOIN stages s ON s.id = sv.stage_id
				 WHERE s.code = 'GO' AND sv.status = $1 ORDER BY l.position`, status))
			return out
		}
		published := layout("published")
		require.Len(t, published, 9)
		assert.Equal(t, published, layout("draft"))
	})
}
