//go:build integration

package courses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/testdb"
)

var update = flag.Bool("update", false, "ghi lại golden JSON trong testdata/")

var (
	adminID   = uuid.MustParse(testdb.AdminQuanTranID)
	teacherID = uuid.MustParse(testdb.TeacherHuongLeID)
	stageDBID = uuid.MustParse(testdb.StageDBID)
	dbV1ID    = uuid.MustParse(testdb.StageDBV1ID)
	dbV2ID    = uuid.MustParse(testdb.StageDBV2ID)
	basicID   = uuid.MustParse(testdb.CourseBasicID)
	basicV1ID = uuid.MustParse(testdb.CourseBasicV1ID)
	basic01ID = uuid.MustParse(testdb.ClassBasic01ID)
)

// stageRefs là StageVersionReader trên repository thật của stages, cùng ánh xạ với adapter ở app/deps.go.
type stageRefs struct{}

func (stageRefs) Refs(ctx context.Context, ex db.Executor, versionIDs []uuid.UUID) (map[uuid.UUID]StageVersionRef, error) {
	refs, err := stages.PGStageVersionRepo{}.Refs(ctx, ex, versionIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]StageVersionRef, len(refs))
	for id, v := range refs {
		out[id] = StageVersionRef{ID: v.ID, StageID: v.StageID, StageCode: v.StageCode, VersionNo: v.VersionNo, Status: v.Status}
	}
	return out, nil
}

type itEnv struct {
	t     *testing.T
	dbx   *sqlx.DB
	repo  PGCourseVersionRepo
	clk   *clock.Fake
	svc   *Service
	actor Actor
}

// newIT dọn database rồi nạp fixture seed.js (chặng DB v1, khóa học BASIC v1, lớp basic01 đang chạy). Test ghi thật
// (commit) qua service; câu ghi trực tiếp lên repository dùng inTx và luôn rollback.
func newIT(t *testing.T, fixtures ...string) *itEnv {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	for _, f := range append([]string{"class_basic01_active"}, fixtures...) {
		testdb.Fixture(t, dbx, f)
	}
	e := &itEnv{t: t, dbx: dbx, clk: &clock.Fake{T: time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)}, actor: Actor{ID: adminID, RequestID: "it"}}
	e.svc = e.service(audit.PG{Clock: e.clk})
	return e
}

func (e *itEnv) service(rec audit.Recorder) *Service {
	return NewService(Deps{
		DB: e.dbx, Tx: db.TxRunner{DB: e.dbx}, Courses: PGCourseRepo{}, Versions: PGCourseVersionRepo{},
		StageVersions: stageRefs{}, Clock: e.clk, Audit: rec, IDs: ids.V7{},
	})
}

func (e *itEnv) ctx() context.Context { return context.Background() }

// inTx chạy fn trong transaction thật và luôn rollback, để thử câu ghi trực tiếp lên repository.
func (e *itEnv) inTx(fn func(tx db.Executor) error) error {
	errRollback := errors.New("rollback")
	err := db.Transact(e.ctx(), e.dbx, func(tx db.Executor) error {
		if err := fn(tx); err != nil {
			return err
		}
		return errRollback
	})
	if errors.Is(err, errRollback) {
		return nil
	}
	return err
}

