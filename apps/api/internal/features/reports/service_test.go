package reports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/learning"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
)

type fakeTx struct{ calls int }

func (f *fakeTx) Transact(_ context.Context, fn func(db.Executor) error) error {
	f.calls++
	return fn(nil)
}

type fakeReports struct {
	header    *ClassHeader
	rows      []ReportRow
	member    *ReportRow
	summaryQ  *SummaryQuery
	rowsCalls int
}

func (f *fakeReports) ClassHeader(context.Context, db.Executor, uuid.UUID) (ClassHeader, error) {
	if f.header == nil {
		return ClassHeader{}, classes.ErrClassNotFound
	}
	return *f.header, nil
}

func (f *fakeReports) StageHeaders(context.Context, db.Executor, uuid.UUID) ([]StageHeader, error) {
	return []StageHeader{}, nil
}

func (f *fakeReports) Rows(context.Context, db.Executor, uuid.UUID, Filter, time.Time) ([]ReportRow, error) {
	f.rowsCalls++
	return f.rows, nil
}

func (f *fakeReports) MemberRow(context.Context, db.Executor, uuid.UUID, uuid.UUID) (*ReportRow, error) {
	return f.member, nil
}

func (f *fakeReports) Summary(_ context.Context, _ db.Executor, _ uuid.UUID, q SummaryQuery) (Summary, error) {
	f.summaryQ = &q
	return Summary{MemberCount: len(f.rows), ActiveCount: len(f.rows)}, nil
}

type fakeDashboard struct {
	classes []ClassRow
	targets map[TargetRef]TargetInfo
}

func (f *fakeDashboard) KPIs(context.Context, db.Executor) (KPIs, error) {
	return KPIs{Stages: 5, Courses: 1, Classes: 2, Students: 11}, nil
}

func (f *fakeDashboard) Hints(context.Context, db.Executor) (Hints, error) {
	return Hints{DraftClasses: 1, NotLoggedIn: 2, FailedInvites: 1}, nil
}

func (f *fakeDashboard) Classes(context.Context, db.Executor) ([]ClassRow, error) {
	return f.classes, nil
}

func (f *fakeDashboard) TargetLabels(context.Context, db.Executor, []TargetRef) (map[TargetRef]TargetInfo, error) {
	return f.targets, nil
}

type fakeProgress struct {
	learning.ProgressReader
	avg     map[uuid.UUID]int
	roadmap learning.Roadmap
}

func (f fakeProgress) MemberLessonProgress(context.Context, db.Executor, uuid.UUID, uuid.UUID) (learning.Roadmap, error) {
	return f.roadmap, nil
}

func (f fakeProgress) ClassAveragePercent(context.Context, db.Executor, []uuid.UUID) (map[uuid.UUID]int, error) {
	return f.avg, nil
}

type fakeOutdated struct {
	stages.OutdatedReader
	rows []stages.OutdatedCourse
	err  error
}

func (f fakeOutdated) AllOutdated(context.Context) ([]stages.OutdatedCourse, error) {
	return f.rows, f.err
}

type fakeAudit struct{ entries []audit.LoggedEntry }

func (f fakeAudit) Latest(_ context.Context, _ db.Executor, limit int) ([]audit.LoggedEntry, error) {
	if limit != recentActivityLimit {
		return nil, errors.New("limit sai")
	}
	return f.entries, nil
}

type fakeUsers struct {
	identity.UserReader
	users map[uuid.UUID]identity.UserSnapshot
}

func (f fakeUsers) SnapshotByIDs(context.Context, db.Executor, []uuid.UUID) (map[uuid.UUID]identity.UserSnapshot, error) {
	return f.users, nil
}

var testNow = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

type serviceFixture struct {
	svc       *Service
	tx        *fakeTx
	reports   *fakeReports
	dashboard *fakeDashboard
	teacherID uuid.UUID
	classID   uuid.UUID
}

func newServiceFixture(progress fakeProgress, outdated fakeOutdated, au fakeAudit, users fakeUsers) serviceFixture {
	f := serviceFixture{tx: &fakeTx{}, teacherID: uuid.New(), classID: uuid.New(), dashboard: &fakeDashboard{}}
	f.reports = &fakeReports{header: &ClassHeader{ID: f.classID, Code: "basic01", TeacherID: f.teacherID}}
	f.svc = NewService(Deps{
		Tx: f.tx, Reports: f.reports, Dashboard: f.dashboard, Progress: progress, Outdated: outdated,
		Audit: au, Users: users, Clock: &clock.Fake{T: testNow}, StaleDays: 7,
	})
	return f
}

