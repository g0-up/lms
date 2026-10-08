//go:build integration

package classes

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
)

var update = flag.Bool("update", false, "ghi lại golden JSON trong testdata/")

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

func (e *itEnv) mustInTx(fn func(tx db.Executor) error) {
	e.t.Helper()
	if err := e.inTx(fn); err != nil {
		e.t.Fatal(err)
	}
}

// addFailedInvite thêm khang.vu vào basic01 với lời mời gửi thất bại 3 lần như bản failed của seed.js.
func (e *itEnv) addFailedInvite() {
	e.t.Helper()
	outboxID, memberID, invID := ids.New(), uuid.MustParse("01990000-0000-7000-8000-000000000971"), ids.New()
	e.exec(`INSERT INTO class_members (id, class_id, user_id, status, joined_at) VALUES ($1, $2, $3, 'active', now() - interval '30 days')`,
		memberID, basic01ID, khangID)
	e.exec(`INSERT INTO email_outbox (id, to_email, template, payload, status, attempts, last_error, created_at)
		VALUES ($1, 'khang.vu@gmail.com', 'added', '{}', 'failed', 3, 'Mailbox không tồn tại (550 5.1.1)', now() - interval '30 days')`, outboxID)
	e.exec(`INSERT INTO invitations (id, class_id, user_id, kind, email_outbox_id, invited_by, created_at)
		VALUES ($1, $2, $3, 'added', $4, $5, now() - interval '30 days')`, invID, basic01ID, khangID, outboxID, adminID)
}

func TestClassRepoConstraints(t *testing.T) {
	e := newIT(t)
	repo := PGClassRepo{}
	sameDay := day("2026-11-01")
	newClass := func(code string, dates DateRange) *Class {
		return &Class{id: ids.New(), code: domain.Code(code), name: "Lớp thử", courseVersionID: basicV1ID, status: domain.ClassDraft,
			dates: dates, teacherID: huongID, createdBy: adminID, createdAt: e.clk.Now()}
	}

	if err := e.inTx(func(tx db.Executor) error {
		return repo.Create(e.ctx(), tx, newClass("basic09", DateRange{Start: sameDay, End: sameDay}))
	}); !errors.Is(err, ErrInvalidDates) {
		t.Fatalf("CHECK end > start: %v", err)
	}
	if err := e.inTx(func(tx db.Executor) error {
		return repo.Create(e.ctx(), tx, newClass("basic01", DateRange{Start: sameDay, End: day("2027-01-01")}))
	}); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("UNIQUE code: %v", err)
	}

	// Cập nhật với trạng thái nguồn đã lệch (lớp vừa bị chuyển ở request khác) → ErrInvalidTransition.
	if err := e.inTx(func(tx db.Executor) error {
		c, err := repo.ByIDForUpdate(e.ctx(), tx, basic03ID)
		if err != nil {
			return err
		}
		if err := c.Activate(e.clk.Now()); err != nil {
			return err
		}
		return repo.Update(e.ctx(), tx, c, domain.ClassActive, e.clk.Now())
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("guard trạng thái: %v", err)
	}
	if err := e.inTx(func(tx db.Executor) error {
		c, err := repo.ByID(e.ctx(), tx, basic03ID)
		if err != nil {
			return err
		}
		c.dates = DateRange{Start: sameDay, End: sameDay}
		return repo.Update(e.ctx(), tx, c, domain.ClassDraft, e.clk.Now())
	}); !errors.Is(err, ErrInvalidDates) {
		t.Fatalf("CHECK ngày khi cập nhật: %v", err)
	}
	if _, err := repo.Detail(e.ctx(), e.dbx, uuid.New()); !errors.Is(err, ErrClassNotFound) {
		t.Fatalf("Detail lớp không có: %v", err)
	}
	if _, err := repo.ByID(e.ctx(), e.dbx, uuid.New()); !errors.Is(err, ErrClassNotFound) {
		t.Fatalf("ByID lớp không có: %v", err)
	}
}

