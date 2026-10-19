package homework

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
)

func fieldsOf(t *testing.T, err error) map[string]string {
	t.Helper()
	var fe *FieldErrors
	require.True(t, errors.As(err, &fe), "lỗi phải là FieldErrors: %v", err)
	require.ErrorIs(t, err, domain.ErrInvalid)
	return fe.Fields
}

func TestSubmitInputNormalize(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	six := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	cases := []struct {
		name   string
		in     SubmitInput
		fields map[string]string
	}{
		{"repo GitHub", SubmitInput{RepoURL: " https://github.com/an/sql-bai-1 "}, nil},
		{"repo GitLab có đường dẫn con", SubmitInput{RepoURL: "https://gitlab.com/an/sql/-/tree/main"}, nil},
		{"repo Bitbucket, host viết hoa", SubmitInput{RepoURL: "https://BitBucket.org/an/sql"}, nil},
		{"repo www", SubmitInput{RepoURL: "https://www.github.com/an/sql"}, nil},
		{"chỉ file", SubmitInput{MediaIDs: []uuid.UUID{a, b}}, nil},
		{"repo kèm commit", SubmitInput{RepoURL: "https://github.com/an/sql", CommitRef: "v1.0/rc_2-a"}, nil},
		{"đủ 5 file", SubmitInput{MediaIDs: six[:5]}, nil},
		{"rỗng", SubmitInput{RepoURL: "  "}, map[string]string{"repoUrl": ErrNoContent.Msg}},
		{"http", SubmitInput{RepoURL: "http://github.com/an/sql"}, map[string]string{"repoUrl": ErrRepoURL.Msg}},
		{"host khác", SubmitInput{RepoURL: "https://example.com/an/sql"}, map[string]string{"repoUrl": ErrRepoURL.Msg}},
		{"host giả", SubmitInput{RepoURL: "https://github.com.evil.io/an/sql"}, map[string]string{"repoUrl": ErrRepoURL.Msg}},
		{"thiếu repo", SubmitInput{RepoURL: "https://github.com/an"}, map[string]string{"repoUrl": ErrRepoURL.Msg}},
		{"có khoảng trắng", SubmitInput{RepoURL: "https://github.com/an/sql bai"}, map[string]string{"repoUrl": ErrRepoURL.Msg}},
		{"quá dài", SubmitInput{RepoURL: "https://github.com/an/" + strings.Repeat("a", 500)}, map[string]string{"repoUrl": ErrRepoURL.Msg}},
		{"commit không có repo", SubmitInput{CommitRef: "abc", MediaIDs: []uuid.UUID{a}}, map[string]string{"commitRef": ErrCommitNeedsRepo.Msg}},
		{"commit ký tự lạ", SubmitInput{RepoURL: "https://github.com/an/sql", CommitRef: "abc def"}, map[string]string{"commitRef": ErrCommitRef.Msg}},
		{"commit quá 100", SubmitInput{RepoURL: "https://github.com/an/sql", CommitRef: strings.Repeat("a", 101)}, map[string]string{"commitRef": ErrCommitRef.Msg}},
		{"6 file", SubmitInput{MediaIDs: six}, map[string]string{"mediaIds": ErrTooManyFiles.Msg}},
		{"file trùng", SubmitInput{MediaIDs: []uuid.UUID{a, a}}, map[string]string{"mediaIds": ErrDuplicateFile.Msg}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.in.Normalize()
			if tc.fields == nil {
				require.NoError(t, err)
				require.Equal(t, strings.TrimSpace(tc.in.RepoURL), out.RepoURL)
				return
			}
			require.Equal(t, tc.fields, fieldsOf(t, err))
		})
	}
}

func TestResultOfAtThreshold(t *testing.T) {
	// 69.99% và 70% với ngưỡng 70: so bằng số nguyên nên không lệch.
	require.Equal(t, ResultRework, ResultOf(6999, 10000, 70))
	require.Equal(t, ResultPassed, ResultOf(7000, 10000, 70))
	require.Equal(t, ResultPassed, ResultOf(7, 10, 70))
	require.Equal(t, ResultRework, ResultOf(6, 10, 70))
	require.Equal(t, ResultPassed, ResultOf(10, 10, 100))
	require.Equal(t, ResultRework, ResultOf(0, 10, 1))
	require.Equal(t, "Đạt", ResultPassed.Label())
	require.Equal(t, "Cần làm lại", ResultRework.Label())
}

