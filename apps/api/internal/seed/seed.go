// Package seed nạp dữ liệu mẫu tương đương prototype/seed.js cho môi trường dev và e2e.
//
// Seed ghi SQL trực tiếp (không qua repository) trong một transaction: header phiên bản chặng/khóa học được chèn
// thẳng ở trạng thái cuối (published) nhưng dữ liệu vẫn thỏa mọi bất biến ứng dụng giả định (bài markdown của bản
// published có HTML render sẵn, course_version_stages.stage_id khớp phiên bản chặng).
package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jmoiron/sqlx"

	homeworkfeature "lms/api/internal/features/homework"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/mailer"
	"lms/api/internal/features/media"
	"lms/api/internal/features/quizzes"
	stagefeature "lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/secretbox"
	"lms/api/internal/platform/storage"
)

// Options là các cờ của lệnh seed.
type Options struct {
	// Reset xóa toàn bộ dữ liệu (trừ schema_migrations) rồi seed lại; chỉ cho phép ở dev|e2e.
	Reset bool
	// UploadSample tải video mẫu lên storage_key của mọi bài video mẫu (S3_*), sau khi seed xong.
	UploadSample bool
}

// TableCount là số bản ghi seed đã chèn vào một bảng.
type TableCount struct {
	Table string
	Rows  int
}

// Result mô tả kết quả một lần chạy seed.
type Result struct {
	// AlreadySeeded: database đã có dữ liệu mẫu nên không chèn gì.
	AlreadySeeded bool
	// Counts theo thứ tự chèn (thứ tự khóa ngoại).
	Counts []TableCount
}

// Lỗi kiểm tra đầu vào của seed.
var (
	ErrProduction      = errors.New("seed bị tắt ở production")
	ErrResetNotAllowed = errors.New("seed --reset chỉ chạy khi APP_ENV là dev hoặc e2e")
	ErrSeedPassword    = fmt.Errorf("SEED_PASSWORD bắt buộc, tối thiểu %d ký tự", minSeedPasswordLen)
)

const minSeedPasswordLen = 12

// markerEmail là tài khoản admin của dữ liệu mẫu; có tài khoản này nghĩa là đã seed.
const markerEmail = "quan.tran@goup.vn"

// insertOrder là thứ tự chèn theo khóa ngoại, cũng là thứ tự in số bản ghi.
var insertOrder = []string{
	"users", "media_files", "stages", "stage_versions", "stage_units", "lessons", "quiz_settings", "questions",
	"question_options", "homework_settings", "rubric_criteria", "homework_attachments", "lesson_media",
	"courses", "course_versions", "course_version_stages", "classes", "class_members",
	"email_outbox", "invitations", "lesson_progress", "class_lesson_deadlines", "submissions", "submission_files",
	"submission_grades", "grade_criterion_scores", "quiz_attempts", "quiz_attempt_answers", "quiz_attempt_reviews", "quiz_review_notes",
	"stage_reviews", "audit_logs",
}

// Validate kiểm tra môi trường, cờ và SEED_PASSWORD mà không chạm database; Run cũng gọi nó trước khi ghi.
func Validate(cfg config.Config, opts Options) error {
	if cfg.IsProduction() {
		return ErrProduction
	}
	if opts.Reset && !cfg.SeedResetAllowed() {
		return ErrResetNotAllowed
	}
	if utf8.RuneCountInString(cfg.SeedPassword) < minSeedPasswordLen {
		return ErrSeedPassword
	}
	return nil
}

// Run nạp dữ liệu mẫu trong một transaction. Idempotent: nếu đã có dữ liệu mẫu thì trả AlreadySeeded và không
// ghi gì; với Reset thì xóa sạch dữ liệu rồi seed lại. UploadSample tải video mẫu sau khi commit (cả khi đã seed).
// Không ghi email hay mật khẩu ra log.
func Run(ctx context.Context, dbx *sqlx.DB, cfg config.Config, clk clock.Clock, opts Options) (Result, error) {
	if err := Validate(cfg, opts); err != nil {
		return Result{}, err
	}
	if err := checkData(); err != nil {
		return Result{}, err
	}
	var up storage.Uploader
	if opts.UploadSample {
		var err error
		if up, err = newSampleUploader(storage.ConfigFrom(cfg)); err != nil {
			return Result{}, err
		}
	}

	tx, err := dbx.BeginTxx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("seed: mở transaction: %w", err)
	}
	res, err := seedTx(ctx, tx, cfg, clk, opts)
	if err != nil {
		_ = tx.Rollback()
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("seed: commit: %w", err)
	}
	if up != nil {
		if err := uploadSample(ctx, dbx, up); err != nil {
			return Result{}, err
		}
	}
	return res, nil
}

func seedTx(ctx context.Context, tx *sqlx.Tx, cfg config.Config, clk clock.Clock, opts Options) (Result, error) {
	// Hai lệnh seed chạy cùng lúc thì lệnh sau chờ lệnh trước commit rồi thấy dữ liệu đã có.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('lms.seed'))`); err != nil {
		return Result{}, fmt.Errorf("seed: khóa: %w", err)
	}
	if opts.Reset {
		if err := truncateAll(ctx, tx); err != nil {
			return Result{}, err
		}
	} else {
		var exists bool
		if err := tx.GetContext(ctx, &exists, `SELECT EXISTS (SELECT 1 FROM users WHERE email_normalized = $1)`, markerEmail); err != nil {
			return Result{}, fmt.Errorf("seed: kiểm tra dữ liệu cũ: %w", err)
		}
		if exists {
			return Result{AlreadySeeded: true}, nil
		}
	}

	// Hash bằng đúng hàm của tính năng đăng nhập để mọi tài khoản mẫu đăng nhập được bằng SEED_PASSWORD.
	hash, err := identity.NewPasswordHash(cfg.SeedPassword)
	if err != nil {
		return Result{}, fmt.Errorf("seed: hash mật khẩu: %w", err)
	}
	box, err := mailer.NewSecretBox(cfg.OutboxSecretKey)
	if err != nil {
		return Result{}, fmt.Errorf("seed: %w", err)
	}
	s := newSeeder(tx, cfg, clk.Now(), hash.PHC())
	s.box, s.tempPassword = box, cfg.SeedPassword
	steps := []func(context.Context) error{
		s.insertUsers, s.insertStages, s.insertCourses, s.insertClasses, s.insertEnrollments, s.insertHomeworkWork, s.insertQuizWork,
		s.insertStageReviews, s.insertAudits,
	}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			return Result{}, err
		}
	}
	counts := make([]TableCount, 0, len(insertOrder))
	for _, table := range insertOrder {
		counts = append(counts, TableCount{Table: table, Rows: s.counts[table]})
	}
	return Result{Counts: counts}, nil
}

// truncateAll xóa dữ liệu mọi bảng của schema public trừ schema_migrations trong một câu TRUNCATE (CASCADE nên
// thứ tự khóa ngoại không quan trọng).
func truncateAll(ctx context.Context, tx *sqlx.Tx) error {
	var tables []string
	if err := tx.SelectContext(ctx, &tables,
		`SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'schema_migrations' ORDER BY tablename`); err != nil {
		return fmt.Errorf("seed: liệt kê bảng: %w", err)
	}
	if len(tables) == 0 {
		return errors.New("seed: schema trống, cần chạy lms migrate up trước")
	}
	quoted := make([]string, len(tables))
	for i, name := range tables {
		quoted[i] = pgx.Identifier{"public", name}.Sanitize()
	}
	if _, err := tx.ExecContext(ctx, "TRUNCATE "+strings.Join(quoted, ", ")+" RESTART IDENTITY CASCADE"); err != nil {
		return fmt.Errorf("seed: truncate: %w", err)
	}
	return nil
}

type seeder struct {
	tx       *sqlx.Tx
	now      time.Time
	loc      *time.Location
	hash     string
	loginURL string
	tempTTL  time.Duration
	// box niêm phong mật khẩu tạm (= SEED_PASSWORD) vào secret_enc của hàng outbox queued để worker gửi được.
	box          *secretbox.Box
	tempPassword string

	ids      map[string]uuid.UUID           // khóa trong data.go → id thật
	media    map[string]uuid.UUID           // lesson key → media_files.id của bài video
	docs     map[string]uuid.UUID           // tên file kèm homework → media_files.id (kind document)
	members  map[string]uuid.UUID           // memberKey → class_members.id
	criteria map[uuid.UUID][]uuid.UUID      // lesson id của homework → id tiêu chí theo thứ tự
	quizzes  map[uuid.UUID][]seededQuestion // lesson id của quiz → câu hỏi theo thứ tự soạn
	counts   map[string]int
}

// seededQuestion là câu hỏi quiz đã chèn: phương án theo thứ tự soạn, đáp án đúng và phương án sai đầu tiên.
type seededQuestion struct {
	id, firstWrong   uuid.UUID
	options, correct []uuid.UUID
	points           int
}

