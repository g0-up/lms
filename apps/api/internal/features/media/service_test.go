package media

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/httpx"
	"lms/api/internal/platform/storage"
)

var (
	testNow  = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	adminP   = Principal{ID: uuid.MustParse("01990000-0000-7000-8000-000000000001"), Role: domain.RoleAdmin}
	studentP = Principal{ID: uuid.MustParse("01990000-0000-7000-8000-000000000003"), Role: domain.RoleStudent}
	testCfg  = Config{Limits: Limits{MaxVideoBytes: 2 << 30, MaxImageBytes: 10 << 20, MaxDocumentBytes: 20 << 20}, URLTTL: 2 * time.Hour}
)

type fixedIDs struct{ id uuid.UUID }

func (f fixedIDs) New() uuid.UUID { return f.id }

// memRepo là Repo trong bộ nhớ; access liệt kê cặp media-user được CanUserAccess cho phép; quota là điều kiện
// upload của mọi học viên.
type memRepo struct {
	mu     sync.Mutex
	files  map[uuid.UUID]MediaFile
	access map[[2]uuid.UUID]bool
	quota  StudentQuota
	window QuotaWindow // QuotaWindow của lần đọc hạn mức gần nhất
}

func newMemRepo() *memRepo {
	return &memRepo{files: map[uuid.UUID]MediaFile{}, access: map[[2]uuid.UUID]bool{}}
}

func (r *memRepo) Create(_ context.Context, _ db.Executor, m *MediaFile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files[m.ID()] = *m
	return nil
}

func (r *memRepo) Update(_ context.Context, _ db.Executor, m *MediaFile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.files[m.ID()]; !ok {
		return ErrMediaNotFound
	}
	r.files[m.ID()] = *m
	return nil
}

func (r *memRepo) ByID(_ context.Context, _ db.Executor, id uuid.UUID) (*MediaFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.files[id]
	if !ok {
		return nil, ErrMediaNotFound
	}
	return &m, nil
}

func (r *memRepo) CanUserAccess(_ context.Context, _ db.Executor, mediaID, userID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.access[[2]uuid.UUID{mediaID, userID}], nil
}

func (r *memRepo) StudentUploadQuota(_ context.Context, _ db.Executor, _ uuid.UUID, w QuotaWindow) (StudentQuota, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.window = w
	return r.quota, nil
}

// nopTx chạy fn không có transaction thật; đủ cho memRepo.
type nopTx struct{}

func (nopTx) Transact(_ context.Context, fn func(db.Executor) error) error { return fn(nil) }

type fixture struct {
	svc   *Service
	repo  *memRepo
	store *storage.MemStorage
	clk   *clock.Fake
	id    uuid.UUID
}

func newFixture() fixture {
	return newFixtureWith(testCfg)
}

func newFixtureWith(cfg Config) fixture {
	repo, store, clk := newMemRepo(), storage.NewMemStorage(), &clock.Fake{T: testNow}
	id := uuid.MustParse("01990000-0000-7000-8000-000000000901")
	return fixture{svc: NewService(nil, repo, store, clk, fixedIDs{id}, cfg), repo: repo, store: store, clk: clk, id: id}
}

