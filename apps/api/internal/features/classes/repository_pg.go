package classes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
)

// PGClassRepo là ClassRepo trên PostgreSQL. Mọi câu đổi trạng thái mang điều kiện status cũ ngay trong SQL và
// chạy qua db.ExecAffectOne.
type PGClassRepo struct{}

// PGMemberRepo là MemberRepo trên PostgreSQL.
type PGMemberRepo struct{}

// PGInvitationRepo là InvitationRepo trên PostgreSQL.
type PGInvitationRepo struct{}

var (
	_ ClassRepo      = PGClassRepo{}
	_ MemberRepo     = PGMemberRepo{}
	_ InvitationRepo = PGInvitationRepo{}
)

// constraintOf trả mã lỗi và tên constraint của lỗi PostgreSQL (rỗng nếu không phải lỗi PostgreSQL).
func constraintOf(err error) (code, constraint string) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code, pg.ConstraintName
	}
	return "", ""
}

// escapeLike thoát ký tự đặc biệt của LIKE để từ khóa tìm kiếm được so khớp nguyên văn.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (PGClassRepo) Create(ctx context.Context, ex db.Executor, c *Class) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO classes (id, code, name, course_version_id, teacher_id, status, start_date, end_date, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
		c.id, c.code.String(), c.name, c.courseVersionID, c.teacherID, c.status.String(),
		c.dates.Start, c.dates.End, c.createdBy, c.createdAt)
	if err != nil {
		switch code, cn := constraintOf(err); {
		case code == pgerrcode.UniqueViolation && cn == pgerr.UqClassesCode:
			return ErrCodeTaken
		case code == pgerrcode.CheckViolation && cn == pgerr.CkClassesDates:
			return ErrInvalidDates
		}
		return fmt.Errorf("classes: tạo lớp: %w", pgerr.Map(err))
	}
	return nil
}

func (PGClassRepo) Update(ctx context.Context, ex db.Executor, c *Class, from domain.ClassStatus, now time.Time) error {
	err := db.ExecAffectOne(ctx, ex,
		`UPDATE classes SET name = $3, course_version_id = $4, teacher_id = $5, status = $6, start_date = $7, end_date = $8, updated_at = $9
		 WHERE id = $1 AND status = $2`,
		c.id, from.String(), c.name, c.courseVersionID, c.teacherID, c.status.String(), c.dates.Start, c.dates.End, now)
	switch code, cn := constraintOf(err); {
	case err == nil:
		return nil
	case errors.Is(err, db.ErrNoRowsAffected):
		return ErrInvalidTransition
	case code == pgerrcode.CheckViolation && cn == pgerr.CkClassesDates:
		return ErrInvalidDates
	default:
		return fmt.Errorf("classes: cập nhật lớp: %w", pgerr.Map(err))
	}
}

const classColumns = `c.id, c.code, c.name, c.course_version_id, c.teacher_id, c.status, c.start_date, c.end_date, c.created_by, c.created_at`

type classRecord struct {
	ID              uuid.UUID `db:"id"`
	Code            string    `db:"code"`
	Name            string    `db:"name"`
	CourseVersionID uuid.UUID `db:"course_version_id"`
	TeacherID       uuid.UUID `db:"teacher_id"`
	Status          string    `db:"status"`
	StartDate       time.Time `db:"start_date"`
	EndDate         time.Time `db:"end_date"`
	CreatedBy       uuid.UUID `db:"created_by"`
	CreatedAt       time.Time `db:"created_at"`
}

func (r classRecord) class() (*Class, error) {
	st, err := domain.ParseClassStatus(r.Status)
	if err != nil {
		return nil, fmt.Errorf("classes: trạng thái lớp %q: %w", r.Status, err)
	}
	return &Class{
		id: r.ID, code: domain.Code(r.Code), name: r.Name, courseVersionID: r.CourseVersionID, status: st,
		dates:     DateRange{Start: truncateDay(r.StartDate), End: truncateDay(r.EndDate)},
		teacherID: r.TeacherID, createdBy: r.CreatedBy, createdAt: r.CreatedAt,
	}, nil
}

