package reports

import (
	"net/http"
	"time"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
)

// ErrInvalidFilter là lỗi của mọi tham số lọc/sắp xếp sai; hợp đồng API ghi 422 nên tạo bằng apperr thay vì
// domain Kind Invalid (400).
var ErrInvalidFilter = apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidationFailed, "Tham số lọc không hợp lệ.")

// Sort là cách sắp xếp dòng báo cáo; chỉ nhận giá trị trong orderBy.
type Sort string

// Các cách sắp xếp của báo cáo lớp.
const (
	SortName        Sort = "name"         // tên A → Z (mặc định)
	SortPct         Sort = "pct"          // % toàn khóa thấp → cao
	SortPctDesc     Sort = "pct-desc"     // % toàn khóa cao → thấp
	SortActivity    Sort = "activity"     // hoạt động mới nhất trước, chưa hoạt động cuối
	SortActivityAsc Sort = "activity-asc" // lâu không hoạt động trước, chưa hoạt động đầu
)

// orderBy là allowlist Sort → mệnh đề ORDER BY hằng trên cột của truy vấn Rows; không có chuỗi nào lấy từ request.
// Cùng khóa thì sắp theo tên rồi member_id để thứ tự ổn định.
var orderBy = map[Sort]string{
	SortName:        `m.name, m.member_id`,
	SortPct:         `m.percent ASC, m.name, m.member_id`,
	SortPctDesc:     `m.percent DESC, m.name, m.member_id`,
	SortActivity:    `m.last_activity_at DESC NULLS LAST, m.name, m.member_id`,
	SortActivityAsc: `m.last_activity_at ASC NULLS FIRST, m.name, m.member_id`,
}

// ParseSort đọc tham số sort; rỗng là SortName, giá trị ngoài allowlist là ErrInvalidFilter.
func ParseSort(s string) (Sort, error) {
	if s == "" {
		return SortName, nil
	}
	if _, ok := orderBy[Sort(s)]; !ok {
		return "", ErrInvalidFilter
	}
	return Sort(s), nil
}

// Filter là bộ lọc và sắp xếp của báo cáo lớp. Match là bản Go thuần, Apply dịch sang tham số của truy vấn Rows;
// hai bản phải cho cùng tập dòng.
type Filter struct {
	NotLoggedIn    bool // chưa đăng nhập lần nào
	InactiveDays   *int // không hoạt động quá N ngày (N > 0)
	BelowPercent   *int // % toàn khóa dưới N (0..100)
	IncludeDropped bool // hiện cả thành viên đã rời lớp; mặc định chỉ thành viên active
	Sort           Sort
}

// Validate kiểm biên: inactiveDays phải > 0, belowPercent trong 0..100, sort thuộc allowlist.
func (f Filter) Validate() error {
	if f.InactiveDays != nil && *f.InactiveDays <= 0 {
		return ErrInvalidFilter
	}
	if f.BelowPercent != nil && (*f.BelowPercent < 0 || *f.BelowPercent > 100) {
		return ErrInvalidFilter
	}
	if _, ok := orderBy[f.Sort]; !ok {
		return ErrInvalidFilter
	}
	return nil
}

// inactiveCutoff là mốc "không hoạt động quá N ngày": hoạt động trước mốc (hoặc chưa từng) là thỏa. Đúng N ngày
// trước now thì chưa quá N ngày.
func inactiveCutoff(now time.Time, days int) time.Time {
	return now.Add(-time.Duration(days) * 24 * time.Hour)
}

// Match cho biết dòng r có thuộc kết quả lọc tại thời điểm now không.
func (f Filter) Match(r ReportRow, now time.Time) bool {
	if !f.IncludeDropped && r.MemberStatus != domain.MemberActive {
		return false
	}
	if f.NotLoggedIn && r.LastLoginAt != nil {
		return false
	}
	if f.InactiveDays != nil && r.LastActivityAt != nil && !r.LastActivityAt.Before(inactiveCutoff(now, *f.InactiveDays)) {
		return false
	}
	if f.BelowPercent != nil && r.Percent >= *f.BelowPercent {
		return false
	}
	return true
}

// sqlFilter là phần tham số hóa của truy vấn Rows: args theo thứ tự $3..$6 và ORDER BY lấy từ allowlist.
type sqlFilter struct {
	args    []any
	orderBy string
}

// Apply dịch f sang tham số của truy vấn Rows: $3 notLoggedIn, $4 mốc không hoạt động (NULL = không lọc),
// $5 belowPercent (NULL = không lọc), $6 includeDropped. Sort ngoài allowlist là ErrInvalidFilter.
func (f Filter) Apply(now time.Time) (sqlFilter, error) {
	order, ok := orderBy[f.Sort]
	if !ok {
		return sqlFilter{}, ErrInvalidFilter
	}
	var cutoff *time.Time
	if f.InactiveDays != nil {
		c := inactiveCutoff(now, *f.InactiveDays)
		cutoff = &c
	}
	var below *int
	if f.BelowPercent != nil {
		v := *f.BelowPercent
		below = &v
	}
	return sqlFilter{args: []any{f.NotLoggedIn, cutoff, below, f.IncludeDropped}, orderBy: order}, nil
}
