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
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/platform/clock"
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
	m, err := NewPendingUpload(ids.New(), kind, "file", ct, 1024, adminID, testNow, testCfg.Limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.MarkReady(storage.ObjectStat{Size: 1024, ContentType: ct}, testNow); err != nil {
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
		{"người ngoài lớp", true, "", adminID, false},
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
