package courses

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
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
)

// fakeTx chạy fn ngay, không rollback; test chỉ kiểm lỗi và thứ tự gọi repo.
type fakeTx struct{}

func (fakeTx) Transact(_ context.Context, fn func(tx db.Executor) error) error { return fn(nil) }

type fakeCourses struct {
	rows  map[uuid.UUID]*CourseRow
	calls *[]string
}

func (f *fakeCourses) Create(_ context.Context, _ db.Executor, c *Course) error {
	for _, r := range f.rows {
		if r.Code == c.Code().String() {
			return ErrCodeTaken
		}
	}
	f.rows[c.ID()] = &CourseRow{ID: c.ID(), Code: c.Code().String(), Name: c.Name(), Description: c.Description(), CreatedAt: c.CreatedAt()}
	*f.calls = append(*f.calls, "courses.Create")
	return nil
}

func (f *fakeCourses) ByID(_ context.Context, _ db.Executor, id uuid.UUID) (*CourseRow, error) {
	r, ok := f.rows[id]
	if !ok {
		return nil, ErrCourseNotFound
	}
	return r, nil
}

func (f *fakeCourses) Delete(_ context.Context, _ db.Executor, id uuid.UUID) error {
	delete(f.rows, id)
	return nil
}

func (f *fakeCourses) List(context.Context, db.Executor, ListQuery) ([]CourseListRow, error) {
	return nil, nil
}

func (f *fakeCourses) LockForUpdate(_ context.Context, _ db.Executor, id uuid.UUID) error {
	if _, ok := f.rows[id]; !ok {
		return ErrCourseNotFound
	}
	*f.calls = append(*f.calls, "courses.LockForUpdate")
	return nil
}

type fakeVersions struct {
	byID  map[uuid.UUID]*CourseVersion
	calls *[]string
}

// Create giữ bất biến của repository thật: chỉ chèn header draft.
func (f *fakeVersions) Create(_ context.Context, _ db.Executor, v *CourseVersion) error {
	if v.Status() != domain.VersionDraft {
		return errors.New("Create nhận header không phải draft")
	}
	if d, _ := f.DraftOf(context.Background(), nil, v.CourseID()); d != nil {
		return &ErrDraftExists{DraftID: d.ID(), No: d.VersionNo()}
	}
	f.byID[v.ID()] = v
	*f.calls = append(*f.calls, "versions.Create")
	return nil
}

func (f *fakeVersions) SaveDraft(context.Context, db.Executor, *CourseVersion) error {
	*f.calls = append(*f.calls, "versions.SaveDraft")
	return nil
}

func (f *fakeVersions) TransitionStatus(_ context.Context, _ db.Executor, _ uuid.UUID, from, to domain.VersionStatus, _ *time.Time) error {
	*f.calls = append(*f.calls, "versions.TransitionStatus:"+from.String()+"->"+to.String())
	return nil
}

func (f *fakeVersions) Delete(_ context.Context, _ db.Executor, id uuid.UUID) error {
	delete(f.byID, id)
	return nil
}

func (f *fakeVersions) ByID(_ context.Context, _ db.Executor, id uuid.UUID) (*CourseVersion, error) {
	v, ok := f.byID[id]
	if !ok {
		return nil, ErrVersionNotFound
	}
	return v, nil
}

func (f *fakeVersions) ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*CourseVersion, error) {
	return f.ByID(ctx, ex, id)
}

func (f *fakeVersions) CountByCourse(_ context.Context, _ db.Executor, courseID uuid.UUID) (int, error) {
	n := 0
	for _, v := range f.byID {
		if v.CourseID() == courseID {
			n++
		}
	}
	return n, nil
}

func (f *fakeVersions) ListByCourse(context.Context, db.Executor, uuid.UUID) ([]VersionListRow, error) {
	return nil, nil
}

func (f *fakeVersions) LatestPublished(_ context.Context, _ db.Executor, courseID uuid.UUID) (*CourseVersion, error) {
	var latest *CourseVersion
	for _, v := range f.byID {
		if v.CourseID() == courseID && v.Status() == domain.VersionPublished && (latest == nil || v.VersionNo() > latest.VersionNo()) {
			latest = v
		}
	}
	return latest, nil
}