func newSeeder(tx *sqlx.Tx, cfg config.Config, now time.Time, hash string) *seeder {
	loc := cfg.Location
	if loc == nil {
		loc = time.UTC
	}
	s := &seeder{
		tx: tx, now: now, loc: loc, hash: hash,
		loginURL: cfg.PublicBaseURL + "/login",
		tempTTL:  cfg.TempPasswordTTL,
		ids:      map[string]uuid.UUID{},
		media:    map[string]uuid.UUID{},
		docs:     map[string]uuid.UUID{},
		members:  map[string]uuid.UUID{},
		criteria: map[uuid.UUID][]uuid.UUID{},
		quizzes:  map[uuid.UUID][]seededQuestion{},
		counts:   map[string]int{},
	}
	for _, u := range users {
		s.ids[u.key] = ids.New()
	}
	for _, st := range stages {
		s.ids[st.key] = ids.New()
	}
	for _, sv := range stageVersions {
		s.ids[sv.key] = ids.New()
		for _, u := range sv.units {
			s.ids[u.key] = ids.New()
		}
		for _, l := range sv.lessons {
			s.ids[l.key] = ids.New()
			if l.typ == "video" {
				s.media[l.key] = ids.New()
			}
			if l.homework != nil {
				for _, name := range l.homework.attachments {
					s.docs[name] = ids.New()
				}
			}
		}
	}
	for _, d := range stageDrafts {
		s.ids[d.key] = ids.New()
		from := stageVersionByKey(d.from)
		for _, u := range from.units {
			s.ids[draftKey(d, u.key)] = ids.New()
		}
		for _, l := range from.lessons {
			s.ids[draftKey(d, l.key)] = ids.New()
		}
	}
	for _, c := range courses {
		s.ids[c.key] = ids.New()
	}
	for _, cv := range courseVersions {
		s.ids[cv.key] = ids.New()
	}
	for _, c := range classes {
		s.ids[c.key] = ids.New()
	}
	for _, e := range enrollments {
		s.members[memberKey(e.class, e.user)] = ids.New()
	}
	return s
}

func memberKey(class, user string) string { return class + "/" + user }

// draftKey là khóa nội bộ của bài học/học liệu trong bản nháp; unit_key và lesson_key vẫn giữ khóa của bản gốc.
func draftKey(d stageDraftSeed, key string) string { return d.key + "/" + key }

// stageVersionByKey trả phiên bản published theo khóa; checkData đảm bảo khóa nguồn của bản nháp tồn tại.
func stageVersionByKey(key string) stageVersionSeed {
	for _, sv := range stageVersions {
		if sv.key == key {
			return sv
		}
	}
	return stageVersionSeed{}
}

func (s *seeder) at(o offset) time.Time { return o.at(s.now) }

func (s *seeder) atPtr(o *offset) *time.Time {
	if o == nil {
		return nil
	}
	t := s.at(*o)
	return &t
}

func daysAgo(days float64) offset { return ago(days, 0) }

func (s *seeder) exec(ctx context.Context, table, query string, args ...any) error {
	if _, err := s.tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("seed: chèn %s: %w", table, err)
	}
	s.counts[table]++
	return nil
}

// userCreatedAt: nhân sự tạo trước mọi dữ liệu; học viên tạo lúc nhận lời mời đầu tiên.
func userCreatedAt(u userSeed) offset {
	if u.role != "student" {
		return staffCreatedAt
	}
	first := 0.0
	for _, e := range enrollments {
		if e.user == u.key && e.invitedDays > first {
			first = e.invitedDays
		}
	}
	return daysAgo(first)
}

