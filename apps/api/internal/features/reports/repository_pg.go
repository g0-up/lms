package reports

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/platform/db"
)

// PGReportRepo là ReportRepo trên PostgreSQL.
type PGReportRepo struct{}

// PGDashboardRepo là DashboardRepo trên PostgreSQL.
type PGDashboardRepo struct{}

var (
	_ ReportRepo    = PGReportRepo{}
	_ DashboardRepo = PGDashboardRepo{}
)

// percentOf là phần trăm round(100 * done / total) như domain.Percent; total = 0 → 0.
func percentOf(done, total string) string {
	return `(CASE WHEN ` + total + ` = 0 THEN 0 ELSE round(100.0 * ` + done + ` / ` + total + `) END)::int`
}

// memberProgress là các CTE tiến độ thành viên của lớp $1 ($2 là một thành viên, NULL = mọi thành viên):
// per_stage một dòng mỗi (thành viên, chặng) gồm học liệu bắt buộc và lần mở/hoàn thành gần nhất (lớp chưa có
// chặng: một dòng stage_id NULL); m một dòng mỗi thành viên với % toàn khóa và hoạt động gần nhất là mốc mới hơn
// giữa users.last_active_at và học liệu.
var memberProgress = `WITH per_stage AS (
	  SELECT cm.id AS member_id, cvs.stage_id, cvs.position,
	    count(l.id) FILTER (WHERE l.required) AS required_total,
	    count(lp.completed_at) FILTER (WHERE l.required) AS required_done,
	    max(greatest(lp.first_opened_at, lp.completed_at)) AS lesson_activity_at
	  FROM class_members cm
	  JOIN classes c ON c.id = cm.class_id
	  LEFT JOIN course_version_stages cvs ON cvs.course_version_id = c.course_version_id
	  LEFT JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
	  LEFT JOIN lesson_progress lp ON lp.class_member_id = cm.id AND lp.lesson_id = l.id
	  WHERE cm.class_id = $1 AND ($2::uuid IS NULL OR cm.id = $2::uuid)
	  GROUP BY cm.id, cvs.stage_id, cvs.position
	), per_member AS (
	  SELECT member_id, sum(required_total)::int AS required_total, sum(required_done)::int AS required_done,
	    max(lesson_activity_at) AS lesson_activity_at
	  FROM per_stage GROUP BY member_id
	), m AS (
	  SELECT cm.id AS member_id, cm.user_id, u.full_name AS name, u.email, u.status AS account_status,
	    u.must_change_password, cm.status AS member_status, u.last_login_at,
	    greatest(pm.lesson_activity_at, u.last_active_at) AS last_activity_at,
	    pm.required_total, pm.required_done, ` + percentOf("pm.required_done", "pm.required_total") + ` AS percent
	  FROM per_member pm
	  JOIN class_members cm ON cm.id = pm.member_id
	  JOIN users u ON u.id = cm.user_id
	)`

// rowsQuery là truy vấn Rows: một dòng mỗi (thành viên, chặng) kèm lời mời mới nhất (sending hiển thị như
// queued); $3..$6 là tham số lọc của Filter.Apply, ORDER BY nối sau từ allowlist orderBy.
var rowsQuery = memberProgress + `
	SELECT m.member_id, m.user_id, m.name, m.email, m.account_status, m.must_change_password, m.member_status,
	  m.last_login_at, m.last_activity_at, m.required_total, m.required_done, m.percent,
	  ps.stage_id, ps.required_total AS stage_required_total, ps.required_done AS stage_required_done,
	  ` + percentOf("ps.required_done", "ps.required_total") + ` AS stage_percent,
	  inv.kind AS invite_kind,
	  CASE WHEN eo.id IS NULL THEN NULL WHEN eo.status = 'sending' THEN 'queued' ELSE eo.status END AS invite_status,
	  eo.attempts AS invite_attempts, eo.last_error AS invite_last_error
	FROM m
	JOIN per_stage ps ON ps.member_id = m.member_id
	LEFT JOIN LATERAL (SELECT i.kind, i.email_outbox_id FROM invitations i
	  WHERE i.class_id = $1 AND i.user_id = m.user_id ORDER BY i.created_at DESC, i.id DESC LIMIT 1) inv ON true
	LEFT JOIN email_outbox eo ON eo.id = inv.email_outbox_id
	WHERE ($3::bool IS FALSE OR m.last_login_at IS NULL)
	  AND ($4::timestamptz IS NULL OR m.last_activity_at IS NULL OR m.last_activity_at < $4::timestamptz)
	  AND ($5::int IS NULL OR m.percent < $5::int)
	  AND ($6::bool OR m.member_status = 'active')
	ORDER BY `

