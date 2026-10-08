// Package clock trừu tượng hóa thời gian hiện tại để use case và seed test được với mốc cố định.
package clock

import "time"

// Clock trả thời điểm hiện tại.
type Clock interface {
	Now() time.Time
}

// Real là đồng hồ hệ thống theo múi giờ nghiệp vụ Loc (nil = giờ địa phương của tiến trình).
type Real struct {
	Loc *time.Location
}

// Now trả time.Now() quy về Loc.
func (r Real) Now() time.Time {
	if r.Loc == nil {
		return time.Now()
	}
	return time.Now().In(r.Loc)
}
