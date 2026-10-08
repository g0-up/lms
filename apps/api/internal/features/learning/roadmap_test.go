package learning

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
)

// testStructure là hai chặng: A có 3 bài bắt buộc và 1 bài tùy chọn, B có 2 bài bắt buộc; thêm chặng C chưa có bài.
func testStructure() (structure []StageView, a, b [4]uuid.UUID) {
	lesson := func(id uuid.UUID, title string, pos int, required bool) LessonView {
		return LessonView{ID: id, Title: title, Type: stages.LessonMarkdown, Position: pos, Required: required, State: StateNotOpened}
	}
	for i := range a {
		a[i], b[i] = uuid.New(), uuid.New()
	}
	structure = []StageView{
		{StageID: uuid.New(), Name: "Database", Position: 1, Lessons: []LessonView{
			lesson(a[0], "A1", 1, true), lesson(a[1], "A2", 2, true), lesson(a[2], "A3", 3, true), lesson(a[3], "A4 tùy chọn", 4, false),
		}},
		{StageID: uuid.New(), Name: "Data structure", Position: 2, Lessons: []LessonView{
			lesson(b[0], "B1", 1, true), lesson(b[1], "B2", 2, true),
		}},
		{StageID: uuid.New(), Name: "Chặng trống", Position: 3, Lessons: []LessonView{}},
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
	if structure[0].Lessons[0].State != StateNotOpened || structure[0].Percent != 0 {
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

func TestLocatePrevNextAcrossStages(t *testing.T) {
	structure, a, b := testStructure()
	r := BuildRoadmap(activeClass(), structure, nil)

	l, st, prev, next, ok := r.Locate(a[3])
	if !ok || l.ID != a[3] || st.Name != "Database" || prev == nil || prev.LessonID != a[2] || next == nil || next.LessonID != b[0] {
		t.Fatalf("Locate(A4) = %v %v %+v %+v %v", l.ID, st.Name, prev, next, ok)
	}
	if _, _, prev, next, ok := r.Locate(a[0]); !ok || prev != nil || next == nil || next.LessonID != a[1] {
		t.Fatalf("Locate(A1): prev=%+v next=%+v", prev, next)
	}
	if _, _, prev, next, ok := r.Locate(b[1]); !ok || prev == nil || prev.LessonID != b[0] || next != nil {
		t.Fatalf("Locate(B2): prev=%+v next=%+v", prev, next)
	}
	if _, _, _, _, ok := r.Locate(uuid.New()); ok {
		t.Fatal("Locate học liệu lạ phải ok=false")
	}
}