func TestNewPendingUploadRules(t *testing.T) {
	limits := testCfg.Limits
	tests := []struct {
		name, kind, file, ct string
		size                 int64
		wantMsg              string
		wantKey              string
		wantCT               string
	}{
		{name: "video mp4", kind: "video", file: "intro.mp4", ct: "video/mp4", size: 1 << 20, wantKey: "media/video/2026/10/01990000-0000-7000-8000-000000000901.mp4"},
		{name: "ảnh jpeg chuẩn hóa content type", kind: "image", file: "a.JPG", ct: "Image/JPEG; charset=binary", size: 10, wantKey: "media/image/2026/10/01990000-0000-7000-8000-000000000901.jpg"},
		{name: "video sai định dạng", kind: "video", file: "a.mov", ct: "video/quicktime", size: 10, wantMsg: "Định dạng không hỗ trợ."},
		{name: "ảnh svg bị chặn", kind: "image", file: "a.svg", ct: "image/svg+xml", size: 10, wantMsg: "Định dạng không hỗ trợ."},
		{name: "video dán nhãn ảnh", kind: "image", file: "a.mp4", ct: "video/mp4", size: 10, wantMsg: "Định dạng không hỗ trợ."},
		{name: "video quá 2 GB", kind: "video", file: "a.mp4", ct: "video/mp4", size: 2<<30 + 1, wantMsg: "File vượt giới hạn 2 GB."},
		{name: "ảnh quá 10 MB", kind: "image", file: "a.png", ct: "image/png", size: 10<<20 + 1, wantMsg: "File vượt giới hạn 10 MB."},
		{name: "dung lượng 0", kind: "image", file: "a.png", ct: "image/png", size: 0, wantMsg: "Kích thước file không hợp lệ."},
		{name: "tên rỗng", kind: "image", file: "  ", ct: "image/png", size: 1, wantMsg: "Tên file không hợp lệ."},
		{name: "tên có đường dẫn", kind: "image", file: "../a.png", ct: "image/png", size: 1, wantMsg: "Tên file không hợp lệ."},
		{name: "kind lạ", kind: "audio", file: "a.mp3", ct: "audio/mpeg", size: 1, wantMsg: "Loại file không hợp lệ."},
		{name: "document sql gửi octet-stream", kind: "document", file: "schema.SQL", ct: "application/octet-stream", size: 10, wantKey: "media/document/2026/10/01990000-0000-7000-8000-000000000901.sql", wantCT: "text/plain; charset=utf-8"},
		{name: "document sql không có MIME", kind: "document", file: "seed.sql", ct: "", size: 10, wantCT: "text/plain; charset=utf-8"},
		{name: "document zip", kind: "document", file: "starter.zip", ct: "application/x-zip-compressed", size: 10, wantCT: "application/zip"},
		{name: "document pdf", kind: "document", file: "de bai.pdf", ct: "application/pdf", size: 20 << 20, wantCT: "application/pdf"},
		{name: "document markdown", kind: "document", file: "README.md", ct: "text/markdown", size: 10, wantCT: "text/plain; charset=utf-8"},
		{name: "document txt", kind: "document", file: "notes.txt", ct: "text/plain", size: 10, wantCT: "text/plain; charset=utf-8"},
		{name: "document ảnh jpeg", kind: "document", file: "erd.jpeg", ct: "image/jpeg", size: 10, wantKey: "media/document/2026/10/01990000-0000-7000-8000-000000000901.jpeg", wantCT: "image/jpeg"},
		{name: "document png", kind: "document", file: "erd.png", ct: "", size: 10, wantCT: "image/png"},
		{name: "document html bị chặn dù khai text/plain", kind: "document", file: "a.html", ct: "text/plain", size: 10, wantMsg: "Định dạng không hỗ trợ."},
		{name: "document svg bị chặn", kind: "document", file: "a.svg", ct: "image/svg+xml", size: 10, wantMsg: "Định dạng không hỗ trợ."},
		{name: "document không có đuôi", kind: "document", file: "Makefile", ct: "text/plain", size: 10, wantMsg: "Định dạng không hỗ trợ."},
		{name: "document đuôi giả trước", kind: "document", file: "a.pdf.exe", ct: "application/pdf", size: 10, wantMsg: "Định dạng không hỗ trợ."},
		{name: "document quá 20 MB", kind: "document", file: "a.zip", ct: "application/zip", size: 20<<20 + 1, wantMsg: "File vượt giới hạn 20 MB."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewPendingUpload(uuid.MustParse("01990000-0000-7000-8000-000000000901"), Kind(tt.kind), tt.file, tt.ct, tt.size, adminP.ID, testNow, limits)
			if tt.wantMsg != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantMsg, err.Error())
				assert.ErrorIs(t, err, domain.ErrInvalid)
				return
			}
			require.NoError(t, err)
			if tt.wantKey != "" {
				assert.Equal(t, tt.wantKey, m.StorageKey())
			}
			if tt.wantCT != "" {
				assert.Equal(t, tt.wantCT, m.ContentType())
			}
			assert.Equal(t, StatusPending, m.Status())
		})
	}
}

