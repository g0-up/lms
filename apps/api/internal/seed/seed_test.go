package seed

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/config"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"18:24", 1104, true},
		{"0:59", 59, true},
		{"1:02:03", 3723, true},
		{"28:40", 1720, true},
		{"18", 0, false},
		{"18:6", 0, false},
		{"18:60", 0, false},
		{"-1:00", 0, false},
		{"a:00", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, err := parseDuration(c.in)
		if !c.ok {
			assert.Error(t, err, c.in)
			continue
		}
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got, c.in)
	}
}

// TestDatasetMatchesPrototype ghim số lượng của prototype/seed.js để lỗi chép dữ liệu hiện ra ngay.
func TestDatasetMatchesPrototype(t *testing.T) {
	require.NoError(t, checkData())

	assert.Len(t, users, 17)
	assert.Len(t, stages, 5)
	assert.Len(t, stageVersions, 5)
	assert.Equal(t, []stageDraftSeed{{key: "sv-go-2", from: "sv-go-1", no: 2, createdAt: ago(2.8, 0)}}, stageDrafts,
		"seed.js: bản nháp Golang basic v2 nhân bản từ v1")
	assert.Equal(t, 30, lessonCount())
	assert.Equal(t, 16, unitCount())
	assert.Equal(t, 9, videoLessonCount())
	assert.Equal(t, map[string]int{"practice": 6, "assessment": 5}, quizCountByMode())
	assert.Equal(t, 2, homeworkCount())
	assert.Len(t, courses, 1)
	assert.Len(t, courseVersions, 1)
	assert.Len(t, classes, 3)
	assert.Len(t, enrollments, 15, "basic01: 8 (gồm Phong vào muộn và Thảo đã rời), basic02: 5, basic03: 2")
	assert.Len(t, audits, 10)
	assert.Len(t, deadlines, 3)
	assert.Len(t, submissions, 10)
	assert.Len(t, quizAttempts, 33, "qa1–qa32 đã nộp và lượt đang làm của Bích")
	autos := 0
	for _, a := range quizAttempts {
		if a.auto {
			autos++
		}
	}
	assert.Equal(t, 1, autos, "một lượt tự nộp khi hết giờ")
	assert.Len(t, stageReviews, 2, "stageReviews của seed.js: An và Cường, chặng Data structure")
	reviewers := map[string]int{}
	for _, a := range quizAttempts {
		if a.review != nil {
			reviewers[a.review.by]++
		}
	}
	assert.Equal(t, map[string]int{"u-gv": 11, "u-gv2": 1}, reviewers, "review() của seed.js: Bảo nhận xét qa19")

	byStatus := map[string]int{}
	for _, e := range enrollments {
		byStatus[e.invite]++
	}
	assert.Equal(t, map[string]int{inviteSent: 13, inviteFailed: 1, inviteQueued: 1}, byStatus)

	var first, last userSeed
	for _, u := range users {
		if u.key == adminKey {
			first = u
		}
		if u.key == "u-thao" {
			last = u
		}
	}
	assert.Equal(t, markerEmail, first.email)
	assert.Equal(t, "admin", first.role)
	assert.Equal(t, "disabled", last.status)
}

func lessonCount() int {
	n := 0
	for _, sv := range stageVersions {
		n += len(sv.lessons)
	}
	return n
}

func unitCount() int {
	n := 0
	for _, sv := range stageVersions {
		n += len(sv.units)
	}
	return n
}

