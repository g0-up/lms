package seed

import (
	"time"
)

// Dữ liệu mẫu chép từ prototype/seed.js: khóa học "BASIC" v1 dựng từ năm chặng đã phát hành, lớp basic01 và basic02
// đang chạy trên nó, basic03 còn nháp. Mọi mốc thời gian tương đối so với thời điểm seed, giống seed.js.
// Khóa chuỗi (u-admin, sv-db-1, ...) chỉ dùng để nối dữ liệu trong package; id thật là uuid v7 sinh lúc seed.

// offset là khoảng cách so với thời điểm seed, tính bằng giờ; âm là quá khứ.
type offset float64

func ago(days, hours float64) offset { return offset(-(days*24 + hours)) }
func ahead(days float64) offset      { return offset(days * 24) }
func opt(o offset) *offset           { return &o }

func (o offset) at(now time.Time) time.Time {
	return now.Add(time.Duration(float64(o) * float64(time.Hour)))
}

// Mã email outbox của lời mời theo seed.js.
const (
	inviteSent   = "sent"
	inviteFailed = "failed"
	inviteQueued = "queued"
)

// failedInviteError là lỗi SMTP của lời mời thất bại trong seed.js.
const failedInviteError = "Mailbox không tồn tại (550 5.1.1)"

// adminKey là người thực hiện mọi lời mời và hành động audit trong seed.js.
const adminKey = "u-admin"

type userSeed struct {
	key, role, name, email, status string
	mustChangePassword             bool
	lastLoginAt, lastActiveAt      *offset
	tempPasswordExpiresAt          *offset
}

var users = []userSeed{
	{key: "u-admin", role: "admin", name: "Trần Minh Quân", email: "quan.tran@goup.vn", status: "active", lastLoginAt: opt(ago(0, 2)), lastActiveAt: opt(ago(0, 1))},
	{key: "u-gv", role: "teacher", name: "Lê Thu Hương", email: "huong.le@goup.vn", status: "active", lastLoginAt: opt(ago(1, 0)), lastActiveAt: opt(ago(1, 0))},
	{key: "u-gv2", role: "teacher", name: "Phạm Quốc Bảo", email: "bao.pham@goup.vn", status: "active", lastLoginAt: opt(ago(3, 0)), lastActiveAt: opt(ago(3, 0))},
	{key: "u-an", role: "student", name: "Nguyễn Hoàng An", email: "an.nguyen@gmail.com", status: "active", lastLoginAt: opt(ago(0, 5)), lastActiveAt: opt(ago(0, 4))},
	{key: "u-bich", role: "student", name: "Trần Thị Bích", email: "bich.tran@gmail.com", status: "active", lastLoginAt: opt(ago(2, 0)), lastActiveAt: opt(ago(2, 0))},
	{key: "u-cuong", role: "student", name: "Lê Văn Cường", email: "cuong.le@outlook.com", status: "active", lastLoginAt: opt(ago(1, 0)), lastActiveAt: opt(ago(1, 0))},
	{key: "u-dung", role: "student", name: "Phạm Minh Dũng", email: "dung.pham@gmail.com", status: "invited", mustChangePassword: true, tempPasswordExpiresAt: opt(ago(55, 0))},
	{key: "u-ha", role: "student", name: "Hoàng Thu Hà", email: "ha.hoang@gmail.com", status: "active", lastLoginAt: opt(ago(4, 0)), lastActiveAt: opt(ago(4, 0))},
	{key: "u-khang", role: "student", name: "Vũ Đức Khang", email: "khang.vu@gmail.com", status: "active", lastLoginAt: opt(ago(21, 0)), lastActiveAt: opt(ago(21, 0))},
	{key: "u-phong", role: "student", name: "Đặng Hải Phong", email: "phong.dang@gmail.com", status: "active", lastLoginAt: opt(ago(0, 9)), lastActiveAt: opt(ago(0, 8))},
	{key: "u-linh", role: "student", name: "Đỗ Ngọc Linh", email: "linh.do@gmail.com", status: "active", lastLoginAt: opt(ago(1, 0)), lastActiveAt: opt(ago(1, 0))},
	{key: "u-minh", role: "student", name: "Bùi Quang Minh", email: "minh.bui@gmail.com", status: "invited", mustChangePassword: true, tempPasswordExpiresAt: opt(ahead(2))},
	{key: "u-nhan", role: "student", name: "Ngô Thanh Nhàn", email: "nhan.ngo@gmail.com", status: "active", lastLoginAt: opt(ago(6, 0)), lastActiveAt: opt(ago(6, 0))},
	{key: "u-quyen", role: "student", name: "Lý Mai Quyên", email: "quyen.ly@gmail.com", status: "invited", mustChangePassword: true, tempPasswordExpiresAt: opt(ahead(1.5))},
	{key: "u-thao", role: "student", name: "Võ Phương Thảo", email: "thao.vo@gmail.com", status: "disabled", lastLoginAt: opt(ago(15, 0)), lastActiveAt: opt(ago(15, 0))},
	{key: "u-son", role: "student", name: "Trịnh Bảo Sơn", email: "son.trinh@gmail.com", status: "invited", mustChangePassword: true, tempPasswordExpiresAt: opt(ahead(2.8))},
	{key: "u-tu", role: "student", name: "Mai Anh Tú", email: "tu.mai@gmail.com", status: "invited", mustChangePassword: true, tempPasswordExpiresAt: opt(ahead(2.9))},
}

// staffCreatedAt là thời điểm tạo tài khoản admin/giảng viên (seed.js không ghi; trước mọi dữ liệu khác).
var staffCreatedAt = ago(70, 0)

// disabledAt là thời điểm vô hiệu hóa tài khoản disabled, khớp audit user.disabled của seed.js.
var disabledAt = ago(15, 0)

type stageSeed struct{ key, code, name string }

var stages = []stageSeed{
	{"st-db", "DB", "Database"},
	{"st-ds", "DS", "Data structure"},
	{"st-go", "GO", "Golang basic"},
	{"st-re", "RE", "ReactJS basic"},
	{"st-web", "WEB", "HTML CSS JS"},
}

type lessonSeed struct {
	key, typ, title string
	unit            string // khóa bài học chứa học liệu
	required        bool
	video           string // tên file video (bài video)
	duration        string // "mm:ss" (bài video)
	markdown        *markdownDoc
	quiz            *quizSeed
	homework        *homeworkSeed
}

func video(key, title string, required bool, file, duration string) lessonSeed {
	return lessonSeed{key: key, typ: "video", title: title, required: required, video: file, duration: duration}
}

func doc(key, title string, required bool, md *markdownDoc) lessonSeed {
	return lessonSeed{key: key, typ: "markdown", title: title, required: required, markdown: md}
}

// quizSeed là cấu hình và câu hỏi của quiz; quiz luyện tập để trống lượt, thời gian và ngưỡng đạt.
type quizSeed struct {
	mode                       string // practice | assessment
	attempts, minutes, percent int    // chỉ quiz đánh giá
	questions                  []questionSeed
}

// questionSeed là một câu hỏi Q() của seed.js (mọi câu 1 điểm); nội dung và giải thích là markdown.
type questionSeed struct {
	key, typ, body, explanation string
	options                     []optionSeed
}

type optionSeed struct {
	body    string
	correct bool
}

func right(body string) optionSeed { return optionSeed{body: body, correct: true} }
func wrong(body string) optionSeed { return optionSeed{body: body} }

func question(key, typ, body, explanation string, options ...optionSeed) questionSeed {
	return questionSeed{key: key, typ: typ, body: body, explanation: explanation, options: options}
}

