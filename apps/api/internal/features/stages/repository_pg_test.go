//go:build integration

package stages

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
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/media"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/storage"
	"lms/api/internal/platform/testdb"
)

var update = flag.Bool("update", false, "ghi lại golden JSON trong testdata/")

var (
	adminID   = uuid.MustParse(testdb.AdminQuanTranID)
	stageDBID = uuid.MustParse(testdb.StageDBID)
	dbV1ID    = uuid.MustParse(testdb.StageDBV1ID)
	dbV2ID    = uuid.MustParse(testdb.StageDBV2ID)
	unitV2ID  = uuid.MustParse(testdb.UnitDBV2ID)
)

type itEnv struct {
	t     *testing.T
	dbx   *sqlx.DB
	repo  PGStageVersionRepo
	clk   *clock.Fake
	svc   *Service
	actor Actor
}

// newIT dọn database rồi nạp fixture seed.js (chặng DB v1, khóa học BASIC v1, lớp basic01). Test ghi thật (commit)
// vì phần song song cần hai kết nối.
func newIT(t *testing.T, fixtures ...string) *itEnv {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	for _, f := range append([]string{"class_basic01_active"}, fixtures...) {
		testdb.Fixture(t, dbx, f)
	}
	e := &itEnv{t: t, dbx: dbx, clk: &clock.Fake{T: time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)}, actor: Actor{ID: adminID, RequestID: "it"}}
	e.svc = e.service(PGStageVersionRepo{})
	return e
}

func (e *itEnv) service(versions StageVersionRepo) *Service {
	return NewService(Deps{
		DB: e.dbx, Tx: db.TxRunner{DB: e.dbx}, Stages: PGStageRepo{}, Versions: versions, Media: media.PGRepo{},
		Render: NewMarkdownRenderer(), Clock: e.clk, Audit: audit.PG{Clock: e.clk}, IDs: ids.V7{},
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

func (e *itEnv) addMedia(kind media.Kind, ct, name string) uuid.UUID {
	e.t.Helper()
	m, err := media.NewPendingUpload(ids.New(), kind, name, ct, 1024, adminID, e.clk.Now(),
		media.Limits{MaxVideoBytes: 1 << 30, MaxImageBytes: 1 << 20, MaxDocumentBytes: 1 << 20})
	if err != nil {
		e.t.Fatal(err)
	}
	if err := m.MarkReady(storage.ObjectStat{Size: 1024, ContentType: ct}, e.clk.Now()); err != nil {
		e.t.Fatal(err)
	}
	if err := (media.PGRepo{}).Create(e.ctx(), e.dbx, m); err != nil {
		e.t.Fatal(err)
	}
	return m.ID()
}

func (e *itEnv) createStage(code string) uuid.UUID {
	e.t.Helper()
	d, err := e.svc.CreateStage(e.ctx(), e.actor, CreateStageCmd{Code: code, Name: "Chặng " + code})
	if err != nil {
		e.t.Fatal(err)
	}
	return d.Versions[0].ID
}

func (e *itEnv) addUnit(vid uuid.UUID, title string) uuid.UUID {
	e.t.Helper()
	d, err := e.svc.AddUnit(e.ctx(), e.actor, vid, title)
	if err != nil {
		e.t.Fatal(err)
	}
	return d.Unit.ID()
}

// firstUnit trả bài học đầu của phiên bản, tạo bài học "Bài 1" khi phiên bản chưa có bài học nào.
func (e *itEnv) firstUnit(vid uuid.UUID) *uuid.UUID {
	e.t.Helper()
	v, err := e.repo.ByID(e.ctx(), e.dbx, vid)
	if err != nil {
		e.t.Fatal(err)
	}
	if units := v.Units(); len(units) > 0 {
		id := units[0].ID()
		return &id
	}
	id := e.addUnit(vid, "Bài 1")
	return &id
}

func (e *itEnv) addMarkdown(vid uuid.UUID, title, src string) *Lesson {
	e.t.Helper()
	return e.addMarkdownTo(vid, *e.firstUnit(vid), title, src)
}

func (e *itEnv) addMarkdownTo(vid, unitID uuid.UUID, title, src string) *Lesson {
	e.t.Helper()
	l, err := e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{UnitID: &unitID, Title: &title, Type: strp("markdown"), MarkdownSource: &src})
	if err != nil {
		e.t.Fatal(err)
	}
	return l
}

// addLegacyMarkdown thêm bài markdown rồi ghi đè nguồn thẳng bằng SQL, giả lập dữ liệu lưu trước khi có luật ảnh
// ngoài.
func (e *itEnv) addLegacyMarkdown(vid uuid.UUID, title, src string) *Lesson {
	e.t.Helper()
	l := e.addMarkdown(vid, title, "# Tạm")
	if _, err := e.dbx.Exec(`UPDATE lessons SET markdown_source = $1 WHERE id = $2`, src, l.ID()); err != nil {
		e.t.Fatal(err)
	}
	return l
}

func (e *itEnv) addVideo(vid uuid.UUID, title string, mediaID uuid.UUID, seconds int) *Lesson {
	e.t.Helper()
	l, err := e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{UnitID: e.firstUnit(vid), Title: &title, Type: strp("video"),
		VideoMediaID: &mediaID, DurationSeconds: &seconds})
	if err != nil {
		e.t.Fatal(err)
	}
	return l
}

// versionSnapshot là toàn bộ header, bài học, học liệu, nội dung quiz/homework và lesson_media của một phiên bản dưới
// dạng JSON để so trước/sau.
func (e *itEnv) versionSnapshot(vid uuid.UUID, withHeader bool) string {
	e.t.Helper()
	return e.snapshotOn(e.dbx, vid, withHeader)
}

// snapshotOn như versionSnapshot nhưng đọc qua ex, để so ngay trong transaction trước khi rollback.
func (e *itEnv) snapshotOn(ex db.Executor, vid uuid.UUID, withHeader bool) string {
	e.t.Helper()
	var s string
	header := `NULL`
	if withHeader {
		header = `(SELECT row_to_json(sv) FROM stage_versions sv WHERE sv.id = $1)`
	}
	ofVersion := func(table, alias, order string) string {
		return `COALESCE((SELECT json_agg(` + alias + ` ORDER BY ` + order + `) FROM ` + table + ` ` + alias +
			` JOIN lessons l ON l.id = ` + alias + `.lesson_id WHERE l.stage_version_id = $1), '[]')`
	}
	err := sqlx.GetContext(e.ctx(), ex, &s, `SELECT json_build_object(
		'header', `+header+`,
		'units', COALESCE((SELECT json_agg(su ORDER BY su.position) FROM stage_units su WHERE su.stage_version_id = $1), '[]'),
		'lessons', COALESCE((SELECT json_agg(l ORDER BY l.position) FROM lessons l WHERE l.stage_version_id = $1), '[]'),
		'media', `+ofVersion("lesson_media", "lm", "lm.lesson_id, lm.media_id")+`,
		'quiz', `+ofVersion("quiz_settings", "qs", "qs.lesson_id")+`,
		'questions', `+ofVersion("questions", "q", "q.lesson_id, q.position")+`,
		'options', COALESCE((SELECT json_agg(o ORDER BY o.question_id, o.position) FROM question_options o
		                     JOIN questions q ON q.id = o.question_id JOIN lessons l ON l.id = q.lesson_id
		                     WHERE l.stage_version_id = $1), '[]'),
		'homework', `+ofVersion("homework_settings", "hs", "hs.lesson_id")+`,
		'criteria', `+ofVersion("rubric_criteria", "rc", "rc.lesson_id, rc.position")+`,
		'attachments', `+ofVersion("homework_attachments", "ha", "ha.lesson_id, ha.position")+`)::text`, vid)
	if err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *itEnv) lessonMedia(lessonID uuid.UUID) []uuid.UUID {
	e.t.Helper()
	var out []uuid.UUID
	if err := e.dbx.Select(&out, `SELECT media_id FROM lesson_media WHERE lesson_id = $1 ORDER BY media_id`, lessonID); err != nil {
		e.t.Fatal(err)
	}
	return out
}

func (e *itEnv) count(q string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.dbx.Get(&n, q, args...); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// expectImmutable gọi fn trực tiếp trên repository (bỏ qua service) với bản published v1 và kiểm dữ liệu không đổi.
func expectImmutable(t *testing.T, fn func(e *itEnv, tx db.Executor, v *StageVersion) error) {
	t.Helper()
	e := newIT(t)
	before := e.versionSnapshot(dbV1ID, true)
	err := e.inTx(func(tx db.Executor) error {
		v, err := e.repo.ByID(e.ctx(), tx, dbV1ID)
		if err != nil {
			return err
		}
		return fn(e, tx, v)
	})
	if !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("err = %v, want ErrVersionImmutable", err)
	}
	if after := e.versionSnapshot(dbV1ID, true); after != before {
		t.Fatalf("published version changed:\nbefore %s\nafter  %s", before, after)
	}
}

func TestImmutability_SaveDraftHeader(t *testing.T) {
	expectImmutable(t, func(e *itEnv, tx db.Executor, v *StageVersion) error {
		v.title = "Sửa tiêu đề"
		return e.repo.SaveDraft(e.ctx(), tx, v)
	})
}

func TestImmutability_UpdateLesson(t *testing.T) {
	expectImmutable(t, func(e *itEnv, tx db.Executor, v *StageVersion) error {
		v.lessons[0].title = "Sửa học liệu"
		return e.repo.SaveDraft(e.ctx(), tx, v)
	})
}

func TestImmutability_DeleteLesson(t *testing.T) {
	expectImmutable(t, func(e *itEnv, tx db.Executor, v *StageVersion) error {
		v.lessons = v.lessons[1:]
		v.renumber()
		return e.repo.SaveDraft(e.ctx(), tx, v)
	})
}

func TestImmutability_InsertLesson(t *testing.T) {
	expectImmutable(t, func(e *itEnv, tx db.Executor, v *StageVersion) error {
		html := "<p>x</p>"
		v.lessons = append(v.lessons, &Lesson{
			id: ids.New(), key: "chen-them", title: "Chèn thêm", position: len(v.lessons) + 1, required: true,
			content: MarkdownContent{Source: "x", HTML: &html},
		})
		return e.repo.SaveDraft(e.ctx(), tx, v)
	})
}

func TestImmutability_LessonMedia(t *testing.T) {
	expectImmutable(t, func(e *itEnv, tx db.Executor, v *StageVersion) error {
		img := e.addMedia(media.KindImage, "image/png", "a.png")
		return replaceLessonMedia(e.ctx(), tx, v.lessons[0].id, []uuid.UUID{img})
	})
}

func TestImmutability_InsertUnit(t *testing.T) {
	expectImmutable(t, func(e *itEnv, tx db.Executor, v *StageVersion) error {
		v.units = append(v.units, &Unit{id: ids.New(), key: "chen-them", title: "Chèn thêm", position: len(v.units) + 1})
		return e.repo.SaveDraft(e.ctx(), tx, v)
	})
}

func TestImmutability_RenameUnit(t *testing.T) {
	expectImmutable(t, func(e *itEnv, tx db.Executor, v *StageVersion) error {
		v.units[0].title = "Đổi tên"
		return e.repo.SaveDraft(e.ctx(), tx, v)
	})
}

// Bỏ bài học khỏi aggregate nhưng giữ học liệu để câu DELETE stage_units là câu ghi đầu tiên chạm DB.
func TestImmutability_DeleteUnit(t *testing.T) {
	expectImmutable(t, func(e *itEnv, tx db.Executor, v *StageVersion) error {
		v.units = nil
		return e.repo.SaveDraft(e.ctx(), tx, v)
	})
}

func TestImmutability_DeleteHeader(t *testing.T) {
	expectImmutable(t, func(e *itEnv, tx db.Executor, v *StageVersion) error {
		return e.repo.Delete(e.ctx(), tx, v.ID())
	})
}

func TestImmutability_ServiceMutations(t *testing.T) {
	e := newIT(t)
	before := e.versionSnapshot(dbV1ID, true)
	lid := uuid.MustParse(testdb.LessonDBTableV1ID)
	title := "Sửa"
	errs := map[string]error{}
	uid := uuid.MustParse(testdb.UnitDBV1ID)
	_, errs["add"] = e.svc.AddLesson(e.ctx(), e.actor, dbV1ID, LessonCmd{UnitID: &uid, Title: &title, Type: strp("markdown"), MarkdownSource: &title})
	_, errs["update"] = e.svc.UpdateLesson(e.ctx(), e.actor, dbV1ID, lid, LessonCmd{Title: &title})
	errs["remove"] = e.svc.RemoveLesson(e.ctx(), e.actor, dbV1ID, lid)
	_, errs["reorder"] = e.svc.ReorderLessons(e.ctx(), e.actor, dbV1ID,
		[]UnitLessons{{UnitID: uid, LessonIDs: []uuid.UUID{uuid.MustParse(testdb.LessonDBIndexV1ID), lid}}})
	_, errs["addUnit"] = e.svc.AddUnit(e.ctx(), e.actor, dbV1ID, title)
	_, errs["renameUnit"] = e.svc.RenameUnit(e.ctx(), e.actor, dbV1ID, uid, title)
	_, errs["removeUnit"] = e.svc.RemoveUnit(e.ctx(), e.actor, dbV1ID, uid)
	_, errs["reorderUnits"] = e.svc.ReorderUnits(e.ctx(), e.actor, dbV1ID, []uuid.UUID{uid})
	for name, err := range errs {
		if !errors.Is(err, ErrVersionImmutable) {
			t.Errorf("%s err = %v", name, err)
		}
	}
	if e.versionSnapshot(dbV1ID, true) != before {
		t.Fatal("published version changed through service")
	}
}

func TestCreateStageCreatesSingleDraftAndAudit(t *testing.T) {
	e := newIT(t)
	vid := e.createStage("DOCKER")
	var stageID uuid.UUID
	if err := e.dbx.Get(&stageID, `SELECT stage_id FROM stage_versions WHERE id = $1`, vid); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM stage_versions WHERE stage_id = $1 AND version_no = 1 AND status = 'draft'`, stageID); n != 1 {
		t.Fatalf("v1 drafts = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM stage_versions WHERE stage_id = $1`, stageID); n != 1 {
		t.Fatalf("versions = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = 'stage.created' AND target_id = $1 AND actor_id = $2`, stageID, adminID); n != 1 {
		t.Fatalf("audit stage.created = %d", n)
	}
	if _, err := e.svc.CreateStage(e.ctx(), e.actor, CreateStageCmd{Code: "DOCKER", Name: "Lại"}); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("duplicate err = %v", err)
	}
}