func TestClassListFilters(t *testing.T) {
	e := newIT(t)
	repo := PGClassRepo{}
	codes := func(q ListQuery) []string {
		t.Helper()
		rows, err := repo.List(e.ctx(), e.dbx, q)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.Code)
		}
		return out
	}
	active, draft := domain.ClassActive, domain.ClassDraft
	tests := []struct {
		name string
		q    ListQuery
		want string
	}{
		{"tất cả", ListQuery{}, "basic02,basic03,basic01"},
		{"đang chạy", ListQuery{Status: &active}, "basic02,basic01"},
		{"nháp", ListQuery{Status: &draft}, "basic03"},
		{"theo giảng viên", ListQuery{TeacherID: &baoID}, "basic03"},
		{"tìm theo tên", ListQuery{Q: "khóa 2"}, "basic02"},
		{"tìm theo mã, không phân biệt hoa thường", ListQuery{Q: "BASIC01"}, "basic01"},
		{"ký tự đại diện LIKE bị escape", ListQuery{Q: "%"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(codes(tt.q), ","); got != tt.want {
				t.Fatalf("List = %s, muốn %s", got, tt.want)
			}
		})
	}
}

func TestMemberRepoConstraintsAndRows(t *testing.T) {
	e := newIT(t)
	repo := PGMemberRepo{}
	if err := e.inTx(func(tx db.Executor) error {
		return repo.Create(e.ctx(), tx, NewMember(ids.New(), basic01ID, anID, e.clk.Now()))
	}); !errors.Is(err, ErrAlreadyMember) {
		t.Fatalf("UNIQUE (class_id, user_id): %v", err)
	}
	if err := e.inTx(func(tx db.Executor) error {
		m, err := repo.ByIDForUpdate(e.ctx(), tx, basic01ID, memberAnID)
		if err != nil {
			return err
		}
		if err := m.Drop(e.clk.Now()); err != nil {
			return err
		}
		return repo.Update(e.ctx(), tx, m, domain.MemberDropped)
	}); !errors.Is(err, ErrMemberNotDropped) {
		t.Fatalf("guard trạng thái thành viên: %v", err)
	}
	if _, err := repo.ByID(e.ctx(), e.dbx, basic02ID, memberAnID); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("thành viên lớp khác: %v", err)
	}
	if m, err := repo.ByClassAndUser(e.ctx(), e.dbx, basic02ID, anID); m != nil || err != nil {
		t.Fatalf("ByClassAndUser không có = %v, %v", m, err)
	}

	// Lời mời mới nhất quyết định trạng thái; outbox sending hiển thị là queued.
	e.addFailedInvite()
	e.mustInTx(func(tx db.Executor) error {
		rows, err := repo.ListRows(e.ctx(), tx, basic01ID, false)
		if err != nil {
			return err
		}
		if len(rows) != 2 || rows[0].FullName != "Nguyễn Hoàng An" || rows[0].InviteStatus != nil || rows[0].InviteKind != nil {
			return fmt.Errorf("không có lời mời: %+v", rows)
		}
		k := rows[1]
		if k.InviteStatus == nil || *k.InviteStatus != "failed" || *k.InviteAttempts != 3 || *k.InviteLastError != "Mailbox không tồn tại (550 5.1.1)" {
			return fmt.Errorf("lời mời thất bại: %+v", k)
		}
		outboxID := ids.New()
		if _, err := tx.ExecContext(e.ctx(), `INSERT INTO email_outbox (id, to_email, template, status, attempts, created_at)
			VALUES ($1, 'khang.vu@gmail.com', 'added', 'sending', 1, now())`, outboxID); err != nil {
			return err
		}
		if err := (PGInvitationRepo{}).Create(e.ctx(), tx, &Invitation{ID: ids.New(), ClassID: basic01ID, UserID: khangID,
			Kind: InvitationAdded, EmailOutboxID: outboxID, InvitedBy: adminID, CreatedAt: e.clk.Now()}); err != nil {
			return err
		}
		row, err := repo.RowByID(e.ctx(), tx, basic01ID, k.ID)
		if err != nil {
			return err
		}
		if row.InviteStatus == nil || *row.InviteStatus != "queued" || *row.InviteAttempts != 1 || row.InviteLastError != nil {
			return fmt.Errorf("lời mời mới nhất đang gửi: %+v", row)
		}
		return nil
	})
}