type reportRowRecord struct {
	MemberID           uuid.UUID  `db:"member_id"`
	UserID             uuid.UUID  `db:"user_id"`
	Name               string     `db:"name"`
	Email              string     `db:"email"`
	AccountStatus      string     `db:"account_status"`
	MustChangePassword bool       `db:"must_change_password"`
	MemberStatus       string     `db:"member_status"`
	LastLoginAt        *time.Time `db:"last_login_at"`
	LastActivityAt     *time.Time `db:"last_activity_at"`
	RequiredTotal      int        `db:"required_total"`
	RequiredDone       int        `db:"required_done"`
	Percent            int        `db:"percent"`
	StageID            *uuid.UUID `db:"stage_id"`
	StageRequiredTotal int        `db:"stage_required_total"`
	StageRequiredDone  int        `db:"stage_required_done"`
	StagePercent       int        `db:"stage_percent"`
	InviteKind         *string    `db:"invite_kind"`
	InviteStatus       *string    `db:"invite_status"`
	InviteAttempts     *int       `db:"invite_attempts"`
	InviteLastError    *string    `db:"invite_last_error"`
}

func (r reportRowRecord) row() (ReportRow, error) {
	as, err := domain.ParseUserStatus(r.AccountStatus)
	if err != nil {
		return ReportRow{}, fmt.Errorf("reports: trạng thái tài khoản %q: %w", r.AccountStatus, err)
	}
	ms, err := domain.ParseMemberStatus(r.MemberStatus)
	if err != nil {
		return ReportRow{}, fmt.Errorf("reports: trạng thái thành viên %q: %w", r.MemberStatus, err)
	}
	row := ReportRow{
		MemberID: r.MemberID, UserID: r.UserID, Name: r.Name, Email: r.Email, AccountStatus: as, MemberStatus: ms,
		MustChangePassword: r.MustChangePassword, StagePercents: []StagePercent{}, Percent: r.Percent,
		RequiredDone: r.RequiredDone, RequiredTotal: r.RequiredTotal, LastLoginAt: r.LastLoginAt, LastActivityAt: r.LastActivityAt,
	}
	if r.InviteKind != nil {
		inv := &InviteStatus{Kind: *r.InviteKind, Status: r.InviteStatus, LastError: r.InviteLastError}
		if r.InviteAttempts != nil {
			inv.Attempts = *r.InviteAttempts
		}
		row.Invite = inv
	}
	return row, nil
}

// selectRows chạy rowsQuery rồi gộp các dòng (thành viên, chặng) liên tiếp thành một ReportRow.
func selectRows(ctx context.Context, ex db.Executor, classID uuid.UUID, memberID *uuid.UUID, sf sqlFilter) ([]ReportRow, error) {
	var recs []reportRowRecord
	args := append([]any{classID, memberID}, sf.args...)
	if err := sqlx.SelectContext(ctx, ex, &recs, rowsQuery+sf.orderBy+`, ps.position`, args...); err != nil {
		return nil, fmt.Errorf("reports: đọc dòng báo cáo: %w", err)
	}
	out := []ReportRow{}
	for _, r := range recs {
		if len(out) == 0 || out[len(out)-1].MemberID != r.MemberID {
			row, err := r.row()
			if err != nil {
				return nil, err
			}
			out = append(out, row)
		}
		if r.StageID == nil {
			continue
		}
		last := &out[len(out)-1]
		last.StagePercents = append(last.StagePercents, StagePercent{
			StageID: *r.StageID, Percent: r.StagePercent, RequiredDone: r.StageRequiredDone, RequiredTotal: r.StageRequiredTotal,
		})
	}
	return out, nil
}