func TestPublishMarkdownOnRealSchema(t *testing.T) {
	e := newIT(t)
	vid := e.createStage("DOCKER")
	if _, err := e.svc.Publish(e.ctx(), e.actor, vid); !errors.Is(err, ErrNoUnits) {
		t.Fatalf("empty publish err = %v", err)
	}
	img := e.addMedia(media.KindImage, "image/png", "so-do.png")
	// Ảnh ngoài chỉ còn trong dữ liệu cũ (lưu không qua được luật ảnh); phát hành vẫn phải lọc nó.
	l := e.addLegacyMarkdown(vid, "Giới thiệu", "# Hi <script>alert(1)</script> ![x](https://x/a.png) ![s](/api/v1/media/"+img.String()+"/content)")
	if got := e.lessonMedia(l.ID()); len(got) != 0 {
		t.Fatalf("draft lesson_media before render = %v", got)
	}
	d, err := e.svc.Publish(e.ctx(), e.actor, vid)
	if err != nil {
		t.Fatal(err)
	}
	if d.Version.Status() != domain.VersionPublished {
		t.Fatalf("status = %s", d.Version.Status())
	}
	var row struct {
		HTML        *string    `db:"markdown_html"`
		PublishedAt *time.Time `db:"published_at"`
	}
	if err := e.dbx.Get(&row, `SELECT l.markdown_html, sv.published_at FROM lessons l JOIN stage_versions sv ON sv.id = l.stage_version_id
		WHERE l.id = $1`, l.ID()); err != nil {
		t.Fatal(err)
	}
	if row.HTML == nil || strings.Contains(*row.HTML, "<script") || strings.Contains(*row.HTML, "x/a.png") || !strings.Contains(*row.HTML, img.String()) {
		t.Fatalf("markdown_html = %v", row.HTML)
	}
	if row.PublishedAt == nil || !row.PublishedAt.Equal(e.clk.Now()) {
		t.Fatalf("published_at = %v", row.PublishedAt)
	}
	if got := e.lessonMedia(l.ID()); len(got) != 1 || got[0] != img {
		t.Fatalf("lesson_media after publish = %v", got)
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = 'stage_version.published' AND target_id = $1`, vid); n != 1 {
		t.Fatalf("audit published = %d", n)
	}
}

func TestPublishRejectsMarkdownImageNotUploaded(t *testing.T) {
	e := newIT(t)
	vid := e.createStage("DOCKER")
	e.addMarkdown(vid, "Ảnh", "![s](/api/v1/media/"+ids.New().String()+"/content)")
	if _, err := e.svc.Publish(e.ctx(), e.actor, vid); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if n := e.count(`SELECT count(*) FROM stage_versions WHERE id = $1 AND status = 'draft'`, vid); n != 1 {
		t.Fatal("version must stay draft")
	}
}

func TestTransitionPublishGuardRequiresRenderedMarkdown(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	now := e.clk.Now()
	err := e.inTx(func(tx db.Executor) error {
		return e.repo.TransitionStatus(e.ctx(), tx, dbV2ID, domain.VersionDraft, domain.VersionPublished, &now)
	})
	if !errors.Is(err, ErrNotRendered) || err.Error() != "Phiên bản còn học liệu chưa render." {
		t.Fatalf("err = %v", err)
	}
}

func TestArchiveTouchesHeaderOnly(t *testing.T) {
	e := newIT(t)
	before := e.versionSnapshot(dbV1ID, false)
	d, err := e.svc.Archive(e.ctx(), e.actor, dbV1ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Version.Status() != domain.VersionArchived || d.Version.ArchivedAt() == nil {
		t.Fatalf("status=%s archivedAt=%v", d.Version.Status(), d.Version.ArchivedAt())
	}
	if after := e.versionSnapshot(dbV1ID, false); after != before {
		t.Fatalf("lessons changed by archive:\nbefore %s\nafter  %s", before, after)
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = 'stage_version.archived' AND target_id = $1`, dbV1ID); n != 1 {
		t.Fatalf("audit archived = %d", n)
	}
}

func TestTransitionStatusMatrix(t *testing.T) {
	e := newIT(t)
	now := e.clk.Now()
	tr := func(from, to domain.VersionStatus, at *time.Time) error {
		return db.Transact(e.ctx(), e.dbx, func(tx db.Executor) error {
			return e.repo.TransitionStatus(e.ctx(), tx, dbV1ID, from, to, at)
		})
	}
	if err := tr(domain.VersionPublished, domain.VersionDraft, nil); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("published→draft err = %v", err)
	}
	if err := tr(domain.VersionDraft, domain.VersionPublished, &now); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("draft→published on published row err = %v", err)
	}
	if err := tr(domain.VersionPublished, domain.VersionArchived, nil); err != nil {
		t.Fatalf("published→archived: %v", err)
	}
	if n := e.count(`SELECT count(*) FROM stage_versions WHERE id = $1 AND status = 'archived' AND archived_at IS NOT NULL`, dbV1ID); n != 1 {
		t.Fatal("archived_at not set")
	}
	if err := tr(domain.VersionArchived, domain.VersionPublished, &now); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("archived→published err = %v", err)
	}
	if err := tr(domain.VersionPublished, domain.VersionArchived, nil); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("archive twice err = %v", err)
	}
}

// pausingRepo giữ khóa FOR UPDATE của Publish cho tới khi test cho phép, để AddLesson chắc chắn phải chờ.
type pausingRepo struct {
	PGStageVersionRepo
	locked  chan struct{}
	release chan struct{}
}

func (p pausingRepo) ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*StageVersion, error) {
	v, err := p.PGStageVersionRepo.ByIDForUpdate(ctx, ex, id)
	close(p.locked)
	<-p.release
	return v, err
}