func (s *seeder) insertUsers(ctx context.Context) error {
	for _, u := range users {
		created := s.at(userCreatedAt(u))
		updated := created
		var disabled *time.Time
		if u.status == "disabled" {
			t := s.at(disabledAt)
			disabled, updated = &t, t
		}
		err := s.exec(ctx, "users", `
			INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash, must_change_password,
			                   temp_password_expires_at, last_login_at, last_active_at, disabled_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			s.ids[u.key], u.email, normalizeEmail(u.email), u.name, u.role, u.status, s.hash, u.mustChangePassword,
			s.atPtr(u.tempPasswordExpiresAt), s.atPtr(u.lastLoginAt), s.atPtr(u.lastActiveAt), disabled, created, updated)
		if err != nil {
			return err
		}
	}
	return nil
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// insertStages chèn media (video và file kèm homework), chặng, phiên bản chặng (published), bài học kèm nội dung
// quiz/homework và lesson_media, rồi bản nháp nhân bản.
func (s *seeder) insertStages(ctx context.Context) error {
	admin := s.ids[adminKey]
	for _, sv := range stageVersions {
		published := s.at(sv.publishedAt)
		for _, l := range sv.lessons {
			switch {
			case l.typ == "video":
				err := s.exec(ctx, "media_files", `
					INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by, created_at, ready_at)
					VALUES ($1, 'video', $2, $3, 'video/mp4', $4, 'ready', $5, $6, $6)`,
					s.media[l.key], "seed/"+l.video, l.video, seedVideoSizeBytes, admin, published)
				if err != nil {
					return err
				}
			case l.homework != nil:
				for _, name := range l.homework.attachments {
					contentType, err := media.DocumentContentType(name)
					if err != nil {
						return fmt.Errorf("seed: file kèm %s: %w", name, err)
					}
					err = s.exec(ctx, "media_files", `
						INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by, created_at, ready_at)
						VALUES ($1, 'document', $2, $3, $4, $5, 'ready', $6, $7, $7)`,
						s.docs[name], "seed/"+name, name, contentType, seedDocumentSizeBytes, admin, published)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	stageNames := map[string]string{}
	for _, st := range stages {
		stageNames[st.key] = st.name
		var created time.Time
		for _, sv := range stageVersions {
			if sv.stage == st.key && (created.IsZero() || s.at(sv.publishedAt).Before(created)) {
				created = s.at(sv.publishedAt)
			}
		}
		err := s.exec(ctx, "stages", `INSERT INTO stages (id, code, name, created_by, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5)`,
			s.ids[st.key], st.code, st.name, admin, created)
		if err != nil {
			return err
		}
	}

	for _, sv := range stageVersions {
		published := s.at(sv.publishedAt)
		err := s.exec(ctx, "stage_versions", `
			INSERT INTO stage_versions (id, stage_id, version_no, status, title, published_at, created_by, created_at, updated_at)
			VALUES ($1, $2, $3, 'published', $4, $5, $6, $5, $5)`,
			s.ids[sv.key], s.ids[sv.stage], sv.no, stageNames[sv.stage], published, admin)
		if err != nil {
			return err
		}
		for i, u := range sv.units {
			err = s.exec(ctx, "stage_units", `
				INSERT INTO stage_units (id, stage_version_id, unit_key, title, position, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $6)`,
				s.ids[u.key], s.ids[sv.key], u.key, u.title, i+1, published)
			if err != nil {
				return err
			}
		}
		for i, l := range sv.lessons {
			if err := s.insertLesson(ctx, s.ids[sv.key], s.ids[l.unit], s.ids[l.key], i+1, l, published); err != nil {
				return err
			}
		}
	}
	for _, d := range stageDrafts {
		if err := s.insertDraft(ctx, d, stageNames); err != nil {
			return err
		}
	}
	return nil
}

// insertDraft chèn bản nháp như Clone của tính năng chặng: header draft trỏ cloned_from_id về bản gốc, bài học và
// học liệu id mới cùng khóa, vị trí, nội dung (video và file kèm dùng chung media), kèm audit stage_version.cloned.
func (s *seeder) insertDraft(ctx context.Context, d stageDraftSeed, stageNames map[string]string) error {
	from, admin, at := stageVersionByKey(d.from), s.ids[adminKey], s.at(d.createdAt)
	err := s.exec(ctx, "stage_versions", `
		INSERT INTO stage_versions (id, stage_id, version_no, status, title, cloned_from_id, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, 'draft', $4, $5, $6, $7, $7)`,
		s.ids[d.key], s.ids[from.stage], d.no, stageNames[from.stage], s.ids[from.key], admin, at)
	if err != nil {
		return err
	}
	for i, u := range from.units {
		err = s.exec(ctx, "stage_units", `
			INSERT INTO stage_units (id, stage_version_id, unit_key, title, position, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)`,
			s.ids[draftKey(d, u.key)], s.ids[d.key], u.key, u.title, i+1, at)
		if err != nil {
			return err
		}
	}
	for i, l := range from.lessons {
		if err := s.insertLesson(ctx, s.ids[d.key], s.ids[draftKey(d, l.unit)], s.ids[draftKey(d, l.key)], i+1, l, at); err != nil {
			return err
		}
	}
	after := map[string]any{"stageId": s.ids[from.stage], "fromVersionNo": from.no, "toVersionNo": d.no}
	return s.insertAudit(ctx, admin, audit.ActionStageVersionCloned, "stage_version", s.ids[d.key], nil, after, at)
}

func (s *seeder) insertLesson(ctx context.Context, versionID, unitID, lessonID uuid.UUID, position int, l lessonSeed, created time.Time) error {
	var (
		source, html *string
		mediaID      *uuid.UUID
		duration     *int
	)
	if l.typ == "video" {
		id := s.media[l.key]
		secs, err := parseDuration(l.duration)
		if err != nil {
			return fmt.Errorf("seed: bài %s: %w", l.key, err)
		}
		mediaID, duration = &id, &secs
	} else if l.markdown != nil {
		source, html = &l.markdown.source, &l.markdown.html
	}
	err := s.exec(ctx, "lessons", `
		INSERT INTO lessons (id, stage_version_id, unit_id, lesson_key, position, title, type, required, markdown_source,
		                     markdown_html, video_media_id, duration_seconds, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)`,
		lessonID, versionID, unitID, l.key, position, l.title, l.typ, l.required, source, html, mediaID, duration, created)
	if err != nil {
		return err
	}
	if mediaID != nil {
		return s.exec(ctx, "lesson_media", `INSERT INTO lesson_media (lesson_id, media_id) VALUES ($1, $2)`, lessonID, *mediaID)
	}
	switch {
	case l.quiz != nil:
		return s.insertQuiz(ctx, lessonID, l.quiz)
	case l.homework != nil:
		return s.insertHomework(ctx, lessonID, l.homework)
	}
	return nil
}

// insertQuiz chèn cấu hình, câu hỏi và phương án; nội dung và giải thích render sẵn như lúc phát hành.
func (s *seeder) insertQuiz(ctx context.Context, lessonID uuid.UUID, q *quizSeed) error {
	var attempts, seconds, percent *int
	if q.mode == "assessment" {
		secs := q.minutes * 60
		attempts, seconds, percent = &q.attempts, &secs, &q.percent
	}
	err := s.exec(ctx, "quiz_settings", `INSERT INTO quiz_settings (lesson_id, mode, max_attempts, time_limit_seconds, shuffle, pass_percent)
		VALUES ($1, $2, $3, $4, true, $5)`, lessonID, q.mode, attempts, seconds, percent)
	if err != nil {
		return err
	}
	for i, qs := range q.questions {
		body, explanation := renderDoc(qs.body), renderDoc(qs.explanation)
		questionID := ids.New()
		err := s.exec(ctx, "questions", `INSERT INTO questions (id, lesson_id, question_key, type, body_source, body_html,
			explanation_source, explanation_html, points, position) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, $9)`,
			questionID, lessonID, qs.key, qs.typ, body.source, body.html, explanation.source, explanation.html, i+1)
		if err != nil {
			return err
		}
		seeded := seededQuestion{id: questionID, points: 1}
		for j, o := range qs.options {
			optionID := ids.New()
			err := s.exec(ctx, "question_options", `INSERT INTO question_options (id, question_id, body, is_correct, position)
				VALUES ($1, $2, $3, $4, $5)`, optionID, questionID, o.body, o.correct, j+1)
			if err != nil {
				return err
			}
			seeded.options = append(seeded.options, optionID)
			switch {
			case o.correct:
				seeded.correct = append(seeded.correct, optionID)
			case seeded.firstWrong == uuid.Nil:
				seeded.firstWrong = optionID
			}
		}
		s.quizzes[lessonID] = append(s.quizzes[lessonID], seeded)
	}
	return nil
}

// insertHomework chèn đề bài (HTML render sẵn), rubric và file kèm; file kèm cũng vào lesson_media để học viên và
// giảng viên của lớp tải được.
func (s *seeder) insertHomework(ctx context.Context, lessonID uuid.UUID, hw *homeworkSeed) error {
	err := s.exec(ctx, "homework_settings", `INSERT INTO homework_settings (lesson_id, brief_source, brief_html, pass_percent)
		VALUES ($1, $2, $3, $4)`, lessonID, hw.brief.source, hw.brief.html, hw.percent)
	if err != nil {
		return err
	}
	for i, c := range hw.criteria {
		id := ids.New()
		err := s.exec(ctx, "rubric_criteria", `INSERT INTO rubric_criteria (id, lesson_id, description, max_points, position)
			VALUES ($1, $2, $3, $4, $5)`, id, lessonID, c.description, c.maxPoints, i+1)
		if err != nil {
			return err
		}
		s.criteria[lessonID] = append(s.criteria[lessonID], id)
	}
	for i, name := range hw.attachments {
		err := s.exec(ctx, "homework_attachments", `INSERT INTO homework_attachments (lesson_id, media_id, position) VALUES ($1, $2, $3)`,
			lessonID, s.docs[name], i+1)
		if err == nil {
			err = s.exec(ctx, "lesson_media", `INSERT INTO lesson_media (lesson_id, media_id) VALUES ($1, $2)`, lessonID, s.docs[name])
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// parseDuration đổi "mm:ss" (hoặc "h:mm:ss") của seed.js thành số giây.
func parseDuration(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("thời lượng %q không đúng dạng mm:ss", s)
	}
	total := 0
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || (i > 0 && (len(p) != 2 || n > 59)) {
			return 0, fmt.Errorf("thời lượng %q không đúng dạng mm:ss", s)
		}
		total = total*60 + n
	}
	return total, nil
}

func (s *seeder) insertCourses(ctx context.Context) error {
	admin := s.ids[adminKey]
	stageOf := map[string]string{}
	for _, sv := range stageVersions {
		stageOf[sv.key] = sv.stage
	}
	courseNames := map[string]string{}
	for _, c := range courses {
		courseNames[c.key] = c.name
		var created time.Time
		for _, cv := range courseVersions {
			if cv.course == c.key && (created.IsZero() || s.at(cv.publishedAt).Before(created)) {
				created = s.at(cv.publishedAt)
			}
		}
		err := s.exec(ctx, "courses", `INSERT INTO courses (id, code, name, created_by, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5)`,
			s.ids[c.key], c.code, c.name, admin, created)
		if err != nil {
			return err
		}
	}
	for _, cv := range courseVersions {
		published := s.at(cv.publishedAt)
		err := s.exec(ctx, "course_versions", `
			INSERT INTO course_versions (id, course_id, version_no, status, title, published_at, created_by, created_at, updated_at)
			VALUES ($1, $2, $3, 'published', $4, $5, $6, $5, $5)`,
			s.ids[cv.key], s.ids[cv.course], cv.no, courseNames[cv.course], published, admin)
		if err != nil {
			return err
		}
		for i, svKey := range cv.stageVersions {
			err := s.exec(ctx, "course_version_stages", `
				INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position) VALUES ($1, $2, $3, $4)`,
				s.ids[cv.key], s.ids[svKey], s.ids[stageOf[svKey]], i+1)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// date đổi mốc tương đối thành ngày lịch theo múi giờ ứng dụng.
func (s *seeder) date(o offset) string { return s.at(o).In(s.loc).Format(time.DateOnly) }

func (s *seeder) insertClasses(ctx context.Context) error {
	admin := s.ids[adminKey]
	for _, c := range classes {
		err := s.exec(ctx, "classes", `
			INSERT INTO classes (id, code, name, course_version_id, teacher_id, status, start_date, end_date, created_by, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
			s.ids[c.key], c.code, c.name, s.ids[c.courseVersion], s.ids[c.teacher], c.status,
			s.date(c.startDate), s.date(c.endDate), admin, s.at(c.createdAt))
		if err != nil {
			return err
		}
	}
	return nil
}