// TestCheckDataRejectsBadUnitGrouping: học liệu phải thuộc bài học của chính phiên bản, xếp theo thứ tự bài học,
// và không có bài học rỗng (phiên bản đã phát hành không chứa bài học rỗng).
func TestCheckDataRejectsBadUnitGrouping(t *testing.T) {
	db := stageVersions[0]
	cases := []struct {
		name    string
		mutate  func(sv *stageVersionSeed)
		wantMsg string
	}{
		{"unknown unit", func(sv *stageVersionSeed) { sv.lessons[0] = sv.lessons[0].in("ux-go-hello") }, "không có trong sv-db-1"},
		{"out of unit order", func(sv *stageVersionSeed) {
			sv.lessons[0], sv.lessons[2] = sv.lessons[2], sv.lessons[0]
		}, "không xếp theo thứ tự bài học"},
		{"empty unit", func(sv *stageVersionSeed) {
			sv.units = append(sv.units, unitSeed{"ux-db-extra", "Ôn tập"})
		}, `bài học "ux-db-extra" của sv-db-1 chưa có học liệu`},
		{"no units", func(sv *stageVersionSeed) { sv.units = nil }, "phiên bản \"sv-db-1\" chưa có bài học"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sv := db
			sv.units = append([]unitSeed(nil), db.units...)
			sv.lessons = append([]lessonSeed(nil), db.lessons...)
			c.mutate(&sv)
			stageVersions[0] = sv
			t.Cleanup(func() { stageVersions[0] = db })

			err := checkData()
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantMsg)
		})
	}
	require.NoError(t, checkData())
}

// TestCheckDataRejectsBadQuizHomework: nội dung quiz/homework seed phải qua cùng luật với API soạn bài.
func TestCheckDataRejectsBadQuizHomework(t *testing.T) {
	db := stageVersions[0]
	ds := stageVersions[1]
	quizAt := slices.IndexFunc(db.lessons, func(l lessonSeed) bool { return l.key == "db-practice" })
	hwAt := slices.IndexFunc(ds.lessons, func(l lessonSeed) bool { return l.key == "ds-hw" })
	require.GreaterOrEqual(t, quizAt, 0)
	require.GreaterOrEqual(t, hwAt, 0)

	quizCases := []struct {
		name    string
		mutate  func(q *quizSeed)
		wantMsg string
	}{
		{"no questions", func(q *quizSeed) { q.questions = nil }, "chưa có câu hỏi"},
		{"duplicate key", func(q *quizSeed) { q.questions = append(q.questions, q.questions[0]) }, "trùng mã câu hỏi"},
		{"bad key", func(q *quizSeed) { q.questions[0].key = "Bad Key" }, `mã câu hỏi "Bad Key"`},
		{"one option", func(q *quizSeed) { q.questions[0].options = q.questions[0].options[:1] }, "sai loại, nội dung hoặc số phương án"},
		{"no correct option", func(q *quizSeed) {
			q.questions[0].options = []optionSeed{wrong("A"), wrong("B")}
		}, "sai số đáp án đúng"},
		{"two correct on single", func(q *quizSeed) {
			q.questions[0].typ = "single"
			q.questions[0].options = []optionSeed{right("A"), right("B")}
		}, "sai số đáp án đúng"},
		{"option too long", func(q *quizSeed) {
			q.questions[0].options = []optionSeed{right(strings.Repeat("x", 501)), wrong("B")}
		}, "phương án sai độ dài"},
		{"assessment without settings", func(q *quizSeed) { q.mode = "assessment"; q.attempts, q.minutes, q.percent = 0, 0, 0 }, `quiz "db-practice"`},
	}
	for _, c := range quizCases {
		t.Run(c.name, func(t *testing.T) {
			sv := db
			sv.lessons = append([]lessonSeed(nil), db.lessons...)
			q := *db.lessons[quizAt].quiz
			q.questions = append([]questionSeed(nil), q.questions...)
			q.questions[0].options = append([]optionSeed(nil), q.questions[0].options...)
			c.mutate(&q)
			sv.lessons[quizAt].quiz = &q
			stageVersions[0] = sv
			t.Cleanup(func() { stageVersions[0] = db })

			err := checkData()
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantMsg)
		})
	}

	hwCases := []struct {
		name    string
		mutate  func(hw *homeworkSeed)
		wantMsg string
	}{
		{"no brief", func(hw *homeworkSeed) { hw.brief = nil }, "thiếu đề bài, tiêu chí hoặc ngưỡng đạt"},
		{"no criteria", func(hw *homeworkSeed) { hw.criteria = nil }, "thiếu đề bài, tiêu chí hoặc ngưỡng đạt"},
		{"criterion over 100", func(hw *homeworkSeed) { hw.criteria = []criterionSeed{{"Đúng", 101}} }, "có tiêu chí sai"},
		{"csv attachment", func(hw *homeworkSeed) { hw.attachments = []string{"hocvien.csv"} }, "file kèm hocvien.csv"},
	}
	for _, c := range hwCases {
		t.Run(c.name, func(t *testing.T) {
			sv := ds
			sv.lessons = append([]lessonSeed(nil), ds.lessons...)
			hw := *ds.lessons[hwAt].homework
			c.mutate(&hw)
			sv.lessons[hwAt].homework = &hw
			stageVersions[1] = sv
			t.Cleanup(func() { stageVersions[1] = ds })

			err := checkData()
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantMsg)
		})
	}

	t.Run("two content kinds", func(t *testing.T) {
		sv := db
		sv.lessons = append([]lessonSeed(nil), db.lessons...)
		sv.lessons[quizAt].homework = ds.lessons[hwAt].homework
		stageVersions[0] = sv
		t.Cleanup(func() { stageVersions[0] = db })

		err := checkData()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `bài "db-practice" sai loại nội dung`)
	})
	require.NoError(t, checkData())
}

