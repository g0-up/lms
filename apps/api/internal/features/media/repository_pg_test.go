//go:build integration

package media

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
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
	"lms/api/internal/platform/storage"
	"lms/api/internal/platform/testdb"
)

var update = flag.Bool("update", false, "ghi lại golden JSON trong testdata/")

var (
	teacherID = uuid.MustParse(testdb.TeacherHuongLeID)
	studentID = uuid.MustParse(testdb.StudentAnNguyenID)
	adminID   = uuid.MustParse(testdb.AdminQuanTranID)
)

// openWithClass dọn database và nạp lớp basic01 (active) chạy BASIC v1 gồm DB v1.
func openWithClass(t *testing.T) *sqlx.DB {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	testdb.Fixture(t, dbx, "class_basic01_active")
	return dbx
}

func insertReadyMedia(t *testing.T, dbx *sqlx.DB, kind Kind, ct string) uuid.UUID {
	t.Helper()
	return insertReadyFile(t, dbx, kind, "file", ct)
}

func insertReadyFile(t *testing.T, dbx *sqlx.DB, kind Kind, name, ct string) uuid.UUID {
	t.Helper()
	m, err := NewPendingUpload(ids.New(), kind, name, ct, 1024, adminID, testNow, testCfg.Limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.MarkReady(storage.ObjectStat{Size: 1024, ContentType: m.ContentType()}, testNow); err != nil {
		t.Fatal(err)
	}
	if err := (PGRepo{}).Create(context.Background(), dbx, m); err != nil {
		t.Fatal(err)
	}
	return m.ID()
}

func exec(t *testing.T, dbx *sqlx.DB, q string, args ...any) {
	t.Helper()
	if _, err := dbx.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func TestCanUserAccess(t *testing.T) {
	tests := []struct {
		name   string
		attach bool
		setup  string
		user   uuid.UUID
		want   bool
	}{
		{"giảng viên của lớp", true, "", teacherID, true},
		{"học viên active trong lớp active", true, "", studentID, true},
		{"lớp đã kết thúc vẫn xem lại được", true, `UPDATE classes SET status = 'ended' WHERE id = '` + testdb.ClassBasic01ID + `'`, studentID, true},
		{"lớp nháp chưa mở", true, `UPDATE classes SET status = 'draft' WHERE id = '` + testdb.ClassBasic01ID + `'`, studentID, false},
		{"học viên đã rời lớp", true, `UPDATE class_members SET status = 'dropped', dropped_at = now() WHERE id = '` + testdb.MemberAnBasic01ID + `'`, studentID, false},
		{"người ngoài lớp", true, "", otherTeacherID, false},
		{"người tải lên dù không gắn học liệu", false, "", adminID, true},
		{"media không gắn học liệu", false, "", teacherID, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbx := openWithClass(t)
			id := insertReadyMedia(t, dbx, KindImage, "image/png")
			if tt.attach {
				exec(t, dbx, `INSERT INTO lesson_media (lesson_id, media_id) VALUES ($1, $2)`, testdb.LessonDBTableV1ID, id)
			}
			if tt.setup != "" {
				exec(t, dbx, tt.setup)
			}
			got, err := PGRepo{}.CanUserAccess(context.Background(), dbx, id, tt.user)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("CanUserAccess = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDocumentPersistsCanonicalType(t *testing.T) {
	dbx := openWithClass(t)
	id := insertReadyFile(t, dbx, KindDocument, "schema.sql", "application/octet-stream")
	got, err := PGRepo{}.ByID(context.Background(), dbx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind() != KindDocument || got.ContentType() != "text/plain; charset=utf-8" || !strings.HasSuffix(got.StorageKey(), ".sql") {
		t.Fatalf("document = kind %s type %q key %s", got.Kind(), got.ContentType(), got.StorageKey())
	}
	exec(t, dbx, `INSERT INTO lesson_media (lesson_id, media_id) VALUES ($1, $2)`, testdb.LessonDBTableV1ID, id)
	ok, err := PGRepo{}.CanUserAccess(context.Background(), dbx, id, studentID)
	if err != nil || !ok {
		t.Fatalf("học viên của lớp tải file kèm: ok=%v err=%v", ok, err)
	}
}

func TestPGRepoCreateUpdateByID(t *testing.T) {
	dbx := openWithClass(t)
	ctx := context.Background()
	m, err := NewPendingUpload(ids.New(), KindVideo, "bai-1.mp4", "video/mp4", 2048, adminID, testNow, testCfg.Limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := (PGRepo{}).Create(ctx, dbx, m); err != nil {
		t.Fatal(err)
	}
	got, err := PGRepo{}.ByID(ctx, dbx, m.ID())
	if err != nil || got.Status() != StatusPending || got.StorageKey() != m.StorageKey() || got.ReadyAt() != nil {
		t.Fatalf("pending = %+v err = %v", got, err)
	}
	if err := got.MarkReady(storage.ObjectStat{Size: 2048, ContentType: "video/mp4"}, testNow); err != nil {
		t.Fatal(err)
	}
	if err := (PGRepo{}).Update(ctx, dbx, got); err != nil {
		t.Fatal(err)
	}
	got, err = PGRepo{}.ByID(ctx, dbx, m.ID())
	if err != nil || !got.IsReady() || got.ReadyAt() == nil {
		t.Fatalf("ready = %+v err = %v", got, err)
	}
	if _, err := (PGRepo{}).ByID(ctx, dbx, ids.New()); !errors.Is(err, ErrMediaNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}

// --- golden JSON qua HTTP handler với MemStorage ---

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizeGolden thay thời điểm (khóa kết thúc bằng "At") bằng mốc cố định và id sinh lúc chạy bằng id giả theo thứ
// tự xuất hiện (khóa sắp xếp), để golden ổn định giữa các lần chạy.
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
	dbx := openWithClass(t)
	store := storage.NewMemStorage()
	svc := NewService(dbx, PGRepo{}, store, &clock.Fake{T: testNow}, ids.V7{}, testCfg)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	as := Principal{ID: adminID, Role: domain.RoleAdmin}
	NewHandler(svc, func(*gin.Context) (Principal, bool) { return as, true }).Register(r.Group("/api/v1/media"), func(*gin.Context) {})

	call := func(method, path, body string, want int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, w.Code, want, w.Body)
		}
		return w.Body.Bytes()
	}

	raw := call(http.MethodPost, "/api/v1/media/uploads",
		`{"kind":"video","fileName":"join-co-ban.mp4","contentType":"video/mp4","sizeBytes":4096}`, http.StatusCreated)
	assertGolden(t, "upload_ticket.json", raw)
	var ticket UploadTicketDTO
	if err := json.Unmarshal(raw, &ticket); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "error_not_uploaded.json", call(http.MethodPost, "/api/v1/media/uploads/"+ticket.MediaID+"/complete", "", http.StatusUnprocessableEntity))

	m, err := PGRepo{}.ByID(context.Background(), dbx, uuid.MustParse(ticket.MediaID))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), m.StorageKey(), bytes.NewReader(make([]byte, 4096)), 4096, "video/mp4"); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "media.json", call(http.MethodPost, "/api/v1/media/uploads/"+ticket.MediaID+"/complete", "", http.StatusOK))
	assertGolden(t, "signed_url.json", call(http.MethodGet, "/api/v1/media/"+ticket.MediaID+"/url", "", http.StatusOK))

	as = Principal{ID: studentID, Role: domain.RoleStudent}
	assertGolden(t, "error_forbidden.json", call(http.MethodGet, "/api/v1/media/"+ticket.MediaID+"/url", "", http.StatusForbidden))
}

// --- file bài tập học viên tải lên ---

var otherTeacherID = uuid.MustParse("01990000-0000-7000-8000-000000000901")

// studentEnv là media trên Postgres thật có StudentTx, thêm một giảng viên không dạy basic01.
type studentEnv struct {
	dbx   *sqlx.DB
	store *storage.MemStorage
	svc   *Service
	r     *gin.Engine
}

func newStudentEnv(t *testing.T) studentEnv {
	t.Helper()
	dbx := openWithClass(t)
	exec(t, dbx, `INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash)
		VALUES ($1, 'bao.pham@goup.vn', 'bao.pham@goup.vn', 'Phạm Quốc Bảo', 'teacher', 'active', 'x')`, otherTeacherID)
	store := storage.NewMemStorage()
	cfg := testCfg
	cfg.StudentTx = db.TxRunner{DB: dbx}
	svc := NewService(dbx, PGRepo{}, store, &clock.Fake{T: testNow}, ids.V7{}, cfg)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	sp := Principal{ID: studentID, Role: domain.RoleStudent}
	NewHandler(svc, func(*gin.Context) (Principal, bool) { return sp, true }).RegisterStudent(r.Group("/api/v1/me/uploads"))
	return studentEnv{dbx: dbx, store: store, svc: svc, r: r}
}

func (e studentEnv) post(path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

// upload tải một file bài tập của An qua HTTP tới ready.
func (e studentEnv) upload(t *testing.T, name string) uuid.UUID {
	t.Helper()
	w := e.post("/api/v1/me/uploads", `{"kind":"document","fileName":"`+name+`","contentType":"application/zip","sizeBytes":5}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("init upload = %d: %s", w.Code, w.Body)
	}
	var ticket UploadTicketDTO
	if err := json.Unmarshal(w.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	id := uuid.MustParse(ticket.MediaID)
	m, err := PGRepo{}.ByID(context.Background(), e.dbx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.Put(context.Background(), m.StorageKey(), strings.NewReader("12345"), 5, ticket.Headers["Content-Type"]); err != nil {
		t.Fatal(err)
	}
	if w := e.post("/api/v1/me/uploads/"+ticket.MediaID+"/complete", ""); w.Code != http.StatusOK {
		t.Fatalf("complete = %d: %s", w.Code, w.Body)
	}
	return id
}

// attach gắn file vào một bản nộp của An ở basic01.
func attach(t *testing.T, dbx *sqlx.DB, mediaID uuid.UUID) {
	t.Helper()
	sid := ids.New()
	exec(t, dbx, `INSERT INTO submissions (id, class_member_id, lesson_id, attempt_no, submitted_at, status)
		VALUES ($1, $2, $3, (SELECT count(*) + 1 FROM submissions WHERE class_member_id = $2), now(), 'superseded')`,
		sid, testdb.MemberAnBasic01ID, testdb.LessonDBTableV1ID)
	exec(t, dbx, `INSERT INTO submission_files (submission_id, media_id, position) VALUES ($1, $2, 1)`, sid, mediaID)
}

func TestStudentUploadAndAccess(t *testing.T) {
	e := newStudentEnv(t)
	ctx := context.Background()
	id := e.upload(t, "bài nộp.zip")

	m, err := PGRepo{}.ByID(ctx, e.dbx, id)
	if err != nil || m.UploadedBy() != studentID || m.Kind() != KindDocument || !m.IsReady() {
		t.Fatalf("file học viên = %+v err = %v", m, err)
	}

	// Người tải lên xem được file của mình, luôn dạng attachment.
	s, err := e.svc.SignedURL(ctx, Principal{ID: studentID, Role: domain.RoleStudent}, id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.URL, "response-content-disposition=attachment") {
		t.Fatalf("URL file học viên không phải attachment: %s", s.URL)
	}

	// Chưa gắn bản nộp: giảng viên lớp chưa xem được.
	teacher := Principal{ID: teacherID, Role: domain.RoleTeacher}
	if _, err := e.svc.SignedURL(ctx, teacher, id); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("giảng viên xem file chưa nộp: err = %v", err)
	}
	attach(t, e.dbx, id)
	s, err = e.svc.SignedURL(ctx, teacher, id)
	if err != nil || !strings.Contains(s.URL, "response-content-disposition=attachment") {
		t.Fatalf("giảng viên lớp xem file đã nộp: url=%s err=%v", s.URL, err)
	}
	// Giảng viên lớp khác và học viên khác không xem được.
	if _, err := e.svc.SignedURL(ctx, Principal{ID: otherTeacherID, Role: domain.RoleTeacher}, id); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("giảng viên lớp khác: err = %v", err)
	}
	other := ids.New()
	exec(t, e.dbx, `INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash)
		VALUES ($1, 'linh.do@gmail.com', 'linh.do@gmail.com', 'Đỗ Ngọc Linh', 'student', 'active', 'x')`, other)
	exec(t, e.dbx, `INSERT INTO class_members (id, class_id, user_id, status, joined_at) VALUES ($1, $2, $3, 'active', now())`,
		ids.New(), testdb.ClassBasic01ID, other)
	if _, err := e.svc.SignedURL(ctx, Principal{ID: other, Role: domain.RoleStudent}, id); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("học viên khác cùng lớp: err = %v", err)
	}
	// Học viên khác không hoàn tất được file của An.
	if _, err := e.svc.CompleteUpload(ctx, Principal{ID: other, Role: domain.RoleStudent}, id); !errors.Is(err, ErrMediaNotFound) {
		t.Fatalf("học viên khác complete: err = %v", err)
	}
}

func TestStudentUploadRequiresActiveEnrollment(t *testing.T) {
	tests := []struct {
		name  string
		setup string
	}{
		{"lớp đã kết thúc", `UPDATE classes SET status = 'ended' WHERE id = '` + testdb.ClassBasic01ID + `'`},
		{"lớp nháp", `UPDATE classes SET status = 'draft' WHERE id = '` + testdb.ClassBasic01ID + `'`},
		{"đã rời lớp", `UPDATE class_members SET status = 'dropped', dropped_at = now() WHERE id = '` + testdb.MemberAnBasic01ID + `'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newStudentEnv(t)
			exec(t, e.dbx, tt.setup)
			w := e.post("/api/v1/me/uploads", `{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":5}`)
			if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "Bạn cần đang học một lớp để tải file bài tập.") {
				t.Fatalf("upload = %d: %s", w.Code, w.Body)
			}
		})
	}
}

