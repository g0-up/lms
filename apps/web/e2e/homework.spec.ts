/*
 * Bài tập và chấm bài (FR-61–FR-64) trên stack thật: học viên An nộp "Bài tập: Đọc CSV và thống kê" của basic01
 * (repo kèm file), nộp lại khi đang chờ chấm; giảng viên Hương đặt hạn nộp ở tab "Bài tập" của lớp, chấm từ hàng
 * chờ được Cần làm lại; An nộp lại; giảng viên chấm Đạt; An thấy điểm từng tiêu chí và không còn nút nộp.
 * Mailpit nhận hai thư "Bài đã được chấm"; file nộp tải về với `Content-Disposition: attachment`.
 * Ghi bài nộp, điểm và hạn nộp vào DB seed, nên chạy @serial và seed lại ở beforeAll/afterAll.
 */
import { expect, test, type APIRequestContext, type Browser, type Page } from "@playwright/test";
import { apiAs, browserAs, type Role } from "./support/auth";
import { closeDb, ids, one, query } from "./support/db";
import { messagesTo, waitForNewMessage, type MailSummary } from "./support/mailpit";
import { PIXEL_PNG } from "./support/media";
import { seedReset } from "./support/seed";

const AN = "an.nguyen@gmail.com";
const AN_NAME = "Nguyễn Hoàng An";
const HOMEWORK = "Bài tập: Đọc CSV và thống kê";
const SHORT = "Đọc CSV và thống kê";
const REPO = "https://github.com/hoangan-dev/go-csv-stats";
const GRADED_SUBJECT = "Bài đã được chấm";
const CRITERIA = [
  "Đọc CSV bằng encoding/csv, xử lý lỗi file không tồn tại",
  "Thống kê đúng theo lớp và sắp xếp theo mã lớp",
  "Code rõ ràng, có README",
] as const;
const README = Buffer.from("# Thống kê CSV\n\nChạy: `go run . hocvien.csv`\n");
const DAY_MS = 86_400_000;
const VN_OFFSET_MS = 7 * 3_600_000;

let classId: string;
let lessonId: string;
let teacherApi: APIRequestContext;
/** Thư tới An trước khi chấm; mỗi lần chấm thêm thư vừa nhận vào. */
let mailSeen: MailSummary[];
let mailBefore: Set<string>;

const lessonPath = () => `/learn/classes/${classId}/lessons/${lessonId}`;

/** Trang mới trong context mang phiên của vai trò; đóng context khi xong. */
async function openAs(browser: Browser, role: Role, path: string): Promise<{ page: Page; close: () => Promise<void> }> {
  const context = await browserAs(browser, role);
  const page = await context.newPage();
  await page.goto(path);
  return { page, close: () => context.close() };
}

/** Một dòng trong "Lịch sử nộp" của học viên. */
function historyItem(page: Page, no: number) {
  return page.getByRole("listitem").filter({ hasText: `Lần nộp #${String(no)}` });
}

/** Ngày `daysAhead` ngày tới theo giờ Việt Nam, 23:59, dạng giá trị của `datetime-local`. */
function vnLocalInput(daysAhead: number): string {
  const vn = new Date(Date.now() + daysAhead * DAY_MS + VN_OFFSET_MS);
  return `${vn.toISOString().slice(0, 10)}T23:59`;
}

/** Từ hàng chờ, mở bài của An và chấm theo `points` (thứ tự tiêu chí) cùng nhận xét chung. */
async function gradeFromQueue(page: Page, points: readonly number[], comment: string): Promise<void> {
  await page.goto("/teach/grading");
  await page.getByRole("link", { name: `Chấm bài của ${AN_NAME}: ${SHORT}` }).click();
  await expect(page.getByRole("heading", { level: 1, name: `${AN_NAME} · ${SHORT}` })).toBeVisible();
  for (const [i, description] of CRITERIA.entries()) {
    await page.getByRole("spinbutton", { name: description }).fill(String(points[i]));
  }
  await page.getByRole("textbox", { name: /Nhận xét chung/ }).fill(comment);
  await page.getByRole("button", { name: "Lưu kết quả", exact: true }).click();
}