// insertEnrollments chèn thành viên lớp, email outbox + lời mời, rồi tiến độ học.
func (s *seeder) insertEnrollments(ctx context.Context) error {
	userByKey := map[string]userSeed{}
	for _, u := range users {
		userByKey[u.key] = u
	}
	classByKey := map[string]classSeed{}
	for _, c := range classes {
		classByKey[c.key] = c
	}

	for _, e := range enrollments {
		joined := s.at(daysAgo(e.invitedDays))
		var dropped *time.Time
		if e.memberStatus == "dropped" {
			t := s.at(droppedAt)
			dropped = &t
		}
		err := s.exec(ctx, "class_members", `
			INSERT INTO class_members (id, class_id, user_id, status, joined_at, dropped_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			s.members[memberKey(e.class, e.user)], s.ids[e.class], s.ids[e.user], e.memberStatus, joined, dropped)
		if err != nil {
			return err
		}
	}

	for _, e := range enrollments {
		if err := s.insertInvitation(ctx, e, userByKey[e.user], classByKey[e.class]); err != nil {
			return err
		}
	}

	for _, e := range enrollments {
		if err := s.insertProgress(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// insertInvitation chèn hàng email_outbox (template invite, không bí mật trong payload) và lời mời. Trạng thái
// outbox theo hợp đồng worker: sent có sent_at; failed đã thử đủ 3 lần; queued chưa thử lần nào. Chỉ hàng queued
// có secret_enc (mật khẩu tạm đã niêm phong); hàng sent/failed đã xóa bí mật như worker làm.
func (s *seeder) insertInvitation(ctx context.Context, e enrollment, u userSeed, c classSeed) error {
	invitedAt := s.at(daysAgo(e.invitedDays))
	expires := invitedAt.Add(s.tempTTL)
	if u.tempPasswordExpiresAt != nil {
		expires = s.at(*u.tempPasswordExpiresAt)
	}
	payload, err := json.Marshal(map[string]any{
		"Name":      u.name,
		"ClassName": c.name,
		"ClassCode": c.code,
		"LoginURL":  s.loginURL,
		"Email":     u.email,
		"ExpiresAt": expires.In(s.loc).Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("seed: payload outbox: %w", err)
	}

	var (
		attempts  int
		lastError *string
		sentAt    *time.Time
		secretEnc []byte
	)
	switch e.invite {
	case inviteSent:
		sentAt = &invitedAt
	case inviteFailed:
		msg := failedInviteError
		attempts, lastError = 3, &msg
	case inviteQueued:
		if secretEnc, err = s.box.Seal([]byte(s.tempPassword)); err != nil {
			return fmt.Errorf("seed: niêm phong mật khẩu tạm: %w", err)
		}
	}

	outboxID := ids.New()
	err = s.exec(ctx, "email_outbox", `
		INSERT INTO email_outbox (id, to_email, template, payload, secret_enc, status, attempts, run_at, last_error, sent_at, created_at)
		VALUES ($1, $2, 'invite', $3, $4, $5, $6, $7, $8, $9, $7)`,
		outboxID, u.email, string(payload), secretEnc, e.invite, attempts, invitedAt, lastError, sentAt)
	if err != nil {
		return err
	}
	return s.exec(ctx, "invitations", `
		INSERT INTO invitations (id, class_id, user_id, kind, email_outbox_id, invited_by, created_at)
		VALUES ($1, $2, $3, 'invite', $4, $5, $6)`,
		ids.New(), s.ids[e.class], s.ids[e.user], outboxID, s.ids[adminKey], invitedAt)
}

// insertProgress chép done()/opened() của seed.js: bài thứ i trong n bài hoàn thành lúc
// (doneLastDays + (n-i)*1.3) ngày trước; bài chỉ mở có first_opened_at, completed_at NULL.
func (s *seeder) insertProgress(ctx context.Context, e enrollment) error {
	member := s.members[memberKey(e.class, e.user)]
	const q = `INSERT INTO lesson_progress (class_member_id, lesson_id, first_opened_at, completed_at, updated_at) VALUES ($1, $2, $3, $4, $3)`
	for i, key := range e.done {
		t := s.at(daysAgo(e.doneLastDays + float64(len(e.done)-i)*doneStepDays))
		if err := s.exec(ctx, "lesson_progress", q, member, s.ids[key], t, t); err != nil {
			return err
		}
	}
	for _, key := range e.opened {
		if err := s.exec(ctx, "lesson_progress", q, member, s.ids[key], s.at(daysAgo(e.openedDays)), nil); err != nil {
			return err
		}
	}
	return nil
}

// insertHomeworkWork chèn hạn nộp, bài nộp kèm file, các lần chấm và audit tương ứng như service homework ghi, rồi
// tiến độ học liệu homework: completed_at đặt ở lần chấm Đạt đầu tiên, xóa khi bị chấm Cần làm lại.
func (s *seeder) insertHomeworkWork(ctx context.Context) error {
	lessons := map[string]lessonSeed{}
	for _, sv := range stageVersions {
		for _, l := range sv.lessons {
			lessons[l.key] = l
		}
	}
	classByKey := map[string]classSeed{}
	for _, c := range classes {
		classByKey[c.key] = c
	}
	userByKey := map[string]userSeed{}
	for _, u := range users {
		userByKey[u.key] = u
	}

	dueOf := map[string]time.Time{} // lớp/học liệu → hạn nộp
	for _, d := range deadlines {
		dueAt, setAt := s.at(d.due), s.at(daysAgo(d.setDays))
		err := s.exec(ctx, "class_lesson_deadlines", `
			INSERT INTO class_lesson_deadlines (class_id, lesson_id, due_at, set_by, updated_at) VALUES ($1, $2, $3, $4, $5)`,
			s.ids[d.class], s.ids[d.lesson], dueAt, s.ids[d.setBy], setAt)
		if err != nil {
			return err
		}
		payload := func(due any) map[string]any {
			return map[string]any{"classCode": classByKey[d.class].code, "lessonId": s.ids[d.lesson], "lessonTitle": lessons[d.lesson].title, "dueAt": due}
		}
		if err := s.insertAudit(ctx, s.ids[d.setBy], audit.ActionClassDeadlineChanged, "class", s.ids[d.class], payload(nil), payload(dueAt.UTC()), setAt); err != nil {
			return err
		}
		dueOf[d.class+"/"+d.lesson] = dueAt
	}

	lastAttempt := map[string]int{} // thành viên/học liệu → số lần nộp
	for _, sub := range submissions {
		k := memberKey(sub.class, sub.user) + "/" + sub.lesson
		lastAttempt[k] = max(lastAttempt[k], sub.no)
	}
	type progress struct {
		member, lesson  uuid.UUID
		opened, updated time.Time
		completed       *time.Time
	}
	var progressRows []*progress
	progressOf := map[string]*progress{}

	for _, sub := range submissions {
		k := memberKey(sub.class, sub.user) + "/" + sub.lesson
		member, lessonID, student := s.members[memberKey(sub.class, sub.user)], s.ids[sub.lesson], s.ids[sub.user]
		hw := lessons[sub.lesson].homework
		submittedAt := s.at(daysAgo(sub.days))

		var results []homeworkfeature.Result
		for _, g := range sub.grades {
			total := 0
			for _, p := range g.points {
				total += p
			}
			results = append(results, homeworkfeature.ResultOf(total, maxPointsOf(hw), hw.percent))
		}
		status := "pending"
		switch {
		case len(results) > 0:
			status = string(results[len(results)-1])
		case sub.no < lastAttempt[k]:
			status = "superseded"
		}
		due, hasDue := dueOf[sub.class+"/"+sub.lesson]
		late := hasDue && submittedAt.After(due)

		submissionID := ids.New()
		err := s.exec(ctx, "submissions", `
			INSERT INTO submissions (id, class_member_id, lesson_id, attempt_no, repo_url, commit_ref, submitted_at, is_late, status)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9)`,
			submissionID, member, lessonID, sub.no, sub.repoURL, sub.commitRef, submittedAt, late, status)
		if err != nil {
			return err
		}
		for i, f := range sub.files {
			contentType, err := media.DocumentContentType(f.name)
			if err != nil {
				return fmt.Errorf("seed: file nộp %s: %w", f.name, err)
			}
			mediaID := ids.New()
			err = s.exec(ctx, "media_files", `
				INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by, created_at, ready_at)
				VALUES ($1, 'document', $2, $3, $4, $5, 'ready', $6, $7, $7)`,
				mediaID, fmt.Sprintf("seed/submissions/%s/%s", submissionID, f.name), f.name, contentType, f.sizeBytes, student, submittedAt)
			if err == nil {
				err = s.exec(ctx, "submission_files", `INSERT INTO submission_files (submission_id, media_id, position) VALUES ($1, $2, $3)`,
					submissionID, mediaID, i+1)
			}
			if err != nil {
				return err
			}
		}

		p := progressOf[k]
		if p == nil {
			p = &progress{member: member, lesson: lessonID, opened: submittedAt}
			progressOf[k] = p
			progressRows = append(progressRows, p)
		}
		p.updated = submittedAt

		base := func() map[string]any {
			return map[string]any{
				"classCode": classByKey[sub.class].code, "lessonId": lessonID, "lessonTitle": lessons[sub.lesson].title, "memberId": member,
				"studentName": userByKey[sub.user].name, "submissionId": submissionID, "attemptNo": sub.no,
			}
		}
		var before map[string]any
		for i, g := range sub.grades {
			gradeID, gradedAt := ids.New(), s.at(daysAgo(g.days))
			total := 0
			scores := make([]map[string]any, len(g.points))
			for j, pts := range g.points {
				total += pts
				scores[j] = map[string]any{"criterionId": s.criteria[lessonID][j], "points": pts}
			}
			err := s.exec(ctx, "submission_grades", `
				INSERT INTO submission_grades (id, submission_id, graded_by, total_points, max_points, result, comment, graded_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				gradeID, submissionID, s.ids[g.by], total, maxPointsOf(hw), results[i], g.comment, gradedAt)
			if err != nil {
				return err
			}
			for j, pts := range g.points {
				note := ""
				if j < len(g.notes) {
					note = g.notes[j]
				}
				err := s.exec(ctx, "grade_criterion_scores", `
					INSERT INTO grade_criterion_scores (grade_id, criterion_id, points, comment) VALUES ($1, $2, $3, $4)`,
					gradeID, s.criteria[lessonID][j], pts, note)
				if err != nil {
					return err
				}
			}
			after := base()
			after["totalPoints"], after["maxPoints"], after["result"], after["scores"] = total, maxPointsOf(hw), results[i], scores
			action := audit.ActionHomeworkGraded
			if before != nil {
				action = audit.ActionHomeworkGradeUpdated
			}
			if err := s.insertAudit(ctx, s.ids[g.by], action, "class", s.ids[sub.class], before, after, gradedAt); err != nil {
				return err
			}
			before = after

			switch {
			case results[i] == homeworkfeature.ResultRework:
				p.completed = nil
			case p.completed == nil:
				p.completed = &gradedAt
			}
			p.updated = gradedAt
		}
	}

	for _, p := range progressRows {
		err := s.exec(ctx, "lesson_progress", `
			INSERT INTO lesson_progress (class_member_id, lesson_id, first_opened_at, completed_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
			p.member, p.lesson, p.opened, p.completed, p.updated)
		if err != nil {
			return err
		}
	}
	return nil
}

// insertQuizWork chèn lượt làm quiz như service quizzes ghi: câu trả lời chấm lúc nộp, trạng thái xác nhận, nhận xét
// của giảng viên kèm audit, rồi tiến độ học liệu quiz: mở lúc bắt đầu lượt đầu, hoàn thành ở lần nộp đầu (practice)
// hoặc lượt Đạt đầu tiên (assessment) và không bị xóa. Xác nhận không đổi điểm hay tiến độ.
func (s *seeder) insertQuizWork(ctx context.Context) error {
	lessons := map[string]lessonSeed{}
	for _, sv := range stageVersions {
		for _, l := range sv.lessons {
			if l.quiz != nil {
				lessons[l.key] = l
			}
		}
	}
	classByKey := map[string]classSeed{}
	for _, c := range classes {
		classByKey[c.key] = c
	}
	userByKey := map[string]userSeed{}
	for _, u := range users {
		userByKey[u.key] = u
	}
	lastSubmitted := map[string]int{} // thành viên/học liệu → lượt nộp sau cùng
	for _, a := range quizAttempts {
		if !a.open {
			k := memberKey(a.class, a.user) + "/" + a.lesson
			lastSubmitted[k] = max(lastSubmitted[k], a.no)
		}
	}
	type progress struct {
		member, lesson  uuid.UUID
		opened, updated time.Time
		completed       *time.Time
	}
	var progressRows []*progress
	progressOf := map[string]*progress{}

	for _, a := range quizAttempts {
		k := memberKey(a.class, a.user) + "/" + a.lesson
		member, lessonID, q := s.members[memberKey(a.class, a.user)], s.ids[a.lesson], lessons[a.lesson].quiz
		questions := s.quizzes[lessonID]
		startedAt := s.at(daysAgo(a.days))
		spent := time.Duration(a.minutes * float64(time.Minute))

		content := stagefeature.QuizContent{Mode: stagefeature.QuizMode(q.mode)}
		var deadline *time.Time
		if q.mode == "assessment" {
			d := startedAt.Add(time.Duration(q.minutes) * time.Minute)
			deadline, content.PassPercent = &d, &q.percent
		}
		questionOrder := make([]uuid.UUID, len(questions))
		optionOrder := make(map[uuid.UUID][]uuid.UUID, len(questions))
		maxScore := 0
		for i, qs := range questions {
			questionOrder[i], optionOrder[qs.id] = qs.id, qs.options
			maxScore += qs.points
		}
		order, err := json.Marshal(optionOrder)
		if err != nil {
			return fmt.Errorf("seed: option_order: %w", err)
		}

		var (
			submittedAt *time.Time
			score       *int
			passed      *bool
			review      *string
		)
		if !a.open {
			at, total := startedAt.Add(spent), 0
			for i, right := range a.mask {
				if right == 1 {
					total += questions[i].points
				}
			}
			submittedAt, score, passed = &at, &total, quizzes.Passed(content, total, maxScore)
			status := ""
			switch {
			case a.review != nil:
				status = string(quizzes.ReviewConfirmed)
			case q.mode != "assessment":
			case a.no < lastSubmitted[k]:
				status = string(quizzes.ReviewSuperseded)
			default:
				status = string(quizzes.ReviewPending)
			}
			if status != "" {
				review = &status
			}
		}

		attemptID := ids.New()
		err = s.exec(ctx, "quiz_attempts", `
			INSERT INTO quiz_attempts (id, class_member_id, lesson_id, attempt_no, question_order, option_order, started_at,
			                           deadline_at, submitted_at, auto_submitted, score, max_score, passed, review_status)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, $11, $12, $13, $14)`,
			attemptID, member, lessonID, a.no, pgtype.FlatArray[uuid.UUID](questionOrder), string(order), startedAt,
			deadline, submittedAt, a.auto, score, maxScore, passed, review)
		if err != nil {
			return err
		}
		for i, right := range a.mask {
			qs := questions[i]
			selected := []uuid.UUID{qs.firstWrong}
			if right == 1 {
				selected = qs.correct
			}
			savedAt := startedAt.Add(spent * time.Duration(i+1) / time.Duration(len(a.mask)))
			var correct *bool
			var points *int
			if !a.open {
				ok, pts := right == 1, 0
				if ok {
					pts = qs.points
				}
				correct, points = &ok, &pts
			}
			err := s.exec(ctx, "quiz_attempt_answers", `
				INSERT INTO quiz_attempt_answers (attempt_id, question_id, selected_option_ids, is_correct, points_awarded, saved_at)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				attemptID, qs.id, pgtype.FlatArray[uuid.UUID](selected), correct, points, savedAt)
			if err != nil {
				return err
			}
		}
		if r := a.review; r != nil {
			after := map[string]any{
				"classCode": classByKey[a.class].code, "lessonId": lessonID, "lessonTitle": lessons[a.lesson].title,
				"memberId": member, "studentName": userByKey[a.user].name, "attemptId": attemptID, "attemptNo": a.no,
			}
			if err := s.insertQuizReview(ctx, attemptID, q.mode, questions, s.ids[a.class], after, r); err != nil {
				return err
			}
		}

		p := progressOf[k]
		if p == nil {
			p = &progress{member: member, lesson: lessonID, opened: startedAt}
			progressOf[k] = p
			progressRows = append(progressRows, p)
		}
		p.updated = startedAt
		if submittedAt != nil {
			p.updated = *submittedAt
			if p.completed == nil && (passed == nil || *passed) {
				p.completed = submittedAt
			}
		}
	}

	for _, p := range progressRows {
		err := s.exec(ctx, "lesson_progress", `
			INSERT INTO lesson_progress (class_member_id, lesson_id, first_opened_at, completed_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
			p.member, p.lesson, p.opened, p.completed, p.updated)
		if err != nil {
			return err
		}
	}
	return nil
}

// insertQuizReview chèn lần nhận xét của giảng viên (ghi chú theo thứ tự câu) và audit như service quizzes: xác nhận
// với quiz assessment, nhận xét với quiz luyện tập. after là phần định danh lượt của payload audit.
func (s *seeder) insertQuizReview(ctx context.Context, attemptID uuid.UUID, mode string, questions []seededQuestion, classID uuid.UUID, after map[string]any, r *quizReviewSeed) error {
	reviewID, at := ids.New(), s.at(daysAgo(r.days))
	err := s.exec(ctx, "quiz_attempt_reviews", `
		INSERT INTO quiz_attempt_reviews (id, attempt_id, reviewer_id, comment, created_at) VALUES ($1, $2, $3, $4, $5)`,
		reviewID, attemptID, s.ids[r.by], r.comment, at)
	if err != nil {
		return err
	}
	notes := []map[string]any{}
	for i, qs := range questions {
		note, ok := r.notes[i]
		if !ok {
			continue
		}
		err := s.exec(ctx, "quiz_review_notes", `INSERT INTO quiz_review_notes (review_id, question_id, note) VALUES ($1, $2, $3)`,
			reviewID, qs.id, note)
		if err != nil {
			return err
		}
		notes = append(notes, map[string]any{"questionId": qs.id, "note": note})
	}
	after["comment"], after["notes"] = r.comment, notes
	action := audit.ActionQuizAttemptCommented
	if mode == "assessment" {
		action = audit.ActionQuizAttemptConfirmed
	}
	return s.insertAudit(ctx, s.ids[r.by], action, "class", classID, nil, after, at)
}

// insertStageReviews chèn nhận xét tổng kết chặng và audit stage_review.saved với payload như service summary.
func (s *seeder) insertStageReviews(ctx context.Context) error {
	names := map[string]string{}
	for _, u := range users {
		names[u.key] = u.name
	}
	for _, st := range stages {
		names[st.key] = st.name
	}
	stageOf := map[string]string{}
	for _, sv := range stageVersions {
		stageOf[sv.key] = sv.stage
	}
	codes := map[string]string{}
	for _, c := range classes {
		codes[c.key] = c.code
	}
	for _, r := range stageReviews {
		id, member, at := ids.New(), s.members[memberKey(r.class, r.user)], s.at(daysAgo(r.days))
		err := s.exec(ctx, "stage_reviews", `
			INSERT INTO stage_reviews (id, class_member_id, stage_version_id, body, author_id, revision, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 1, $6, $6)`,
			id, member, s.ids[r.stageVersion], r.body, s.ids[r.by], at)
		if err != nil {
			return err
		}
		after := map[string]any{
			"classCode": codes[r.class], "stageVersionId": s.ids[r.stageVersion], "stageName": names[stageOf[r.stageVersion]],
			"memberId": member, "studentName": names[r.user], "body": r.body, "revision": 1, "authorId": s.ids[r.by],
		}
		if err := s.insertAudit(ctx, s.ids[r.by], audit.ActionStageReviewSaved, "class", s.ids[r.class], nil, after, at); err != nil {
			return err
		}
	}
	return nil
}

// maxPointsOf là tổng điểm tối đa của rubric.
func maxPointsOf(hw *homeworkSeed) int {
	total := 0
	for _, c := range hw.criteria {
		total += c.maxPoints
	}
	return total
}

func (s *seeder) insertAudits(ctx context.Context) error {
	userByKey := map[string]userSeed{}
	for _, u := range users {
		userByKey[u.key] = u
	}
	classCodes := map[string]string{}
	for _, c := range classes {
		classCodes[c.key] = c.code
	}

	for _, a := range audits {
		after := map[string]any{}
		for k, v := range a.after {
			after[k] = v
		}
		if a.targetType == "class" {
			after["classId"] = s.ids[a.target]
			after["classCode"] = classCodes[a.target]
		}
		switch len(a.members) {
		case 0:
		case 1:
			after["userId"] = s.ids[a.members[0]]
			after["email"] = userByKey[a.members[0]].email
			after["kind"] = a.invitationKind
		default:
			userIDs := make([]uuid.UUID, len(a.members))
			for i, m := range a.members {
				userIDs[i] = s.ids[m]
			}
			after["userIds"] = userIDs
			after["count"] = len(userIDs)
			after["kind"] = a.invitationKind
		}
		if err := s.insertAudit(ctx, s.ids[adminKey], a.action, a.targetType, s.ids[a.target], a.before, after, s.at(daysAgo(a.days))); err != nil {
			return err
		}
	}
	return nil
}

// insertAudit ghi một dòng audit; before/after rỗng thành NULL.
func (s *seeder) insertAudit(ctx context.Context, actor uuid.UUID, action, targetType string, target uuid.UUID, before, after map[string]any, at time.Time) error {
	beforeJSON, err := jsonOrNil(before)
	if err != nil {
		return err
	}
	afterJSON, err := jsonOrNil(after)
	if err != nil {
		return err
	}
	return s.exec(ctx, "audit_logs", `
		INSERT INTO audit_logs (id, actor_id, action, target_type, target_id, before, after, at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		ids.New(), actor, action, targetType, target, beforeJSON, afterJSON, at)
}

func jsonOrNil(m map[string]any) (any, error) {
	if len(m) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("seed: json audit: %w", err)
	}
	return string(b), nil
}

