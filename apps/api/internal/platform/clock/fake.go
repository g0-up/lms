package clock

import (
	"sync"
	"time"
)

// Fake là đồng hồ cố định cho test: Now trả T cho tới khi Set hoặc Advance. An toàn khi dùng từ nhiều goroutine.
// Dùng qua con trỏ (&clock.Fake{T: ...}) để Set/Advance có hiệu lực với nơi đã nhận Clock.
type Fake struct {
	mu sync.Mutex
	T  time.Time
}

var _ Clock = (*Fake)(nil)

// Now trả thời điểm hiện tại của đồng hồ giả.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.T
}

// Set đặt thời điểm hiện tại.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.T = t
}

// Advance dời thời điểm hiện tại thêm d (d âm thì lùi).
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.T = f.T.Add(d)
}