func (f *fakeVersions) DraftOf(_ context.Context, _ db.Executor, courseID uuid.UUID) (*CourseVersion, error) {
	for _, v := range f.byID {
		if v.CourseID() == courseID && v.Status() == domain.VersionDraft {
			return v, nil
		}
	}
	return nil, nil
}

func (f *fakeVersions) NextVersionNo(_ context.Context, _ db.Executor, courseID uuid.UUID) (domain.VersionNo, error) {
	var maxNo domain.VersionNo
	for _, v := range f.byID {
		if v.CourseID() == courseID && v.VersionNo() > maxNo {
			maxNo = v.VersionNo()
		}
	}
	return maxNo + 1, nil
}

func (f *fakeVersions) UsedByClasses(context.Context, db.Executor, uuid.UUID) ([]UsedByClassRow, error) {
	return nil, nil
}

func (f *fakeVersions) ClassesUsing(context.Context, db.Executor, uuid.UUID) ([]ClassUsingRow, error) {
	return nil, nil
}

func (f *fakeVersions) StageRows(context.Context, db.Executor, uuid.UUID) ([]VersionStageRow, error) {
	return nil, nil
}

type fakeStageVersions map[uuid.UUID]StageVersionRef

func (f fakeStageVersions) Refs(_ context.Context, _ db.Executor, ids []uuid.UUID) (map[uuid.UUID]StageVersionRef, error) {
	out := make(map[uuid.UUID]StageVersionRef, len(ids))
	for _, id := range ids {
		if r, ok := f[id]; ok {
			out[id] = r
		}
	}
	return out, nil
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
	courses  *fakeCourses
	versions *fakeVersions
	refs     fakeStageVersions
	audit    *fakeAudit
	calls    *[]string
	actor    Actor
}

func newEnv(t *testing.T) *env {
	t.Helper()
	calls := &[]string{}
	e := &env{
		courses:  &fakeCourses{rows: map[uuid.UUID]*CourseRow{}, calls: calls},
		versions: &fakeVersions{byID: map[uuid.UUID]*CourseVersion{}, calls: calls},
		refs:     fakeStageVersions{},
		audit:    &fakeAudit{},
		calls:    calls,
		actor:    Actor{ID: uuid.New(), RequestID: "req-1"},
	}
	e.svc = NewService(Deps{
		Tx: fakeTx{}, Courses: e.courses, Versions: e.versions, StageVersions: e.refs,
		Clock: &clock.Fake{T: testNow}, Audit: e.audit, IDs: seqIDs{},
	})
	return e
}

// addRef đăng ký một phiên bản chặng cho StageVersionReader giả.
func (e *env) addRef(stageID uuid.UUID, code string, no domain.VersionNo, status domain.VersionStatus) StageVersionRef {
	r := StageVersionRef{ID: uuid.New(), StageID: stageID, StageCode: code, VersionNo: no, Status: status}
	e.refs[r.ID] = r
	return r
}

// addCourse tạo khóa học có v1 đã phát hành gồm refs.
func (e *env) addCourse(t *testing.T, code string, refs ...StageVersionRef) (uuid.UUID, *CourseVersion) {
	t.Helper()
	id := uuid.New()
	e.courses.rows[id] = &CourseRow{ID: id, Code: code, Name: "Khóa " + code}
	v := NewDraftCourseVersion(uuid.New(), id, domain.FirstVersionNo, "Khóa "+code, "", nil, e.actor.ID)
	if err := v.SetStages(refs); err != nil {
		t.Fatal(err)
	}
	if err := v.Publish(testNow, lookupOf(refs...)); err != nil {
		t.Fatal(err)
	}
	e.versions.byID[v.ID()] = v
	return id, v
}