// TestCheckDataRejectsBadHomeworkWork: bài nộp và lần chấm mẫu phải đi được qua luật của service homework.
func TestCheckDataRejectsBadHomeworkWork(t *testing.T) {
	origSubs, origDeadlines := submissions, deadlines
	cases := []struct {
		name    string
		mutate  func(subs []submissionSeed) []submissionSeed
		wantMsg string
	}{
		{"skipped attempt", func(subs []submissionSeed) []submissionSeed { subs[1].no = 3; return subs }, "lần nộp phải liên tiếp từ 1"},
		{"resubmit after pass", func(subs []submissionSeed) []submissionSeed {
			return append(subs, submissionSeed{class: "cl-basic01", user: "u-an", lesson: "ds-hw", no: 3, days: 10, repoURL: "https://github.com/a/b"})
		}, "lần trước đã Đạt"},
		{"resubmit before graded", func(subs []submissionSeed) []submissionSeed { subs[1].days = 24; return subs }, "lần trước phải chấm xong"},
		{"points over max", func(subs []submissionSeed) []submissionSeed {
			subs[0].grades = []gradeSeed{{by: "u-gv", days: 23.5, points: []int{41, 25, 5}, comment: "x"}}
			return subs
		}, "điểm tiêu chí 1 ngoài 0–40"},
		{"grader of another class", func(subs []submissionSeed) []submissionSeed {
			subs[0].grades = []gradeSeed{{by: "u-gv2", days: 23.5, points: []int{30, 25, 5}, comment: "x"}}
			return subs
		}, "u-gv2 không chấm được lớp cl-basic01"},
		{"graded before submitted", func(subs []submissionSeed) []submissionSeed {
			subs[0].grades = []gradeSeed{{by: "u-gv", days: 26, points: []int{30, 25, 5}, comment: "x"}}
			return subs
		}, "lần chấm phải sau lần nộp"},
		{"not a member", func(subs []submissionSeed) []submissionSeed { subs[2].user = "u-son"; return subs }, "không phải thành viên lớp"},
		{"csv file", func(subs []submissionSeed) []submissionSeed {
			subs[2].files = []fileSeed{{"diem.csv", 10}}
			return subs
		}, `file "diem.csv" không hợp lệ`},
		{"http repo", func(subs []submissionSeed) []submissionSeed { subs[7].repoURL = "http://github.com/a/b"; return subs },
			"bài nộp cl-basic02/u-phong/ds-hw lần 1"},
		{"not homework", func(subs []submissionSeed) []submissionSeed { subs[7].lesson = "ds-test"; return subs }, `"ds-test" không phải homework`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			subs := slices.Clone(origSubs)
			submissions = c.mutate(subs)
			t.Cleanup(func() { submissions = origSubs })

			err := checkData()
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantMsg)
		})
	}

	t.Run("deadline twice", func(t *testing.T) {
		deadlines = append(slices.Clone(origDeadlines), origDeadlines[0])
		t.Cleanup(func() { deadlines = origDeadlines })

		err := checkData()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "hạn nộp cl-basic01/ds-hw đặt hai lần")
	})
	require.NoError(t, checkData())
}