func TestMarkReady(t *testing.T) {
	m, err := NewPendingUpload(uuid.New(), KindImage, "a.png", "image/png", 5, adminP.ID, testNow, testCfg.Limits)
	require.NoError(t, err)
	require.ErrorIs(t, m.MarkReady(storage.ObjectStat{Size: 6, ContentType: "image/png"}, testNow), ErrUploadMismatch)
	require.ErrorIs(t, m.MarkReady(storage.ObjectStat{Size: 5, ContentType: "image/gif"}, testNow), ErrUploadMismatch)
	require.NoError(t, m.MarkReady(storage.ObjectStat{Size: 5, ContentType: "image/png"}, testNow))
	assert.True(t, m.IsReady())
	assert.Equal(t, testNow, *m.ReadyAt())
	// Gọi lại khi đã ready không kiểm lại, không lỗi.
	require.NoError(t, m.MarkReady(storage.ObjectStat{}, testNow.Add(time.Hour)))
	assert.Equal(t, testNow, *m.ReadyAt())
}

func TestMarkReadyDocumentWithCharset(t *testing.T) {
	m, err := NewPendingUpload(uuid.New(), KindDocument, "schema.sql", "application/sql", 5, adminP.ID, testNow, testCfg.Limits)
	require.NoError(t, err)
	require.ErrorIs(t, m.MarkReady(storage.ObjectStat{Size: 5, ContentType: "application/sql"}, testNow), ErrUploadMismatch)
	require.NoError(t, m.MarkReady(storage.ObjectStat{Size: 5, ContentType: "text/plain; charset=utf-8"}, testNow))
	assert.Equal(t, "text/plain; charset=utf-8", m.ContentType())
}

func TestUploadHandshake(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	ticket, err := f.svc.InitUpload(ctx, adminP, InitUploadCmd{Kind: "video", FileName: "a.mp4", ContentType: "video/mp4", SizeBytes: 5})
	require.NoError(t, err)
	assert.Equal(t, f.id, ticket.MediaID)
	assert.Equal(t, testNow.Add(UploadTTL), ticket.ExpiresAt)
	assert.Contains(t, ticket.UploadURL, "X-Amz-Signature")
	assert.Equal(t, map[string]string{"Content-Type": "video/mp4"}, ticket.Headers)

	_, err = f.svc.CompleteUpload(ctx, adminP, f.id)
	require.ErrorIs(t, err, ErrNotUploaded)
	assert.Equal(t, "Chưa nhận được file. Tải lên lại.", err.Error())

	pending, err := f.repo.ByID(ctx, nil, f.id)
	require.NoError(t, err)
	require.NoError(t, f.store.Put(ctx, pending.StorageKey(), strings.NewReader("12345"), 5, "video/mp4"))

	m, err := f.svc.CompleteUpload(ctx, adminP, f.id)
	require.NoError(t, err)
	assert.Equal(t, StatusReady, m.Status())
	stored, err := f.repo.ByID(ctx, nil, f.id)
	require.NoError(t, err)
	assert.True(t, stored.IsReady())

	// complete lần hai là no-op.
	_, err = f.svc.CompleteUpload(ctx, adminP, f.id)
	require.NoError(t, err)
}

func TestUploadRequiresAdminWithoutStudentTx(t *testing.T) {
	f := newFixture()
	_, err := f.svc.InitUpload(context.Background(), studentP, InitUploadCmd{Kind: "document", FileName: "a.zip", ContentType: "application/zip", SizeBytes: 5})
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = f.svc.CompleteUpload(context.Background(), studentP, f.id)
	require.ErrorIs(t, err, domain.ErrForbidden)
	teacherP := Principal{ID: uuid.MustParse("01990000-0000-7000-8000-000000000002"), Role: domain.RoleTeacher}
	_, err = newFixtureWith(studentCfg()).svc.InitUpload(context.Background(), teacherP, InitUploadCmd{Kind: "document", FileName: "a.zip", ContentType: "application/zip", SizeBytes: 5})
	require.ErrorIs(t, err, domain.ErrForbidden, "giảng viên không upload")
}

