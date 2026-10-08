package courses

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

var testNow = time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)

func ref(stageID uuid.UUID, no domain.VersionNo, status domain.VersionStatus) StageVersionRef {
	return StageVersionRef{ID: uuid.New(), StageID: stageID, StageCode: "DB", VersionNo: no, Status: status}
}

func newDraft() *CourseVersion {
	return NewDraftCourseVersion(uuid.New(), uuid.New(), domain.FirstVersionNo, "Lập trình cơ bản", "", nil, uuid.New())
}

// lookupOf trả lookup cho Publish từ danh sách ref.
func lookupOf(refs ...StageVersionRef) func(uuid.UUID) (StageVersionRef, error) {
	m := make(map[uuid.UUID]StageVersionRef, len(refs))
	for _, r := range refs {
		m[r.ID] = r
	}
	return func(id uuid.UUID) (StageVersionRef, error) {
		r, ok := m[id]
		if !ok {
			return StageVersionRef{}, ErrStageVersionNotFound
		}
		return r, nil
	}
}

func publishedWith(t *testing.T, refs ...StageVersionRef) *CourseVersion {
	t.Helper()
	v := newDraft()
	if err := v.SetStages(refs); err != nil {
		t.Fatal(err)
	}
	if err := v.Publish(testNow, lookupOf(refs...)); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestNewCourseRequiresCodeAndName(t *testing.T) {
	if _, err := NewCourse(uuid.New(), "BASIC", "  ", "", uuid.New(), testNow); !errors.Is(err, ErrCourseNameRequired) {
		t.Fatalf("thiếu tên: %v", err)
	}
	if _, err := NewCourse(uuid.New(), "", "Lập trình", "", uuid.New(), testNow); !errors.Is(err, ErrCourseNameRequired) {
		t.Fatalf("thiếu mã: %v", err)
	}
	c, err := NewCourse(uuid.New(), "BASIC", " Lập trình cơ bản ", " Nhập môn ", uuid.New(), testNow)
	if err != nil || c.Name() != "Lập trình cơ bản" || c.Description() != "Nhập môn" {
		t.Fatalf("NewCourse = %+v, %v", c, err)
	}
}

func TestSetStages(t *testing.T) {
	db, web := uuid.New(), uuid.New()
	tests := []struct {
		name string
		refs []StageVersionRef
		want error
	}{
		{"hợp lệ", []StageVersionRef{ref(db, 1, domain.VersionPublished), ref(web, 2, domain.VersionPublished)}, nil},
		{"rỗng được phép trên bản nháp", nil, nil},
		{"ref bản nháp", []StageVersionRef{ref(db, 2, domain.VersionDraft)}, ErrStageVersionNotPublished},
		{"ref đã lưu trữ", []StageVersionRef{ref(db, 1, domain.VersionArchived)}, ErrStageVersionNotPublished},
		{"hai phiên bản cùng chặng", []StageVersionRef{ref(db, 1, domain.VersionPublished), ref(db, 2, domain.VersionPublished)}, ErrDuplicateStage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newDraft()
			err := v.SetStages(tt.refs)
			if !errors.Is(err, tt.want) {
				t.Fatalf("SetStages = %v, muốn %v", err, tt.want)
			}
			if tt.want != nil {
				return
			}
			got := v.Stages()
			if len(got) != len(tt.refs) {
				t.Fatalf("len = %d", len(got))
			}
			for i, s := range got {
				if s.Position != i+1 || s.StageVersionID != tt.refs[i].ID || s.StageID != tt.refs[i].StageID {
					t.Fatalf("stage %d = %+v", i, s)
				}
			}
		})
	}
}

func TestSetStagesOnPublishedIsImmutable(t *testing.T) {
	v := publishedWith(t, ref(uuid.New(), 1, domain.VersionPublished))
	if err := v.SetStages(nil); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("SetStages trên bản published = %v", err)
	}
}

func TestPublish(t *testing.T) {
	db := uuid.New()
	t.Run("rỗng", func(t *testing.T) {
		v := newDraft()
		if err := v.Publish(testNow, lookupOf()); !errors.Is(err, ErrNoStages) || err.Error() != "Khóa học cần ít nhất một chặng." {
			t.Fatalf("Publish rỗng = %v", err)
		}
	})
	t.Run("ref đã lưu trữ tại thời điểm phát hành", func(t *testing.T) {
		r := ref(db, 1, domain.VersionPublished)
		v := newDraft()
		if err := v.SetStages([]StageVersionRef{r}); err != nil {
			t.Fatal(err)
		}
		archived := r
		archived.Status = domain.VersionArchived
		err := v.Publish(testNow, lookupOf(archived))
		if !errors.Is(err, domain.ErrInvalid) || err.Error() != "Phiên bản chặng DB v1 đã lưu trữ, không thể phát hành lại." {
			t.Fatalf("Publish = %v", err)
		}
		if v.Status() != domain.VersionDraft || v.PublishedAt() != nil {
			t.Fatal("Publish lỗi không được đổi trạng thái")
		}
	})
	t.Run("hợp lệ", func(t *testing.T) {
		v := publishedWith(t, ref(db, 1, domain.VersionPublished))
		if v.Status() != domain.VersionPublished || v.PublishedAt() == nil || !v.PublishedAt().Equal(testNow) {
			t.Fatalf("status=%s publishedAt=%v", v.Status(), v.PublishedAt())
		}
		if err := v.Publish(testNow, lookupOf()); !errors.Is(err, ErrPublishNotDraft) {
			t.Fatalf("Publish lần hai = %v", err)
		}
	})
}