func getClass(ctx context.Context, ex db.Executor, q string, id uuid.UUID) (*Class, error) {
	var rec classRecord
	err := sqlx.GetContext(ctx, ex, &rec, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrClassNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("classes: đọc lớp: %w", err)
	}
	return rec.class()
}

func (PGClassRepo) ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*Class, error) {
	return getClass(ctx, ex, `SELECT `+classColumns+` FROM classes c WHERE c.id = $1`, id)
}

func (PGClassRepo) ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*Class, error) {
	return getClass(ctx, ex, `SELECT `+classColumns+` FROM classes c WHERE c.id = $1 FOR UPDATE`, id)
}

type classDetailRecord struct {
	ID              uuid.UUID  `db:"id"`
	Code            string     `db:"code"`
	Name            string     `db:"name"`
	Status          string     `db:"status"`
	StartDate       time.Time  `db:"start_date"`
	EndDate         time.Time  `db:"end_date"`
	TeacherID       uuid.UUID  `db:"teacher_id"`
	TeacherName     string     `db:"teacher_name"`
	CourseVersionID uuid.UUID  `db:"course_version_id"`
	CourseID        uuid.UUID  `db:"course_id"`
	CourseName      string     `db:"course_name"`
	VersionNo       int        `db:"version_no"`
	StageCount      int        `db:"stage_count"`
	LessonCount     int        `db:"lesson_count"`
	MemberCount     int        `db:"member_count"`
	ActivatedAt     *time.Time `db:"activated_at"`
	EndedAt         *time.Time `db:"ended_at"`
}

// Detail lấy mốc kích hoạt/kết thúc từ audit_logs vì bảng classes không có cột riêng (audit ghi cùng tx với
// lần chuyển trạng thái nên luôn có khi trạng thái đã đổi).
func (PGClassRepo) Detail(ctx context.Context, ex db.Executor, id uuid.UUID) (*ClassDetail, error) {
	var r classDetailRecord
	err := sqlx.GetContext(ctx, ex, &r, `SELECT c.id, c.code, c.name, c.status, c.start_date, c.end_date,
		c.teacher_id, t.full_name AS teacher_name, c.course_version_id, cv.course_id, co.name AS course_name, cv.version_no,
		(SELECT count(*) FROM course_version_stages cvs WHERE cvs.course_version_id = c.course_version_id) AS stage_count,
		(SELECT count(*) FROM course_version_stages cvs JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		  WHERE cvs.course_version_id = c.course_version_id) AS lesson_count,
		(SELECT count(*) FROM class_members cm WHERE cm.class_id = c.id AND cm.status = 'active') AS member_count,
		(SELECT max(a.at) FROM audit_logs a WHERE a.target_type = 'class' AND a.target_id = c.id AND a.action = $2) AS activated_at,
		(SELECT max(a.at) FROM audit_logs a WHERE a.target_type = 'class' AND a.target_id = c.id AND a.action = $3) AS ended_at
		FROM classes c
		JOIN users t ON t.id = c.teacher_id
		JOIN course_versions cv ON cv.id = c.course_version_id
		JOIN courses co ON co.id = cv.course_id
		WHERE c.id = $1`, id, audit.ActionClassActivated, audit.ActionClassEnded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrClassNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("classes: đọc chi tiết lớp: %w", err)
	}
	st, err := domain.ParseClassStatus(r.Status)
	if err != nil {
		return nil, fmt.Errorf("classes: trạng thái lớp %q: %w", r.Status, err)
	}
	return &ClassDetail{
		ID: r.ID, Code: r.Code, Name: r.Name, Status: st, StartDate: r.StartDate, EndDate: r.EndDate,
		TeacherID: r.TeacherID, TeacherName: r.TeacherName,
		CourseVersionID: r.CourseVersionID, CourseID: r.CourseID, CourseName: r.CourseName, VersionNo: r.VersionNo,
		StageCount: r.StageCount, LessonCount: r.LessonCount, MemberCount: r.MemberCount,
		ActivatedAt: r.ActivatedAt, EndedAt: r.EndedAt,
	}, nil
}

