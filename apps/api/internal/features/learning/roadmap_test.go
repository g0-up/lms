package learning

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
	"lms/api/internal/features/summary"
)

// testStage dựng chặng từ các bài học như repository: học liệu mang UnitID, Lessons phẳng theo thứ tự bài học với
// position 1..n.
func testStage(name string, pos int, units ...UnitView) StageView {
	st := StageView{StageID: uuid.New(), Name: name, Position: pos, Units: []UnitView{}, Lessons: []LessonView{}}
	for i, u := range units {
		u.Position = i + 1
		for j := range u.Lessons {
			u.Lessons[j].UnitID, u.Lessons[j].Position = u.ID, len(st.Lessons)+1
			st.Lessons = append(st.Lessons, u.Lessons[j])
		}
		st.Units = append(st.Units, u)
	}
	return st
}

func testUnit(title string, lessons ...LessonView) UnitView {
	return UnitView{ID: uuid.New(), Title: title, Lessons: lessons}
}

func testLesson(id uuid.UUID, title string, required bool) LessonView {
	return LessonView{ID: id, Title: title, Type: stages.LessonMarkdown, Required: required, State: StateNotOpened}
}

// testStructure là ba chặng:
//   - A (Database): bài học "Nhập môn" có A1, A2 bắt buộc; "Thiết kế" có A3 bắt buộc; "Đọc thêm" chỉ có A4 tùy chọn.
//   - B (Data structure): bài học "Cơ bản" có B1, B2 bắt buộc.
//   - C chưa có bài học.
func testStructure() (structure []StageView, a, b [4]uuid.UUID) {
	for i := range a {
		a[i], b[i] = uuid.New(), uuid.New()
	}
	structure = []StageView{
		testStage("Database", 1,
			testUnit("Nhập môn", testLesson(a[0], "A1", true), testLesson(a[1], "A2", true)),
			testUnit("Thiết kế", testLesson(a[2], "A3", true)),
			testUnit("Đọc thêm", testLesson(a[3], "A4 tùy chọn", false)),
		),
		testStage("Data structure", 2, testUnit("Cơ bản", testLesson(b[0], "B1", true), testLesson(b[1], "B2", true))),
		testStage("Chặng trống", 3),
	}
	return structure, a, b
}

func completedAt(id uuid.UUID, opened, done time.Time) *LessonProgress {
	return RestoreProgress(uuid.Nil, id, opened, &done)
}

func activeClass() ClassSummary { return ClassSummary{ID: uuid.New(), Status: domain.ClassActive} }

func TestBuildRoadmapPercentByRequiredOnly(t *testing.T) {
	structure, a, _ := testStructure()
	progress := map[uuid.UUID]*LessonProgress{
		a[0]: completedAt(a[0], testNow, testNow.Add(time.Minute)),
		a[1]: completedAt(a[1], testNow, testNow.Add(2*time.Minute)),
		a[2]: OpenLesson(uuid.Nil, a[2], testNow.Add(3*time.Minute)),
		a[3]: completedAt(a[3], testNow, testNow.Add(10*time.Minute)), // tùy chọn: không đổi %
	}
	r := BuildRoadmap(activeClass(), structure, progress)

	if got := []int{r.Stages[0].Percent, r.Stages[1].Percent, r.Stages[2].Percent}; got[0] != 67 || got[1] != 0 || got[2] != 0 {
		t.Fatalf("percent chặng = %v, muốn [67 0 0]", got)
	}
	if r.Percent != 40 || r.RequiredDone != 2 || r.RequiredTotal != 5 {
		t.Fatalf("tổng = %d%% (%d/%d), muốn 40%% (2/5)", r.Percent, r.RequiredDone, r.RequiredTotal)
	}
	if r.NextLesson == nil || r.NextLesson.LessonID != a[2] || r.NextLesson.StageName != "Database" {
		t.Fatalf("NextLesson = %+v, muốn A3", r.NextLesson)
	}
	if got := r.Stages[0].Lessons; got[0].State != StateCompleted || got[2].State != StateOpened || r.Stages[1].Lessons[0].State != StateNotOpened {
		t.Fatalf("trạng thái học liệu sai: %+v", got)
	}
	if r.LastActivityAt == nil || !r.LastActivityAt.Equal(testNow.Add(10*time.Minute)) {
		t.Fatalf("LastActivityAt = %v", r.LastActivityAt)
	}
	if r.ReadOnly || r.ReadOnlyReason != "" || !r.SelfReported {
		t.Fatalf("lớp active: ReadOnly=%v reason=%q selfReported=%v", r.ReadOnly, r.ReadOnlyReason, r.SelfReported)
	}
	if structure[0].Lessons[0].State != StateNotOpened || structure[0].Percent != 0 ||
		structure[0].Units[0].Lessons[0].State != StateNotOpened || structure[0].Units[0].RequiredDone != 0 {
		t.Fatal("BuildRoadmap không được sửa cấu trúc đầu vào")
	}

	delete(progress, a[3])
	if again := BuildRoadmap(activeClass(), structure, progress); again.Percent != r.Percent {
		t.Fatalf("bỏ hoàn thành bài tùy chọn đổi %%: %d → %d", r.Percent, again.Percent)
	}
}

