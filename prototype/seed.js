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
- CSS và JavaScript dễ bám vào hơn.`
  };

  let lessonSeq = 0;
  const L = (key, type, title, required, extra = {}) => ({
    id: 'l' + (++lessonSeq), key, type, title, required, ...extra
  });

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
      L('ds-hash', 'video', 'Hash table', true, { video: 'ds-03-hash-table.mp4', duration: '16:45' })
    ] },
    { id: 'sv-go-1', stageId: 'st-go', no: 1, status: 'published', clonedFrom: null, publishedAt: ago(66), lessons: [
      L('go-setup', 'video', 'Cài đặt và Hello World', true, { video: 'go-01-cai-dat.mp4', duration: '12:02' }),
      L('go-types', 'markdown', 'Kiểu dữ liệu và hàm', true, { content: md.goTypes }),
      L('go-routine', 'video', 'Goroutine cơ bản', true, { video: 'go-03-goroutine.mp4', duration: '25:31' }),
      L('go-exercise', 'markdown', 'Bài tập tổng hợp', false, { content: md.goExercise })
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

  const audit = [
    { at: ago(70), by: 'u-admin', action: 'Phát hành chặng', target: 'Database v1' },
    { at: ago(60), by: 'u-admin', action: 'Phát hành khóa học', target: 'Lập trình cơ bản v1' },
    { at: ago(58), by: 'u-admin', action: 'Kích hoạt lớp', target: 'basic01' },
    { at: ago(22), by: 'u-admin', action: 'Mời học viên', target: 'basic02 · 5 học viên' },
    { at: ago(20), by: 'u-admin', action: 'Kích hoạt lớp', target: 'basic02' },
    { at: ago(15), by: 'u-admin', action: 'Vô hiệu hóa tài khoản', target: 'thao.vo@gmail.com' },
    { at: ago(12), by: 'u-admin', action: 'Mời học viên', target: 'basic01 · phong.dang@gmail.com' },
    { at: ago(3), by: 'u-admin', action: 'Tạo lớp', target: 'basic03' },
    { at: ago(1.5), by: 'u-admin', action: 'Gửi lại lời mời', target: 'quyen.ly@gmail.com' },
    { at: ago(0.2), by: 'u-admin', action: 'Mời học viên', target: 'basic03 · 2 học viên' }
  ];

  return { session: null, users, stages, stageVersions, courses, courseVersions, classes, members, invitations, progress, audit, extraContent, seededAt: new Date(now).toISOString() };
};