test.describe("Bài tập và chấm bài", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });

  test.beforeAll(async () => {
    await seedReset();
    classId = await ids.classId("basic01");
    lessonId = await ids.lessonInClass("basic01", "go-hw");
    teacherApi = await apiAs("teacher");
    mailSeen = await messagesTo(AN);
    mailBefore = new Set(mailSeen.map((mail) => mail.ID));
  });

  test.afterAll(async () => {
    await teacherApi.dispose();
    await closeDb();
    await seedReset();
  });

  test("học viên nộp repo kèm file; nộp lại khi đang chờ chấm thay bản cũ", async ({ browser }) => {
    const { page, close } = await openAs(browser, "student", lessonPath());
    // Đề bài seed mở đầu bằng chính tiêu đề của nó: tiêu đề trang và tiêu đề đề bài.
    await expect(page.getByRole("heading", { level: 1, name: HOMEWORK })).toHaveCount(2);
    await expect(page.getByText("GitHub, GitLab hoặc Bitbucket. Repo phải để public để giảng viên mở được.")).toBeVisible();

    await page.getByLabel(/^Đường dẫn repo/).fill(REPO);
    await page.getByLabel(/^File đính kèm/).setInputFiles({ name: "README.md", mimeType: "text/markdown", buffer: README });
    await page.getByRole("button", { name: "Nộp bài" }).click();
    await expect(page.getByText("Đã nộp lần #1. Bạn sẽ nhận email khi có kết quả.")).toBeVisible({ timeout: 30_000 });
    await expect(historyItem(page, 1).getByText("Chờ chấm")).toBeVisible();
    await expect(historyItem(page, 1)).toContainText("README.md");

    // Lần nộp mới thay bản đang chờ: giảng viên chỉ chấm lần mới nhất.
    await expect(page.getByText(/^Lần nộp #1 đang chờ chấm\./)).toBeVisible();
    await expect(page.getByLabel(/^Đường dẫn repo/)).toHaveValue(REPO);
    await page.getByLabel(/^Commit hoặc tag/).fill("v0.2");
    await page.getByLabel(/^File đính kèm/).setInputFiles({ name: "ket-qua.png", mimeType: "image/png", buffer: PIXEL_PNG });
    await page.getByRole("button", { name: "Nộp lại" }).click();
    await expect(page.getByText(/^Đã nộp lần #2, thay lần #1 đang chờ chấm\./)).toBeVisible({ timeout: 30_000 });
    await expect(historyItem(page, 1).getByText("Đã thay bằng lần nộp sau")).toBeVisible();
    await expect(historyItem(page, 2).getByText("Chờ chấm")).toBeVisible();

    const rows = await query<{ no: number; status: string }>(
      `SELECT s.attempt_no AS no, s.status FROM submissions s JOIN class_members m ON m.id = s.class_member_id
         JOIN users u ON u.id = m.user_id WHERE m.class_id = $1 AND s.lesson_id = $2 AND u.email_normalized = $3
        ORDER BY s.attempt_no`,
      [classId, lessonId, AN],
    );
    expect(rows).toEqual([
      { no: 1, status: "superseded" },
      { no: 2, status: "pending" },
    ]);
    await close();
  });

  test("giảng viên đặt hạn nộp ở tab Bài tập của lớp", async ({ browser }) => {
    const { page, close } = await openAs(browser, "teacher", `/teach/classes/${classId}?tab=homework`);
    const card = page.getByRole("region", { name: HOMEWORK });
    await expect(card.getByRole("link", { name: `Chấm bài của ${AN_NAME}: ${SHORT}` })).toBeVisible();

    await card.getByRole("button", { name: "Đổi hạn nộp" }).click();
    const dialog = page.getByRole("dialog", { name: `Hạn nộp: ${SHORT}` });
    const due = vnLocalInput(7);
    await dialog.getByLabel(/^Hạn nộp/).fill(due);
    await dialog.getByRole("button", { name: "Lưu hạn nộp" }).click();
    await expect(page.getByText("Đã lưu hạn nộp.")).toBeVisible();
    await expect(dialog).toBeHidden();

    const [y, m, d] = due.slice(0, 10).split("-");
    await expect(card.getByText(new RegExp(`^Hạn ${d}/${m}/${y} 23:59 · `))).toBeVisible();
    const row = await one<{ due_at: Date }>(
      "SELECT due_at FROM class_lesson_deadlines WHERE class_id = $1 AND lesson_id = $2",
      [classId, lessonId],
    );
    expect(row.due_at.toISOString()).toBe(new Date(`${due}:00+07:00`).toISOString());
    await close();
  });

  test("giảng viên chấm từ hàng chờ được Cần làm lại; file nộp tải về dạng attachment", async ({ browser }) => {
    const { page, close } = await openAs(browser, "teacher", "/teach/grading");
    await expect(page.getByRole("heading", { level: 1, name: "Chấm bài" })).toBeVisible();

    await page.getByRole("link", { name: `Chấm bài của ${AN_NAME}: ${SHORT}` }).click();
    await expect(page.getByRole("heading", { name: "Bài nộp #2" })).toBeVisible();
    const file = page.getByRole("link", { name: /ket-qua\.png/ });
    const href = await file.getAttribute("href");
    expect(href).toMatch(/^\/api\/v1\/media\/[0-9a-f-]{36}\/content$/);
    // API chuyển hướng tới URL ký của storage, trả về để trình duyệt lưu file chứ không xem trước.
    const download = await teacherApi.get(href ?? "");
    expect(download.status()).toBe(200);
    expect(download.headers()["content-disposition"]).toMatch(/^attachment/);

    await gradeFromQueue(page, [30, 15, 5], "Chưa sắp xếp theo mã lớp và thiếu README. Sửa rồi nộp lại nhé.");
    await expect(page.getByText(/^Đã lưu 50\/100 · Cần làm lại\. Email kết quả đã gửi cho học viên\./)).toBeVisible();
    const first = await waitForNewMessage(AN, mailSeen);
    expect(first.Subject).toContain(`${GRADED_SUBJECT}: ${HOMEWORK} · Cần làm lại`);
    mailSeen = [...mailSeen, first];
    await close();
  });

  test("học viên nộp lại sau Cần làm lại, giảng viên chấm Đạt", async ({ browser }) => {
    const student = await openAs(browser, "student", lessonPath());
    await expect(student.page.getByRole("heading", { name: "Kết quả lần nộp #2" })).toBeVisible();
    await expect(student.page.getByText("50/100", { exact: false }).first()).toBeVisible();
    await student.page.getByLabel(/^Commit hoặc tag/).fill("v0.3");
    await student.page.getByRole("button", { name: "Nộp lại" }).click();
    await expect(student.page.getByText(/^Đã nộp lần #3\./)).toBeVisible({ timeout: 30_000 });
    await student.close();

    const teacher = await openAs(browser, "teacher", "/teach/grading");
    await gradeFromQueue(teacher.page, [40, 35, 15], "Đã sửa đủ. Code gọn, README rõ ràng.");
    await expect(teacher.page.getByText(/^Đã lưu 90\/100 · Đạt\. Email kết quả đã gửi cho học viên\./)).toBeVisible();
    const second = await waitForNewMessage(AN, mailSeen);
    expect(second.Subject).toContain(`${GRADED_SUBJECT}: ${HOMEWORK} · Đạt`);
    await teacher.close();
  });

  test("học viên thấy điểm từng tiêu chí, không còn nút nộp; hai thư báo đã chấm", async ({ browser }) => {
    const { page, close } = await openAs(browser, "student", lessonPath());
    await expect(page.getByRole("heading", { name: "Kết quả lần nộp #3" })).toBeVisible();
    const scores = page.getByRole("table", { name: "Điểm từng tiêu chí của lần nộp #3" });
    for (const [description, points] of [
      [CRITERIA[0], "40/40"],
      [CRITERIA[1], "35/40"],
      [CRITERIA[2], "15/20"],
    ] as const) {
      await expect(scores.getByRole("row").filter({ hasText: description })).toContainText(points);
    }
    await expect(page.getByText("Bài đã được chấm Đạt nên không cần nộp lại.")).toBeVisible();
    await expect(page.getByRole("button", { name: /^Nộp/ })).toHaveCount(0);
    await expect(historyItem(page, 2).getByText("Cần làm lại")).toBeVisible();
    await expect(historyItem(page, 3).getByText("Đạt", { exact: true })).toBeVisible();
    await close();

    const graded = (await messagesTo(AN)).filter(
      (mail) => !mailBefore.has(mail.ID) && mail.Subject.includes(`${GRADED_SUBJECT}: ${HOMEWORK}`),
    );
    expect(graded).toHaveLength(2);
  });
});