func TestApplyPreconditions(t *testing.T) {
	db, web := uuid.New(), uuid.New()
	db1, db2, web1 := ref(db, 1, domain.VersionPublished), ref(db, 2, domain.VersionPublished), ref(web, 1, domain.VersionPublished)
	latest := publishedWith(t, db1, web1)
	draft := newDraft()
	tests := []struct {
		name   string
		latest *CourseVersion
		draft  *CourseVersion
		src    StageVersionRef
		want   error
	}{
		{"đang có bản nháp", latest, draft, db2, nil},
		{"chưa có bản phát hành", nil, nil, db2, ErrNoPublishedVersion},
		{"không chứa chặng", publishedWith(t, web1), nil, db2, ErrCourseLacksStage},
		{"đã dùng phiên bản nguồn", latest, nil, db1, ErrAlreadyUsingVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := applyPreconditions(tt.latest, tt.draft, tt.src)
			if tt.draft != nil {
				var de *ErrDraftExists
				if !errors.As(err, &de) || de.DraftID != tt.draft.ID() || de.No != tt.draft.VersionNo() {
					t.Fatalf("applyPreconditions = %v, muốn ErrDraftExists", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("applyPreconditions = %v, muốn %v", err, tt.want)
			}
		})
	}
	cur, err := applyPreconditions(latest, nil, db2)
	if err != nil || cur.StageVersionID != db1.ID || cur.Position != 1 {
		t.Fatalf("applyPreconditions hợp lệ = %+v, %v", cur, err)
	}
}

func TestAppErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"bản nháp", &ErrDraftExists{DraftID: uuid.New(), No: 2}, http.StatusConflict, apperr.CodeDraftExists},
		{"đang dùng", &ErrInUse{}, http.StatusConflict, apperr.CodeInUse},
		{"dữ liệu không hợp lệ", ErrCourseLacksStage, http.StatusUnprocessableEntity, apperr.CodeValidationFailed},
		{"xung đột", ErrAlreadyUsingVersion, http.StatusConflict, apperr.CodeConflict},
		{"không tìm thấy", ErrCourseNotFound, http.StatusNotFound, apperr.CodeNotFound},
		{"lỗi hạ tầng", errors.New("mất kết nối"), http.StatusInternalServerError, apperr.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appError(context.Background(), tt.err)
			if got.Status != tt.status || got.Code != tt.code {
				t.Fatalf("appError = %d %s, muốn %d %s", got.Status, got.Code, tt.status, tt.code)
			}
		})
	}
	if got := appError(context.Background(), ErrCourseLacksStage); got.Message != "Khóa học không chứa chặng này." {
		t.Fatalf("message = %q", got.Message)
	}
}