func TestCheckDataRejectsBadQuizWork(t *testing.T) {
	orig := quizAttempts
	// Chỉ số trong quizAttempts: 0, 1 là An ds-quiz lượt 1, 2; 7, 8 là Cường go-quiz lượt 1, 2; 32 là lượt đang làm.
	cases := []struct {
		name    string
		mutate  func(as []quizAttemptSeed) []quizAttemptSeed
		wantMsg string
	}{
		{"skipped attempt", func(as []quizAttemptSeed) []quizAttemptSeed { as[1].no = 3; return as }, "lượt phải liên tiếp từ 1"},
		{"starts before previous submit", func(as []quizAttemptSeed) []quizAttemptSeed { as[1].days = 9; return as }, "phải bắt đầu sau khi nộp lượt trước"},
		{"after open attempt", func(as []quizAttemptSeed) []quizAttemptSeed {
			return append(as, quizAttemptSeed{class: "cl-basic01", user: "u-bich", lesson: "go-practice", no: 2, days: 0.01, minutes: 1,
				mask: []int{1, 1, 1, 1}})
		}, "lượt trước còn đang làm"},
		{"over attempt limit", func(as []quizAttemptSeed) []quizAttemptSeed {
			return append(as, quizAttemptSeed{class: "cl-basic01", user: "u-cuong", lesson: "go-quiz", no: 3, days: 1, minutes: 5,
				mask: []int{1, 1, 1, 1, 1}})
		}, "quá 2 lượt"},
		{"over time limit", func(as []quizAttemptSeed) []quizAttemptSeed { as[8].minutes = 16; return as }, "nộp sau giới hạn 15 phút"},
		{"open assessment", func(as []quizAttemptSeed) []quizAttemptSeed { as[8].open = true; return as }, "chỉ có lượt đang làm của quiz luyện tập"},
		{"missing answers", func(as []quizAttemptSeed) []quizAttemptSeed { as[0].mask = []int{1, 1}; return as }, "trả lời đủ 5 câu"},
		{"bad mask", func(as []quizAttemptSeed) []quizAttemptSeed { as[0].mask = []int{1, 1, 2, 1, 1}; return as }, "mask câu 3"},
		{"review of open attempt", func(as []quizAttemptSeed) []quizAttemptSeed { as[32].review = review("u-gv", 0.01); return as }, "chỉ lượt đã nộp mới nhận xét được"},
		{"reviewer not teaching the class", func(as []quizAttemptSeed) []quizAttemptSeed { as[0].review = review("u-gv2", 8.95); return as }, "u-gv2 không nhận xét được lớp cl-basic01"},
		{"review before submit", func(as []quizAttemptSeed) []quizAttemptSeed { as[0].review = review("u-gv", 9); return as }, "nhận xét phải sau lúc nộp"},
		{"review after a newer submit", func(as []quizAttemptSeed) []quizAttemptSeed { as[7].review = review("u-gv", 1.15); return as }, "lượt trước phải được xác nhận trước khi nộp lượt này"},
		{"empty comment", func(as []quizAttemptSeed) []quizAttemptSeed {
			as[0].review = &quizReviewSeed{by: "u-gv", days: 8.95, comment: " "}
			return as
		}, "nhận xét chung bắt buộc"},
		{"note outside the quiz", func(as []quizAttemptSeed) []quizAttemptSeed {
			as[0].review = &quizReviewSeed{by: "u-gv", days: 8.95, comment: "Ổn.", notes: map[int]string{5: "Câu không có."}}
			return as
		}, "ghi chú cho câu 6 không thuộc quiz"},
		{"not a member", func(as []quizAttemptSeed) []quizAttemptSeed { as[0].user = "u-son"; return as }, "không phải thành viên lớp"},
		{"not a quiz", func(as []quizAttemptSeed) []quizAttemptSeed { as[0].lesson = "ds-hw"; return as }, `"ds-hw" không phải quiz`},
		{"submitted in the future", func(as []quizAttemptSeed) []quizAttemptSeed { as[32].minutes = 61; return as }, "kết thúc trước thời điểm seed"},
		{"auto submit before time is up", func(as []quizAttemptSeed) []quizAttemptSeed { as[8].auto = true; return as }, "lượt tự nộp phải là lượt assessment đã nộp sau đủ 15 phút"},
		{"auto submit of a practice quiz", func(as []quizAttemptSeed) []quizAttemptSeed { as[0].auto = true; return as }, "lượt tự nộp phải là lượt assessment"},
		{"auto submit of an open attempt", func(as []quizAttemptSeed) []quizAttemptSeed { as[32].auto = true; return as }, "lượt tự nộp phải là lượt assessment"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			quizAttempts = c.mutate(slices.Clone(orig))
			t.Cleanup(func() { quizAttempts = orig })

			err := checkData()
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantMsg)
		})
	}

	t.Run("progress declared in done", func(t *testing.T) {
		origEnrollments := enrollments
		enrollments = slices.Clone(origEnrollments)
		enrollments[0].done = append(slices.Clone(enrollments[0].done), "ds-quiz")
		t.Cleanup(func() { enrollments = origEnrollments })

		err := checkData()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tiến độ quiz suy ra từ lượt làm")
	})
	require.NoError(t, checkData())
}

