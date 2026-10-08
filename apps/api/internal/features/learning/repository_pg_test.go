//go:build integration

package learning

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
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
	"lms/api/internal/platform/db"
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

func TestProgressRepoOpenAndSetCompleted(t *testing.T) {
	e := newIT(t)
	repo := PGProgressRepo{}
	now := e.clk.Now()
	err := e.inTx(func(tx db.Executor) error {
		inserted, err := repo.Open(e.ctx(), tx, OpenLesson(memberAnID, lessonTableID, now))
		if err != nil || !inserted {
			t.Fatalf("Open lần đầu = %v, %v", inserted, err)
		}
		inserted, err = repo.Open(e.ctx(), tx, OpenLesson(memberAnID, lessonTableID, now.Add(time.Hour)))
		if err != nil || inserted {
			t.Fatalf("Open lần hai = %v, %v", inserted, err)
		}
		p, err := repo.GetForUpdate(e.ctx(), tx, memberAnID, lessonTableID)
		if err != nil || p == nil || !p.FirstOpenedAt().Equal(now) || p.CompletedAt() != nil {
			t.Fatalf("GetForUpdate = %+v, %v", p, err)
		}

		// Bản đọc cũ (chưa tích) ghi sau khi bản khác đã tích → xung đột thay vì ghi đè.
		stale, err := repo.Get(e.ctx(), tx, memberAnID, lessonTableID)
		if err != nil {
			return err
		}
		done := now.Add(time.Minute)
		p.SetCompleted(true, done)
		if err := repo.SetCompleted(e.ctx(), tx, p, done); err != nil {
			t.Fatalf("SetCompleted: %v", err)
		}
		stale.SetCompleted(true, done.Add(time.Minute))
		if err := repo.SetCompleted(e.ctx(), tx, stale, done.Add(time.Minute)); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("ghi từ bản đọc cũ = %v, muốn Conflict", err)
		}
		byMember, err := repo.ByMember(e.ctx(), tx, memberAnID)
		if err != nil || len(byMember) != 1 || byMember[lessonTableID].CompletedAt() == nil ||
			!byMember[lessonTableID].CompletedAt().Equal(done) {
			t.Fatalf("ByMember = %+v, %v", byMember, err)
		}
		missing, err := repo.Get(e.ctx(), tx, memberAnID, lessonIndexID)
		if err != nil || missing != nil {
			t.Fatalf("Get chưa mở = %+v, %v", missing, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCourseStructureRepo(t *testing.T) {
	e := newIT(t)
	repo := PGCourseStructureRepo{}
	structure, err := repo.StagesOfCourseVersion(e.ctx(), e.dbx, basicV1ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, st := range structure {
		keys := make([]string, 0, len(st.Lessons))
		for _, l := range st.Lessons {
			keys = append(keys, string(l.Key))
		}
		got = append(got, fmt.Sprintf("%s:%s", st.Code, strings.Join(keys, ",")))
	}
	if want := "DB:db-intro,db-table,db-index DS:ds-array,ds-stack,ds-hash"; strings.Join(got, " ") != want {
		t.Fatalf("cấu trúc = %v, muốn %s", got, want)
	}
	if structure[0].StageID != stageDBID || structure[1].Name != "Data structure" || structure[1].Position != 2 {
		t.Fatalf("chặng = %+v", structure)
	}

	video, err := repo.LessonInCourseVersion(e.ctx(), e.dbx, basicV1ID, lessonIntroID)
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := video.Content.(stages.VideoContent); !ok || c.MediaID != mediaIntroID {
		t.Fatalf("nội dung video = %#v", video.Content)
	}
	md, err := repo.LessonInCourseVersion(e.ctx(), e.dbx, basicV1ID, lessonStackID)
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := md.Content.(stages.MarkdownContent); !ok || c.HTML == nil || !strings.Contains(*c.HTML, "<h1>Stack và Queue</h1>") {
		t.Fatalf("nội dung markdown = %#v", md.Content)
	}
	if other, err := repo.LessonInCourseVersion(e.ctx(), e.dbx, basicV1ID, lessonV2ID); err != nil || other != nil {
		t.Fatalf("học liệu của phiên bản chặng khác = %+v, %v", other, err)
	}
}

// seedProgress tạo tiến độ cho basic01 theo seed.js: An xong video mở đầu; Khang xong chặng Database và 2/3 chặng
// Data structure (kèm một học liệu tùy chọn); Thảo đã rời lớp sau khi xong một bài; Dũng chưa học.
func (e *itEnv) seedProgress() {
	e.t.Helper()
	day := 24 * time.Hour
	at := func(d time.Duration) time.Time { return e.clk.Now().Add(-d) }
	e.progress(memberAnID, lessonIntroID, at(20*day), ptr(at(20*day-20*time.Minute)))
	e.progress(memberAnID, lessonTableID, at(3*day), nil)
	e.progress(memberKhangID, lessonIntroID, at(45*day), ptr(at(45*day)))
	e.progress(memberKhangID, lessonTableID, at(44*day), ptr(at(44*day)))
	e.progress(memberKhangID, lessonIndexID, at(44*day), ptr(at(44*day)))
	e.progress(memberKhangID, lessonArrayID, at(30*day), ptr(at(30*day)))
	e.progress(memberKhangID, lessonStackID, at(25*day), ptr(at(22*day)))
	e.progress(memberKhangID, lessonHashID, at(22*day), nil)
	e.progress(memberThaoID, lessonArrayID, at(20*day), ptr(at(20*day)))
}

func TestProgressReaderMatchesDomainPercent(t *testing.T) {
	e := newIT(t)
	e.seedProgress()
	reader := PGProgressReader{}

	members, err := reader.ClassProgress(e.ctx(), e.dbx, basic01ID)
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		id               uuid.UUID
		status           domain.MemberStatus
		percent, done    int
		db, ds           int
		lastActivityDays int // -1: chưa có hoạt động
	}
	wants := []want{
		{memberAnID, domain.MemberActive, 20, 1, 50, 0, 3},
		{memberKhangID, domain.MemberActive, 80, 4, 100, 67, 22},
		{memberThaoID, domain.MemberDropped, 20, 1, 0, 33, 20},
		{memberDungID, domain.MemberActive, 0, 0, 0, 0, -1},
	}
	if len(members) != len(wants) {
		t.Fatalf("ClassProgress trả %d thành viên, muốn %d", len(members), len(wants))
	}
	for i, w := range wants {
		m := members[i]
		if m.MemberID != w.id || m.MemberStatus != w.status || m.Percent != w.percent || m.RequiredDone != w.done ||
			m.RequiredTotal != 5 || len(m.Stages) != 2 || m.Stages[0].Percent != w.db || m.Stages[1].Percent != w.ds {
			t.Fatalf("thành viên %d = %+v, muốn %+v", i, m, w)
		}
		if w.lastActivityDays < 0 {
			if m.LastActivityAt != nil {
				t.Fatalf("thành viên %d chưa học mà có LastActivityAt %v", i, m.LastActivityAt)
			}
		} else if m.LastActivityAt == nil || !m.LastActivityAt.Equal(e.clk.Now().Add(-time.Duration(w.lastActivityDays)*24*time.Hour)) {
			t.Fatalf("thành viên %d LastActivityAt = %v", i, m.LastActivityAt)
		}
		if m.Percent != domain.Percent(m.RequiredDone, m.RequiredTotal) {
			t.Fatalf("thành viên %d: SQL %d khác domain.Percent", i, m.Percent)
		}

		// Cùng dữ liệu qua BuildRoadmap phải ra đúng các con số SQL.
		r, err := reader.MemberLessonProgress(e.ctx(), e.dbx, basic01ID, m.MemberID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Percent != m.Percent || r.RequiredDone != m.RequiredDone || r.RequiredTotal != m.RequiredTotal {
			t.Fatalf("thành viên %d: BuildRoadmap %d (%d/%d) khác SQL %d", i, r.Percent, r.RequiredDone, r.RequiredTotal, m.Percent)
		}
		for j, st := range r.Stages {
			if st.StageID != m.Stages[j].StageID || st.Percent != m.Stages[j].Percent || st.RequiredDone != m.Stages[j].RequiredDone {
				t.Fatalf("thành viên %d chặng %d: BuildRoadmap %+v khác SQL %+v", i, j, st, m.Stages[j])
			}
		}
		if (r.LastActivityAt == nil) != (m.LastActivityAt == nil) || (r.LastActivityAt != nil && !r.LastActivityAt.Equal(*m.LastActivityAt)) {
			t.Fatalf("thành viên %d LastActivityAt: BuildRoadmap %v khác SQL %v", i, r.LastActivityAt, m.LastActivityAt)
		}
	}

	if _, err := reader.MemberLessonProgress(e.ctx(), e.dbx, basic02ID, memberAnID); !errors.Is(err, domain.ErrNotFound) ||
		err.Error() != ErrMemberNotFound.Error() {
		t.Fatalf("thành viên của lớp khác = %v", err)
	}

	// /me/classes dùng câu SQL riêng: phải khớp cùng con số.
	mine, err := PGMyClassesRepo{}.ListForStudent(e.ctx(), e.dbx, anID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range mine {
		if row.Class.ID == basic01ID && (row.Percent != 20 || row.RequiredDone != 1 || row.NextLesson == nil ||
			row.NextLesson.LessonID != lessonTableID || row.NextLesson.StageName != "Database") {
			t.Fatalf("ListForStudent basic01 = %+v next %+v", row, row.NextLesson)
		}
	}

	ids := []uuid.UUID{basic01ID, basic02ID, basic03ID, basic00ID}
	avg, err := reader.ClassAveragePercent(e.ctx(), e.dbx, ids)
	if err != nil {
		t.Fatal(err)
	}
	// basic01: round(avg(20, 80, 0)) = 33, không tính Thảo đã rời; basic02 không còn thành viên active nên vắng.
	if len(avg) != 3 || avg[basic01ID] != 33 || avg[basic03ID] != 0 || avg[basic00ID] != 20 {
		t.Fatalf("ClassAveragePercent = %v", avg)
	}

	counts, err := reader.ClassActivityCounts(e.ctx(), e.dbx, ids, e.clk.Now().Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// basic01: Dũng chưa đăng nhập; Khang (21 ngày) và Dũng (chưa từng) không hoạt động. Lớp nháp/kết thúc không tính
	// không hoạt động.
	wantCounts := map[uuid.UUID]ActivityCounts{
		basic01ID: {NotLoggedIn: 1, Inactive: 2}, basic02ID: {}, basic03ID: {}, basic00ID: {},
	}
	if len(counts) != len(wantCounts) {
		t.Fatalf("ClassActivityCounts = %v", counts)
	}
	for id, w := range wantCounts {
		if counts[id] != w {
			t.Fatalf("ClassActivityCounts[%s] = %+v, muốn %+v", id, counts[id], w)
		}
	}
	if empty, err := reader.ClassAveragePercent(e.ctx(), e.dbx, nil); err != nil || len(empty) != 0 {
		t.Fatalf("ClassAveragePercent rỗng = %v, %v", empty, err)
	}
}

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

// TestGoldenJSON ghi phản hồi 4 route và các lỗi học viên gặp; web học viên dùng các file này làm fixture.
func TestGoldenJSON(t *testing.T) {
	e := newIT(t)
	e.seedProgress()
	e.login("an.nguyen@gmail.com")

	assertGolden(t, "my-classes.json", e.must(http.MethodGet, "/api/v1/me/classes", "", http.StatusOK))
	assertGolden(t, "my-class-basic01.json", e.must(http.MethodGet, classURL(basic01ID), "", http.StatusOK))
	assertGolden(t, "my-class-draft.json", e.must(http.MethodGet, classURL(basic03ID), "", http.StatusOK))
	assertGolden(t, "lesson-video.json", e.must(http.MethodGet, lessonURL(basic01ID, lessonIntroID), "", http.StatusOK))
	assertGolden(t, "lesson-markdown.json", e.must(http.MethodGet, lessonURL(basic01ID, lessonTableID), "", http.StatusOK))
	assertGolden(t, "lesson-ended.json", e.must(http.MethodGet, lessonURL(basic00ID, lessonIndexID), "", http.StatusOK))
	assertGolden(t, "completion.json", e.must(http.MethodPut, completionURL(basic01ID, lessonTableID), tick, http.StatusOK))
	assertGolden(t, "completion-unticked.json", e.must(http.MethodPut, completionURL(basic01ID, lessonTableID), untick, http.StatusOK))

	errorCase := func(name, method, path, body string, status int) {
		t.Helper()
		r := e.call(method, path, body)
		if r.code != status {
			t.Fatalf("%s %s = %d, muốn %d: %s", method, path, r.code, status, r.body)
		}
		assertGolden(t, name, r.body)
	}
	errorCase("error-not-member.json", http.MethodGet, classURL(basic02ID), "", http.StatusNotFound)
	errorCase("error-lesson-not-in-course.json", http.MethodGet, lessonURL(basic01ID, lessonV2ID), "", http.StatusNotFound)
	errorCase("error-not-opened.json", http.MethodPut, completionURL(basic01ID, lessonHashID), tick, http.StatusConflict)
	errorCase("error-class-not-active.json", http.MethodPut, completionURL(basic00ID, lessonTableID), tick, http.StatusConflict)
	errorCase("error-class-not-started.json", http.MethodGet, lessonURL(basic03ID, lessonTableID), "", http.StatusConflict)
}