// checkData kiểm tra mọi khóa tham chiếu trong data.go trỏ đúng chỗ, để lỗi chép dữ liệu báo rõ ràng thay vì
// thành lỗi khóa ngoại khó đọc.
func checkData() error {
	keys := map[string]string{} // khóa → loại
	add := func(kind, key string) error {
		if prev, dup := keys[key]; dup {
			return fmt.Errorf("seed: khóa %q trùng (%s, %s)", key, prev, kind)
		}
		keys[key] = kind
		return nil
	}
	has := func(kind, key string) error {
		if keys[key] != kind {
			return fmt.Errorf("seed: không tìm thấy %s %q", kind, key)
		}
		return nil
	}

	var errs []error
	for _, u := range users {
		errs = append(errs, add("user", u.key))
	}
	errs = append(errs, has("user", adminKey))
	for _, st := range stages {
		errs = append(errs, add("stage", st.key))
	}
	lessonsOf := map[string][]string{} // stage version → lesson keys
	homeworkOf := map[string]*homeworkSeed{}
	quizOf := map[string]*quizSeed{}
	for _, sv := range stageVersions {
		errs = append(errs, add("stage version", sv.key), has("stage", sv.stage))
		if len(sv.units) == 0 {
			errs = append(errs, fmt.Errorf("seed: phiên bản %q chưa có bài học", sv.key))
		}
		unitAt := map[string]int{} // khóa bài học → vị trí trong phiên bản
		for i, u := range sv.units {
			errs = append(errs, add("unit", u.key))
			unitAt[u.key] = i + 1
		}
		filled, last := map[string]bool{}, 0
		for _, l := range sv.lessons {
			errs = append(errs, add("lesson", l.key))
			lessonsOf[sv.key] = append(lessonsOf[sv.key], l.key)
			if l.homework != nil {
				homeworkOf[l.key] = l.homework
			}
			if l.quiz != nil {
				quizOf[l.key] = l.quiz
			}
			errs = append(errs, checkContent(l))
			// lessons.position là thứ tự phẳng theo (bài học, thứ tự trong bài học) nên học liệu phải xếp theo bài học.
			at, ok := unitAt[l.unit]
			switch {
			case !ok:
				errs = append(errs, fmt.Errorf("seed: học liệu %q thuộc bài học %q không có trong %s", l.key, l.unit, sv.key))
			case at < last:
				errs = append(errs, fmt.Errorf("seed: học liệu %q của %s không xếp theo thứ tự bài học", l.key, sv.key))
			default:
				filled[l.unit], last = true, at
			}
		}
		for _, u := range sv.units {
			if !filled[u.key] {
				errs = append(errs, fmt.Errorf("seed: bài học %q của %s chưa có học liệu", u.key, sv.key))
			}
		}
	}
	errs = append(errs, checkDrafts(add)...)
	for _, c := range courses {
		errs = append(errs, add("course", c.key))
	}
	lessonsOfCV := map[string]map[string]bool{}
	for _, cv := range courseVersions {
		errs = append(errs, add("course version", cv.key), has("course", cv.course))
		lessonsOfCV[cv.key] = map[string]bool{}
		for _, svKey := range cv.stageVersions {
			errs = append(errs, has("stage version", svKey))
			for _, l := range lessonsOf[svKey] {
				lessonsOfCV[cv.key][l] = true
			}
		}
	}
	courseVersionOf, teacherOf, statusOf := map[string]string{}, map[string]string{}, map[string]string{}
	for _, c := range classes {
		errs = append(errs, add("class", c.key), has("course version", c.courseVersion), has("user", c.teacher))
		courseVersionOf[c.key], teacherOf[c.key], statusOf[c.key] = c.courseVersion, c.teacher, c.status
	}
	enrolled := map[string]enrollment{}
	for _, e := range enrollments {
		errs = append(errs, has("class", e.class), has("user", e.user))
		mk := memberKey(e.class, e.user)
		if _, dup := enrolled[mk]; dup {
			errs = append(errs, fmt.Errorf("seed: %s enroll hai lần", mk))
		}
		enrolled[mk] = e
		for _, l := range append(append([]string{}, e.done...), e.opened...) {
			if !lessonsOfCV[courseVersionOf[e.class]][l] {
				errs = append(errs, fmt.Errorf("seed: bài %q không thuộc khóa học của lớp %s", l, e.class))
			}
		}
	}
	// classHomework kiểm học liệu là homework thuộc khóa học của lớp.
	classHomework := func(class, lesson string) error {
		if homeworkOf[lesson] == nil || !lessonsOfCV[courseVersionOf[class]][lesson] {
			return fmt.Errorf("seed: %q không phải homework của lớp %s", lesson, class)
		}
		return nil
	}
	errs = append(errs, checkHomeworkWork(has, classHomework, homeworkOf, teacherOf, enrolled)...)
	classQuiz := func(class, lesson string) (*quizSeed, error) {
		if quizOf[lesson] == nil || !lessonsOfCV[courseVersionOf[class]][lesson] {
			return nil, fmt.Errorf("seed: %q không phải quiz của lớp %s", lesson, class)
		}
		if statusOf[class] == "draft" {
			return nil, fmt.Errorf("seed: lớp %s còn nháp nên chưa làm quiz được", class)
		}
		return quizOf[lesson], nil
	}
	errs = append(errs, checkQuizWork(has, classQuiz, teacherOf, enrolled)...)
	errs = append(errs, checkStageReviews(has, courseVersionOf, teacherOf, enrolled)...)
	targetKinds := map[string]string{"stage_version": "stage version", "course_version": "course version", "class": "class", "user": "user"}
	for _, a := range audits {
		kind, ok := targetKinds[a.targetType]
		if !ok {
			errs = append(errs, fmt.Errorf("seed: audit %s có target_type lạ %q", a.action, a.targetType))
			continue
		}
		errs = append(errs, has(kind, a.target))
		for _, m := range a.members {
			errs = append(errs, has("user", m))
		}
	}
	return errors.Join(errs...)
}