func practice(key, title string, required bool, questions ...questionSeed) lessonSeed {
	return lessonSeed{key: key, typ: "quiz", title: title, required: required, quiz: &quizSeed{mode: "practice", questions: questions}}
}

func assessment(key, title string, attempts, minutes, percent int, questions ...questionSeed) lessonSeed {
	return lessonSeed{key: key, typ: "quiz", title: title, required: true,
		quiz: &quizSeed{mode: "assessment", attempts: attempts, minutes: minutes, percent: percent, questions: questions}}
}

// homeworkSeed là đề bài, ngưỡng đạt, rubric và file kèm (tên file của media document mẫu).
type homeworkSeed struct {
	brief       *markdownDoc
	percent     int
	criteria    []criterionSeed
	attachments []string
}

type criterionSeed struct {
	description string
	maxPoints   int
}

func homework(key, title string, hw homeworkSeed) lessonSeed {
	return lessonSeed{key: key, typ: "homework", title: title, required: true, homework: &hw}
}

// in đặt học liệu vào bài học unit của phiên bản.
func (l lessonSeed) in(unit string) lessonSeed {
	l.unit = unit
	return l
}

// unitSeed là bài học của phiên bản chặng; key dùng làm unit_key.
type unitSeed struct{ key, title string }

type stageVersionSeed struct {
	key, stage  string
	no          int
	publishedAt offset
	units       []unitSeed   // theo thứ tự hiển thị
	lessons     []lessonSeed // theo (bài học, thứ tự trong bài học), như lessons.position
}

// finalUnit là tên bài học cuối mỗi chặng, chứa quiz đánh giá (và homework nếu có).
const finalUnit = "Kiểm tra cuối chặng"