func (PGReportRepo) ClassHeader(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassHeader, error) {
	var r struct {
		ID              uuid.UUID `db:"id"`
		Code            string    `db:"code"`
		Name            string    `db:"name"`
		Status          string    `db:"status"`
		CourseVersionID uuid.UUID `db:"course_version_id"`
		CourseName      string    `db:"course_name"`
		CourseVersionNo int       `db:"course_version_no"`
		TeacherID       uuid.UUID `db:"teacher_id"`
		TeacherName     string    `db:"teacher_name"`
	}
	err := sqlx.GetContext(ctx, ex, &r, `SELECT c.id, c.code, c.name, c.status, c.course_version_id,
		  co.name AS course_name, cv.version_no AS course_version_no, c.teacher_id, t.full_name AS teacher_name
		FROM classes c
		JOIN course_versions cv ON cv.id = c.course_version_id
		JOIN courses co ON co.id = cv.course_id
		JOIN users t ON t.id = c.teacher_id
		WHERE c.id = $1`, classID)
	if errors.Is(err, sql.ErrNoRows) {
		return ClassHeader{}, classes.ErrClassNotFound
	}
	if err != nil {
		return ClassHeader{}, fmt.Errorf("reports: đọc lớp: %w", err)
	}
	st, err := domain.ParseClassStatus(r.Status)
	if err != nil {
		return ClassHeader{}, fmt.Errorf("reports: trạng thái lớp %q: %w", r.Status, err)
	}
	return ClassHeader{
		ID: r.ID, Code: r.Code, Name: r.Name, Status: st, CourseVersionID: r.CourseVersionID, CourseName: r.CourseName,
		CourseVersionNo: r.CourseVersionNo, TeacherID: r.TeacherID, TeacherName: r.TeacherName,
	}, nil
}

func (PGReportRepo) StageHeaders(ctx context.Context, ex db.Executor, courseVersionID uuid.UUID) ([]StageHeader, error) {
	var recs []struct {
		StageID       uuid.UUID `db:"stage_id"`
		Code          string    `db:"code"`
		Name          string    `db:"name"`
		VersionNo     int       `db:"version_no"`
		Position      int       `db:"position"`
		RequiredTotal int       `db:"required_total"`
	}
	if err := sqlx.SelectContext(ctx, ex, &recs, `SELECT cvs.stage_id, s.code, s.name, sv.version_no, cvs.position,
		  count(l.id) FILTER (WHERE l.required) AS required_total
		FROM course_version_stages cvs
		JOIN stages s ON s.id = cvs.stage_id
		JOIN stage_versions sv ON sv.id = cvs.stage_version_id
		LEFT JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
		WHERE cvs.course_version_id = $1
		GROUP BY cvs.stage_id, s.code, s.name, sv.version_no, cvs.position
		ORDER BY cvs.position`, courseVersionID); err != nil {
		return nil, fmt.Errorf("reports: đọc chặng của khóa học: %w", err)
	}
	out := make([]StageHeader, len(recs))
	for i, r := range recs {
		out[i] = StageHeader{
			StageID: r.StageID, Code: r.Code, Name: r.Name, VersionNo: r.VersionNo, Position: r.Position, RequiredTotal: r.RequiredTotal,
		}
	}
	return out, nil
}

func (PGReportRepo) Rows(ctx context.Context, ex db.Executor, classID uuid.UUID, f Filter, now time.Time) ([]ReportRow, error) {
	sf, err := f.Apply(now)
	if err != nil {
		return nil, err
	}
	return selectRows(ctx, ex, classID, nil, sf)
}