func TestArchive(t *testing.T) {
	if err := newDraft().Archive(testNow); !errors.Is(err, ErrArchiveNotPublished) {
		t.Fatalf("Archive bản nháp = %v", err)
	}
	v := publishedWith(t, ref(uuid.New(), 1, domain.VersionPublished))
	if err := v.Archive(testNow); err != nil || v.Status() != domain.VersionArchived || v.ArchivedAt() == nil {
		t.Fatalf("Archive = %v, status=%s", err, v.Status())
	}
}

func TestCloneAsDraftCopiesStagesIndependently(t *testing.T) {
	db, web := uuid.New(), uuid.New()
	r1, r2 := ref(db, 1, domain.VersionPublished), ref(web, 1, domain.VersionPublished)
	if _, err := newDraft().CloneAsDraft(uuid.New(), 2, uuid.New()); !errors.Is(err, ErrCloneNotPublished) {
		t.Fatalf("Clone bản nháp = %v", err)
	}
	src := publishedWith(t, r1, r2)
	c, err := src.CloneAsDraft(uuid.New(), 2, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if c.Status() != domain.VersionDraft || c.VersionNo() != 2 || c.ClonedFromID() == nil || *c.ClonedFromID() != src.ID() {
		t.Fatalf("clone = status %s no %d from %v", c.Status(), c.VersionNo(), c.ClonedFromID())
	}
	next := ref(db, 2, domain.VersionPublished)
	next.StageID = db
	if err := c.ReplaceStageVersion(r1, next); err != nil {
		t.Fatal(err)
	}
	if s, _ := src.StageVersionFor(db); s.StageVersionID != r1.ID {
		t.Fatal("sửa bản clone làm đổi bản gốc")
	}
	if s, _ := c.StageVersionFor(db); s.StageVersionID != next.ID || s.Position != 1 {
		t.Fatalf("clone sau thay = %+v", s)
	}
}

func TestReplaceStageVersion(t *testing.T) {
	db, web := uuid.New(), uuid.New()
	r1, w1 := ref(db, 1, domain.VersionPublished), ref(web, 1, domain.VersionPublished)
	tests := []struct {
		name string
		old  StageVersionRef
		next StageVersionRef
		want error
	}{
		{"cùng chặng giữ vị trí", w1, ref(web, 2, domain.VersionPublished), nil},
		{"khác chặng", r1, ref(web, 2, domain.VersionPublished), ErrStageMismatch},
		{"bản thay chưa phát hành", r1, ref(db, 2, domain.VersionDraft), ErrStageVersionNotPublished},
		{"bản cũ không có trong khóa học", ref(db, 3, domain.VersionPublished), ref(db, 2, domain.VersionPublished), ErrCourseLacksStage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newDraft()
			if err := v.SetStages([]StageVersionRef{r1, w1}); err != nil {
				t.Fatal(err)
			}
			err := v.ReplaceStageVersion(tt.old, tt.next)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ReplaceStageVersion = %v, muốn %v", err, tt.want)
			}
			if tt.want == nil {
				if s, _ := v.StageVersionFor(web); s.StageVersionID != tt.next.ID || s.Position != 2 {
					t.Fatalf("sau thay = %+v", s)
				}
			}
		})
	}
	v := publishedWith(t, r1)
	if err := v.ReplaceStageVersion(r1, ref(db, 2, domain.VersionPublished)); !errors.Is(err, ErrVersionImmutable) {
		t.Fatalf("thay trên bản published = %v", err)
	}
}

func TestCanDelete(t *testing.T) {
	if err := newDraft().CanDelete(); err != nil {
		t.Fatalf("CanDelete bản nháp = %v", err)
	}
	v := publishedWith(t, ref(uuid.New(), 1, domain.VersionPublished))
	if err := v.CanDelete(); !errors.Is(err, ErrDeleteNotDraft) {
		t.Fatalf("CanDelete bản published = %v", err)
	}
}

func TestErrorMessages(t *testing.T) {
	draft := &ErrDraftExists{DraftID: uuid.New(), No: 3}
	if got := draft.Error(); got != "Khóa học đang có bản nháp v3. Phát hành hoặc xóa bản nháp trước." {
		t.Fatalf("ErrDraftExists = %q", got)
	}
	inUse := &ErrInUse{UsedBy: []UsedByClassRow{{Code: "basic01"}, {Code: "basic02"}}}
	if got := inUse.Error(); got != "Đang được dùng bởi lớp basic01, basic02. Hãy lưu trữ thay vì xóa." {
		t.Fatalf("ErrInUse = %q", got)
	}
}