// review là nhận xét hợp lệ về nội dung của giảng viên by, days ngày trước.
func review(by string, days float64) *quizReviewSeed {
	return &quizReviewSeed{by: by, days: days, comment: "Ổn."}
}

func quizCountByMode() map[string]int {
	n := map[string]int{}
	for _, sv := range stageVersions {
		for _, l := range sv.lessons {
			if l.quiz != nil {
				n[l.quiz.mode]++
			}
		}
	}
	return n
}

func homeworkCount() int {
	n := 0
	for _, sv := range stageVersions {
		for _, l := range sv.lessons {
			if l.homework != nil {
				n++
			}
		}
	}
	return n
}

func videoLessonCount() int {
	n := 0
	for _, sv := range stageVersions {
		for _, l := range sv.lessons {
			if l.typ == "video" {
				n++
			}
		}
	}
	return n
}

func progressCount() int {
	n := 0
	for _, e := range enrollments {
		n += len(e.done) + len(e.opened)
	}
	return n
}

func TestCheckDataRejectsBadStageReviews(t *testing.T) {
	orig := stageReviews
	cases := []struct {
		name    string
		mutate  func(rs []stageReviewSeed)
		wantMsg string
	}{
		{"not a member", func(rs []stageReviewSeed) { rs[0].user = "u-son" }, "không phải thành viên lớp"},
		{"stage outside the course", func(rs []stageReviewSeed) { rs[0].stageVersion = "sv-khong-co" }, "chặng không thuộc khóa học của lớp"},
		{"author not teaching the class", func(rs []stageReviewSeed) { rs[0].by = "u-gv2" }, "người viết phải là giảng viên lớp hoặc admin"},
		{"unknown author", func(rs []stageReviewSeed) { rs[0].by = "u-khong-co" }, `không tìm thấy user "u-khong-co"`},
		{"blank body", func(rs []stageReviewSeed) { rs[0].body = "  " }, "từ 1 đến 5000 ký tự"},
		{"untrimmed body", func(rs []stageReviewSeed) { rs[0].body = " Tốt" }, "từ 1 đến 5000 ký tự"},
		{"body too long", func(rs []stageReviewSeed) { rs[0].body = strings.Repeat("ữ", 5001) }, "từ 1 đến 5000 ký tự"},
		{"duplicate", func(rs []stageReviewSeed) { rs[1].user = rs[0].user }, "trùng"},
		{"before joining the class", func(rs []stageReviewSeed) { rs[0].days = 60 }, "phải viết sau khi vào lớp"},
		{"in the future", func(rs []stageReviewSeed) { rs[0].days = 0 }, "trước thời điểm seed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stageReviews = slices.Clone(orig)
			c.mutate(stageReviews)
			t.Cleanup(func() { stageReviews = orig })

			err := checkData()
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantMsg)
		})
	}
}