// Mọi phiên bản trong stageVersions đều published, không clone từ bản nào; bài học, quiz và homework theo
// group()/place() của seed.js. Bản nháp Go v2 nhân bản từ Go v1 nằm ở stageDrafts.
var stageVersions = []stageVersionSeed{
	{key: "sv-db-1", stage: "st-db", no: 1, publishedAt: ago(70, 0),
		units: []unitSeed{{"ux-db-sql", "Nhập môn SQL"}, {"ux-db-design", "Thiết kế bảng"}, {"ux-db-final", finalUnit}},
		lessons: []lessonSeed{
			video("db-intro", "Giới thiệu SQL", true, "db-01-gioi-thieu-sql.mp4", "18:24").in("ux-db-sql"),
			practice("db-practice", "Luyện tập: Truy vấn SQL cơ bản", true,
				question("qk-db-select", "single", "Lệnh nào dùng để **đọc** dữ liệu từ bảng?",
					"SELECT đọc dữ liệu; INSERT và UPDATE thay đổi dữ liệu; GRANT phân quyền.",
					right("`SELECT`"), wrong("`INSERT`"), wrong("`UPDATE`"), wrong("`GRANT`")),
				question("qk-db-where", "single", "Mệnh đề nào lọc các dòng theo điều kiện?",
					"WHERE giữ lại các dòng thỏa điều kiện trước khi sắp xếp hay gom nhóm.",
					right("`WHERE`"), wrong("`ORDER BY`"), wrong("`GROUP BY`"), wrong("`LIMIT`")),
				question("qk-db-ddl", "multi", "Những lệnh nào thay đổi **cấu trúc** bảng? Chọn tất cả đáp án đúng.",
					"CREATE và ALTER thuộc nhóm DDL. DELETE chỉ xóa dữ liệu, cấu trúc giữ nguyên.",
					right("`CREATE`"), right("`ALTER`"), wrong("`DELETE`"), wrong("`SELECT`")),
				question("qk-db-pk", "single", "Mỗi bảng nên có bao nhiêu khóa chính?",
					"Khóa chính có thể gồm nhiều cột, nhưng mỗi bảng chỉ có một khóa chính.",
					right("Đúng một"), wrong("Không cần khóa chính"), wrong("Hai"), wrong("Tùy số cột")),
			).in("ux-db-sql"),
			doc("db-table", "Thiết kế bảng và khóa", true, &mdTableDesign).in("ux-db-design"),
			doc("db-index", "Đọc thêm: chỉ mục", false, &mdIndexes).in("ux-db-design"),
			assessment("db-test", "Kiểm tra cuối chặng Database", 2, 10, 70,
				question("qk-db-fk", "single", "Khóa ngoại tham chiếu tới cột nào của bảng khác?",
					"Khóa ngoại phải trỏ tới cột định danh duy nhất một dòng, thường là khóa chính.",
					right("Khóa chính (hoặc cột UNIQUE)"), wrong("Cột bất kỳ"), wrong("Cột có index"), wrong("Cột kiểu text")),
				question("qk-db-restrict", "single", "`ON DELETE RESTRICT` làm gì khi xóa một dòng đang được tham chiếu?",
					"RESTRICT chặn việc xóa; CASCADE mới xóa dây chuyền, SET NULL mới đặt về NULL.",
					right("Từ chối lệnh xóa"), wrong("Xóa luôn các dòng tham chiếu"), wrong("Đặt khóa ngoại về NULL"), wrong("Bỏ qua ràng buộc")),
				question("qk-db-unique", "single", "`UNIQUE (class_id, user_id)` đảm bảo điều gì?",
					"Ràng buộc trên cặp cột: cùng một cặp (lớp, học viên) không được lặp lại.",
					right("Một học viên chỉ có một dòng trong mỗi lớp"), wrong("Mỗi lớp chỉ có một học viên"),
					wrong("user_id không trùng trên toàn bảng"), wrong("class_id không được NULL")),
				question("qk-db-index", "multi", "Nên đánh index cho những cột nào? Chọn tất cả đáp án đúng.",
					"Index tăng tốc đọc nhưng làm chậm ghi, nên chỉ đánh cho cột hay dùng để lọc, nối hoặc sắp xếp.",
					right("Cột thường xuất hiện trong `WHERE`"), right("Cột dùng để `JOIN`"), wrong("Mọi cột của bảng"),
					wrong("Cột chỉ được ghi, không bao giờ đọc")),
				question("qk-db-gin", "single", "Loại index nào phù hợp cho cột JSONB?",
					"GIN đánh index từng phần tử bên trong mảng và JSONB.",
					right("GIN"), wrong("B-tree"), wrong("Hash"), wrong("Không đánh index được")),
			).in("ux-db-final"),
		}},
	{key: "sv-ds-1", stage: "st-ds", no: 1, publishedAt: ago(68, 0),
		units: []unitSeed{{"ux-ds-linear", "Mảng, Stack và Queue"}, {"ux-ds-hash", "Hash table"}, {"ux-ds-final", finalUnit}},
		lessons: []lessonSeed{
			video("ds-array", "Mảng và danh sách liên kết", true, "ds-01-mang-danh-sach.mp4", "22:10").in("ux-ds-linear"),
			doc("ds-stack", "Stack và Queue", true, &mdStackQueue).in("ux-ds-linear"),
			video("ds-hash", "Hash table", true, "ds-03-hash-table.mp4", "16:45").in("ux-ds-hash"),
			practice("ds-quiz", "Luyện tập: Stack, Queue và Hash", true,
				question("qk-ds-lifo", "single", "Cấu trúc nào hoạt động theo nguyên tắc **vào sau ra trước** (LIFO)?",
					"Stack lấy ra phần tử được đưa vào sau cùng, như chồng đĩa.",
					right("Stack"), wrong("Queue"), wrong("Hash table"), wrong("Mảng vòng")),
				question("qk-ds-hash-o", "single", "Độ phức tạp **trung bình** khi tra cứu một khóa trong hash table là gì?",
					"Hàm băm tính thẳng vị trí của khóa; trường hợp xấu nhất mới là O(n) khi xung đột nhiều.",
					right("O(1)"), wrong("O(log n)"), wrong("O(n)"), wrong("O(n log n)")),
				question("qk-ds-queue-use", "multi", "Những bài toán nào thường dùng **Queue**? Chọn tất cả đáp án đúng.",
					"BFS và hàng đợi gửi email xử lý theo thứ tự đến trước. Kiểm tra ngoặc và undo dùng stack.",
					right("Duyệt đồ thị theo chiều rộng (BFS)"), right("Hàng đợi gửi email"), wrong("Kiểm tra cặp ngoặc hợp lệ"),
					wrong("Chức năng hoàn tác (undo)")),
				question("qk-ds-pop", "single", "Trong Go, câu lệnh `stack = stack[:len(stack)-1]` làm gì?",
					"Cắt slice bỏ phần tử cuối cùng chính là thao tác pop.",
					right("Bỏ phần tử ở đỉnh stack (pop)"), wrong("Thêm phần tử vào stack (push)"),
					wrong("Đọc phần tử ở đỉnh mà không bỏ (peek)"), wrong("Xóa toàn bộ stack")),
				question("qk-ds-collision", "single", "Hai khóa khác nhau cho cùng một giá trị băm được gọi là gì?",
					"Xung đột được xử lý bằng chaining hoặc open addressing.",
					right("Xung đột (collision)"), wrong("Tràn bộ nhớ"), wrong("Rehash"), wrong("Cân bằng cây")),
			).in("ux-ds-hash"),
			homework("ds-hw", "Bài tập: Cài đặt Stack và Queue", homeworkSeed{brief: &mdDSHomework, percent: 70,
				criteria: []criterionSeed{
					{"Stack đủ Push, Pop, Peek, Len; Pop trên stack rỗng trả lỗi", 40}, {"Queue đúng thứ tự FIFO", 30},
					{"Có test cho trường hợp rỗng và README cách chạy", 30},
				},
				attachments: []string{"ds-stack-queue-starter.zip"}}).in("ux-ds-final"),
			assessment("ds-test", "Kiểm tra cuối chặng Data structure", 2, 10, 70,
				question("qk-ds-bfs", "single", "BFS dùng cấu trúc nào để lưu các đỉnh chờ duyệt?",
					"BFS duyệt theo lớp: đỉnh phát hiện trước được duyệt trước, đúng thứ tự FIFO.",
					right("Queue"), wrong("Stack"), wrong("Hash table"), wrong("Cây nhị phân")),
				question("qk-ds-paren", "single", "Kiểm tra chuỗi `({[]})` có ngoặc hợp lệ nên dùng cấu trúc nào?",
					"Ngoặc mở gần nhất phải đóng trước: đúng nguyên tắc LIFO của stack.",
					right("Stack"), wrong("Queue"), wrong("Mảng đã sắp xếp"), wrong("Hash table")),
				question("qk-ds-linked", "single", "Ưu điểm của danh sách liên kết so với mảng là gì?",
					"Chỉ cần đổi con trỏ; đổi lại, truy cập theo chỉ số là O(n).",
					right("Chèn hoặc xóa ở giữa không phải dời phần tử"), wrong("Truy cập phần tử thứ i trong O(1)"),
					wrong("Tốn ít bộ nhớ hơn"), wrong("Luôn được sắp xếp")),
				question("qk-ds-collide", "multi", "Những cách nào xử lý xung đột trong hash table? Chọn tất cả đáp án đúng.",
					"Chaining nối các khóa trùng vị trí thành danh sách; open addressing tìm ô trống khác.",
					right("Chaining"), right("Open addressing"), wrong("Sắp xếp lại mảng"), wrong("Bỏ qua khóa mới")),
				question("qk-ds-worst", "single", "Khi nào tra cứu trong hash table rơi vào O(n)?",
					"Nếu mọi khóa dồn vào một vị trí, tra cứu phải duyệt cả danh sách.",
					right("Khi rất nhiều khóa xung đột cùng một vị trí"), wrong("Khi bảng rỗng"), wrong("Khi khóa là chuỗi"),
					wrong("Khi dùng chaining")),
			).in("ux-ds-final"),
		}},
	{key: "sv-go-1", stage: "st-go", no: 1, publishedAt: ago(66, 0),
		units: []unitSeed{
			{"ux-go-hello", "Cài đặt và Hello World"}, {"ux-go-basics", "Kiểu dữ liệu và hàm"},
			{"ux-go-concurrency", "Goroutine và channel"}, {"ux-go-final", finalUnit},
		},
		lessons: []lessonSeed{
			video("go-setup", "Cài đặt và Hello World", true, "go-01-cai-dat.mp4", "12:02").in("ux-go-hello"),
			doc("go-hello-read", "Hello World từng bước", false, &mdGoHello).in("ux-go-hello"),
			practice("go-hello-practice", "Luyện tập: Lệnh go và cấu trúc chương trình", false,
				question("qk-go-main", "single", "Chương trình Go bắt đầu chạy từ đâu?",
					"Lệnh go run và go build tìm package main và gọi hàm main.",
					right("Hàm `main` trong `package main`"), wrong("Hàm `init` của mọi package"), wrong("Dòng đầu tiên của file"),
					wrong("Hàm `start`")),
				question("qk-go-mod", "single", "Lệnh nào tạo file `go.mod` cho một dự án mới?",
					"go mod init ghi tên module; go mod tidy chỉ dọn và bổ sung phụ thuộc.",
					right("`go mod init`"), wrong("`go get`"), wrong("`go new`"), wrong("`go mod tidy`")),
				question("qk-go-fmt", "multi", "Những lệnh nào có sẵn trong bộ công cụ Go? Chọn tất cả đáp án đúng.",
					"go fmt, go test và go vet đi kèm Go; linter là công cụ bên ngoài.",
					right("`go fmt`"), right("`go test`"), right("`go vet`"), wrong("`go lint`")),
			).in("ux-go-hello"),
			doc("go-types", "Kiểu dữ liệu và hàm", true, &mdGoTypes).in("ux-go-basics"),
			doc("go-exercise", "Bài tập tổng hợp", false, &mdGoExercise).in("ux-go-basics"),
			video("go-routine", "Goroutine cơ bản", true, "go-03-goroutine.mp4", "25:31").in("ux-go-concurrency"),
			practice("go-practice", "Luyện tập: Goroutine và channel", false,
				question("qk-go-go", "single", "Từ khóa nào chạy một hàm trong goroutine mới?",
					"`go f()` chạy f đồng thời; defer chỉ hoãn lời gọi tới cuối hàm.",
					right("`go`"), wrong("`async`"), wrong("`thread`"), wrong("`defer`")),
				question("qk-go-chan", "single", "Gửi giá trị `v` vào channel `ch` viết thế nào?",
					"Mũi tên chỉ hướng dữ liệu đi: từ v vào ch.",
					right("`ch <- v`"), wrong("`v <- ch`"), wrong("`ch.send(v)`"), wrong("`ch = v`")),
				question("qk-go-wg", "multi", "Những bước nào cần khi dùng `sync.WaitGroup`? Chọn tất cả đáp án đúng.",
					"Add, Done và Wait là đủ; Sleep không cần và không đảm bảo gì.",
					right("Gọi `Add` trước khi chạy goroutine"), right("Gọi `Done` khi goroutine xong"), right("Gọi `Wait` để chờ"),
					wrong("Gọi `time.Sleep` sau `Wait`")),
				question("qk-go-deadlock", "single", "Hàm `main` chỉ có `ch := make(chan int); ch <- 1` sẽ thế nào?",
					"Channel không đệm chặn người gửi cho tới khi có người nhận; không có ai nên runtime báo deadlock.",
					right("Bị deadlock vì không ai nhận"), wrong("In ra 1"), wrong("Chạy xong bình thường"), wrong("Lỗi biên dịch")),
			).in("ux-go-concurrency"),
			// File hocvien.csv của seed.js không có: .csv nằm ngoài các đuôi tài liệu được phép upload.
			homework("go-hw", "Bài tập: Đọc CSV và thống kê", homeworkSeed{brief: &mdGoHomework, percent: 70,
				criteria: []criterionSeed{
					{"Đọc CSV bằng encoding/csv, xử lý lỗi file không tồn tại", 40},
					{"Thống kê đúng theo lớp và sắp xếp theo mã lớp", 40}, {"Code rõ ràng, có README", 20},
				}}).in("ux-go-final"),
			assessment("go-quiz", "Kiểm tra cuối chặng Go", 2, 15, 70,
				question("qk-go-short", "single", "Cú pháp nào **khai báo và gán** biến ngắn gọn bên trong hàm?",
					"Toán tử := vừa khai báo vừa suy ra kiểu, chỉ dùng trong thân hàm.",
					right("`:=`"), wrong("`=`"), wrong("`var x`"), wrong("`let x =`")),
				question("qk-go-error", "single", "Theo quy ước, hàm Go báo lỗi bằng cách nào?",
					"Quy ước (kết quả, error) giúp người gọi xử lý lỗi tường minh.",
					right("Trả về giá trị cuối cùng có kiểu `error`"), wrong("Gọi `panic` cho mọi lỗi"), wrong("Ném exception"),
					wrong("Ghi lỗi vào biến toàn cục")),
				question("qk-go-sync", "multi", "Những cách nào dùng để **đồng bộ** các goroutine? Chọn tất cả đáp án đúng.",
					"WaitGroup và channel là cơ chế đồng bộ; Sleep chỉ đoán thời gian, không đảm bảo thứ tự.",
					right("`sync.WaitGroup`"), right("Channel"), wrong("`time.Sleep`"), wrong("Biến toàn cục kiểu bool")),
				question("qk-go-range-int", "single", "Vòng lặp `for i := range 3` hợp lệ từ phiên bản Go nào?",
					"Range trên số nguyên được thêm vào Go 1.22.",
					right("Go 1.22"), wrong("Go 1.18"), wrong("Go 1.20"), wrong("Go 1.21")),
				question("qk-go-run", "single", "Lệnh nào biên dịch và chạy chương trình mà **không** để lại file nhị phân?",
					"go run biên dịch vào thư mục tạm rồi chạy.",
					right("`go run`"), wrong("`go build`"), wrong("`go install`"), wrong("`go mod tidy`")),
			).in("ux-go-final"),
		}},
	{key: "sv-re-1", stage: "st-re", no: 1, publishedAt: ago(64, 0),
		units: []unitSeed{{"ux-re-basics", "Component, props và state"}, {"ux-re-hooks", "Hooks"}, {"ux-re-final", finalUnit}},
		lessons: []lessonSeed{
			video("re-component", "Component và props", true, "re-01-component.mp4", "20:15").in("ux-re-basics"),
			doc("re-state", "State và sự kiện", true, &mdReactState).in("ux-re-basics"),
			practice("re-practice", "Luyện tập: Component, props và state", true,
				question("qk-re-props", "single", "Props trong React là gì?",
					"Props chỉ đọc ở component con; muốn đổi thì component cha đổi.",
					right("Dữ liệu component cha truyền xuống component con"), wrong("State nội bộ của component"),
					wrong("Hàm gọi API"), wrong("Thuộc tính CSS")),
				question("qk-re-setstate", "single", "Gọi `setCount(count + 1)` sẽ làm gì?",
					"Hàm set lên lịch cập nhật; React render lại với giá trị mới.",
					right("Cập nhật state và render lại component"), wrong("Đổi biến count ngay mà không render"),
					wrong("Tải lại trang"), wrong("Không làm gì")),
				question("qk-re-key", "single", "Khi render một danh sách, mỗi phần tử cần prop nào?",
					"key giúp React nhận ra phần tử nào thêm, bớt hay đổi chỗ.",
					right("`key`"), wrong("`id`"), wrong("`index`"), wrong("`ref`")),
				question("qk-re-immut", "multi", "Cách nào cập nhật mảng trong state đúng? Chọn tất cả đáp án đúng.",
					"Luôn tạo mảng mới; sửa trực tiếp mảng cũ thì React không biết để render lại.",
					right("`setItems([...items, x])`"), right("`setItems(items.filter(fn))`"), wrong("`items.push(x)`"),
					wrong("`items[0] = x`")),
			).in("ux-re-basics"),
			video("re-hooks", "Hooks", true, "re-03-hooks.mp4", "28:40").in("ux-re-hooks"),
			assessment("re-test", "Kiểm tra cuối chặng ReactJS", 2, 15, 70,
				question("qk-re-effect", "single", "`useEffect(fn, [])` chạy `fn` khi nào?",
					"Mảng phụ thuộc rỗng nghĩa là không có gì thay đổi để chạy lại.",
					right("Một lần sau lần render đầu tiên"), wrong("Sau mỗi lần render"), wrong("Trước lần render đầu tiên"),
					wrong("Không bao giờ")),
				question("qk-re-hookrule", "single", "Quy tắc nào đúng khi dùng hook?",
					"React dựa vào thứ tự gọi hook ở mỗi lần render, nên thứ tự đó không được đổi.",
					right("Chỉ gọi hook ở cấp cao nhất của component"), wrong("Gọi hook trong vòng lặp"),
					wrong("Gọi hook trong điều kiện if"), wrong("Gọi hook trong hàm thường")),
				question("qk-re-lift", "single", "Hai component anh em cần dùng chung một state thì làm gì?",
					"Lifting state up: cha giữ state và truyền xuống bằng props.",
					right("Đưa state lên component cha chung"), wrong("Sao chép state vào cả hai"), wrong("Dùng biến toàn cục"),
					wrong("Đọc DOM của nhau")),
				question("qk-re-memo", "multi",
					"Những hook nào giúp tránh tính lại hoặc tạo lại giá trị không cần thiết? Chọn tất cả đáp án đúng.",
					"useMemo nhớ kết quả tính toán, useCallback nhớ hàm giữa các lần render.",
					right("`useMemo`"), right("`useCallback`"), wrong("`useState`"), wrong("`useEffect`")),
				question("qk-re-controlled", "single", "Input \"được điều khiển\" (controlled) là input như thế nào?",
					"State là nguồn sự thật duy nhất của giá trị input.",
					right("Giá trị lấy từ state và đổi qua `onChange`"), wrong("Không có thuộc tính value"),
					wrong("Đọc giá trị bằng `document.querySelector`"), wrong("Chỉ dùng trong form submit")),
			).in("ux-re-final"),
		}},
	{key: "sv-web-1", stage: "st-web", no: 1, publishedAt: ago(62, 0),
		units: []unitSeed{{"ux-web-layout", "HTML và CSS"}, {"ux-web-dom", "JavaScript và DOM"}, {"ux-web-final", finalUnit}},
		lessons: []lessonSeed{
			doc("web-html", "HTML ngữ nghĩa", true, &mdHTMLSemantic).in("ux-web-layout"),
			video("web-css", "CSS layout", true, "web-02-css-layout.mp4", "24:05").in("ux-web-layout"),
			practice("web-practice", "Luyện tập: HTML ngữ nghĩa và CSS layout", true,
				question("qk-web-nav", "single", "Thẻ nào dành cho khối điều hướng chính?",
					"nav báo cho trình đọc màn hình đây là vùng điều hướng.",
					right("`<nav>`"), wrong("`<div class=\"menu\">`"), wrong("`<section>`"), wrong("`<aside>`")),
				question("qk-web-flex", "single", "Thuộc tính nào căn các phần tử theo trục chính trong flexbox?",
					"justify-content theo trục chính, align-items theo trục phụ.",
					right("`justify-content`"), wrong("`align-items`"), wrong("`text-align`"), wrong("`vertical-align`")),
				question("qk-web-a11y", "multi", "HTML ngữ nghĩa mang lại lợi ích gì? Chọn tất cả đáp án đúng.",
					"Thẻ đúng nghĩa giúp máy hiểu trang; tốc độ và giao diện không đổi.",
					right("Trình đọc màn hình hiểu cấu trúc trang"), right("Công cụ tìm kiếm đánh giá nội dung tốt hơn"),
					wrong("Trang tải nhanh gấp đôi"), wrong("Không cần viết CSS nữa")),
				question("qk-web-grid", "single", "`grid-template-columns: repeat(3, 1fr)` tạo ra gì?",
					"1fr là một phần không gian còn lại; ba cột chia đều.",
					right("Ba cột rộng bằng nhau"), wrong("Ba hàng cao bằng nhau"), wrong("Một cột rộng 3fr"), wrong("Lưới 3×3")),
			).in("ux-web-layout"),
			video("web-js", "JavaScript và DOM", true, "web-03-js-dom.mp4", "19:50").in("ux-web-dom"),
			assessment("web-test", "Kiểm tra cuối chặng HTML CSS JS", 3, 15, 80,
				question("qk-web-dom", "single", "Lấy phần tử **đầu tiên** khớp selector `.card` bằng cách nào?",
					"querySelector trả về phần tử đầu tiên; querySelectorAll trả về cả danh sách.",
					right("`document.querySelector('.card')`"), wrong("`document.querySelectorAll('.card')`"),
					wrong("`document.getElementsByClassName('card')`"), wrong("`document.getElementById('card')`")),
				question("qk-web-event", "single", "Gắn sự kiện click cho nút `btn` đúng cách là?",
					"addEventListener cho phép gắn nhiều listener và gỡ ra khi cần.",
					right("`btn.addEventListener('click', fn)`"), wrong("`btn.click = fn()`"), wrong("`btn.on('click')`"),
					wrong("`btn.event = 'click'`")),
				question("qk-web-main", "single", "Mỗi trang nên có bao nhiêu thẻ `<main>` đang hiển thị?",
					"main đánh dấu nội dung chính duy nhất của trang.",
					right("Một"), wrong("Không giới hạn"), wrong("Một cho mỗi section"), wrong("Không cần thẻ main")),
				question("qk-web-box", "multi", "Những thành phần nào thuộc box model của CSS? Chọn tất cả đáp án đúng.",
					"Box model gồm content, padding, border và margin; z-index là thứ tự xếp lớp.",
					right("content"), right("padding"), right("border"), wrong("z-index")),
				question("qk-web-delegation", "single", "Event delegation là gì?",
					"Sự kiện nổi bọt lên cha, nên một listener ở cha xử lý được cả các con thêm sau.",
					right("Gắn một listener ở phần tử cha để xử lý sự kiện của các con"), wrong("Gắn listener cho từng phần tử con"),
					wrong("Chặn sự kiện lan ra ngoài"), wrong("Gọi sự kiện bằng `setTimeout`")),
			).in("ux-web-final"),
		}},
}

