package media

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	testCfg  = Config{Limits: Limits{MaxVideoBytes: 2 << 30, MaxImageBytes: 10 << 20}, URLTTL: 2 * time.Hour}
)

type fixedIDs struct{ id uuid.UUID }

func (f fixedIDs) New() uuid.UUID { return f.id }

// memRepo là Repo trong bộ nhớ; access liệt kê cặp media-user được CanUserAccess cho phép.
type memRepo struct {
	mu     sync.Mutex
	files  map[uuid.UUID]MediaFile
	access map[[2]uuid.UUID]bool
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

type fixture struct {
	svc   *Service
	repo  *memRepo
	store *storage.MemStorage
	clk   *clock.Fake
	id    uuid.UUID
}

func newFixture() fixture {
	repo, store, clk := newMemRepo(), storage.NewMemStorage(), &clock.Fake{T: testNow}
	id := uuid.MustParse("01990000-0000-7000-8000-000000000901")
	return fixture{svc: NewService(nil, repo, store, clk, fixedIDs{id}, testCfg), repo: repo, store: store, clk: clk, id: id}
}

func TestNewPendingUploadRules(t *testing.T) {
	limits := testCfg.Limits
	tests := []struct {
		name, kind, file, ct string
		size                 int64
		wantMsg              string
		wantKey              string
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
			assert.Equal(t, tt.wantKey, m.StorageKey())
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

func TestUploadHandshake(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	ticket, err := f.svc.InitUpload(ctx, adminP, InitUploadCmd{Kind: "video", FileName: "a.mp4", ContentType: "video/mp4", SizeBytes: 5})
	require.NoError(t, err)
	assert.Equal(t, f.id, ticket.MediaID)
	assert.Equal(t, testNow.Add(UploadTTL), ticket.ExpiresAt)
	assert.Contains(t, ticket.UploadURL, "X-Amz-Signature")

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

func TestUploadRequiresAdmin(t *testing.T) {
	f := newFixture()
	_, err := f.svc.InitUpload(context.Background(), studentP, InitUploadCmd{Kind: "video", FileName: "a.mp4", ContentType: "video/mp4", SizeBytes: 5})
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = f.svc.CompleteUpload(context.Background(), studentP, f.id)
	require.ErrorIs(t, err, domain.ErrForbidden)
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