func TestStudentUploadKindAndLimits(t *testing.T) {
	e := newStudentEnv(t)
	tests := []struct {
		body string
		code int
		msg  string
	}{
		{`{"kind":"video","fileName":"a.mp4","contentType":"video/mp4","sizeBytes":5}`, http.StatusForbidden, "Học viên chỉ tải lên được file bài tập."},
		{`{"kind":"image","fileName":"a.png","contentType":"image/png","sizeBytes":5}`, http.StatusForbidden, "Học viên chỉ tải lên được file bài tập."},
		{`{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":20971521}`, http.StatusUnprocessableEntity, "File vượt giới hạn 20 MB."},
		{`{"kind":"document","fileName":"a.html","contentType":"text/plain","sizeBytes":5}`, http.StatusUnprocessableEntity, "Định dạng không hỗ trợ."},
	}
	for _, tt := range tests {
		w := e.post("/api/v1/me/uploads", tt.body)
		if w.Code != tt.code || !strings.Contains(w.Body.String(), tt.msg) {
			t.Fatalf("%s = %d: %s", tt.body, w.Code, w.Body)
		}
	}
	var n int
	if err := e.dbx.Get(&n, `SELECT count(*) FROM media_files WHERE uploaded_by = $1`, studentID); err != nil || n != 0 {
		t.Fatalf("file bị từ chối vẫn lưu: n=%d err=%v", n, err)
	}
}