// stageDraftSeed là bản nháp nhân bản từ một phiên bản đã phát hành, như Clone của tính năng chặng: cùng bài học,
// học liệu, câu hỏi, rubric và file kèm (id mới, khóa giữ nguyên), HTML render sẵn chép nguyên. Admin nhân bản lúc
// createdAt, kèm audit stage_version.cloned.
type stageDraftSeed struct {
	key, from string
	no        int
	createdAt offset
}

var stageDrafts = []stageDraftSeed{
	{key: "sv-go-2", from: "sv-go-1", no: 2, createdAt: ago(2.8, 0)},
}

// seedDocumentSizeBytes là kích thước giữ chỗ của file kèm mẫu; như video mẫu, object không có trong storage.
const seedDocumentSizeBytes = 4 << 10

// seedVideoSizeBytes là kích thước giữ chỗ của bản ghi video mẫu: file thật không có trong object storage
// (video mẫu chỉ được tải lên khi chạy seed với cờ upload-sample), còn cột size_bytes bắt buộc > 0.
const seedVideoSizeBytes = 1 << 20

type courseSeed struct{ key, code, name string }

var courses = []courseSeed{
	{"co-basic", "BASIC", "Lập trình cơ bản"},
}

type courseVersionSeed struct {
	key, course   string
	no            int
	publishedAt   offset
	stageVersions []string // theo thứ tự position
}

