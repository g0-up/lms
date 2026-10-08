package learning

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
	"lms/api/internal/features/media"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
)

type fakeMembers struct {
	mc  classes.MemberContext
	err error
}

func (f fakeMembers) MemberContext(context.Context, db.Executor, uuid.UUID, uuid.UUID) (classes.MemberContext, error) {
	return f.mc, f.err
}

type fakeStructure struct {
	stages  []StageView
	lessons map[uuid.UUID]*LessonRow
}

func (f fakeStructure) StagesOfCourseVersion(context.Context, db.Executor, uuid.UUID) ([]StageView, error) {
	return f.stages, nil
}

func (f fakeStructure) LessonInCourseVersion(_ context.Context, _ db.Executor, _, lessonID uuid.UUID) (*LessonRow, error) {
	return f.lessons[lessonID], nil
}

// fakeProgress giữ tiến độ trong bộ nhớ và đếm số lần ghi.
type fakeProgress struct {
	rows        map[uuid.UUID]*LessonProgress
	opens, sets int
}

func (f *fakeProgress) Open(_ context.Context, _ db.Executor, p *LessonProgress) (bool, error) {
	f.opens++
	if _, ok := f.rows[p.LessonID()]; ok {
		return false, nil
	}
	f.rows[p.LessonID()] = p
	return true, nil
}

func (f *fakeProgress) Get(_ context.Context, _ db.Executor, _, lessonID uuid.UUID) (*LessonProgress, error) {
	return f.rows[lessonID], nil
}