func (e *itEnv) waitForLockWaiter() {
	e.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if e.count(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'
			AND wait_event <> 'advisory'`) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.t.Fatal("AddLesson never waited on the version lock")
}

// assertNoOrphans: mọi học liệu của bản published đều đã render (học liệu chen vào sau phát hành sẽ có HTML NULL).
func (e *itEnv) assertNoOrphans(vid uuid.UUID, wantLessons int) {
	e.t.Helper()
	if n := e.count(`SELECT count(*) FROM lessons WHERE stage_version_id = $1`, vid); n != wantLessons {
		e.t.Fatalf("lessons on published = %d, want %d", n, wantLessons)
	}
	if n := e.count(`SELECT count(*) FROM lessons WHERE stage_version_id = $1 AND markdown_html IS NULL`, vid); n != 0 {
		e.t.Fatalf("orphan unrendered lessons on published = %d", n)
	}
}

func TestConcurrentAddLessonAndPublish(t *testing.T) {
	t.Run("publish giữ khóa trước", func(t *testing.T) {
		e := newIT(t)
		vid := e.createStage("RACE")
		e.addMarkdown(vid, "Bài 1", "# Một")
		unit := e.firstUnit(vid)
		p := pausingRepo{locked: make(chan struct{}), release: make(chan struct{})}
		publisher := e.service(p)

		var wg sync.WaitGroup
		var pubErr, addErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, pubErr = publisher.Publish(e.ctx(), e.actor, vid)
		}()
		<-p.locked
		go func() {
			defer wg.Done()
			title, src := "Bài chen", "# Chen"
			_, addErr = e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{UnitID: unit, Title: &title, Type: strp("markdown"), MarkdownSource: &src})
		}()
		e.waitForLockWaiter()
		close(p.release)
		wg.Wait()

		if pubErr != nil {
			t.Fatalf("publish: %v", pubErr)
		}
		if !errors.Is(addErr, ErrVersionImmutable) {
			t.Fatalf("add err = %v, want VERSION_IMMUTABLE", addErr)
		}
		e.assertNoOrphans(vid, 1)
	})

	t.Run("chạy đua thật", func(t *testing.T) {
		e := newIT(t)
		for i := range 10 {
			vid := e.createStage("RACE" + string(rune('A'+i)))
			e.addMarkdown(vid, "Bài 1", "# Một")
			unit := e.firstUnit(vid)
			start := make(chan struct{})
			var wg sync.WaitGroup
			var pubErr, addErr error
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				_, pubErr = e.svc.Publish(e.ctx(), e.actor, vid)
			}()
			go func() {
				defer wg.Done()
				<-start
				title, src := "Bài chen", "# Chen"
				_, addErr = e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{UnitID: unit, Title: &title, Type: strp("markdown"), MarkdownSource: &src})
			}()
			close(start)
			wg.Wait()
			if pubErr != nil {
				t.Fatalf("round %d publish: %v", i, pubErr)
			}
			switch {
			case addErr == nil:
				e.assertNoOrphans(vid, 2)
			case errors.Is(addErr, ErrVersionImmutable):
				e.assertNoOrphans(vid, 1)
			default:
				t.Fatalf("round %d add err = %v", i, addErr)
			}
		}
	})
}

func TestSecondDraftIsRejected(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	err := e.inTx(func(tx db.Executor) error {
		return e.repo.Create(e.ctx(), tx, NewDraftVersion(ids.New(), stageDBID, 3, "Database", "", adminID))
	})
	var de *ErrDraftExists
	if !errors.As(err, &de) || de.DraftID != dbV2ID || de.No != 2 {
		t.Fatalf("repo create err = %v", err)
	}
	_, err = e.svc.Clone(e.ctx(), e.actor, dbV1ID)
	if !errors.As(err, &de) || de.DraftID != dbV2ID {
		t.Fatalf("clone err = %v", err)
	}
}

func TestCloneKeepsLessonsAndMedia(t *testing.T) {
	e := newIT(t)
	vid := e.createStage("CLONE")
	video := e.addMedia(media.KindVideo, "video/mp4", "bai-1.mp4")
	img := e.addMedia(media.KindImage, "image/png", "anh.png")
	e.addVideo(vid, "Video mở đầu", video, 125)
	e.addMarkdown(vid, "Đọc thêm", "![a](/api/v1/media/"+img.String()+"/content)")
	if _, err := e.svc.Publish(e.ctx(), e.actor, vid); err != nil {
		t.Fatal(err)
	}
	c, err := e.svc.Clone(e.ctx(), e.actor, vid)
	if err != nil {
		t.Fatal(err)
	}
	if c.Version.VersionNo() != 2 || c.Version.Status() != domain.VersionDraft || c.ClonedFromVersionNo == nil || *c.ClonedFromVersionNo != 1 {
		t.Fatalf("clone header v%d %s", c.Version.VersionNo(), c.Version.Status())
	}
	src, err := e.repo.ByID(e.ctx(), e.dbx, vid)
	if err != nil {
		t.Fatal(err)
	}
	for i, sl := range src.Lessons() {
		cl := c.Version.Lessons()[i]
		if cl.ID() == sl.ID() || cl.Key() != sl.Key() || cl.Required() != sl.Required() || cl.Position() != sl.Position() {
			t.Fatalf("lesson %d differs", i)
		}
		if a, b := e.lessonMedia(sl.ID()), e.lessonMedia(cl.ID()); len(a) != 1 || len(b) != 1 || a[0] != b[0] {
			t.Fatalf("lesson %d media %v vs %v", i, a, b)
		}
	}
	if vc := c.Version.Lessons()[0].Content().(VideoContent); vc.MediaID != video || vc.FileName != "bai-1.mp4" || *vc.DurationSeconds != 125 {
		t.Fatalf("video content = %+v", vc)
	}
	if n := e.count(`SELECT count(*) FROM audit_logs WHERE action = 'stage_version.cloned' AND target_id = $1`, c.Version.ID()); n != 1 {
		t.Fatalf("audit cloned = %d", n)
	}
	_, err = e.svc.Clone(e.ctx(), e.actor, vid)
	var de *ErrDraftExists
	if !errors.As(err, &de) || de.DraftID != c.Version.ID() {
		t.Fatalf("second clone err = %v", err)
	}
}

// guardedRouter dựng route soạn sau SessionAuth và RequireRole(admin) thật như router của app; login trả cookie
// phiên, call gọi route bằng cookie đó.
type guardedRouter struct {
	t     *testing.T
	e     *itEnv
	r     *gin.Engine
	ident *identity.Service
}

func (e *itEnv) guardedRouter() *guardedRouter {
	ident := identity.NewService(identity.ServiceDeps{
		DB: e.dbx, Tx: db.TxRunner{DB: e.dbx},
		Users: identity.PGUserRepo{}, Sessions: identity.PGSessionRepo{}, Attempts: identity.PGLoginAttemptRepo{},
		Resets: identity.PGResetTokenRepo{}, Clock: clock.Real{}, Audit: audit.Noop{},
		Cfg: config.Config{
			SessionTTL: 12 * time.Hour, TempPasswordTTL: 72 * time.Hour, ResetTokenTTL: 30 * time.Minute,
			PasswordMinLength: 8, LoginMaxFailures: 5, LoginLockWindow: 15 * time.Minute,
			PublicBaseURL: "http://localhost:5173", AppEnv: config.EnvE2E,
		},
	})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := r.Group("/api/v1", identity.NewMiddleware(ident, false).SessionAuth())
	NewHandler(e.svc, func(c *gin.Context) (uuid.UUID, bool) {
		u, ok := identity.CurrentUser(c)
		if !ok {
			return uuid.Nil, false
		}
		return u.ID(), true
	}).Register(authed, identity.RequireRole(domain.RoleAdmin))
	return &guardedRouter{t: e.t, e: e, r: r, ident: ident}
}

func (g *guardedRouter) login(email string) string {
	g.t.Helper()
	res, err := g.ident.Login(g.e.ctx(), email, testdb.FixturePassword, "127.0.0.1", "it")
	if err != nil {
		g.t.Fatalf("đăng nhập %s: %v", email, err)
	}
	return res.Token.Reveal()
}

func (g *guardedRouter) call(cookie, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: identity.CookieName(false), Value: cookie})
	w := httptest.NewRecorder()
	g.r.ServeHTTP(w, req)
	return w
}

// expectForbidden gọi từng route bằng cookie và đòi 403 FORBIDDEN.
func (g *guardedRouter) expectForbidden(cookie string, routes []struct{ method, path, body string }) {
	g.t.Helper()
	for _, rt := range routes {
		w := g.call(cookie, rt.method, rt.path, rt.body)
		var env struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); w.Code != http.StatusForbidden || err != nil || env.Error.Code != "FORBIDDEN" {
			g.t.Errorf("teacher %s %s = %d %s", rt.method, rt.path, w.Code, w.Body)
		}
	}
}

// TestUnitRoutesRequireAdmin: teacher nhận FORBIDDEN ở mọi route soạn bài học và không đổi được dữ liệu, admin dùng
// được.
func TestUnitRoutesRequireAdmin(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	g := e.guardedRouter()

	base := "/api/v1/stage-versions/" + dbV2ID.String()
	unit := testdb.UnitDBV2ID
	lessons := `["` + testdb.LessonDBIndexV2ID + `","` + testdb.LessonDBTableV2ID + `"]`
	routes := []struct{ method, path, body string }{
		{http.MethodPost, base + "/units", `{"title":"Nâng cao"}`},
		{http.MethodPatch, base + "/units/" + unit, `{"title":"Đổi tên"}`},
		{http.MethodPut, base + "/units/order", `{"unitIds":["` + unit + `"]}`},
		{http.MethodPut, base + "/lessons/order", `{"units":[{"unitId":"` + unit + `","lessonIds":` + lessons + `}]}`},
		{http.MethodDelete, base + "/units/" + unit, ""},
	}
	before := e.versionSnapshot(dbV2ID, true)
	g.expectForbidden(g.login("huong.le@goup.vn"), routes)
	if e.versionSnapshot(dbV2ID, true) != before {
		t.Fatal("teacher calls changed the draft")
	}

	admin := g.login("quan.tran@goup.vn")
	want := []int{http.StatusCreated, http.StatusOK, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, http.StatusNoContent}
	for i, rt := range routes {
		// Sau khi admin thêm "Nâng cao", body thứ tự chỉ liệt kê một bài học nên nhận 422: admin qua được guard.
		if w := g.call(admin, rt.method, rt.path, rt.body); w.Code != want[i] {
			t.Errorf("admin %s %s = %d, want %d: %s", rt.method, rt.path, w.Code, want[i], w.Body)
		}
	}
	if got, want := e.unitLayout(dbV2ID), "1.Nâng cao[]"; got != want {
		t.Fatalf("after admin calls = %s, want %s", got, want)
	}
}

// lessonUnits trả "lesson_key=unit_key" theo position học liệu, chỉ khi bài học thuộc đúng phiên bản của học liệu.
func (e *itEnv) lessonUnits(vid uuid.UUID) []string {
	e.t.Helper()
	var out []string
	if err := e.dbx.Select(&out, `SELECT l.lesson_key || '=' || su.unit_key FROM lessons l
		JOIN stage_units su ON su.id = l.unit_id AND su.stage_version_id = l.stage_version_id
		WHERE l.stage_version_id = $1 ORDER BY l.position`, vid); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// unitLayout đọc bố cục bài học và học liệu thẳng từ DB, cùng dạng với layoutOf của aggregate.
func (e *itEnv) unitLayout(vid uuid.UUID) string {
	e.t.Helper()
	var rows []struct {
		UnitPos  int     `db:"unit_pos"`
		Title    string  `db:"title"`
		Key      *string `db:"lesson_key"`
		Position *int    `db:"position"`
	}
	if err := e.dbx.Select(&rows, `SELECT su.position AS unit_pos, su.title, l.lesson_key, l.position FROM stage_units su
		LEFT JOIN lessons l ON l.unit_id = su.id WHERE su.stage_version_id = $1 ORDER BY su.position, l.position`, vid); err != nil {
		e.t.Fatal(err)
	}
	var b strings.Builder
	for i, r := range rows {
		if i == 0 || rows[i-1].UnitPos != r.UnitPos {
			if i > 0 {
				b.WriteString("] ")
			}
			fmt.Fprintf(&b, "%d.%s[", r.UnitPos, r.Title)
		} else {
			b.WriteString(" ")
		}
		if r.Key != nil {
			fmt.Fprintf(&b, "%s@%d", *r.Key, *r.Position)
		}
	}
	if len(rows) > 0 {
		b.WriteString("]")
	}
	return b.String()
}

func TestCreateStageStartsWithoutUnits(t *testing.T) {
	e := newIT(t)
	vid := e.createStage("UNIT")
	if n := e.count(`SELECT count(*) FROM stage_units WHERE stage_version_id = $1`, vid); n != 0 {
		t.Fatalf("units of new stage = %d", n)
	}
	title, src := "Một", "# Một"
	if _, err := e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{Title: &title, Type: strp("markdown"), MarkdownSource: &src}); !errors.Is(err, ErrLessonUnitRequired) {
		t.Fatalf("add lesson without unit err = %v", err)
	}
	if n := e.count(`SELECT count(*) FROM lessons WHERE stage_version_id = $1`, vid); n != 0 {
		t.Fatalf("lessons = %d", n)
	}
}

// Dữ liệu MVP sau migration nằm trong một bài học "Nội dung chặng" và giữ thứ tự cũ.
func TestMigratedVersionKeepsOrderInDefaultUnit(t *testing.T) {
	e := newIT(t)
	v, err := e.repo.ByID(e.ctx(), e.dbx, dbV1ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(v), "1.Nội dung chặng[Thiết kế bảng và khóa@1 Đọc thêm: chỉ mục@2]"; got != want {
		t.Fatalf("layout = %s, want %s", got, want)
	}
	if got, want := e.unitLayout(dbV1ID), "1.Nội dung chặng[db-table@1 db-index@2]"; got != want {
		t.Fatalf("db layout = %s, want %s", got, want)
	}
}

func TestUnitOperationsPersist(t *testing.T) {
	e := newIT(t)
	vid := e.createStage("UNITS")
	a := e.addUnit(vid, "Nhập môn")
	b := e.addUnit(vid, "Thiết kế bảng")
	one := e.addMarkdownTo(vid, a, "Một", "# Một")
	e.addMarkdownTo(vid, b, "Hai", "# Hai")
	e.addMarkdownTo(vid, a, "Ba", "# Ba")
	if got, want := e.unitLayout(vid), "1.Nhập môn[mot@1 ba@2] 2.Thiết kế bảng[hai@3]"; got != want {
		t.Fatalf("after add = %s, want %s", got, want)
	}

	if _, err := e.svc.RenameUnit(e.ctx(), e.actor, vid, b, "Thiết kế CSDL"); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM stage_units WHERE id = $1 AND title = 'Thiết kế CSDL' AND unit_key = 'thiet-ke-bang'`, b); n != 1 {
		t.Fatal("rename not persisted or unit_key changed")
	}
	if _, err := e.svc.UpdateLesson(e.ctx(), e.actor, vid, one.ID(), LessonCmd{UnitID: &b}); err != nil {
		t.Fatal(err)
	}
	if got, want := e.unitLayout(vid), "1.Nhập môn[ba@1] 2.Thiết kế CSDL[hai@2 mot@3]"; got != want {
		t.Fatalf("after move = %s, want %s", got, want)
	}
	if _, err := e.svc.ReorderUnits(e.ctx(), e.actor, vid, []uuid.UUID{b, a}); err != nil {
		t.Fatal(err)
	}
	if got, want := e.unitLayout(vid), "1.Thiết kế CSDL[hai@1 mot@2] 2.Nhập môn[ba@3]"; got != want {
		t.Fatalf("after unit reorder = %s, want %s", got, want)
	}
	stored, err := e.repo.ByID(e.ctx(), e.dbx, vid)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(stored), "1.Thiết kế CSDL[Hai@1 Một@2] 2.Nhập môn[Ba@3]"; got != want {
		t.Fatalf("reloaded aggregate = %s, want %s", got, want)
	}
}