func (e *itEnv) exec(q string, args ...any) {
	e.t.Helper()
	if _, err := e.dbx.Exec(q, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *itEnv) count(q string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.dbx.Get(&n, q, args...); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// createStage tạo chặng với v1 đã phát hành (không học liệu); trả id chặng và id phiên bản.
func (e *itEnv) createStage(code string) (uuid.UUID, uuid.UUID) {
	e.t.Helper()
	stageID, vid := ids.New(), ids.New()
	e.exec(`INSERT INTO stages (id, code, name, created_by) VALUES ($1, $2, $3, $4)`, stageID, code, "Chặng "+code, adminID)
	e.exec(`INSERT INTO stage_versions (id, stage_id, version_no, status, title, published_at, created_by)
		VALUES ($1, $2, 1, 'published', $3, $4, $5)`, vid, stageID, "Chặng "+code, e.clk.Now(), adminID)
	return stageID, vid
}

// publishDBV2 phát hành DB v2 của fixture stage_db_draft (render bài còn thiếu HTML rồi đổi trạng thái header).
func (e *itEnv) publishDBV2() {
	e.t.Helper()
	e.exec(`UPDATE lessons SET markdown_html = '<p>Đã render.</p>' WHERE stage_version_id = $1 AND markdown_html IS NULL`, dbV2ID)
	e.exec(`UPDATE stage_versions SET status = 'published', published_at = $2 WHERE id = $1`, dbV2ID, e.clk.Now())
}

type stageRef struct{ stageID, versionID uuid.UUID }

type cvSpec struct {
	status domain.VersionStatus
	stages []stageRef
}

// insertCourse tạo khóa học với các phiên bản theo thứ tự v1, v2…; trả id khóa học và id từng phiên bản.
func (e *itEnv) insertCourse(code string, versions ...cvSpec) (uuid.UUID, []uuid.UUID) {
	e.t.Helper()
	courseID := ids.New()
	e.exec(`INSERT INTO courses (id, code, name, created_by) VALUES ($1, $2, $3, $4)`, courseID, code, "Khóa "+code, adminID)
	out := make([]uuid.UUID, 0, len(versions))
	for i, v := range versions {
		cvID := ids.New()
		var publishedAt *time.Time
		if v.status != domain.VersionDraft {
			now := e.clk.Now()
			publishedAt = &now
		}
		e.exec(`INSERT INTO course_versions (id, course_id, version_no, status, title, published_at, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, cvID, courseID, i+1, v.status.String(), "Khóa "+code, publishedAt, adminID)
		for p, s := range v.stages {
			e.exec(`INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position)
				VALUES ($1, $2, $3, $4)`, cvID, s.versionID, s.stageID, p+1)
		}
		out = append(out, cvID)
	}
	return courseID, out
}

// versionSnapshot là header và danh sách chặng của một phiên bản dưới dạng JSON để so trước/sau.
func (e *itEnv) versionSnapshot(vid uuid.UUID) string {
	e.t.Helper()
	var s string
	if err := e.dbx.Get(&s, `SELECT json_build_object(
		'header', (SELECT row_to_json(cv) FROM course_versions cv WHERE cv.id = $1),
		'stages', COALESCE((SELECT json_agg(x ORDER BY x.position) FROM course_version_stages x WHERE x.course_version_id = $1), '[]'))::text`, vid); err != nil {
		e.t.Fatal(err)
	}
	return s
}

// courseSnapshot là mọi phiên bản (kèm chặng) của một khóa học.
func (e *itEnv) courseSnapshot(courseID uuid.UUID) string {
	e.t.Helper()
	var s string
	if err := e.dbx.Get(&s, `SELECT COALESCE(json_agg(json_build_object('header', row_to_json(cv),
		'stages', COALESCE((SELECT json_agg(x ORDER BY x.position) FROM course_version_stages x WHERE x.course_version_id = cv.id), '[]'))
		ORDER BY cv.version_no), '[]')::text FROM course_versions cv WHERE cv.course_id = $1`, courseID); err != nil {
		e.t.Fatal(err)
	}
	return s
}

// newDraftFor tạo bản nháp (chưa lưu) cho khóa học với danh sách chặng đặt thẳng, bỏ qua kiểm tra domain để thử
// constraint của DB.
func newDraftFor(courseID uuid.UUID, no domain.VersionNo, stages ...CourseVersionStage) *CourseVersion {
	v := NewDraftCourseVersion(ids.New(), courseID, no, "Bản nháp", "", nil, adminID)
	v.stages = stages
	return v
}

func TestOneDraftPerCourse(t *testing.T) {
	e := newIT(t)
	err := e.inTx(func(tx db.Executor) error {
		first := newDraftFor(basicID, 2, CourseVersionStage{StageID: stageDBID, StageVersionID: dbV1ID, Position: 1})
		if err := e.repo.Create(e.ctx(), tx, first); err != nil {
			return err
		}
		err := e.repo.Create(e.ctx(), tx, newDraftFor(basicID, 3))
		var de *ErrDraftExists
		if !errors.As(err, &de) || de.DraftID != first.ID() || de.No != 2 {
			t.Fatalf("bản nháp thứ hai = %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateStageRejected(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	e.publishDBV2()
	tests := []struct {
		name   string
		stages []CourseVersionStage
	}{
		{"hai phiên bản cùng chặng", []CourseVersionStage{{stageDBID, dbV1ID, 1}, {stageDBID, dbV2ID, 2}}},
		{"cùng một phiên bản hai lần", []CourseVersionStage{{stageDBID, dbV1ID, 1}, {stageDBID, dbV1ID, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.inTx(func(tx db.Executor) error {
				return e.repo.Create(e.ctx(), tx, newDraftFor(basicID, 2, tt.stages...))
			})
			if !errors.Is(err, ErrDuplicateStage) {
				t.Fatalf("Create = %v, muốn ErrDuplicateStage", err)
			}
		})
	}
}

func TestCompositeFKRejectsStageMismatch(t *testing.T) {
	e := newIT(t)
	webID, _ := e.createStage("WEB")
	err := e.inTx(func(tx db.Executor) error {
		return e.repo.Create(e.ctx(), tx, newDraftFor(basicID, 2, CourseVersionStage{StageID: webID, StageVersionID: dbV1ID, Position: 1}))
	})
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != pgerrcode.ForeignKeyViolation || pg.ConstraintName != pgerr.FkCvsStageVersion {
		t.Fatalf("Create stage_id lệch = %v", err)
	}
}

func TestReorderSwapsWithoutUniqueViolation(t *testing.T) {
	e := newIT(t)
	webID, webV1 := e.createStage("WEB")
	err := e.inTx(func(tx db.Executor) error {
		v := newDraftFor(basicID, 2, CourseVersionStage{stageDBID, dbV1ID, 1}, CourseVersionStage{webID, webV1, 2})
		if err := e.repo.Create(e.ctx(), tx, v); err != nil {
			return err
		}
		v.stages = []CourseVersionStage{{webID, webV1, 1}, {stageDBID, dbV1ID, 2}}
		if err := e.repo.SaveDraft(e.ctx(), tx, v); err != nil {
			return err
		}
		got, err := e.repo.ByID(e.ctx(), tx, v.ID())
		if err != nil {
			return err
		}
		if ids := got.StageVersionIDs(); len(ids) != 2 || ids[0] != webV1 || ids[1] != dbV1ID {
			t.Fatalf("sau sắp lại = %v", ids)
		}
		// Hai chặng trùng vị trí vẫn bị uq_cvs_version_position chặn khi transaction kết thúc kiểm tra.
		v.stages = []CourseVersionStage{{webID, webV1, 1}, {stageDBID, dbV1ID, 1}}
		if err := e.repo.SaveDraft(e.ctx(), tx, v); err != nil {
			return err
		}
		_, err = tx.ExecContext(e.ctx(), `SET CONSTRAINTS `+pgerr.UqCvsVersionPosition+` IMMEDIATE`)
		return err
	})
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != pgerrcode.UniqueViolation || pg.ConstraintName != pgerr.UqCvsVersionPosition {
		t.Fatalf("trùng vị trí = %v", err)
	}
}

func TestClassesRestrictDeletingVersion(t *testing.T) {
	e := newIT(t)
	err := e.inTx(func(tx db.Executor) error {
		v := newDraftFor(basicID, 2, CourseVersionStage{stageDBID, dbV1ID, 1})
		if err := e.repo.Create(e.ctx(), tx, v); err != nil {
			return err
		}
		if _, err := tx.ExecContext(e.ctx(), `INSERT INTO classes (id, code, name, course_version_id, teacher_id, status, start_date, end_date, created_by)
			VALUES ($1, 'basic02', 'Lớp nháp', $2, $3, 'draft', current_date, current_date + 30, $4)`, ids.New(), v.ID(), teacherID, adminID); err != nil {
			return err
		}
		return e.repo.Delete(e.ctx(), tx, v.ID())
	})
	if !errors.Is(err, domain.ErrInUse) {
		t.Fatalf("Delete phiên bản lớp đang dùng = %v", err)
	}
}

// expectImmutable gọi fn trực tiếp trên repository (bỏ qua service) với BASIC v1 đã phát hành và kiểm dữ liệu không đổi.
func expectImmutable(t *testing.T, want error, fn func(e *itEnv, tx db.Executor, v *CourseVersion) error) {
	t.Helper()
	e := newIT(t)
	webID, webV1 := e.createStage("WEB")
	before := e.versionSnapshot(basicV1ID)
	err := e.inTx(func(tx db.Executor) error {
		v, err := e.repo.ByID(e.ctx(), tx, basicV1ID)
		if err != nil {
			return err
		}
		v.stages = append(v.stages, CourseVersionStage{StageID: webID, StageVersionID: webV1, Position: 2})
		return fn(e, tx, v)
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, muốn %v", err, want)
	}
	if after := e.versionSnapshot(basicV1ID); after != before {
		t.Fatalf("bản published bị đổi:\ntrước %s\nsau   %s", before, after)
	}
}

func TestImmutability_SaveDraftStages(t *testing.T) {
	expectImmutable(t, ErrVersionImmutable, func(e *itEnv, tx db.Executor, v *CourseVersion) error {
		return e.repo.SaveDraft(e.ctx(), tx, v)
	})
}

func TestImmutability_InsertStage(t *testing.T) {
	expectImmutable(t, ErrVersionImmutable, func(e *itEnv, tx db.Executor, v *CourseVersion) error {
		return insertStage(e.ctx(), tx, v.ID(), v.stages[1])
	})
}

func TestImmutability_DeleteStages(t *testing.T) {
	expectImmutable(t, ErrVersionImmutable, func(e *itEnv, tx db.Executor, v *CourseVersion) error {
		return deleteStages(e.ctx(), tx, v.ID())
	})
}

func TestImmutability_DeleteHeader(t *testing.T) {
	expectImmutable(t, ErrVersionImmutable, func(e *itEnv, tx db.Executor, v *CourseVersion) error {
		return e.repo.Delete(e.ctx(), tx, v.ID())
	})
}

func TestImmutability_CreatePublishedHeader(t *testing.T) {
	expectImmutable(t, ErrVersionImmutable, func(e *itEnv, tx db.Executor, v *CourseVersion) error {
		c := newDraftFor(basicID, 2, v.stages...)
		c.status = domain.VersionPublished
		return e.repo.Create(e.ctx(), tx, c)
	})
}

func TestImmutability_TransitionBackToDraft(t *testing.T) {
	expectImmutable(t, domain.ErrInvalidTransition, func(e *itEnv, tx db.Executor, v *CourseVersion) error {
		return e.repo.TransitionStatus(e.ctx(), tx, v.ID(), domain.VersionPublished, domain.VersionDraft, nil)
	})
}

func TestImmutability_RepublishPublished(t *testing.T) {
	now := time.Now()
	expectImmutable(t, ErrPublishNotDraft, func(e *itEnv, tx db.Executor, v *CourseVersion) error {
		return e.repo.TransitionStatus(e.ctx(), tx, v.ID(), domain.VersionDraft, domain.VersionPublished, &now)
	})
}

func TestImmutability_ServiceMutations(t *testing.T) {
	e := newIT(t)
	before := e.versionSnapshot(basicV1ID)
	if _, err := e.svc.SetStages(e.ctx(), e.actor, basicV1ID, []uuid.UUID{dbV1ID}); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("SetStages = %v", err)
	}
	if _, err := e.svc.Publish(e.ctx(), e.actor, basicV1ID); !errors.Is(err, ErrPublishNotDraft) {
		t.Fatalf("Publish = %v", err)
	}
	if after := e.versionSnapshot(basicV1ID); after != before {
		t.Fatalf("bản published bị đổi:\ntrước %s\nsau   %s", before, after)
	}
}

func TestCreateCourseAuditAndDeleteLastDraft(t *testing.T) {
	e := newIT(t)
	d, err := e.svc.CreateCourse(e.ctx(), e.actor, CreateCourseCmd{Code: "DEMO", Name: "Khóa demo", Description: "Mô tả"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Versions) != 1 || d.Versions[0].Status != domain.VersionDraft || d.Versions[0].VersionNo != 1 || d.Course.Description != "Mô tả" {
		t.Fatalf("CreateCourse = %+v", d)
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = 'course.created' AND target_id = $1
		AND after->>'code' = 'DEMO' AND actor_id = $2`, d.Course.ID, adminID); n != 1 {
		t.Fatalf("audit course.created = %d", n)
	}
	if _, err := e.svc.CreateCourse(e.ctx(), e.actor, CreateCourseCmd{Code: "BASIC", Name: "Trùng"}); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("trùng mã = %v", err)
	}
	deleted, err := e.svc.Delete(e.ctx(), e.actor, d.Versions[0].ID)
	if err != nil || !deleted {
		t.Fatalf("Delete = %v, %v", deleted, err)
	}
	if n := e.count(`SELECT count(*) FROM courses WHERE id = $1`, d.Course.ID); n != 0 {
		t.Fatal("khóa học còn sau khi xóa bản nháp cuối")
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = 'course_version.deleted' AND (after->>'courseDeleted')::boolean`); n != 1 {
		t.Fatalf("audit course_version.deleted = %d", n)
	}
}

func TestPublishAndArchiveTouchHeaderOnly(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	d, err := e.svc.CreateCourse(e.ctx(), e.actor, CreateCourseCmd{Code: "DEMO", Name: "Khóa demo"})
	if err != nil {
		t.Fatal(err)
	}
	vid := d.Versions[0].ID
	if _, err := e.svc.Publish(e.ctx(), e.actor, vid); !errors.Is(err, ErrNoStages) {
		t.Fatalf("Publish rỗng = %v", err)
	}
	if _, err := e.svc.SetStages(e.ctx(), e.actor, vid, []uuid.UUID{dbV2ID}); !errors.Is(err, ErrStageVersionNotPublished) {
		t.Fatalf("SetStages ref nháp = %v", err)
	}
	if _, err := e.svc.SetStages(e.ctx(), e.actor, vid, []uuid.UUID{dbV1ID}); err != nil {
		t.Fatal(err)
	}
	stagesOf := func() string {
		var s string
		if err := e.dbx.Get(&s, `SELECT COALESCE(json_agg(x ORDER BY x.position), '[]')::text FROM course_version_stages x
			WHERE x.course_version_id = $1`, vid); err != nil {
			t.Fatal(err)
		}
		return s
	}
	before := stagesOf()

	pub, err := e.svc.Publish(e.ctx(), e.actor, vid)
	if err != nil {
		t.Fatal(err)
	}
	if pub.Version.Status() != domain.VersionPublished || pub.Version.PublishedAt() == nil || stagesOf() != before {
		t.Fatalf("Publish = status %s publishedAt %v", pub.Version.Status(), pub.Version.PublishedAt())
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = 'course_version.published' AND target_id = $1
		AND after->'stageVersionIds' = $2::jsonb`, vid, `["`+dbV1ID.String()+`"]`); n != 1 {
		t.Fatalf("audit published = %d", n)
	}

	arch, err := e.svc.Archive(e.ctx(), e.actor, vid)
	if err != nil {
		t.Fatal(err)
	}
	if arch.Version.Status() != domain.VersionArchived || arch.Version.ArchivedAt() == nil || stagesOf() != before {
		t.Fatalf("Archive = status %s archivedAt %v", arch.Version.Status(), arch.Version.ArchivedAt())
	}
	if _, err := e.svc.Archive(e.ctx(), e.actor, vid); !errors.Is(err, ErrArchiveNotPublished) {
		t.Fatalf("Archive lần hai = %v", err)
	}
}

func TestCloneKeepsRefsAndSecondCloneIsDraftExists(t *testing.T) {
	e := newIT(t)
	c, err := e.svc.Clone(e.ctx(), e.actor, basicV1ID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Version.VersionNo() != 2 || c.Version.Status() != domain.VersionDraft || c.ClonedFromVersionNo == nil || *c.ClonedFromVersionNo != 1 {
		t.Fatalf("clone = no %d status %s from %v", c.Version.VersionNo(), c.Version.Status(), c.ClonedFromVersionNo)
	}
	if got := c.Version.StageVersionIDs(); len(got) != 1 || got[0] != dbV1ID {
		t.Fatalf("clone refs = %v", got)
	}
	_, err = e.svc.Clone(e.ctx(), e.actor, basicV1ID)
	var de *ErrDraftExists
	if !errors.As(err, &de) || de.DraftID != c.Version.ID() || de.No != 2 {
		t.Fatalf("clone lần hai = %v", err)
	}
	if _, err := e.svc.Clone(e.ctx(), e.actor, c.Version.ID()); !errors.Is(err, ErrCloneNotPublished) {
		t.Fatalf("clone từ bản nháp = %v", err)
	}
}

func TestDeletePublishedInUse(t *testing.T) {
	e := newIT(t)
	_, err := e.svc.Delete(e.ctx(), e.actor, basicV1ID)
	var iu *ErrInUse
	if !errors.As(err, &iu) || len(iu.UsedBy) != 1 || iu.UsedBy[0].Code != "basic01" || iu.UsedBy[0].MemberCount != 1 || iu.UsedBy[0].Status != "active" {
		t.Fatalf("Delete published = %v", err)
	}
	if e.count(`SELECT count(*) FROM course_versions WHERE id = $1`, basicV1ID) != 1 {
		t.Fatal("bản published bị xóa")
	}
}

func TestPublishedVersionReader(t *testing.T) {
	e := newIT(t)
	var reader VersionReader = e.svc
	s, err := reader.PublishedVersion(e.ctx(), e.dbx, basicV1ID)
	if err != nil || s.CourseCode != "BASIC" || s.CourseName != "Lập trình cơ bản" || s.VersionNo != 1 || s.CourseID != basicID {
		t.Fatalf("PublishedVersion = %+v, %v", s, err)
	}
	c, err := e.svc.Clone(e.ctx(), e.actor, basicV1ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.PublishedVersion(e.ctx(), e.dbx, c.Version.ID()); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("PublishedVersion bản nháp = %v", err)
	}
	if _, err := reader.PublishedVersion(e.ctx(), e.dbx, ids.New()); !errors.Is(err, ErrVersionNotFound) {
		t.Fatalf("PublishedVersion không tồn tại = %v", err)
	}
}

func TestListAndDetailCounts(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	e.publishDBV2()
	rows, err := e.svc.ListCourses(e.ctx(), ListQuery{Q: "cơ bản"})
	if err != nil || len(rows) != 1 || rows[0].Code != "BASIC" || rows[0].Description != "Lộ trình nhập môn lập trình." {
		t.Fatalf("ListCourses = %+v, %v", rows, err)
	}
	v := rows[0].Versions[0]
	if v.StageCount != 1 || v.OutdatedStageCount != 1 || v.ClassCount != 1 || len(rows[0].ClassesUsing) != 1 {
		t.Fatalf("version row = %+v", v)
	}
	if rows, err := e.svc.ListCourses(e.ctx(), ListQuery{Q: "100%"}); err != nil || len(rows) != 0 {
		t.Fatalf("ListCourses ký tự LIKE = %+v, %v", rows, err)
	}
	d, err := e.svc.GetVersion(e.ctx(), basicV1ID)
	if err != nil || len(d.Stages) != 1 || !d.Stages[0].Outdated() || d.Stages[0].LessonCount != 2 || d.Stages[0].RequiredCount != 1 {
		t.Fatalf("GetVersion = %+v, %v", d.Stages, err)
	}
}

// --- golden JSON qua HTTP handler ---

func (e *itEnv) router() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(e.svc, func(*gin.Context) (uuid.UUID, bool) { return adminID, true }).Register(r.Group("/api/v1"))
	return r
}

func (e *itEnv) call(r http.Handler, method, path, body string, wantStatus int) []byte {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != wantStatus {
		e.t.Fatalf("%s %s = %d, muốn %d: %s", method, path, w.Code, wantStatus, w.Body)
	}
	return w.Body.Bytes()
}

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizeGolden thay thời điểm (khóa kết thúc bằng "At") bằng một mốc cố định và id sinh lúc chạy (không phải id
// fixture 01990000-…) bằng id giả theo thứ tự xuất hiện, để golden ổn định giữa các lần chạy.
func normalizeGolden(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("golden: %v: %s", err, raw)
	}
	seen := map[string]string{}
	var walk func(key string, v any) any
	walk = func(key string, v any) any {
		switch x := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				x[k] = walk(k, x[k])
			}
			return x
		case []any:
			for i := range x {
				x[i] = walk(key, x[i])
			}
			return x
		case string:
			if strings.HasSuffix(key, "At") {
				return "2026-10-05T08:00:00Z"
			}
			return uuidPattern.ReplaceAllStringFunc(x, func(id string) string {
				if strings.HasPrefix(id, "01990000-") {
					return id
				}
				if p, ok := seen[id]; ok {
					return p
				}
				p := fmt.Sprintf("0199ffff-0000-7000-8000-%012d", len(seen)+1)
				seen[id] = p
				return p
			})
		default:
			return x
		}
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(walk("", v)); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func assertGolden(t *testing.T, name string, raw []byte) {
	t.Helper()
	got := normalizeGolden(t, raw)
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil { //nolint:gosec // golden JSON công khai trong repo
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // đường dẫn cố định trong testdata
	if err != nil {
		t.Fatalf("golden %s: %v (chạy lại với -update)", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden %s khác (chạy lại với -update nếu thay đổi là chủ ý):\n%s", name, got)
	}
}

func TestGoldenJSON(t *testing.T) {
	t.Run("course list, detail và version từ seed", func(t *testing.T) {
		e := newIT(t, "stage_db_draft")
		e.publishDBV2()
		r := e.router()
		basicV1 := "/api/v1/course-versions/" + basicV1ID.String()
		assertGolden(t, "course_list.json", e.call(r, http.MethodGet, "/api/v1/courses", "", http.StatusOK))
		assertGolden(t, "course_detail.json", e.call(r, http.MethodGet, "/api/v1/courses/"+basicID.String(), "", http.StatusOK))
		assertGolden(t, "course_version.json", e.call(r, http.MethodGet, basicV1, "", http.StatusOK))
		assertGolden(t, "error_in_use.json", e.call(r, http.MethodDelete, basicV1, "", http.StatusConflict))
		assertGolden(t, "error_version_immutable.json", e.call(r, http.MethodPut, basicV1+"/stages",
			`{"stageVersionIds":["`+dbV2ID.String()+`"]}`, http.StatusConflict))
	})
	t.Run("bản nháp clone, lỗi và xóa", func(t *testing.T) {
		e := newIT(t, "stage_db_draft")
		r := e.router()
		basicV1 := "/api/v1/course-versions/" + basicV1ID.String()
		draftJSON := e.call(r, http.MethodPost, basicV1+"/clone", "", http.StatusCreated)
		assertGolden(t, "course_version_draft.json", draftJSON)
		var draft CourseVersionDTO
		if err := json.Unmarshal(draftJSON, &draft); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "error_draft_exists.json", e.call(r, http.MethodPost, basicV1+"/clone", "", http.StatusConflict))
		assertGolden(t, "error_validation.json", e.call(r, http.MethodPut, "/api/v1/course-versions/"+draft.ID+"/stages",
			`{"stageVersionIds":["`+dbV2ID.String()+`"]}`, http.StatusUnprocessableEntity))
		assertGolden(t, "delete_version.json", e.call(r, http.MethodDelete, "/api/v1/course-versions/"+draft.ID, "", http.StatusOK))
	})
	t.Run("kết quả áp dụng phiên bản chặng", func(t *testing.T) {
		e := applyScenario(t)
		body := fmt.Sprintf(`{"courseIds":["%s","%s","%s","%s"]}`, basicID, e.hasDraft, e.webOnly, e.dbV2Course)
		assertGolden(t, "apply_results.json", e.call(e.router(), http.MethodPost, "/api/v1/stage-versions/"+dbV2ID.String()+"/apply", body, http.StatusOK))
	})
}
