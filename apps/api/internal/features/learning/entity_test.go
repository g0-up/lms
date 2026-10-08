package learning

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/classes"
)

var testNow = time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)

func TestSetCompletedIsIdempotent(t *testing.T) {
	p := OpenLesson(uuid.New(), uuid.New(), testNow)
	if p.State() != StateOpened {
		t.Fatalf("State sau khi mở = %s", p.State())
	}
	if !p.SetCompleted(true, testNow.Add(time.Minute)) {
		t.Fatal("tích lần đầu phải đổi")
	}
	first := *p.CompletedAt()
	if p.SetCompleted(true, testNow.Add(time.Hour)) {
		t.Fatal("tích lại không được đổi")
	}
	if !p.CompletedAt().Equal(first) || p.State() != StateCompleted {
		t.Fatalf("completedAt = %v, muốn giữ %v", p.CompletedAt(), first)
	}
	if !p.SetCompleted(false, testNow.Add(2*time.Hour)) || p.CompletedAt() != nil {
		t.Fatal("bỏ tích phải xóa completedAt")
	}
	if p.SetCompleted(false, testNow.Add(3*time.Hour)) {
		t.Fatal("bỏ tích khi chưa tích không được đổi")
	}
	if p.State() != StateOpened {
		t.Fatalf("State sau bỏ tích = %s", p.State())
	}
}

func TestSetCompletedNeverBeforeFirstOpened(t *testing.T) {
	p := OpenLesson(uuid.New(), uuid.New(), testNow)
	p.SetCompleted(true, testNow.Add(-time.Second))
	if !p.CompletedAt().Equal(testNow) {
		t.Fatalf("completedAt = %v, muốn kẹp về firstOpenedAt", p.CompletedAt())
	}
}

func TestNilProgressIsNotOpened(t *testing.T) {
	var p *LessonProgress
	if p.State() != StateNotOpened {
		t.Fatalf("State(nil) = %s", p.State())
	}
}

func TestLearningContextGuards(t *testing.T) {
	member := func(ms domain.MemberStatus, cs domain.ClassStatus) LearningContext {
		return LearningContext{Member: classes.MemberContext{MemberID: uuid.New(), MemberStatus: ms, ClassStatus: cs}}
	}
	tests := []struct {
		name               string
		lc                 LearningContext
		view, open, record error
		reason             string
	}{
		{"không phải thành viên", LearningContext{Member: classes.MemberContext{ClassStatus: domain.ClassActive}},
			ErrNotFound, ErrNotFound, ErrNotFound, ""},
		{"đã rời lớp", member(domain.MemberDropped, domain.ClassActive), ErrNotFound, ErrNotFound, ErrNotFound, ""},
		{"lớp nháp", member(domain.MemberActive, domain.ClassDraft), nil, ErrClassNotStarted, ErrClassNotActive, ReadOnlyDraft},
		{"lớp đang chạy", member(domain.MemberActive, domain.ClassActive), nil, nil, nil, ""},
		{"lớp đã kết thúc", member(domain.MemberActive, domain.ClassEnded), nil, nil, ErrClassNotActive, ReadOnlyEnded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.lc.CanView(); !errors.Is(err, tt.view) {
				t.Errorf("CanView = %v, muốn %v", err, tt.view)
			}
			if err := tt.lc.CanOpen(); !errors.Is(err, tt.open) {
				t.Errorf("CanOpen = %v, muốn %v", err, tt.open)
			}
			if err := tt.lc.CanRecord(); !errors.Is(err, tt.record) {
				t.Errorf("CanRecord = %v, muốn %v", err, tt.record)
			}
			if got := tt.lc.ReadOnlyReason(); got != tt.reason {
				t.Errorf("ReadOnlyReason = %q, muốn %q", got, tt.reason)
			}
		})
	}
}