// Hoán đổi vị trí bài học và chuyển học liệu sang bài học khác trong một lần lưu: UNIQUE vị trí được hoãn tới commit.
func TestReorderAcrossUnitsWithoutUniqueViolation(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	a, b := uuid.MustParse(testdb.LessonDBTableV2ID), uuid.MustParse(testdb.LessonDBIndexV2ID)
	extra := e.addUnit(dbV2ID, "Nâng cao")
	d, err := e.svc.ReorderLessons(e.ctx(), e.actor, dbV2ID, []UnitLessons{
		{UnitID: unitV2ID, LessonIDs: []uuid.UUID{b}},
		{UnitID: extra, LessonIDs: []uuid.UUID{a}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Version.Lessons(); got[0].ID() != b || got[0].Position() != 1 || got[1].ID() != a || got[1].Position() != 2 {
		t.Fatal("order not swapped")
	}
	if got, want := e.unitLayout(dbV2ID), "1.Nội dung chặng[db-index@1] 2.Nâng cao[db-table@2]"; got != want {
		t.Fatalf("after layout = %s, want %s", got, want)
	}
	if _, err := e.svc.ReorderUnits(e.ctx(), e.actor, dbV2ID, []uuid.UUID{extra, unitV2ID}); err != nil {
		t.Fatal(err)
	}
	if got, want := e.unitLayout(dbV2ID), "1.Nâng cao[db-table@1] 2.Nội dung chặng[db-index@2]"; got != want {
		t.Fatalf("after unit swap = %s, want %s", got, want)
	}
	if _, err := e.svc.ReorderLessons(e.ctx(), e.actor, dbV2ID, []UnitLessons{{UnitID: extra, LessonIDs: []uuid.UUID{a, b}}}); !errors.Is(err, ErrReorderMismatch) {
		t.Fatalf("mismatch err = %v", err)
	}
}

func TestRemoveUnitCascadesLessonsAndMedia(t *testing.T) {
	e := newIT(t)
	vid := e.createStage("RMUNIT")
	a := e.addUnit(vid, "Bỏ đi")
	b := e.addUnit(vid, "Giữ lại")
	video := e.addMedia(media.KindVideo, "video/mp4", "v.mp4")
	seconds := 60
	vl, err := e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{UnitID: &a, Title: strp("Video"), Type: strp("video"),
		VideoMediaID: &video, DurationSeconds: &seconds})
	if err != nil {
		t.Fatal(err)
	}
	ml := e.addMarkdownTo(vid, a, "Ghi chú", "# Ghi chú")
	e.addMarkdownTo(vid, b, "Giữ", "# Giữ")
	if len(e.lessonMedia(vl.ID())) != 1 {
		t.Fatal("lesson_media missing before remove")
	}

	n, err := e.svc.RemoveUnit(e.ctx(), e.actor, vid, a)
	if err != nil || n != 2 {
		t.Fatalf("RemoveUnit = %d, %v; want 2", n, err)
	}
	if got, want := e.unitLayout(vid), "1.Giữ lại[giu@1]"; got != want {
		t.Fatalf("after remove = %s, want %s", got, want)
	}
	if c := e.count(`SELECT count(*) FROM lessons WHERE id = ANY($1)`, []uuid.UUID{vl.ID(), ml.ID()}); c != 0 {
		t.Fatalf("lessons left = %d", c)
	}
	if c := e.count(`SELECT count(*) FROM lesson_media WHERE lesson_id = $1`, vl.ID()); c != 0 {
		t.Fatalf("lesson_media left = %d", c)
	}
	if c := e.count(`SELECT count(*) FROM media_files WHERE id = $1`, video); c != 1 {
		t.Fatal("media files must survive unit delete")
	}
	if _, err := e.svc.RemoveUnit(e.ctx(), e.actor, vid, a); !errors.Is(err, ErrUnitNotFound) {
		t.Fatalf("remove twice err = %v", err)
	}
}

// Unique index lower(btrim(title)) chặn cả khi bỏ qua kiểm tra của domain, và lỗi map về CONFLICT.
func TestUnitTitleUniqueOnRealSchema(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	err := e.inTx(func(tx db.Executor) error {
		return insertUnit(e.ctx(), tx, dbV2ID, &Unit{id: ids.New(), key: "trung-ten", title: "  NỘI DUNG CHẶNG ", position: 2})
	})
	if !errors.Is(err, ErrUnitTitleTaken) || !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("direct insert err = %v", err)
	}
	if _, err := e.svc.AddUnit(e.ctx(), e.actor, dbV2ID, "nội dung chặng"); !errors.Is(err, ErrUnitTitleTaken) {
		t.Fatalf("service err = %v", err)
	}
	if n := e.count(`SELECT count(*) FROM stage_units WHERE stage_version_id = $1`, dbV2ID); n != 1 {
		t.Fatalf("units = %d", n)
	}
}

func TestCloneDeepCopiesUnits(t *testing.T) {
	e := newIT(t)
	vid := e.createStage("UNITCLONE")
	intro := e.addUnit(vid, "Nhập môn")
	adv := e.addUnit(vid, "Nâng cao")
	a := e.addMarkdownTo(vid, intro, "Một", "# Một")
	b := e.addMarkdownTo(vid, adv, "Hai", "# Hai")
	if _, err := e.svc.Publish(e.ctx(), e.actor, vid); err != nil {
		t.Fatal(err)
	}
	c, err := e.svc.Clone(e.ctx(), e.actor, vid)
	if err != nil {
		t.Fatal(err)
	}
	cid := c.Version.ID()
	want := []string{a.Key().String() + "=nhap-mon", b.Key().String() + "=nang-cao"}
	if got := e.lessonUnits(cid); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("clone lesson units = %v, want %v", got, want)
	}
	if n := e.count(`SELECT count(*) FROM stage_units cu JOIN stage_units su ON su.stage_version_id = $1
		AND su.unit_key = cu.unit_key AND su.title = cu.title AND su.position = cu.position AND su.id <> cu.id
		WHERE cu.stage_version_id = $2`, vid, cid); n != 2 {
		t.Fatalf("copied units = %d", n)
	}
	// Sửa bản nháp mới không chạm bài học của bản đã phát hành.
	before := e.versionSnapshot(vid, true)
	cloneAdv := c.Version.Units()[1].ID()
	if _, err := e.svc.RenameUnit(e.ctx(), e.actor, cid, cloneAdv, "Chuyên sâu"); err != nil {
		t.Fatal(err)
	}
	added := e.addMarkdownTo(cid, cloneAdv, "Ba", "# Ba")
	want = append(want, added.Key().String()+"=nang-cao")
	if got := e.lessonUnits(cid); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after add = %v, want %v", got, want)
	}
	if e.versionSnapshot(vid, true) != before {
		t.Fatal("editing the clone changed the published version")
	}
}