func studentCfg() Config {
	cfg := testCfg
	cfg.StudentTx = nopTx{}
	return cfg
}

func TestStudentUploadRules(t *testing.T) {
	ctx := context.Background()
	doc := InitUploadCmd{Kind: "document", FileName: "bai-nop.zip", ContentType: "application/zip", SizeBytes: 5}
	tests := []struct {
		name  string
		cmd   InitUploadCmd
		quota StudentQuota
		want  error
		msg   string
	}{
		{"đang học, còn chỗ", doc, StudentQuota{Enrolled: true, Unattached: MaxUnattachedDocuments - 1}, nil, ""},
		{"không học lớp active nào", doc, StudentQuota{}, domain.ErrForbidden, "Bạn cần đang học một lớp để tải file bài tập."},
		{"đủ 10 file chưa gắn", doc, StudentQuota{Enrolled: true, Unattached: MaxUnattachedDocuments}, domain.ErrInvalidTransition,
			"Bạn có quá nhiều file chưa nộp. Hãy nộp hoặc chờ trước khi tải thêm."},
		{"video bị chặn", InitUploadCmd{Kind: "video", FileName: "a.mp4", ContentType: "video/mp4", SizeBytes: 5}, StudentQuota{Enrolled: true},
			domain.ErrForbidden, "Học viên chỉ tải lên được file bài tập."},
		{"ảnh bị chặn", InitUploadCmd{Kind: "image", FileName: "a.png", ContentType: "image/png", SizeBytes: 5}, StudentQuota{Enrolled: true},
			domain.ErrForbidden, "Học viên chỉ tải lên được file bài tập."},
		{"quá 20 MB", InitUploadCmd{Kind: "document", FileName: "a.zip", ContentType: "application/zip", SizeBytes: 20<<20 + 1},
			StudentQuota{Enrolled: true}, domain.ErrInvalid, "File vượt giới hạn 20 MB."},
		{"đuôi lạ", InitUploadCmd{Kind: "document", FileName: "a.exe", ContentType: "application/zip", SizeBytes: 5},
			StudentQuota{Enrolled: true}, domain.ErrInvalid, "Định dạng không hỗ trợ."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixtureWith(studentCfg())
			f.repo.quota = tt.quota
			_, err := f.svc.InitUpload(ctx, studentP, tt.cmd)
			if tt.want == nil {
				require.NoError(t, err)
				m, err := f.repo.ByID(ctx, nil, f.id)
				require.NoError(t, err)
				assert.Equal(t, studentP.ID, m.UploadedBy())
				assert.Equal(t, QuotaWindow{ReadySince: testNow.Add(-24 * time.Hour), PendingSince: testNow.Add(-15 * time.Minute)},
					f.repo.window, "file ready giữ chỗ 24 giờ, file pending giữ chỗ trong hạn URL PUT")
				return
			}
			require.ErrorIs(t, err, tt.want)
			assert.Equal(t, tt.msg, err.Error())
			_, err = f.repo.ByID(ctx, nil, f.id)
			require.ErrorIs(t, err, ErrMediaNotFound, "bị từ chối thì không lưu bản ghi")
		})
	}
}

func TestStudentCompletesOnlyOwnUpload(t *testing.T) {
	ctx := context.Background()
	f := newFixtureWith(studentCfg())
	f.repo.quota = StudentQuota{Enrolled: true}
	_, err := f.svc.InitUpload(ctx, studentP, InitUploadCmd{Kind: "document", FileName: "bai-nop.zip", ContentType: "application/zip", SizeBytes: 5})
	require.NoError(t, err)
	m, err := f.repo.ByID(ctx, nil, f.id)
	require.NoError(t, err)
	require.NoError(t, f.store.Put(ctx, m.StorageKey(), strings.NewReader("12345"), 5, m.ContentType()))

	other := Principal{ID: uuid.MustParse("01990000-0000-7000-8000-000000000004"), Role: domain.RoleStudent}
	_, err = f.svc.CompleteUpload(ctx, other, f.id)
	require.ErrorIs(t, err, ErrMediaNotFound, "học viên khác không hoàn tất được file")

	got, err := f.svc.CompleteUpload(ctx, studentP, f.id)
	require.NoError(t, err)
	assert.True(t, got.IsReady())
}

