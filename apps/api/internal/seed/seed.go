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
	"github.com/jmoiron/sqlx"

	"lms/api/internal/features/identity"
	"lms/api/internal/features/mailer"
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
	"users", "media_files", "stages", "stage_versions", "lessons", "lesson_media",
	"courses", "course_versions", "course_version_stages", "classes", "class_members",
	"email_outbox", "invitations", "lesson_progress", "audit_logs",
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
		s.insertUsers, s.insertStages, s.insertCourses, s.insertClasses, s.insertEnrollments, s.insertAudits,
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

	ids     map[string]uuid.UUID // khóa trong data.go → id thật
	media   map[string]uuid.UUID // lesson key → media_files.id của bài video
	members map[string]uuid.UUID // memberKey → class_members.id
	counts  map[string]int
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
		members:  map[string]uuid.UUID{},
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
		for _, l := range sv.lessons {
			s.ids[l.key] = ids.New()
			if l.typ == "video" {
				s.media[l.key] = ids.New()
			}
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

// insertStages chèn media video, chặng, phiên bản chặng (published), bài học và lesson_media.
func (s *seeder) insertStages(ctx context.Context) error {
	admin := s.ids[adminKey]
	for _, sv := range stageVersions {
		published := s.at(sv.publishedAt)
		for _, l := range sv.lessons {
			if l.typ != "video" {
				continue
			}
			err := s.exec(ctx, "media_files", `
				INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by, created_at, ready_at)
				VALUES ($1, 'video', $2, $3, 'video/mp4', $4, 'ready', $5, $6, $6)`,
				s.media[l.key], "seed/"+l.video, l.video, seedVideoSizeBytes, admin, published)
			if err != nil {
				return err
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
		for i, l := range sv.lessons {
			if err := s.insertLesson(ctx, sv.key, i+1, l, published); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *seeder) insertLesson(ctx context.Context, svKey string, position int, l lessonSeed, published time.Time) error {
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
	} else {
		source, html = &l.markdown.source, &l.markdown.html
	}
	err := s.exec(ctx, "lessons", `
		INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, required, markdown_source, markdown_html,
		                     video_media_id, duration_seconds, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)`,
		s.ids[l.key], s.ids[svKey], l.key, position, l.title, l.typ, l.required, source, html, mediaID, duration, published)
	if err != nil {
		return err
	}
	if mediaID != nil {
		return s.exec(ctx, "lesson_media", `INSERT INTO lesson_media (lesson_id, media_id) VALUES ($1, $2)`, s.ids[l.key], *mediaID)
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
		beforeJSON, err := jsonOrNil(a.before)
		if err != nil {
			return err
		}
		afterJSON, err := jsonOrNil(after)
		if err != nil {
			return err
		}
		err = s.exec(ctx, "audit_logs", `
			INSERT INTO audit_logs (id, actor_id, action, target_type, target_id, before, after, at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			ids.New(), s.ids[adminKey], a.action, a.targetType, s.ids[a.target], beforeJSON, afterJSON, s.at(daysAgo(a.days)))
		if err != nil {
			return err
		}
	}
	return nil
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
	for _, sv := range stageVersions {
		errs = append(errs, add("stage version", sv.key), has("stage", sv.stage))
		for _, l := range sv.lessons {
			errs = append(errs, add("lesson", l.key))
			lessonsOf[sv.key] = append(lessonsOf[sv.key], l.key)
			if (l.typ == "video") != (l.markdown == nil) {
				errs = append(errs, fmt.Errorf("seed: bài %q sai loại nội dung", l.key))
			}
		}
	}
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
	courseVersionOf := map[string]string{}
	for _, c := range classes {
		errs = append(errs, add("class", c.key), has("course version", c.courseVersion), has("user", c.teacher))
		courseVersionOf[c.key] = c.courseVersion
	}
	seen := map[string]bool{}
	for _, e := range enrollments {
		errs = append(errs, has("class", e.class), has("user", e.user))
		mk := memberKey(e.class, e.user)
		if seen[mk] {
			errs = append(errs, fmt.Errorf("seed: %s enroll hai lần", mk))
		}
		seen[mk] = true
		for _, l := range append(append([]string{}, e.done...), e.opened...) {
			if !lessonsOfCV[courseVersionOf[e.class]][l] {
				errs = append(errs, fmt.Errorf("seed: bài %q không thuộc khóa học của lớp %s", l, e.class))
			}
		}
	}
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