func TestDeletePublishedInUse(t *testing.T) {
	e := newIT(t)
	_, err := e.svc.Delete(e.ctx(), e.actor, dbV1ID)
	var iu *ErrInUse
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v", err)
	}
	if len(iu.UsedBy) != 1 || iu.UsedBy[0].CourseName != "Lập trình cơ bản" || iu.UsedBy[0].VersionNo != 1 ||
		len(iu.UsedBy[0].ClassCodes) != 1 || iu.UsedBy[0].ClassCodes[0] != "basic01" {
		t.Fatalf("usedBy = %+v", iu.UsedBy)
	}
	if !strings.Contains(iu.Error(), "Lập trình cơ bản v1") {
		t.Fatalf("message = %s", iu.Error())
	}
}

func TestDeleteDraftCascadesAndLastDeletesStage(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	deleted, err := e.svc.Delete(e.ctx(), e.actor, dbV2ID)
	if err != nil || deleted {
		t.Fatalf("delete DB v2 deleted=%v err=%v", deleted, err)
	}
	if n := e.count(`SELECT count(*) FROM lessons WHERE stage_version_id = $1`, dbV2ID); n != 0 {
		t.Fatalf("lessons left = %d", n)
	}

	vid := e.createStage("SOLO")
	video := e.addMedia(media.KindVideo, "video/mp4", "solo.mp4")
	l := e.addVideo(vid, "Video", video, 10)
	var stageID uuid.UUID
	if err := e.dbx.Get(&stageID, `SELECT stage_id FROM stage_versions WHERE id = $1`, vid); err != nil {
		t.Fatal(err)
	}
	deleted, err = e.svc.Delete(e.ctx(), e.actor, vid)
	if err != nil || !deleted {
		t.Fatalf("delete last draft deleted=%v err=%v", deleted, err)
	}
	if n := e.count(`SELECT count(*) FROM stages WHERE id = $1`, stageID); n != 0 {
		t.Fatal("stage not deleted")
	}
	if n := e.count(`SELECT count(*) FROM lesson_media WHERE lesson_id = $1`, l.ID()); n != 0 {
		t.Fatal("lesson_media not cascaded")
	}
	if n := e.count(`SELECT count(*) FROM media_files WHERE id = $1`, video); n != 1 {
		t.Fatal("media file must survive version delete")
	}
}

func TestLessonMediaFollowsSaveDraft(t *testing.T) {
	e := newIT(t)
	vid := e.createStage("MEDIA")
	video := e.addMedia(media.KindVideo, "video/mp4", "v.mp4")
	img1 := e.addMedia(media.KindImage, "image/png", "1.png")
	img2 := e.addMedia(media.KindImage, "image/webp", "2.webp")
	vl := e.addVideo(vid, "Video", video, 60)
	ml := e.addMarkdown(vid, "Ảnh", "![1](/api/v1/media/"+img1.String()+"/content) ![2](/api/v1/media/"+img2.String()+"/content)")
	if got := e.lessonMedia(vl.ID()); len(got) != 1 || got[0] != video {
		t.Fatalf("video lesson_media = %v", got)
	}

	render := func(src string) error {
		return db.Transact(e.ctx(), e.dbx, func(tx db.Executor) error {
			v, err := e.repo.ByIDForUpdate(e.ctx(), tx, vid)
			if err != nil {
				return err
			}
			if err := v.UpdateLesson(ml.ID(), "Ảnh", MarkdownContent{Source: src}, true); err != nil {
				return err
			}
			if err := v.RenderMarkdown(NewMarkdownRenderer()); err != nil {
				return err
			}
			return e.repo.SaveDraft(e.ctx(), tx, v)
		})
	}
	if err := render("![1](/api/v1/media/" + img1.String() + "/content) ![2](/api/v1/media/" + img2.String() + "/content)"); err != nil {
		t.Fatal(err)
	}
	want := []uuid.UUID{img1, img2}
	sort.Slice(want, func(i, j int) bool { return want[i].String() < want[j].String() })
	if got := e.lessonMedia(ml.ID()); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("markdown lesson_media = %v, want %v", got, want)
	}
	if err := render("![2](/api/v1/media/" + img2.String() + "/content)"); err != nil {
		t.Fatal(err)
	}
	if got := e.lessonMedia(ml.ID()); len(got) != 1 || got[0] != img2 {
		t.Fatalf("after edit lesson_media = %v", got)
	}
	if got := e.lessonMedia(vl.ID()); len(got) != 1 || got[0] != video {
		t.Fatalf("video lesson_media changed = %v", got)
	}
}