// checkDrafts kiểm bản nháp theo luật Clone: nguồn là phiên bản đã phát hành, mỗi chặng tối đa một bản nháp, số
// phiên bản kế tiếp số lớn nhất của chặng, nhân bản sau lúc nguồn phát hành và trước thời điểm seed.
func checkDrafts(add func(kind, key string) error) []error {
	var errs []error
	draftOf := map[string]string{} // chặng → bản nháp
	for _, d := range stageDrafts {
		errs = append(errs, add("stage version", d.key))
		from, latest := stageVersionByKey(d.from), 0
		if from.key == "" {
			errs = append(errs, fmt.Errorf("seed: bản nháp %s: không tìm thấy phiên bản đã phát hành %q", d.key, d.from))
			continue
		}
		for _, sv := range stageVersions {
			if sv.stage == from.stage {
				latest = max(latest, sv.no)
			}
		}
		if prev, dup := draftOf[from.stage]; dup {
			errs = append(errs, fmt.Errorf("seed: chặng %s đã có bản nháp %s", from.stage, prev))
		}
		draftOf[from.stage] = d.key
		if d.no != latest+1 {
			errs = append(errs, fmt.Errorf("seed: bản nháp %s phải là v%d", d.key, latest+1))
		}
		if d.createdAt <= from.publishedAt || d.createdAt >= 0 {
			errs = append(errs, fmt.Errorf("seed: bản nháp %s phải nhân bản sau khi %s phát hành và trước thời điểm seed", d.key, d.from))
		}
	}
	return errs
}