func TestBuildRoadmapNextLessonIncludesOptional(t *testing.T) {
	structure, a, b := testStructure()
	progress := map[uuid.UUID]*LessonProgress{}
	for _, id := range a[:3] {
		progress[id] = completedAt(id, testNow, testNow)
	}
	r := BuildRoadmap(activeClass(), structure, progress)
	if r.NextLesson == nil || r.NextLesson.LessonID != a[3] {
		t.Fatalf("NextLesson = %+v, muốn bài tùy chọn A4", r.NextLesson)
	}
	progress[a[3]] = completedAt(a[3], testNow, testNow)
	if r := BuildRoadmap(activeClass(), structure, progress); r.NextLesson == nil || r.NextLesson.LessonID != b[0] ||
		r.NextLesson.StageName != "Data structure" {
		t.Fatalf("NextLesson = %+v, muốn B1", r.NextLesson)
	}
	progress[b[0]] = completedAt(b[0], testNow, testNow)
	progress[b[1]] = completedAt(b[1], testNow, testNow)
	if r := BuildRoadmap(activeClass(), structure, progress); r.NextLesson != nil || r.Percent != 100 {
		t.Fatalf("hoàn thành hết: NextLesson=%+v percent=%d", r.NextLesson, r.Percent)
	}
}

func TestBuildRoadmapRounding(t *testing.T) {
	structure, a, b := testStructure()
	progress := map[uuid.UUID]*LessonProgress{a[0]: completedAt(a[0], testNow, testNow), b[0]: completedAt(b[0], testNow, testNow)}
	r := BuildRoadmap(activeClass(), structure, progress)
	if r.Stages[0].Percent != 33 || r.Stages[1].Percent != 50 || r.Percent != 40 {
		t.Fatalf("percent = %d/%d tổng %d, muốn 33/50 tổng 40", r.Stages[0].Percent, r.Stages[1].Percent, r.Percent)
	}
}

func TestBuildRoadmapWithoutRequiredLessonsIsZero(t *testing.T) {
	r := BuildRoadmap(activeClass(), nil, nil)
	if r.Percent != 0 || r.RequiredTotal != 0 || r.NextLesson != nil || len(r.Stages) != 0 {
		t.Fatalf("lộ trình rỗng: %+v", r)
	}
}

func TestBuildRoadmapReadOnlyClasses(t *testing.T) {
	structure, _, _ := testStructure()
	ended := BuildRoadmap(ClassSummary{Status: domain.ClassEnded}, structure, nil)
	if !ended.ReadOnly || ended.ReadOnlyReason != ReadOnlyEnded || ended.NextLesson == nil {
		t.Fatalf("lớp ended: ReadOnly=%v reason=%q next=%+v", ended.ReadOnly, ended.ReadOnlyReason, ended.NextLesson)
	}
	draft := BuildRoadmap(ClassSummary{Status: domain.ClassDraft}, structure, nil)
	if !draft.ReadOnly || draft.ReadOnlyReason != ReadOnlyDraft || draft.NextLesson != nil {
		t.Fatalf("lớp draft: ReadOnly=%v reason=%q next=%+v", draft.ReadOnly, draft.ReadOnlyReason, draft.NextLesson)
	}
}

func TestBuildRoadmapUnitProgress(t *testing.T) {
	structure, a, _ := testStructure()
	progress := map[uuid.UUID]*LessonProgress{
		a[0]: completedAt(a[0], testNow, testNow),
		a[1]: completedAt(a[1], testNow, testNow),
		a[2]: OpenLesson(uuid.Nil, a[2], testNow),
	}
	r := BuildRoadmap(activeClass(), structure, progress)
	type unitState struct {
		done, total      int
		complete, option bool
		lessons          int
	}
	got := []unitState{}
	for _, u := range r.Stages[0].Units {
		got = append(got, unitState{u.RequiredDone, u.RequiredTotal, u.Complete, u.Optional, len(u.Lessons)})
	}
	want := []unitState{{2, 2, true, false, 2}, {0, 1, false, false, 1}, {0, 0, false, true, 1}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("bài học chặng A = %v, muốn %v", got, want)
	}
	if u := r.Stages[0].Units[0]; u.Lessons[0].State != StateCompleted || u.Lessons[1].State != StateCompleted {
		t.Fatal("học liệu trong bài học phải mang trạng thái tiến độ")
	}
	if u := r.Stages[0].Units[1]; u.Lessons[0].State != StateOpened {
		t.Fatalf("A3 = %s, muốn opened", u.Lessons[0].State)
	}
	// Bài học tùy chọn không bao giờ "Hoàn thành" kể cả khi đã xong học liệu của nó.
	progress[a[3]] = completedAt(a[3], testNow, testNow)
	if u := BuildRoadmap(activeClass(), structure, progress).Stages[0].Units[2]; u.Complete || !u.Optional {
		t.Fatalf("bài học tùy chọn: complete=%v optional=%v", u.Complete, u.Optional)
	}
	if st := r.Stages[2]; len(st.Units) != 0 || len(st.Lessons) != 0 {
		t.Fatalf("chặng trống = %+v", st)
	}
}

