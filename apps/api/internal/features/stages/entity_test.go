package stages

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

var testNow = time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)

func intPtr(n int) *int { return &n }

func newDraft(t *testing.T) *StageVersion {
	t.Helper()
	no, err := domain.ParseVersionNo(1)
	if err != nil {
		t.Fatal(err)
	}
	return NewDraftVersion(uuid.New(), uuid.New(), no, "Cơ sở dữ liệu", "", uuid.New())
}

func addUnit(t *testing.T, v *StageVersion, title string) *Unit {
	t.Helper()
	u, err := v.AddUnit(uuid.New(), title)
	if err != nil {
		t.Fatalf("AddUnit: %v", err)
	}
	return u
}

func addMarkdown(t *testing.T, v *StageVersion, u *Unit, title, src string) *Lesson {
	t.Helper()
	l, err := v.AddLesson(uuid.New(), u.ID(), "", title, MarkdownContent{Source: src}, true)
	if err != nil {
		t.Fatalf("AddLesson: %v", err)
	}
	return l
}

// layoutOf mô tả bố cục hiện tại "bài học: học liệu@position, …" để so sánh trong test.
func layoutOf(v *StageVersion) string {
	var parts []string
	for _, u := range v.Units() {
		var ls []string
		for _, l := range v.LessonsOf(u.ID()) {
			ls = append(ls, fmt.Sprintf("%s@%d", l.Title(), l.Position()))
		}
		parts = append(parts, fmt.Sprintf("%d.%s[%s]", u.Position(), u.Title(), strings.Join(ls, " ")))
	}
	return strings.Join(parts, " ")
}

// assertInvariant kiểm bất biến aggregate: bài học đánh số 1..m, học liệu nhóm liền theo thứ tự bài học và position
// phẳng 1..n.
func assertInvariant(t *testing.T, v *StageVersion) {
	t.Helper()
	unitPos := map[uuid.UUID]int{}
	for i, u := range v.Units() {
		if u.Position() != i+1 {
			t.Fatalf("unit %q position = %d, want %d", u.Title(), u.Position(), i+1)
		}
		unitPos[u.ID()] = u.Position()
	}
	last := 0
	for i, l := range v.Lessons() {
		if l.Position() != i+1 {
			t.Fatalf("lesson %q position = %d, want %d", l.Title(), l.Position(), i+1)
		}
		p, ok := unitPos[l.UnitID()]
		if !ok || p < last {
			t.Fatalf("lesson %q out of unit order (unit pos %d after %d)", l.Title(), p, last)
		}
		last = p
	}
}

type upperRenderer struct{}

func (upperRenderer) Render(src string) (string, error) {
	return "<p>" + strings.ToUpper(src) + "</p>", nil
}

// twoOptions là phương án hợp lệ cho câu một đáp án: phương án đầu đúng.
func twoOptions() []Option {
	return []Option{{Body: "Đúng", Correct: true}, {Body: "Sai"}}
}

func singleQuestion(body string) QuestionInput {
	return QuestionInput{Type: QuestionSingle, BodySource: body, Points: 1, Options: twoOptions()}
}

func addQuiz(t *testing.T, v *StageVersion, u *Unit, title string, c QuizContent) *Lesson {
	t.Helper()
	l, err := v.AddLesson(uuid.New(), u.ID(), "", title, c, true)
	if err != nil {
		t.Fatalf("AddLesson quiz: %v", err)
	}
	return l
}

func addQuestion(t *testing.T, v *StageVersion, l *Lesson, in QuestionInput) *Question {
	t.Helper()
	q, err := v.AddQuestion(l.ID(), uuid.New(), in, uuid.New)
	if err != nil {
		t.Fatalf("AddQuestion: %v", err)
	}
	return q
}

func addHomework(t *testing.T, v *StageVersion, u *Unit, title, brief string) *Lesson {
	t.Helper()
	c := NewHomeworkContent()
	c.BriefSource = brief
	c.Attachments = []Attachment{{MediaID: uuid.New(), FileName: "starter.zip", SizeBytes: 10}}
	l, err := v.AddLesson(uuid.New(), u.ID(), "", title, c, true)
	if err != nil {
		t.Fatalf("AddLesson homework: %v", err)
	}
	return l
}

func assessment() QuizContent {
	c, err := NewQuizContent().Configure(QuizSettings{Mode: ptr(QuizAssessment)})
	if err != nil {
		panic(err)
	}
	return c
}

func ptr[T any](v T) *T { return &v }