// checkStageReviews kiểm nhận xét tổng kết chặng theo luật của service summary: thành viên lớp, chặng thuộc khóa học
// của lớp, người viết là giảng viên lớp hoặc admin, nội dung 1..5000 ký tự sau trim, mỗi (thành viên, chặng) một
// nhận xét, viết sau khi vào lớp và trước thời điểm seed.
func checkStageReviews(has func(kind, key string) error, courseVersionOf, teacherOf map[string]string, enrolled map[string]enrollment) []error {
	stagesOfCV := map[string]map[string]bool{}
	for _, cv := range courseVersions {
		stagesOfCV[cv.key] = map[string]bool{}
		for _, sv := range cv.stageVersions {
			stagesOfCV[cv.key][sv] = true
		}
	}
	var errs []error
	seen := map[string]bool{}
	for _, r := range stageReviews {
		mk := memberKey(r.class, r.user)
		name := fmt.Sprintf("nhận xét chặng %s/%s", mk, r.stageVersion)
		e, ok := enrolled[mk]
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("seed: %s: %s không phải thành viên lớp", name, r.user))
		case r.days <= 0 || r.days >= e.invitedDays:
			errs = append(errs, fmt.Errorf("seed: %s: phải viết sau khi vào lớp và trước thời điểm seed", name))
		}
		if !stagesOfCV[courseVersionOf[r.class]][r.stageVersion] {
			errs = append(errs, fmt.Errorf("seed: %s: chặng không thuộc khóa học của lớp", name))
		}
		if err := has("user", r.by); err != nil {
			errs = append(errs, err)
		} else if r.by != teacherOf[r.class] && r.by != adminKey {
			errs = append(errs, fmt.Errorf("seed: %s: người viết phải là giảng viên lớp hoặc admin", name))
		}
		if body := strings.TrimSpace(r.body); body != r.body || body == "" || utf8.RuneCountInString(body) > 5000 {
			errs = append(errs, fmt.Errorf("seed: %s: nội dung phải trim sẵn, từ 1 đến 5000 ký tự", name))
		}
		if seen[mk+"/"+r.stageVersion] {
			errs = append(errs, fmt.Errorf("seed: %s: trùng", name))
		}
		seen[mk+"/"+r.stageVersion] = true
	}
	return errs
}

// checkHomeworkWork kiểm hạn nộp, bài nộp và lần chấm theo luật của service homework: nộp lần lượt từ lần 1, không
// nộp lại sau khi Đạt, điểm trong rubric, người chấm là giảng viên lớp hoặc admin, mốc thời gian đúng thứ tự.
func checkHomeworkWork(has func(kind, key string) error, classHomework func(class, lesson string) error,
	homeworkOf map[string]*homeworkSeed, teacherOf map[string]string, enrolled map[string]enrollment) []error {
	var errs []error
	seenDeadline := map[string]bool{}
	for _, d := range deadlines {
		errs = append(errs, has("class", d.class), has("user", d.setBy), classHomework(d.class, d.lesson))
		if k := d.class + "/" + d.lesson; seenDeadline[k] {
			errs = append(errs, fmt.Errorf("seed: hạn nộp %s đặt hai lần", k))
		} else {
			seenDeadline[k] = true
		}
	}

	type attempt struct {
		no         int
		days       float64
		lastResult homeworkfeature.Result
		lastGraded float64
	}
	previous := map[string]attempt{} // thành viên/học liệu → lần nộp trước
	for _, sub := range submissions {
		mk := memberKey(sub.class, sub.user)
		name := fmt.Sprintf("bài nộp %s/%s lần %d", mk, sub.lesson, sub.no)
		e, ok := enrolled[mk]
		if !ok {
			errs = append(errs, fmt.Errorf("seed: %s: %s không phải thành viên lớp", name, sub.user))
			continue
		}
		if err := classHomework(sub.class, sub.lesson); err != nil {
			errs = append(errs, err)
			continue
		}
		for _, l := range append(append([]string{}, e.done...), e.opened...) {
			if l == sub.lesson {
				errs = append(errs, fmt.Errorf("seed: %s: tiến độ homework suy ra từ lần chấm, không khai trong done/opened", name))
			}
		}

		k := mk + "/" + sub.lesson
		prev, hasPrev := previous[k]
		switch {
		case sub.no != prev.no+1:
			errs = append(errs, fmt.Errorf("seed: %s: lần nộp phải liên tiếp từ 1", name))
		case hasPrev && sub.days >= prev.days:
			errs = append(errs, fmt.Errorf("seed: %s: phải nộp sau lần trước", name))
		case hasPrev && prev.lastResult == homeworkfeature.ResultPassed:
			errs = append(errs, fmt.Errorf("seed: %s: lần trước đã Đạt nên không nộp lại được", name))
		case hasPrev && prev.lastResult != "" && sub.days >= prev.lastGraded:
			errs = append(errs, fmt.Errorf("seed: %s: lần trước phải chấm xong trước khi nộp lại", name))
		}

		mediaIDs := make([]uuid.UUID, len(sub.files))
		for i, f := range sub.files {
			mediaIDs[i] = uuid.New()
			if _, err := media.DocumentContentType(f.name); err != nil || f.sizeBytes <= 0 {
				errs = append(errs, fmt.Errorf("seed: %s: file %q không hợp lệ", name, f.name))
			}
		}
		if _, err := (homeworkfeature.SubmitInput{RepoURL: sub.repoURL, CommitRef: sub.commitRef, MediaIDs: mediaIDs}).Normalize(); err != nil {
			errs = append(errs, fmt.Errorf("seed: %s: %w", name, err))
		}

		cur := attempt{no: sub.no, days: sub.days}
		hw := homeworkOf[sub.lesson]
		last := sub.days
		for _, g := range sub.grades {
			errs = append(errs, has("user", g.by))
			if g.by != adminKey && g.by != teacherOf[sub.class] {
				errs = append(errs, fmt.Errorf("seed: %s: %s không chấm được lớp %s", name, g.by, sub.class))
			}
			if g.days >= last {
				errs = append(errs, fmt.Errorf("seed: %s: lần chấm phải sau lần nộp và lần chấm trước", name))
			}
			last = g.days
			if strings.TrimSpace(g.comment) == "" {
				errs = append(errs, fmt.Errorf("seed: %s: thiếu nhận xét", name))
			}
			if len(g.points) != len(hw.criteria) || (len(g.notes) != 0 && len(g.notes) != len(hw.criteria)) {
				errs = append(errs, fmt.Errorf("seed: %s: điểm và ghi chú phải đủ %d tiêu chí", name, len(hw.criteria)))
				continue
			}
			total := 0
			for i, p := range g.points {
				if p < 0 || p > hw.criteria[i].maxPoints {
					errs = append(errs, fmt.Errorf("seed: %s: điểm tiêu chí %d ngoài 0–%d", name, i+1, hw.criteria[i].maxPoints))
				}
				total += p
			}
			cur.lastResult, cur.lastGraded = homeworkfeature.ResultOf(total, maxPointsOf(hw), hw.percent), g.days
		}
		previous[k] = cur
	}
	return errs
}