func TestNextStatusOnResubmit(t *testing.T) {
	st := func(s SubmissionStatus) *SubmissionStatus { return &s }
	supersede, err := NextStatusOnResubmit(nil)
	require.NoError(t, err)
	require.False(t, supersede)

	supersede, err = NextStatusOnResubmit(st(StatusPending))
	require.NoError(t, err)
	require.True(t, supersede, "bản đang chờ chấm bị thay")

	supersede, err = NextStatusOnResubmit(st(StatusRework))
	require.NoError(t, err)
	require.False(t, supersede)

	_, err = NextStatusOnResubmit(st(StatusPassed))
	require.ErrorIs(t, err, ErrAlreadyPassed)
	require.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestIsLate(t *testing.T) {
	due := time.Date(2026, 10, 20, 23, 59, 0, 0, time.UTC)
	require.False(t, IsLate(nil, due.AddDate(1, 0, 0)), "không có hạn thì không muộn")
	require.False(t, IsLate(&due, due), "nộp đúng hạn không muộn")
	require.True(t, IsLate(&due, due.Add(time.Nanosecond)))
	require.False(t, IsLate(&due, due.Add(-time.Hour)))
}

func TestNewGrade(t *testing.T) {
	c1 := Criterion{ID: uuid.New(), Description: "Đúng kết quả", MaxPoints: 6}
	c2 := Criterion{ID: uuid.New(), Description: "Trình bày", MaxPoints: 4}
	criteria := []Criterion{c1, c2}
	pts := func(n int) *int { return &n }

	g, err := NewGrade(criteria, 70, []ScoreInput{
		{CriterionID: c2.ID, Points: pts(2), Comment: "  cần đặt tên rõ  "},
		{CriterionID: c1.ID, Points: pts(5)},
	}, "  Tốt ")
	require.NoError(t, err)
	require.Equal(t, 7, g.TotalPoints)
	require.Equal(t, 10, g.MaxPoints)
	require.Equal(t, ResultPassed, g.Result)
	require.Equal(t, "Tốt", g.Comment)
	require.Equal(t, []Score{{CriterionID: c1.ID, Points: 5}, {CriterionID: c2.ID, Points: 2, Comment: "cần đặt tên rõ"}}, g.Scores,
		"điểm theo thứ tự tiêu chí")

	g, err = NewGrade(criteria, 70, []ScoreInput{{CriterionID: c1.ID, Points: pts(6)}, {CriterionID: c2.ID, Points: pts(0)}}, "Thiếu")
	require.NoError(t, err)
	require.Equal(t, ResultRework, g.Result, "6/10 dưới ngưỡng 70%")

	_, err = NewGrade(criteria, 70, []ScoreInput{
		{CriterionID: c1.ID, Points: pts(7)},
		{CriterionID: c2.ID},
		{CriterionID: uuid.New(), Points: pts(1)},
	}, " ")
	require.Equal(t, map[string]string{
		"scores." + c1.ID.String(): `Điểm tiêu chí "Đúng kết quả" phải từ 0 đến 6.`,
		"scores." + c2.ID.String(): `Nhập điểm cho tiêu chí "Trình bày".`,
		"scores":                   ErrUnknownCriterion.Msg,
		"comment":                  ErrCommentRequired.Msg,
	}, fieldsOf(t, err))

	_, err = NewGrade(criteria, 70, []ScoreInput{{CriterionID: c1.ID, Points: pts(-1)}, {CriterionID: c2.ID, Points: pts(1)}},
		strings.Repeat("x", 5001))
	require.Equal(t, map[string]string{
		"scores." + c1.ID.String(): `Điểm tiêu chí "Đúng kết quả" phải từ 0 đến 6.`,
		"comment":                  ErrCommentTooLong.Msg,
	}, fieldsOf(t, err))
}

func TestMemberGate(t *testing.T) {
	require.NoError(t, MemberGate(true, domain.MemberActive, domain.ClassActive))
	require.ErrorIs(t, MemberGate(false, "", domain.ClassActive), ErrNotMember)
	require.ErrorIs(t, MemberGate(true, domain.MemberDropped, domain.ClassActive), ErrMemberDropped)
	require.ErrorIs(t, MemberGate(true, domain.MemberActive, domain.ClassDraft), ErrClassNotStarted)
	require.ErrorIs(t, MemberGate(true, domain.MemberActive, domain.ClassEnded), ErrClassEndedSubmit)
}

func TestStaffCanAccessClass(t *testing.T) {
	teacher, other := uuid.New(), uuid.New()
	require.NoError(t, staffCanAccessClass(Actor{ID: uuid.New(), Role: domain.RoleAdmin}, teacher, ErrNotOwnGrade))
	require.NoError(t, staffCanAccessClass(Actor{ID: teacher, Role: domain.RoleTeacher}, teacher, ErrNotOwnGrade))
	require.ErrorIs(t, staffCanAccessClass(Actor{ID: other, Role: domain.RoleTeacher}, teacher, ErrNotOwnGrade), ErrNotOwnGrade)
	require.ErrorIs(t, staffCanAccessClass(Actor{ID: teacher, Role: domain.RoleStudent}, teacher, ErrNotOwnGrade), domain.ErrForbidden)
}