var courseVersions = []courseVersionSeed{
	{key: "cv-basic-1", course: "co-basic", no: 1, publishedAt: ago(60, 0),
		stageVersions: []string{"sv-db-1", "sv-ds-1", "sv-go-1", "sv-re-1", "sv-web-1"}},
}

type classSeed struct {
	key, code, name, courseVersion, status, teacher string
	startDate, endDate, createdAt                   offset
}

var classes = []classSeed{
	{key: "cl-basic01", code: "basic01", name: "Lập trình cơ bản – khóa 1", courseVersion: "cv-basic-1", status: "active",
		startDate: ago(58, 0), endDate: ahead(55), teacher: "u-gv", createdAt: ago(60, 0)},
	{key: "cl-basic02", code: "basic02", name: "Lập trình cơ bản – khóa 2", courseVersion: "cv-basic-1", status: "active",
		startDate: ago(20, 0), endDate: ahead(95), teacher: "u-gv2", createdAt: ago(24, 0)},
	{key: "cl-basic03", code: "basic03", name: "Lập trình cơ bản – khóa 3", courseVersion: "cv-basic-1", status: "draft",
		startDate: ahead(28), endDate: ahead(140), teacher: "u-gv", createdAt: ago(3, 0)},
}

// enrollment là một lần enroll() của seed.js: thành viên lớp, lời mời kèm email outbox, và tiến độ done()/opened().
type enrollment struct {
	class, user  string
	invite       string  // inviteSent | inviteFailed | inviteQueued
	invitedDays  float64 // số ngày trước thời điểm seed
	memberStatus string
	done         []string // bài đã hoàn thành, theo thứ tự trong seed.js
	doneLastDays float64
	opened       []string // bài đã mở nhưng chưa hoàn thành
	openedDays   float64
}

