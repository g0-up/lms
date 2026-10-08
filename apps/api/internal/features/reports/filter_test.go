package reports

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
)

func intp(v int) *int { return &v }

func TestParseSort(t *testing.T) {
	cases := []struct {
		in   string
		want Sort
		ok   bool
	}{
		{"", SortName, true},
		{"name", SortName, true},
		{"pct", SortPct, true},
		{"pct-desc", SortPctDesc, true},
		{"activity", SortActivity, true},
		{"activity-asc", SortActivityAsc, true},
		{"evil", "", false},
		{"name; DROP TABLE users", "", false},
		{"NAME", "", false},
	}
	for _, tc := range cases {
		got, err := ParseSort(tc.in)
		if !tc.ok {
			assert.ErrorIs(t, err, ErrInvalidFilter, tc.in)
			continue
		}
		require.NoError(t, err, tc.in)
		assert.Equal(t, tc.want, got, tc.in)
	}
}

func TestFilterValidate(t *testing.T) {
	cases := []struct {
		name string
		f    Filter
		ok   bool
	}{
		{"mặc định", Filter{Sort: SortName}, true},
		{"inactiveDays 1", Filter{InactiveDays: intp(1), Sort: SortName}, true},
		{"inactiveDays 0", Filter{InactiveDays: intp(0), Sort: SortName}, false},
		{"inactiveDays âm", Filter{InactiveDays: intp(-1), Sort: SortName}, false},
		{"belowPercent 0", Filter{BelowPercent: intp(0), Sort: SortName}, true},
		{"belowPercent 100", Filter{BelowPercent: intp(100), Sort: SortName}, true},
		{"belowPercent 101", Filter{BelowPercent: intp(101), Sort: SortName}, false},
		{"belowPercent âm", Filter{BelowPercent: intp(-1), Sort: SortName}, false},
		{"sort rỗng", Filter{}, false},
		{"sort lạ", Filter{Sort: "evil"}, false},
	}
	for _, tc := range cases {
		err := tc.f.Validate()
		if tc.ok {
			assert.NoError(t, err, tc.name)
		} else {
			assert.ErrorIs(t, err, ErrInvalidFilter, tc.name)
		}
	}
}

func TestErrInvalidFilterIs422(t *testing.T) {
	ae := apperr.FromDomain(ErrInvalidFilter)
	assert.Equal(t, http.StatusUnprocessableEntity, ae.Status)
	assert.Equal(t, apperr.CodeValidationFailed, ae.Code)
	assert.Equal(t, "Tham số lọc không hợp lệ.", ae.Message)
}

func TestFilterMatch(t *testing.T) {
	now := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	day := 24 * time.Hour
	active := func(r ReportRow) ReportRow { r.MemberStatus = domain.MemberActive; return r }

	cases := []struct {
		name string
		f    Filter
		row  ReportRow
		want bool
	}{
		{"không lọc", Filter{}, active(ReportRow{}), true},
		{"dropped bị ẩn mặc định", Filter{}, ReportRow{MemberStatus: domain.MemberDropped}, false},
		{"dropped hiện khi includeDropped", Filter{IncludeDropped: true}, ReportRow{MemberStatus: domain.MemberDropped}, true},
		{"chưa đăng nhập", Filter{NotLoggedIn: true}, active(ReportRow{}), true},
		{"đã đăng nhập", Filter{NotLoggedIn: true}, active(ReportRow{LastLoginAt: ago(day)}), false},
		{"chưa hoạt động thỏa inactive", Filter{InactiveDays: intp(7)}, active(ReportRow{}), true},
		{"đúng N ngày chưa quá", Filter{InactiveDays: intp(7)}, active(ReportRow{LastActivityAt: ago(7 * day)}), false},
		{"N+1 ngày đã quá", Filter{InactiveDays: intp(7)}, active(ReportRow{LastActivityAt: ago(8 * day)}), true},
		{"vừa hoạt động", Filter{InactiveDays: intp(7)}, active(ReportRow{LastActivityAt: ago(time.Hour)}), false},
		{"dưới ngưỡng", Filter{BelowPercent: intp(50)}, active(ReportRow{Percent: 49}), true},
		{"bằng ngưỡng", Filter{BelowPercent: intp(50)}, active(ReportRow{Percent: 50}), false},
		{"belowPercent 0 không ai", Filter{BelowPercent: intp(0)}, active(ReportRow{Percent: 0}), false},
		{
			"kết hợp là giao: thỏa cả ba",
			Filter{NotLoggedIn: true, InactiveDays: intp(7), BelowPercent: intp(30)},
			active(ReportRow{Percent: 0}), true,
		},
		{
			"kết hợp là giao: trượt một điều kiện",
			Filter{NotLoggedIn: true, InactiveDays: intp(7), BelowPercent: intp(30)},
			active(ReportRow{Percent: 40}), false,
		},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.f.Match(tc.row, now), tc.name)
	}
}

func TestFilterApply(t *testing.T) {
	now := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

	sf, err := Filter{Sort: SortName}.Apply(now)
	require.NoError(t, err)
	assert.Equal(t, []any{false, (*time.Time)(nil), (*int)(nil), false}, sf.args)
	assert.Equal(t, `m.name, m.member_id`, sf.orderBy)

	sf, err = Filter{NotLoggedIn: true, InactiveDays: intp(7), BelowPercent: intp(30), IncludeDropped: true, Sort: SortPctDesc}.Apply(now)
	require.NoError(t, err)
	cutoff := now.Add(-7 * 24 * time.Hour)
	assert.Equal(t, []any{true, &cutoff, intp(30), true}, sf.args)
	assert.Equal(t, orderBy[SortPctDesc], sf.orderBy)

	for s, clause := range orderBy {
		sf, err := Filter{Sort: s}.Apply(now)
		require.NoError(t, err)
		assert.Equal(t, clause, sf.orderBy)
		assert.Contains(t, clause, "m.member_id", "thứ tự phải ổn định với %s", s)
	}

	_, err = Filter{Sort: "m.name; DROP TABLE users"}.Apply(now)
	assert.ErrorIs(t, err, ErrInvalidFilter)
}