func (PGReportRepo) MemberRow(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (*ReportRow, error) {
	sf, err := Filter{IncludeDropped: true, Sort: SortName}.Apply(time.Time{})
	if err != nil {
		return nil, err
	}
	rows, err := selectRows(ctx, ex, classID, &memberID, sf)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (PGReportRepo) Summary(ctx context.Context, ex db.Executor, classID uuid.UUID, q SummaryQuery) (Summary, error) {
	var r struct {
		MemberCount      int `db:"member_count"`
		ActiveCount      int `db:"active_count"`
		AvgPercent       int `db:"avg_percent"`
		NotLoggedInCount int `db:"not_logged_in_count"`
		InactiveCount    int `db:"inactive_count"`
		BelowCount       int `db:"below_count"`
	}
	err := sqlx.GetContext(ctx, ex, &r, memberProgress+`
		SELECT count(*) AS member_count,
		  count(*) FILTER (WHERE member_status = 'active') AS active_count,
		  coalesce(round(avg(percent)), 0)::int AS avg_percent,
		  count(*) FILTER (WHERE last_login_at IS NULL) AS not_logged_in_count,
		  count(*) FILTER (WHERE last_activity_at IS NULL OR last_activity_at < $4) AS inactive_count,
		  count(*) FILTER (WHERE percent < $5) AS below_count
		FROM m
		WHERE ($3::bool OR member_status = 'active')`, classID, nil, q.IncludeDropped, q.InactiveCutoff, q.BelowPercent)
	if err != nil {
		return Summary{}, fmt.Errorf("reports: tính tổng hợp lớp: %w", err)
	}
	return Summary{
		MemberCount: r.MemberCount, ActiveCount: r.ActiveCount, AvgPercent: r.AvgPercent,
		NotLoggedInCount: r.NotLoggedInCount, InactiveCount: r.InactiveCount, BelowCount: r.BelowCount,
	}, nil
}

func (PGDashboardRepo) KPIs(ctx context.Context, ex db.Executor) (KPIs, error) {
	var r struct {
		Stages   int `db:"stages"`
		Courses  int `db:"courses"`
		Classes  int `db:"classes"`
		Students int `db:"students"`
	}
	if err := sqlx.GetContext(ctx, ex, &r, `SELECT
		  (SELECT count(*) FROM stages) AS stages,
		  (SELECT count(*) FROM courses) AS courses,
		  (SELECT count(*) FROM classes WHERE status = 'active') AS classes,
		  (SELECT count(DISTINCT cm.user_id) FROM class_members cm JOIN classes c ON c.id = cm.class_id
		    WHERE cm.status = 'active' AND c.status = 'active') AS students`); err != nil {
		return KPIs{}, fmt.Errorf("reports: đếm KPI: %w", err)
	}
	return KPIs{Stages: r.Stages, Courses: r.Courses, Classes: r.Classes, Students: r.Students}, nil
}

// Hints: lời mời thất bại chỉ xét lời mời mới nhất của mỗi thành viên active trong lớp chưa kết thúc, nên lời
// mời đã gửi lại thành công không còn bị đếm.
func (PGDashboardRepo) Hints(ctx context.Context, ex db.Executor) (Hints, error) {
	var r struct {
		DraftClasses  int `db:"draft_classes"`
		NotLoggedIn   int `db:"not_logged_in"`
		FailedInvites int `db:"failed_invites"`
	}
	if err := sqlx.GetContext(ctx, ex, &r, `SELECT
		  (SELECT count(*) FROM classes WHERE status = 'draft') AS draft_classes,
		  (SELECT count(DISTINCT u.id) FROM class_members cm
		    JOIN classes c ON c.id = cm.class_id JOIN users u ON u.id = cm.user_id
		    WHERE cm.status = 'active' AND c.status = 'active' AND u.last_login_at IS NULL) AS not_logged_in,
		  (SELECT count(*) FROM class_members cm
		    JOIN classes c ON c.id = cm.class_id
		    JOIN LATERAL (SELECT i.email_outbox_id FROM invitations i
		      WHERE i.class_id = cm.class_id AND i.user_id = cm.user_id ORDER BY i.created_at DESC, i.id DESC LIMIT 1) inv ON true
		    JOIN email_outbox eo ON eo.id = inv.email_outbox_id
		    WHERE cm.status = 'active' AND c.status <> 'ended' AND eo.status = 'failed') AS failed_invites`); err != nil {
		return Hints{}, fmt.Errorf("reports: đếm gợi ý dashboard: %w", err)
	}
	return Hints{DraftClasses: r.DraftClasses, NotLoggedIn: r.NotLoggedIn, FailedInvites: r.FailedInvites}, nil
}

func (PGDashboardRepo) Classes(ctx context.Context, ex db.Executor) ([]ClassRow, error) {
	var recs []struct {
		ID              uuid.UUID `db:"id"`
		Code            string    `db:"code"`
		Name            string    `db:"name"`
		Status          string    `db:"status"`
		CourseName      string    `db:"course_name"`
		CourseVersionNo int       `db:"course_version_no"`
		MemberCount     int       `db:"member_count"`
	}
	if err := sqlx.SelectContext(ctx, ex, &recs, `SELECT c.id, c.code, c.name, c.status, co.name AS course_name,
		  cv.version_no AS course_version_no,
		  (SELECT count(*) FROM class_members cm WHERE cm.class_id = c.id AND cm.status = 'active') AS member_count
		FROM classes c
		JOIN course_versions cv ON cv.id = c.course_version_id
		JOIN courses co ON co.id = cv.course_id
		ORDER BY c.code`); err != nil {
		return nil, fmt.Errorf("reports: liệt kê lớp: %w", err)
	}
	out := make([]ClassRow, len(recs))
	for i, r := range recs {
		st, err := domain.ParseClassStatus(r.Status)
		if err != nil {
			return nil, fmt.Errorf("reports: trạng thái lớp %q: %w", r.Status, err)
		}
		out[i] = ClassRow{
			ID: r.ID, Code: r.Code, Name: r.Name, Status: st, CourseName: r.CourseName,
			CourseVersionNo: r.CourseVersionNo, MemberCount: r.MemberCount,
		}
	}
	return out, nil
}

func (PGDashboardRepo) TargetLabels(ctx context.Context, ex db.Executor, refs []TargetRef) (map[TargetRef]TargetInfo, error) {
	out := make(map[TargetRef]TargetInfo, len(refs))
	if len(refs) == 0 {
		return out, nil
	}
	byType := map[string][]string{}
	for _, r := range refs {
		byType[r.Type] = append(byType[r.Type], r.ID.String())
	}
	var recs []struct {
		Type      string    `db:"type"`
		ID        uuid.UUID `db:"id"`
		Name      string    `db:"name"`
		VersionNo int       `db:"version_no"`
	}
	if err := sqlx.SelectContext(ctx, ex, &recs, `
		SELECT 'class' AS type, id, code AS name, 0 AS version_no FROM classes WHERE id = ANY($1::uuid[])
		UNION ALL
		SELECT 'stage', id, name, 0 FROM stages WHERE id = ANY($2::uuid[])
		UNION ALL
		SELECT 'stage_version', sv.id, s.name, sv.version_no FROM stage_versions sv JOIN stages s ON s.id = sv.stage_id
		  WHERE sv.id = ANY($3::uuid[])
		UNION ALL
		SELECT 'course', id, name, 0 FROM courses WHERE id = ANY($4::uuid[])
		UNION ALL
		SELECT 'course_version', cv.id, co.name, cv.version_no FROM course_versions cv JOIN courses co ON co.id = cv.course_id
		  WHERE cv.id = ANY($5::uuid[])`,
		byType[targetClass], byType[targetStage], byType[targetStageVersion], byType[targetCourse],
		byType[targetCourseVersion]); err != nil {
		return nil, fmt.Errorf("reports: tra tên đối tượng nhật ký: %w", err)
	}
	for _, r := range recs {
		out[TargetRef{Type: r.Type, ID: r.ID}] = TargetInfo{Name: r.Name, VersionNo: r.VersionNo}
	}
	return out, nil
}
