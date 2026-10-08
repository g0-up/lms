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
	"lms/api/internal/features/media"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
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
		media.Limits{MaxVideoBytes: 1 << 30, MaxImageBytes: 1 << 20})
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

func (e *itEnv) addMarkdown(vid uuid.UUID, title, src string) *Lesson {
	e.t.Helper()
	l, err := e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{Title: &title, Type: strp("markdown"), MarkdownSource: &src})
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
	l, err := e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{Title: &title, Type: strp("video"), VideoMediaID: &mediaID, DurationSeconds: &seconds})
	if err != nil {
		e.t.Fatal(err)
	}
	return l
}

// versionSnapshot là toàn bộ header, học liệu và lesson_media của một phiên bản dưới dạng JSON để so trước/sau.
func (e *itEnv) versionSnapshot(vid uuid.UUID, withHeader bool) string {
	e.t.Helper()
	var s string
	header := `NULL`
	if withHeader {
		header = `(SELECT row_to_json(sv) FROM stage_versions sv WHERE sv.id = $1)`
	}
	err := e.dbx.Get(&s, `SELECT json_build_object(
		'header', `+header+`,
		'lessons', COALESCE((SELECT json_agg(l ORDER BY l.position) FROM lessons l WHERE l.stage_version_id = $1), '[]'),
		'media', COALESCE((SELECT json_agg(lm ORDER BY lm.lesson_id, lm.media_id) FROM lesson_media lm
		                   JOIN lessons l ON l.id = lm.lesson_id WHERE l.stage_version_id = $1), '[]'))::text`, vid)
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
	_, errs["add"] = e.svc.AddLesson(e.ctx(), e.actor, dbV1ID, LessonCmd{Title: &title, Type: strp("markdown"), MarkdownSource: &title})
	_, errs["update"] = e.svc.UpdateLesson(e.ctx(), e.actor, dbV1ID, lid, LessonCmd{Title: &title})
	errs["remove"] = e.svc.RemoveLesson(e.ctx(), e.actor, dbV1ID, lid)
	_, errs["reorder"] = e.svc.ReorderLessons(e.ctx(), e.actor, dbV1ID, []uuid.UUID{uuid.MustParse(testdb.LessonDBIndexV1ID), lid})
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
	if _, err := e.svc.Publish(e.ctx(), e.actor, vid); !errors.Is(err, ErrNoLessons) {
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
		if e.count(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`) > 0 {
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
			_, addErr = e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{Title: &title, Type: strp("markdown"), MarkdownSource: &src})
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
				_, addErr = e.svc.AddLesson(e.ctx(), e.actor, vid, LessonCmd{Title: &title, Type: strp("markdown"), MarkdownSource: &src})
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

func TestReorderSwapsWithoutUniqueViolation(t *testing.T) {
	e := newIT(t, "stage_db_draft")
	a, b := uuid.MustParse(testdb.LessonDBTableV2ID), uuid.MustParse(testdb.LessonDBIndexV2ID)
	d, err := e.svc.ReorderLessons(e.ctx(), e.actor, dbV2ID, []uuid.UUID{b, a})
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Version.Lessons(); got[0].ID() != b || got[0].Position() != 1 || got[1].ID() != a || got[1].Position() != 2 {
		t.Fatal("order not swapped")
	}
	if _, err := e.svc.ReorderLessons(e.ctx(), e.actor, dbV2ID, []uuid.UUID{a}); !errors.Is(err, ErrReorderMismatch) {
		t.Fatalf("mismatch err = %v", err)
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
			`{"title":"Video: JOIN cơ bản","type":"video","required":true,"durationSeconds":540,"videoMediaId":"`+video.String()+`"}`, http.StatusCreated))
		assertGolden(t, "lesson_markdown.json", e.call(r, http.MethodPost, "/api/v1/stage-versions/"+dbV2ID.String()+"/lessons",
			`{"lessonKey":"db-join-notes","title":"Ghi chú JOIN","type":"markdown","required":false,"markdownSource":"# JOIN\n\nINNER và LEFT JOIN."}`, http.StatusCreated))
		assertGolden(t, "error_validation.json", e.call(r, http.MethodPost, "/api/v1/stage-versions/"+dbV2ID.String()+"/lessons",
			`{"title":"Video thiếu file","type":"video"}`, http.StatusUnprocessableEntity))
		assertGolden(t, "delete_version.json", e.call(r, http.MethodDelete, "/api/v1/stage-versions/"+dbV2ID.String(), "", http.StatusOK))
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