func TestClassReportScope(t *testing.T) {
	f := newServiceFixture(fakeProgress{}, fakeOutdated{}, fakeAudit{}, fakeUsers{})
	ctx := context.Background()
	f.reports.rows = []ReportRow{{Name: "An"}}

	cases := []struct {
		name  string
		actor Actor
		want  error
	}{
		{"admin xem mọi lớp", Actor{ID: uuid.New(), Role: domain.RoleAdmin}, nil},
		{"giảng viên lớp mình", Actor{ID: f.teacherID, Role: domain.RoleTeacher}, nil},
		{"giảng viên lớp khác", Actor{ID: uuid.New(), Role: domain.RoleTeacher}, classes.ErrNotOwnClass},
		{"học viên", Actor{ID: uuid.New(), Role: domain.RoleStudent}, domain.ErrForbidden},
	}
	for _, tc := range cases {
		r, err := f.svc.ClassReport(ctx, tc.actor, f.classID, Filter{Sort: SortName})
		if tc.want != nil {
			assert.ErrorIs(t, err, tc.want, tc.name)
			continue
		}
		require.NoError(t, err, tc.name)
		assert.True(t, r.SelfReported, tc.name)
		assert.Len(t, r.Rows, 1, tc.name)
	}
	_, err := f.svc.ClassReport(ctx, Actor{ID: uuid.New(), Role: domain.RoleTeacher}, f.classID, Filter{Sort: SortName})
	assert.Equal(t, "Bạn chỉ xem được lớp mình phụ trách.", err.Error())

	f.reports.header = nil
	_, err = f.svc.ClassReport(ctx, Actor{ID: uuid.New(), Role: domain.RoleAdmin}, f.classID, Filter{Sort: SortName})
	assert.ErrorIs(t, err, classes.ErrClassNotFound)
}

func TestClassReportValidatesBeforeReading(t *testing.T) {
	f := newServiceFixture(fakeProgress{}, fakeOutdated{}, fakeAudit{}, fakeUsers{})
	_, err := f.svc.ClassReport(context.Background(), Actor{Role: domain.RoleAdmin}, f.classID, Filter{InactiveDays: intp(0), Sort: SortName})
	assert.ErrorIs(t, err, ErrInvalidFilter)
	assert.Zero(t, f.tx.calls)
	assert.Zero(t, f.reports.rowsCalls)
}

func TestClassReportSummaryThresholds(t *testing.T) {
	f := newServiceFixture(fakeProgress{}, fakeOutdated{}, fakeAudit{}, fakeUsers{})
	admin := Actor{ID: uuid.New(), Role: domain.RoleAdmin}

	r, err := f.svc.ClassReport(context.Background(), admin, f.classID, Filter{Sort: SortName})
	require.NoError(t, err)
	assert.Equal(t, SummaryQuery{InactiveCutoff: testNow.Add(-7 * 24 * time.Hour), BelowPercent: 50}, *f.reports.summaryQ)
	assert.Equal(t, 7, r.Summary.InactiveDays)
	assert.Equal(t, 50, r.Summary.BelowPercent)

	r, err = f.svc.ClassReport(context.Background(), admin, f.classID,
		Filter{InactiveDays: intp(14), BelowPercent: intp(30), IncludeDropped: true, Sort: SortPct})
	require.NoError(t, err)
	assert.Equal(t, SummaryQuery{IncludeDropped: true, InactiveCutoff: testNow.Add(-14 * 24 * time.Hour), BelowPercent: 30}, *f.reports.summaryQ)
	assert.Equal(t, 14, r.Summary.InactiveDays)
	assert.Equal(t, 30, r.Summary.BelowPercent)
	assert.Equal(t, SortPct, r.Filter.Sort)
}

