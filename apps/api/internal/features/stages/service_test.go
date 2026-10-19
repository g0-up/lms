package stages

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/media"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/storage"
)

// fakeTx chạy fn ngay, không rollback; test chỉ kiểm lỗi và thứ tự gọi repo.
type fakeTx struct{}

func (fakeTx) Transact(_ context.Context, fn func(tx db.Executor) error) error { return fn(nil) }

type fakeStages struct {
	rows  map[uuid.UUID]*StageRow
	calls *[]string
}

func (f *fakeStages) Create(_ context.Context, _ db.Executor, s *Stage) error {
	for _, r := range f.rows {
		if r.Code == s.Code().String() {
			return ErrCodeTaken
		}
	}
	f.rows[s.ID()] = &StageRow{ID: s.ID(), Code: s.Code().String(), Name: s.Name(), Description: s.Description(), CreatedAt: s.CreatedAt()}
	return nil
}

func (f *fakeStages) ByID(_ context.Context, _ db.Executor, id uuid.UUID) (*StageRow, error) {
	r, ok := f.rows[id]
	if !ok {
		return nil, ErrStageNotFound
	}
	cp := *r
	return &cp, nil
}

func (f *fakeStages) Delete(_ context.Context, _ db.Executor, id uuid.UUID) error {
	*f.calls = append(*f.calls, "stages.Delete")
	delete(f.rows, id)
	return nil
}

func (f *fakeStages) LockForUpdate(context.Context, db.Executor, uuid.UUID) error {
	*f.calls = append(*f.calls, "stages.LockForUpdate")
	return nil
}

func (f *fakeStages) List(context.Context, db.Executor, ListQuery) ([]StageListRow, error) {
	return nil, nil
}

// fakeVersions giữ bản sao sâu của phiên bản để thay đổi chưa SaveDraft không lọt vào "DB".
type fakeVersions struct {
	byID   map[uuid.UUID]*StageVersion
	usedBy map[uuid.UUID][]UsedByRow
	calls  *[]string
}

func snapshot(v *StageVersion) *StageVersion {
	cp := *v
	cp.units = make([]*Unit, 0, len(v.units))
	for _, u := range v.units {
		uc := *u
		cp.units = append(cp.units, &uc)
	}
	cp.lessons = make([]*Lesson, 0, len(v.lessons))
	for _, l := range v.lessons {
		lc := *l
		lc.content = copyContent(l.content)
		cp.lessons = append(cp.lessons, &lc)
	}
	return &cp
}

func (f *fakeVersions) Create(_ context.Context, _ db.Executor, v *StageVersion) error {
	*f.calls = append(*f.calls, "versions.Create")
	f.byID[v.ID()] = snapshot(v)
	return nil
}

func (f *fakeVersions) SaveDraft(_ context.Context, _ db.Executor, v *StageVersion) error {
	*f.calls = append(*f.calls, "versions.SaveDraft")
	cur, ok := f.byID[v.ID()]
	if !ok || cur.Status() != domain.VersionDraft {
		return ErrVersionImmutable
	}
	saved := snapshot(v)
	saved.status = cur.status
	f.byID[v.ID()] = saved
	return nil
}

func (f *fakeVersions) TransitionStatus(_ context.Context, _ db.Executor, id uuid.UUID, from, to domain.VersionStatus, at *time.Time) error {
	*f.calls = append(*f.calls, "versions.TransitionStatus")
	cur, ok := f.byID[id]
	if !ok || cur.status != from {
		return domain.ErrInvalidTransition
	}
	cur.status = to
	if to == domain.VersionPublished {
		cur.publishedAt = at
	}
	return nil
}

func (f *fakeVersions) Delete(_ context.Context, _ db.Executor, id uuid.UUID) error {
	*f.calls = append(*f.calls, "versions.Delete")
	delete(f.byID, id)
	return nil
}

func (f *fakeVersions) ByID(_ context.Context, _ db.Executor, id uuid.UUID) (*StageVersion, error) {
	v, ok := f.byID[id]
	if !ok {
		return nil, ErrVersionNotFound
	}
	return snapshot(v), nil
}

func (f *fakeVersions) ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*StageVersion, error) {
	*f.calls = append(*f.calls, "versions.ByIDForUpdate")
	return f.ByID(ctx, ex, id)
}

func (f *fakeVersions) ListByStage(_ context.Context, _ db.Executor, stageID uuid.UUID) ([]VersionSummary, error) {
	var out []VersionSummary
	for _, v := range f.byID {
		if v.StageID() == stageID {
			out = append(out, VersionSummary{ID: v.ID(), StageID: stageID, VersionNo: v.VersionNo(), Status: v.Status(), LessonCount: len(v.lessons)})
		}
	}
	return out, nil
}

func (f *fakeVersions) CountByStage(ctx context.Context, ex db.Executor, stageID uuid.UUID) (int, error) {
	l, _ := f.ListByStage(ctx, ex, stageID)
	return len(l), nil
}

func (f *fakeVersions) NextVersionNo(ctx context.Context, ex db.Executor, stageID uuid.UUID) (domain.VersionNo, error) {
	l, _ := f.ListByStage(ctx, ex, stageID)
	var maxNo domain.VersionNo
	for _, s := range l {
		maxNo = max(maxNo, s.VersionNo)
	}
	return maxNo.Next(), nil
}

func (f *fakeVersions) DraftOf(ctx context.Context, ex db.Executor, stageID uuid.UUID) (*VersionSummary, error) {
	l, _ := f.ListByStage(ctx, ex, stageID)
	for _, s := range l {
		if s.Status == domain.VersionDraft {
			return &s, nil
		}
	}
	return nil, nil
}

func (f *fakeVersions) UsedBy(_ context.Context, _ db.Executor, id uuid.UUID) ([]UsedByRow, error) {
	return f.usedBy[id], nil
}

func (f *fakeVersions) UsedByStage(context.Context, db.Executor, uuid.UUID) ([]StageUsedByRow, error) {
	return nil, nil
}

func (f *fakeVersions) OutdatedCourses(context.Context, db.Executor, uuid.UUID) ([]OutdatedCourse, error) {
	return nil, nil
}

func (f *fakeVersions) AllOutdated(context.Context, db.Executor) ([]OutdatedCourse, error) {
	return nil, nil
}

type fakeMedia map[uuid.UUID]*media.MediaFile

func (f fakeMedia) ByID(_ context.Context, _ db.Executor, id uuid.UUID) (*media.MediaFile, error) {
	m, ok := f[id]
	if !ok {
		return nil, media.ErrMediaNotFound
	}
	return m, nil
}

type fakeAudit struct{ entries []audit.Entry }

func (f *fakeAudit) Record(_ context.Context, _ db.Executor, e audit.Entry) error {
	f.entries = append(f.entries, e)
	return nil
}

type seqIDs struct{}

func (seqIDs) New() uuid.UUID { return uuid.New() }

type env struct {
	svc      *Service
	stages   *fakeStages
	versions *fakeVersions
	media    fakeMedia
	audit    *fakeAudit
	calls    *[]string
	actor    Actor
}

func newEnv(t *testing.T) *env {
	t.Helper()
	calls := &[]string{}
	e := &env{
		stages:   &fakeStages{rows: map[uuid.UUID]*StageRow{}, calls: calls},
		versions: &fakeVersions{byID: map[uuid.UUID]*StageVersion{}, usedBy: map[uuid.UUID][]UsedByRow{}, calls: calls},
		media:    fakeMedia{},
		audit:    &fakeAudit{},
		calls:    calls,
		actor:    Actor{ID: uuid.New(), RequestID: "req-1"},
	}
	e.svc = NewService(Deps{
		Tx: fakeTx{}, Stages: e.stages, Versions: e.versions, Media: e.media, Render: NewMarkdownRenderer(),
		Clock: &clock.Fake{T: testNow}, Audit: e.audit, IDs: seqIDs{},
	})
	return e
}

func (e *env) addMedia(t *testing.T, kind media.Kind, ct, name string, ready bool) uuid.UUID {
	t.Helper()
	limits := media.Limits{MaxVideoBytes: 1 << 30, MaxImageBytes: 1 << 20, MaxDocumentBytes: 1 << 20}
	m, err := media.NewPendingUpload(uuid.New(), kind, name, ct, 100, e.actor.ID, testNow, limits)
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		if err := m.MarkReady(storage.ObjectStat{Size: 100, ContentType: ct}, testNow); err != nil {
			t.Fatal(err)
		}
	}
	e.media[m.ID()] = m
	return m.ID()
}

// createDraft tạo chặng và trả id bản nháp v1.
func (e *env) createDraft(t *testing.T, code string) uuid.UUID {
	t.Helper()
	d, err := e.svc.CreateStage(context.Background(), e.actor, CreateStageCmd{Code: code, Name: "Chặng " + code})
	if err != nil {
		t.Fatal(err)
	}
	return d.Versions[0].ID
}

func strp(s string) *string { return &s }

func (e *env) addUnit(t *testing.T, vid uuid.UUID, title string) uuid.UUID {
	t.Helper()
	d, err := e.svc.AddUnit(context.Background(), e.actor, vid, title)
	if err != nil {
		t.Fatalf("AddUnit: %v", err)
	}
	return d.Unit.ID()
}

// inFirstUnit gắn lệnh vào bài học đầu của bản nháp, tạo bài học "Bài 1" nếu chưa có.
func (e *env) inFirstUnit(t *testing.T, vid uuid.UUID, cmd LessonCmd) LessonCmd {
	t.Helper()
	var unit uuid.UUID
	if units := e.versions.byID[vid].Units(); len(units) > 0 {
		unit = units[0].ID()
	} else {
		unit = e.addUnit(t, vid, "Bài 1")
	}
	cmd.UnitID = &unit
	return cmd
}

