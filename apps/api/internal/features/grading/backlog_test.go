package grading

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestBuildBacklogCountsPerTeacherAndWholeQueue(t *testing.T) {
	huong, bao, disabled := uuid.New(), uuid.New(), uuid.New()
	mk := func(teacher uuid.UUID, hoursAgo int) QueueItem {
		return QueueItem{TeacherID: teacher, SubmittedAt: t0.Add(-time.Duration(hoursAgo) * time.Hour)}
	}
	all := stamp([]QueueItem{mk(huong, 108), mk(huong, 84), mk(huong, 5), mk(bao, 72), mk(disabled, 200)}, t0)
	teachers := []Teacher{{ID: huong, Name: "Lê Thu Hương"}, {ID: bao, Name: "Phạm Quốc Bảo"}}

	rows, overdue := buildBacklog(all, teachers, map[uuid.UUID]float64{huong: 30.5, disabled: 10})
	assert.Equal(t, 3, overdue, "tổng quá hạn tính cả lớp của giảng viên đã bị khóa")
	h := 30.5
	assert.Equal(t, []TeacherBacklog{
		{TeacherID: huong, TeacherName: "Lê Thu Hương", Pending: 3, Overdue: 2, AvgHours: &h},
		{TeacherID: bao, TeacherName: "Phạm Quốc Bảo", Pending: 1, Overdue: 0},
	}, rows, "theo thứ tự teachers; đúng 72 giờ chưa quá hạn; chưa xử lý mục nào thì AvgHours nil")
}

func TestBuildBacklogTeacherWithoutItems(t *testing.T) {
	id := uuid.New()
	rows, overdue := buildBacklog(nil, []Teacher{{ID: id, Name: "Phạm Quốc Bảo"}}, nil)
	assert.Zero(t, overdue)
	assert.Equal(t, []TeacherBacklog{{TeacherID: id, TeacherName: "Phạm Quốc Bảo"}}, rows)
}

func TestOverdueByTeacherKeepsOldestFirst(t *testing.T) {
	huong := uuid.New()
	a := QueueItem{TeacherID: huong, SubmittedAt: t0.Add(-100 * time.Hour)}
	b := QueueItem{TeacherID: huong, SubmittedAt: t0.Add(-80 * time.Hour)}
	c := QueueItem{TeacherID: huong, SubmittedAt: t0.Add(-1 * time.Hour)}
	got := overdueByTeacher(stamp([]QueueItem{a, b, c}, t0))
	assert.Len(t, got, 1)
	assert.Equal(t, []time.Time{a.SubmittedAt, b.SubmittedAt},
		[]time.Time{got[huong][0].SubmittedAt, got[huong][1].SubmittedAt})
	assert.Len(t, got[huong], 2)
}
