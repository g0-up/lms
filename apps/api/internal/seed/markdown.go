package seed

import (
	"fmt"
	"strings"

	stagefeature "lms/api/internal/features/stages"
)

// markdownDoc là nội dung một bài markdown: nguồn chép nguyên văn từ prototype/seed.js và HTML render bằng đúng
// renderer + sanitizer mà publish của tính năng chặng dùng, nên bài mẫu hiển thị giống bài do admin phát hành.
type markdownDoc struct {
	source string
	html   string
}

var markdownRenderer = stagefeature.NewMarkdownRenderer()

// renderDoc render nguồn markdown cố định trong mã lúc khởi tạo package; lỗi chỉ có thể là lỗi lập trình nên panic
// như regexp.MustCompile.
func renderDoc(source string) markdownDoc {
	html, err := markdownRenderer.Render(source)
	if err != nil {
		panic(fmt.Sprintf("seed: render markdown mẫu: %v", err))
	}
	return markdownDoc{source: source, html: html}
}

// backtick thay ký tự đánh dấu "ˋ" (U+02CB) bằng dấu backtick vì raw string của Go không chứa được backtick.
func backtick(s string) string { return strings.ReplaceAll(s, "ˋ", "`") }

var mdTableDesign = renderDoc(backtick(`# Thiết kế bảng và khóa

Một bảng tốt có khóa chính rõ ràng, kiểu dữ liệu đúng và ràng buộc được khai báo ở tầng database.

## Khóa chính và khóa ngoại

1. Mỗi bảng có đúng một khóa chính.
2. Khóa ngoại tham chiếu tới khóa chính của bảng khác.
3. Dùng ˋON DELETE RESTRICTˋ cho dữ liệu đã được tham chiếu.

ˋˋˋsql
CREATE TABLE class_members (
  id         bigserial PRIMARY KEY,
  class_id   bigint NOT NULL REFERENCES classes(id) ON DELETE RESTRICT,
  user_id    bigint NOT NULL REFERENCES users(id),
  UNIQUE (class_id, user_id)
);
ˋˋˋ`))

var mdIndexes = renderDoc(backtick(`# Đọc thêm: chỉ mục

Chỉ mục (index) giúp truy vấn nhanh hơn nhưng làm chậm ghi. Chỉ đánh index cho cột xuất hiện trong ˋWHEREˋ, ˋJOINˋ hoặc ˋORDER BYˋ thường xuyên.

- B-tree: mặc định, phù hợp so sánh và sắp xếp.
- GIN: cho mảng, JSONB, tìm kiếm toàn văn.`))

var mdStackQueue = renderDoc(backtick(`# Stack và Queue

- **Stack** (ngăn xếp): vào sau ra trước (LIFO). Dùng cho undo, duyệt cây, kiểm tra ngoặc.
- **Queue** (hàng đợi): vào trước ra trước (FIFO). Dùng cho hàng đợi gửi email, BFS.

ˋˋˋgo
stack := []int{}
stack = append(stack, 1)        // push
top := stack[len(stack)-1]      // peek
stack = stack[:len(stack)-1]    // pop
ˋˋˋ`))

var mdGoTypes = renderDoc(backtick(`# Kiểu dữ liệu và hàm trong Go

Go là ngôn ngữ tĩnh kiểu. Khai báo biến bằng ˋvarˋ hoặc ˋ:=ˋ.

ˋˋˋgo
func sum(xs []int) int {
    total := 0
    for _, x := range xs {
        total += x
    }
    return total
}
ˋˋˋ

Hàm có thể trả về nhiều giá trị, thường là ˋ(kết quả, error)ˋ.`))

var mdGoExercise = renderDoc(backtick(`# Bài tập tổng hợp Go

Viết chương trình đọc file CSV danh sách học viên và in ra số học viên theo từng lớp.

Yêu cầu:
1. Dùng package ˋencoding/csvˋ.
2. Xử lý lỗi khi file không tồn tại.
3. Kết quả sắp xếp theo mã lớp.`))

var mdReactState = renderDoc(backtick(`# State và sự kiện

ˋuseStateˋ giữ dữ liệu thay đổi theo thời gian trong component.

ˋˋˋjsx
function Counter() {
  const [count, setCount] = useState(0);
  return <button onClick={() => setCount(count + 1)}>{count}</button>;
}
ˋˋˋ

Khi state đổi, React render lại component đó và các component con.`))

var mdHTMLSemantic = renderDoc(backtick(`# HTML ngữ nghĩa

Dùng đúng thẻ cho đúng ý nghĩa: ˋ<header>ˋ, ˋ<nav>ˋ, ˋ<main>ˋ, ˋ<article>ˋ, ˋ<section>ˋ, ˋ<footer>ˋ.

Lợi ích:
- Trình đọc màn hình hiểu cấu trúc trang.
- Công cụ tìm kiếm đánh giá nội dung tốt hơn.
- CSS và JavaScript dễ bám vào hơn.`))