// layout đọc bố cục đã lưu của phiên bản dạng "bài học[học liệu@position …] …".
func (e *env) layout(vid uuid.UUID) string {
	return layoutOf(e.versions.byID[vid])
}

func markdownCmd(title, src string) LessonCmd {
	return LessonCmd{Title: strp(title), Type: strp("markdown"), MarkdownSource: strp(src)}
}

// addLegacyMarkdown thêm bài markdown rồi ghi đè nguồn thẳng trong repo giả, giả lập dữ liệu lưu trước khi có luật
// ảnh ngoài.
func (e *env) addLegacyMarkdown(t *testing.T, vid uuid.UUID, title, src string) uuid.UUID {
	t.Helper()
	l, err := e.svc.AddLesson(context.Background(), e.actor, vid, e.inFirstUnit(t, vid, markdownCmd(title, "# Tạm")))
	if err != nil {
		t.Fatal(err)
	}
	stored := e.versions.byID[vid]
	for _, sl := range stored.lessons {
		if sl.ID() == l.ID() {
			sl.content = MarkdownContent{Source: src}
		}
	}
	return l.ID()
}

func (e *env) publish(t *testing.T, vid uuid.UUID) {
	t.Helper()
	if _, err := e.svc.AddLesson(context.Background(), e.actor, vid, e.inFirstUnit(t, vid, markdownCmd("Bài 1", "# Một"))); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Publish(context.Background(), e.actor, vid); err != nil {
		t.Fatal(err)
	}
}

func TestCreateStage(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, cmd := range []CreateStageCmd{{Code: "", Name: "A"}, {Code: "DB", Name: " "}} {
		if _, err := e.svc.CreateStage(ctx, e.actor, cmd); !errors.Is(err, ErrStageNameRequired) {
			t.Fatalf("%+v err = %v", cmd, err)
		}
	}
	if _, err := e.svc.CreateStage(ctx, e.actor, CreateStageCmd{Code: "db cơ bản", Name: "A"}); !errors.Is(err, domain.ErrInvalidCode) {
		t.Fatalf("bad code err = %v", err)
	}
	d, err := e.svc.CreateStage(ctx, e.actor, CreateStageCmd{Code: "DOCKER", Name: "Docker cơ bản", Description: "Container"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Versions) != 1 || d.Versions[0].Status != domain.VersionDraft || d.Versions[0].VersionNo != 1 {
		t.Fatalf("versions = %+v", d.Versions)
	}
	if len(e.audit.entries) != 1 || e.audit.entries[0].Action != audit.ActionStageCreated || e.audit.entries[0].RequestID != "req-1" {
		t.Fatalf("audit = %+v", e.audit.entries)
	}
	if _, err := e.svc.CreateStage(ctx, e.actor, CreateStageCmd{Code: "DOCKER", Name: "Khác"}); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("duplicate err = %v", err)
	}
}

func TestAddLessonContentRules(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	readyVideo := e.addMedia(t, media.KindVideo, "video/mp4", "bang.mp4", true)
	pendingVideo := e.addMedia(t, media.KindVideo, "video/mp4", "cho.mp4", false)
	image := e.addMedia(t, media.KindImage, "image/png", "anh.png", true)

	videoCmd := func(id uuid.UUID) LessonCmd {
		return LessonCmd{Title: strp("Video"), Type: strp("video"), VideoMediaID: &id, DurationSeconds: intPtr(90)}
	}
	tests := []struct {
		name string
		cmd  LessonCmd
		want error
	}{
		{"thiếu loại", LessonCmd{Title: strp("A")}, ErrInvalidLessonType},
		{"loại lạ", LessonCmd{Title: strp("A"), Type: strp("pdf")}, ErrInvalidLessonType},
		{"video không có file", LessonCmd{Title: strp("A"), Type: strp("video")}, ErrVideoRequired},
		{"video chưa upload xong", videoCmd(pendingVideo), ErrVideoRequired},
		{"media là ảnh", videoCmd(image), ErrVideoRequired},
		{"media không tồn tại", videoCmd(uuid.New()), ErrVideoRequired},
		{"tiêu đề trống", markdownCmd(" ", "x"), ErrLessonTitleRequired},
		{"markdown trống", markdownCmd("A", ""), ErrMarkdownRequired},
		{"ảnh base64", markdownCmd("A", "![x](data:image/png;base64,AAA=)"), ErrEmbeddedImage},
		{"ảnh ngoài", markdownCmd("A", "![x](https://cdn.example/a.png)"), ErrExternalImage},
		{"ảnh đường dẫn khác", markdownCmd("A", "![x](/uploads/a.png)"), ErrExternalImage},
		{"ảnh ngoài dạng reference", markdownCmd("A", "![x][r]\n\n[r]: https://cdn.example/a.png"), ErrExternalImage},
		{"lessonKey sai", LessonCmd{LessonKey: strp("Bad Key"), Title: strp("A"), Type: strp("markdown"), MarkdownSource: strp("x")}, domain.ErrInvalidLessonKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := e.svc.AddLesson(ctx, e.actor, vid, e.inFirstUnit(t, vid, tt.cmd)); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}

	if _, err := e.svc.AddLesson(ctx, e.actor, vid, videoCmd(readyVideo)); !errors.Is(err, ErrLessonUnitRequired) {
		t.Fatalf("missing unit err = %v", err)
	}
	l, err := e.svc.AddLesson(ctx, e.actor, vid, e.inFirstUnit(t, vid, videoCmd(readyVideo)))
	if err != nil {
		t.Fatal(err)
	}
	vc := l.Content().(VideoContent)
	if vc.FileName != "bang.mp4" || *vc.DurationSeconds != 90 || !l.Required() {
		t.Fatalf("video lesson = %+v required=%v", vc, l.Required())
	}
	stored, _ := e.versions.ByID(ctx, nil, vid)
	if len(stored.Lessons()) != 1 {
		t.Fatalf("stored lessons = %d", len(stored.Lessons()))
	}

	// Ảnh media nội bộ (kể cả chưa upload xong) và thẻ <img> HTML thô vẫn lưu được; HTML thô bị lọc lúc render.
	withMedia := "# Sơ đồ\n\n![sơ đồ](/api/v1/media/" + image.String() + "/content)\n\n<img src=\"x\" onerror=\"alert(1)\">"
	if _, err := e.svc.AddLesson(ctx, e.actor, vid, e.inFirstUnit(t, vid, markdownCmd("Ảnh", withMedia))); err != nil {
		t.Fatalf("media image rejected: %v", err)
	}
}

func TestUpdateLessonImageRuleOnlyForNewSource(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	legacy := "# Cũ\n\n![x](https://cdn.example/a.png)"
	lid := e.addLegacyMarkdown(t, vid, "Bài cũ", legacy)

	no := false
	u, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{Title: strp("Bài cũ 2"), Required: &no})
	if err != nil {
		t.Fatalf("title/required update on legacy source must pass: %v", err)
	}
	if u.Title() != "Bài cũ 2" || u.Content().(MarkdownContent).Source != legacy {
		t.Fatalf("legacy update = %q %q", u.Title(), u.Content().(MarkdownContent).Source)
	}
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{MarkdownSource: strp(legacy)}); !errors.Is(err, ErrExternalImage) {
		t.Fatalf("new source with external image err = %v", err)
	}
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{MarkdownSource: strp("![x](data:image/png;base64,AAA=)")}); !errors.Is(err, ErrEmbeddedImage) {
		t.Fatalf("new source with base64 err = %v", err)
	}
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{MarkdownSource: strp("# Mới")}); err != nil {
		t.Fatalf("clean source err = %v", err)
	}

	// Kiểm base64 vẫn chạy trên nội dung đang có như trước.
	b64 := e.addLegacyMarkdown(t, vid, "Base64", "![x](data:image/png;base64,AAA=)")
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, b64, LessonCmd{Title: strp("Đổi tên")}); !errors.Is(err, ErrEmbeddedImage) {
		t.Fatalf("legacy base64 err = %v", err)
	}
}

func TestPreviewMarkdown(t *testing.T) {
	e := newEnv(t)
	html, err := e.svc.PreviewMarkdown("# Tiêu đề\n\n<script>alert(1)</script>\n\n![s](/api/v1/media/" + testMediaID + "/content)")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<h1>Tiêu đề</h1>") || strings.Contains(html, "<script") || !strings.Contains(html, testMediaID) {
		t.Fatalf("html = %s", html)
	}
	if _, err := e.svc.PreviewMarkdown("![x](https://x/a.png)"); !errors.Is(err, ErrExternalImage) {
		t.Fatalf("external image err = %v", err)
	}
	if _, err := e.svc.PreviewMarkdown("![x](data:image/png;base64,AAA=)"); !errors.Is(err, ErrEmbeddedImage) {
		t.Fatalf("base64 err = %v", err)
	}
	if html, err := e.svc.PreviewMarkdown(""); err != nil || html != "" {
		t.Fatalf("empty = %q %v", html, err)
	}
}