func TestApplyStageVersionPerCourseResults(t *testing.T) {
	e := newEnv(t)
	dbStage, webStage := uuid.New(), uuid.New()
	db1, web1 := e.addRef(dbStage, "DB", 1, domain.VersionPublished), e.addRef(webStage, "WEB", 1, domain.VersionPublished)
	db2 := e.addRef(dbStage, "DB", 2, domain.VersionPublished)

	basic, basicV1 := e.addCourse(t, "BASIC", db1, web1)
	hasDraft, hasDraftV1 := e.addCourse(t, "HASDRAFT", db1)
	draft, err := hasDraftV1.CloneAsDraft(uuid.New(), 2, e.actor.ID)
	if err != nil {
		t.Fatal(err)
	}
	e.versions.byID[draft.ID()] = draft
	webOnly, _ := e.addCourse(t, "WEBONLY", web1)
	missing := uuid.New()
	*e.calls = nil

	results, err := e.svc.ApplyStageVersion(context.Background(), e.actor, db2.ID, []uuid.UUID{basic, hasDraft, webOnly, missing, basic})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("results = %+v (trùng id phải bị gộp)", results)
	}
	ok := results[0]
	if ok.Error != nil || ok.NewVersionNo == nil || *ok.NewVersionNo != 2 || ok.CourseCode != "BASIC" {
		t.Fatalf("BASIC = %+v", ok)
	}
	wantErr := []struct {
		code   string
		status int
	}{
		{apperr.CodeDraftExists, http.StatusConflict},
		{apperr.CodeValidationFailed, http.StatusUnprocessableEntity},
		{apperr.CodeNotFound, http.StatusNotFound},
	}
	for i, w := range wantErr {
		r := results[i+1]
		if r.Error == nil || r.Error.Code != w.code || r.Error.Status != w.status || r.NewVersionNo != nil {
			t.Fatalf("results[%d] = %+v, muốn %s", i+1, r, w.code)
		}
	}
	if results[1].CourseCode != "HASDRAFT" || results[3].CourseCode != "" {
		t.Fatalf("courseCode = %q %q", results[1].CourseCode, results[3].CourseCode)
	}

	// Thứ tự ghi: chèn header draft (kèm chặng) rồi mới chuyển draft → published.
	var writes []string
	for _, c := range *e.calls {
		if strings.HasPrefix(c, "versions.") {
			writes = append(writes, c)
		}
	}
	if strings.Join(writes, ",") != "versions.Create,versions.TransitionStatus:draft->published" {
		t.Fatalf("writes = %v", writes)
	}

	v2, err := e.versions.LatestPublished(context.Background(), nil, basic)
	if err != nil || v2 == nil || v2.VersionNo() != 2 || *v2.ClonedFromID() != basicV1.ID() {
		t.Fatalf("BASIC v2 = %+v, %v", v2, err)
	}
	if s, _ := v2.StageVersionFor(dbStage); s.StageVersionID != db2.ID || s.Position != 1 {
		t.Fatalf("DB trên v2 = %+v", s)
	}
	if s, _ := v2.StageVersionFor(webStage); s.StageVersionID != web1.ID || s.Position != 2 {
		t.Fatalf("WEB trên v2 = %+v", s)
	}
	if s, _ := basicV1.StageVersionFor(dbStage); s.StageVersionID != db1.ID {
		t.Fatal("v1 bị sửa")
	}

	var actions []string
	for _, en := range e.audit.entries {
		actions = append(actions, en.Action)
	}
	want := []string{audit.ActionCourseVersionCloned, audit.ActionCourseVersionPublished, audit.ActionCourseStageVersionApplied}
	if strings.Join(actions, ",") != strings.Join(want, ",") {
		t.Fatalf("audit = %v", actions)
	}
	for _, en := range e.audit.entries {
		if !en.At.Equal(testNow) {
			t.Fatalf("audit %s at = %v, muốn cùng mốc phát hành %v", en.Action, en.At, testNow)
		}
	}
	applied, _ := e.audit.entries[2].After.(map[string]any)
	if applied["fromStageVersionId"] != db1.ID || applied["toStageVersionId"] != db2.ID || applied["toVersionNo"] != domain.VersionNo(2) {
		t.Fatalf("audit applied = %+v", applied)
	}
}

func TestApplyStageVersionGlobalErrors(t *testing.T) {
	e := newEnv(t)
	stage := uuid.New()
	db1 := e.addRef(stage, "DB", 1, domain.VersionPublished)
	draftRef := e.addRef(stage, "DB", 2, domain.VersionDraft)
	course, _ := e.addCourse(t, "BASIC", db1)
	tests := []struct {
		name    string
		src     uuid.UUID
		courses []uuid.UUID
		want    error
	}{
		{"không chọn khóa học", db1.ID, nil, ErrNoCoursesSelected},
		{"nguồn không tồn tại", uuid.New(), []uuid.UUID{course}, ErrStageVersionNotFound},
		{"nguồn chưa phát hành", draftRef.ID, []uuid.UUID{course}, ErrSourceNotPublished},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			*e.calls = nil
			results, err := e.svc.ApplyStageVersion(context.Background(), e.actor, tt.src, tt.courses)
			if !errors.Is(err, tt.want) || results != nil {
				t.Fatalf("ApplyStageVersion = %v, %v; muốn %v", results, err, tt.want)
			}
			if len(*e.calls) != 0 || len(e.audit.entries) != 0 {
				t.Fatalf("lỗi toàn cục không được ghi: %v", *e.calls)
			}
		})
	}
}