func TestMembershipReader(t *testing.T) {
	e := newIT(t)
	r := PGMembershipReader{}
	mc, err := r.MemberContext(e.ctx(), e.dbx, basic01ID, anID)
	if err != nil || !mc.IsActiveMember() || mc.MemberID != memberAnID || mc.ClassStatus != domain.ClassActive ||
		mc.CourseVersionID != basicV1ID || mc.TeacherID != huongID {
		t.Fatalf("thành viên = %+v, %v", mc, err)
	}
	mc, err = r.MemberContext(e.ctx(), e.dbx, basic02ID, anID)
	if err != nil || mc.IsMember() || mc.ClassStatus != domain.ClassActive {
		t.Fatalf("không thuộc lớp = %+v, %v", mc, err)
	}
	if _, err := r.MemberContext(e.ctx(), e.dbx, uuid.New(), anID); !errors.Is(err, ErrClassNotFound) {
		t.Fatalf("lớp không có: %v", err)
	}
	e.exec(`UPDATE class_members SET status = 'dropped', dropped_at = now() WHERE id = $1`, memberAnID)
	mc, err = r.MemberContext(e.ctx(), e.dbx, basic01ID, anID)
	if err != nil || !mc.IsMember() || mc.IsActiveMember() || mc.MemberStatus != domain.MemberDropped {
		t.Fatalf("đã rời lớp = %+v, %v", mc, err)
	}
}

