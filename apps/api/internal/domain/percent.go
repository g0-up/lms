package domain

import "math"

// Percent tính phần trăm hoàn thành 0..100. math.Round làm tròn nửa ra xa 0, giống round(numeric) của Postgres,
// nên báo cáo tính ở SQL và ở Go cho cùng một số. total <= 0 trả 0.
func Percent(done, total int) int {
	if total <= 0 {
		return 0
	}
	p := int(math.Round(float64(done) * 100 / float64(total)))
	return min(max(p, 0), 100)
}
