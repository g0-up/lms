package stages

import (
	"errors"
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

func addMarkdown(t *testing.T, v *StageVersion, title, src string) *Lesson {
	t.Helper()
	l, err := v.AddLesson(uuid.New(), "", title, MarkdownContent{Source: src}, true)
	if err != nil {
		t.Fatalf("AddLesson: %v", err)
	}
	return l
}

type upperRenderer struct{}

func (upperRenderer) Render(src string) (string, error) {
	return "<p>" + strings.ToUpper(src) + "</p>", nil
}

func publishedVersion(t *testing.T) *StageVersion {
	t.Helper()
	v := newDraft(t)
	addMarkdown(t, v, "Bảng", "bảng")
	if _, err := v.AddLesson(uuid.New(), "video-index", "Chỉ mục", VideoContent{MediaID: uuid.New(), DurationSeconds: intPtr(300)}, false); err != nil {
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
	a := addMarkdown(t, v, "Giới thiệu", "x")
	b := addMarkdown(t, v, "Giới thiệu", "y")
	if a.Key() != "gioi-thieu" || b.Key() != "gioi-thieu-2" {
		t.Fatalf("keys = %q, %q", a.Key(), b.Key())
	}
	if a.Position() != 1 || b.Position() != 2 {
		t.Fatalf("positions = %d, %d", a.Position(), b.Position())
	}
	if _, err := v.AddLesson(uuid.New(), "gioi-thieu", "Khác", MarkdownContent{Source: "z"}, true); !errors.Is(err, ErrLessonKeyTaken) {
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
			if _, err := v.AddLesson(uuid.New(), "", tt.title, tt.content, true); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestMutationsOnPublishedAreImmutable(t *testing.T) {
	v := publishedVersion(t)
	first := v.Lessons()[0]
	checks := map[string]error{
		"AddLesson": func() error {
			_, err := v.AddLesson(uuid.New(), "", "Mới", MarkdownContent{Source: "x"}, true)
			return err
		}(),
		"UpdateLesson":   v.UpdateLesson(first.ID(), "Sửa", MarkdownContent{Source: "x"}, true),
		"RemoveLesson":   v.RemoveLesson(first.ID()),
		"Reorder":        v.Reorder([]uuid.UUID{v.Lessons()[1].ID(), first.ID()}),
		"RenderMarkdown": v.RenderMarkdown(upperRenderer{}),
	}
	for name, err := range checks {
		if !errors.Is(err, ErrVersionImmutable) {
			t.Errorf("%s err = %v, want ErrVersionImmutable", name, err)
		}
	}
	if len(v.Lessons()) != 2 || v.Lessons()[0].Title() != "Bảng" {
		t.Fatal("lessons changed on published version")
	}
}

func TestPublishRules(t *testing.T) {
	t.Run("rỗng", func(t *testing.T) {
		if err := newDraft(t).Publish(testNow); !errors.Is(err, ErrNoLessons) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("markdown chưa render", func(t *testing.T) {
		v := newDraft(t)
		addMarkdown(t, v, "A", "x")
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

func TestReorder(t *testing.T) {
	v := newDraft(t)
	a := addMarkdown(t, v, "A", "a")
	b := addMarkdown(t, v, "B", "b")
	c := addMarkdown(t, v, "C", "c")
	if err := v.Reorder([]uuid.UUID{a.ID(), b.ID()}); !errors.Is(err, ErrReorderMismatch) {
		t.Fatalf("missing id err = %v", err)
	}
	if err := v.Reorder([]uuid.UUID{a.ID(), a.ID(), b.ID()}); !errors.Is(err, ErrReorderMismatch) {
		t.Fatalf("duplicate id err = %v", err)
	}
	if err := v.Reorder([]uuid.UUID{c.ID(), a.ID(), b.ID()}); err != nil {
		t.Fatal(err)
	}
	got := v.Lessons()
	if got[0].ID() != c.ID() || got[0].Position() != 1 || got[2].ID() != b.ID() || got[2].Position() != 3 {
		t.Fatalf("order wrong: %v", []string{got[0].Title(), got[1].Title(), got[2].Title()})
	}
	if err := v.RemoveLesson(c.ID()); err != nil {
		t.Fatal(err)
	}
	if got := v.Lessons(); got[0].ID() != a.ID() || got[0].Position() != 1 || got[1].Position() != 2 {
		t.Fatal("positions not compacted after remove")
	}
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

func TestCloneAsDraftKeepsLessons(t *testing.T) {
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
	for i, sl := range src.Lessons() {
		cl := c.Lessons()[i]
		if cl.ID() == sl.ID() || cl.Key() != sl.Key() || cl.Required() != sl.Required() || cl.Position() != sl.Position() {
			t.Fatalf("lesson %d not cloned correctly", i)
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
	if src.Lessons()[0].Title() != "Bảng" {
		t.Fatal("editing clone changed source")
	}
}