// insertCourse tạo khóa học với các phiên bản (status, phiên bản chặng dùng) để dựng tình huống FR-18.
func (e *itEnv) insertCourse(code, name string, versions ...struct {
	status       domain.VersionStatus
	stageVersion uuid.UUID
}) []uuid.UUID {
	e.t.Helper()
	courseID := ids.New()
	if _, err := e.dbx.Exec(`INSERT INTO courses (id, code, name, created_by) VALUES ($1, $2, $3, $4)`, courseID, code, name, adminID); err != nil {
		e.t.Fatal(err)
	}
	out := make([]uuid.UUID, 0, len(versions))
	for i, v := range versions {
		cvID := ids.New()
		var publishedAt *time.Time
		if v.status != domain.VersionDraft {
			now := e.clk.Now()
			publishedAt = &now
		}
		if _, err := e.dbx.Exec(`INSERT INTO course_versions (id, course_id, version_no, status, title, published_at, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, cvID, courseID, i+1, v.status.String(), name, publishedAt, adminID); err != nil {
			e.t.Fatal(err)
		}
		if _, err := e.dbx.Exec(`INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position)
			VALUES ($1, $2, $3, 1)`, cvID, v.stageVersion, stageDBID); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, cvID)
	}
	return out
}

type cv = struct {
	status       domain.VersionStatus
	stageVersion uuid.UUID
}

// outdatedScenario: DB v2 phát hành làm BASIC (dùng DB v1) thành cũ; UPTODATE dùng DB v2; HASDRAFT dùng DB v1 và
// có bản nháp nên chưa áp được.
func outdatedScenario(t *testing.T) *itEnv {
	t.Helper()
	e := newIT(t, "stage_db_draft")
	if _, err := e.svc.Publish(e.ctx(), e.actor, dbV2ID); err != nil {
		t.Fatal(err)
	}
	e.insertCourse("UPTODATE", "Khóa cập nhật", cv{domain.VersionPublished, dbV2ID})
	e.insertCourse("HASDRAFT", "Khóa có nháp", cv{domain.VersionPublished, dbV1ID}, cv{domain.VersionDraft, dbV1ID})
	return e
}

func TestOutdatedCourses(t *testing.T) {
	e := outdatedScenario(t)
	check := func(name string, got []OutdatedCourse) {
		t.Helper()
		codes := make([]string, 0, len(got))
		for _, o := range got {
			codes = append(codes, o.CourseCode)
			if o.UsingVersionNo != 1 || o.LatestVersionNo != 2 || o.LatestVersionID != dbV2ID || o.StageID != stageDBID {
				t.Fatalf("%s row = %+v", name, o)
			}
			row := ToOutdatedRow(o)
			switch o.CourseCode {
			case "BASIC":
				if !row.CanApply || row.BlockedReason != nil {
					t.Fatalf("%s BASIC = %+v", name, row)
				}
			case "HASDRAFT":
				if row.CanApply || row.BlockedReason == nil || *row.BlockedReason != "Khóa học đang có bản nháp v2. Phát hành hoặc xóa bản nháp trước." {
					t.Fatalf("%s HASDRAFT = %+v", name, row)
				}
			}
		}
		sort.Strings(codes)
		if strings.Join(codes, ",") != "BASIC,HASDRAFT" {
			t.Fatalf("%s courses = %v", name, codes)
		}
	}
	one, err := e.svc.OutdatedCourses(e.ctx(), stageDBID)
	if err != nil {
		t.Fatal(err)
	}
	check("OutdatedCourses", one)
	var reader OutdatedReader = e.svc
	all, err := reader.AllOutdated(e.ctx())
	if err != nil {
		t.Fatal(err)
	}
	check("AllOutdated", all)

	d, err := e.svc.GetStage(e.ctx(), stageDBID)
	if err != nil {
		t.Fatal(err)
	}
	flags := map[string]bool{}
	for _, u := range d.UsedBy {
		flags[u.CourseCode+"/"+u.Status.String()] = u.Outdated
	}
	if !flags["BASIC/published"] || flags["UPTODATE/published"] || !flags["HASDRAFT/draft"] {
		t.Fatalf("usedBy outdated flags = %v", flags)
	}
}

func TestVersionRefReader(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	refs, err := e.repo.Refs(e.ctx(), e.dbx, []uuid.UUID{dbV1ID, dbV2ID, ids.New()})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[dbV1ID].StageCode != "DB" || refs[dbV1ID].LessonCount != 2 || refs[dbV2ID].Status != domain.VersionDraft {
		t.Fatalf("refs = %+v", refs)
	}
	latest, err := e.repo.LatestPublishedOfStage(e.ctx(), e.dbx, stageDBID)
	if err != nil || latest.ID != dbV1ID {
		t.Fatalf("latest = %+v err = %v", latest, err)
	}
	vid := e.createStage("NOPUB")
	var stageID uuid.UUID
	if err := e.dbx.Get(&stageID, `SELECT stage_id FROM stage_versions WHERE id = $1`, vid); err != nil {
		t.Fatal(err)
	}
	if _, err := e.repo.LatestPublishedOfStage(e.ctx(), e.dbx, stageID); !errors.Is(err, ErrVersionNotFound) {
		t.Fatalf("no published err = %v", err)
	}
}

// --- golden JSON qua HTTP handler, dữ liệu seed.js ---

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
		e.t.Fatalf("%s %s = %d, want %d: %s", method, path, w.Code, wantStatus, w.Body)
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
	t.Run("stage list và version từ seed", func(t *testing.T) {
		e := newIT(t, "stage_db_draft")
		r := e.router()
		assertGolden(t, "stage_list.json", e.call(r, http.MethodGet, "/api/v1/stages", "", http.StatusOK))
		assertGolden(t, "stage_version.json", e.call(r, http.MethodGet, "/api/v1/stage-versions/"+dbV1ID.String(), "", http.StatusOK))
		assertGolden(t, "stage_version_draft.json", e.call(r, http.MethodGet, "/api/v1/stage-versions/"+dbV2ID.String(), "", http.StatusOK))
		assertGolden(t, "error_draft_exists.json", e.call(r, http.MethodPost, "/api/v1/stage-versions/"+dbV1ID.String()+"/clone", "", http.StatusConflict))
		assertGolden(t, "error_in_use.json", e.call(r, http.MethodDelete, "/api/v1/stage-versions/"+dbV1ID.String(), "", http.StatusConflict))
		assertGolden(t, "error_version_immutable.json", e.call(r, http.MethodPatch,
			"/api/v1/stage-versions/"+dbV1ID.String()+"/lessons/"+testdb.LessonDBTableV1ID, `{"title":"Sửa"}`, http.StatusConflict))
	})
	t.Run("stage detail có khóa học dùng bản cũ", func(t *testing.T) {
		e := outdatedScenario(t)
		assertGolden(t, "stage_detail.json", e.call(e.router(), http.MethodGet, "/api/v1/stages/"+stageDBID.String(), "", http.StatusOK))
	})
	t.Run("lesson video và markdown", func(t *testing.T) {
		e := newIT(t, "stage_db_draft")
		r := e.router()
		video := e.addMedia(media.KindVideo, "video/mp4", "join-co-ban.mp4")
		assertGolden(t, "lesson_video.json", e.call(r, http.MethodPost, "/api/v1/stage-versions/"+dbV2ID.String()+"/lessons",
			`{"unitId":"`+testdb.UnitDBV2ID+`","title":"Video: JOIN cơ bản","type":"video","required":true,"durationSeconds":540,"videoMediaId":"`+video.String()+`"}`, http.StatusCreated))
		assertGolden(t, "lesson_markdown.json", e.call(r, http.MethodPost, "/api/v1/stage-versions/"+dbV2ID.String()+"/lessons",
			`{"unitId":"`+testdb.UnitDBV2ID+`","lessonKey":"db-join-notes","title":"Ghi chú JOIN","type":"markdown","required":false,"markdownSource":"# JOIN\n\nINNER và LEFT JOIN."}`, http.StatusCreated))
		assertGolden(t, "error_validation.json", e.call(r, http.MethodPost, "/api/v1/stage-versions/"+dbV2ID.String()+"/lessons",
			`{"unitId":"`+testdb.UnitDBV2ID+`","title":"Video thiếu file","type":"video"}`, http.StatusUnprocessableEntity))
		assertGolden(t, "delete_version.json", e.call(r, http.MethodDelete, "/api/v1/stage-versions/"+dbV2ID.String(), "", http.StatusOK))
	})
	t.Run("lesson quiz và homework", func(t *testing.T) {
		e := newIT(t, "stage_db_draft")
		r := e.router()
		base := "/api/v1/stage-versions/" + dbV2ID.String() + "/lessons"
		var quiz LessonDTO
		if err := json.Unmarshal(e.call(r, http.MethodPost, base, `{"unitId":"`+testdb.UnitDBV2ID+`","title":"Kiểm tra JOIN","type":"quiz","quiz":{"mode":"assessment","maxAttempts":2,"timeLimitMinutes":10,"passPercent":70}}`, http.StatusCreated), &quiz); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "lesson_quiz.json", e.call(r, http.MethodPost, base+"/"+quiz.ID+"/questions",
			`{"questionKey":"join-left","type":"single","bodySource":"LEFT JOIN giữ dòng nào?","explanationSource":"Giữ mọi dòng bảng trái.","points":1,"options":[{"body":"Mọi dòng bảng trái","correct":true},{"body":"Chỉ dòng khớp"}]}`, http.StatusCreated))
		doc := e.addMedia(media.KindDocument, "application/zip", "starter.zip")
		var hw LessonDTO
		if err := json.Unmarshal(e.call(r, http.MethodPost, base, `{"unitId":"`+testdb.UnitDBV2ID+`","title":"Bài tập JOIN","type":"homework","homework":{"briefSource":"# Viết truy vấn JOIN","passPercent":70,"attachmentMediaIds":["`+doc.String()+`"]}}`, http.StatusCreated), &hw); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "lesson_homework.json", e.call(r, http.MethodPost, base+"/"+hw.ID+"/criteria",
			`{"description":"Truy vấn chạy đúng","maxPoints":60}`, http.StatusCreated))
	})
	t.Run("xem trước markdown bài seed", func(t *testing.T) {
		e := newIT(t)
		body, err := json.Marshal(map[string]string{"markdownSource": firstSeedMarkdown(t)})
		if err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "markdown_preview.json", e.call(e.router(), http.MethodPost, "/api/v1/stages/markdown-preview", string(body), http.StatusOK))
	})
}

// firstSeedMarkdown đọc nguồn bài markdown seed đầu tiên (tiêu đề, danh sách, khối mã) từ golden của package seed;
// không import seed vì seed đã import stages.
func firstSeedMarkdown(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "seed", "testdata", "seed_markdown.json")) //nolint:gosec // đường dẫn cố định trong repo
	if err != nil {
		t.Fatal(err)
	}
	var docs []struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(raw, &docs); err != nil || len(docs) == 0 {
		t.Fatalf("seed_markdown.json: %v (%d bài)", err, len(docs))
	}
	return docs[0].Source
}

// contentVersion là phiên bản có một quiz đánh giá (hai câu hỏi) và một homework (đề, hai tiêu chí, một file kèm).
type contentVersion struct {
	vid, quiz, homework, doc uuid.UUID
}

func (e *itEnv) quizHomeworkVersion(code string, publish bool) contentVersion {
	e.t.Helper()
	cv := contentVersion{vid: e.createStage(code), doc: e.addMedia(media.KindDocument, "application/zip", "starter.zip")}
	unit := e.firstUnit(cv.vid)
	quiz, err := e.svc.AddLesson(e.ctx(), e.actor, cv.vid, LessonCmd{UnitID: unit, Title: strp("Kiểm tra"), Type: strp("quiz"),
		Quiz: &QuizCmd{Mode: strp("assessment"), TimeLimitMinutes: intp(10)}})
	if err != nil {
		e.t.Fatal(err)
	}
	cv.quiz = quiz.ID()
	for _, body := range []string{"Khóa chính là gì?", "Khóa ngoại là gì?"} {
		if _, err := e.svc.AddQuestion(e.ctx(), e.actor, cv.vid, cv.quiz, questionCmd(body)); err != nil {
			e.t.Fatal(err)
		}
	}
	hw, err := e.svc.AddLesson(e.ctx(), e.actor, cv.vid, LessonCmd{UnitID: unit, Title: strp("Bài tập"), Type: strp("homework"),
		Homework: &HomeworkCmd{BriefSource: strp("# Thiết kế lược đồ"), AttachmentMediaIDs: []uuid.UUID{cv.doc}}})
	if err != nil {
		e.t.Fatal(err)
	}
	cv.homework = hw.ID()
	for _, c := range []CriterionCmd{{Description: "Đủ bảng", MaxPoints: 60}, {Description: "Khóa ngoại đúng", MaxPoints: 40}} {
		if _, err := e.svc.AddCriterion(e.ctx(), e.actor, cv.vid, cv.homework, c); err != nil {
			e.t.Fatal(err)
		}
	}
	if publish {
		if _, err := e.svc.Publish(e.ctx(), e.actor, cv.vid); err != nil {
			e.t.Fatal(err)
		}
	}
	return cv
}

// storedContent đọc lại nội dung học liệu từ DB.
func (e *itEnv) storedContent(vid, lid uuid.UUID) LessonContent {
	e.t.Helper()
	v, err := e.repo.ByID(e.ctx(), e.dbx, vid)
	if err != nil {
		e.t.Fatal(err)
	}
	l, err := v.Lesson(lid)
	if err != nil {
		e.t.Fatal(err)
	}
	return l.Content()
}

func TestQuizHomeworkPersistAndDiff(t *testing.T) {
	e := newIT(t)
	cv := e.quizHomeworkVersion("QZ", false)

	quiz := e.storedContent(cv.vid, cv.quiz).(QuizContent)
	if quiz.Mode != QuizAssessment || *quiz.MaxAttempts != 2 || *quiz.TimeLimit != 10*time.Minute || *quiz.PassPercent != 70 || !quiz.Shuffle {
		t.Fatalf("quiz settings = %+v", quiz)
	}
	if n := e.count(`SELECT time_limit_seconds FROM quiz_settings WHERE lesson_id = $1`, cv.quiz); n != 600 {
		t.Fatalf("time_limit_seconds = %d", n)
	}
	if len(quiz.Questions) != 2 || quiz.Questions[0].Key != "cau-hoi" || quiz.Questions[1].Key != "cau-hoi-2" ||
		!quiz.Questions[0].Options[0].Correct || quiz.Questions[0].Options[1].Correct || quiz.Questions[0].ExplanationSource != "Vì vậy." {
		t.Fatalf("questions = %+v %+v", quiz.Questions[0], quiz.Questions[1])
	}
	hw := e.storedContent(cv.vid, cv.homework).(HomeworkContent)
	if hw.BriefSource != "# Thiết kế lược đồ" || hw.PassPercent != 70 || hw.MaxScore() != 100 || len(hw.Attachments) != 1 ||
		hw.Attachments[0] != (Attachment{MediaID: cv.doc, FileName: "starter.zip", SizeBytes: 1024}) {
		t.Fatalf("homework = %+v", hw)
	}
	if got := e.lessonMedia(cv.homework); len(got) != 1 || got[0] != cv.doc {
		t.Fatalf("homework lesson_media = %v", got)
	}

	// Sửa câu 1 (giữ một phương án, thêm một), thêm câu 3, rồi đảo thứ tự: id giữ nguyên, vị trí đổi không vướng UNIQUE.
	q1, q2 := quiz.Questions[0], quiz.Questions[1]
	keep := q1.Options[1].ID
	upd := QuestionCmd{Type: "multi", BodySource: "Khóa chính?", Points: 2,
		Options: []OptionCmd{{ID: &keep, Body: "Định danh", Correct: true}, {Body: "Duy nhất", Correct: true}, {Body: "Tùy chọn"}}}
	l, err := e.svc.UpdateQuestion(e.ctx(), e.actor, cv.vid, cv.quiz, q1.ID, upd)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.storedContent(cv.vid, cv.quiz), l.Content()) {
		t.Fatalf("stored quiz differs from aggregate after update")
	}
	if n := e.count(`SELECT count(*) FROM question_options WHERE id = $1`, q1.Options[0].ID); n != 0 {
		t.Fatal("dropped option still stored")
	}
	if l, err = e.svc.AddQuestion(e.ctx(), e.actor, cv.vid, cv.quiz, questionCmd("Câu 3")); err != nil {
		t.Fatal(err)
	}
	q3 := l.Content().(QuizContent).Questions[2]
	if l, err = e.svc.ReorderQuestions(e.ctx(), e.actor, cv.vid, cv.quiz, []uuid.UUID{q3.ID, q1.ID, q2.ID}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.storedContent(cv.vid, cv.quiz), l.Content()) {
		t.Fatalf("stored quiz differs from aggregate after reorder")
	}
	if err := e.svc.RemoveQuestion(e.ctx(), e.actor, cv.vid, cv.quiz, q2.ID); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM question_options WHERE question_id = $1`, q2.ID); n != 0 {
		t.Fatal("options of removed question still stored")
	}
	if l, err = e.svc.UpdateLesson(e.ctx(), e.actor, cv.vid, cv.quiz, LessonCmd{Quiz: &QuizCmd{Mode: strp("practice"), Shuffle: boolp(false)}}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.storedContent(cv.vid, cv.quiz), l.Content()) {
		t.Fatalf("stored quiz differs from aggregate after settings")
	}

	hwl, err := e.svc.ReorderCriteria(e.ctx(), e.actor, cv.vid, cv.homework, []uuid.UUID{hw.Criteria[1].ID, hw.Criteria[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.storedContent(cv.vid, cv.homework), hwl.Content()) {
		t.Fatalf("stored homework differs from aggregate after reorder")
	}

	// Bỏ file kèm thì lesson_media cũng bỏ; đổi loại học liệu xóa sạch dòng con của loại cũ.
	if _, err := e.svc.UpdateLesson(e.ctx(), e.actor, cv.vid, cv.homework, LessonCmd{Homework: &HomeworkCmd{AttachmentMediaIDs: []uuid.UUID{}}}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM homework_attachments WHERE lesson_id = $1`, cv.homework); n != 0 || len(e.lessonMedia(cv.homework)) != 0 {
		t.Fatalf("attachments = %d, lesson_media = %v", n, e.lessonMedia(cv.homework))
	}
	for _, lid := range []uuid.UUID{cv.quiz, cv.homework} {
		if _, err := e.svc.UpdateLesson(e.ctx(), e.actor, cv.vid, lid, LessonCmd{Type: strp("markdown"), MarkdownSource: strp("# Đọc")}); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"quiz_settings", "questions", "homework_settings", "rubric_criteria"} {
		if n := e.count(`SELECT count(*) FROM `+table+` WHERE lesson_id = ANY($1)`, []uuid.UUID{cv.quiz, cv.homework}); n != 0 {
			t.Errorf("%s rows left after type change = %d", table, n)
		}
	}
	if n := e.count(`SELECT count(*) FROM question_options o JOIN questions q ON q.id = o.question_id WHERE q.lesson_id = $1`, cv.quiz); n != 0 {
		t.Fatalf("options left = %d", n)
	}
}

func TestQuestionKeyUniqueOnRealSchema(t *testing.T) {
	e := newIT(t)
	cv := e.quizHomeworkVersion("QZ", false)
	cmd := questionCmd("Trùng mã")
	cmd.QuestionKey = strp("cau-hoi")
	if _, err := e.svc.AddQuestion(e.ctx(), e.actor, cv.vid, cv.quiz, cmd); !errors.Is(err, ErrQuestionKeyTaken) {
		t.Fatalf("err = %v", err)
	}
	// Bỏ qua aggregate: chính ràng buộc DB cũng chặn và được dịch sang lỗi nghiệp vụ.
	err := e.inTx(func(tx db.Executor) error {
		v, err := e.repo.ByID(e.ctx(), tx, cv.vid)
		if err != nil {
			return err
		}
		l, _ := v.Lesson(cv.quiz)
		c := l.content.(QuizContent)
		dup := *c.Questions[0]
		dup.ID, dup.Options = ids.New(), []Option{{ID: ids.New(), Body: "A", Correct: true}, {ID: ids.New(), Body: "B"}}
		c.Questions = append(c.Questions, &dup)
		l.content = c
		return e.repo.SaveDraft(e.ctx(), tx, v)
	})
	if !errors.Is(err, ErrQuestionKeyTaken) {
		t.Fatalf("repo err = %v", err)
	}
}

// TestImmutability_QuizHomeworkContent ghi thẳng repository lên bản đã phát hành. Snapshot chụp ngay trong cùng
// transaction sau lỗi, nên câu ghi bảng con nào thiếu điều kiện draft sẽ lộ ra dù header ghi sau cùng cũng bị chặn.
func TestImmutability_QuizHomeworkContent(t *testing.T) {
	quizOf := func(v *StageVersion, lid uuid.UUID, fn func(c *QuizContent)) {
		l, _ := v.Lesson(lid)
		c := l.content.(QuizContent)
		fn(&c)
		l.content = c
	}
	homeworkOf := func(v *StageVersion, lid uuid.UUID, fn func(c *HomeworkContent)) {
		l, _ := v.Lesson(lid)
		c := l.content.(HomeworkContent)
		fn(&c)
		l.content = c
	}
	cases := map[string]func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error{
		"insert question": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			quizOf(v, cv.quiz, func(c *QuizContent) {
				html := "<p>x</p>"
				c.Questions = append(c.Questions, &Question{ID: ids.New(), Key: "chen-them", Type: QuestionSingle, BodySource: "x",
					BodyHTML: &html, ExplanationHTML: &html, Points: 1,
					Options: []Option{{ID: ids.New(), Body: "A", Correct: true}, {ID: ids.New(), Body: "B"}}})
			})
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"update question": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			quizOf(v, cv.quiz, func(c *QuizContent) { c.Questions[0].BodySource, c.Questions[0].Points = "Sửa", 50 })
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"delete question": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			quizOf(v, cv.quiz, func(c *QuizContent) { c.Questions = c.Questions[:1] })
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"reorder questions": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			quizOf(v, cv.quiz, func(c *QuizContent) { c.Questions[0], c.Questions[1] = c.Questions[1], c.Questions[0] })
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"insert option": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			quizOf(v, cv.quiz, func(c *QuizContent) {
				c.Questions[0].Options = append(c.Questions[0].Options, Option{ID: ids.New(), Body: "Thêm"})
			})
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"update option": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			quizOf(v, cv.quiz, func(c *QuizContent) { c.Questions[0].Options[1].Correct = true })
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"delete option": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			quizOf(v, cv.quiz, func(c *QuizContent) { c.Questions[0].Options = c.Questions[0].Options[:1] })
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"quiz settings": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			quizOf(v, cv.quiz, func(c *QuizContent) { c.MaxAttempts = intp(9) })
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"homework settings": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			homeworkOf(v, cv.homework, func(c *HomeworkContent) { c.PassPercent = 90 })
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"insert criterion": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			homeworkOf(v, cv.homework, func(c *HomeworkContent) {
				c.Criteria = append(c.Criteria, &Criterion{ID: ids.New(), Description: "Thêm", MaxPoints: 5})
			})
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"update criterion": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			homeworkOf(v, cv.homework, func(c *HomeworkContent) { c.Criteria[0].MaxPoints = 1 })
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"delete criterion": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			homeworkOf(v, cv.homework, func(c *HomeworkContent) { c.Criteria = c.Criteria[1:] })
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		"replace attachments": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			doc := e.addMedia(media.KindDocument, "application/pdf", "de.pdf")
			homeworkOf(v, cv.homework, func(c *HomeworkContent) { c.Attachments = []Attachment{{MediaID: doc}} })
			return e.repo.SaveDraft(e.ctx(), tx, v)
		},
		// saveContent gọi thẳng để câu xóa dòng con khi đổi loại chạy trước mọi câu ghi lessons.
		"drop quiz rows": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			l, _ := v.Lesson(cv.quiz)
			old := l.content
			html := "<p>x</p>"
			l.content = MarkdownContent{Source: "x", HTML: &html}
			return saveContent(e.ctx(), tx, l, old)
		},
		"drop homework rows": func(e *itEnv, cv contentVersion, tx db.Executor, v *StageVersion) error {
			l, _ := v.Lesson(cv.homework)
			old := l.content
			l.content = NewQuizContent()
			return saveContent(e.ctx(), tx, l, old)
		},
	}
	e := newIT(t)
	cv := e.quizHomeworkVersion("QZ", true)
	before := e.versionSnapshot(cv.vid, true)
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			e.t = t
			var writeErr error
			var during string
			err := e.inTx(func(tx db.Executor) error {
				v, err := e.repo.ByID(e.ctx(), tx, cv.vid)
				if err != nil {
					return err
				}
				writeErr = fn(e, cv, tx, v)
				during = e.snapshotOn(tx, cv.vid, true)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if !errors.Is(writeErr, ErrVersionImmutable) {
				t.Fatalf("err = %v, want ErrVersionImmutable", writeErr)
			}
			if during != before {
				t.Fatalf("published version changed inside tx:\nbefore %s\nduring %s", before, during)
			}
		})
	}
}

func TestImmutability_QuizHomeworkServiceMutations(t *testing.T) {
	e := newIT(t)
	cv := e.quizHomeworkVersion("QZ", true)
	before := e.versionSnapshot(cv.vid, true)
	quiz := e.storedContent(cv.vid, cv.quiz).(QuizContent)
	hw := e.storedContent(cv.vid, cv.homework).(HomeworkContent)
	q, cr := quiz.Questions[0].ID, hw.Criteria[0].ID
	errs := map[string]error{}
	_, errs["addQuestion"] = e.svc.AddQuestion(e.ctx(), e.actor, cv.vid, cv.quiz, questionCmd("Thêm"))
	_, errs["updateQuestion"] = e.svc.UpdateQuestion(e.ctx(), e.actor, cv.vid, cv.quiz, q, questionCmd("Sửa"))
	errs["removeQuestion"] = e.svc.RemoveQuestion(e.ctx(), e.actor, cv.vid, cv.quiz, q)
	_, errs["reorderQuestions"] = e.svc.ReorderQuestions(e.ctx(), e.actor, cv.vid, cv.quiz, []uuid.UUID{quiz.Questions[1].ID, q})
	_, errs["quizSettings"] = e.svc.UpdateLesson(e.ctx(), e.actor, cv.vid, cv.quiz, LessonCmd{Quiz: &QuizCmd{MaxAttempts: intp(5)}})
	_, errs["homeworkSettings"] = e.svc.UpdateLesson(e.ctx(), e.actor, cv.vid, cv.homework, LessonCmd{Homework: &HomeworkCmd{PassPercent: intp(90)}})
	_, errs["addCriterion"] = e.svc.AddCriterion(e.ctx(), e.actor, cv.vid, cv.homework, CriterionCmd{Description: "Thêm", MaxPoints: 5})
	_, errs["updateCriterion"] = e.svc.UpdateCriterion(e.ctx(), e.actor, cv.vid, cv.homework, cr, CriterionCmd{Description: "Sửa", MaxPoints: 5})
	errs["removeCriterion"] = e.svc.RemoveCriterion(e.ctx(), e.actor, cv.vid, cv.homework, cr)
	_, errs["reorderCriteria"] = e.svc.ReorderCriteria(e.ctx(), e.actor, cv.vid, cv.homework, []uuid.UUID{hw.Criteria[1].ID, cr})
	for name, err := range errs {
		if !errors.Is(err, ErrVersionImmutable) {
			t.Errorf("%s err = %v", name, err)
		}
	}
	if e.versionSnapshot(cv.vid, true) != before {
		t.Fatal("published quiz/homework changed through service")
	}
}

func TestPublishQuizHomeworkOnRealSchema(t *testing.T) {
	e := newIT(t)
	cv := e.quizHomeworkVersion("QZ", false)
	img := e.addMedia(media.KindImage, "image/png", "so-do.png")
	src := "Xem ![s](/api/v1/media/" + img.String() + "/content)"
	brief := "# Đề <script>x</script> " + src
	if _, err := e.svc.UpdateLesson(e.ctx(), e.actor, cv.vid, cv.homework, LessonCmd{Homework: &HomeworkCmd{BriefSource: &brief}}); err != nil {
		t.Fatal(err)
	}
	quiz := e.storedContent(cv.vid, cv.quiz).(QuizContent)
	if _, err := e.svc.UpdateQuestion(e.ctx(), e.actor, cv.vid, cv.quiz, quiz.Questions[0].ID, questionCmd(src)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Publish(e.ctx(), e.actor, cv.vid); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM questions WHERE lesson_id = $1 AND (body_html IS NULL OR explanation_html IS NULL)`, cv.quiz); n != 0 {
		t.Fatalf("unrendered questions = %d", n)
	}
	var html string
	if err := e.dbx.Get(&html, `SELECT body_html FROM questions WHERE lesson_id = $1 ORDER BY position LIMIT 1`, cv.quiz); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, img.String()) {
		t.Fatalf("question body_html = %s", html)
	}
	if err := e.dbx.Get(&html, `SELECT brief_html FROM homework_settings WHERE lesson_id = $1`, cv.homework); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<script") || !strings.Contains(html, "<h1") {
		t.Fatalf("brief_html = %s", html)
	}
	// lesson_media gom cả ảnh trong câu hỏi/đề bài lẫn file kèm, để quyền tải media đi theo lớp học.
	if got := e.lessonMedia(cv.quiz); len(got) != 1 || got[0] != img {
		t.Fatalf("quiz lesson_media = %v", got)
	}
	want := []uuid.UUID{cv.doc, img}
	sort.Slice(want, func(i, j int) bool { return want[i].String() < want[j].String() })
	if got := e.lessonMedia(cv.homework); !reflect.DeepEqual(got, want) {
		t.Fatalf("homework lesson_media = %v, want %v", got, want)
	}
}