var enrollments = []enrollment{
	// basic01 — tuần thứ tám
	{class: "cl-basic01", user: "u-an", invite: inviteSent, invitedDays: 58, memberStatus: "active",
		done:         []string{"db-intro", "db-table", "db-index", "ds-array", "ds-stack", "ds-hash", "go-setup", "go-types", "go-routine", "re-component", "re-state"},
		doneLastDays: 0.2, opened: []string{"re-hooks"}, openedDays: 0.17},
	{class: "cl-basic01", user: "u-bich", invite: inviteSent, invitedDays: 58, memberStatus: "active",
		done:         []string{"db-intro", "db-table", "ds-array", "ds-stack", "ds-hash", "go-setup", "go-types"},
		doneLastDays: 2, opened: []string{"go-routine"}, openedDays: 2},
	{class: "cl-basic01", user: "u-cuong", invite: inviteSent, invitedDays: 58, memberStatus: "active",
		done:         []string{"db-intro", "db-table", "ds-array", "ds-stack", "ds-hash", "go-setup", "go-types", "go-routine", "go-exercise", "re-component", "re-state", "re-hooks", "web-html", "web-css"},
		doneLastDays: 1, opened: []string{"web-js"}, openedDays: 1},
	// Chưa đăng nhập lần nào; mật khẩu tạm đã hết hạn.
	{class: "cl-basic01", user: "u-dung", invite: inviteSent, invitedDays: 58, memberStatus: "active"},
	{class: "cl-basic01", user: "u-ha", invite: inviteSent, invitedDays: 57, memberStatus: "active",
		done: []string{"db-intro", "db-table", "ds-array", "ds-stack"}, doneLastDays: 4, opened: []string{"ds-hash"}, openedDays: 4},
	{class: "cl-basic01", user: "u-khang", invite: inviteSent, invitedDays: 57, memberStatus: "active",
		done: []string{"db-intro", "db-table"}, doneLastDays: 21},
	{class: "cl-basic01", user: "u-thao", invite: inviteSent, invitedDays: 57, memberStatus: "dropped",
		done: []string{"db-intro"}, doneLastDays: 15},

	// basic02 — tuần thứ ba
	{class: "cl-basic02", user: "u-phong", invite: inviteSent, invitedDays: 22, memberStatus: "active",
		done:         []string{"db-intro", "db-table", "ds-array", "ds-stack", "ds-hash", "go-setup"},
		doneLastDays: 0.3, opened: []string{"go-types"}, openedDays: 0.3},
	{class: "cl-basic02", user: "u-linh", invite: inviteSent, invitedDays: 22, memberStatus: "active",
		done: []string{"db-intro", "db-table", "ds-array"}, doneLastDays: 1},
	{class: "cl-basic02", user: "u-minh", invite: inviteFailed, invitedDays: 22, memberStatus: "active"},
	{class: "cl-basic02", user: "u-nhan", invite: inviteSent, invitedDays: 22, memberStatus: "active",
		done: []string{"db-intro"}, doneLastDays: 6, opened: []string{"db-table"}, openedDays: 6},
	{class: "cl-basic02", user: "u-quyen", invite: inviteSent, invitedDays: 1.5, memberStatus: "active"},
	// Phong vào basic01 muộn, là lớp thứ hai của anh (tài khoản đã có được mời lại).
	{class: "cl-basic01", user: "u-phong", invite: inviteSent, invitedDays: 12, memberStatus: "active",
		done: []string{"db-intro", "db-table", "ds-array", "ds-stack"}, doneLastDays: 2},

	// basic03 — nháp, đã gửi hai lời mời
	{class: "cl-basic03", user: "u-son", invite: inviteSent, invitedDays: 0.2, memberStatus: "active"},
	{class: "cl-basic03", user: "u-tu", invite: inviteQueued, invitedDays: 0.1, memberStatus: "active"},
}

// droppedAt là thời điểm thành viên rời lớp (seed.js không ghi; trùng lúc tài khoản bị vô hiệu hóa).
var droppedAt = disabledAt

// doneStepDays là khoảng cách giữa hai bài hoàn thành liên tiếp trong done() của seed.js.
const doneStepDays = 1.3

// auditSeed là một dòng audit của seed.js với action theo tập chuẩn của ứng dụng.
// members (nếu có) là học viên được mời/gửi lại lời mời trong lớp đích.
type auditSeed struct {
	days               float64
	action, targetType string
	target             string
	before, after      map[string]any
	members            []string
	invitationKind     string
}

var audits = []auditSeed{
	{days: 70, action: "stage_version.published", targetType: "stage_version", target: "sv-db-1",
		before: map[string]any{"status": "draft"}, after: map[string]any{"status": "published"}},
	{days: 60, action: "course_version.published", targetType: "course_version", target: "cv-basic-1",
		before: map[string]any{"status": "draft"}, after: map[string]any{"status": "published"}},
	{days: 58, action: "class.activated", targetType: "class", target: "cl-basic01",
		before: map[string]any{"status": "draft"}, after: map[string]any{"status": "active"}},
	{days: 22, action: "class.member_invited", targetType: "class", target: "cl-basic02",
		members: []string{"u-phong", "u-linh", "u-minh", "u-nhan", "u-quyen"}, invitationKind: "invite"},
	{days: 20, action: "class.activated", targetType: "class", target: "cl-basic02",
		before: map[string]any{"status": "draft"}, after: map[string]any{"status": "active"}},
	{days: 15, action: "user.disabled", targetType: "user", target: "u-thao",
		before: map[string]any{"status": "active"}, after: map[string]any{"status": "disabled"}},
	{days: 12, action: "class.member_invited", targetType: "class", target: "cl-basic01",
		members: []string{"u-phong"}, invitationKind: "invite"},
	{days: 3, action: "class.created", targetType: "class", target: "cl-basic03",
		after: map[string]any{"status": "draft"}},
	{days: 1.5, action: "class.invitation_resent", targetType: "class", target: "cl-basic02",
		members: []string{"u-quyen"}, invitationKind: "resend"},
	{days: 0.2, action: "class.member_invited", targetType: "class", target: "cl-basic03",
		members: []string{"u-son", "u-tu"}, invitationKind: "invite"},
}

// deadlineSeed là hạn nộp homework của một lớp theo seed.js; setBy đặt hạn lúc setDays ngày trước.
type deadlineSeed struct {
	class, lesson string
	due           offset
	setBy         string
	setDays       float64
}

var deadlines = []deadlineSeed{
	{class: "cl-basic01", lesson: "ds-hw", due: ago(20, 0), setBy: "u-admin", setDays: 21},
	{class: "cl-basic01", lesson: "go-hw", due: ahead(0.8), setBy: "u-gv", setDays: 2.5},
	{class: "cl-basic02", lesson: "ds-hw", due: ahead(5), setBy: "u-gv2", setDays: 10},
}

// submissionSeed là một lần submit() của seed.js. Trạng thái suy ra như ứng dụng: theo kết quả lần chấm cuối, hoặc
// superseded khi có lần nộp sau mà bản này chưa chấm, còn lại pending. Muộn khi nộp sau hạn của lớp.
type submissionSeed struct {
	class, user, lesson string
	no                  int
	days                float64
	repoURL, commitRef  string
	files               []fileSeed
	grades              []gradeSeed // theo thời gian; từ lần thứ hai là sửa điểm
}

// fileSeed là file học viên nộp; sizeBytes giữ chỗ tới khi seed --upload-sample tải file mẫu và sửa lại.
type fileSeed struct {
	name      string
	sizeBytes int64
}

// gradeSeed là một lần grade() của seed.js: điểm và nhận xét theo thứ tự tiêu chí.
type gradeSeed struct {
	by      string
	days    float64
	points  []int
	comment string
	notes   []string
}