// checkQuizWork kiểm lượt làm quiz theo luật của service quizzes: lượt liên tiếp từ 1 và bắt đầu sau khi lượt trước
// đã nộp, chỉ lượt cuối được đang làm, lượt nộp có đủ câu và nộp trong giờ, assessment không quá số lượt (seed không
// cấp thêm lượt). Nhận xét chỉ cho lượt đã nộp, do giảng viên lớp hoặc admin, sau lúc nộp và (assessment) trước khi
// có lượt nộp mới hơn, theo luật form nhận xét.
func checkQuizWork(has func(kind, key string) error, classQuiz func(class, lesson string) (*quizSeed, error),
	teacherOf map[string]string, enrolled map[string]enrollment) []error {
	var errs []error
	type previous struct {
		no               int
		open             bool
		submittedMinutes float64 // phút trước thời điểm seed
		reviewMinutes    float64 // 0 khi chưa nhận xét
	}
	prevOf := map[string]previous{} // thành viên/học liệu → lượt trước
	for _, a := range quizAttempts {
		mk := memberKey(a.class, a.user)
		name := fmt.Sprintf("lượt quiz %s/%s lần %d", mk, a.lesson, a.no)
		e, ok := enrolled[mk]
		if !ok {
			errs = append(errs, fmt.Errorf("seed: %s: %s không phải thành viên lớp", name, a.user))
			continue
		}
		q, err := classQuiz(a.class, a.lesson)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, l := range append(append([]string{}, e.done...), e.opened...) {
			if l == a.lesson {
				errs = append(errs, fmt.Errorf("seed: %s: tiến độ quiz suy ra từ lượt làm, không khai trong done/opened", name))
			}
		}

		startMinutes := a.days * 24 * 60
		k := mk + "/" + a.lesson
		prev, hasPrev := prevOf[k]
		switch {
		case a.no != prev.no+1:
			errs = append(errs, fmt.Errorf("seed: %s: lượt phải liên tiếp từ 1", name))
		case hasPrev && prev.open:
			errs = append(errs, fmt.Errorf("seed: %s: lượt trước còn đang làm", name))
		case hasPrev && startMinutes >= prev.submittedMinutes:
			errs = append(errs, fmt.Errorf("seed: %s: phải bắt đầu sau khi nộp lượt trước", name))
		case q.mode == "assessment" && a.no > q.attempts:
			errs = append(errs, fmt.Errorf("seed: %s: quá %d lượt", name, q.attempts))
		}
		submittedMinutes := startMinutes - a.minutes
		if hasPrev && q.mode == "assessment" && !a.open && prev.reviewMinutes > 0 && prev.reviewMinutes <= submittedMinutes {
			errs = append(errs, fmt.Errorf("seed: %s: lượt trước phải được xác nhận trước khi nộp lượt này", name))
		}
		cur := previous{no: a.no, open: a.open, submittedMinutes: submittedMinutes}
		if r := a.review; r != nil {
			cur.reviewMinutes = r.days * 24 * 60
			errs = append(errs, checkQuizReview(name, has, teacherOf[a.class], a, q, cur.reviewMinutes)...)
		}
		prevOf[k] = cur

		switch {
		case a.minutes <= 0 || a.minutes > startMinutes:
			errs = append(errs, fmt.Errorf("seed: %s: thời gian làm phải dương và kết thúc trước thời điểm seed", name))
		case !a.open && q.mode == "assessment" && a.minutes > float64(q.minutes):
			errs = append(errs, fmt.Errorf("seed: %s: nộp sau giới hạn %d phút", name, q.minutes))
		case a.open && q.mode == "assessment":
			// Lượt assessment đang làm có hạn tính theo thời điểm seed, dễ hết giờ trước khi được dùng.
			errs = append(errs, fmt.Errorf("seed: %s: dữ liệu mẫu chỉ có lượt đang làm của quiz luyện tập", name))
		case a.auto && (a.open || q.mode != "assessment" || a.minutes != float64(q.minutes)):
			// Hệ thống chỉ tự nộp lượt assessment đã hết giờ, tại đúng hạn của lượt.
			errs = append(errs, fmt.Errorf("seed: %s: lượt tự nộp phải là lượt assessment đã nộp sau đủ %d phút", name, q.minutes))
		}
		if len(a.mask) > len(q.questions) || (!a.open && len(a.mask) != len(q.questions)) {
			errs = append(errs, fmt.Errorf("seed: %s: lượt nộp phải trả lời đủ %d câu", name, len(q.questions)))
		}
		for i, v := range a.mask {
			if v != 0 && v != 1 {
				errs = append(errs, fmt.Errorf("seed: %s: mask câu %d phải là 0 hoặc 1", name, i+1))
			} else if i < len(q.questions) && v == 0 && !hasWrongOption(q.questions[i]) {
				errs = append(errs, fmt.Errorf("seed: %s: câu %d không có phương án sai để chọn", name, i+1))
			}
		}
	}
	return errs
}

// checkQuizReview kiểm một lần nhận xét: người nhận xét là giảng viên lớp hoặc admin, lượt đã nộp trước lúc nhận xét,
// nhận xét chung bắt buộc, ghi chú thuộc câu của quiz và trong giới hạn độ dài.
func checkQuizReview(name string, has func(kind, key string) error, teacher string, a quizAttemptSeed, q *quizSeed, reviewMinutes float64) []error {
	r := a.review
	errs := []error{has("user", r.by)}
	if r.by != adminKey && r.by != teacher {
		errs = append(errs, fmt.Errorf("seed: %s: %s không nhận xét được lớp %s", name, r.by, a.class))
	}
	switch {
	case a.open:
		errs = append(errs, fmt.Errorf("seed: %s: chỉ lượt đã nộp mới nhận xét được", name))
	case reviewMinutes <= 0 || reviewMinutes >= a.days*24*60-a.minutes:
		errs = append(errs, fmt.Errorf("seed: %s: nhận xét phải sau lúc nộp và trước thời điểm seed", name))
	}
	if c := strings.TrimSpace(r.comment); c == "" || utf8.RuneCountInString(c) > 5000 {
		errs = append(errs, fmt.Errorf("seed: %s: nhận xét chung bắt buộc, tối đa 5000 ký tự", name))
	}
	for i, note := range r.notes {
		if i < 0 || i >= len(q.questions) {
			errs = append(errs, fmt.Errorf("seed: %s: ghi chú cho câu %d không thuộc quiz", name, i+1))
		}
		if n := strings.TrimSpace(note); n == "" || utf8.RuneCountInString(n) > 1000 {
			errs = append(errs, fmt.Errorf("seed: %s: ghi chú câu %d phải có nội dung, tối đa 1000 ký tự", name, i+1))
		}
	}
	return errs
}

func hasWrongOption(q questionSeed) bool {
	for _, o := range q.options {
		if !o.correct {
			return true
		}
	}
	return false
}

// checkContent kiểm học liệu có đúng một loại nội dung khớp typ, và quiz/homework theo đúng luật soạn của tính
// năng chặng (cấu hình theo chế độ, 2–6 phương án, đáp án đúng, rubric) để dữ liệu mẫu phát hành được thật.
func checkContent(l lessonSeed) error {
	kinds := map[string]bool{"video": l.video != "", "markdown": l.markdown != nil, "quiz": l.quiz != nil, "homework": l.homework != nil}
	n := 0
	for _, has := range kinds {
		if has {
			n++
		}
	}
	if n != 1 || !kinds[l.typ] {
		return fmt.Errorf("seed: bài %q sai loại nội dung", l.key)
	}
	var errs []error
	if q := l.quiz; q != nil {
		c := stagefeature.QuizContent{Mode: stagefeature.QuizMode(q.mode), Shuffle: true}
		if q.mode == "assessment" {
			limit := time.Duration(q.minutes) * time.Minute
			c.MaxAttempts, c.TimeLimit, c.PassPercent = &q.attempts, &limit, &q.percent
		}
		if err := c.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("seed: quiz %q: %w", l.key, err))
		}
		if len(q.questions) == 0 {
			errs = append(errs, fmt.Errorf("seed: quiz %q chưa có câu hỏi", l.key))
		}
		seen := map[string]bool{}
		for _, qs := range q.questions {
			errs = append(errs, checkQuestion(l.key, qs))
			if seen[qs.key] {
				errs = append(errs, fmt.Errorf("seed: quiz %q trùng mã câu hỏi %q", l.key, qs.key))
			}
			seen[qs.key] = true
		}
	}
	if hw := l.homework; hw != nil {
		if hw.brief == nil || strings.TrimSpace(hw.brief.source) == "" || len(hw.criteria) == 0 || hw.percent < 1 || hw.percent > 100 {
			errs = append(errs, fmt.Errorf("seed: homework %q thiếu đề bài, tiêu chí hoặc ngưỡng đạt", l.key))
		}
		for _, c := range hw.criteria {
			if strings.TrimSpace(c.description) == "" || c.maxPoints < 1 || c.maxPoints > 100 {
				errs = append(errs, fmt.Errorf("seed: homework %q có tiêu chí sai", l.key))
			}
		}
		for _, name := range hw.attachments {
			if _, err := media.DocumentContentType(name); err != nil {
				errs = append(errs, fmt.Errorf("seed: homework %q: file kèm %s: %w", l.key, name, err))
			}
		}
	}
	return errors.Join(errs...)
}

func checkQuestion(lessonKey string, q questionSeed) error {
	if _, err := stagefeature.ParseQuestionKey(q.key); err != nil {
		return fmt.Errorf("seed: quiz %q: mã câu hỏi %q: %w", lessonKey, q.key, err)
	}
	typ, err := stagefeature.ParseQuestionType(q.typ)
	if err != nil || strings.TrimSpace(q.body) == "" || len(q.options) < 2 || len(q.options) > 6 {
		return fmt.Errorf("seed: câu %q của %q sai loại, nội dung hoặc số phương án", q.key, lessonKey)
	}
	correct := 0
	for _, o := range q.options {
		if o.correct {
			correct++
		}
		if n := utf8.RuneCountInString(strings.TrimSpace(o.body)); n < 1 || n > 500 {
			return fmt.Errorf("seed: câu %q của %q có phương án sai độ dài", q.key, lessonKey)
		}
	}
	if correct == 0 || (typ == stagefeature.QuestionSingle && correct != 1) {
		return fmt.Errorf("seed: câu %q của %q sai số đáp án đúng", q.key, lessonKey)
	}
	return nil
}