// --- golden JSON qua HTTP handler (dùng lại làm fixture MSW của web) ---

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizeGolden thay thời điểm (khóa kết thúc bằng "At"), ngày (khóa kết thúc bằng "Date") bằng mốc cố định và id
// sinh lúc chạy (không phải id cố định 01990000-…) bằng id giả theo thứ tự xuất hiện, để golden ổn định giữa các lần chạy.
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
			if strings.HasSuffix(key, "Date") {
				return "2026-10-05"
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
	e := newIT(t)
	e.addFailedInvite()
	// Có bài đã hoàn thành để teaching_classes.json có ví dụ avgPercent khác null.
	e.exec(`INSERT INTO lesson_progress (class_member_id, lesson_id, completed_at) VALUES ($1, $2, now())`,
		"01990000-0000-7000-8000-000000000971", lessonID)

	created := e.must(http.MethodPost, "/api/v1/classes", `{"code":"basic04","name":"Lập trình cơ bản 04","courseVersionId":"`+
		basicV1ID.String()+`","startDate":"2026-11-01","endDate":"2027-03-01","teacherId":"`+huongID.String()+`"}`, http.StatusCreated)
	assertGolden(t, "class_created.json", created)
	var d ClassDetailDTO
	if err := json.Unmarshal(created, &d); err != nil {
		t.Fatal(err)
	}
	newClassID := uuid.MustParse(d.ID)
	assertGolden(t, "class_activated.json", e.must(http.MethodPost, classPath(newClassID, "/activate"), "", http.StatusOK))
	assertGolden(t, "class_updated.json", e.must(http.MethodPatch, classPath(basic03ID, ""),
		`{"name":"Lập trình cơ bản – khóa 3 (tối)","courseVersionId":"`+basicV3ID.String()+`"}`, http.StatusOK))
	assertGolden(t, "class_list.json", e.must(http.MethodGet, "/api/v1/classes", "", http.StatusOK))
	assertGolden(t, "class_detail.json", e.must(http.MethodGet, classPath(basic01ID, ""), "", http.StatusOK))

	invited := e.must(http.MethodPost, classPath(basic01ID, "/invitations"), inviteBody(" Moi.Hoc.Vien@Gmail.com ", "Học Viên Mới"), http.StatusCreated)
	assertGolden(t, "invite_invited.json", invited)
	assertGolden(t, "invite_added.json", e.must(http.MethodPost, classPath(basic01ID, "/invitations"), inviteBody("bich.tran@gmail.com", ""), http.StatusCreated))
	assertGolden(t, "invite_added_invited_account.json",
		e.must(http.MethodPost, classPath(basic01ID, "/invitations"), inviteBody("minh.bui@gmail.com", ""), http.StatusCreated))
	e.deliver()
	var inv InviteResponse
	if err := json.Unmarshal(invited, &inv); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "resend.json", e.must(http.MethodPost, classPath(basic01ID, "/members/"+inv.Member.ID+"/resend"), "", http.StatusOK))
	assertGolden(t, "remove_member.json", e.must(http.MethodDelete, classPath(basic01ID, "/members/"+memberAnID.String()), "", http.StatusOK))
	assertGolden(t, "members.json", e.must(http.MethodGet, classPath(basic01ID, "/members"), "", http.StatusOK))
	assertGolden(t, "members_include_dropped.json", e.must(http.MethodGet, classPath(basic01ID, "/members?includeDropped=true"), "", http.StatusOK))

	errorCases := []struct {
		file, method, path, body string
		status                   int
	}{
		{"error_validation.json", http.MethodPost, "/api/v1/classes", `{"code":"basic05","name":"x","courseVersionId":"` + basicV1ID.String() +
			`","startDate":"2026-11-01","endDate":"2026-10-01","teacherId":"` + huongID.String() + `"}`, http.StatusUnprocessableEntity},
		{"error_version_not_published.json", http.MethodPost, "/api/v1/classes", `{"code":"basic05","name":"x","courseVersionId":"` +
			basicV2DraftID.String() + `","startDate":"2026-11-01","endDate":"2027-03-01","teacherId":"` + huongID.String() + `"}`, http.StatusUnprocessableEntity},
		{"error_code_taken.json", http.MethodPost, "/api/v1/classes", `{"code":"basic01","name":"x","courseVersionId":"` + basicV1ID.String() +
			`","startDate":"2026-11-01","endDate":"2027-03-01","teacherId":"` + huongID.String() + `"}`, http.StatusConflict},
		{"error_invalid_transition.json", http.MethodPost, classPath(basic03ID, "/end"), "", http.StatusConflict},
		{"error_class_not_draft.json", http.MethodPatch, classPath(basic01ID, ""), `{"courseVersionId":"` + basicV3ID.String() + `"}`, http.StatusConflict},
		{"error_already_member.json", http.MethodPost, classPath(basic01ID, "/invitations"), inviteBody("bich.tran@gmail.com", ""), http.StatusConflict},
		{"error_internal_email.json", http.MethodPost, classPath(basic01ID, "/invitations"), inviteBody("huong.le@goup.vn", ""), http.StatusForbidden},
		{"error_account_disabled.json", http.MethodPost, classPath(basic01ID, "/invitations"), inviteBody("thao.vo@gmail.com", ""), http.StatusConflict},
		{"error_name_required.json", http.MethodPost, classPath(basic01ID, "/invitations"), inviteBody("moi.nua@gmail.com", ""), http.StatusUnprocessableEntity},
	}
	for _, c := range errorCases {
		assertGolden(t, c.file, e.must(c.method, c.path, c.body, c.status))
	}
	// Đã gửi lại một lần ở trên; dùng hết hạn mức rồi lấy envelope 429.
	resendPath := classPath(basic01ID, "/members/"+inv.Member.ID+"/resend")
	for i := 1; i < resendLimit; i++ {
		e.must(http.MethodPost, resendPath, "", http.StatusOK)
	}
	assertGolden(t, "error_rate_limited.json", e.must(http.MethodPost, resendPath, "", http.StatusTooManyRequests))
	bichMember := e.str(`SELECT id::text FROM class_members WHERE class_id = $1 AND user_id = $2`, basic01ID, bichID)
	assertGolden(t, "error_already_activated.json",
		e.must(http.MethodPost, classPath(basic01ID, "/members/"+bichMember+"/resend"), "", http.StatusConflict))

	e.as(baoID, domain.RoleTeacher)
	assertGolden(t, "error_not_own_class.json", e.must(http.MethodGet, classPath(basic01ID, ""), "", http.StatusForbidden))
	e.as(huongID, domain.RoleTeacher)
	assertGolden(t, "teaching_classes.json", e.must(http.MethodGet, "/api/v1/teach/classes", "", http.StatusOK))
	e.assertNoSecretLeak()
}