var submissions = []submissionSeed{
	{class: "cl-basic01", user: "u-an", lesson: "ds-hw", no: 1, days: 25,
		repoURL: "https://github.com/hoangan-dev/ds-stack-queue", commitRef: "4e1c2a7",
		grades: []gradeSeed{{by: "u-gv", days: 23.5, points: []int{30, 25, 5},
			comment: "Stack và Queue chạy đúng nhưng thiếu test cho trường hợp rỗng và chưa có README. Bổ sung rồi nộp lại nhé.",
			notes:   []string{"Pop trên stack rỗng đang panic, cần trả về error.", "", "Chưa có test và README."}}}},
	// Nộp lại sau hạn (hạn basic01 là 20 ngày trước).
	{class: "cl-basic01", user: "u-an", lesson: "ds-hw", no: 2, days: 18,
		repoURL: "https://github.com/hoangan-dev/ds-stack-queue", commitRef: "b93f0d1",
		grades: []gradeSeed{{by: "u-gv", days: 17, points: []int{40, 28, 26}, comment: "Đã sửa đủ các điểm lần trước. Test rõ ràng."}}},
	{class: "cl-basic01", user: "u-bich", lesson: "ds-hw", no: 1, days: 4.5, files: []fileSeed{{"stack-queue-bich.zip", 184320}}},
	{class: "cl-basic01", user: "u-cuong", lesson: "ds-hw", no: 1, days: 22, repoURL: "https://gitlab.com/cuongle/ds-go", commitRef: "v0.1"},
	// Lần chấm đầu quên điểm README; giảng viên sửa điểm nửa ngày sau.
	{class: "cl-basic01", user: "u-cuong", lesson: "ds-hw", no: 2, days: 21.5, repoURL: "https://gitlab.com/cuongle/ds-go", commitRef: "v0.2",
		grades: []gradeSeed{
			{by: "u-gv", days: 21, points: []int{38, 30, 14}, comment: "Làm tốt. Chưa thấy README."},
			{by: "u-gv", days: 20.5, points: []int{38, 30, 24}, comment: "Làm tốt. README nên ghi thêm phiên bản Go yêu cầu."},
		}},
	{class: "cl-basic01", user: "u-cuong", lesson: "go-hw", no: 1, days: 0.5, repoURL: "https://gitlab.com/cuongle/go-csv-stats",
		files: []fileSeed{{"ket-qua-chay.png", 96256}}},
	{class: "cl-basic01", user: "u-phong", lesson: "ds-hw", no: 1, days: 1, files: []fileSeed{{"phong-ds.zip", 212992}, {"README.md", 2150}}},
	{class: "cl-basic02", user: "u-phong", lesson: "ds-hw", no: 1, days: 3.5, repoURL: "https://github.com/haiphong/ds-basic02"},
	{class: "cl-basic02", user: "u-linh", lesson: "ds-hw", no: 1, days: 0.5, repoURL: "https://bitbucket.org/linhdo/stack-queue", commitRef: "main"},
	// Bản nộp mới nhất bị chấm Cần làm lại và chưa nộp lại: ô tổng kết "Cần làm lại" của basic02.
	{class: "cl-basic02", user: "u-nhan", lesson: "ds-hw", no: 1, days: 4, repoURL: "https://github.com/thanhnhan/ds-stack-queue", commitRef: "a1c9e30",
		grades: []gradeSeed{{by: "u-gv2", days: 3, points: []int{20, 25, 0},
			comment: "Queue đúng thứ tự nhưng Pop trên stack rỗng còn panic và chưa có test. Sửa rồi nộp lại trước hạn nhé.",
			notes:   []string{"Pop phải trả về error khi stack rỗng.", "", "Chưa có test và README."}}}},
}

// quizAttemptSeed là một lần attempt() của seed.js: bắt đầu days ngày trước, làm trong minutes phút. mask theo thứ tự
// câu soạn: 1 chọn đủ đáp án đúng, 0 chọn phương án sai đầu tiên; câu trả lời được lưu rải đều trong lúc làm. Thứ tự
// câu và phương án là thứ tự soạn như seed.js. open là lượt đang làm, mask chỉ phủ các câu đã lưu. review là nhận
// xét của giảng viên theo review() của seed.js. Trạng thái xác nhận suy ra như ứng dụng: lượt có nhận xét là confirmed
// (quiz luyện tập là "Đã nhận xét"); lượt assessment chưa nhận xét là superseded khi có lượt nộp sau, còn lại pending.
// auto là lượt assessment hệ thống tự nộp lúc hết giờ: minutes bằng giới hạn thời gian, nộp đúng lúc hết hạn.
type quizAttemptSeed struct {
	class, user, lesson string
	no                  int
	days, minutes       float64
	mask                []int
	open, auto          bool
	review              *quizReviewSeed
}

// quizReviewSeed là một lần nhận xét: giảng viên by nhận xét days ngày trước; notes là ghi chú theo chỉ số câu soạn
// (từ 0).
type quizReviewSeed struct {
	by      string
	days    float64
	comment string
	notes   map[int]string
}