func TestPublishBlocksIncompleteQuizOnRealSchema(t *testing.T) {
	e := newIT(t)
	vid := e.createStage("QZ")
	quiz, err := e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{UnitID: e.firstUnit(vid), Title: strp("Kiểm tra"), Type: strp("quiz")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Publish(e.ctx(), e.actor, vid); err == nil || err.Error() != `"Kiểm tra" chưa có câu hỏi.` {
		t.Fatalf("err = %v", err)
	}
	if _, err := e.svc.AddQuestion(e.ctx(), e.actor, vid, quiz.ID(), questionCmd("Câu 1")); err != nil {
		t.Fatal(err)
	}
	// Chốt chặn cuối ở DB: câu hỏi chưa render thì không chuyển được sang published.
	now := e.clk.Now()
	err = e.inTx(func(tx db.Executor) error {
		return e.repo.TransitionStatus(e.ctx(), tx, vid, domain.VersionDraft, domain.VersionPublished, &now)
	})
	if !errors.Is(err, ErrNotRendered) {
		t.Fatalf("transition err = %v", err)
	}
	if n := e.count(`SELECT count(*) FROM stage_versions WHERE id = $1 AND status = 'draft'`, vid); n != 1 {
		t.Fatal("version must stay draft")
	}
}

func TestCloneDeepCopiesQuizAndHomeworkOnRealSchema(t *testing.T) {
	e := newIT(t)
	cv := e.quizHomeworkVersion("QZ", true)
	before := e.versionSnapshot(cv.vid, true)
	c, err := e.svc.Clone(e.ctx(), e.actor, cv.vid)
	if err != nil {
		t.Fatal(err)
	}
	src, err := e.repo.ByID(e.ctx(), e.dbx, cv.vid)
	if err != nil {
		t.Fatal(err)
	}
	clone, err := e.repo.ByID(e.ctx(), e.dbx, c.Version.ID())
	if err != nil {
		t.Fatal(err)
	}
	sq, _ := src.Lesson(cv.quiz)
	cq := clone.Lessons()[sq.Position()-1]
	sqc, cqc := sq.Content().(QuizContent), cq.Content().(QuizContent)
	if cqc.Mode != sqc.Mode || *cqc.TimeLimit != *sqc.TimeLimit || *cqc.MaxAttempts != *sqc.MaxAttempts || len(cqc.Questions) != len(sqc.Questions) {
		t.Fatalf("clone quiz settings = %+v", cqc)
	}
	for i, sqn := range sqc.Questions {
		cqn := cqc.Questions[i]
		if cqn.ID == sqn.ID || cqn.Key != sqn.Key || cqn.BodySource != sqn.BodySource || cqn.BodyHTML == nil || *cqn.BodyHTML != *sqn.BodyHTML {
			t.Fatalf("question %d: %+v vs %+v", i, cqn, sqn)
		}
		for j, so := range sqn.Options {
			if co := cqn.Options[j]; co.ID == so.ID || co.Body != so.Body || co.Correct != so.Correct {
				t.Fatalf("option %d.%d: %+v vs %+v", i, j, co, so)
			}
		}
	}
	sh, _ := src.Lesson(cv.homework)
	ch := clone.Lessons()[sh.Position()-1]
	shc, chc := sh.Content().(HomeworkContent), ch.Content().(HomeworkContent)
	if chc.BriefSource != shc.BriefSource || chc.PassPercent != shc.PassPercent || !reflect.DeepEqual(chc.Attachments, shc.Attachments) {
		t.Fatalf("clone homework = %+v", chc)
	}
	for i, sc := range shc.Criteria {
		if cc := chc.Criteria[i]; cc.ID == sc.ID || cc.Description != sc.Description || cc.MaxPoints != sc.MaxPoints {
			t.Fatalf("criterion %d: %+v vs %+v", i, cc, sc)
		}
	}
	if got := e.lessonMedia(ch.ID()); len(got) != 1 || got[0] != cv.doc {
		t.Fatalf("clone homework lesson_media = %v", got)
	}

	// Sửa bản nháp nhân bản không chạm bản gốc.
	if _, err := e.svc.UpdateQuestion(e.ctx(), e.actor, c.Version.ID(), cq.ID(), cqc.Questions[0].ID, questionCmd("Sửa ở v2")); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RemoveCriterion(e.ctx(), e.actor, c.Version.ID(), ch.ID(), chc.Criteria[0].ID); err != nil {
		t.Fatal(err)
	}
	if e.versionSnapshot(cv.vid, true) != before {
		t.Fatal("editing the clone changed the published source")
	}
}

// TestQuizRoutesRequireAdmin: teacher không gọi được route soạn quiz/homework (kể cả PATCH học liệu), admin gọi được.
func TestQuizRoutesRequireAdmin(t *testing.T) {
	e := newIT(t)
	cv := e.quizHomeworkVersion("QZ", false)
	quiz := e.storedContent(cv.vid, cv.quiz).(QuizContent)
	hw := e.storedContent(cv.vid, cv.homework).(HomeworkContent)
	g := e.guardedRouter()
	lessons := "/api/v1/stage-versions/" + cv.vid.String() + "/lessons/"
	q, h := lessons+cv.quiz.String(), lessons+cv.homework.String()
	q1, q2 := quiz.Questions[0].ID.String(), quiz.Questions[1].ID.String()
	c1, c2 := hw.Criteria[0].ID.String(), hw.Criteria[1].ID.String()
	question := `{"type":"single","bodySource":"Câu mới","points":1,"options":[{"body":"A","correct":true},{"body":"B"}]}`
	routes := []struct{ method, path, body string }{
		{http.MethodPost, q + "/questions", question},
		{http.MethodPatch, q, `{"quiz":{"maxAttempts":5}}`},
		{http.MethodPut, q + "/questions/order", `{"questionIds":["` + q2 + `","` + q1 + `"]}`},
		{http.MethodPatch, q + "/questions/" + q1, question},
		{http.MethodPost, h + "/criteria", `{"description":"Thêm","maxPoints":5}`},
		{http.MethodPatch, h + "/criteria/" + c1, `{"description":"Sửa","maxPoints":5}`},
		{http.MethodPut, h + "/criteria/order", `{"criterionIds":["` + c2 + `","` + c1 + `"]}`},
		{http.MethodDelete, q + "/questions/" + q2, ""},
		{http.MethodDelete, h + "/criteria/" + c2, ""},
	}
	before := e.versionSnapshot(cv.vid, true)
	g.expectForbidden(g.login("huong.le@goup.vn"), routes)
	if e.versionSnapshot(cv.vid, true) != before {
		t.Fatal("teacher calls changed the draft")
	}
	admin := g.login("quan.tran@goup.vn")
	want := []int{http.StatusCreated, http.StatusOK, http.StatusUnprocessableEntity, http.StatusOK, http.StatusCreated, http.StatusOK,
		http.StatusUnprocessableEntity, http.StatusNoContent, http.StatusNoContent}
	for i, rt := range routes {
		// Sau khi admin thêm câu hỏi/tiêu chí, body thứ tự thiếu một id nên nhận 422: admin qua được guard.
		if w := g.call(admin, rt.method, rt.path, rt.body); w.Code != want[i] {
			t.Errorf("admin %s %s = %d, want %d: %s", rt.method, rt.path, w.Code, want[i], w.Body)
		}
	}
}

func boolp(b bool) *bool { return &b }