// File chưa nộp chỉ giữ chỗ trong hạn mức một thời gian: pending quá hạn URL PUT và ready quá 24 giờ không còn tính,
// nên học viên bỏ dở upload vẫn tải được về sau. File mới hơn các mốc đó vẫn tính đủ.
func TestStudentUnattachedLimitForgetsStaleUploads(t *testing.T) {
	e := newStudentEnv(t)
	insert := func(status string, createdAt time.Time) {
		id := ids.New()
		var readyAt *time.Time
		if status == "ready" {
			readyAt = &createdAt
		}
		exec(t, e.dbx, `INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by, created_at, ready_at)
			VALUES ($1::uuid, 'document', 'documents/' || $1::text, 'a.zip', 'application/zip', 5, $2, $3, $4, $5)`,
			id, status, studentID, createdAt, readyAt)
	}
	for range MaxUnattachedDocuments / 2 {
		insert("pending", testNow.Add(-UploadTTL-time.Second))
		insert("ready", testNow.Add(-UnattachedWindow-time.Second))
	}
	const body = `{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":5}`
	if w := e.post("/api/v1/me/uploads", body); w.Code != http.StatusCreated {
		t.Fatalf("file cũ vẫn giữ chỗ = %d: %s", w.Code, w.Body)
	}

	// Đã có 1 pending vừa tạo; thêm file ngay trong hai mốc cho đủ 10.
	insert("pending", testNow.Add(-UploadTTL))
	for range MaxUnattachedDocuments - 2 {
		insert("ready", testNow.Add(-UnattachedWindow))
	}
	if w := e.post("/api/v1/me/uploads", body); w.Code != http.StatusConflict {
		t.Fatalf("file trong hạn phải tính vào hạn mức = %d: %s", w.Code, w.Body)
	}
}

