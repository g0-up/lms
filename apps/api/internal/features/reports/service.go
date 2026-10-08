package reports

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/learning"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
)

// ReadTx là db.Tx chỉ đọc REPEATABLE READ: mọi truy vấn của một báo cáo thấy cùng một snapshot, nên dòng và
// tổng hợp không lệch nhau.
type ReadTx struct{ DB *sqlx.DB }

// Transact chạy fn trong transaction chỉ đọc.
func (r ReadTx) Transact(ctx context.Context, fn func(tx db.Executor) error) error {
	return db.TransactWithOptions(ctx, r.DB, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, fn)
}

// Actor là người xem báo cáo.
type Actor struct {
	ID   uuid.UUID
	Role domain.Role
}

// Service là use case đọc báo cáo lớp và dashboard.
type Service struct {
	tx        db.Tx
	reports   ReportRepo
	dashboard DashboardRepo
	progress  learning.ProgressReader
	outdated  stages.OutdatedReader
	audit     audit.Reader
	users     identity.UserReader
	clock     clock.Clock
	staleDays int
}

// Deps là phụ thuộc của Service. Tx nên là ReadTx; StaleDays là ngưỡng "không hoạt động" mặc định của Summary.
type Deps struct {
	Tx        db.Tx
	Reports   ReportRepo
	Dashboard DashboardRepo
	Progress  learning.ProgressReader
	Outdated  stages.OutdatedReader
	Audit     audit.Reader
	Users     identity.UserReader
	Clock     clock.Clock
	StaleDays int
}

// NewService nối phụ thuộc.
func NewService(d Deps) *Service {
	return &Service{
		tx: d.Tx, reports: d.Reports, dashboard: d.Dashboard, progress: d.Progress, outdated: d.Outdated,
		audit: d.Audit, users: d.Users, clock: d.Clock, staleDays: d.StaleDays,
	}
}

// classHeader đọc lớp và kiểm quyền xem: admin mọi lớp, giảng viên chỉ lớp mình phụ trách, vai trò khác không.
func (s *Service) classHeader(ctx context.Context, tx db.Executor, actor Actor, classID uuid.UUID) (ClassHeader, error) {
	h, err := s.reports.ClassHeader(ctx, tx, classID)
	if err != nil {
		return ClassHeader{}, err
	}
	switch {
	case actor.Role == domain.RoleAdmin:
		return h, nil
	case actor.Role == domain.RoleTeacher && h.TeacherID == actor.ID:
		return h, nil
	case actor.Role == domain.RoleTeacher:
		return ClassHeader{}, classes.ErrNotOwnClass
	default:
		return ClassHeader{}, domain.ErrForbidden
	}
}

// ClassReport là báo cáo tiến độ lớp: dòng đã lọc/sắp xếp theo f, tổng hợp tính trên toàn lớp.
func (s *Service) ClassReport(ctx context.Context, actor Actor, classID uuid.UUID, f Filter) (ClassReport, error) {
	if err := f.Validate(); err != nil {
		return ClassReport{}, err
	}
	now := s.clock.Now()
	inactiveDays, below := s.staleDays, defaultBelowPercent
	if f.InactiveDays != nil {
		inactiveDays = *f.InactiveDays
	}
	if f.BelowPercent != nil {
		below = *f.BelowPercent
	}
	out := ClassReport{Filter: f, SelfReported: true}
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		h, err := s.classHeader(ctx, tx, actor, classID)
		if err != nil {
			return err
		}
		out.Class = h
		if out.Stages, err = s.reports.StageHeaders(ctx, tx, h.CourseVersionID); err != nil {
			return err
		}
		if out.Rows, err = s.reports.Rows(ctx, tx, classID, f, now); err != nil {
			return err
		}
		out.Summary, err = s.reports.Summary(ctx, tx, classID, SummaryQuery{
			IncludeDropped: f.IncludeDropped, InactiveCutoff: inactiveCutoff(now, inactiveDays), BelowPercent: below,
		})
		return err
	})
	if err != nil {
		return ClassReport{}, err
	}
	out.Summary.InactiveDays, out.Summary.BelowPercent = inactiveDays, below
	return out, nil
}