// TestCheckDataRejectsBadDrafts: bản nháp theo luật Clone của tính năng chặng.
func TestCheckDataRejectsBadDrafts(t *testing.T) {
	orig := stageDrafts
	cases := []struct {
		name    string
		mutate  func(ds []stageDraftSeed) []stageDraftSeed
		wantMsg string
	}{
		{"unknown source", func(ds []stageDraftSeed) []stageDraftSeed { ds[0].from = "sv-khong-co"; return ds }, `không tìm thấy phiên bản đã phát hành "sv-khong-co"`},
		{"draft of a draft", func(ds []stageDraftSeed) []stageDraftSeed {
			return append(ds, stageDraftSeed{key: "sv-go-3", from: "sv-go-2", no: 3, createdAt: ago(1, 0)})
		}, `không tìm thấy phiên bản đã phát hành "sv-go-2"`},
		{"second draft of a stage", func(ds []stageDraftSeed) []stageDraftSeed {
			return append(ds, stageDraftSeed{key: "sv-go-3", from: "sv-go-1", no: 2, createdAt: ago(1, 0)})
		}, "chặng st-go đã có bản nháp sv-go-2"},
		{"version number gap", func(ds []stageDraftSeed) []stageDraftSeed { ds[0].no = 3; return ds }, "bản nháp sv-go-2 phải là v2"},
		{"before source published", func(ds []stageDraftSeed) []stageDraftSeed { ds[0].createdAt = ago(67, 0); return ds }, "phải nhân bản sau khi sv-go-1 phát hành"},
		{"in the future", func(ds []stageDraftSeed) []stageDraftSeed { ds[0].createdAt = ahead(1); return ds }, "trước thời điểm seed"},
		{"duplicate key", func(ds []stageDraftSeed) []stageDraftSeed { ds[0].key = "sv-db-1"; return ds }, `khóa "sv-db-1" trùng`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stageDrafts = c.mutate(slices.Clone(orig))
			t.Cleanup(func() { stageDrafts = orig })

			err := checkData()
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantMsg)
		})
	}
	require.NoError(t, checkData())
}

func TestEveryInvitedUserHasTemporaryPasswordExpiry(t *testing.T) {
	for _, u := range users {
		invited := u.status == "invited"
		assert.Equal(t, invited, u.mustChangePassword, u.key)
		assert.Equal(t, invited, u.tempPasswordExpiresAt != nil, u.key)
		if invited {
			assert.Nil(t, u.lastLoginAt, u.key)
		}
		assert.Equal(t, normalizeEmail(u.email), u.email, "email trong seed.js đã ở dạng chuẩn hóa: %s", u.key)
	}
}

var allowedTag = regexp.MustCompile(`^(h1|h2|h3|h4|p|br|ul|ol|li|strong|em|del|code|pre|blockquote|hr|table|thead|tbody|tr|th|td)$`)