func readyMedia(t *testing.T, f fixture) {
	t.Helper()
	ctx := context.Background()
	_, err := f.svc.InitUpload(ctx, adminP, InitUploadCmd{Kind: "video", FileName: "a.mp4", ContentType: "video/mp4", SizeBytes: 5})
	require.NoError(t, err)
	m, err := f.repo.ByID(ctx, nil, f.id)
	require.NoError(t, err)
	require.NoError(t, f.store.Put(ctx, m.StorageKey(), strings.NewReader("12345"), 5, "video/mp4"))
	_, err = f.svc.CompleteUpload(ctx, adminP, f.id)
	require.NoError(t, err)
}

func TestSignedURLAuthorization(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	readyMedia(t, f)

	s, err := f.svc.SignedURL(ctx, adminP, f.id)
	require.NoError(t, err)
	assert.Equal(t, testNow.Add(2*time.Hour), s.ExpiresAt)
	assert.Contains(t, s.URL, "response-content-type=video%2Fmp4")

	_, err = f.svc.SignedURL(ctx, studentP, f.id)
	require.ErrorIs(t, err, domain.ErrForbidden)

	f.repo.access[[2]uuid.UUID{f.id, studentP.ID}] = true
	_, err = f.svc.SignedURL(ctx, studentP, f.id)
	require.NoError(t, err)

	_, err = f.svc.SignedURL(ctx, adminP, uuid.New())
	require.ErrorIs(t, err, ErrMediaNotFound)
}

func TestDocumentUploadUsesCanonicalTypeAndAttachment(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	ticket, err := f.svc.InitUpload(ctx, adminP, InitUploadCmd{Kind: "document", FileName: "bài tập 1.sql", ContentType: "application/octet-stream", SizeBytes: 5})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"Content-Type": "text/plain; charset=utf-8"}, ticket.Headers, "ticket phải yêu cầu type chuẩn, không phải type client")

	m, err := f.repo.ByID(ctx, nil, f.id)
	require.NoError(t, err)
	assert.Equal(t, "text/plain; charset=utf-8", m.ContentType())
	require.NoError(t, f.store.Put(ctx, m.StorageKey(), strings.NewReader("12345"), 5, ticket.Headers["Content-Type"]))
	_, err = f.svc.CompleteUpload(ctx, adminP, f.id)
	require.NoError(t, err)

	s, err := f.svc.SignedURL(ctx, adminP, f.id)
	require.NoError(t, err)
	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	assert.Equal(t, "text/plain; charset=utf-8", u.Query().Get("response-content-type"))
	assert.Equal(t, "attachment; filename*=UTF-8''b%C3%A0i%20t%E1%BA%ADp%201.sql", u.Query().Get("response-content-disposition"))

	_, err = f.svc.InitUpload(ctx, adminP, InitUploadCmd{Kind: "document", FileName: "x.html", ContentType: "text/plain", SizeBytes: 5})
	require.ErrorIs(t, err, ErrUnsupportedType)
}

func TestSignedURLImageAndVideoAreInline(t *testing.T) {
	f := newFixture()
	readyMedia(t, f)
	s, err := f.svc.SignedURL(context.Background(), adminP, f.id)
	require.NoError(t, err)
	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	assert.False(t, u.Query().Has("response-content-disposition"), "video phải phát trực tiếp")
}

func TestSignedURLPendingMedia(t *testing.T) {
	f := newFixture()
	_, err := f.svc.InitUpload(context.Background(), adminP, InitUploadCmd{Kind: "image", FileName: "a.png", ContentType: "image/png", SizeBytes: 5})
	require.NoError(t, err)
	_, err = f.svc.SignedURL(context.Background(), adminP, f.id)
	require.ErrorIs(t, err, ErrNotReady)
}