var quizAttempts = []quizAttemptSeed{
	{class: "cl-basic01", user: "u-an", lesson: "ds-quiz", no: 1, days: 9, minutes: 6, mask: []int{0, 1, 1, 0, 1},
		review: &quizReviewSeed{by: "u-gv", days: 8.95,
			comment: "Lượt đầu bạn nhầm stack với queue ở câu 1. Đọc lại bài Stack và Queue rồi làm lại, lượt sau sẽ ổn.",
			notes:   map[int]string{0: "LIFO là stack: phần tử vào sau cùng được lấy ra trước.", 3: "Cắt slice bỏ phần tử cuối là pop, không phải peek."}}},
	{class: "cl-basic01", user: "u-an", lesson: "ds-quiz", no: 2, days: 8.8, minutes: 4, mask: []int{1, 1, 1, 1, 1}},
	{class: "cl-basic01", user: "u-bich", lesson: "ds-quiz", no: 1, days: 2.2, minutes: 7, mask: []int{1, 0, 0, 1, 0}},
	{class: "cl-basic01", user: "u-cuong", lesson: "ds-quiz", no: 1, days: 6, minutes: 5, mask: []int{1, 1, 1, 1, 1}},
	{class: "cl-basic01", user: "u-phong", lesson: "ds-quiz", no: 1, days: 2.1, minutes: 6, mask: []int{1, 1, 0, 1, 1}},
	{class: "cl-basic02", user: "u-phong", lesson: "ds-quiz", no: 1, days: 0.4, minutes: 5, mask: []int{1, 1, 1, 1, 0}},
	{class: "cl-basic01", user: "u-an", lesson: "go-quiz", no: 1, days: 0.3, minutes: 12, mask: []int{1, 1, 0, 0, 1}},
	{class: "cl-basic01", user: "u-cuong", lesson: "go-quiz", no: 1, days: 1.4, minutes: 14, mask: []int{1, 0, 0, 0, 1}},
	{class: "cl-basic01", user: "u-cuong", lesson: "go-quiz", no: 2, days: 1.2, minutes: 13, mask: []int{1, 1, 1, 0, 0},
		review: &quizReviewSeed{by: "u-gv", days: 1.1,
			comment: "Bạn nắm chắc cú pháp, cách báo lỗi và đồng bộ goroutine. Hai câu cuối sai do nhầm phiên bản Go và lệnh chạy; ôn lại bài Cài đặt và Hello World trước khi xin thêm lượt.",
			notes:   map[int]string{3: "Range trên số nguyên có từ Go 1.22, xem ghi chú phát hành.", 4: "go build để lại file nhị phân; go run thì không."}}},
	{class: "cl-basic01", user: "u-an", lesson: "db-practice", no: 1, days: 50, minutes: 4, mask: []int{1, 1, 0, 1}},
	{class: "cl-basic01", user: "u-an", lesson: "db-test", no: 1, days: 48, minutes: 8, mask: []int{1, 1, 1, 1, 0},
		review: &quizReviewSeed{by: "u-gv", days: 47.5,
			comment: "Nắm chắc khóa chính, khóa ngoại và ràng buộc. Câu về index cho JSONB sai, đọc lại bài Đọc thêm: chỉ mục.",
			notes:   map[int]string{4: "JSONB và mảng dùng GIN; B-tree hợp với so sánh và sắp xếp."}}},
	{class: "cl-basic01", user: "u-bich", lesson: "db-practice", no: 1, days: 47, minutes: 5, mask: []int{1, 0, 1, 1}},
	{class: "cl-basic01", user: "u-bich", lesson: "db-test", no: 1, days: 46, minutes: 9, mask: []int{1, 1, 1, 0, 1},
		review: &quizReviewSeed{by: "u-gv", days: 45.5,
			comment: "Đạt ngưỡng. Lưu ý không đánh index tràn lan.",
			notes:   map[int]string{3: "Mỗi index làm chậm INSERT và UPDATE, chỉ đánh cho cột hay lọc hoặc JOIN."}}},
	{class: "cl-basic01", user: "u-cuong", lesson: "db-test", no: 1, days: 49, minutes: 6, mask: []int{1, 1, 1, 1, 1},
		review: &quizReviewSeed{by: "u-gv", days: 48.5,
			comment: "Đúng cả năm câu trong 6 phút. Rất tốt."}},
	{class: "cl-basic01", user: "u-ha", lesson: "db-test", no: 1, days: 40, minutes: 10, mask: []int{1, 0, 1, 0, 0},
		review: &quizReviewSeed{by: "u-gv", days: 39,
			comment: "Bạn còn nhầm giữa các kiểu ràng buộc khóa ngoại. Xem lại bài Thiết kế bảng và khóa, làm lại phần luyện tập rồi dùng lượt thứ hai.",
			notes:   map[int]string{1: "RESTRICT từ chối xóa; CASCADE mới xóa dây chuyền.", 3: "Index cho cột hay lọc và JOIN, không phải mọi cột.", 4: "JSONB dùng GIN."}}},
	{class: "cl-basic01", user: "u-ha", lesson: "db-test", no: 2, days: 3.5, minutes: 9, mask: []int{1, 1, 1, 0, 1}},
	// Khang để hết giờ: hệ thống tự nộp với các câu đã lưu.
	{class: "cl-basic01", user: "u-khang", lesson: "db-test", no: 1, days: 22, minutes: 10, mask: []int{1, 0, 0, 1, 0}, auto: true,
		review: &quizReviewSeed{by: "u-gv", days: 21,
			comment: "Bạn mới học xong hai bài đầu của chặng. Học hết bài Đọc thêm: chỉ mục trước khi dùng lượt còn lại nhé.",
			notes:   map[int]string{1: "Xem lại mục ON DELETE trong bài Thiết kế bảng và khóa.", 2: "UNIQUE trên cặp cột chặn trùng cặp, không chặn từng cột."}}},
	{class: "cl-basic01", user: "u-phong", lesson: "db-test", no: 1, days: 9, minutes: 7, mask: []int{1, 1, 1, 1, 0},
		review: &quizReviewSeed{by: "u-gv", days: 8,
			comment: "Vào lớp muộn nhưng bắt kịp nhanh. Câu GIN sai, phần còn lại chắc.",
			notes:   map[int]string{4: "Cột JSONB dùng GIN."}}},
	{class: "cl-basic02", user: "u-phong", lesson: "db-test", no: 1, days: 15, minutes: 8, mask: []int{1, 1, 0, 1, 1},
		review: &quizReviewSeed{by: "u-gv2", days: 14,
			comment: "Đạt. Câu UNIQUE trên cặp cột cần đọc kỹ đề hơn.",
			notes:   map[int]string{2: "Ràng buộc áp lên cặp (lớp, học viên), không áp lên từng cột riêng."}}},
	{class: "cl-basic02", user: "u-linh", lesson: "db-practice", no: 1, days: 2, minutes: 6, mask: []int{0, 1, 1, 0}},
	{class: "cl-basic02", user: "u-linh", lesson: "db-test", no: 1, days: 1.5, minutes: 9, mask: []int{1, 1, 0, 0, 1}},
	{class: "cl-basic02", user: "u-nhan", lesson: "db-practice", no: 1, days: 5.5, minutes: 7, mask: []int{1, 0, 0, 1}},
	{class: "cl-basic01", user: "u-an", lesson: "ds-test", no: 1, days: 15, minutes: 7, mask: []int{1, 1, 1, 1, 1},
		review: &quizReviewSeed{by: "u-gv", days: 14,
			comment: "Đúng hết, phần BFS và xử lý xung đột trả lời rất chắc."}},
	{class: "cl-basic01", user: "u-cuong", lesson: "ds-test", no: 1, days: 19, minutes: 9, mask: []int{1, 1, 0, 1, 1},
		review: &quizReviewSeed{by: "u-gv", days: 18,
			comment: "Đạt. Câu danh sách liên kết: nhớ đổi lại là truy cập theo chỉ số chậm.",
			notes:   map[int]string{2: "Ưu điểm là chèn, xóa ở giữa nhanh; truy cập phần tử thứ i là O(n)."}}},
	{class: "cl-basic01", user: "u-bich", lesson: "ds-test", no: 1, days: 1.8, minutes: 10, mask: []int{1, 0, 0, 1, 0}},
	{class: "cl-basic02", user: "u-phong", lesson: "ds-test", no: 1, days: 0.35, minutes: 8, mask: []int{1, 1, 1, 1, 0}},
	{class: "cl-basic01", user: "u-an", lesson: "go-practice", no: 1, days: 1.5, minutes: 5, mask: []int{1, 0, 1, 1}},
	{class: "cl-basic01", user: "u-cuong", lesson: "go-practice", no: 1, days: 6.5, minutes: 4, mask: []int{1, 1, 1, 0},
		review: &quizReviewSeed{by: "u-gv", days: 6,
			comment: "Phần goroutine và WaitGroup ổn. Câu deadlock là lỗi hay gặp khi làm bài tập, chạy thử đoạn code đó một lần cho nhớ.",
			notes:   map[int]string{3: "Channel không đệm chặn người gửi cho tới khi có người nhận."}}},
	{class: "cl-basic01", user: "u-cuong", lesson: "re-practice", no: 1, days: 4, minutes: 5, mask: []int{1, 1, 0, 1}},
	{class: "cl-basic01", user: "u-an", lesson: "re-practice", no: 1, days: 0.18, minutes: 6, mask: []int{1, 1, 1, 0}},
	{class: "cl-basic01", user: "u-cuong", lesson: "re-test", no: 1, days: 2.5, minutes: 11, mask: []int{1, 1, 1, 1, 0}},
	{class: "cl-basic01", user: "u-cuong", lesson: "web-practice", no: 1, days: 1.05, minutes: 4, mask: []int{1, 1, 1, 1}},
	// Bích đang làm dở quiz luyện tập Go từ một giờ trước, đã lưu hai câu đầu: mở lại thì tiếp tục ở câu thứ ba.
	{class: "cl-basic01", user: "u-bich", lesson: "go-practice", no: 1, days: 1.0 / 24, minutes: 6, mask: []int{1, 0}, open: true},
}

// stageReviewSeed là nhận xét tổng kết chặng của giảng viên (stageReviews của seed.js): giảng viên by viết days ngày
// trước, chưa sửa lần nào nên revision 1.
type stageReviewSeed struct {
	class, user, stageVersion string
	by                        string
	days                      float64
	body                      string
}

var stageReviews = []stageReviewSeed{
	{class: "cl-basic01", user: "u-an", stageVersion: "sv-ds-1", by: "u-gv", days: 16,
		body: "An nắm chắc stack và queue, bài nộp lại sửa đúng hướng. Chặng Go nên ôn lại phần đồng bộ goroutine trước bài kiểm tra."},
	{class: "cl-basic01", user: "u-cuong", stageVersion: "sv-ds-1", by: "u-gv", days: 19,
		body: "Hoàn thành tốt, code gọn. Có thể hỗ trợ bạn khác trong nhóm."},
}