type classListRecord struct {
	ID              uuid.UUID `db:"id"`
	Code            string    `db:"code"`
	Name            string    `db:"name"`
	Status          string    `db:"status"`
	StartDate       time.Time `db:"start_date"`
	EndDate         time.Time `db:"end_date"`
	TeacherID       uuid.UUID `db:"teacher_id"`
	TeacherName     string    `db:"teacher_name"`
	CourseName      string    `db:"course_name"`
	CourseVersionNo int       `db:"course_version_no"`
	MemberCount     int       `db:"member_count"`
	NotLoggedIn     int       `db:"not_logged_in"`
	InactiveCount   int       `db:"inactive_count"`
}

// List đếm thành viên bằng một lần GROUP BY. "Không hoạt động" theo prototype: chưa từng hoạt động hoặc lần cuối
// trước StaleCutoff, chỉ tính khi lớp đang chạy.
func (PGClassRepo) List(ctx context.Context, ex db.Executor, q ListQuery) ([]ClassListRow, error) {
	status := ""
	if q.Status != nil {
		status = q.Status.String()
	}
	var recs []classListRecord
	err := sqlx.SelectContext(ctx, ex, &recs, `SELECT c.id, c.code, c.name, c.status, c.start_date, c.end_date,
		c.teacher_id, t.full_name AS teacher_name, co.name AS course_name, cv.version_no AS course_version_no,
		count(cm.id) FILTER (WHERE cm.status = 'active') AS member_count,
		count(cm.id) FILTER (WHERE cm.status = 'active' AND u.status = 'invited') AS not_logged_in,
		count(cm.id) FILTER (WHERE c.status = 'active' AND cm.status = 'active'
		  AND (u.last_active_at IS NULL OR u.last_active_at < $4)) AS inactive_count
		FROM classes c
		JOIN users t ON t.id = c.teacher_id
		JOIN course_versions cv ON cv.id = c.course_version_id
		JOIN courses co ON co.id = cv.course_id
		LEFT JOIN class_members cm ON cm.class_id = c.id
		LEFT JOIN users u ON u.id = cm.user_id
		WHERE ($1 = '' OR c.status = $1)
		  AND ($2::uuid IS NULL OR c.teacher_id = $2)
		  AND ($3 = '' OR c.code ILIKE '%' || $3 || '%' OR c.name ILIKE '%' || $3 || '%')
		GROUP BY c.id, t.full_name, co.name, cv.version_no
		ORDER BY c.created_at DESC, c.code`,
		status, q.TeacherID, escapeLike(strings.TrimSpace(q.Q)), q.StaleCutoff)
	if err != nil {
		return nil, fmt.Errorf("classes: liệt kê lớp: %w", err)
	}
	out := make([]ClassListRow, 0, len(recs))
	for _, r := range recs {
		st, err := domain.ParseClassStatus(r.Status)
		if err != nil {
			return nil, fmt.Errorf("classes: trạng thái lớp %q: %w", r.Status, err)
		}
		out = append(out, ClassListRow{
			ID: r.ID, Code: r.Code, Name: r.Name, Status: st, StartDate: r.StartDate, EndDate: r.EndDate,
			TeacherID: r.TeacherID, TeacherName: r.TeacherName, CourseName: r.CourseName, CourseVersionNo: r.CourseVersionNo,
			MemberCount: r.MemberCount, NotLoggedIn: r.NotLoggedIn, InactiveCount: r.InactiveCount,
		})
	}
	return out, nil
}

func (PGMemberRepo) Create(ctx context.Context, ex db.Executor, m *ClassMember) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO class_members (id, class_id, user_id, status, joined_at, dropped_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		m.id, m.classID, m.userID, m.status.String(), m.joinedAt, m.droppedAt)
	if err != nil {
		if code, cn := constraintOf(err); code == pgerrcode.UniqueViolation && cn == pgerr.UqClassMembersClassUser {
			return ErrAlreadyMember
		}
		return fmt.Errorf("classes: thêm thành viên: %w", pgerr.Map(err))
	}
	return nil
}

