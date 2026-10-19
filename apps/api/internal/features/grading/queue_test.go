package grading

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

var t0 = time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)

func intp(v int) *int { return &v }

func TestWithClockOverdueBoundary(t *testing.T) {
	q := QueueItem{SubmittedAt: t0}
	cases := []struct {
		name    string
		now     time.Time
		hours   int
		overdue bool
	}{
		{"vừa nộp", t0, 0, false},
		{"đồng hồ lệch về trước lúc nộp", t0.Add(-time.Minute), 0, false},
		{"gần 2 giờ làm tròn xuống", t0.Add(119 * time.Minute), 1, false},
		{"đúng 72 giờ chưa quá hạn", t0.Add(GradingSLA), 72, false},
		{"quá 72 giờ một micro giây", t0.Add(GradingSLA + time.Microsecond), 72, true},
		{"4,5 ngày", t0.Add(108 * time.Hour), 108, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := q.withClock(c.now)
			assert.Equal(t, c.hours, got.WaitedHours)
			assert.Equal(t, c.overdue, got.Overdue)
		})
	}
}

func TestScorePercentOnlyForQuiz(t *testing.T) {
	assert.Nil(t, QueueItem{}.ScorePercent())
	assert.Equal(t, intp(67), QueueItem{Score: intp(2), MaxScore: intp(3)}.ScorePercent())
	assert.Equal(t, intp(0), QueueItem{Score: intp(0), MaxScore: intp(0)}.ScorePercent())
}

func TestParseKind(t *testing.T) {
	for _, s := range []string{"", "homework", "quiz"} {
		k, err := ParseKind(s)
		require.NoError(t, err)
		assert.Equal(t, Kind(s), k)
	}
	for _, s := range []string{"video", "Quiz", " homework"} {
		_, err := ParseKind(s)
		assert.ErrorIs(t, err, ErrInvalidFilter, s)
	}
}

func TestActorScope(t *testing.T) {
	id := uuid.New()
	got, err := Actor{ID: id, Role: domain.RoleAdmin}.scope()
	require.NoError(t, err)
	assert.Nil(t, got, "admin xem mọi lớp")
	got, err = Actor{ID: id, Role: domain.RoleTeacher}.scope()
	require.NoError(t, err)
	assert.Equal(t, &id, got)
	_, err = Actor{ID: id, Role: domain.RoleStudent}.scope()
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

// fixture: hai lớp, homework và quiz xen kẽ, đã sắp cũ nhất trước như repository trả.
func fixture() (all []QueueItem, classA, classB, hw, quiz uuid.UUID) {
	classA, classB, hw, quiz = uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mk := func(kind Kind, class, lesson uuid.UUID, hoursAgo int) QueueItem {
		return QueueItem{
			Key: QueueKey{Kind: kind, ItemID: uuid.New()}, SubmittedAt: t0.Add(-time.Duration(hoursAgo) * time.Hour),
			Class: Ref{ID: class}, Lesson: Ref{ID: lesson},
		}.withClock(t0)
	}
	all = []QueueItem{
		mk(KindHomework, classA, hw, 100),
		mk(KindQuiz, classB, quiz, 80),
		mk(KindQuiz, classA, quiz, 50),
		mk(KindHomework, classB, hw, 10),
	}
	return all, classA, classB, hw, quiz
}

func keys(items []QueueItem) []QueueKey {
	out := make([]QueueKey, len(items))
	for i, it := range items {
		out[i] = it.Key
	}
	return out
}

func TestBuildQueueFiltersItemsButCountsWholeQueue(t *testing.T) {
	all, classA, classB, hw, quiz := fixture()
	cases := []struct {
		name string
		f    Filter
		want []QueueItem
	}{
		{"không lọc", Filter{}, all},
		{"loại quiz", Filter{Kind: KindQuiz}, []QueueItem{all[1], all[2]}},
		{"lớp A", Filter{ClassID: &classA}, []QueueItem{all[0], all[2]}},
		{"học liệu homework", Filter{LessonID: &hw}, []QueueItem{all[0], all[3]}},
		{"quá hạn", Filter{OverdueOnly: true}, []QueueItem{all[0], all[1]}},
		{"lớp B, quiz, quá hạn", Filter{ClassID: &classB, Kind: KindQuiz, OverdueOnly: true}, []QueueItem{all[1]}},
		{"học liệu quiz ở lớp B nhưng loại homework", Filter{ClassID: &classB, LessonID: &quiz, Kind: KindHomework}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := buildQueue(all, c.f, nil, nil)
			assert.Equal(t, keys(c.want), keys(q.Items))
			assert.NotNil(t, q.Items, "items rỗng vẫn là mảng")
			assert.Equal(t, 4, q.Total)
			assert.Equal(t, 2, q.QuizPending)
			assert.Equal(t, 2, q.OverdueCount)
		})
	}
}