// MemberReport là drilldown một thành viên của lớp; thành viên không thuộc lớp → không tìm thấy (không lộ thành
// viên của lớp khác).
func (s *Service) MemberReport(ctx context.Context, actor Actor, classID, memberID uuid.UUID) (MemberReport, error) {
	out := MemberReport{SelfReported: true}
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		h, err := s.classHeader(ctx, tx, actor, classID)
		if err != nil {
			return err
		}
		out.Class = h
		row, err := s.reports.MemberRow(ctx, tx, classID, memberID)
		if err != nil {
			return err
		}
		if row == nil {
			return classes.ErrMemberNotFound
		}
		out.Member = *row
		rm, err := s.progress.MemberLessonProgress(ctx, tx, classID, memberID)
		if err != nil {
			return err
		}
		out.Stages = rm.Stages
		return nil
	})
	if err != nil {
		return MemberReport{}, err
	}
	return out, nil
}

// Dashboard là trang tổng quan của admin. Danh sách khóa học dùng chặng cũ do stages đọc bằng kết nối riêng;
// phần còn lại đọc trong một transaction chỉ đọc.
func (s *Service) Dashboard(ctx context.Context, actor Actor) (Dashboard, error) {
	if actor.Role != domain.RoleAdmin {
		return Dashboard{}, domain.ErrForbidden
	}
	outdated, err := s.outdated.AllOutdated(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	var out Dashboard
	out.Outdated, out.Hints.OutdatedCourses = outdatedRows(outdated)
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		var err error
		if out.KPIs, err = s.dashboard.KPIs(ctx, tx); err != nil {
			return err
		}
		hints, err := s.dashboard.Hints(ctx, tx)
		if err != nil {
			return err
		}
		hints.OutdatedCourses = out.Hints.OutdatedCourses
		out.Hints = hints
		if out.Classes, err = s.classRows(ctx, tx); err != nil {
			return err
		}
		out.RecentActivity, err = s.recentActivity(ctx, tx)
		return err
	})
	if err != nil {
		return Dashboard{}, err
	}
	return out, nil
}

// outdatedRows map 1:1 từ stages và đếm số khóa học khác nhau.
func outdatedRows(in []stages.OutdatedCourse) ([]OutdatedRow, int) {
	out := make([]OutdatedRow, len(in))
	courses := map[uuid.UUID]struct{}{}
	for i, o := range in {
		out[i] = OutdatedRow{
			CourseID: o.CourseID, CourseCode: o.CourseCode, CourseName: o.CourseName,
			CourseVersionID: o.CourseVersionID, CourseVersionNo: int(o.CourseVersionNo),
			StageID: o.StageID, StageCode: o.StageCode, StageName: o.StageName,
			CurrentVersionNo: int(o.UsingVersionNo), LatestVersionID: o.LatestVersionID,
			LatestPublishedNo: int(o.LatestVersionNo),
		}
		courses[o.CourseID] = struct{}{}
	}
	return out, len(courses)
}

func (s *Service) classRows(ctx context.Context, tx db.Executor) ([]ClassRow, error) {
	rows, err := s.dashboard.Classes(ctx, tx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	avg, err := s.progress.ClassAveragePercent(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].AvgPercent = avg[rows[i].ID]
	}
	return rows, nil
}

// recentActivity dựng các dòng nhật ký mới nhất; tên đối tượng và người dùng tra theo lô.
func (s *Service) recentActivity(ctx context.Context, tx db.Executor) ([]ActivityRow, error) {
	entries, err := s.audit.Latest(ctx, tx, recentActivityLimit)
	if err != nil {
		return nil, err
	}
	refs, userIDs := activityRefs(entries)
	lk := activityLookups{Users: map[uuid.UUID]UserRef{}}
	if lk.Targets, err = s.dashboard.TargetLabels(ctx, tx, refs); err != nil {
		return nil, err
	}
	users, err := s.users.SnapshotByIDs(ctx, tx, userIDs)
	if err != nil {
		return nil, err
	}
	for id, u := range users {
		lk.Users[id] = UserRef{Name: u.Name, Email: u.Email.String()}
	}
	out := make([]ActivityRow, len(entries))
	for i, e := range entries {
		if _, ok := actionLabel(e.Action); !ok {
			slog.Default().WarnContext(ctx, "reports: action audit chưa có nhãn", slog.String("action", e.Action))
		}
		out[i] = describeActivity(e, lk)
	}
	return out, nil
}