func TestLocatePrevNextAcrossStages(t *testing.T) {
	structure, a, b := testStructure()
	r := BuildRoadmap(activeClass(), structure, nil)

	loc, ok := r.Locate(a[3])
	if !ok || loc.Lesson.ID != a[3] || loc.Stage.Name != "Database" || loc.Unit.Title != "Đọc thêm" || loc.Prev == nil ||
		loc.Prev.LessonID != a[2] || loc.Next == nil || loc.Next.LessonID != b[0] || loc.Next.StageName != "Data structure" {
		t.Fatalf("Locate(A4) = %+v %v", loc, ok)
	}
	// Học liệu cuối chặng dẫn sang chặng sau: qua cả ranh giới bài học.
	if !loc.CrossesUnit || loc.NextUnitTitle != "Cơ bản" {
		t.Fatalf("Locate(A4) crossesUnit=%v nextUnitTitle=%q", loc.CrossesUnit, loc.NextUnitTitle)
	}
	if loc, ok := r.Locate(a[0]); !ok || loc.Prev != nil || loc.Next == nil || loc.Next.LessonID != a[1] || loc.CrossesUnit ||
		loc.NextUnitTitle != "" || loc.Unit.Position != 1 {
		t.Fatalf("Locate(A1) = %+v", loc)
	}
	if loc, ok := r.Locate(b[1]); !ok || loc.Prev == nil || loc.Prev.LessonID != b[0] || loc.Next != nil || loc.CrossesUnit {
		t.Fatalf("Locate(B2) = %+v", loc)
	}
	if _, ok := r.Locate(uuid.New()); ok {
		t.Fatal("Locate học liệu lạ phải ok=false")
	}
}

func TestLocateAcrossUnitBoundary(t *testing.T) {
	structure, a, _ := testStructure()
	r := BuildRoadmap(activeClass(), structure, nil)
	loc, ok := r.Locate(a[1])
	if !ok || loc.Unit.Title != "Nhập môn" || loc.Next == nil || loc.Next.LessonID != a[2] || !loc.CrossesUnit ||
		loc.NextUnitTitle != "Thiết kế" {
		t.Fatalf("Locate(A2) = %+v", loc)
	}
	loc, ok = r.Locate(a[2])
	if !ok || loc.Unit.Title != "Thiết kế" || loc.Unit.Position != 2 || loc.Prev == nil || loc.Prev.LessonID != a[1] ||
		loc.Next == nil || loc.Next.LessonID != a[3] || !loc.CrossesUnit || loc.NextUnitTitle != "Đọc thêm" {
		t.Fatalf("Locate(A3) = %+v", loc)
	}
}

// Nhãn trạng thái chỉ gắn cho quiz và homework, ở cả danh sách theo bài học lẫn danh sách phẳng của chặng; video
// giữ dấu tích, phần trăm không đổi và lộ trình gốc không bị sửa.
func TestRoadmapWithStatusesOnlyQuizAndHomework(t *testing.T) {
	video, quiz, hw := uuid.New(), uuid.New(), uuid.New()
	structure := []StageView{testStage("Database", 1, testUnit("Nhập môn",
		LessonView{ID: video, Title: "Video", Type: stages.LessonVideo, Required: true},
		LessonView{ID: quiz, Title: "Quiz", Type: stages.LessonQuiz, Required: true},
		LessonView{ID: hw, Title: "Bài tập", Type: stages.LessonHomework, Required: true},
	))}
	r := BuildRoadmap(activeClass(), structure, map[uuid.UUID]*LessonProgress{video: completedAt(video, testNow, testNow)})
	got := r.WithStatuses(map[uuid.UUID]summary.ItemStatus{
		video: {Done: true, Label: "Đã học xong", Tone: summary.ToneOK},
		quiz:  {Label: "Chưa đạt 60%", Tone: summary.ToneWarn},
		hw:    {Label: "Quá hạn", Tone: summary.ToneDanger, Overdue: true},
	})

	want := map[uuid.UUID]*LessonStatus{
		video: nil, quiz: {Label: "Chưa đạt 60%", Tone: "warn"}, hw: {Label: "Quá hạn", Tone: "danger"},
	}
	for name, lessons := range map[string][]LessonView{"chặng": got.Stages[0].Lessons, "bài học": got.Stages[0].Units[0].Lessons} {
		for _, l := range lessons {
			w := want[l.ID]
			if (w == nil) != (l.Status == nil) || (w != nil && *w != *l.Status) {
				t.Errorf("%s: %s Status = %+v, muốn %+v", name, l.Title, l.Status, w)
			}
		}
	}
	if got.Percent != r.Percent || got.Stages[0].RequiredDone != 1 {
		t.Errorf("Percent = %d, RequiredDone = %d; nhãn không được đổi phần trăm", got.Percent, got.Stages[0].RequiredDone)
	}
	if r.Stages[0].Lessons[1].Status != nil || r.Stages[0].Units[0].Lessons[1].Status != nil {
		t.Error("WithStatuses sửa lộ trình gốc")
	}
}
