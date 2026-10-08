package stages

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	limits := media.Limits{MaxVideoBytes: 1 << 30, MaxImageBytes: 1 << 20}
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

func markdownCmd(title, src string) LessonCmd {
	return LessonCmd{Title: strp(title), Type: strp("markdown"), MarkdownSource: strp(src)}
}

// addLegacyMarkdown thêm bài markdown rồi ghi đè nguồn thẳng trong repo giả, giả lập dữ liệu lưu trước khi có luật
// ảnh ngoài.
func (e *env) addLegacyMarkdown(t *testing.T, vid uuid.UUID, title, src string) uuid.UUID {
	t.Helper()
	l, err := e.svc.AddLesson(context.Background(), e.actor, vid, markdownCmd(title, "# Tạm"))
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
	if _, err := e.svc.AddLesson(context.Background(), e.actor, vid, markdownCmd("Bài 1", "# Một")); err != nil {
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
			if _, err := e.svc.AddLesson(ctx, e.actor, vid, tt.cmd); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}

	l, err := e.svc.AddLesson(ctx, e.actor, vid, videoCmd(readyVideo))
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
	if _, err := e.svc.AddLesson(ctx, e.actor, vid, markdownCmd("Ảnh", withMedia)); err != nil {
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
	l, err := e.svc.AddLesson(ctx, e.actor, vid, markdownCmd("Bảng", "# Bảng"))
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
	if _, err := e.svc.Publish(ctx, e.actor, vid); !errors.Is(err, ErrPublishNotDraft) {
		t.Fatalf("publish twice err = %v", err)
	}
}

func TestPublishOrderAndAudit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	vid := e.createDraft(t, "DB")
	if _, err := e.svc.Publish(ctx, e.actor, vid); !errors.Is(err, ErrNoLessons) {
		t.Fatalf("empty publish err = %v", err)
	}
	img := e.addMedia(t, media.KindImage, "image/png", "a.png", true)
	// Ảnh ngoài chỉ còn trong dữ liệu cũ (lưu không qua được luật ảnh); phát hành vẫn phải lọc nó.
	src := "# Hi <script>alert(1)</script> ![x](https://x/a.png) ![s](/api/v1/media/" + img.String() + "/content)"
	e.addLegacyMarkdown(t, vid, "Giới thiệu", src)
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
	if _, err := e.svc.AddLesson(ctx, e.actor, vid, markdownCmd("Ảnh", "![s](/api/v1/media/"+pending.String()+"/content)")); err != nil {
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
	if w.Code != http.StatusCreated {
		t.Fatalf("add lesson = %d %s", w.Code, w.Body)
	}
	var lesson LessonDTO
	_ = json.Unmarshal(w.Body.Bytes(), &lesson)
	if lesson.MarkdownHTML != nil || lesson.MarkdownSource == nil || *lesson.MarkdownSource != "# A" || lesson.Position != 1 {
		t.Fatalf("lesson dto = %s", w.Body)
	}
	w = do(r, http.MethodPost, "/api/v1/stage-versions/"+vid+"/lessons", `{"title":"Ảnh","type":"markdown","markdownSource":"![x](https://x/a.png)"}`)
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