func TestMemberReport(t *testing.T) {
	roadmap := learning.Roadmap{Stages: []learning.StageView{{Code: "DB", Name: "Database"}}}
	f := newServiceFixture(fakeProgress{roadmap: roadmap}, fakeOutdated{}, fakeAudit{}, fakeUsers{})
	ctx := context.Background()
	teacher := Actor{ID: f.teacherID, Role: domain.RoleTeacher}

	_, err := f.svc.MemberReport(ctx, teacher, f.classID, uuid.New())
	assert.ErrorIs(t, err, classes.ErrMemberNotFound, "thành viên không thuộc lớp")

	f.reports.member = &ReportRow{Name: "An"}
	r, err := f.svc.MemberReport(ctx, teacher, f.classID, uuid.New())
	require.NoError(t, err)
	assert.True(t, r.SelfReported)
	assert.Equal(t, "An", r.Member.Name)
	assert.Equal(t, roadmap.Stages, r.Stages)

	_, err = f.svc.MemberReport(ctx, Actor{ID: uuid.New(), Role: domain.RoleTeacher}, f.classID, uuid.New())
	assert.ErrorIs(t, err, classes.ErrNotOwnClass)
}

func TestDashboard(t *testing.T) {
	adminID, courseID, c1, c2, classID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	email, err := domain.ParseEmail("quan.tran@goup.vn")
	require.NoError(t, err)
	courseVersionID, latestStageVersionID := uuid.New(), uuid.New()
	outdated := fakeOutdated{rows: []stages.OutdatedCourse{
		{CourseID: courseID, CourseCode: "BASIC", StageCode: "DB", UsingVersionNo: 1, LatestVersionNo: 2},
		{
			CourseID: courseID, CourseCode: "BASIC", CourseVersionID: courseVersionID, CourseVersionNo: 2,
			StageCode: "GO", UsingVersionNo: 1, LatestVersionID: latestStageVersionID, LatestVersionNo: 3,
		},
	}}
	au := fakeAudit{entries: []audit.LoggedEntry{
		{ID: uuid.New(), At: testNow, ActorID: &adminID, Action: audit.ActionClassActivated, TargetType: targetClass, TargetID: &classID},
		{ID: uuid.New(), At: testNow, Action: audit.ActionClassEnded, TargetType: targetClass, TargetID: &classID},
	}}
	users := fakeUsers{users: map[uuid.UUID]identity.UserSnapshot{adminID: {ID: adminID, Name: "Quân Trần", Email: email}}}
	f := newServiceFixture(fakeProgress{avg: map[uuid.UUID]int{c1: 42}}, outdated, au, users)
	f.dashboard.classes = []ClassRow{{ID: c1, Code: "basic01"}, {ID: c2, Code: "basic03"}}
	f.dashboard.targets = map[TargetRef]TargetInfo{{targetClass, classID}: {Name: "basic02"}}
	ctx := context.Background()

	_, err = f.svc.Dashboard(ctx, Actor{ID: uuid.New(), Role: domain.RoleTeacher})
	assert.ErrorIs(t, err, domain.ErrForbidden)

	d, err := f.svc.Dashboard(ctx, Actor{ID: adminID, Role: domain.RoleAdmin})
	require.NoError(t, err)
	assert.Equal(t, KPIs{Stages: 5, Courses: 1, Classes: 2, Students: 11}, d.KPIs)
	assert.Equal(t, Hints{DraftClasses: 1, NotLoggedIn: 2, FailedInvites: 1, OutdatedCourses: 1}, d.Hints)
	require.Len(t, d.Outdated, 2)
	assert.Equal(t, OutdatedRow{
		CourseID: courseID, CourseCode: "BASIC", CourseVersionID: courseVersionID, CourseVersionNo: 2,
		StageCode: "GO", CurrentVersionNo: 1, LatestVersionID: latestStageVersionID, LatestPublishedNo: 3,
	}, d.Outdated[1])
	assert.Equal(t, 42, d.Classes[0].AvgPercent)
	assert.Equal(t, 0, d.Classes[1].AvgPercent, "lớp không có thành viên active")
	require.Len(t, d.RecentActivity, 2)
	assert.Equal(t, "Quân Trần", d.RecentActivity[0].ActorName)
	assert.Equal(t, "Kích hoạt lớp", d.RecentActivity[0].ActionLabel)
	assert.Equal(t, "basic02", d.RecentActivity[0].Target.Label)
	assert.Equal(t, "Hệ thống", d.RecentActivity[1].ActorName)

	f.svc.outdated = fakeOutdated{err: errors.New("db down")}
	_, err = f.svc.Dashboard(ctx, Actor{ID: adminID, Role: domain.RoleAdmin})
	assert.Error(t, err)
}