func TestUpdateLessonIsPartial(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	l, err := e.svc.AddLesson(ctx, e.actor, vid, e.inFirstUnit(t, vid, markdownCmd("Bảng", "# Bảng")))
	if err != nil {
		t.Fatal(err)
	}
	no := false
	u, err := e.svc.UpdateLesson(ctx, e.actor, vid, l.ID(), LessonCmd{Required: &no})
	if err != nil {
		t.Fatal(err)
	}
	if u.Title() != "Bảng" || u.Required() || u.Content().(MarkdownContent).Source != "# Bảng" {
		t.Fatalf("partial update lost fields: %q %v", u.Title(), u.Required())
	}
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, l.ID(), LessonCmd{LessonKey: strp("khac")}); !errors.Is(err, ErrLessonKeyImmutable) {
		t.Fatalf("change key err = %v", err)
	}
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, l.ID(), LessonCmd{LessonKey: strp(string(l.Key())), Title: strp("Bảng 2")}); err != nil {
		t.Fatalf("same key must be accepted: %v", err)
	}
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, uuid.New(), LessonCmd{Title: strp("x")}); !errors.Is(err, ErrLessonNotFound) {
		t.Fatalf("missing lesson err = %v", err)
	}
	video := e.addMedia(t, media.KindVideo, "video/mp4", "v.mp4", true)
	u, err = e.svc.UpdateLesson(ctx, e.actor, vid, l.ID(), LessonCmd{Type: strp("video"), VideoMediaID: &video})
	if err != nil {
		t.Fatal(err)
	}
	if vc, ok := u.Content().(VideoContent); !ok || vc.MediaID != video {
		t.Fatalf("type switch content = %#v", u.Content())
	}
}

func TestMutationsOnPublishedReturnImmutableFirst(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	e.publish(t, vid)
	stored, _ := e.versions.ByID(ctx, nil, vid)
	lid := stored.Lessons()[0].ID()
	uid := stored.Units()[0].ID()
	before := e.layout(vid)

	// Dữ liệu vào sai vẫn phải nhận VERSION_IMMUTABLE vì kiểm trạng thái trước.
	if _, err := e.svc.AddLesson(ctx, e.actor, vid, LessonCmd{}); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("add err = %v", err)
	}
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{Title: strp("")}); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("update err = %v", err)
	}
	if err := e.svc.RemoveLesson(ctx, e.actor, vid, lid); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("remove err = %v", err)
	}
	if _, err := e.svc.ReorderLessons(ctx, e.actor, vid, nil); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("reorder err = %v", err)
	}
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{UnitID: &uid}); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("move lesson err = %v", err)
	}
	if _, err := e.svc.AddUnit(ctx, e.actor, vid, ""); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("add unit err = %v", err)
	}
	if _, err := e.svc.RenameUnit(ctx, e.actor, vid, uid, "Đổi"); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("rename unit err = %v", err)
	}
	if _, err := e.svc.RemoveUnit(ctx, e.actor, vid, uid); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("remove unit err = %v", err)
	}
	if _, err := e.svc.ReorderUnits(ctx, e.actor, vid, nil); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("reorder units err = %v", err)
	}
	if got := e.layout(vid); got != before {
		t.Fatalf("published layout changed: %s, want %s", got, before)
	}
	if _, err := e.svc.Publish(ctx, e.actor, vid); !errors.Is(err, ErrPublishNotDraft) {
		t.Fatalf("publish twice err = %v", err)
	}
}

func TestPublishOrderAndAudit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	if _, err := e.svc.Publish(ctx, e.actor, vid); !errors.Is(err, ErrNoUnits) {
		t.Fatalf("empty publish err = %v", err)
	}
	empty := e.addUnit(t, vid, "Rỗng")
	if _, err := e.svc.Publish(ctx, e.actor, vid); err == nil || err.Error() != `Bài học "Rỗng" chưa có học liệu.` {
		t.Fatalf("empty unit publish err = %v", err)
	}
	img := e.addMedia(t, media.KindImage, "image/png", "a.png", true)
	// Ảnh ngoài chỉ còn trong dữ liệu cũ (lưu không qua được luật ảnh); phát hành vẫn phải lọc nó.
	src := "# Hi <script>alert(1)</script> ![x](https://x/a.png) ![s](/api/v1/media/" + img.String() + "/content)"
	e.addLegacyMarkdown(t, vid, "Giới thiệu", src)
	if _, err := e.svc.RenameUnit(ctx, e.actor, vid, empty, "Nhập môn"); err != nil {
		t.Fatal(err)
	}
	*e.calls = nil
	d, err := e.svc.Publish(ctx, e.actor, vid)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"versions.ByIDForUpdate", "versions.SaveDraft", "versions.TransitionStatus"}
	if strings.Join(*e.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v, want %v", *e.calls, want)
	}
	if d.Version.Status() != domain.VersionPublished || d.Version.PublishedAt() == nil {
		t.Fatalf("status = %s", d.Version.Status())
	}
	html := *d.Version.Lessons()[0].Content().(MarkdownContent).HTML
	if strings.Contains(html, "<script") || strings.Contains(html, "x/a.png") || !strings.Contains(html, img.String()) {
		t.Fatalf("html = %s", html)
	}
	last := e.audit.entries[len(e.audit.entries)-1]
	if last.Action != audit.ActionStageVersionPublished || last.TargetID == nil || *last.TargetID != vid {
		t.Fatalf("audit = %+v", last)
	}
}

func TestPublishRejectsImageNotUploaded(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	pending := e.addMedia(t, media.KindImage, "image/png", "a.png", false)
	if _, err := e.svc.AddLesson(ctx, e.actor, vid, e.inFirstUnit(t, vid, markdownCmd("Ảnh", "![s](/api/v1/media/"+pending.String()+"/content)"))); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.Publish(ctx, e.actor, vid)
	if !errors.Is(err, domain.ErrInvalid) || !strings.Contains(err.Error(), "chưa được tải lên") {
		t.Fatalf("err = %v", err)
	}
	if v, _ := e.versions.ByID(ctx, nil, vid); v.Status() != domain.VersionDraft {
		t.Fatal("version must stay draft")
	}
}

func TestCloneArchiveDelete(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	v1 := e.createDraft(t, "DB")
	if _, err := e.svc.Clone(ctx, e.actor, v1); !errors.Is(err, ErrCloneNotPublished) {
		t.Fatalf("clone draft err = %v", err)
	}
	e.publish(t, v1)

	c, err := e.svc.Clone(ctx, e.actor, v1)
	if err != nil {
		t.Fatal(err)
	}
	if c.Version.VersionNo() != 2 || c.Version.Status() != domain.VersionDraft || len(c.Version.Lessons()) != 1 {
		t.Fatalf("clone = v%d %s", c.Version.VersionNo(), c.Version.Status())
	}
	if got, want := e.layout(c.Version.ID()), e.layout(v1); got != want || c.Version.Units()[0].ID() == e.versions.byID[v1].Units()[0].ID() {
		t.Fatalf("clone layout = %s, want %s with new unit ids", got, want)
	}
	_, err = e.svc.Clone(ctx, e.actor, v1)
	var de *ErrDraftExists
	if !errors.As(err, &de) || de.DraftID != c.Version.ID() || de.No != 2 {
		t.Fatalf("second clone err = %v", err)
	}

	e.versions.usedBy[v1] = []UsedByRow{{CourseName: "Lập trình cơ bản", VersionNo: 1, Status: domain.VersionPublished}}
	_, err = e.svc.Delete(ctx, e.actor, v1)
	var iu *ErrInUse
	if !errors.As(err, &iu) || !strings.Contains(iu.Error(), "Lập trình cơ bản v1") {
		t.Fatalf("delete used err = %v", err)
	}
	delete(e.versions.usedBy, v1)
	if _, err := e.svc.Delete(ctx, e.actor, v1); !errors.Is(err, ErrDeleteNotDraft) {
		t.Fatalf("delete unused published err = %v", err)
	}

	*e.calls = nil
	a, err := e.svc.Archive(ctx, e.actor, v1)
	if err != nil {
		t.Fatal(err)
	}
	if a.Version.Status() != domain.VersionArchived || strings.Contains(strings.Join(*e.calls, ","), "SaveDraft") {
		t.Fatalf("archive status=%s calls=%v", a.Version.Status(), *e.calls)
	}

	deleted, err := e.svc.Delete(ctx, e.actor, c.Version.ID())
	if err != nil || deleted {
		t.Fatalf("delete non-last draft deleted=%v err=%v", deleted, err)
	}
	solo := e.createDraft(t, "SOLO")
	deleted, err = e.svc.Delete(ctx, e.actor, solo)
	if err != nil || !deleted {
		t.Fatalf("delete last draft deleted=%v err=%v", deleted, err)
	}
	last := e.audit.entries[len(e.audit.entries)-1]
	if last.Action != audit.ActionStageVersionDeleted {
		t.Fatalf("audit = %+v", last)
	}
}