// publishedVersion: bài "Thiết kế bảng" có markdown và video; bài "Kiểm tra cuối chặng" có quiz đánh giá một câu và
// homework một tiêu chí.
func publishedVersion(t *testing.T) *StageVersion {
	t.Helper()
	v := newDraft(t)
	u := addUnit(t, v, "Thiết kế bảng")
	addMarkdown(t, v, u, "Bảng", "bảng")
	if _, err := v.AddLesson(uuid.New(), u.ID(), "video-index", "Chỉ mục", VideoContent{MediaID: uuid.New(), DurationSeconds: intPtr(300)}, false); err != nil {
		t.Fatal(err)
	}
	final := addUnit(t, v, "Kiểm tra cuối chặng")
	quiz := addQuiz(t, v, final, "Kiểm tra", assessment())
	in := singleQuestion("Khóa chính là gì?")
	in.ExplanationSource = "Định danh duy nhất."
	addQuestion(t, v, quiz, in)
	hw := addHomework(t, v, final, "Bài tập", "Thiết kế lược đồ")
	if _, err := v.AddCriterion(hw.ID(), uuid.New(), "Đủ bảng", 100); err != nil {
		t.Fatal(err)
	}
	if err := v.RenderMarkdown(upperRenderer{}); err != nil {
		t.Fatal(err)
	}
	if err := v.Publish(testNow); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestAddLessonGeneratesUniqueKeyAndPosition(t *testing.T) {
	v := newDraft(t)
	u := addUnit(t, v, "Nhập môn")
	a := addMarkdown(t, v, u, "Giới thiệu", "x")
	b := addMarkdown(t, v, u, "Giới thiệu", "y")
	if a.Key() != "gioi-thieu" || b.Key() != "gioi-thieu-2" {
		t.Fatalf("keys = %q, %q", a.Key(), b.Key())
	}
	if a.Position() != 1 || b.Position() != 2 {
		t.Fatalf("positions = %d, %d", a.Position(), b.Position())
	}
	if _, err := v.AddLesson(uuid.New(), u.ID(), "gioi-thieu", "Khác", MarkdownContent{Source: "z"}, true); !errors.Is(err, ErrLessonKeyTaken) {
		t.Fatalf("explicit duplicate key err = %v", err)
	}
}

func TestAddLessonValidation(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		content LessonContent
		want    error
	}{
		{"tiêu đề trống", "  ", MarkdownContent{Source: "x"}, ErrLessonTitleRequired},
		{"thiếu nội dung", "A", nil, ErrInvalidLessonType},
		{"video không có file", "A", VideoContent{}, ErrVideoRequired},
		{"thời lượng âm", "A", VideoContent{MediaID: uuid.New(), DurationSeconds: intPtr(-1)}, ErrInvalidDuration},
		{"markdown rỗng", "A", MarkdownContent{Source: " \n"}, ErrMarkdownRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newDraft(t)
			u := addUnit(t, v, "Bài")
			if _, err := v.AddLesson(uuid.New(), u.ID(), "", tt.title, tt.content, true); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestMutationsOnPublishedAreImmutable(t *testing.T) {
	v := publishedVersion(t)
	first, second := v.Lessons()[0], v.Lessons()[1]
	unit := v.Units()[0]
	quiz, hw := v.Lessons()[2], v.Lessons()[3]
	question := quiz.Content().(QuizContent).Questions[0]
	criterion := hw.Content().(HomeworkContent).Criteria[0]
	before := layoutOf(v)
	checks := map[string]func() error{
		"AddLesson": func() error {
			_, err := v.AddLesson(uuid.New(), unit.ID(), "", "Mới", MarkdownContent{Source: "x"}, true)
			return err
		},
		"UpdateLesson": func() error { return v.UpdateLesson(first.ID(), "Sửa", MarkdownContent{Source: "x"}, true) },
		"RemoveLesson": func() error { return v.RemoveLesson(first.ID()) },
		"MoveLesson":   func() error { return v.MoveLesson(first.ID(), unit.ID()) },
		"ReorderLessons": func() error {
			return v.ReorderLessons([]UnitLessons{{UnitID: unit.ID(), LessonIDs: []uuid.UUID{second.ID(), first.ID()}}})
		},
		"AddUnit": func() error {
			_, err := v.AddUnit(uuid.New(), "Bài mới")
			return err
		},
		"RenameUnit": func() error { return v.RenameUnit(unit.ID(), "Đổi tên") },
		"RemoveUnit": func() error {
			_, err := v.RemoveUnit(unit.ID())
			return err
		},
		"ReorderUnits":   func() error { return v.ReorderUnits([]uuid.UUID{unit.ID()}) },
		"RenderMarkdown": func() error { return v.RenderMarkdown(upperRenderer{}) },
		"AddQuestion": func() error {
			_, err := v.AddQuestion(quiz.ID(), uuid.New(), singleQuestion("Mới"), uuid.New)
			return err
		},
		"UpdateQuestion": func() error {
			_, err := v.UpdateQuestion(quiz.ID(), question.ID, singleQuestion("Sửa"), uuid.New)
			return err
		},
		"RemoveQuestion":   func() error { return v.RemoveQuestion(quiz.ID(), question.ID) },
		"ReorderQuestions": func() error { return v.ReorderQuestions(quiz.ID(), []uuid.UUID{question.ID}) },
		"UpdateLesson quiz": func() error {
			return v.UpdateLesson(quiz.ID(), "Sửa", NewQuizContent(), true)
		},
		"AddCriterion": func() error {
			_, err := v.AddCriterion(hw.ID(), uuid.New(), "Mới", 10)
			return err
		},
		"UpdateCriterion": func() error {
			_, err := v.UpdateCriterion(hw.ID(), criterion.ID, "Sửa", 10)
			return err
		},
		"RemoveCriterion": func() error { return v.RemoveCriterion(hw.ID(), criterion.ID) },
		"ReorderCriteria": func() error { return v.ReorderCriteria(hw.ID(), []uuid.UUID{criterion.ID}) },
	}
	for name, call := range checks {
		if err := call(); !errors.Is(err, ErrVersionImmutable) {
			t.Errorf("%s err = %v, want ErrVersionImmutable", name, err)
		}
	}
	if got := layoutOf(v); got != before || unit.Title() != "Thiết kế bảng" {
		t.Fatalf("published version changed: %s, want %s", got, before)
	}
	qc, hc := quiz.Content().(QuizContent), hw.Content().(HomeworkContent)
	if len(qc.Questions) != 1 || qc.Questions[0] != question || question.BodySource != "Khóa chính là gì?" {
		t.Fatalf("published questions changed: %+v", qc.Questions)
	}
	if len(hc.Criteria) != 1 || hc.Criteria[0] != criterion || criterion.Description != "Đủ bảng" {
		t.Fatalf("published criteria changed: %+v", hc.Criteria)
	}
}

func TestUnitLifecycle(t *testing.T) {
	v := newDraft(t)
	a := addUnit(t, v, "  Nhập môn SQL ")
	b := addUnit(t, v, "Thiết kế bảng")
	if a.Title() != "Nhập môn SQL" || a.Key() != "nhap-mon-sql" || a.Position() != 1 || b.Position() != 2 {
		t.Fatalf("units = %s key=%q", layoutOf(v), a.Key())
	}
	if err := v.RenameUnit(a.ID(), "Thiết kế bảng"); !errors.Is(err, ErrUnitTitleTaken) {
		t.Fatalf("rename to existing err = %v", err)
	}
	if err := v.RenameUnit(a.ID(), "nhập môn sql"); err != nil {
		t.Fatalf("rename to own title with different case err = %v", err)
	}
	if a.Title() != "nhập môn sql" || a.Key() != "nhap-mon-sql" {
		t.Fatalf("rename changed key or kept title: %q %q", a.Title(), a.Key())
	}
	if err := v.RenameUnit(uuid.New(), "X"); !errors.Is(err, ErrUnitNotFound) {
		t.Fatalf("rename unknown err = %v", err)
	}
	same := addUnit(t, v, "Nhập-môn SQL")
	if same.Key() != "nhap-mon-sql-2" {
		t.Fatalf("unit key collision not suffixed: %q", same.Key())
	}
	if _, err := v.RemoveUnit(uuid.New()); !errors.Is(err, ErrUnitNotFound) {
		t.Fatalf("remove unknown err = %v", err)
	}
	if n, err := v.RemoveUnit(same.ID()); err != nil || n != 0 {
		t.Fatalf("remove empty unit = %d, %v", n, err)
	}
	assertInvariant(t, v)
}

func TestAddUnitValidation(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  error
	}{
		{"tên trống", "   ", ErrUnitTitleRequired},
		{"quá 120 ký tự", strings.Repeat("ă", 121), ErrUnitTitleTooLong},
		{"trùng tên khác hoa thường", "  thiết KẾ bảng ", ErrUnitTitleTaken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newDraft(t)
			addUnit(t, v, "Thiết kế bảng")
			if _, err := v.AddUnit(uuid.New(), tt.title); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if len(v.Units()) != 1 {
				t.Fatal("rejected unit was added")
			}
		})
	}
	v := newDraft(t)
	if _, err := v.AddUnit(uuid.New(), strings.Repeat("ă", 120)); err != nil {
		t.Fatalf("120 runes err = %v", err)
	}
}

func TestAddLessonRequiresUnit(t *testing.T) {
	v := newDraft(t)
	for name, unitID := range map[string]uuid.UUID{"thiếu": uuid.Nil, "lạ": uuid.New()} {
		if _, err := v.AddLesson(uuid.New(), unitID, "", "A", MarkdownContent{Source: "x"}, true); !errors.Is(err, ErrLessonUnitRequired) {
			t.Errorf("%s unit err = %v", name, err)
		}
	}
	if len(v.Lessons()) != 0 {
		t.Fatal("lesson added without unit")
	}
}

func TestAddLessonAppendsToChosenUnit(t *testing.T) {
	v := newDraft(t)
	a, b := addUnit(t, v, "A"), addUnit(t, v, "B")
	addMarkdown(t, v, a, "a1", "x")
	addMarkdown(t, v, b, "b1", "x")
	addMarkdown(t, v, a, "a2", "x")
	if got, want := layoutOf(v), "1.A[a1@1 a2@2] 2.B[b1@3]"; got != want {
		t.Fatalf("layout = %s, want %s", got, want)
	}
	assertInvariant(t, v)
}

func TestRemoveUnitCascadesLessons(t *testing.T) {
	v := newDraft(t)
	a, b, c := addUnit(t, v, "A"), addUnit(t, v, "B"), addUnit(t, v, "C")
	addMarkdown(t, v, a, "a1", "x")
	addMarkdown(t, v, b, "b1", "x")
	addMarkdown(t, v, b, "b2", "x")
	addMarkdown(t, v, c, "c1", "x")
	n, err := v.RemoveUnit(b.ID())
	if err != nil || n != 2 {
		t.Fatalf("RemoveUnit = %d, %v; want 2", n, err)
	}
	if got, want := layoutOf(v), "1.A[a1@1] 2.C[c1@2]"; got != want {
		t.Fatalf("layout = %s, want %s", got, want)
	}
	assertInvariant(t, v)
}

func TestMoveLessonGoesToEndOfNewUnit(t *testing.T) {
	v := newDraft(t)
	a, b := addUnit(t, v, "A"), addUnit(t, v, "B")
	a1 := addMarkdown(t, v, a, "a1", "x")
	addMarkdown(t, v, a, "a2", "x")
	addMarkdown(t, v, b, "b1", "x")
	addMarkdown(t, v, b, "b2", "x")
	if err := v.MoveLesson(a1.ID(), b.ID()); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(v), "1.A[a2@1] 2.B[b1@2 b2@3 a1@4]"; got != want {
		t.Fatalf("layout = %s, want %s", got, want)
	}
	if err := v.MoveLesson(a1.ID(), b.ID()); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(v), "1.A[a2@1] 2.B[b1@2 b2@3 a1@4]"; got != want {
		t.Fatalf("same unit move changed layout: %s", got)
	}
	if err := v.MoveLesson(a1.ID(), uuid.New()); !errors.Is(err, ErrLessonUnitRequired) {
		t.Fatalf("move to unknown unit err = %v", err)
	}
	if err := v.MoveLesson(uuid.New(), a.ID()); !errors.Is(err, ErrLessonNotFound) {
		t.Fatalf("move unknown lesson err = %v", err)
	}
	assertInvariant(t, v)
}

func TestReorderUnitsCarriesLessons(t *testing.T) {
	v := newDraft(t)
	a, b, c := addUnit(t, v, "A"), addUnit(t, v, "B"), addUnit(t, v, "C")
	addMarkdown(t, v, a, "a1", "x")
	addMarkdown(t, v, b, "b1", "x")
	addMarkdown(t, v, c, "c1", "x")
	addMarkdown(t, v, c, "c2", "x")
	for name, ids := range map[string][]uuid.UUID{
		"thiếu": {a.ID(), b.ID()},
		"trùng": {a.ID(), a.ID(), b.ID()},
		"lạ":    {a.ID(), b.ID(), uuid.New()},
	} {
		if err := v.ReorderUnits(ids); !errors.Is(err, ErrReorderMismatch) {
			t.Errorf("%s err = %v", name, err)
		}
	}
	if err := v.ReorderUnits([]uuid.UUID{c.ID(), a.ID(), b.ID()}); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(v), "1.C[c1@1 c2@2] 2.A[a1@3] 3.B[b1@4]"; got != want {
		t.Fatalf("layout = %s, want %s", got, want)
	}
	assertInvariant(t, v)
}

func TestReorderLessonsLayout(t *testing.T) {
	setup := func(t *testing.T) (*StageVersion, *Unit, *Unit, []*Lesson) {
		v := newDraft(t)
		a, b := addUnit(t, v, "A"), addUnit(t, v, "B")
		ls := []*Lesson{addMarkdown(t, v, a, "a1", "x"), addMarkdown(t, v, a, "a2", "x"), addMarkdown(t, v, b, "b1", "x")}
		return v, a, b, ls
	}
	t.Run("bố cục sai", func(t *testing.T) {
		v, a, b, ls := setup(t)
		before := layoutOf(v)
		cases := map[string][]UnitLessons{
			"thiếu học liệu": {{a.ID(), []uuid.UUID{ls[0].ID()}}, {b.ID(), []uuid.UUID{ls[2].ID()}}},
			"thừa học liệu":  {{a.ID(), []uuid.UUID{ls[0].ID(), ls[1].ID(), uuid.New()}}, {b.ID(), []uuid.UUID{ls[2].ID()}}},
			"học liệu lặp":   {{a.ID(), []uuid.UUID{ls[0].ID(), ls[1].ID()}}, {b.ID(), []uuid.UUID{ls[2].ID(), ls[0].ID()}}},
			"bài học lạ":     {{a.ID(), []uuid.UUID{ls[0].ID(), ls[1].ID()}}, {uuid.New(), []uuid.UUID{ls[2].ID()}}},
			"thiếu bài học":  {{a.ID(), []uuid.UUID{ls[0].ID(), ls[1].ID(), ls[2].ID()}}},
			"bài học lặp":    {{a.ID(), []uuid.UUID{ls[0].ID(), ls[1].ID()}}, {a.ID(), []uuid.UUID{ls[2].ID()}}},
		}
		for name, layout := range cases {
			if err := v.ReorderLessons(layout); !errors.Is(err, ErrReorderMismatch) {
				t.Errorf("%s err = %v", name, err)
			}
		}
		if got := layoutOf(v); got != before {
			t.Fatalf("rejected layout changed version: %s", got)
		}
	})
	t.Run("chuyển và sắp lại", func(t *testing.T) {
		v, a, b, ls := setup(t)
		layout := []UnitLessons{{b.ID(), []uuid.UUID{ls[1].ID(), ls[2].ID()}}, {a.ID(), []uuid.UUID{ls[0].ID()}}}
		if err := v.ReorderLessons(layout); err != nil {
			t.Fatal(err)
		}
		if got, want := layoutOf(v), "1.A[a1@1] 2.B[a2@2 b1@3]"; got != want {
			t.Fatalf("layout = %s, want %s", got, want)
		}
		if ls[1].UnitID() != b.ID() {
			t.Fatal("moved lesson kept old unit")
		}
		assertInvariant(t, v)
	})
}

func TestPublishRules(t *testing.T) {
	t.Run("chưa có bài học", func(t *testing.T) {
		err := newDraft(t).Publish(testNow)
		if !errors.Is(err, ErrNoUnits) || err.Error() != "Chặng cần ít nhất một bài học trước khi phát hành." {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("có bài học rỗng", func(t *testing.T) {
		v := newDraft(t)
		addMarkdown(t, v, addUnit(t, v, "Nhập môn"), "A", "x")
		addUnit(t, v, "Kiểm tra cuối chặng")
		if err := v.RenderMarkdown(upperRenderer{}); err != nil {
			t.Fatal(err)
		}
		err := v.Publish(testNow)
		if !errors.Is(err, domain.ErrInvalid) || err.Error() != `Bài học "Kiểm tra cuối chặng" chưa có học liệu.` {
			t.Fatalf("err = %v", err)
		}
		if v.Status() != domain.VersionDraft {
			t.Fatal("blocked publish changed status")
		}
	})
	t.Run("markdown chưa render", func(t *testing.T) {
		v := newDraft(t)
		addMarkdown(t, v, addUnit(t, v, "Bài"), "A", "x")
		if err := v.Publish(testNow); !errors.Is(err, ErrNotRendered) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("thành công", func(t *testing.T) {
		v := publishedVersion(t)
		if v.Status() != domain.VersionPublished || v.PublishedAt() == nil || !v.PublishedAt().Equal(testNow) {
			t.Fatalf("status=%s publishedAt=%v", v.Status(), v.PublishedAt())
		}
		mc := v.Lessons()[0].Content().(MarkdownContent)
		if mc.HTML == nil || *mc.HTML != "<p>BẢNG</p>" {
			t.Fatalf("html = %v", mc.HTML)
		}
	})
	t.Run("publish lần hai", func(t *testing.T) {
		if err := publishedVersion(t).Publish(testNow); !errors.Is(err, ErrPublishNotDraft) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRemoveLessonCompactsPositions(t *testing.T) {
	v := newDraft(t)
	a, b := addUnit(t, v, "A"), addUnit(t, v, "B")
	a1 := addMarkdown(t, v, a, "a1", "x")
	addMarkdown(t, v, a, "a2", "x")
	addMarkdown(t, v, b, "b1", "x")
	if err := v.RemoveLesson(a1.ID()); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(v), "1.A[a2@1] 2.B[b1@2]"; got != want {
		t.Fatalf("layout = %s, want %s", got, want)
	}
	if err := v.RemoveLesson(a1.ID()); !errors.Is(err, ErrLessonNotFound) {
		t.Fatalf("remove twice err = %v", err)
	}
	assertInvariant(t, v)
}

func TestArchive(t *testing.T) {
	if err := newDraft(t).Archive(testNow); !errors.Is(err, ErrArchiveNotPublished) {
		t.Fatalf("draft archive err = %v", err)
	}
	v := publishedVersion(t)
	if err := v.Archive(testNow); err != nil {
		t.Fatal(err)
	}
	if v.Status() != domain.VersionArchived || v.ArchivedAt() == nil {
		t.Fatalf("status=%s archivedAt=%v", v.Status(), v.ArchivedAt())
	}
	if err := v.Archive(testNow); !errors.Is(err, ErrArchiveNotPublished) {
		t.Fatalf("archive twice err = %v", err)
	}
	if err := v.CanDelete(); !errors.Is(err, ErrDeleteNotDraft) {
		t.Fatalf("CanDelete archived err = %v", err)
	}
}

func TestCloneAsDraftKeepsUnitsAndLessons(t *testing.T) {
	if _, err := newDraft(t).CloneAsDraft(uuid.New(), 2, uuid.New(), uuid.New); !errors.Is(err, ErrCloneNotPublished) {
		t.Fatalf("clone draft err = %v", err)
	}
	src := publishedVersion(t)
	c, err := src.CloneAsDraft(uuid.New(), 2, uuid.New(), uuid.New)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status() != domain.VersionDraft || c.VersionNo() != 2 || c.ClonedFromID() == nil || *c.ClonedFromID() != src.ID() {
		t.Fatalf("clone header wrong: status=%s no=%d", c.Status(), c.VersionNo())
	}
	if c.PublishedAt() != nil {
		t.Fatal("clone must not carry publishedAt")
	}
	if got, want := layoutOf(c), layoutOf(src); got != want {
		t.Fatalf("clone layout = %s, want %s", got, want)
	}
	unitMap := map[uuid.UUID]uuid.UUID{}
	for i, su := range src.Units() {
		cu := c.Units()[i]
		if cu.ID() == su.ID() || cu.Key() != su.Key() || cu.Title() != su.Title() || cu.Position() != su.Position() {
			t.Fatalf("unit %d not cloned correctly", i)
		}
		unitMap[su.ID()] = cu.ID()
	}
	for i, sl := range src.Lessons() {
		cl := c.Lessons()[i]
		if cl.ID() == sl.ID() || cl.Key() != sl.Key() || cl.Required() != sl.Required() || cl.Position() != sl.Position() {
			t.Fatalf("lesson %d not cloned correctly", i)
		}
		if cl.UnitID() != unitMap[sl.UnitID()] {
			t.Fatalf("lesson %d points to unit %s, want clone of %s", i, cl.UnitID(), sl.UnitID())
		}
	}
	sv, cv := src.Lessons()[1].Content().(VideoContent), c.Lessons()[1].Content().(VideoContent)
	if cv.MediaID != sv.MediaID || cv.DurationSeconds == nil || *cv.DurationSeconds != 300 {
		t.Fatal("video content not cloned")
	}
	*cv.DurationSeconds = 1
	if *sv.DurationSeconds != 300 {
		t.Fatal("clone shares duration pointer with source")
	}
	if err := c.UpdateLesson(c.Lessons()[0].ID(), "Sửa", MarkdownContent{Source: "mới"}, true); err != nil {
		t.Fatal(err)
	}
	if err := c.RenameUnit(c.Units()[0].ID(), "Đổi tên"); err != nil {
		t.Fatal(err)
	}
	if src.Lessons()[0].Title() != "Bảng" || src.Units()[0].Title() != "Thiết kế bảng" {
		t.Fatal("editing clone changed source")
	}
}

func TestQuizSettingsByMode(t *testing.T) {
	t.Run("mặc định luyện tập, đảo thứ tự bật", func(t *testing.T) {
		c := NewQuizContent()
		if c.Mode != QuizPractice || !c.Shuffle || c.Validate() != nil {
			t.Fatalf("default quiz = %+v", c)
		}
	})
	t.Run("đánh giá lấy mặc định 2 lượt, 20 phút, 70%", func(t *testing.T) {
		c := assessment()
		if *c.MaxAttempts != 2 || *c.TimeLimit != 20*time.Minute || *c.PassPercent != 70 {
			t.Fatalf("assessment defaults = %d %v %d", *c.MaxAttempts, *c.TimeLimit, *c.PassPercent)
		}
	})
	t.Run("về luyện tập bỏ mọi giới hạn", func(t *testing.T) {
		c, err := assessment().Configure(QuizSettings{Mode: ptr(QuizPractice), MaxAttempts: intPtr(5)})
		if err != nil || c.MaxAttempts != nil || c.TimeLimit != nil || c.PassPercent != nil {
			t.Fatalf("practice = %+v, err %v", c, err)
		}
	})
	t.Run("trường không gửi giữ giá trị đang có", func(t *testing.T) {
		c, err := assessment().Configure(QuizSettings{MaxAttempts: intPtr(3)})
		if err != nil {
			t.Fatal(err)
		}
		c, err = c.Configure(QuizSettings{PassPercent: intPtr(80), Shuffle: ptr(false)})
		if err != nil || *c.MaxAttempts != 3 || *c.TimeLimit != 20*time.Minute || *c.PassPercent != 80 || c.Shuffle {
			t.Fatalf("configured = %+v, err %v", c, err)
		}
	})
	invalid := []struct {
		name string
		s    QuizSettings
		want error
	}{
		{"chế độ lạ", QuizSettings{Mode: ptr(QuizMode("exam"))}, ErrInvalidQuizMode},
		{"0 lượt", QuizSettings{Mode: ptr(QuizAssessment), MaxAttempts: intPtr(0)}, ErrQuizAttempts},
		{"11 lượt", QuizSettings{Mode: ptr(QuizAssessment), MaxAttempts: intPtr(11)}, ErrQuizAttempts},
		{"0 phút", QuizSettings{Mode: ptr(QuizAssessment), TimeLimit: ptr(time.Duration(0))}, ErrQuizTimeLimit},
		{"181 phút", QuizSettings{Mode: ptr(QuizAssessment), TimeLimit: ptr(181 * time.Minute)}, ErrQuizTimeLimit},
		{"lẻ giây", QuizSettings{Mode: ptr(QuizAssessment), TimeLimit: ptr(90 * time.Second)}, ErrQuizTimeLimit},
		{"ngưỡng 0", QuizSettings{Mode: ptr(QuizAssessment), PassPercent: intPtr(0)}, ErrPassPercent},
		{"ngưỡng 101", QuizSettings{Mode: ptr(QuizAssessment), PassPercent: intPtr(101)}, ErrPassPercent},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewQuizContent().Configure(tt.s); !errors.Is(err, tt.want) || err.Error() != tt.want.Error() {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
	t.Run("luyện tập mang giới hạn bị từ chối", func(t *testing.T) {
		c := NewQuizContent()
		c.MaxAttempts = intPtr(1)
		if err := c.Validate(); !errors.Is(err, ErrPracticeHasLimits) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestQuestionValidation(t *testing.T) {
	opts := func(correct ...bool) []Option {
		out := make([]Option, len(correct))
		for i, c := range correct {
			out[i] = Option{Body: fmt.Sprintf("Phương án %d", i+1), Correct: c}
		}
		return out
	}
	tests := []struct {
		name string
		in   QuestionInput
		want error
	}{
		{"nội dung trống", QuestionInput{Type: QuestionSingle, BodySource: " ", Points: 1, Options: opts(true, false)}, ErrQuestionBodyRequired},
		{"loại lạ", QuestionInput{Type: "essay", BodySource: "Q", Points: 1, Options: opts(true, false)}, ErrInvalidQuestionType},
		{"điểm 0", QuestionInput{Type: QuestionSingle, BodySource: "Q", Points: 0, Options: opts(true, false)}, ErrQuestionPoints},
		{"điểm 101", QuestionInput{Type: QuestionSingle, BodySource: "Q", Points: 101, Options: opts(true, false)}, ErrQuestionPoints},
		{"một phương án", QuestionInput{Type: QuestionSingle, BodySource: "Q", Points: 1, Options: opts(true)}, ErrTooFewOptions},
		{"bảy phương án", QuestionInput{Type: QuestionMulti, BodySource: "Q", Points: 1, Options: opts(true, false, false, false, false, false, false)}, ErrTooManyOptions},
		{"phương án trống", QuestionInput{Type: QuestionSingle, BodySource: "Q", Points: 1, Options: []Option{{Body: "A", Correct: true}, {Body: "  "}}}, ErrOptionBody},
		{"phương án quá 500 ký tự", QuestionInput{Type: QuestionSingle, BodySource: "Q", Points: 1, Options: []Option{{Body: "A", Correct: true}, {Body: strings.Repeat("ă", 501)}}}, ErrOptionBody},
		{"không có đáp án đúng", QuestionInput{Type: QuestionMulti, BodySource: "Q", Points: 1, Options: opts(false, false)}, ErrNoCorrectOption},
		{"một đáp án mà hai đúng", QuestionInput{Type: QuestionSingle, BodySource: "Q", Points: 1, Options: opts(true, true)}, ErrSingleManyCorrect},
		{"phương án lạ", QuestionInput{Type: QuestionSingle, BodySource: "Q", Points: 1, Options: []Option{{ID: uuid.New(), Body: "A", Correct: true}, {Body: "B"}}}, ErrOptionNotInQuestion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newDraft(t)
			l := addQuiz(t, v, addUnit(t, v, "Bài"), "Quiz", NewQuizContent())
			if _, err := v.AddQuestion(l.ID(), uuid.New(), tt.in, uuid.New); !errors.Is(err, tt.want) || err.Error() != tt.want.Error() {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if n := len(l.Content().(QuizContent).Questions); n != 0 {
				t.Fatalf("rejected question was stored (%d)", n)
			}
		})
	}
	t.Run("hợp lệ: cắt khoảng trắng, nhiều đáp án, 500 ký tự", func(t *testing.T) {
		v := newDraft(t)
		l := addQuiz(t, v, addUnit(t, v, "Bài"), "Quiz", NewQuizContent())
		long := strings.Repeat("ă", 500)
		in := QuestionInput{Type: QuestionMulti, BodySource: "  Q  ", ExplanationSource: " vì ", Points: 3,
			Options: []Option{{Body: " A ", Correct: true}, {Body: long, Correct: true}, {Body: "C"}}}
		q := addQuestion(t, v, l, in)
		if q.BodySource != "Q" || q.ExplanationSource != "vì" || q.Options[0].Body != "A" || q.Options[1].Body != long || q.Key != "cau-hoi" {
			t.Fatalf("question = %+v", q)
		}
		for _, o := range q.Options {
			if o.ID == uuid.Nil {
				t.Fatal("new option got no id")
			}
		}
		if q2 := addQuestion(t, v, l, singleQuestion("Q2")); q2.Key != "cau-hoi-2" {
			t.Fatalf("second key = %q", q2.Key)
		}
		if c := l.Content().(QuizContent); c.MaxScore() != 4 {
			t.Fatalf("max score = %d", c.MaxScore())
		}
	})
}

func TestUpdateQuestionKeepsIdentity(t *testing.T) {
	v := newDraft(t)
	l := addQuiz(t, v, addUnit(t, v, "Bài"), "Quiz", NewQuizContent())
	in := singleQuestion("Q")
	in.Key = "qk-index"
	q := addQuestion(t, v, l, in)
	if _, err := v.AddQuestion(l.ID(), uuid.New(), in, uuid.New); !errors.Is(err, ErrQuestionKeyTaken) {
		t.Fatalf("duplicate key err = %v", err)
	}
	keep := q.Options[0]
	upd := QuestionInput{Type: QuestionMulti, BodySource: "Q mới", Points: 2,
		Options: []Option{{ID: keep.ID, Body: "Giữ", Correct: true}, {Body: "Mới", Correct: true}}}
	got, err := v.UpdateQuestion(l.ID(), q.ID, upd, uuid.New)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != q.ID || got.Key != "qk-index" || got.Options[0].ID != keep.ID || got.Options[1].ID == q.Options[1].ID {
		t.Fatalf("updated = %+v", got)
	}
	if q.BodySource != "Q" {
		t.Fatal("update mutated the previous question in place")
	}
	upd.Key = "khac"
	if _, err := v.UpdateQuestion(l.ID(), q.ID, upd, uuid.New); !errors.Is(err, ErrQuestionKeyImmutable) {
		t.Fatalf("change key err = %v", err)
	}
	dup := singleQuestion("Q")
	dup.Options = []Option{{ID: keep.ID, Body: "A", Correct: true}, {ID: keep.ID, Body: "B"}}
	if _, err := v.UpdateQuestion(l.ID(), q.ID, dup, uuid.New); !errors.Is(err, ErrOptionNotInQuestion) {
		t.Fatalf("reused option id err = %v", err)
	}
	if _, err := v.UpdateQuestion(l.ID(), uuid.New(), singleQuestion("Q"), uuid.New); !errors.Is(err, ErrQuestionNotFound) {
		t.Fatalf("missing question err = %v", err)
	}
	md := addMarkdown(t, v, v.Units()[0], "Đọc", "x")
	if _, err := v.AddQuestion(md.ID(), uuid.New(), singleQuestion("Q"), uuid.New); !errors.Is(err, ErrNotQuiz) {
		t.Fatalf("question on markdown err = %v", err)
	}
}

func TestQuestionRemoveAndReorder(t *testing.T) {
	v := newDraft(t)
	l := addQuiz(t, v, addUnit(t, v, "Bài"), "Quiz", NewQuizContent())
	a, b, c := addQuestion(t, v, l, singleQuestion("A")), addQuestion(t, v, l, singleQuestion("B")), addQuestion(t, v, l, singleQuestion("C"))
	if err := v.ReorderQuestions(l.ID(), []uuid.UUID{c.ID, a.ID}); !errors.Is(err, ErrReorderMismatch) {
		t.Fatalf("partial reorder err = %v", err)
	}
	if err := v.ReorderQuestions(l.ID(), []uuid.UUID{c.ID, a.ID, a.ID}); !errors.Is(err, ErrReorderMismatch) {
		t.Fatalf("duplicate reorder err = %v", err)
	}
	if err := v.ReorderQuestions(l.ID(), []uuid.UUID{c.ID, a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	if err := v.RemoveQuestion(l.ID(), a.ID); err != nil {
		t.Fatal(err)
	}
	if err := v.RemoveQuestion(l.ID(), a.ID); !errors.Is(err, ErrQuestionNotFound) {
		t.Fatalf("remove twice err = %v", err)
	}
	qs := l.Content().(QuizContent).Questions
	if len(qs) != 2 || qs[0] != c || qs[1] != b {
		t.Fatalf("questions = %v", qs)
	}
}

func TestChangeQuizModeKeepsQuestionsOtherTypeDropsThem(t *testing.T) {
	v := newDraft(t)
	l := addQuiz(t, v, addUnit(t, v, "Bài"), "Quiz", NewQuizContent())
	q := addQuestion(t, v, l, singleQuestion("Q"))
	c, err := l.Content().(QuizContent).Configure(QuizSettings{Mode: ptr(QuizAssessment)})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.UpdateLesson(l.ID(), "Kiểm tra", c, true); err != nil {
		t.Fatal(err)
	}
	got := l.Content().(QuizContent)
	if got.Mode != QuizAssessment || len(got.Questions) != 1 || got.Questions[0].ID != q.ID {
		t.Fatalf("assessment lost questions: %+v", got)
	}
	if err := v.UpdateLesson(l.ID(), "Video", VideoContent{MediaID: uuid.New()}, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := l.Content().(QuizContent); ok {
		t.Fatal("video lesson still holds quiz content")
	}
	if _, err := v.AddQuestion(l.ID(), uuid.New(), singleQuestion("Q"), uuid.New); !errors.Is(err, ErrNotQuiz) {
		t.Fatalf("question on video err = %v", err)
	}
}

func TestHomeworkCriteria(t *testing.T) {
	v := newDraft(t)
	l := addHomework(t, v, addUnit(t, v, "Bài"), "Bài tập", "")
	for name, tt := range map[string]struct {
		desc   string
		points int
		want   error
	}{
		"mô tả trống": {" ", 10, ErrCriterionRequired},
		"điểm 0":      {"A", 0, ErrCriterionPoints},
		"điểm 101":    {"A", 101, ErrCriterionPoints},
	} {
		if _, err := v.AddCriterion(l.ID(), uuid.New(), tt.desc, tt.points); !errors.Is(err, tt.want) || err.Error() != tt.want.Error() {
			t.Errorf("%s: err = %v, want %v", name, err, tt.want)
		}
	}
	a, err := v.AddCriterion(l.ID(), uuid.New(), " Stack ", 40)
	if err != nil {
		t.Fatal(err)
	}
	b, err := v.AddCriterion(l.ID(), uuid.New(), "Queue", 60)
	if err != nil {
		t.Fatal(err)
	}
	if a.Description != "Stack" || l.Content().(HomeworkContent).MaxScore() != 100 {
		t.Fatalf("criterion = %+v", a)
	}
	if _, err := v.UpdateCriterion(l.ID(), a.ID, "Stack đủ", 30); err != nil {
		t.Fatal(err)
	}
	if err := v.ReorderCriteria(l.ID(), []uuid.UUID{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	cs := l.Content().(HomeworkContent).Criteria
	if len(cs) != 2 || cs[0].ID != b.ID || cs[1].Description != "Stack đủ" || cs[1].MaxPoints != 30 {
		t.Fatalf("criteria = %+v %+v", cs[0], cs[1])
	}
	if err := v.RemoveCriterion(l.ID(), b.ID); err != nil {
		t.Fatal(err)
	}
	if err := v.RemoveCriterion(l.ID(), b.ID); !errors.Is(err, ErrCriterionNotFound) {
		t.Fatalf("remove twice err = %v", err)
	}
	quiz := addQuiz(t, v, v.Units()[0], "Quiz", NewQuizContent())
	if _, err := v.AddCriterion(quiz.ID(), uuid.New(), "A", 1); !errors.Is(err, ErrNotHomework) {
		t.Fatalf("criterion on quiz err = %v", err)
	}
}

func TestHomeworkSettings(t *testing.T) {
	c := NewHomeworkContent()
	if c.PassPercent != 70 {
		t.Fatalf("default pass = %d", c.PassPercent)
	}
	html := "<p>cũ</p>"
	c.BriefSource, c.BriefHTML = "cũ", &html
	c.Attachments = []Attachment{{MediaID: uuid.New()}}
	same, err := c.Configure(HomeworkSettings{BriefSource: ptr(" cũ "), PassPercent: intPtr(80)})
	if err != nil || same.BriefHTML == nil || same.PassPercent != 80 || len(same.Attachments) != 1 {
		t.Fatalf("same brief = %+v, err %v", same, err)
	}
	changed, err := c.Configure(HomeworkSettings{BriefSource: ptr("mới"), Attachments: []Attachment{}})
	if err != nil || changed.BriefHTML != nil || changed.BriefSource != "mới" || len(changed.Attachments) != 0 {
		t.Fatalf("changed brief = %+v, err %v", changed, err)
	}
	if _, err := c.Configure(HomeworkSettings{PassPercent: intPtr(0)}); !errors.Is(err, ErrPassPercent) {
		t.Fatalf("pass 0 err = %v", err)
	}
	id := uuid.New()
	if _, err := c.Configure(HomeworkSettings{Attachments: []Attachment{{MediaID: id}, {MediaID: id}}}); !errors.Is(err, ErrAttachmentDuplicate) {
		t.Fatalf("duplicate attachment err = %v", err)
	}
	if _, err := c.Configure(HomeworkSettings{Attachments: []Attachment{{}}}); !errors.Is(err, ErrAttachmentNotDocument) {
		t.Fatalf("nil attachment err = %v", err)
	}
}

func TestPublishBlocksIncompleteQuizAndHomework(t *testing.T) {
	setup := func(t *testing.T) (*StageVersion, *Unit) {
		v := newDraft(t)
		return v, addUnit(t, v, "Kiểm tra cuối chặng")
	}
	publish := func(t *testing.T, v *StageVersion, want string) {
		t.Helper()
		if err := v.RenderMarkdown(upperRenderer{}); err != nil {
			t.Fatal(err)
		}
		err := v.Publish(testNow)
		if !errors.Is(err, domain.ErrInvalid) || err.Error() != want {
			t.Fatalf("err = %v, want %q", err, want)
		}
		if v.Status() != domain.VersionDraft {
			t.Fatal("blocked publish changed status")
		}
	}
	t.Run("quiz chưa có câu hỏi", func(t *testing.T) {
		v, u := setup(t)
		addQuiz(t, v, u, "Kiểm tra", assessment())
		publish(t, v, `"Kiểm tra" chưa có câu hỏi.`)
	})
	t.Run("câu chưa đánh dấu đáp án đúng", func(t *testing.T) {
		// Lưu câu hỏi luôn đòi đáp án đúng; dữ liệu cũ thiếu đáp án vẫn phải bị chặn khi phát hành.
		v, u := setup(t)
		c := assessment()
		noAnswer := func(key domain.LessonKey) *Question {
			return &Question{ID: uuid.New(), Key: key, Type: QuestionSingle, BodySource: "Q", Points: 1,
				Options: []Option{{ID: uuid.New(), Body: "A"}, {ID: uuid.New(), Body: "B"}}}
		}
		c.Questions = []*Question{noAnswer("q1"), noAnswer("q2")}
		addQuiz(t, v, u, "Kiểm tra", c)
		publish(t, v, `"Kiểm tra" có 2 câu chưa đánh dấu đáp án đúng.`)
	})
	t.Run("homework chưa có đề bài", func(t *testing.T) {
		v, u := setup(t)
		l := addHomework(t, v, u, "Bài tập", "  ")
		if _, err := v.AddCriterion(l.ID(), uuid.New(), "A", 10); err != nil {
			t.Fatal(err)
		}
		publish(t, v, `"Bài tập" chưa có đề bài.`)
	})
	t.Run("homework chưa có tiêu chí", func(t *testing.T) {
		v, u := setup(t)
		addHomework(t, v, u, "Bài tập", "Đề")
		publish(t, v, `"Bài tập" chưa có tiêu chí chấm.`)
	})
	t.Run("câu hỏi chưa render", func(t *testing.T) {
		v, u := setup(t)
		addQuestion(t, v, addQuiz(t, v, u, "Kiểm tra", assessment()), singleQuestion("Q"))
		if err := v.Publish(testNow); !errors.Is(err, ErrNotRendered) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("render đủ câu hỏi, giải thích và đề bài", func(t *testing.T) {
		v := publishedVersion(t)
		q := v.Lessons()[2].Content().(QuizContent).Questions[0]
		if q.BodyHTML == nil || *q.BodyHTML != "<p>KHÓA CHÍNH LÀ GÌ?</p>" || q.ExplanationHTML == nil || *q.ExplanationHTML != "<p>ĐỊNH DANH DUY NHẤT.</p>" {
			t.Fatalf("question html = %v %v", q.BodyHTML, q.ExplanationHTML)
		}
		hw := v.Lessons()[3].Content().(HomeworkContent)
		if hw.BriefHTML == nil || *hw.BriefHTML != "<p>THIẾT KẾ LƯỢC ĐỒ</p>" {
			t.Fatalf("brief html = %v", hw.BriefHTML)
		}
		if got := renderedHTML(v.Lessons()[2].Content()); len(got) != 2 {
			t.Fatalf("rendered html = %v", got)
		}
	})
}

func TestCloneDeepCopiesQuizAndHomework(t *testing.T) {
	src := publishedVersion(t)
	c, err := src.CloneAsDraft(uuid.New(), 2, uuid.New(), uuid.New)
	if err != nil {
		t.Fatal(err)
	}
	sq, cq := src.Lessons()[2].Content().(QuizContent), c.Lessons()[2].Content().(QuizContent)
	if cq.Mode != QuizAssessment || *cq.MaxAttempts != *sq.MaxAttempts || *cq.TimeLimit != *sq.TimeLimit || *cq.PassPercent != *sq.PassPercent || cq.Shuffle != sq.Shuffle {
		t.Fatalf("quiz settings not cloned: %+v", cq)
	}
	if cq.MaxAttempts == sq.MaxAttempts {
		t.Fatal("clone shares settings pointer with source")
	}
	if len(cq.Questions) != 1 {
		t.Fatalf("questions = %d", len(cq.Questions))
	}
	s0, c0 := sq.Questions[0], cq.Questions[0]
	if c0 == s0 || c0.ID == s0.ID || c0.Key != s0.Key || c0.BodySource != s0.BodySource || *c0.BodyHTML != *s0.BodyHTML {
		t.Fatalf("question not deep-cloned: %+v", c0)
	}
	for i := range s0.Options {
		if c0.Options[i].ID == s0.Options[i].ID || c0.Options[i].Body != s0.Options[i].Body || c0.Options[i].Correct != s0.Options[i].Correct {
			t.Fatalf("option %d not cloned with new id", i)
		}
	}
	sh, ch := src.Lessons()[3].Content().(HomeworkContent), c.Lessons()[3].Content().(HomeworkContent)
	if ch.BriefSource != sh.BriefSource || ch.PassPercent != sh.PassPercent || len(ch.Criteria) != 1 ||
		ch.Criteria[0].ID == sh.Criteria[0].ID || ch.Criteria[0].Description != sh.Criteria[0].Description || ch.Criteria[0].MaxPoints != sh.Criteria[0].MaxPoints {
		t.Fatalf("homework not cloned: %+v", ch)
	}
	if len(ch.Attachments) != 1 || ch.Attachments[0].MediaID != sh.Attachments[0].MediaID {
		t.Fatal("attachments must keep media references")
	}
	if _, err := c.UpdateQuestion(c.Lessons()[2].ID(), c0.ID, singleQuestion("Sửa"), uuid.New); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateCriterion(c.Lessons()[3].ID(), ch.Criteria[0].ID, "Sửa", 1); err != nil {
		t.Fatal(err)
	}
	if s0.BodySource != "Khóa chính là gì?" || sh.Criteria[0].Description != "Đủ bảng" {
		t.Fatal("editing clone changed source")
	}
}