func TestMarkdownDocs(t *testing.T) {
	tags := regexp.MustCompile(`<(/?)([a-zA-Z0-9]+)([^>]*)>`)
	for _, sv := range stageVersions {
		for _, l := range sv.lessons {
			if l.markdown == nil {
				continue
			}
			md := l.markdown
			assert.NotContains(t, md.source, "ˋ", "%s: còn ký tự thay backtick", l.key)
			assert.True(t, strings.HasPrefix(md.source, "# "), l.key)
			require.NotEmpty(t, md.html, l.key)

			for _, m := range tags.FindAllStringSubmatch(md.html, -1) {
				assert.Regexp(t, allowedTag, m[2], "%s: thẻ ngoài allowlist", l.key)
				if m[3] != "" {
					assert.Regexp(t, `^ class="language-[a-z0-9]+"$`, m[3], "%s: thuộc tính ngoài allowlist", l.key)
				}
			}
			title := strings.TrimPrefix(strings.SplitN(md.source, "\n", 2)[0], "# ")
			assert.Contains(t, md.html, "<h1>"+title+"</h1>", l.key)
		}
	}
	assert.Contains(t, mdTableDesign.source, "`ON DELETE RESTRICT`")
	assert.Contains(t, mdTableDesign.source, "```sql\nCREATE TABLE class_members (")
}

// update khai báo ở file không có build tag để TestSeedMarkdownGolden chạy -update mà không cần DB.
var update = flag.Bool("update", false, "ghi lại golden JSON trong testdata/")

// TestSeedMarkdownGolden ghim nguồn mọi bài markdown seed vào testdata/seed_markdown.json; web dùng file này để
// kiểm trình soạn giữ nguyên nội dung seed thật.
func TestSeedMarkdownGolden(t *testing.T) {
	type entry struct {
		Key    string `json:"key"`
		Source string `json:"source"`
	}
	var docs []entry
	for _, sv := range stageVersions {
		for _, l := range sv.lessons {
			if l.markdown != nil {
				docs = append(docs, entry{Key: l.key, Source: l.markdown.source})
			}
		}
	}
	require.Len(t, docs, 8)

	var got bytes.Buffer
	enc := json.NewEncoder(&got)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	require.NoError(t, enc.Encode(docs))

	path := filepath.Join("testdata", "seed_markdown.json")
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, got.Bytes(), 0o644)) //nolint:gosec // golden JSON công khai trong repo
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // đường dẫn cố định trong testdata
	require.NoError(t, err, "chạy lại với -update")
	assert.Equal(t, string(want), got.String(), "golden seed_markdown.json khác (chạy lại với -update nếu thay đổi là chủ ý)")
}

func TestOffset(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, now.Add(-26*time.Hour), ago(1, 2).at(now))
	assert.Equal(t, now.Add(36*time.Hour), ahead(1.5).at(now))
	assert.Equal(t, now.Add(-time.Duration(0.17*24*float64(time.Hour))), ago(0.17, 0).at(now))
}

func TestValidate(t *testing.T) {
	const pw = "Dev-Password-123"
	cases := []struct {
		name string
		cfg  config.Config
		opts Options
		want error
	}{
		{"dev", config.Config{AppEnv: config.EnvDev, SeedPassword: pw}, Options{}, nil},
		{"e2e reset", config.Config{AppEnv: config.EnvE2E, SeedPassword: pw}, Options{Reset: true}, nil},
		{"production", config.Config{AppEnv: config.EnvProduction, SeedPassword: pw}, Options{}, ErrProduction},
		{"production reset", config.Config{AppEnv: config.EnvProduction, SeedPassword: pw}, Options{Reset: true}, ErrProduction},
		{"reset outside dev and e2e", config.Config{AppEnv: "staging", SeedPassword: pw}, Options{Reset: true}, ErrResetNotAllowed},
		{"missing password", config.Config{AppEnv: config.EnvDev}, Options{}, ErrSeedPassword},
		{"short password", config.Config{AppEnv: config.EnvDev, SeedPassword: "short-pw-11"}, Options{}, ErrSeedPassword},
		{"upload sample", config.Config{AppEnv: config.EnvDev, SeedPassword: pw}, Options{UploadSample: true}, nil},
		{"upload sample production", config.Config{AppEnv: config.EnvProduction, SeedPassword: pw}, Options{UploadSample: true}, ErrProduction},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Validate(c.cfg, c.opts)
			if c.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, c.want)
		})
	}
}