func TestUnitUseCases(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	sql, err := e.svc.AddUnit(ctx, e.actor, vid, " Nhập môn SQL ")
	if err != nil {
		t.Fatal(err)
	}
	if sql.Unit.Title() != "Nhập môn SQL" || sql.Unit.Key() != "nhap-mon-sql" || sql.Unit.Position() != 1 || len(sql.Lessons) != 0 {
		t.Fatalf("added unit = %+v", sql.Unit)
	}
	if _, err := e.svc.AddUnit(ctx, e.actor, vid, "NHẬP MÔN sql"); !errors.Is(err, ErrUnitTitleTaken) {
		t.Fatalf("duplicate title err = %v", err)
	}
	if _, err := e.svc.AddUnit(ctx, e.actor, vid, " "); !errors.Is(err, ErrUnitTitleRequired) {
		t.Fatalf("empty title err = %v", err)
	}
	design := e.addUnit(t, vid, "Thiết kế bảng")
	sqlID := sql.Unit.ID()
	add := func(unit uuid.UUID, title string, required bool) *Lesson {
		t.Helper()
		cmd := markdownCmd(title, "# "+title)
		cmd.UnitID, cmd.Required = &unit, &required
		l, err := e.svc.AddLesson(ctx, e.actor, vid, cmd)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	intro := add(sqlID, "Giới thiệu", true)
	add(design, "Bảng", true)
	add(design, "Chỉ mục", false)
	if got, want := e.layout(vid), "1.Nhập môn SQL[Giới thiệu@1] 2.Thiết kế bảng[Bảng@2 Chỉ mục@3]"; got != want {
		t.Fatalf("layout = %s, want %s", got, want)
	}

	renamed, err := e.svc.RenameUnit(ctx, e.actor, vid, design, "Thiết kế CSDL")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Unit.Title() != "Thiết kế CSDL" || renamed.Unit.Key() != "thiet-ke-bang" || len(renamed.Lessons) != 2 {
		t.Fatalf("renamed = %q key=%q lessons=%d", renamed.Unit.Title(), renamed.Unit.Key(), len(renamed.Lessons))
	}
	if _, err := e.svc.RenameUnit(ctx, e.actor, vid, design, "nhập môn SQL"); !errors.Is(err, ErrUnitTitleTaken) {
		t.Fatalf("rename duplicate err = %v", err)
	}
	if _, err := e.svc.RenameUnit(ctx, e.actor, vid, uuid.New(), "X"); !errors.Is(err, ErrUnitNotFound) {
		t.Fatalf("rename unknown err = %v", err)
	}

	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, intro.ID(), LessonCmd{UnitID: &design}); err != nil {
		t.Fatal(err)
	}
	if got, want := e.layout(vid), "1.Nhập môn SQL[] 2.Thiết kế CSDL[Bảng@1 Chỉ mục@2 Giới thiệu@3]"; got != want {
		t.Fatalf("after move layout = %s, want %s", got, want)
	}
	stray := uuid.New()
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, intro.ID(), LessonCmd{UnitID: &stray}); !errors.Is(err, ErrLessonUnitRequired) {
		t.Fatalf("move to unknown unit err = %v", err)
	}

	if _, err := e.svc.ReorderUnits(ctx, e.actor, vid, []uuid.UUID{design}); !errors.Is(err, ErrReorderMismatch) {
		t.Fatalf("partial unit order err = %v", err)
	}
	d, err := e.svc.ReorderUnits(ctx, e.actor, vid, []uuid.UUID{design, sqlID})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(d.Version), "1.Thiết kế CSDL[Bảng@1 Chỉ mục@2 Giới thiệu@3] 2.Nhập môn SQL[]"; got != want {
		t.Fatalf("after unit reorder = %s, want %s", got, want)
	}

	lessons := e.versions.byID[vid].Lessons()
	layout := []UnitLessons{
		{UnitID: design, LessonIDs: []uuid.UUID{lessons[1].ID()}},
		{UnitID: sqlID, LessonIDs: []uuid.UUID{lessons[2].ID(), lessons[0].ID()}},
	}
	if d, err = e.svc.ReorderLessons(ctx, e.actor, vid, layout); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(d.Version), "1.Thiết kế CSDL[Chỉ mục@1] 2.Nhập môn SQL[Giới thiệu@2 Bảng@3]"; got != want {
		t.Fatalf("after lesson layout = %s, want %s", got, want)
	}

	removed, err := e.svc.RemoveUnit(ctx, e.actor, vid, sqlID)
	if err != nil || removed != 2 {
		t.Fatalf("RemoveUnit = %d, %v; want 2", removed, err)
	}
	if got, want := e.layout(vid), "1.Thiết kế CSDL[Chỉ mục@1]"; got != want {
		t.Fatalf("after remove = %s, want %s", got, want)
	}
	if _, err := e.svc.RemoveUnit(ctx, e.actor, vid, sqlID); !errors.Is(err, ErrUnitNotFound) {
		t.Fatalf("remove twice err = %v", err)
	}
}

func handlerEnv(t *testing.T, e *env, authed bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(e.svc, func(*gin.Context) (uuid.UUID, bool) { return e.actor.ID, authed })
	h.Register(r.Group("/api/v1"))
	return r
}

func do(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type errResp struct {
	Error struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Details json.RawMessage `json:"details"`
	} `json:"error"`
}

func decodeErr(t *testing.T, w *httptest.ResponseRecorder) errResp {
	t.Helper()
	var er errResp
	if err := json.Unmarshal(w.Body.Bytes(), &er); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return er
}

