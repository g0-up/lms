//go:build integration

package courses

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/db"
)

// applyEnv là kịch bản áp dụng DB v2: BASIC v1 [DB v1, WEB v1], HASDRAFT có bản nháp, WEBONLY không chứa chặng DB,
// DBV2 đã dùng DB v2.
type applyEnv struct {
	*itEnv
	webID, webV1                  uuid.UUID
	hasDraft, webOnly, dbV2Course uuid.UUID
}

func applyScenario(t *testing.T) *applyEnv {
	t.Helper()
	e := &applyEnv{itEnv: newIT(t, "stage_db_draft")}
	e.publishDBV2()
	e.webID, e.webV1 = e.createStage("WEB")
	e.exec(`INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position) VALUES ($1, $2, $3, 2)`,
		basicV1ID, e.webV1, e.webID)
	db1 := stageRef{stageDBID, dbV1ID}
	e.hasDraft, _ = e.insertCourse("HASDRAFT", cvSpec{domain.VersionPublished, []stageRef{db1}}, cvSpec{domain.VersionDraft, []stageRef{db1}})
	e.webOnly, _ = e.insertCourse("WEBONLY", cvSpec{domain.VersionPublished, []stageRef{{e.webID, e.webV1}}})
	e.dbV2Course, _ = e.insertCourse("DBV2", cvSpec{domain.VersionPublished, []stageRef{{stageDBID, dbV2ID}}})
	return e
}

// failingAudit ghi nhật ký thật nhưng lỗi ở một action, để mô phỏng lỗi giữa transaction sau khi đã ghi phiên bản.
type failingAudit struct {
	audit.PG
	action string
}

func (f failingAudit) Record(ctx context.Context, tx db.Executor, e audit.Entry) error {
	if e.Action == f.action {
		return errors.New("audit: lỗi giả lập")
	}
	return f.PG.Record(ctx, tx, e)
}

func TestApplyStageVersionPerCourse(t *testing.T) {
	e := applyScenario(t)
	snapshots := map[uuid.UUID]string{}
	for _, id := range []uuid.UUID{e.hasDraft, e.webOnly, e.dbV2Course} {
		snapshots[id] = e.courseSnapshot(id)
	}
	basicV1Before := e.versionSnapshot(basicV1ID)

	results, err := e.svc.ApplyStageVersion(e.ctx(), e.actor, dbV2ID, []uuid.UUID{basicID, e.hasDraft, e.webOnly, e.dbV2Course})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("results = %+v", results)
	}
	basic := results[0]
	if basic.Error != nil || basic.NewVersionNo == nil || *basic.NewVersionNo != 2 || basic.CourseCode != "BASIC" {
		t.Fatalf("BASIC = %+v (err %v)", basic, basic.Error)
	}
	wantErr := []struct {
		code, msg string
		status    int
	}{
		{apperr.CodeDraftExists, "Khóa học đang có bản nháp v2. Phát hành hoặc xóa bản nháp trước.", http.StatusConflict},
		{apperr.CodeValidationFailed, "Khóa học không chứa chặng này.", http.StatusUnprocessableEntity},
		{apperr.CodeConflict, "Khóa học đã dùng phiên bản này.", http.StatusConflict},
	}
	for i, w := range wantErr {
		r := results[i+1]
		if r.NewVersionNo != nil || r.Error == nil || r.Error.Code != w.code || r.Error.Message != w.msg || r.Error.Status != w.status {
			t.Fatalf("kết quả %d (%s) = %+v, lỗi %+v", i+1, r.CourseCode, r, r.Error)
		}
	}

	v2, err := e.svc.versions.LatestPublished(e.ctx(), e.dbx, basicID)
	if err != nil {
		t.Fatal(err)
	}
	if v2.VersionNo() != 2 || v2.Status() != domain.VersionPublished || v2.PublishedAt() == nil ||
		v2.ClonedFromID() == nil || *v2.ClonedFromID() != basicV1ID {
		t.Fatalf("BASIC v2 = no %d status %s from %v", v2.VersionNo(), v2.Status(), v2.ClonedFromID())
	}
	got := v2.Stages()
	if len(got) != 2 || got[0] != (CourseVersionStage{stageDBID, dbV2ID, 1}) || got[1] != (CourseVersionStage{e.webID, e.webV1, 2}) {
		t.Fatalf("BASIC v2 stages = %+v", got)
	}
	if e.versionSnapshot(basicV1ID) != basicV1Before {
		t.Fatal("BASIC v1 bị đổi")
	}
	for id, before := range snapshots {
		if e.courseSnapshot(id) != before {
			t.Fatalf("khóa học lỗi %s bị đổi", id)
		}
	}
	if n := e.count(`SELECT count(*) FROM classes WHERE id = $1 AND course_version_id = $2`, basic01ID, basicV1ID); n != 1 {
		t.Fatal("lớp basic01 không còn trỏ BASIC v1")
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = 'course.stage_version_applied'`); n != 1 {
		t.Fatalf("audit applied = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = 'course.stage_version_applied' AND target_id = $1
		AND after->>'toStageVersionId' = $2 AND after->>'fromStageVersionId' = $3`, basicID, dbV2ID.String(), dbV1ID.String()); n != 1 {
		t.Fatal("audit applied không trỏ BASIC với DB v1 → v2")
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action IN ('course_version.cloned', 'course_version.published')`); n != 2 {
		t.Fatalf("audit cloned/published = %d", n)
	}
}

func TestApplyStageVersionRollsBackCourseOnMidTxFailure(t *testing.T) {
	e := applyScenario(t)
	before := e.courseSnapshot(basicID)
	audits := e.count(`SELECT count(*) FROM audit_logs`)
	svc := e.service(failingAudit{PG: audit.PG{Clock: e.clk}, action: audit.ActionCourseStageVersionApplied})

	results, err := svc.ApplyStageVersion(e.ctx(), e.actor, dbV2ID, []uuid.UUID{basicID})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].NewVersionNo != nil || results[0].Error == nil || results[0].Error.Code != apperr.CodeInternal {
		t.Fatalf("results = %+v", results)
	}
	if e.courseSnapshot(basicID) != before || e.count(`SELECT count(*) FROM course_versions WHERE course_id = $1`, basicID) != 1 {
		t.Fatal("BASIC còn phiên bản ghi dở sau rollback")
	}
	if n := e.count(`SELECT count(*) FROM audit_logs`); n != audits {
		t.Fatalf("audit_logs %d → %d sau rollback", audits, n)
	}
}

func TestApplyUnpublishedSourceChangesNothing(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	before := e.courseSnapshot(basicID)
	if _, err := e.svc.ApplyStageVersion(e.ctx(), e.actor, dbV2ID, []uuid.UUID{basicID}); !errors.Is(err, ErrSourceNotPublished) {
		t.Fatalf("ApplyStageVersion nguồn nháp = %v", err)
	}
	body := `{"courseIds":["` + basicID.String() + `"]}`
	e.call(e.router(), http.MethodPost, "/api/v1/stage-versions/"+dbV2ID.String()+"/apply", body, http.StatusUnprocessableEntity)
	if e.courseSnapshot(basicID) != before {
		t.Fatal("BASIC bị đổi khi nguồn chưa phát hành")
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action LIKE 'course%'`); n != 0 {
		t.Fatalf("audit = %d", n)
	}
}