func TestBuildQueueLessonOptions(t *testing.T) {
	classA, classB := uuid.New(), uuid.New()
	hw1, quiz1, hw2 := uuid.New(), uuid.New(), uuid.New()
	opt := func(class, id uuid.UUID, kind Kind) LessonOptionRow {
		return LessonOptionRow{ClassID: class, LessonOption: LessonOption{ID: id, Title: id.String(), Kind: kind}}
	}
	// Hai lớp chung phiên bản khóa (hw1, quiz1) và lớp B có thêm hw2.
	rows := []LessonOptionRow{
		opt(classA, hw1, KindHomework), opt(classA, quiz1, KindQuiz),
		opt(classB, hw1, KindHomework), opt(classB, quiz1, KindQuiz), opt(classB, hw2, KindHomework),
	}
	ids := func(ls []LessonOption) []uuid.UUID {
		out := make([]uuid.UUID, len(ls))
		for i, l := range ls {
			out[i] = l.ID
		}
		return out
	}
	assert.Equal(t, []uuid.UUID{hw1, quiz1, hw2}, ids(buildQueue(nil, Filter{}, nil, rows).Lessons), "mỗi học liệu một lần")
	assert.Equal(t, []uuid.UUID{hw1, quiz1}, ids(buildQueue(nil, Filter{ClassID: &classA}, nil, rows).Lessons))
	assert.Equal(t, []uuid.UUID{hw1, hw2}, ids(buildQueue(nil, Filter{Kind: KindHomework}, nil, rows).Lessons))
	assert.Equal(t, []uuid.UUID{quiz1}, ids(buildQueue(nil, Filter{ClassID: &classB, Kind: KindQuiz}, nil, rows).Lessons))
	other := uuid.New()
	assert.Empty(t, buildQueue(nil, Filter{ClassID: &other}, nil, rows).Lessons)
	assert.NotNil(t, buildQueue(nil, Filter{ClassID: &other}, nil, rows).Lessons, "lessons rỗng vẫn là mảng")
}

func TestNextSkipsHandledItem(t *testing.T) {
	all, _, _, _, _ := fixture()
	assert.Equal(t, all[0].Key, next(all, QueueKey{}).Key)
	assert.Equal(t, all[1].Key, next(all, all[0].Key).Key, "mục vừa xử lý vẫn còn trong snapshot thì bỏ qua")
	assert.Equal(t, all[0].Key, next(all, all[2].Key).Key)
	assert.Nil(t, next(all[:1], all[0].Key))
	assert.Nil(t, next(nil, QueueKey{}))
}

func TestParseFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := uuid.MustParse("01990000-0000-7000-8000-000000000001")
	parse := func(query string) (Filter, error) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/grading/queue?"+query, nil)
		return parseFilter(c)
	}

	f, err := parse("")
	require.NoError(t, err)
	assert.Equal(t, Filter{}, f)
	f, err = parse("kind=quiz&classId=" + id.String() + "&lessonId=" + id.String() + "&overdue=1")
	require.NoError(t, err)
	assert.Equal(t, Filter{Kind: KindQuiz, ClassID: &id, LessonID: &id, OverdueOnly: true}, f)
	f, err = parse("overdue=0&classId=")
	require.NoError(t, err)
	assert.Equal(t, Filter{}, f)

	for _, q := range []string{"kind=all", "classId=nope", "lessonId=" + id.String() + "x", "overdue=yes"} {
		_, err := parse(q)
		assert.True(t, errors.Is(err, ErrInvalidFilter), q)
	}
}
