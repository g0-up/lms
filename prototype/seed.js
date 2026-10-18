/* Seed data for the GoUp LMS prototype.
   Mirrors the reference scenario in spec-lms-mvp.md §7.3: course "Basic" v1 built from
   five published stage versions, classes basic01 and basic02 running on it, basic03 still a draft.
   All timestamps are relative to the moment the prototype is first opened. */
window.LMS_SEED = function (now) {
  const H = 3600e3, D = 24 * H;
  const ago = (days, hours = 0) => new Date(now - days * D - hours * H).toISOString();
  const ahead = (days) => new Date(now + days * D).toISOString();

  const md = {
    sqlIntro: `# Giới thiệu SQL

SQL (Structured Query Language) là ngôn ngữ chuẩn để làm việc với cơ sở dữ liệu quan hệ. Trong chặng này bạn sẽ dùng PostgreSQL.

## Bốn nhóm lệnh cần nhớ

- **SELECT** – đọc dữ liệu
- **INSERT / UPDATE / DELETE** – thay đổi dữ liệu
- **CREATE / ALTER / DROP** – thay đổi cấu trúc
- **GRANT / REVOKE** – phân quyền

\`\`\`sql
SELECT id, email FROM users WHERE status = 'active' ORDER BY created_at DESC;
\`\`\`

> Bài tập: liệt kê 10 học viên đăng nhập gần nhất.`,
    tableDesign: `# Thiết kế bảng và khóa

Một bảng tốt có khóa chính rõ ràng, kiểu dữ liệu đúng và ràng buộc được khai báo ở tầng database.

## Khóa chính và khóa ngoại

1. Mỗi bảng có đúng một khóa chính.
2. Khóa ngoại tham chiếu tới khóa chính của bảng khác.
3. Dùng \`ON DELETE RESTRICT\` cho dữ liệu đã được tham chiếu.

\`\`\`sql
CREATE TABLE class_members (
  id         bigserial PRIMARY KEY,
  class_id   bigint NOT NULL REFERENCES classes(id) ON DELETE RESTRICT,
  user_id    bigint NOT NULL REFERENCES users(id),
  UNIQUE (class_id, user_id)
);
\`\`\``,
    tableDesignV2: `# Thiết kế bảng và khóa (bản cập nhật)

Bản này bổ sung phần **partial unique index** mà nhiều bạn hỏi trong lớp basic01.

## Khóa chính và khóa ngoại

1. Mỗi bảng có đúng một khóa chính.
2. Khóa ngoại tham chiếu tới khóa chính của bảng khác.
3. Dùng \`ON DELETE RESTRICT\` cho dữ liệu đã được tham chiếu.

## Partial unique index

Chỉ cho phép một bản nháp cho mỗi khóa học:

\`\`\`sql
CREATE UNIQUE INDEX one_draft_per_course
  ON course_versions (course_id) WHERE status = 'draft';
\`\`\``,
    indexes: `# Đọc thêm: chỉ mục

Chỉ mục (index) giúp truy vấn nhanh hơn nhưng làm chậm ghi. Chỉ đánh index cho cột xuất hiện trong \`WHERE\`, \`JOIN\` hoặc \`ORDER BY\` thường xuyên.

- B-tree: mặc định, phù hợp so sánh và sắp xếp.
- GIN: cho mảng, JSONB, tìm kiếm toàn văn.`,
    stackQueue: `# Stack và Queue

- **Stack** (ngăn xếp): vào sau ra trước (LIFO). Dùng cho undo, duyệt cây, kiểm tra ngoặc.
- **Queue** (hàng đợi): vào trước ra trước (FIFO). Dùng cho hàng đợi gửi email, BFS.

\`\`\`go
stack := []int{}
stack = append(stack, 1)        // push
top := stack[len(stack)-1]      // peek
stack = stack[:len(stack)-1]    // pop
\`\`\``,
    goTypes: `# Kiểu dữ liệu và hàm trong Go

Go là ngôn ngữ tĩnh kiểu. Khai báo biến bằng \`var\` hoặc \`:=\`.

\`\`\`go
func sum(xs []int) int {
    total := 0
    for _, x := range xs {
        total += x
    }
    return total
}
\`\`\`

Hàm có thể trả về nhiều giá trị, thường là \`(kết quả, error)\`.`,
    goHello: `# Hello World từng bước

## 1. Tạo module

\`\`\`bash
mkdir hello && cd hello
go mod init example.com/hello
\`\`\`

## 2. Viết chương trình

\`\`\`go
package main

import "fmt"

func main() {
    fmt.Println("Xin chào, Go!")
}
\`\`\`

## 3. Chạy và biên dịch

- \`go run .\` biên dịch vào thư mục tạm rồi chạy.
- \`go build\` tạo file nhị phân \`hello\` để chạy lại nhiều lần.

> Chạy \`go fmt ./...\` trước khi nộp bài để code đúng định dạng chuẩn.`,
    goExercise: `# Bài tập tổng hợp Go

Viết chương trình đọc file CSV danh sách học viên và in ra số học viên theo từng lớp.

Yêu cầu:
1. Dùng package \`encoding/csv\`.
2. Xử lý lỗi khi file không tồn tại.
3. Kết quả sắp xếp theo mã lớp.`,
    reactState: `# State và sự kiện

\`useState\` giữ dữ liệu thay đổi theo thời gian trong component.

\`\`\`jsx
function Counter() {
  const [count, setCount] = useState(0);
  return <button onClick={() => setCount(count + 1)}>{count}</button>;
}
\`\`\`

Khi state đổi, React render lại component đó và các component con.`,
    htmlSemantic: `# HTML ngữ nghĩa

Dùng đúng thẻ cho đúng ý nghĩa: \`<header>\`, \`<nav>\`, \`<main>\`, \`<article>\`, \`<section>\`, \`<footer>\`.

Lợi ích:
- Trình đọc màn hình hiểu cấu trúc trang.
- Công cụ tìm kiếm đánh giá nội dung tốt hơn.
- CSS và JavaScript dễ bám vào hơn.`,
    dsHomework: `# Bài tập: Cài đặt Stack và Queue

Cài đặt hai cấu trúc dữ liệu bằng Go, **không dùng thư viện ngoài**.

## Yêu cầu

1. \`Stack\` có \`Push\`, \`Pop\`, \`Peek\`, \`Len\`. \`Pop\` trên stack rỗng trả về lỗi, không panic.
2. \`Queue\` có \`Enqueue\`, \`Dequeue\`, \`Len\`, đúng thứ tự FIFO.
3. Viết test với \`go test\` cho cả hai, gồm trường hợp rỗng.
4. README ghi cách chạy test.

> Nộp link repo kèm commit hash, hoặc nén thư mục thành file zip.`,
    goHomework: `# Bài tập: Đọc CSV và thống kê

Viết chương trình đọc file \`hocvien.csv\` (cột: email, ma_lop) và in số học viên theo từng lớp.

## Yêu cầu

1. Dùng package \`encoding/csv\`.
2. File không tồn tại: in thông báo lỗi rõ ràng và thoát với mã khác 0.
3. Kết quả sắp xếp theo mã lớp tăng dần.

\`\`\`text
basic01  7
basic02  5
\`\`\``
  };

  let lessonSeq = 0, qSeq = 0, oSeq = 0, cSeq = 0;
  const L = (key, type, title, required, extra = {}) => ({
    id: 'l' + (++lessonSeq), key, type, title, required, ...extra
  });
  // Quiz question: options are [text, correct]. The key stays stable when a stage version is cloned,
  // so per-question statistics can be compared across versions.
  const Q = (key, type, text, options, explanation, points = 1) => ({
    id: 'q' + (++qSeq), key, type, text, explanation, points,
    options: options.map(([t, correct]) => ({ id: 'o' + (++oSeq), text: t, correct: !!correct }))
  });
  const C = (text, max) => ({ id: 'c' + (++cSeq), text, max });

  const stages = [
    { id: 'st-db', code: 'DB', name: 'Database' },
    { id: 'st-ds', code: 'DS', name: 'Data structure' },
    { id: 'st-go', code: 'GO', name: 'Golang basic' },
    { id: 'st-re', code: 'RE', name: 'ReactJS basic' },
    { id: 'st-web', code: 'WEB', name: 'HTML CSS JS' }
  ];

  const stageVersions = [
    { id: 'sv-db-1', stageId: 'st-db', no: 1, status: 'published', clonedFrom: null, publishedAt: ago(70), lessons: [
      L('db-intro', 'video', 'Giới thiệu SQL', true, { video: 'db-01-gioi-thieu-sql.mp4', duration: '18:24' }),
      L('db-table', 'markdown', 'Thiết kế bảng và khóa', true, { content: md.tableDesign }),
      L('db-index', 'markdown', 'Đọc thêm: chỉ mục', false, { content: md.indexes })
    ] },
    { id: 'sv-ds-1', stageId: 'st-ds', no: 1, status: 'published', clonedFrom: null, publishedAt: ago(68), lessons: [
      L('ds-array', 'video', 'Mảng và danh sách liên kết', true, { video: 'ds-01-mang-danh-sach.mp4', duration: '22:10' }),
      L('ds-stack', 'markdown', 'Stack và Queue', true, { content: md.stackQueue }),
      L('ds-hash', 'video', 'Hash table', true, { video: 'ds-03-hash-table.mp4', duration: '16:45' }),
      L('ds-quiz', 'quiz', 'Luyện tập: Stack, Queue và Hash', true, { quiz: { mode: 'practice', attempts: null, timeLimit: null, threshold: 70, shuffle: true, questions: [
        Q('qk-ds-lifo', 'single', 'Cấu trúc nào hoạt động theo nguyên tắc **vào sau ra trước** (LIFO)?', [['Stack', 1], ['Queue'], ['Hash table'], ['Mảng vòng']], 'Stack lấy ra phần tử được đưa vào sau cùng, như chồng đĩa.'),
        Q('qk-ds-hash-o', 'single', 'Độ phức tạp **trung bình** khi tra cứu một khóa trong hash table là gì?', [['O(1)', 1], ['O(log n)'], ['O(n)'], ['O(n log n)']], 'Hàm băm tính thẳng vị trí của khóa; trường hợp xấu nhất mới là O(n) khi xung đột nhiều.'),
        Q('qk-ds-queue-use', 'multi', 'Những bài toán nào thường dùng **Queue**? Chọn tất cả đáp án đúng.', [['Duyệt đồ thị theo chiều rộng (BFS)', 1], ['Hàng đợi gửi email', 1], ['Kiểm tra cặp ngoặc hợp lệ'], ['Chức năng hoàn tác (undo)']], 'BFS và hàng đợi gửi email xử lý theo thứ tự đến trước. Kiểm tra ngoặc và undo dùng stack.'),
        Q('qk-ds-pop', 'single', 'Trong Go, câu lệnh `stack = stack[:len(stack)-1]` làm gì?', [['Bỏ phần tử ở đỉnh stack (pop)', 1], ['Thêm phần tử vào stack (push)'], ['Đọc phần tử ở đỉnh mà không bỏ (peek)'], ['Xóa toàn bộ stack']], 'Cắt slice bỏ phần tử cuối cùng chính là thao tác pop.'),
        Q('qk-ds-collision', 'single', 'Hai khóa khác nhau cho cùng một giá trị băm được gọi là gì?', [['Xung đột (collision)', 1], ['Tràn bộ nhớ'], ['Rehash'], ['Cân bằng cây']], 'Xung đột được xử lý bằng chaining hoặc open addressing.')
      ] } }),
      L('ds-hw', 'homework', 'Bài tập: Cài đặt Stack và Queue', true, { homework: { brief: md.dsHomework, threshold: 70, attachments: ['ds-stack-queue-starter.zip'], criteria: [
        C('Stack đủ Push, Pop, Peek, Len; Pop trên stack rỗng trả lỗi', 40), C('Queue đúng thứ tự FIFO', 30), C('Có test cho trường hợp rỗng và README cách chạy', 30)
      ] } })
    ] },
    { id: 'sv-go-1', stageId: 'st-go', no: 1, status: 'published', clonedFrom: null, publishedAt: ago(66), lessons: [
      L('go-setup', 'video', 'Video: Cài đặt Go', true, { video: 'go-01-cai-dat.mp4', duration: '12:02' }),
      L('go-types', 'markdown', 'Kiểu dữ liệu và hàm', true, { content: md.goTypes }),
      L('go-routine', 'video', 'Goroutine cơ bản', true, { video: 'go-03-goroutine.mp4', duration: '25:31' }),
      L('go-exercise', 'markdown', 'Bài tập tổng hợp', false, { content: md.goExercise }),
      L('go-hw', 'homework', 'Bài tập: Đọc CSV và thống kê', true, { homework: { brief: md.goHomework, threshold: 70, attachments: ['hocvien.csv'], criteria: [
        C('Đọc CSV bằng encoding/csv, xử lý lỗi file không tồn tại', 40), C('Thống kê đúng theo lớp và sắp xếp theo mã lớp', 40), C('Code rõ ràng, có README', 20)
      ] } }),
      L('go-quiz', 'quiz', 'Kiểm tra cuối chặng Go', true, { quiz: { mode: 'assessment', attempts: 2, timeLimit: 15, threshold: 70, shuffle: true, questions: [
        Q('qk-go-short', 'single', 'Cú pháp nào **khai báo và gán** biến ngắn gọn bên trong hàm?', [['`:=`', 1], ['`=`'], ['`var x`'], ['`let x =`']], 'Toán tử := vừa khai báo vừa suy ra kiểu, chỉ dùng trong thân hàm.'),
        Q('qk-go-error', 'single', 'Theo quy ước, hàm Go báo lỗi bằng cách nào?', [['Trả về giá trị cuối cùng có kiểu `error`', 1], ['Gọi `panic` cho mọi lỗi'], ['Ném exception'], ['Ghi lỗi vào biến toàn cục']], 'Quy ước (kết quả, error) giúp người gọi xử lý lỗi tường minh.'),
        Q('qk-go-sync', 'multi', 'Những cách nào dùng để **đồng bộ** các goroutine? Chọn tất cả đáp án đúng.', [['`sync.WaitGroup`', 1], ['Channel', 1], ['`time.Sleep`'], ['Biến toàn cục kiểu bool']], 'WaitGroup và channel là cơ chế đồng bộ; Sleep chỉ đoán thời gian, không đảm bảo thứ tự.'),
        Q('qk-go-range-int', 'single', 'Vòng lặp `for i := range 3` hợp lệ từ phiên bản Go nào?', [['Go 1.22', 1], ['Go 1.18'], ['Go 1.20'], ['Go 1.21']], 'Range trên số nguyên được thêm vào Go 1.22.'),
        Q('qk-go-run', 'single', 'Lệnh nào biên dịch và chạy chương trình mà **không** để lại file nhị phân?', [['`go run`', 1], ['`go build`'], ['`go install`'], ['`go mod tidy`']], 'go run biên dịch vào thư mục tạm rồi chạy.')
      ] } })
    ] },
    { id: 'sv-re-1', stageId: 'st-re', no: 1, status: 'published', clonedFrom: null, publishedAt: ago(64), lessons: [
      L('re-component', 'video', 'Component và props', true, { video: 're-01-component.mp4', duration: '20:15' }),
      L('re-state', 'markdown', 'State và sự kiện', true, { content: md.reactState }),
      L('re-hooks', 'video', 'Hooks', true, { video: 're-03-hooks.mp4', duration: '28:40' })
    ] },
    { id: 'sv-web-1', stageId: 'st-web', no: 1, status: 'published', clonedFrom: null, publishedAt: ago(62), lessons: [
      L('web-html', 'markdown', 'HTML ngữ nghĩa', true, { content: md.htmlSemantic }),
      L('web-css', 'video', 'CSS layout', true, { video: 'web-02-css-layout.mp4', duration: '24:05' }),
      L('web-js', 'video', 'JavaScript và DOM', true, { video: 'web-03-js-dom.mp4', duration: '19:50' })
    ] }
  ];
  // Draft v2 of the Go stage, cloned from v1 like the app's clone action: new ids, same keys.
  // The admin authors quizzes in this draft and publishes it.
  const cloneLesson = (l) => {
    const c = JSON.parse(JSON.stringify(l)); c.id = 'l' + (++lessonSeq);
    if (c.quiz) c.quiz.questions.forEach((x) => { x.id = 'q' + (++qSeq); x.options.forEach((o) => { o.id = 'o' + (++oSeq); }); });
    if (c.homework) c.homework.criteria.forEach((x) => { x.id = 'c' + (++cSeq); });
    return c;
  };
  stageVersions.push({ id: 'sv-go-2', stageId: 'st-go', no: 2, status: 'draft', clonedFrom: 'sv-go-1', publishedAt: null,
    lessons: stageVersions.find((v) => v.id === 'sv-go-1').lessons.map(cloneLesson) });

  // A practice quiz and an assessment quiz for every stage. They are built after the lessons above so
  // existing ids stay stable, then placed after the lesson with the given key (or at the end of the stage).
  const place = (svId, afterKey, lesson) => {
    const ls = stageVersions.find((v) => v.id === svId).lessons;
    ls.splice(afterKey ? ls.findIndex((x) => x.key === afterKey) + 1 : ls.length, 0, lesson);
    return lesson;
  };
  const practice = (questions) => ({ quiz: { mode: 'practice', attempts: null, timeLimit: null, threshold: 70, shuffle: true, questions } });
  const assessment = (attempts, timeLimit, threshold, questions) => ({ quiz: { mode: 'assessment', attempts, timeLimit, threshold, shuffle: true, questions } });
  place('sv-db-1', 'db-table', L('db-practice', 'quiz', 'Luyện tập: Truy vấn SQL cơ bản', true, practice([
    Q('qk-db-select', 'single', 'Lệnh nào dùng để **đọc** dữ liệu từ bảng?', [['`SELECT`', 1], ['`INSERT`'], ['`UPDATE`'], ['`GRANT`']], 'SELECT đọc dữ liệu; INSERT và UPDATE thay đổi dữ liệu; GRANT phân quyền.'),
    Q('qk-db-where', 'single', 'Mệnh đề nào lọc các dòng theo điều kiện?', [['`WHERE`', 1], ['`ORDER BY`'], ['`GROUP BY`'], ['`LIMIT`']], 'WHERE giữ lại các dòng thỏa điều kiện trước khi sắp xếp hay gom nhóm.'),
    Q('qk-db-ddl', 'multi', 'Những lệnh nào thay đổi **cấu trúc** bảng? Chọn tất cả đáp án đúng.', [['`CREATE`', 1], ['`ALTER`', 1], ['`DELETE`'], ['`SELECT`']], 'CREATE và ALTER thuộc nhóm DDL. DELETE chỉ xóa dữ liệu, cấu trúc giữ nguyên.'),
    Q('qk-db-pk', 'single', 'Mỗi bảng nên có bao nhiêu khóa chính?', [['Đúng một', 1], ['Không cần khóa chính'], ['Hai'], ['Tùy số cột']], 'Khóa chính có thể gồm nhiều cột, nhưng mỗi bảng chỉ có một khóa chính.')
  ])));
  place('sv-db-1', null, L('db-test', 'quiz', 'Kiểm tra cuối chặng Database', true, assessment(2, 10, 70, [
    Q('qk-db-fk', 'single', 'Khóa ngoại tham chiếu tới cột nào của bảng khác?', [['Khóa chính (hoặc cột UNIQUE)', 1], ['Cột bất kỳ'], ['Cột có index'], ['Cột kiểu text']], 'Khóa ngoại phải trỏ tới cột định danh duy nhất một dòng, thường là khóa chính.'),
    Q('qk-db-restrict', 'single', '`ON DELETE RESTRICT` làm gì khi xóa một dòng đang được tham chiếu?', [['Từ chối lệnh xóa', 1], ['Xóa luôn các dòng tham chiếu'], ['Đặt khóa ngoại về NULL'], ['Bỏ qua ràng buộc']], 'RESTRICT chặn việc xóa; CASCADE mới xóa dây chuyền, SET NULL mới đặt về NULL.'),
    Q('qk-db-unique', 'single', '`UNIQUE (class_id, user_id)` đảm bảo điều gì?', [['Một học viên chỉ có một dòng trong mỗi lớp', 1], ['Mỗi lớp chỉ có một học viên'], ['user_id không trùng trên toàn bảng'], ['class_id không được NULL']], 'Ràng buộc trên cặp cột: cùng một cặp (lớp, học viên) không được lặp lại.'),
    Q('qk-db-index', 'multi', 'Nên đánh index cho những cột nào? Chọn tất cả đáp án đúng.', [['Cột thường xuất hiện trong `WHERE`', 1], ['Cột dùng để `JOIN`', 1], ['Mọi cột của bảng'], ['Cột chỉ được ghi, không bao giờ đọc']], 'Index tăng tốc đọc nhưng làm chậm ghi, nên chỉ đánh cho cột hay dùng để lọc, nối hoặc sắp xếp.'),
    Q('qk-db-gin', 'single', 'Loại index nào phù hợp cho cột JSONB?', [['GIN', 1], ['B-tree'], ['Hash'], ['Không đánh index được']], 'GIN đánh index từng phần tử bên trong mảng và JSONB.')
  ])));
  place('sv-ds-1', null, L('ds-test', 'quiz', 'Kiểm tra cuối chặng Data structure', true, assessment(2, 10, 70, [
    Q('qk-ds-bfs', 'single', 'BFS dùng cấu trúc nào để lưu các đỉnh chờ duyệt?', [['Queue', 1], ['Stack'], ['Hash table'], ['Cây nhị phân']], 'BFS duyệt theo lớp: đỉnh phát hiện trước được duyệt trước, đúng thứ tự FIFO.'),
    Q('qk-ds-paren', 'single', 'Kiểm tra chuỗi `({[]})` có ngoặc hợp lệ nên dùng cấu trúc nào?', [['Stack', 1], ['Queue'], ['Mảng đã sắp xếp'], ['Hash table']], 'Ngoặc mở gần nhất phải đóng trước: đúng nguyên tắc LIFO của stack.'),
    Q('qk-ds-linked', 'single', 'Ưu điểm của danh sách liên kết so với mảng là gì?', [['Chèn hoặc xóa ở giữa không phải dời phần tử', 1], ['Truy cập phần tử thứ i trong O(1)'], ['Tốn ít bộ nhớ hơn'], ['Luôn được sắp xếp']], 'Chỉ cần đổi con trỏ; đổi lại, truy cập theo chỉ số là O(n).'),
    Q('qk-ds-collide', 'multi', 'Những cách nào xử lý xung đột trong hash table? Chọn tất cả đáp án đúng.', [['Chaining', 1], ['Open addressing', 1], ['Sắp xếp lại mảng'], ['Bỏ qua khóa mới']], 'Chaining nối các khóa trùng vị trí thành danh sách; open addressing tìm ô trống khác.'),
    Q('qk-ds-worst', 'single', 'Khi nào tra cứu trong hash table rơi vào O(n)?', [['Khi rất nhiều khóa xung đột cùng một vị trí', 1], ['Khi bảng rỗng'], ['Khi khóa là chuỗi'], ['Khi dùng chaining']], 'Nếu mọi khóa dồn vào một vị trí, tra cứu phải duyệt cả danh sách.')
  ])));
  const goPractice = place('sv-go-1', 'go-routine', L('go-practice', 'quiz', 'Luyện tập: Goroutine và channel', false, practice([
    Q('qk-go-go', 'single', 'Từ khóa nào chạy một hàm trong goroutine mới?', [['`go`', 1], ['`async`'], ['`thread`'], ['`defer`']], '`go f()` chạy f đồng thời; defer chỉ hoãn lời gọi tới cuối hàm.'),
    Q('qk-go-chan', 'single', 'Gửi giá trị `v` vào channel `ch` viết thế nào?', [['`ch <- v`', 1], ['`v <- ch`'], ['`ch.send(v)`'], ['`ch = v`']], 'Mũi tên chỉ hướng dữ liệu đi: từ v vào ch.'),
    Q('qk-go-wg', 'multi', 'Những bước nào cần khi dùng `sync.WaitGroup`? Chọn tất cả đáp án đúng.', [['Gọi `Add` trước khi chạy goroutine', 1], ['Gọi `Done` khi goroutine xong', 1], ['Gọi `Wait` để chờ', 1], ['Gọi `time.Sleep` sau `Wait`']], 'Add, Done và Wait là đủ; Sleep không cần và không đảm bảo gì.'),
    Q('qk-go-deadlock', 'single', 'Hàm `main` chỉ có `ch := make(chan int); ch <- 1` sẽ thế nào?', [['Bị deadlock vì không ai nhận', 1], ['In ra 1'], ['Chạy xong bình thường'], ['Lỗi biên dịch']], 'Channel không đệm chặn người gửi cho tới khi có người nhận; không có ai nên runtime báo deadlock.')
  ])));
  place('sv-go-2', 'go-routine', cloneLesson(goPractice));
  place('sv-re-1', 're-state', L('re-practice', 'quiz', 'Luyện tập: Component, props và state', true, practice([
    Q('qk-re-props', 'single', 'Props trong React là gì?', [['Dữ liệu component cha truyền xuống component con', 1], ['State nội bộ của component'], ['Hàm gọi API'], ['Thuộc tính CSS']], 'Props chỉ đọc ở component con; muốn đổi thì component cha đổi.'),
    Q('qk-re-setstate', 'single', 'Gọi `setCount(count + 1)` sẽ làm gì?', [['Cập nhật state và render lại component', 1], ['Đổi biến count ngay mà không render'], ['Tải lại trang'], ['Không làm gì']], 'Hàm set lên lịch cập nhật; React render lại với giá trị mới.'),
    Q('qk-re-key', 'single', 'Khi render một danh sách, mỗi phần tử cần prop nào?', [['`key`', 1], ['`id`'], ['`index`'], ['`ref`']], 'key giúp React nhận ra phần tử nào thêm, bớt hay đổi chỗ.'),
    Q('qk-re-immut', 'multi', 'Cách nào cập nhật mảng trong state đúng? Chọn tất cả đáp án đúng.', [['`setItems([...items, x])`', 1], ['`setItems(items.filter(fn))`', 1], ['`items.push(x)`'], ['`items[0] = x`']], 'Luôn tạo mảng mới; sửa trực tiếp mảng cũ thì React không biết để render lại.')
  ])));
  place('sv-re-1', null, L('re-test', 'quiz', 'Kiểm tra cuối chặng ReactJS', true, assessment(2, 15, 70, [
    Q('qk-re-effect', 'single', '`useEffect(fn, [])` chạy `fn` khi nào?', [['Một lần sau lần render đầu tiên', 1], ['Sau mỗi lần render'], ['Trước lần render đầu tiên'], ['Không bao giờ']], 'Mảng phụ thuộc rỗng nghĩa là không có gì thay đổi để chạy lại.'),
    Q('qk-re-hookrule', 'single', 'Quy tắc nào đúng khi dùng hook?', [['Chỉ gọi hook ở cấp cao nhất của component', 1], ['Gọi hook trong vòng lặp'], ['Gọi hook trong điều kiện if'], ['Gọi hook trong hàm thường']], 'React dựa vào thứ tự gọi hook ở mỗi lần render, nên thứ tự đó không được đổi.'),
    Q('qk-re-lift', 'single', 'Hai component anh em cần dùng chung một state thì làm gì?', [['Đưa state lên component cha chung', 1], ['Sao chép state vào cả hai'], ['Dùng biến toàn cục'], ['Đọc DOM của nhau']], 'Lifting state up: cha giữ state và truyền xuống bằng props.'),
    Q('qk-re-memo', 'multi', 'Những hook nào giúp tránh tính lại hoặc tạo lại giá trị không cần thiết? Chọn tất cả đáp án đúng.', [['`useMemo`', 1], ['`useCallback`', 1], ['`useState`'], ['`useEffect`']], 'useMemo nhớ kết quả tính toán, useCallback nhớ hàm giữa các lần render.'),
    Q('qk-re-controlled', 'single', 'Input "được điều khiển" (controlled) là input như thế nào?', [['Giá trị lấy từ state và đổi qua `onChange`', 1], ['Không có thuộc tính value'], ['Đọc giá trị bằng `document.querySelector`'], ['Chỉ dùng trong form submit']], 'State là nguồn sự thật duy nhất của giá trị input.')
  ])));
  place('sv-web-1', 'web-css', L('web-practice', 'quiz', 'Luyện tập: HTML ngữ nghĩa và CSS layout', true, practice([
    Q('qk-web-nav', 'single', 'Thẻ nào dành cho khối điều hướng chính?', [['`<nav>`', 1], ['`<div class="menu">`'], ['`<section>`'], ['`<aside>`']], 'nav báo cho trình đọc màn hình đây là vùng điều hướng.'),
    Q('qk-web-flex', 'single', 'Thuộc tính nào căn các phần tử theo trục chính trong flexbox?', [['`justify-content`', 1], ['`align-items`'], ['`text-align`'], ['`vertical-align`']], 'justify-content theo trục chính, align-items theo trục phụ.'),
    Q('qk-web-a11y', 'multi', 'HTML ngữ nghĩa mang lại lợi ích gì? Chọn tất cả đáp án đúng.', [['Trình đọc màn hình hiểu cấu trúc trang', 1], ['Công cụ tìm kiếm đánh giá nội dung tốt hơn', 1], ['Trang tải nhanh gấp đôi'], ['Không cần viết CSS nữa']], 'Thẻ đúng nghĩa giúp máy hiểu trang; tốc độ và giao diện không đổi.'),
    Q('qk-web-grid', 'single', '`grid-template-columns: repeat(3, 1fr)` tạo ra gì?', [['Ba cột rộng bằng nhau', 1], ['Ba hàng cao bằng nhau'], ['Một cột rộng 3fr'], ['Lưới 3×3']], '1fr là một phần không gian còn lại; ba cột chia đều.')
  ])));
  place('sv-web-1', null, L('web-test', 'quiz', 'Kiểm tra cuối chặng HTML CSS JS', true, assessment(3, 15, 80, [
    Q('qk-web-dom', 'single', 'Lấy phần tử **đầu tiên** khớp selector `.card` bằng cách nào?', [["`document.querySelector('.card')`", 1], ["`document.querySelectorAll('.card')`"], ["`document.getElementsByClassName('card')`"], ["`document.getElementById('card')`"]], 'querySelector trả về phần tử đầu tiên; querySelectorAll trả về cả danh sách.'),
    Q('qk-web-event', 'single', 'Gắn sự kiện click cho nút `btn` đúng cách là?', [["`btn.addEventListener('click', fn)`", 1], ['`btn.click = fn()`'], ["`btn.on('click')`"], ["`btn.event = 'click'`"]], 'addEventListener cho phép gắn nhiều listener và gỡ ra khi cần.'),
    Q('qk-web-main', 'single', 'Mỗi trang nên có bao nhiêu thẻ `<main>` đang hiển thị?', [['Một', 1], ['Không giới hạn'], ['Một cho mỗi section'], ['Không cần thẻ main']], 'main đánh dấu nội dung chính duy nhất của trang.'),
    Q('qk-web-box', 'multi', 'Những thành phần nào thuộc box model của CSS? Chọn tất cả đáp án đúng.', [['content', 1], ['padding', 1], ['border', 1], ['z-index']], 'Box model gồm content, padding, border và margin; z-index là thứ tự xếp lớp.'),
    Q('qk-web-delegation', 'single', 'Event delegation là gì?', [['Gắn một listener ở phần tử cha để xử lý sự kiện của các con', 1], ['Gắn listener cho từng phần tử con'], ['Chặn sự kiện lan ra ngoài'], ['Gọi sự kiện bằng `setTimeout`']], 'Sự kiện nổi bọt lên cha, nên một listener ở cha xử lý được cả các con thêm sau.')
  ])));

  // More học liệu for the first Go bài học, built last so every older id stays stable.
  const goHello = [
    L('go-hello-read', 'markdown', 'Hello World từng bước', false, { content: md.goHello }),
    L('go-hello-practice', 'quiz', 'Luyện tập: Lệnh go và cấu trúc chương trình', false, practice([
      Q('qk-go-main', 'single', 'Chương trình Go bắt đầu chạy từ đâu?', [['Hàm `main` trong `package main`', 1], ['Hàm `init` của mọi package'], ['Dòng đầu tiên của file'], ['Hàm `start`']], 'Lệnh go run và go build tìm package main và gọi hàm main.'),
      Q('qk-go-mod', 'single', 'Lệnh nào tạo file `go.mod` cho một dự án mới?', [['`go mod init`', 1], ['`go get`'], ['`go new`'], ['`go mod tidy`']], 'go mod init ghi tên module; go mod tidy chỉ dọn và bổ sung phụ thuộc.'),
      Q('qk-go-fmt', 'multi', 'Những lệnh nào có sẵn trong bộ công cụ Go? Chọn tất cả đáp án đúng.', [['`go fmt`', 1], ['`go test`', 1], ['`go vet`', 1], ['`go lint`']], 'go fmt, go test và go vet đi kèm Go; linter là công cụ bên ngoài.')
    ]))
  ];
  goHello.forEach((l) => { place('sv-go-1', null, l); place('sv-go-2', null, cloneLesson(l)); });

  // Bài học group the học liệu of a stage version: [key, title, học liệu keys in order]. The flat
  // lessons list is re-ordered to follow the bài học, so lookups by học liệu id keep working.
  // Bài học keys stay the same when a stage version is cloned, like lesson keys.
  let unitSeq = 0;
  const group = (svId, defs) => {
    const v = stageVersions.find((x) => x.id === svId);
    v.units = defs.map(([key, title]) => ({ id: 'un' + (++unitSeq), key, title }));
    v.lessons = defs.flatMap(([, , keys], i) => keys.map((k) => Object.assign(v.lessons.find((l) => l.key === k), { unitId: v.units[i].id })));
  };
  const FINAL = 'Kiểm tra cuối chặng';
  const goUnits = [['ux-go-hello', 'Cài đặt và Hello World', ['go-setup', 'go-hello-read', 'go-hello-practice']], ['ux-go-basics', 'Kiểu dữ liệu và hàm', ['go-types', 'go-exercise']],
    ['ux-go-concurrency', 'Goroutine và channel', ['go-routine', 'go-practice']], ['ux-go-final', FINAL, ['go-hw', 'go-quiz']]];
  group('sv-db-1', [['ux-db-sql', 'Nhập môn SQL', ['db-intro', 'db-practice']], ['ux-db-design', 'Thiết kế bảng', ['db-table', 'db-index']], ['ux-db-final', FINAL, ['db-test']]]);
  group('sv-ds-1', [['ux-ds-linear', 'Mảng, Stack và Queue', ['ds-array', 'ds-stack']], ['ux-ds-hash', 'Hash table', ['ds-hash', 'ds-quiz']], ['ux-ds-final', FINAL, ['ds-hw', 'ds-test']]]);
  group('sv-go-1', goUnits);
  group('sv-go-2', goUnits);
  group('sv-re-1', [['ux-re-basics', 'Component, props và state', ['re-component', 're-state', 're-practice']], ['ux-re-hooks', 'Hooks', ['re-hooks']], ['ux-re-final', FINAL, ['re-test']]]);
  group('sv-web-1', [['ux-web-layout', 'HTML và CSS', ['web-html', 'web-css', 'web-practice']], ['ux-web-dom', 'JavaScript và DOM', ['web-js']], ['ux-web-final', FINAL, ['web-test']]]);

  // Prepared lesson content for the scenario walk-through: the Database v2 draft the
  // admin will create can swap in this updated article.
  const extraContent = { 'db-table': md.tableDesignV2 };

  const courses = [{ id: 'co-basic', code: 'BASIC', name: 'Lập trình cơ bản' }];
  const courseVersions = [
    { id: 'cv-basic-1', courseId: 'co-basic', no: 1, status: 'published', clonedFrom: null, publishedAt: ago(60),
      stages: ['sv-db-1', 'sv-ds-1', 'sv-go-1', 'sv-re-1', 'sv-web-1'] }
  ];

  const users = [
    { id: 'u-admin', role: 'admin', name: 'Trần Minh Quân', email: 'quan.tran@goup.vn', status: 'active', mustChangePassword: false, lastLoginAt: ago(0, 2), lastActiveAt: ago(0, 1) },
    { id: 'u-gv', role: 'teacher', name: 'Lê Thu Hương', email: 'huong.le@goup.vn', status: 'active', mustChangePassword: false, lastLoginAt: ago(1), lastActiveAt: ago(1) },
    { id: 'u-gv2', role: 'teacher', name: 'Phạm Quốc Bảo', email: 'bao.pham@goup.vn', status: 'active', mustChangePassword: false, lastLoginAt: ago(3), lastActiveAt: ago(3) },
    { id: 'u-an',    role: 'student', name: 'Nguyễn Hoàng An',  email: 'an.nguyen@gmail.com',  status: 'active',   mustChangePassword: false, lastLoginAt: ago(0, 5), lastActiveAt: ago(0, 4) },
    { id: 'u-bich',  role: 'student', name: 'Trần Thị Bích',    email: 'bich.tran@gmail.com',  status: 'active',   mustChangePassword: false, lastLoginAt: ago(2), lastActiveAt: ago(2) },
    { id: 'u-cuong', role: 'student', name: 'Lê Văn Cường',     email: 'cuong.le@outlook.com', status: 'active',   mustChangePassword: false, lastLoginAt: ago(1), lastActiveAt: ago(1) },
    { id: 'u-dung',  role: 'student', name: 'Phạm Minh Dũng',   email: 'dung.pham@gmail.com',  status: 'invited',  mustChangePassword: true,  lastLoginAt: null, lastActiveAt: null, tempPasswordExpiresAt: ago(55) },
    { id: 'u-ha',    role: 'student', name: 'Hoàng Thu Hà',     email: 'ha.hoang@gmail.com',   status: 'active',   mustChangePassword: false, lastLoginAt: ago(4), lastActiveAt: ago(4) },
    { id: 'u-khang', role: 'student', name: 'Vũ Đức Khang',     email: 'khang.vu@gmail.com',   status: 'active',   mustChangePassword: false, lastLoginAt: ago(21), lastActiveAt: ago(21) },
    { id: 'u-phong', role: 'student', name: 'Đặng Hải Phong',   email: 'phong.dang@gmail.com', status: 'active',   mustChangePassword: false, lastLoginAt: ago(0, 9), lastActiveAt: ago(0, 8) },
    { id: 'u-linh',  role: 'student', name: 'Đỗ Ngọc Linh',     email: 'linh.do@gmail.com',    status: 'active',   mustChangePassword: false, lastLoginAt: ago(1), lastActiveAt: ago(1) },
    { id: 'u-minh',  role: 'student', name: 'Bùi Quang Minh',   email: 'minh.bui@gmail.com',   status: 'invited',  mustChangePassword: true,  lastLoginAt: null, lastActiveAt: null, tempPasswordExpiresAt: ahead(2) },
    { id: 'u-nhan',  role: 'student', name: 'Ngô Thanh Nhàn',   email: 'nhan.ngo@gmail.com',   status: 'active',   mustChangePassword: false, lastLoginAt: ago(6), lastActiveAt: ago(6) },
    { id: 'u-quyen', role: 'student', name: 'Lý Mai Quyên',     email: 'quyen.ly@gmail.com',   status: 'invited',  mustChangePassword: true,  lastLoginAt: null, lastActiveAt: null, tempPasswordExpiresAt: ahead(1.5) },
    { id: 'u-thao',  role: 'student', name: 'Võ Phương Thảo',   email: 'thao.vo@gmail.com',    status: 'disabled', mustChangePassword: false, lastLoginAt: ago(15), lastActiveAt: ago(15) },
    { id: 'u-son',   role: 'student', name: 'Trịnh Bảo Sơn',    email: 'son.trinh@gmail.com',  status: 'invited',  mustChangePassword: true,  lastLoginAt: null, lastActiveAt: null, tempPasswordExpiresAt: ahead(2.8) },
    { id: 'u-tu',    role: 'student', name: 'Mai Anh Tú',       email: 'tu.mai@gmail.com',     status: 'invited',  mustChangePassword: true,  lastLoginAt: null, lastActiveAt: null, tempPasswordExpiresAt: ahead(2.9) }
  ];

  const classes = [
    { id: 'cl-basic01', code: 'basic01', name: 'Lập trình cơ bản – khóa 1', courseVersionId: 'cv-basic-1', status: 'active', startDate: ago(58).slice(0, 10), endDate: ahead(55).slice(0, 10), teacherId: 'u-gv', createdAt: ago(60) },
    { id: 'cl-basic02', code: 'basic02', name: 'Lập trình cơ bản – khóa 2', courseVersionId: 'cv-basic-1', status: 'active', startDate: ago(20).slice(0, 10), endDate: ahead(95).slice(0, 10), teacherId: 'u-gv2', createdAt: ago(24) },
    { id: 'cl-basic03', code: 'basic03', name: 'Lập trình cơ bản – khóa 3', courseVersionId: 'cv-basic-1', status: 'draft', startDate: ahead(28).slice(0, 10), endDate: ahead(140).slice(0, 10), teacherId: 'u-gv', createdAt: ago(3) }
  ];

  let memSeq = 0, invSeq = 0;
  const members = [], invitations = [];
  const progress = {};
  function enroll(classId, userId, inviteStatus, invitedDaysAgo, memberStatus = 'active') {
    const m = { id: 'm' + (++memSeq), classId, userId, status: memberStatus, joinedAt: ago(invitedDaysAgo) };
    members.push(m);
    invitations.push({ id: 'inv' + (++invSeq), classId, userId, sentBy: 'u-admin', status: inviteStatus, attempts: inviteStatus === 'failed' ? 3 : 1,
      lastError: inviteStatus === 'failed' ? 'Mailbox không tồn tại (550 5.1.1)' : null, createdAt: ago(invitedDaysAgo), kind: 'invite' });
    return m;
  }
  // (memberId, lessonKeys..., opened only keys) — lessons found by key inside the class's course version
  function done(m, keys, lastDaysAgo) {
    keys.forEach((k, i) => {
      const lesson = findLesson(k);
      const t = ago(lastDaysAgo + (keys.length - i) * 1.3);
      progress[m.id + ':' + lesson.id] = { firstOpenedAt: t, completedAt: t };
    });
  }
  function opened(m, keys, daysAgo) {
    keys.forEach((k) => { const lesson = findLesson(k); progress[m.id + ':' + lesson.id] = { firstOpenedAt: ago(daysAgo), completedAt: null }; });
  }
  function findLesson(key) {
    for (const sv of stageVersions) { const l = sv.lessons.find(x => x.key === key); if (l) return l; }
    throw new Error('seed: lesson ' + key);
  }

  // basic01 — eight weeks in
  const an = enroll('cl-basic01', 'u-an', 'sent', 58);
  done(an, ['db-intro', 'db-table', 'db-index', 'ds-array', 'ds-stack', 'ds-hash', 'go-setup', 'go-types', 'go-routine', 're-component', 're-state'], 0.2);
  opened(an, ['re-hooks'], 0.17);
  const bich = enroll('cl-basic01', 'u-bich', 'sent', 58);
  done(bich, ['db-intro', 'db-table', 'ds-array', 'ds-stack', 'ds-hash', 'go-setup', 'go-types'], 2);
  opened(bich, ['go-routine'], 2);
  const cuong = enroll('cl-basic01', 'u-cuong', 'sent', 58);
  done(cuong, ['db-intro', 'db-table', 'ds-array', 'ds-stack', 'ds-hash', 'go-setup', 'go-types', 'go-routine', 'go-exercise', 're-component', 're-state', 're-hooks', 'web-html', 'web-css'], 1);
  opened(cuong, ['web-js'], 1);
  enroll('cl-basic01', 'u-dung', 'sent', 58); // never logged in; temp password expired
  const ha = enroll('cl-basic01', 'u-ha', 'sent', 57);
  done(ha, ['db-intro', 'db-table', 'ds-array', 'ds-stack'], 4);
  opened(ha, ['ds-hash'], 4);
  const khang = enroll('cl-basic01', 'u-khang', 'sent', 57);
  done(khang, ['db-intro', 'db-table'], 21);
  const thao = enroll('cl-basic01', 'u-thao', 'sent', 57, 'dropped');
  done(thao, ['db-intro'], 15);

  // basic02 — three weeks in
  const phong2 = enroll('cl-basic02', 'u-phong', 'sent', 22);
  done(phong2, ['db-intro', 'db-table', 'ds-array', 'ds-stack', 'ds-hash', 'go-setup'], 0.3);
  opened(phong2, ['go-types'], 0.3);
  const linh = enroll('cl-basic02', 'u-linh', 'sent', 22);
  done(linh, ['db-intro', 'db-table', 'ds-array'], 1);
  enroll('cl-basic02', 'u-minh', 'failed', 22);
  const nhan = enroll('cl-basic02', 'u-nhan', 'sent', 22);
  done(nhan, ['db-intro'], 6);
  opened(nhan, ['db-table'], 6);
  enroll('cl-basic02', 'u-quyen', 'sent', 1.5);
  // Phong also joined basic01 late as his second class (edge case: existing account invited again)
  const phong1 = enroll('cl-basic01', 'u-phong', 'sent', 12);
  done(phong1, ['db-intro', 'db-table', 'ds-array', 'ds-stack'], 2);

  // basic03 — draft, two invitations already sent
  enroll('cl-basic03', 'u-son', 'sent', 0.2);
  enroll('cl-basic03', 'u-tu', 'queued', 0.1);

  // Homework deadlines are set per class: "classId:lessonId" -> ISO time. No entry means no deadline.
  const deadlines = {
    ['cl-basic01:' + findLesson('ds-hw').id]: ago(20),
    ['cl-basic01:' + findLesson('go-hw').id]: ahead(0.8),
    ['cl-basic02:' + findLesson('ds-hw').id]: ahead(5)
  };

  // Submissions are immutable; "no" counts up per member and lesson. Only the latest one is graded.
  let subSeq = 0;
  const submissions = [];
  function submit(m, key, no, daysAgo, opts = {}) {
    const lesson = findLesson(key);
    const due = deadlines[m.classId + ':' + lesson.id];
    const s = { id: 'sub' + (++subSeq), memberId: m.id, lessonId: lesson.id, no, submittedAt: ago(daysAgo),
      repoUrl: opts.repoUrl || null, commitRef: opts.commitRef || null, files: opts.files || [],
      late: !!due && new Date(ago(daysAgo)) > new Date(due), status: opts.status || 'pending', grades: [] };
    submissions.push(s);
    return s;
  }
  function grade(s, by, daysAgo, points, comment, notes = []) {
    const hw = byLessonId(s.lessonId).homework;
    const scores = hw.criteria.map((c, i) => ({ criterionId: c.id, score: points[i], comment: notes[i] || '' }));
    const total = points.reduce((a, b) => a + b, 0), max = hw.criteria.reduce((a, c) => a + c.max, 0);
    const result = total * 100 >= hw.threshold * max ? 'passed' : 'rework';
    s.grades.push({ by, at: ago(daysAgo), scores, total, max, result, comment });
    s.status = result;
  }
  function byLessonId(id) {
    for (const sv of stageVersions) { const l = sv.lessons.find(x => x.id === id); if (l) return l; }
    throw new Error('seed: lesson id ' + id);
  }

  const an1 = submit(an, 'ds-hw', 1, 25, { repoUrl: 'https://github.com/hoangan-dev/ds-stack-queue', commitRef: '4e1c2a7' });
  grade(an1, 'u-gv', 23.5, [30, 25, 5], 'Stack và Queue chạy đúng nhưng thiếu test cho trường hợp rỗng và chưa có README. Bổ sung rồi nộp lại nhé.',
    ['Pop trên stack rỗng đang panic, cần trả về error.', '', 'Chưa có test và README.']);
  const an2 = submit(an, 'ds-hw', 2, 18, { repoUrl: 'https://github.com/hoangan-dev/ds-stack-queue', commitRef: 'b93f0d1' });
  grade(an2, 'u-gv', 17, [40, 28, 26], 'Đã sửa đủ các điểm lần trước. Test rõ ràng.');
  submit(bich, 'ds-hw', 1, 4.5, { files: [{ name: 'stack-queue-bich.zip', size: 184320 }] });
  const cuong1 = submit(cuong, 'ds-hw', 1, 22, { repoUrl: 'https://gitlab.com/cuongle/ds-go', commitRef: 'v0.1' });
  cuong1.status = 'superseded';
  const cuong2 = submit(cuong, 'ds-hw', 2, 21.5, { repoUrl: 'https://gitlab.com/cuongle/ds-go', commitRef: 'v0.2' });
  grade(cuong2, 'u-gv', 20.5, [38, 30, 24], 'Làm tốt. README nên ghi thêm phiên bản Go yêu cầu.');
  submit(cuong, 'go-hw', 1, 0.5, { repoUrl: 'https://gitlab.com/cuongle/go-csv-stats', files: [{ name: 'ket-qua-chay.png', size: 96256 }] });
  submit(phong1, 'ds-hw', 1, 1, { files: [{ name: 'phong-ds.zip', size: 212992 }, { name: 'README.md', size: 2150 }] });
  submit(phong2, 'ds-hw', 1, 3.5, { repoUrl: 'https://github.com/haiphong/ds-basic02' });
  submit(linh, 'ds-hw', 1, 0.5, { repoUrl: 'https://bitbucket.org/linhdo/stack-queue', commitRef: 'main' });

  // Quiz attempts are rebuilt from a right/wrong mask over the quiz's questions, in authored order.
  let attSeq = 0;
  const quizAttempts = [];
  function attempt(m, key, no, mask, daysAgo, minutes) {
    const lesson = findLesson(key), quiz = lesson.quiz;
    const startedAt = now - daysAgo * D;
    const answers = {};
    let score = 0, max = 0;
    quiz.questions.forEach((q, i) => {
      max += q.points;
      answers[q.id] = mask[i] ? q.options.filter(o => o.correct).map(o => o.id) : [q.options.find(o => !o.correct).id];
      if (mask[i]) score += q.points;
    });
    quizAttempts.push({ id: 'qa' + (++attSeq), memberId: m.id, lessonId: lesson.id, no,
      startedAt: new Date(startedAt).toISOString(),
      deadlineAt: quiz.timeLimit ? new Date(startedAt + quiz.timeLimit * 60e3).toISOString() : null,
      submittedAt: new Date(startedAt + minutes * 60e3).toISOString(),
      answers, order: quiz.questions.map(q => q.id), score, max, status: 'submitted' });
  }
  attempt(an, 'ds-quiz', 1, [0, 1, 1, 0, 1], 9, 6);
  attempt(an, 'ds-quiz', 2, [1, 1, 1, 1, 1], 8.8, 4);
  attempt(bich, 'ds-quiz', 1, [1, 0, 0, 1, 0], 2.2, 7);
  attempt(cuong, 'ds-quiz', 1, [1, 1, 1, 1, 1], 6, 5);
  attempt(phong1, 'ds-quiz', 1, [1, 1, 0, 1, 1], 2.1, 6);
  attempt(phong2, 'ds-quiz', 1, [1, 1, 1, 1, 0], 0.4, 5);
  attempt(an, 'go-quiz', 1, [1, 1, 0, 0, 1], 0.3, 12);
  attempt(cuong, 'go-quiz', 1, [1, 0, 0, 0, 1], 1.4, 14);
  attempt(cuong, 'go-quiz', 2, [1, 1, 1, 0, 0], 1.2, 13);
  // Attempts on the per-stage quizzes (qa10 onwards).
  attempt(an, 'db-practice', 1, [1, 1, 0, 1], 50, 4);          // qa10
  attempt(an, 'db-test', 1, [1, 1, 1, 1, 0], 48, 8);           // qa11
  attempt(bich, 'db-practice', 1, [1, 0, 1, 1], 47, 5);        // qa12
  attempt(bich, 'db-test', 1, [1, 1, 1, 0, 1], 46, 9);         // qa13
  attempt(cuong, 'db-test', 1, [1, 1, 1, 1, 1], 49, 6);        // qa14
  attempt(ha, 'db-test', 1, [1, 0, 1, 0, 0], 40, 10);          // qa15
  attempt(ha, 'db-test', 2, [1, 1, 1, 0, 1], 3.5, 9);          // qa16
  attempt(khang, 'db-test', 1, [1, 0, 0, 1, 0], 22, 10);       // qa17
  attempt(phong1, 'db-test', 1, [1, 1, 1, 1, 0], 9, 7);        // qa18
  attempt(phong2, 'db-test', 1, [1, 1, 0, 1, 1], 15, 8);       // qa19
  attempt(linh, 'db-practice', 1, [0, 1, 1, 0], 2, 6);         // qa20
  attempt(linh, 'db-test', 1, [1, 1, 0, 0, 1], 1.5, 9);        // qa21
  attempt(nhan, 'db-practice', 1, [1, 0, 0, 1], 5.5, 7);       // qa22
  attempt(an, 'ds-test', 1, [1, 1, 1, 1, 1], 15, 7);           // qa23
  attempt(cuong, 'ds-test', 1, [1, 1, 0, 1, 1], 19, 9);        // qa24
  attempt(bich, 'ds-test', 1, [1, 0, 0, 1, 0], 1.8, 10);       // qa25
  attempt(phong2, 'ds-test', 1, [1, 1, 1, 1, 0], 0.35, 8);     // qa26
  attempt(an, 'go-practice', 1, [1, 0, 1, 1], 1.5, 5);         // qa27
  attempt(cuong, 'go-practice', 1, [1, 1, 1, 0], 6.5, 4);      // qa28
  attempt(cuong, 're-practice', 1, [1, 1, 0, 1], 4, 5);        // qa29
  attempt(an, 're-practice', 1, [1, 1, 1, 0], 0.18, 6);        // qa30
  attempt(cuong, 're-test', 1, [1, 1, 1, 1, 0], 2.5, 11);      // qa31
  attempt(cuong, 'web-practice', 1, [1, 1, 1, 1], 1.05, 4);    // qa32
  // Bích is halfway through the Go practice quiz: opening it resumes at the first unanswered question.
  (() => {
    const lesson = findLesson('go-practice'), qs = lesson.quiz.questions;
    quizAttempts.push({ id: 'qa' + (++attSeq), memberId: bich.id, lessonId: lesson.id, no: 1, startedAt: ago(0, 1), deadlineAt: null,
      submittedAt: null, savedAt: ago(0, 0.9), answers: { [qs[0].id]: qs[0].options.filter((o) => o.correct).map((o) => o.id), [qs[1].id]: [qs[1].options[1].id] },
      order: qs.map((q) => q.id), score: null, max: null, status: 'open' });
    opened(bich, ['go-practice'], 0.04);
  })();

  // Teacher review of quiz attempts. Assessment attempts wait in the grading queue until a teacher
  // confirms them; a newer submit replaces one still waiting. Practice feedback is optional.
  function review(no, daysAgo, comment, notesByIndex = {}, by = 'u-gv') {
    const a = quizAttempts[no - 1], qs = byLessonId(a.lessonId).quiz.questions;
    const notes = Object.fromEntries(Object.entries(notesByIndex).map(([i, t]) => [qs[i].id, t]));
    a.review = { status: 'confirmed', by, at: ago(daysAgo), comment, notes, history: [] };
  }
  const pending = (...nos) => nos.forEach((no) => { quizAttempts[no - 1].review = { status: 'pending' }; });
  review(1, 8.95, 'Lượt đầu bạn nhầm stack với queue ở câu 1. Đọc lại bài Stack và Queue rồi làm lại, lượt sau sẽ ổn.',
    { 0: 'LIFO là stack: phần tử vào sau cùng được lấy ra trước.', 3: 'Cắt slice bỏ phần tử cuối là pop, không phải peek.' });
  quizAttempts[6].review = { status: 'pending' };
  quizAttempts[7].review = { status: 'superseded' };
  review(9, 1.1, 'Bạn nắm chắc cú pháp, cách báo lỗi và đồng bộ goroutine. Hai câu cuối sai do nhầm phiên bản Go và lệnh chạy; ôn lại bài Cài đặt và Hello World trước khi xin thêm lượt.',
    { 3: 'Range trên số nguyên có từ Go 1.22, xem ghi chú phát hành.', 4: 'go build để lại file nhị phân; go run thì không.' });
  review(11, 47.5, 'Nắm chắc khóa chính, khóa ngoại và ràng buộc. Câu về index cho JSONB sai, đọc lại bài Đọc thêm: chỉ mục.',
    { 4: 'JSONB và mảng dùng GIN; B-tree hợp với so sánh và sắp xếp.' });
  review(13, 45.5, 'Đạt ngưỡng. Lưu ý không đánh index tràn lan.', { 3: 'Mỗi index làm chậm INSERT và UPDATE, chỉ đánh cho cột hay lọc hoặc JOIN.' });
  review(14, 48.5, 'Đúng cả năm câu trong 6 phút. Rất tốt.');
  review(15, 39, 'Bạn còn nhầm giữa các kiểu ràng buộc khóa ngoại. Xem lại bài Thiết kế bảng và khóa, làm lại phần luyện tập rồi dùng lượt thứ hai.',
    { 1: 'RESTRICT từ chối xóa; CASCADE mới xóa dây chuyền.', 3: 'Index cho cột hay lọc và JOIN, không phải mọi cột.', 4: 'JSONB dùng GIN.' });
  pending(16);
  review(17, 21, 'Bạn mới học xong hai bài đầu của chặng. Học hết bài Đọc thêm: chỉ mục trước khi dùng lượt còn lại nhé.',
    { 1: 'Xem lại mục ON DELETE trong bài Thiết kế bảng và khóa.', 2: 'UNIQUE trên cặp cột chặn trùng cặp, không chặn từng cột.' });
  review(18, 8, 'Vào lớp muộn nhưng bắt kịp nhanh. Câu GIN sai, phần còn lại chắc.', { 4: 'Cột JSONB dùng GIN.' });
  review(19, 14, 'Đạt. Câu UNIQUE trên cặp cột cần đọc kỹ đề hơn.', { 2: 'Ràng buộc áp lên cặp (lớp, học viên), không áp lên từng cột riêng.' }, 'u-gv2');
  pending(21);
  review(23, 14, 'Đúng hết, phần BFS và xử lý xung đột trả lời rất chắc.');
  review(24, 18, 'Đạt. Câu danh sách liên kết: nhớ đổi lại là truy cập theo chỉ số chậm.', { 2: 'Ưu điểm là chèn, xóa ở giữa nhanh; truy cập phần tử thứ i là O(n).' });
  pending(25, 26);
  review(28, 6, 'Phần goroutine và WaitGroup ổn. Câu deadlock là lỗi hay gặp khi làm bài tập, chạy thử đoạn code đó một lần cho nhớ.',
    { 3: 'Channel không đệm chặn người gửi cho tới khi có người nhận.' });
  pending(31);
  const emails = [
    { id: 'em-s2', to: 'cuong.le@outlook.com', kind: 'quizReview', subject: 'Giảng viên đã xác nhận kết quả: Kiểm tra cuối chặng Go lượt #2 · 60% · Chưa đạt', at: ago(1.1) },
    { id: 'em-s1', to: 'an.nguyen@gmail.com', kind: 'quizReview', subject: 'Giảng viên đã nhận xét: Stack, Queue và Hash lượt #1', at: ago(8.95) },
    { id: 'em-s3', to: 'phong.dang@gmail.com', kind: 'quizReview', subject: 'Giảng viên đã xác nhận kết quả: Kiểm tra cuối chặng Database lượt #1 · 80% · Đạt', at: ago(8) },
    { id: 'em-s4', to: 'cuong.le@outlook.com', kind: 'quizReview', subject: 'Giảng viên đã nhận xét: Goroutine và channel lượt #1', at: ago(6) },
    { id: 'em-s5', to: 'phong.dang@gmail.com', kind: 'quizReview', subject: 'Giảng viên đã xác nhận kết quả: Kiểm tra cuối chặng Database lượt #1 · 80% · Đạt', at: ago(14) }
  ];

  // Teacher comment on a student's stage summary: "memberId:stageVersionId" -> { text, by, at }.
  const stageReviews = {
    [an.id + ':sv-ds-1']: { text: 'An nắm chắc stack và queue, bài nộp lại sửa đúng hướng. Chặng Go nên ôn lại phần đồng bộ goroutine trước bài kiểm tra.', by: 'u-gv', at: ago(16) },
    [cuong.id + ':sv-ds-1']: { text: 'Hoàn thành tốt, code gọn. Có thể hỗ trợ bạn khác trong nhóm.', by: 'u-gv', at: ago(19) }
  };

  const audit = [
    { at: ago(70), by: 'u-admin', action: 'Phát hành chặng', target: 'Database v1' },
    { at: ago(60), by: 'u-admin', action: 'Phát hành khóa học', target: 'Lập trình cơ bản v1' },
    { at: ago(58), by: 'u-admin', action: 'Kích hoạt lớp', target: 'basic01' },
    { at: ago(23.5), by: 'u-gv', action: 'Chấm bài', target: 'Nguyễn Hoàng An · Cài đặt Stack và Queue #1 · Cần làm lại' },
    { at: ago(22), by: 'u-admin', action: 'Mời học viên', target: 'basic02 · 5 học viên' },
    { at: ago(21), by: 'u-admin', action: 'Đặt hạn nộp', target: 'basic01 · Cài đặt Stack và Queue' },
    { at: ago(20.5), by: 'u-gv', action: 'Chấm bài', target: 'Lê Văn Cường · Cài đặt Stack và Queue #2 · Đạt' },
    { at: ago(20), by: 'u-admin', action: 'Kích hoạt lớp', target: 'basic02' },
    { at: ago(19), by: 'u-gv', action: 'Nhận xét tổng kết chặng', target: 'Lê Văn Cường · Data structure' },
    { at: ago(17), by: 'u-gv', action: 'Chấm bài', target: 'Nguyễn Hoàng An · Cài đặt Stack và Queue #2 · Đạt' },
    { at: ago(16), by: 'u-gv', action: 'Nhận xét tổng kết chặng', target: 'Nguyễn Hoàng An · Data structure' },
    { at: ago(15), by: 'u-admin', action: 'Vô hiệu hóa tài khoản', target: 'thao.vo@gmail.com' },
    { at: ago(14), by: 'u-gv2', action: 'Xác nhận quiz', target: 'Đặng Hải Phong · Kiểm tra cuối chặng Database #1 · 80% · Đạt' },
    { at: ago(12), by: 'u-admin', action: 'Mời học viên', target: 'basic01 · phong.dang@gmail.com' },
    { at: ago(8.95), by: 'u-gv', action: 'Nhận xét quiz', target: 'Nguyễn Hoàng An · Stack, Queue và Hash #1' },
    { at: ago(8), by: 'u-gv', action: 'Xác nhận quiz', target: 'Đặng Hải Phong · Kiểm tra cuối chặng Database #1 · 80% · Đạt' },
    { at: ago(6), by: 'u-gv', action: 'Nhận xét quiz', target: 'Lê Văn Cường · Goroutine và channel #1' },
    { at: ago(3), by: 'u-admin', action: 'Tạo lớp', target: 'basic03' },
    { at: ago(2.8), by: 'u-admin', action: 'Nhân bản chặng', target: 'Golang basic v1 → v2' },
    { at: ago(2.5), by: 'u-gv', action: 'Đặt hạn nộp', target: 'basic01 · Đọc CSV và thống kê' },
    { at: ago(1.5), by: 'u-admin', action: 'Gửi lại lời mời', target: 'quyen.ly@gmail.com' },
    { at: ago(1.1), by: 'u-gv', action: 'Xác nhận quiz', target: 'Lê Văn Cường · Kiểm tra cuối chặng Go #2 · 60% · Chưa đạt' },
    { at: ago(0.2), by: 'u-admin', action: 'Mời học viên', target: 'basic03 · 2 học viên' }
  ];

  return { session: null, users, stages, stageVersions, courses, courseVersions, classes, members, invitations, progress, audit, extraContent,
    deadlines, submissions, quizAttempts, attemptGrants: [], stageReviews, emails, jobRuns: {}, seededAt: new Date(now).toISOString() };
};