func (f *fakeProgress) GetForUpdate(ctx context.Context, ex db.Executor, memberID, lessonID uuid.UUID) (*LessonProgress, error) {
	p := f.rows[lessonID]
	if p == nil {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

func (f *fakeProgress) SetCompleted(_ context.Context, _ db.Executor, p *LessonProgress, _ time.Time) error {
	f.sets++
	cp := *p
	f.rows[p.LessonID()] = &cp
	return nil
}

func (f *fakeProgress) ByMember(context.Context, db.Executor, uuid.UUID) (map[uuid.UUID]*LessonProgress, error) {
	return f.rows, nil
}

type fakeMyClasses struct{}

func (fakeMyClasses) ListForStudent(context.Context, db.Executor, uuid.UUID) ([]MyClassRow, error) {
	return nil, nil
}

func (fakeMyClasses) ClassSummary(_ context.Context, _ db.Executor, id uuid.UUID) (ClassSummary, error) {
	return ClassSummary{ID: id, Status: domain.ClassActive}, nil
}

type fakeSigner struct {
	calls []media.Principal
	err   error
}

func (f *fakeSigner) SignedURL(_ context.Context, user media.Principal, _ uuid.UUID) (media.SignedURL, error) {
	f.calls = append(f.calls, user)
	if f.err != nil {
		return media.SignedURL{}, f.err
	}
	return media.SignedURL{URL: "https://storage.test/get", ExpiresAt: testNow.Add(2 * time.Hour)}, nil
}

type runTx struct{}

func (runTx) Transact(_ context.Context, fn func(db.Executor) error) error { return fn(nil) }

// noTx báo lỗi nếu use case mở transaction: yêu cầu sai phải bị chặn trước khi ghi.
type noTx struct{ t *testing.T }

func (n noTx) Transact(context.Context, func(db.Executor) error) error {
	n.t.Fatal("không được mở transaction")
	return nil
}

type svcEnv struct {
	svc                     *Service
	progress                *fakeProgress
	signer                  *fakeSigner
	student, class          uuid.UUID
	video, markdown, noHTML uuid.UUID
	mediaID                 uuid.UUID
}

func newSvcEnv(t *testing.T, cs domain.ClassStatus, tx db.Tx) *svcEnv {
	t.Helper()
	e := &svcEnv{
		progress: &fakeProgress{rows: map[uuid.UUID]*LessonProgress{}}, signer: &fakeSigner{},
		student: uuid.New(), class: uuid.New(), video: uuid.New(), markdown: uuid.New(), noHTML: uuid.New(), mediaID: uuid.New(),
	}
	stageID, html := uuid.New(), "<p>Bài đọc</p>"
	structure := fakeStructure{
		stages: []StageView{{StageID: stageID, Name: "Database", Position: 1, Lessons: []LessonView{
			{ID: e.video, Title: "Giới thiệu SQL", Type: stages.LessonVideo, Position: 1, Required: true},
			{ID: e.markdown, Title: "Thiết kế bảng và khóa", Type: stages.LessonMarkdown, Position: 2, Required: true},
			{ID: e.noHTML, Title: "Chưa render", Type: stages.LessonMarkdown, Position: 3, Required: false},
		}}},
		lessons: map[uuid.UUID]*LessonRow{
			e.video:    {ID: e.video, StageID: stageID, Content: stages.VideoContent{MediaID: e.mediaID}},
			e.markdown: {ID: e.markdown, StageID: stageID, Content: stages.MarkdownContent{HTML: &html}},
			e.noHTML:   {ID: e.noHTML, StageID: stageID, Content: stages.MarkdownContent{}},
		},
	}
	e.svc = NewService(Deps{
		Tx: tx, Progress: e.progress, Structure: structure, MyClasses: fakeMyClasses{},
		Members: fakeMembers{mc: classes.MemberContext{MemberID: uuid.New(), MemberStatus: domain.MemberActive, ClassStatus: cs}},
		Media:   e.signer, Clock: &clock.Fake{T: testNow},
	})
	return e
}

func TestUnknownClassIsNotFound(t *testing.T) {
	svc := NewService(Deps{Tx: noTx{t}, Members: fakeMembers{err: classes.ErrClassNotFound}})
	if _, err := svc.Roadmap(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Roadmap = %v, muốn ErrNotFound", err)
	}
	if _, err := svc.SetCompletion(context.Background(), uuid.New(), uuid.New(), uuid.New(), true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetCompletion = %v, muốn ErrNotFound", err)
	}
}

func TestOpenLessonRecordsOnlyInActiveClass(t *testing.T) {
	ctx := context.Background()
	e := newSvcEnv(t, domain.ClassActive, noTx{t})
	page, err := e.svc.OpenLesson(ctx, e.student, e.class, e.markdown)
	if err != nil {
		t.Fatal(err)
	}
	if e.progress.opens != 1 || page.Lesson.State != StateOpened || page.Lesson.FirstOpenedAt == nil {
		t.Fatalf("opens=%d state=%s", e.progress.opens, page.Lesson.State)
	}
	if page.Content.Type != stages.LessonMarkdown || page.Content.HTML != "<p>Bài đọc</p>" || page.ReadOnly {
		t.Fatalf("content = %+v readOnly=%v", page.Content, page.ReadOnly)
	}
	if page.Prev == nil || page.Prev.LessonID != e.video || page.Next == nil || page.Next.LessonID != e.noHTML {
		t.Fatalf("prev=%+v next=%+v", page.Prev, page.Next)
	}

	ended := newSvcEnv(t, domain.ClassEnded, noTx{t})
	page, err = ended.svc.OpenLesson(ctx, ended.student, ended.class, ended.markdown)
	if err != nil {
		t.Fatal(err)
	}
	if ended.progress.opens != 0 || page.Lesson.State != StateNotOpened || !page.ReadOnly {
		t.Fatalf("lớp ended: opens=%d state=%s readOnly=%v", ended.progress.opens, page.Lesson.State, page.ReadOnly)
	}

	draft := newSvcEnv(t, domain.ClassDraft, noTx{t})
	if _, err := draft.svc.OpenLesson(ctx, draft.student, draft.class, draft.markdown); !errors.Is(err, ErrClassNotStarted) {
		t.Fatalf("lớp draft: %v, muốn ErrClassNotStarted", err)
	}
	if draft.progress.opens != 0 {
		t.Fatal("lớp draft không được ghi lần mở")
	}
}

func TestOpenLessonContent(t *testing.T) {
	ctx := context.Background()
	e := newSvcEnv(t, domain.ClassActive, noTx{t})
	page, err := e.svc.OpenLesson(ctx, e.student, e.class, e.video)
	if err != nil {
		t.Fatal(err)
	}
	if page.Content.Type != stages.LessonVideo || page.Content.MediaID != e.mediaID || page.Content.URL == "" || page.Content.ExpiresAt.IsZero() {
		t.Fatalf("video content = %+v", page.Content)
	}
	if len(e.signer.calls) != 1 || e.signer.calls[0] != (media.Principal{ID: e.student, Role: domain.RoleStudent}) {
		t.Fatalf("ký URL với %+v", e.signer.calls)
	}

	if _, err := e.svc.OpenLesson(ctx, e.student, e.class, uuid.New()); !errors.Is(err, ErrLessonNotInCourse) {
		t.Fatalf("học liệu ngoài khóa = %v", err)
	}
	if _, err := e.svc.OpenLesson(ctx, e.student, e.class, e.noHTML); err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("markdown thiếu HTML phải là lỗi hệ thống, được %v", err)
	}

	// Ký URL thất bại (file chưa sẵn sàng) thì không ghi lần mở.
	failing := newSvcEnv(t, domain.ClassActive, noTx{t})
	failing.signer.err = media.ErrNotReady
	if _, err := failing.svc.OpenLesson(ctx, failing.student, failing.class, failing.video); !errors.Is(err, media.ErrNotReady) {
		t.Fatalf("ký URL lỗi = %v", err)
	}
	if failing.progress.opens != 0 {
		t.Fatal("không được ghi lần mở khi chưa dựng được nội dung")
	}
}

func TestSetCompletion(t *testing.T) {
	ctx := context.Background()
	e := newSvcEnv(t, domain.ClassActive, runTx{})
	if _, err := e.svc.SetCompletion(ctx, e.student, e.class, e.markdown, true); !errors.Is(err, ErrNotOpened) {
		t.Fatalf("tích trước khi mở = %v", err)
	}
	if _, err := e.svc.SetCompletion(ctx, e.student, e.class, uuid.New(), true); !errors.Is(err, ErrLessonNotInCourse) {
		t.Fatalf("học liệu ngoài khóa = %v", err)
	}
	if _, err := e.svc.OpenLesson(ctx, e.student, e.class, e.markdown); err != nil {
		t.Fatal(err)
	}

	got, err := e.svc.SetCompletion(ctx, e.student, e.class, e.markdown, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.CompletedAt == nil || got.Percent != 50 || got.RequiredDone != 1 || got.RequiredTotal != 2 {
		t.Fatalf("tích = %+v", got)
	}
	again, err := e.svc.SetCompletion(ctx, e.student, e.class, e.markdown, true)
	if err != nil {
		t.Fatal(err)
	}
	if e.progress.sets != 1 || !again.CompletedAt.Equal(*got.CompletedAt) {
		t.Fatalf("tích lại phải giữ completedAt và không ghi: sets=%d", e.progress.sets)
	}
	off, err := e.svc.SetCompletion(ctx, e.student, e.class, e.markdown, false)
	if err != nil || off.CompletedAt != nil || off.Percent != 0 || e.progress.sets != 2 {
		t.Fatalf("bỏ tích = %+v, %v, sets=%d", off, err, e.progress.sets)
	}
	if _, err := e.svc.SetCompletion(ctx, e.student, e.class, e.markdown, false); err != nil || e.progress.sets != 2 {
		t.Fatalf("bỏ tích lại: %v sets=%d", err, e.progress.sets)
	}
}

func TestSetCompletionRejectedOutsideActiveClass(t *testing.T) {
	for _, cs := range []domain.ClassStatus{domain.ClassDraft, domain.ClassEnded} {
		e := newSvcEnv(t, cs, noTx{t})
		if _, err := e.svc.SetCompletion(context.Background(), e.student, e.class, e.markdown, true); !errors.Is(err, ErrClassNotActive) {
			t.Fatalf("lớp %s: %v, muốn ErrClassNotActive", cs, err)
		}
	}
}