func (PGMemberRepo) Update(ctx context.Context, ex db.Executor, m *ClassMember, from domain.MemberStatus) error {
	err := db.ExecAffectOne(ctx, ex,
		`UPDATE class_members SET status = $3, dropped_at = $4 WHERE id = $1 AND status = $2`,
		m.id, from.String(), m.status.String(), m.droppedAt)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, db.ErrNoRowsAffected) && from == domain.MemberActive:
		return ErrMemberAlreadyDropped
	case errors.Is(err, db.ErrNoRowsAffected):
		return ErrMemberNotDropped
	default:
		return fmt.Errorf("classes: cập nhật thành viên: %w", pgerr.Map(err))
	}
}

const memberColumns = `cm.id, cm.class_id, cm.user_id, cm.status, cm.joined_at, cm.dropped_at`

type memberRecord struct {
	ID        uuid.UUID  `db:"id"`
	ClassID   uuid.UUID  `db:"class_id"`
	UserID    uuid.UUID  `db:"user_id"`
	Status    string     `db:"status"`
	JoinedAt  time.Time  `db:"joined_at"`
	DroppedAt *time.Time `db:"dropped_at"`
}

func (r memberRecord) member() (*ClassMember, error) {
	st, err := domain.ParseMemberStatus(r.Status)
	if err != nil {
		return nil, fmt.Errorf("classes: trạng thái thành viên %q: %w", r.Status, err)
	}
	return &ClassMember{id: r.ID, classID: r.ClassID, userID: r.UserID, status: st, joinedAt: r.JoinedAt, droppedAt: r.DroppedAt}, nil
}

func getMember(ctx context.Context, ex db.Executor, q string, args ...any) (*ClassMember, error) {
	var rec memberRecord
	err := sqlx.GetContext(ctx, ex, &rec, q, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("classes: đọc thành viên: %w", err)
	}
	return rec.member()
}