// Mười hai request song song từ 0 file: đúng 10 được nhận (khóa theo học viên), phần còn lại INVALID_TRANSITION.
// Gắn một file vào bản nộp thì tải thêm được đúng một file.
func TestStudentUnattachedLimitIsSerialized(t *testing.T) {
	e := newStudentEnv(t)
	const n = MaxUnattachedDocuments + 2
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = e.post("/api/v1/me/uploads", `{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":5}`).Code
		}()
	}
	wg.Wait()
	created, rejected := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			rejected++
		default:
			t.Fatalf("mã lạ %d trong %v", c, codes)
		}
	}
	if created != MaxUnattachedDocuments || rejected != 2 {
		t.Fatalf("created=%d rejected=%d (%v)", created, rejected, codes)
	}
	w := e.post("/api/v1/me/uploads", `{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":5}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "Bạn có quá nhiều file chưa nộp. Hãy nộp hoặc chờ trước khi tải thêm.") {
		t.Fatalf("file thứ 11 = %d: %s", w.Code, w.Body)
	}

	var first uuid.UUID
	if err := e.dbx.Get(&first, `SELECT id FROM media_files WHERE uploaded_by = $1 ORDER BY id LIMIT 1`, studentID); err != nil {
		t.Fatal(err)
	}
	attach(t, e.dbx, first)
	if w := e.post("/api/v1/me/uploads", `{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":5}`); w.Code != http.StatusCreated {
		t.Fatalf("sau khi nộp một file = %d: %s", w.Code, w.Body)
	}
	if w := e.post("/api/v1/me/uploads", `{"kind":"document","fileName":"a.zip","contentType":"application/zip","sizeBytes":5}`); w.Code != http.StatusConflict {
		t.Fatalf("vượt giới hạn lần nữa = %d: %s", w.Code, w.Body)
	}
}