// --- HTTP ---

func stubAdmin(c *gin.Context) { c.Next() }

func newRouter(f fixture, p *Principal) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(f.svc, func(*gin.Context) (Principal, bool) {
		if p == nil {
			return Principal{}, false
		}
		return *p, true
	})
	h.Register(r.Group("/api/v1/media"), stubAdmin)
	return r
}

func do(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func errorBody(t *testing.T, w *httptest.ResponseRecorder) httpx.ErrorBody {
	t.Helper()
	var env httpx.ErrorEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	return env.Error
}

func TestHandlerInitUpload(t *testing.T) {
	f := newFixture()
	r := newRouter(f, &adminP)

	w := do(r, http.MethodPost, "/api/v1/media/uploads", `{"kind":"video","fileName":"a.mp4","contentType":"video/mp4","sizeBytes":1048576}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, f.id.String(), got["mediaId"])
	assert.NotEmpty(t, got["uploadUrl"])
	assert.Equal(t, "2026-10-05T09:15:00Z", got["expiresAt"])

	w = do(r, http.MethodPost, "/api/v1/media/uploads", `{"kind":"video","fileName":"a.mov","contentType":"video/quicktime","sizeBytes":10}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, httpx.ErrorBody{Code: apperr.CodeValidationFailed, Message: "Định dạng không hỗ trợ."}, errorBody(t, w))

	w = do(r, http.MethodPost, "/api/v1/media/uploads", `{"kind":"video","fileName":"a.mp4","contentType":"video/mp4","sizeBytes":3000000000}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "File vượt giới hạn 2 GB.", errorBody(t, w).Message)

	w = do(r, http.MethodPost, "/api/v1/media/uploads", `{"kind":`)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerCompleteAndURL(t *testing.T) {
	f := newFixture()
	r := newRouter(f, &adminP)
	_, err := f.svc.InitUpload(context.Background(), adminP, InitUploadCmd{Kind: "image", FileName: "a.png", ContentType: "image/png", SizeBytes: 3})
	require.NoError(t, err)

	w := do(r, http.MethodPost, "/api/v1/media/uploads/"+f.id.String()+"/complete", "")
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "Chưa nhận được file. Tải lên lại.", errorBody(t, w).Message)

	m, err := f.repo.ByID(context.Background(), nil, f.id)
	require.NoError(t, err)
	require.NoError(t, f.store.Put(context.Background(), m.StorageKey(), strings.NewReader("abc"), 3, "image/png"))

	w = do(r, http.MethodPost, "/api/v1/media/uploads/"+f.id.String()+"/complete", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"id":"`+f.id.String()+`","kind":"image","fileName":"a.png","contentType":"image/png","sizeBytes":3,"status":"ready"}`, w.Body.String())

	w = do(r, http.MethodGet, "/api/v1/media/"+f.id.String()+"/url", "")
	require.Equal(t, http.StatusOK, w.Code)
	var su map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &su))
	assert.Equal(t, "2026-10-05T11:00:00Z", su["expiresAt"])

	w = do(r, http.MethodGet, "/api/v1/media/"+f.id.String()+"/content", "")
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "private, max-age=300", w.Header().Get("Cache-Control"))
	assert.True(t, strings.HasPrefix(w.Header().Get("Location"), "https://storage.mem/get/"))

	w = do(r, http.MethodGet, "/api/v1/media/not-a-uuid/url", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerForbiddenAndUnauthenticated(t *testing.T) {
	f := newFixture()
	readyMedia(t, f)

	w := do(newRouter(f, &studentP), http.MethodGet, "/api/v1/media/"+f.id.String()+"/content", "")
	require.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, apperr.CodeForbidden, errorBody(t, w).Code)
	assert.Empty(t, w.Header().Get("Location"))

	w = do(newRouter(f, nil), http.MethodGet, "/api/v1/media/"+f.id.String()+"/url", "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