func (PGMemberRepo) ByID(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (*ClassMember, error) {
	return getMember(ctx, ex, `SELECT `+memberColumns+` FROM class_members cm WHERE cm.id = $1 AND cm.class_id = $2`, memberID, classID)
}

func (PGMemberRepo) ByIDForUpdate(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (*ClassMember, error) {
	return getMember(ctx, ex,
		`SELECT `+memberColumns+` FROM class_members cm WHERE cm.id = $1 AND cm.class_id = $2 FOR UPDATE`, memberID, classID)
}

func (PGMemberRepo) ByClassAndUser(ctx context.Context, ex db.Executor, classID, userID uuid.UUID) (*ClassMember, error) {
	m, err := getMember(ctx, ex,
		`SELECT `+memberColumns+` FROM class_members cm WHERE cm.class_id = $1 AND cm.user_id = $2`, classID, userID)
	if errors.Is(err, ErrMemberNotFound) {
		return nil, nil
	}
	return m, err
}

// memberRowQuery đọc thành viên kèm lời mời mới nhất và trạng thái gửi; sending hiển thị như queued, không có
// lời mời → các cột invite NULL.
const memberRowQuery = `SELECT cm.id, cm.user_id, u.email, u.full_name, u.status AS account_status,
	u.must_change_password, u.temp_password_expires_at, u.last_login_at, u.last_active_at,
	cm.status AS member_status, cm.joined_at, cm.dropped_at,
	inv.kind AS invite_kind, inv.created_at AS invited_at,
	CASE WHEN eo.id IS NULL THEN NULL WHEN eo.status = 'sending' THEN 'queued' ELSE eo.status END AS invite_status,
	eo.attempts AS invite_attempts, eo.last_error AS invite_last_error
	FROM class_members cm
	JOIN users u ON u.id = cm.user_id
	LEFT JOIN LATERAL (SELECT i.kind, i.created_at, i.email_outbox_id FROM invitations i
	  WHERE i.class_id = cm.class_id AND i.user_id = cm.user_id ORDER BY i.created_at DESC, i.id DESC LIMIT 1) inv ON true
	LEFT JOIN email_outbox eo ON eo.id = inv.email_outbox_id`

type memberRowRecord struct {
	ID                    uuid.UUID  `db:"id"`
	UserID                uuid.UUID  `db:"user_id"`
	Email                 string     `db:"email"`
	FullName              string     `db:"full_name"`
	AccountStatus         string     `db:"account_status"`
	MustChangePassword    bool       `db:"must_change_password"`
	TempPasswordExpiresAt *time.Time `db:"temp_password_expires_at"`
	LastLoginAt           *time.Time `db:"last_login_at"`
	LastActiveAt          *time.Time `db:"last_active_at"`
	MemberStatus          string     `db:"member_status"`
	JoinedAt              time.Time  `db:"joined_at"`
	DroppedAt             *time.Time `db:"dropped_at"`
	InviteKind            *string    `db:"invite_kind"`
	InvitedAt             *time.Time `db:"invited_at"`
	InviteStatus          *string    `db:"invite_status"`
	InviteAttempts        *int       `db:"invite_attempts"`
	InviteLastError       *string    `db:"invite_last_error"`
}

func (r memberRowRecord) row() (MemberRow, error) {
	acc, err := domain.ParseUserStatus(r.AccountStatus)
	if err != nil {
		return MemberRow{}, fmt.Errorf("classes: trạng thái tài khoản %q: %w", r.AccountStatus, err)
	}
	ms, err := domain.ParseMemberStatus(r.MemberStatus)
	if err != nil {
		return MemberRow{}, fmt.Errorf("classes: trạng thái thành viên %q: %w", r.MemberStatus, err)
	}
	var kind *InvitationKind
	if r.InviteKind != nil {
		k := InvitationKind(*r.InviteKind)
		kind = &k
	}
	return MemberRow{
		ID: r.ID, UserID: r.UserID, Email: r.Email, FullName: r.FullName, AccountStatus: acc,
		MustChangePassword: r.MustChangePassword, TempPasswordExpiresAt: r.TempPasswordExpiresAt,
		MemberStatus: ms, JoinedAt: r.JoinedAt, DroppedAt: r.DroppedAt,
		LastLoginAt: r.LastLoginAt, LastActiveAt: r.LastActiveAt,
		InviteKind: kind, InvitedAt: r.InvitedAt, InviteStatus: r.InviteStatus,
		InviteAttempts: r.InviteAttempts, InviteLastError: r.InviteLastError,
	}, nil
}

func (PGMemberRepo) ListRows(ctx context.Context, ex db.Executor, classID uuid.UUID, includeDropped bool) ([]MemberRow, error) {
	var recs []memberRowRecord
	err := sqlx.SelectContext(ctx, ex, &recs,
		memberRowQuery+` WHERE cm.class_id = $1 AND ($2 OR cm.status <> 'dropped') ORDER BY u.full_name, cm.id`, classID, includeDropped)
	if err != nil {
		return nil, fmt.Errorf("classes: liệt kê thành viên: %w", err)
	}
	out := make([]MemberRow, 0, len(recs))
	for _, r := range recs {
		row, err := r.row()
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (PGMemberRepo) RowByID(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (*MemberRow, error) {
	var rec memberRowRecord
	err := sqlx.GetContext(ctx, ex, &rec, memberRowQuery+` WHERE cm.id = $1 AND cm.class_id = $2`, memberID, classID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("classes: đọc thành viên: %w", err)
	}
	row, err := rec.row()
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (PGInvitationRepo) Create(ctx context.Context, ex db.Executor, inv *Invitation) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO invitations (id, class_id, user_id, kind, email_outbox_id, invited_by, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		inv.ID, inv.ClassID, inv.UserID, string(inv.Kind), inv.EmailOutboxID, inv.InvitedBy, inv.CreatedAt)
	if err != nil {
		return fmt.Errorf("classes: ghi lời mời: %w", pgerr.Map(err))
	}
	return nil
}

func (PGInvitationRepo) CountResendSince(ctx context.Context, ex db.Executor, classID, userID uuid.UUID, since time.Time) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, ex, &n,
		`SELECT count(*) FROM invitations WHERE class_id = $1 AND user_id = $2 AND kind = 'resend' AND created_at >= $3`,
		classID, userID, since)
	if err != nil {
		return 0, fmt.Errorf("classes: đếm lần gửi lại: %w", err)
	}
	return n, nil
}