func TestSetStagesAndPublishThroughService(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	stage := uuid.New()
	db1 := e.addRef(stage, "DB", 1, domain.VersionPublished)
	db2 := e.addRef(stage, "DB", 2, domain.VersionDraft)

	d, err := e.svc.CreateCourse(ctx, e.actor, CreateCourseCmd{Code: "DEMO", Name: "Khóa demo"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Course.Code != "DEMO" || len(e.audit.entries) != 1 || e.audit.entries[0].Action != audit.ActionCourseCreated {
		t.Fatalf("CreateCourse = %+v, audit %+v", d.Course, e.audit.entries)
	}
	draft, _ := e.versions.DraftOf(ctx, nil, d.Course.ID)
	if draft == nil || draft.VersionNo() != 1 {
		t.Fatalf("bản nháp v1 = %+v", draft)
	}
	if _, err := e.svc.CreateCourse(ctx, e.actor, CreateCourseCmd{Code: "DEMO", Name: "Lại"}); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("trùng mã = %v", err)
	}

	if _, err := e.svc.Publish(ctx, e.actor, draft.ID()); !errors.Is(err, ErrNoStages) {
		t.Fatalf("Publish rỗng = %v", err)
	}
	if _, err := e.svc.SetStages(ctx, e.actor, draft.ID(), []uuid.UUID{db2.ID}); !errors.Is(err, ErrStageVersionNotPublished) {
		t.Fatalf("SetStages bản nháp chặng = %v", err)
	}
	if _, err := e.svc.SetStages(ctx, e.actor, draft.ID(), []uuid.UUID{uuid.New()}); !errors.Is(err, ErrStageVersionNotFound) {
		t.Fatalf("SetStages id lạ = %v", err)
	}
	if _, err := e.svc.SetStages(ctx, e.actor, draft.ID(), []uuid.UUID{db1.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Publish(ctx, e.actor, draft.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetStages(ctx, e.actor, draft.ID(), []uuid.UUID{db1.ID}); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("SetStages trên bản published = %v", err)
	}
	if _, err := e.svc.Delete(ctx, e.actor, draft.ID()); !errors.Is(err, ErrDeleteNotDraft) {
		t.Fatalf("Delete bản published = %v", err)
	}
	if _, err := e.svc.PublishedVersion(ctx, nil, draft.ID()); err != nil {
		t.Fatalf("PublishedVersion = %v", err)
	}
}

func TestDeleteLastDraftDeletesCourse(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d, err := e.svc.CreateCourse(ctx, e.actor, CreateCourseCmd{Code: "DEMO", Name: "Khóa demo"})
	if err != nil {
		t.Fatal(err)
	}
	draft, _ := e.versions.DraftOf(ctx, nil, d.Course.ID)
	if _, err := e.svc.PublishedVersion(ctx, nil, draft.ID()); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("PublishedVersion bản nháp = %v", err)
	}
	deleted, err := e.svc.Delete(ctx, e.actor, draft.ID())
	if err != nil || !deleted {
		t.Fatalf("Delete = %v, %v", deleted, err)
	}
	if _, ok := e.courses.rows[d.Course.ID]; ok {
		t.Fatal("khóa học còn sau khi xóa phiên bản cuối")
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
	stage := uuid.New()
	db1 := e.addRef(stage, "DB", 1, domain.VersionPublished)
	db2 := e.addRef(stage, "DB", 2, domain.VersionPublished)

	w := do(r, http.MethodPost, "/api/v1/courses", `{"code":"","name":""}`)
	if er := decodeErr(t, w); w.Code != http.StatusUnprocessableEntity || er.Error.Code != "VALIDATION_FAILED" || er.Error.Message != "Nhập mã và tên khóa học." {
		t.Fatalf("create invalid = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, "/api/v1/courses", `{"code":"DEMO","name":"A","bogus":1}`); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown field = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, "/api/v1/courses", `{"code":"DEMO","name":"Khóa demo"}`); w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, "/api/v1/courses", `{"code":"DEMO","name":"Lại"}`); w.Code != http.StatusConflict || decodeErr(t, w).Error.Code != "CONFLICT" {
		t.Fatalf("duplicate = %d %s", w.Code, w.Body)
	}

	var courseID uuid.UUID
	for id := range e.courses.rows {
		courseID = id
	}
	draft, _ := e.versions.DraftOf(context.Background(), nil, courseID)
	vid := draft.ID().String()
	if w = do(r, http.MethodPost, "/api/v1/course-versions/"+vid+"/publish", ""); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("publish empty = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPut, "/api/v1/course-versions/"+vid+"/stages", `{"stageVersionIds":["`+db1.ID.String()+`"]}`); w.Code != http.StatusOK {
		t.Fatalf("set stages = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, "/api/v1/course-versions/"+vid+"/publish", ""); w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body)
	}
	var pub CourseVersionDTO
	if err := json.Unmarshal(w.Body.Bytes(), &pub); err != nil || pub.Status != "published" || pub.PublishedAt == nil || pub.Stages == nil || pub.Classes == nil {
		t.Fatalf("published dto = %s", w.Body)
	}
	if w = do(r, http.MethodPut, "/api/v1/course-versions/"+vid+"/stages", `{"stageVersionIds":[]}`); w.Code != http.StatusConflict || decodeErr(t, w).Error.Code != "VERSION_IMMUTABLE" {
		t.Fatalf("put published = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, "/api/v1/course-versions/"+vid+"/clone", ""); w.Code != http.StatusCreated {
		t.Fatalf("clone = %d %s", w.Code, w.Body)
	}
	var clone CourseVersionDTO
	_ = json.Unmarshal(w.Body.Bytes(), &clone)
	w = do(r, http.MethodPost, "/api/v1/course-versions/"+vid+"/clone", "")
	er := decodeErr(t, w)
	var dd DraftExistsDetails
	_ = json.Unmarshal(er.Error.Details, &dd)
	if w.Code != http.StatusConflict || er.Error.Code != "DRAFT_EXISTS" || dd.DraftVersionID != clone.ID || dd.DraftVersionNo != 2 {
		t.Fatalf("draft exists = %d %s", w.Code, w.Body)
	}

	w = do(r, http.MethodPost, "/api/v1/stage-versions/"+db2.ID.String()+"/apply", `{"courseIds":["`+courseID.String()+`"]}`)
	var res ApplyResultsDTO
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if w.Code != http.StatusOK || len(res.Results) != 1 || res.Results[0].Error == nil || res.Results[0].Error.Code != "DRAFT_EXISTS" {
		t.Fatalf("apply with draft = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodPost, "/api/v1/stage-versions/"+db2.ID.String()+"/apply", `{"courseIds":[]}`); w.Code != http.StatusUnprocessableEntity || decodeErr(t, w).Error.Message != "Chọn ít nhất một khóa học." {
		t.Fatalf("apply empty = %d %s", w.Code, w.Body)
	}

	w = do(r, http.MethodDelete, "/api/v1/course-versions/"+clone.ID, "")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"courseDeleted":false}` {
		t.Fatalf("delete draft = %d %s", w.Code, w.Body)
	}
	if w = do(r, http.MethodGet, "/api/v1/course-versions/not-a-uuid", ""); w.Code != http.StatusNotFound {
		t.Fatalf("bad uuid = %d", w.Code)
	}
	if w = do(r, http.MethodGet, "/api/v1/courses/"+uuid.NewString(), ""); w.Code != http.StatusNotFound {
		t.Fatalf("missing course = %d", w.Code)
	}
}

func TestHandlerInUseEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		fail(c, &ErrInUse{UsedBy: []UsedByClassRow{{ClassID: uuid.New(), Code: "basic01", Name: "Lớp 01", Status: "active", MemberCount: 3}}})
	})
	w := do(r, http.MethodGet, "/", "")
	er := decodeErr(t, w)
	var iu InUseDetails
	_ = json.Unmarshal(er.Error.Details, &iu)
	if w.Code != http.StatusConflict || er.Error.Code != "IN_USE" || len(iu.UsedBy) != 1 || iu.UsedBy[0].MemberCount != 3 {
		t.Fatalf("in use = %d %s", w.Code, w.Body)
	}
}

func TestHandlerUnauthenticated(t *testing.T) {
	e := newEnv(t)
	r := handlerEnv(t, e, false)
	for _, path := range []string{"/api/v1/courses", "/api/v1/stage-versions/" + uuid.NewString() + "/apply"} {
		if w := do(r, http.MethodPost, path, `{}`); w.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d", path, w.Code)
		}
	}
}

func TestVersionStageRowOutdated(t *testing.T) {
	two := domain.VersionNo(2)
	one := domain.FirstVersionNo
	if !(VersionStageRow{StageVersionNo: 1, LatestPublishedNo: &two}).Outdated() {
		t.Fatal("v1 khi đã có v2 phải outdated")
	}
	if (VersionStageRow{StageVersionNo: 1, LatestPublishedNo: &one}).Outdated() || (VersionStageRow{StageVersionNo: 1}).Outdated() {
		t.Fatal("không outdated khi đã là bản mới nhất hoặc chưa có bản phát hành")
	}
}