func TestHandlerStatusAndEnvelopes(t *testing.T) {
	e := newEnv(t)
	r := handlerEnv(t, e, true)

	w := do(r, http.MethodPost, "/api/v1/stages", `{"code":"","name":""}`)
	if er := decodeErr(t, w); w.Code != http.StatusUnprocessableEntity || er.Error.Message != "Nhập mã và tên chặng." {
		t.Fatalf("create invalid = %d %s", w.Code, w.Body)
	}
	w = do(r, http.MethodPost, "/api/v1/stages", `{"code":"DB","name":"Cơ sở dữ liệu"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", w.Code, w.Body)
	}
	var detail StageDetailDTO
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil || len(detail.Versions) != 1 {
		t.Fatalf("detail = %s", w.Body)
	}
	vid := detail.Versions[0].ID
	if w = do(r, http.MethodPost, "/api/v1/stages", `{"code":"DB","name":"Lại"}`); w.Code != http.StatusConflict || decodeErr(t, w).Error.Code != "CONFLICT" {
		t.Fatalf("duplicate = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, "/api/v1/stage-versions/"+vid+"/publish", ""); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("publish empty = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, "/api/v1/stage-versions/"+vid+"/lessons", `{"title":"A","type":"markdown","markdownSource":"# A","bogus":1}`); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown field = %d %s", w.Code, w.Body)
	}
	w = do(r, http.MethodPost, "/api/v1/stage-versions/"+vid+"/lessons", `{"title":"A","type":"markdown","required":true,"markdownSource":"# A"}`)
	if er := decodeErr(t, w); w.Code != http.StatusUnprocessableEntity || er.Error.Message != "Chọn bài học cho học liệu." {
		t.Fatalf("add lesson without unit = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, "/api/v1/stage-versions/"+vid+"/units", `{"title":"Bài 1"}`); w.Code != http.StatusCreated {
		t.Fatalf("add unit = %d %s", w.Code, w.Body)
	}
	var unit UnitDTO
	_ = json.Unmarshal(w.Body.Bytes(), &unit)
	w = do(r, http.MethodPost, "/api/v1/stage-versions/"+vid+"/lessons", `{"unitId":"`+unit.ID+`","title":"A","type":"markdown","required":true,"markdownSource":"# A"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("add lesson = %d %s", w.Code, w.Body)
	}
	var lesson LessonDTO
	_ = json.Unmarshal(w.Body.Bytes(), &lesson)
	if lesson.MarkdownHTML != nil || lesson.MarkdownSource == nil || *lesson.MarkdownSource != "# A" || lesson.Position != 1 || lesson.UnitID != unit.ID {
		t.Fatalf("lesson dto = %s", w.Body)
	}
	w = do(r, http.MethodPost, "/api/v1/stage-versions/"+vid+"/lessons", `{"unitId":"`+unit.ID+`","title":"Ảnh","type":"markdown","markdownSource":"![x](https://x/a.png)"}`)
	if er := decodeErr(t, w); w.Code != http.StatusUnprocessableEntity || er.Error.Code != "VALIDATION_FAILED" || er.Error.Message != "Ảnh trong markdown phải tải lên qua hệ thống." {
		t.Fatalf("add external image = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, "/api/v1/stage-versions/"+vid+"/publish", ""); w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body)
	}
	var pub StageVersionDTO
	_ = json.Unmarshal(w.Body.Bytes(), &pub)
	if pub.Status != "published" || pub.PublishedAt == nil || pub.Lessons[0].MarkdownHTML == nil || pub.UsedBy == nil {
		t.Fatalf("published dto = %s", w.Body)
	}
	if w = do(r, http.MethodPatch, "/api/v1/stage-versions/"+vid+"/lessons/"+lesson.ID, `{"title":"B"}`); w.Code != http.StatusConflict || decodeErr(t, w).Error.Code != "VERSION_IMMUTABLE" {
		t.Fatalf("patch published = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPatch, "/api/v1/stage-versions/"+vid+"/units/"+unit.ID, `{"title":"B"}`); w.Code != http.StatusConflict || decodeErr(t, w).Error.Code != "VERSION_IMMUTABLE" {
		t.Fatalf("rename unit published = %d %s", w.Code, w.Body)
	}

	if w = do(r, http.MethodPost, "/api/v1/stage-versions/"+vid+"/clone", ""); w.Code != http.StatusCreated {
		t.Fatalf("clone = %d %s", w.Code, w.Body)
	}
	var clone StageVersionDTO
	_ = json.Unmarshal(w.Body.Bytes(), &clone)
	w = do(r, http.MethodPost, "/api/v1/stage-versions/"+vid+"/clone", "")
	er := decodeErr(t, w)
	var dd DraftExistsDetails
	_ = json.Unmarshal(er.Error.Details, &dd)
	if w.Code != http.StatusConflict || er.Error.Code != "DRAFT_EXISTS" || dd.DraftVersionID != clone.ID || dd.DraftVersionNo != 2 {
		t.Fatalf("draft exists = %d %s", w.Code, w.Body)
	}

	pubID, _ := uuid.Parse(vid)
	e.versions.usedBy[pubID] = []UsedByRow{{CourseID: uuid.New(), CourseName: "Lập trình cơ bản", VersionNo: 1, Status: domain.VersionPublished}}
	w = do(r, http.MethodDelete, "/api/v1/stage-versions/"+vid, "")
	er = decodeErr(t, w)
	var iu InUseDetails
	_ = json.Unmarshal(er.Error.Details, &iu)
	if w.Code != http.StatusConflict || er.Error.Code != "IN_USE" || len(iu.UsedBy) != 1 || iu.UsedBy[0].ClassCodes == nil {
		t.Fatalf("in use = %d %s", w.Code, w.Body)
	}

	w = do(r, http.MethodDelete, "/api/v1/stage-versions/"+clone.ID, "")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"stageDeleted":false}` {
		t.Fatalf("delete draft = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodGet, "/api/v1/stage-versions/not-a-uuid", ""); w.Code != http.StatusNotFound {
		t.Fatalf("bad uuid = %d", w.Code)
	}
	if w = do(r, http.MethodGet, "/api/v1/stages/"+uuid.NewString(), ""); w.Code != http.StatusNotFound {
		t.Fatalf("missing stage = %d", w.Code)
	}
}

func TestHandlerUnitRoutes(t *testing.T) {
	e := newEnv(t)
	r := handlerEnv(t, e, true)
	vid := e.createDraft(t, "DB").String()
	base := "/api/v1/stage-versions/" + vid

	addUnit := func(title string) UnitDTO {
		t.Helper()
		w := do(r, http.MethodPost, base+"/units", `{"title":"`+title+`"}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("add unit %q = %d %s", title, w.Code, w.Body)
		}
		var u UnitDTO
		if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	a, b := addUnit("Nhập môn"), addUnit("Thiết kế bảng")
	if a.UnitKey != "nhap-mon" || a.Position != 1 || b.Position != 2 || a.LessonCount != 0 {
		t.Fatalf("units = %+v %+v", a, b)
	}
	w := do(r, http.MethodPost, base+"/units", `{"title":"nhập MÔN"}`)
	if er := decodeErr(t, w); w.Code != http.StatusConflict || er.Error.Code != "CONFLICT" || er.Error.Message != "Tên bài học đã có trong phiên bản này." {
		t.Fatalf("duplicate unit = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, base+"/units", `{"title":""}`); w.Code != http.StatusUnprocessableEntity || decodeErr(t, w).Error.Message != "Nhập tên bài học." {
		t.Fatalf("empty unit = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, base+"/units", `{"title":"X","bogus":1}`); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown field = %d %s", w.Code, w.Body)
	}

	var lessons []LessonDTO
	for _, unitID := range []string{b.ID, b.ID, a.ID} {
		w := do(r, http.MethodPost, base+"/lessons", `{"unitId":"`+unitID+`","title":"L","type":"markdown","required":true,"markdownSource":"# L"}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("add lesson = %d %s", w.Code, w.Body)
		}
		var l LessonDTO
		_ = json.Unmarshal(w.Body.Bytes(), &l)
		lessons = append(lessons, l)
	}

	w = do(r, http.MethodPatch, base+"/units/"+b.ID, `{"title":"Thiết kế CSDL"}`)
	var renamed UnitDTO
	if err := json.Unmarshal(w.Body.Bytes(), &renamed); w.Code != http.StatusOK || err != nil || renamed.Title != "Thiết kế CSDL" ||
		renamed.UnitKey != "thiet-ke-bang" || renamed.LessonCount != 2 || renamed.RequiredCount != 2 {
		t.Fatalf("rename = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPatch, base+"/units/"+uuid.NewString(), `{"title":"X"}`); w.Code != http.StatusNotFound {
		t.Fatalf("rename unknown = %d %s", w.Code, w.Body)
	}

	w = do(r, http.MethodPut, base+"/units/order", `{"unitIds":["`+b.ID+`","`+a.ID+`"]}`)
	var v StageVersionDTO
	if err := json.Unmarshal(w.Body.Bytes(), &v); w.Code != http.StatusOK || err != nil || len(v.Units) != 2 || v.Units[0].ID != b.ID ||
		v.Lessons[2].ID != lessons[2].ID || v.Lessons[2].Position != 3 {
		t.Fatalf("reorder units = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPut, base+"/units/order", `{"unitIds":["`+b.ID+`"]}`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("partial unit order = %d %s", w.Code, w.Body)
	}

	body := `{"units":[{"unitId":"` + b.ID + `","lessonIds":["` + lessons[1].ID + `"]},{"unitId":"` + a.ID + `","lessonIds":["` +
		lessons[2].ID + `","` + lessons[0].ID + `"]}]}`
	w = do(r, http.MethodPut, base+"/lessons/order", body)
	v = StageVersionDTO{}
	if err := json.Unmarshal(w.Body.Bytes(), &v); w.Code != http.StatusOK || err != nil || v.Lessons[2].ID != lessons[0].ID ||
		v.Lessons[2].UnitID != a.ID || v.Units[0].LessonCount != 1 || v.Units[1].LessonCount != 2 {
		t.Fatalf("reorder lessons = %d %s", w.Code, w.Body)
	}
	old := `{"lessonIds":["` + lessons[0].ID + `","` + lessons[1].ID + `","` + lessons[2].ID + `"]}`
	if w = do(r, http.MethodPut, base+"/lessons/order", old); w.Code != http.StatusBadRequest {
		t.Fatalf("old flat body = %d %s", w.Code, w.Body)
	}

	if w = do(r, http.MethodDelete, base+"/units/"+a.ID, ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete unit = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodDelete, base+"/units/"+a.ID, ""); w.Code != http.StatusNotFound {
		t.Fatalf("delete unit twice = %d %s", w.Code, w.Body)
	}
	if got, want := e.layout(uuid.MustParse(vid)), "1.Thiết kế CSDL[L@1]"; got != want {
		t.Fatalf("layout = %s, want %s", got, want)
	}
}

// TestHandlerUnitRoutesNeedAdmin: guard requireAdmin truyền vào Register chặn teacher ở mọi route soạn bài học.
func TestHandlerUnitRoutesNeedAdmin(t *testing.T) {
	e := newEnv(t)
	vid := e.createDraft(t, "DB")
	uid := e.addUnit(t, vid, "Bài 1")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	forbidden := func(c *gin.Context) { httpx.Fail(c, domain.ErrForbidden) }
	NewHandler(e.svc, func(*gin.Context) (uuid.UUID, bool) { return e.actor.ID, true }).Register(r.Group("/api/v1"), forbidden)
	base := "/api/v1/stage-versions/" + vid.String()
	routes := []struct{ method, path, body string }{
		{http.MethodPost, base + "/units", `{"title":"X"}`},
		{http.MethodPatch, base + "/units/" + uid.String(), `{"title":"X"}`},
		{http.MethodDelete, base + "/units/" + uid.String(), ""},
		{http.MethodPut, base + "/units/order", `{"unitIds":[]}`},
		{http.MethodPut, base + "/lessons/order", `{"units":[]}`},
	}
	for _, rt := range routes {
		if w := do(r, rt.method, rt.path, rt.body); w.Code != http.StatusForbidden || decodeErr(t, w).Error.Code != "FORBIDDEN" {
			t.Errorf("%s %s = %d %s", rt.method, rt.path, w.Code, w.Body)
		}
	}
	if got := e.layout(vid); got != "1.Bài 1[]" {
		t.Fatalf("forbidden calls changed version: %s", got)
	}
}

func TestHandlerMarkdownPreview(t *testing.T) {
	e := newEnv(t)
	r := handlerEnv(t, e, true)
	const path = "/api/v1/stages/markdown-preview"

	w := do(r, http.MethodPost, path, `{"markdownSource":"# A\n\n<script>x</script>"}`)
	var got MarkdownPreviewDTO
	if err := json.Unmarshal(w.Body.Bytes(), &got); w.Code != http.StatusOK || err != nil || !strings.HasPrefix(got.HTML, "<h1>A</h1>") || strings.Contains(got.HTML, "script") {
		t.Fatalf("preview = %d %s", w.Code, w.Body)
	}
	w = do(r, http.MethodPost, path, `{"markdownSource":"![x](https://x/a.png)"}`)
	if er := decodeErr(t, w); w.Code != http.StatusUnprocessableEntity || er.Error.Code != "VALIDATION_FAILED" || er.Error.Message != "Ảnh trong markdown phải tải lên qua hệ thống." {
		t.Fatalf("preview external image = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, path, `{"markdownSource":"# A","bogus":1}`); w.Code != http.StatusBadRequest || decodeErr(t, w).Error.Code != "VALIDATION_FAILED" {
		t.Fatalf("preview unknown field = %d %s", w.Code, w.Body)
	}
	big := `{"markdownSource":"` + strings.Repeat("a", 64<<10) + `"}`
	w = do(r, http.MethodPost, path, big)
	if er := decodeErr(t, w); w.Code != http.StatusBadRequest || !strings.Contains(string(er.Error.Details), "Dữ liệu quá lớn (tối đa 64KB)") {
		t.Fatalf("preview too large = %d %s", w.Code, w.Body)
	}
}

func TestHandlerUnauthenticated(t *testing.T) {
	e := newEnv(t)
	r := handlerEnv(t, e, false)
	w := do(r, http.MethodPost, "/api/v1/stages", `{"code":"DB","name":"A"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestOutdatedRowBlockedReason(t *testing.T) {
	draftID := uuid.New()
	draftNo := domain.VersionNo(2)
	row := ToOutdatedRow(OutdatedCourse{DraftVersionID: &draftID, DraftVersionNo: &draftNo, UsingVersionNo: 1, LatestVersionNo: 2})
	if row.CanApply || row.BlockedReason == nil || *row.BlockedReason != "Khóa học đang có bản nháp v2. Phát hành hoặc xóa bản nháp trước." {
		t.Fatalf("row = %+v", row)
	}
	if ok := ToOutdatedRow(OutdatedCourse{}); !ok.CanApply || ok.BlockedReason != nil {
		t.Fatalf("row without draft = %+v", ok)
	}
}

func intp(n int) *int { return &n }

func quizCmd(title string, q *QuizCmd) LessonCmd {
	return LessonCmd{Title: strp(title), Type: strp("quiz"), Quiz: q}
}

// questionCmd là câu một đáp án hợp lệ: phương án đầu đúng.
func questionCmd(body string) QuestionCmd {
	return QuestionCmd{
		Type: "single", BodySource: body, ExplanationSource: "Vì vậy.", Points: 1,
		Options: []OptionCmd{{Body: "Đúng", Correct: true}, {Body: "Sai"}},
	}
}

func (e *env) addQuizLesson(t *testing.T, vid uuid.UUID, q *QuizCmd) uuid.UUID {
	t.Helper()
	l, err := e.svc.AddLesson(context.Background(), e.actor, vid, e.inFirstUnit(t, vid, quizCmd("Kiểm tra", q)))
	if err != nil {
		t.Fatalf("add quiz: %v", err)
	}
	return l.ID()
}

func (e *env) addHomeworkLesson(t *testing.T, vid uuid.UUID, h *HomeworkCmd) uuid.UUID {
	t.Helper()
	cmd := LessonCmd{Title: strp("Bài tập"), Type: strp("homework"), Homework: h}
	l, err := e.svc.AddLesson(context.Background(), e.actor, vid, e.inFirstUnit(t, vid, cmd))
	if err != nil {
		t.Fatalf("add homework: %v", err)
	}
	return l.ID()
}

func (e *env) storedLesson(t *testing.T, vid, lid uuid.UUID) *Lesson {
	t.Helper()
	l, err := e.versions.byID[vid].Lesson(lid)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestQuizLessonSettings(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")

	lid := e.addQuizLesson(t, vid, nil)
	c := e.storedLesson(t, vid, lid).Content().(QuizContent)
	if c.Mode != QuizPractice || !c.Shuffle || c.MaxAttempts != nil || c.TimeLimit != nil || c.PassPercent != nil {
		t.Fatalf("default quiz = %+v", c)
	}
	if _, err := e.svc.AddQuestion(ctx, e.actor, vid, lid, questionCmd("Câu 1")); err != nil {
		t.Fatal(err)
	}

	// Đổi sang đánh giá giữ câu hỏi và lấy mặc định 2 lượt, 20 phút, 70%.
	l, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{Quiz: &QuizCmd{Mode: strp("assessment")}})
	if err != nil {
		t.Fatal(err)
	}
	c = l.Content().(QuizContent)
	if c.Mode != QuizAssessment || *c.MaxAttempts != 2 || *c.TimeLimit != 20*time.Minute || *c.PassPercent != 70 || len(c.Questions) != 1 {
		t.Fatalf("assessment = %+v", c)
	}
	l, err = e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{Quiz: &QuizCmd{TimeLimitMinutes: intp(15), MaxAttempts: intp(3)}})
	if err != nil {
		t.Fatal(err)
	}
	if c = l.Content().(QuizContent); *c.TimeLimit != 15*time.Minute || *c.MaxAttempts != 3 || *c.PassPercent != 70 {
		t.Fatalf("partial settings = %+v", c)
	}

	for name, q := range map[string]*QuizCmd{
		"mode":         {Mode: strp("exam")},
		"time 0":       {TimeLimitMinutes: intp(0)},
		"time 181":     {TimeLimitMinutes: intp(181)},
		"time huge":    {TimeLimitMinutes: intp(1 << 62)},
		"attempts 11":  {MaxAttempts: intp(11)},
		"attempts 0":   {MaxAttempts: intp(0)},
		"pass 101":     {PassPercent: intp(101)},
		"pass 0":       {PassPercent: intp(0)},
		"new mode bad": {Mode: strp("assessment"), MaxAttempts: intp(-1)},
	} {
		if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{Quiz: q}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	// Đổi quiz sang bài đọc bỏ câu hỏi; đổi lại quiz bắt đầu từ quiz rỗng.
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{Type: strp("markdown"), MarkdownSource: strp("# Đọc")}); err != nil {
		t.Fatal(err)
	}
	l, err = e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{Type: strp("quiz")})
	if err != nil {
		t.Fatal(err)
	}
	if c = l.Content().(QuizContent); len(c.Questions) != 0 || c.Mode != QuizPractice {
		t.Fatalf("quiz again = %+v", c)
	}
}

func TestQuestionUseCases(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	lid := e.addQuizLesson(t, vid, nil)

	l, err := e.svc.AddQuestion(ctx, e.actor, vid, lid, questionCmd("Câu 1"))
	if err != nil {
		t.Fatal(err)
	}
	q1 := l.Content().(QuizContent).Questions[0]
	if q1.Key != "cau-hoi" || q1.Options[0].ID == uuid.Nil || !q1.Options[0].Correct {
		t.Fatalf("q1 = %+v", q1)
	}
	second := questionCmd("Câu 2")
	second.QuestionKey = strp("cau-2")
	if l, err = e.svc.AddQuestion(ctx, e.actor, vid, lid, second); err != nil {
		t.Fatal(err)
	}
	q2 := l.Content().(QuizContent).Questions[1]
	if _, err := e.svc.AddQuestion(ctx, e.actor, vid, lid, second); !errors.Is(err, ErrQuestionKeyTaken) {
		t.Fatalf("duplicate key err = %v", err)
	}

	tooMany := questionCmd("Nhiều")
	for range 5 {
		tooMany.Options = append(tooMany.Options, OptionCmd{Body: "Thêm"})
	}
	invalid := map[string]struct {
		cmd  QuestionCmd
		want error
	}{
		"one option":     {QuestionCmd{Type: "single", BodySource: "X", Points: 1, Options: []OptionCmd{{Body: "A", Correct: true}}}, ErrTooFewOptions},
		"seven options":  {tooMany, ErrTooManyOptions},
		"no correct":     {QuestionCmd{Type: "multi", BodySource: "X", Points: 1, Options: []OptionCmd{{Body: "A"}, {Body: "B"}}}, ErrNoCorrectOption},
		"single 2 right": {QuestionCmd{Type: "single", BodySource: "X", Points: 1, Options: []OptionCmd{{Body: "A", Correct: true}, {Body: "B", Correct: true}}}, ErrSingleManyCorrect},
		"empty body":     {QuestionCmd{Type: "single", BodySource: " ", Points: 1, Options: []OptionCmd{{Body: "A", Correct: true}, {Body: "B"}}}, ErrQuestionBodyRequired},
		"bad type":       {QuestionCmd{Type: "essay", BodySource: "X", Points: 1, Options: []OptionCmd{{Body: "A", Correct: true}, {Body: "B"}}}, ErrInvalidQuestionType},
		"points 0":       {QuestionCmd{Type: "single", BodySource: "X", Options: []OptionCmd{{Body: "A", Correct: true}, {Body: "B"}}}, ErrQuestionPoints},
		"empty option":   {QuestionCmd{Type: "single", BodySource: "X", Points: 1, Options: []OptionCmd{{Body: "A", Correct: true}, {Body: " "}}}, ErrOptionBody},
		"bad key":        {QuestionCmd{QuestionKey: strp("Câu 1"), Type: "single", BodySource: "X", Points: 1, Options: []OptionCmd{{Body: "A", Correct: true}, {Body: "B"}}}, ErrInvalidQuestionKey},
		"external image": {QuestionCmd{Type: "single", BodySource: "![x](https://x/a.png)", Points: 1, Options: []OptionCmd{{Body: "A", Correct: true}, {Body: "B"}}}, ErrExternalImage},
		"base64 in expl": {QuestionCmd{Type: "single", BodySource: "X", ExplanationSource: "![x](data:image/png;base64,AA)", Points: 1, Options: []OptionCmd{{Body: "A", Correct: true}, {Body: "B"}}}, ErrEmbeddedImage},
	}
	for name, tc := range invalid {
		_, err := e.svc.AddQuestion(ctx, e.actor, vid, lid, tc.cmd)
		if !errors.Is(err, tc.want) || !errors.Is(err, domain.ErrInvalid) && !errors.Is(err, domain.ErrConflict) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if n := len(e.storedLesson(t, vid, lid).Content().(QuizContent).Questions); n != 2 {
		t.Fatalf("invalid questions were saved: %d", n)
	}

	// Sửa: giữ id câu và id phương án cũ, phương án mới nhận id mới; đổi mã bị chặn.
	keep := q1.Options[1].ID
	upd := QuestionCmd{
		Type: "multi", BodySource: "Câu 1 sửa", Points: 3,
		Options: []OptionCmd{{ID: &keep, Body: "Sai cũ giờ đúng", Correct: true}, {Body: "Mới", Correct: true}},
	}
	if l, err = e.svc.UpdateQuestion(ctx, e.actor, vid, lid, q1.ID, upd); err != nil {
		t.Fatal(err)
	}
	got := l.Content().(QuizContent).Questions[0]
	if got.ID != q1.ID || got.Key != q1.Key || got.Points != 3 || got.Options[0].ID != keep || got.Options[1].ID == uuid.Nil || got.Options[1].ID == q1.Options[0].ID {
		t.Fatalf("updated = %+v", got)
	}
	foreign := q2.Options[0].ID
	upd.Options[1].ID = &foreign
	if _, err := e.svc.UpdateQuestion(ctx, e.actor, vid, lid, q1.ID, upd); !errors.Is(err, ErrOptionNotInQuestion) {
		t.Fatalf("foreign option err = %v", err)
	}
	upd.Options[1].ID, upd.QuestionKey = nil, strp("doi-ma")
	if _, err := e.svc.UpdateQuestion(ctx, e.actor, vid, lid, q1.ID, upd); !errors.Is(err, ErrQuestionKeyImmutable) {
		t.Fatalf("change key err = %v", err)
	}
	if _, err := e.svc.UpdateQuestion(ctx, e.actor, vid, lid, uuid.New(), questionCmd("X")); !errors.Is(err, ErrQuestionNotFound) {
		t.Fatalf("missing question err = %v", err)
	}

	if l, err = e.svc.ReorderQuestions(ctx, e.actor, vid, lid, []uuid.UUID{q2.ID, q1.ID}); err != nil {
		t.Fatal(err)
	}
	if qs := l.Content().(QuizContent).Questions; qs[0].ID != q2.ID || qs[1].ID != q1.ID {
		t.Fatalf("reordered = %v %v", qs[0].Key, qs[1].Key)
	}
	if _, err := e.svc.ReorderQuestions(ctx, e.actor, vid, lid, []uuid.UUID{q2.ID}); !errors.Is(err, ErrReorderMismatch) {
		t.Fatalf("partial reorder err = %v", err)
	}
	if err := e.svc.RemoveQuestion(ctx, e.actor, vid, lid, q2.ID); err != nil {
		t.Fatal(err)
	}
	if qs := e.storedLesson(t, vid, lid).Content().(QuizContent).Questions; len(qs) != 1 || qs[0].ID != q1.ID {
		t.Fatalf("after remove = %d", len(qs))
	}

	// Lệnh câu hỏi trên học liệu không phải quiz.
	md, err := e.svc.AddLesson(ctx, e.actor, vid, e.inFirstUnit(t, vid, markdownCmd("Đọc", "# A")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.AddQuestion(ctx, e.actor, vid, md.ID(), questionCmd("X")); !errors.Is(err, ErrNotQuiz) {
		t.Fatalf("not quiz err = %v", err)
	}
}

func TestHomeworkUseCases(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	zip := e.addMedia(t, media.KindDocument, "application/zip", "starter.zip", true)
	sql := e.addMedia(t, media.KindDocument, "text/plain; charset=utf-8", "schema.sql", true)
	pending := e.addMedia(t, media.KindDocument, "application/pdf", "de.pdf", false)
	video := e.addMedia(t, media.KindVideo, "video/mp4", "a.mp4", true)

	lid := e.addHomeworkLesson(t, vid, &HomeworkCmd{BriefSource: strp("  # Đề  "), AttachmentMediaIDs: []uuid.UUID{zip, sql}})
	c := e.storedLesson(t, vid, lid).Content().(HomeworkContent)
	if c.BriefSource != "# Đề" || c.PassPercent != 70 || len(c.Attachments) != 2 || c.Attachments[0].FileName != "starter.zip" || c.Attachments[1].MediaID != sql {
		t.Fatalf("homework = %+v", c)
	}

	for name, h := range map[string]*HomeworkCmd{
		"video":     {AttachmentMediaIDs: []uuid.UUID{video}},
		"pending":   {AttachmentMediaIDs: []uuid.UUID{pending}},
		"missing":   {AttachmentMediaIDs: []uuid.UUID{uuid.New()}},
		"duplicate": {AttachmentMediaIDs: []uuid.UUID{zip, zip}},
		"pass 0":    {PassPercent: intp(0)},
		"base64":    {BriefSource: strp("![x](data:image/png;base64,AA)")},
		"external":  {BriefSource: strp("![x](https://x/a.png)")},
	} {
		if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{Homework: h}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// Danh sách rỗng xóa file kèm, đề bài giữ nguyên.
	l, err := e.svc.UpdateLesson(ctx, e.actor, vid, lid, LessonCmd{Homework: &HomeworkCmd{AttachmentMediaIDs: []uuid.UUID{}, PassPercent: intp(80)}})
	if err != nil {
		t.Fatal(err)
	}
	if c = l.Content().(HomeworkContent); len(c.Attachments) != 0 || c.PassPercent != 80 || c.BriefSource != "# Đề" {
		t.Fatalf("cleared = %+v", c)
	}

	if l, err = e.svc.AddCriterion(ctx, e.actor, vid, lid, CriterionCmd{Description: "Đủ bảng", MaxPoints: 40}); err != nil {
		t.Fatal(err)
	}
	if got := l.Content().(HomeworkContent).Criteria; len(got) != 1 || got[0].Description != "Đủ bảng" || got[0].MaxPoints != 40 {
		t.Fatalf("first criterion = %+v", got)
	}
	if l, err = e.svc.AddCriterion(ctx, e.actor, vid, lid, CriterionCmd{Description: "Khóa ngoại", MaxPoints: 60}); err != nil {
		t.Fatal(err)
	}
	cs := l.Content().(HomeworkContent).Criteria
	if len(cs) != 2 || l.Content().(HomeworkContent).MaxScore() != 100 {
		t.Fatalf("criteria = %d", len(cs))
	}
	for name, cmd := range map[string]CriterionCmd{"empty": {Description: " ", MaxPoints: 10}, "0": {Description: "X"}, "101": {Description: "X", MaxPoints: 101}} {
		if _, err := e.svc.AddCriterion(ctx, e.actor, vid, lid, cmd); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if l, err = e.svc.UpdateCriterion(ctx, e.actor, vid, lid, cs[0].ID, CriterionCmd{Description: "Đủ bảng, đúng kiểu", MaxPoints: 50}); err != nil {
		t.Fatal(err)
	}
	if got := l.Content().(HomeworkContent).Criteria[0]; got.ID != cs[0].ID || got.MaxPoints != 50 {
		t.Fatalf("updated = %+v", got)
	}
	if l, err = e.svc.ReorderCriteria(ctx, e.actor, vid, lid, []uuid.UUID{cs[1].ID, cs[0].ID}); err != nil {
		t.Fatal(err)
	}
	if got := l.Content().(HomeworkContent).Criteria; got[0].ID != cs[1].ID {
		t.Fatal("reorder criteria not applied")
	}
	if err := e.svc.RemoveCriterion(ctx, e.actor, vid, lid, cs[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RemoveCriterion(ctx, e.actor, vid, lid, cs[1].ID); !errors.Is(err, ErrCriterionNotFound) {
		t.Fatalf("remove twice err = %v", err)
	}
	quiz := e.addQuizLesson(t, vid, nil)
	if _, err := e.svc.AddCriterion(ctx, e.actor, vid, quiz, CriterionCmd{Description: "X", MaxPoints: 1}); !errors.Is(err, ErrNotHomework) {
		t.Fatalf("not homework err = %v", err)
	}
}

func TestPublishQuizAndHomework(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	img := e.addMedia(t, media.KindImage, "image/png", "a.png", true)
	pending := e.addMedia(t, media.KindImage, "image/png", "b.png", false)

	qid := e.addQuizLesson(t, vid, &QuizCmd{Mode: strp("assessment")})
	if _, err := e.svc.Publish(ctx, e.actor, vid); err == nil || err.Error() != `"Kiểm tra" chưa có câu hỏi.` {
		t.Fatalf("no questions err = %v", err)
	}
	l, err := e.svc.AddQuestion(ctx, e.actor, vid, qid, questionCmd("Câu 1"))
	if err != nil {
		t.Fatal(err)
	}
	q := l.Content().(QuizContent).Questions[0]
	hid := e.addHomeworkLesson(t, vid, nil)
	if _, err := e.svc.Publish(ctx, e.actor, vid); err == nil || err.Error() != `"Bài tập" chưa có đề bài.` {
		t.Fatalf("no brief err = %v", err)
	}
	if _, err := e.svc.UpdateLesson(ctx, e.actor, vid, hid, LessonCmd{Homework: &HomeworkCmd{BriefSource: strp("# Đề ![s](/api/v1/media/" + img.String() + "/content)")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Publish(ctx, e.actor, vid); err == nil || err.Error() != `"Bài tập" chưa có tiêu chí chấm.` {
		t.Fatalf("no criteria err = %v", err)
	}
	if _, err := e.svc.AddCriterion(ctx, e.actor, vid, hid, CriterionCmd{Description: "Đủ", MaxPoints: 10}); err != nil {
		t.Fatal(err)
	}
	// Ảnh trong câu hỏi cũng phải upload xong như ảnh bài đọc.
	if _, err := e.svc.UpdateQuestion(ctx, e.actor, vid, qid, q.ID, questionCmd("Xem ![s](/api/v1/media/"+pending.String()+"/content)")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Publish(ctx, e.actor, vid); err == nil || err.Error() != `Ảnh trong học liệu "Kiểm tra" chưa được tải lên.` {
		t.Fatalf("pending image err = %v", err)
	}
	if _, err := e.svc.UpdateQuestion(ctx, e.actor, vid, qid, q.ID, questionCmd("Xem ![s](/api/v1/media/"+img.String()+"/content)")); err != nil {
		t.Fatal(err)
	}
	if v, _ := e.versions.ByID(ctx, nil, vid); v.Status() != domain.VersionDraft {
		t.Fatal("failed publishes must keep draft")
	}

	d, err := e.svc.Publish(ctx, e.actor, vid)
	if err != nil {
		t.Fatal(err)
	}
	pq, _ := d.Version.Lesson(qid)
	pqc := pq.Content().(QuizContent)
	if html := pqc.Questions[0].BodyHTML; html == nil || !strings.Contains(*html, img.String()) || pqc.Questions[0].ExplanationHTML == nil {
		t.Fatalf("question html = %v", html)
	}
	ph, _ := d.Version.Lesson(hid)
	if html := ph.Content().(HomeworkContent).BriefHTML; html == nil || !strings.Contains(*html, "<h1") {
		t.Fatalf("brief html = %v", html)
	}
}

func TestQuizHomeworkMutationsOnPublishedReturnImmutableFirst(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	qid := e.addQuizLesson(t, vid, &QuizCmd{Mode: strp("assessment")})
	l, err := e.svc.AddQuestion(ctx, e.actor, vid, qid, questionCmd("Câu 1"))
	if err != nil {
		t.Fatal(err)
	}
	q := l.Content().(QuizContent).Questions[0]
	hid := e.addHomeworkLesson(t, vid, &HomeworkCmd{BriefSource: strp("# Đề")})
	if l, err = e.svc.AddCriterion(ctx, e.actor, vid, hid, CriterionCmd{Description: "Đủ", MaxPoints: 10}); err != nil {
		t.Fatal(err)
	}
	cr := l.Content().(HomeworkContent).Criteria[0]
	if _, err := e.svc.Publish(ctx, e.actor, vid); err != nil {
		t.Fatal(err)
	}
	before := snapshot(e.versions.byID[vid])

	// Dữ liệu vào sai vẫn phải nhận VERSION_IMMUTABLE vì kiểm trạng thái trước.
	calls := map[string]func() error{
		"add question": func() error { _, err := e.svc.AddQuestion(ctx, e.actor, vid, qid, QuestionCmd{}); return err },
		"update question": func() error {
			_, err := e.svc.UpdateQuestion(ctx, e.actor, vid, qid, q.ID, questionCmd("Sửa"))
			return err
		},
		"remove question":  func() error { return e.svc.RemoveQuestion(ctx, e.actor, vid, qid, q.ID) },
		"reorder question": func() error { _, err := e.svc.ReorderQuestions(ctx, e.actor, vid, qid, nil); return err },
		"quiz settings": func() error {
			_, err := e.svc.UpdateLesson(ctx, e.actor, vid, qid, LessonCmd{Quiz: &QuizCmd{Mode: strp("practice")}})
			return err
		},
		"homework settings": func() error {
			_, err := e.svc.UpdateLesson(ctx, e.actor, vid, hid, LessonCmd{Homework: &HomeworkCmd{BriefSource: strp("")}})
			return err
		},
		"add criterion": func() error { _, err := e.svc.AddCriterion(ctx, e.actor, vid, hid, CriterionCmd{}); return err },
		"update criterion": func() error {
			_, err := e.svc.UpdateCriterion(ctx, e.actor, vid, hid, cr.ID, CriterionCmd{Description: "X", MaxPoints: 1})
			return err
		},
		"remove criterion":  func() error { return e.svc.RemoveCriterion(ctx, e.actor, vid, hid, cr.ID) },
		"reorder criterion": func() error { _, err := e.svc.ReorderCriteria(ctx, e.actor, vid, hid, nil); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrVersionImmutable) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	after := e.versions.byID[vid]
	for i, l := range before.Lessons() {
		if !reflect.DeepEqual(l.Content(), after.Lessons()[i].Content()) {
			t.Fatalf("published content of %q changed", l.Title())
		}
	}
}

func TestHandlerQuizAndHomeworkRoutes(t *testing.T) {
	e := newEnv(t)
	r := handlerEnv(t, e, true)
	vid := e.createDraft(t, "DB")
	unit := e.addUnit(t, vid, "Bài 1")
	base := "/api/v1/stage-versions/" + vid.String()

	w := do(r, http.MethodPost, base+"/lessons", `{"unitId":"`+unit.String()+`","title":"Kiểm tra","type":"quiz","quiz":{"mode":"assessment","maxAttempts":3,"timeLimitMinutes":15,"shuffle":false,"passPercent":80}}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("add quiz = %d %s", w.Code, w.Body)
	}
	var quiz LessonDTO
	if err := json.Unmarshal(w.Body.Bytes(), &quiz); err != nil {
		t.Fatal(err)
	}
	if quiz.Quiz == nil || quiz.Quiz.Mode != "assessment" || *quiz.Quiz.MaxAttempts != 3 || *quiz.Quiz.TimeLimitMinutes != 15 || quiz.Quiz.Shuffle || *quiz.Quiz.PassPercent != 80 {
		t.Fatalf("quiz dto = %s", w.Body)
	}
	qbase := base + "/lessons/" + quiz.ID + "/questions"
	body := `{"type":"single","bodySource":"Khóa chính?","explanationSource":"Định danh.","points":2,"options":[{"body":"A","correct":true},{"body":"B"}]}`
	w = do(r, http.MethodPost, qbase, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("add question = %d %s", w.Code, w.Body)
	}
	var withQ LessonDTO
	if err := json.Unmarshal(w.Body.Bytes(), &withQ); err != nil {
		t.Fatal(err)
	}
	q := withQ.Quiz.Questions[0]
	if q.QuestionKey != "cau-hoi" || q.Points != 2 || !q.Options[0].IsCorrect || q.Options[1].IsCorrect || q.BodyHTML != nil {
		t.Fatalf("question dto = %s", w.Body)
	}
	w = do(r, http.MethodPost, qbase, `{"type":"single","bodySource":"X","points":1,"options":[{"body":"A"},{"body":"B"}]}`)
	if er := decodeErr(t, w); w.Code != http.StatusUnprocessableEntity || er.Error.Code != "VALIDATION_FAILED" || er.Error.Message != "Đánh dấu ít nhất một đáp án đúng." {
		t.Fatalf("no correct = %d %s", w.Code, w.Body)
	}
	w = do(r, http.MethodPatch, qbase+"/"+q.ID, `{"type":"multi","bodySource":"Sửa","points":1,"options":[{"id":"`+q.Options[0].ID+`","body":"A","correct":true},{"body":"C","correct":true}]}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"bodySource":"Sửa"`) {
		t.Fatalf("update question = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPut, qbase+"/order", `{"questionIds":["`+q.ID+`"]}`); w.Code != http.StatusOK {
		t.Fatalf("reorder questions = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPut, qbase+"/order", `{"questionIds":[]}`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad reorder = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodDelete, qbase+"/"+q.ID, ""); w.Code != http.StatusNoContent {
		t.Fatalf("remove question = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodDelete, qbase+"/"+q.ID, ""); w.Code != http.StatusNotFound {
		t.Fatalf("remove missing question = %d %s", w.Code, w.Body)
	}

	doc := e.addMedia(t, media.KindDocument, "application/zip", "starter.zip", true)
	w = do(r, http.MethodPost, base+"/lessons", `{"unitId":"`+unit.String()+`","title":"Bài tập","type":"homework","homework":{"briefSource":"# Đề","passPercent":60,"attachmentMediaIds":["`+doc.String()+`"]}}`)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"fileName":"starter.zip"`) || !strings.Contains(w.Body.String(), `"passPercent":60`) {
		t.Fatalf("add homework = %d %s", w.Code, w.Body)
	}
	var hw LessonDTO
	if err := json.Unmarshal(w.Body.Bytes(), &hw); err != nil {
		t.Fatal(err)
	}
	cbase := base + "/lessons/" + hw.ID + "/criteria"
	w = do(r, http.MethodPost, cbase, `{"description":"Đủ bảng","maxPoints":40}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("add criterion = %d %s", w.Code, w.Body)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &hw); err != nil {
		t.Fatal(err)
	}
	if hw.Homework.MaxPoints != 40 || len(hw.Homework.Criteria) != 1 {
		t.Fatalf("homework dto = %s", w.Body)
	}
	cid := hw.Homework.Criteria[0].ID
	if w = do(r, http.MethodPatch, cbase+"/"+cid, `{"description":"Đủ","maxPoints":101}`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad criterion = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPatch, cbase+"/"+cid, `{"description":"Đủ","maxPoints":30}`); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"maxPoints":30`) {
		t.Fatalf("update criterion = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPut, cbase+"/order", `{"criterionIds":["`+cid+`"]}`); w.Code != http.StatusOK {
		t.Fatalf("reorder criteria = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodDelete, cbase+"/"+cid, ""); w.Code != http.StatusNoContent {
		t.Fatalf("remove criterion = %d %s", w.Code, w.Body)
	}
}

func TestHandlerQuizRoutesNeedAdmin(t *testing.T) {
	e := newEnv(t)
	vid := e.createDraft(t, "DB")
	qid := e.addQuizLesson(t, vid, nil)
	hid := e.addHomeworkLesson(t, vid, nil)
	before := snapshot(e.versions.byID[vid])
	gin.SetMode(gin.TestMode)
	r := gin.New()
	forbidden := func(c *gin.Context) { httpx.Fail(c, domain.ErrForbidden) }
	NewHandler(e.svc, func(*gin.Context) (uuid.UUID, bool) { return e.actor.ID, true }).Register(r.Group("/api/v1"), forbidden)
	base := "/api/v1/stage-versions/" + vid.String() + "/lessons/"
	q, h, id := base+qid.String(), base+hid.String(), uuid.NewString()
	for _, rt := range []struct{ method, path, body string }{
		{http.MethodPatch, q, `{"quiz":{"mode":"assessment"}}`},
		{http.MethodPost, q + "/questions", `{}`},
		{http.MethodPatch, q + "/questions/" + id, `{}`},
		{http.MethodDelete, q + "/questions/" + id, ""},
		{http.MethodPut, q + "/questions/order", `{"questionIds":[]}`},
		{http.MethodPost, h + "/criteria", `{}`},
		{http.MethodPatch, h + "/criteria/" + id, `{}`},
		{http.MethodDelete, h + "/criteria/" + id, ""},
		{http.MethodPut, h + "/criteria/order", `{"criterionIds":[]}`},
	} {
		if w := do(r, rt.method, rt.path, rt.body); w.Code != http.StatusForbidden || decodeErr(t, w).Error.Code != "FORBIDDEN" {
			t.Errorf("%s %s = %d %s", rt.method, rt.path, w.Code, w.Body)
		}
	}
	if !reflect.DeepEqual(before.Lessons()[0].Content(), e.versions.byID[vid].Lessons()[0].Content()) {
		t.Fatal("forbidden calls changed the quiz")
	}
}
