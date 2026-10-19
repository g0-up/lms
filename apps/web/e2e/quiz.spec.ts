/*
 * Quiz (FR-75–FR-77) trên stack thật, học viên An lớp basic01:
 * - luyện tập "Luyện tập: Truy vấn SQL cơ bản": chọn, tải lại trang tiếp tục đúng câu đang xem, nộp với 1 câu trống,
 *   thấy đáp án và giải thích;
 * - bài tập "Kiểm tra cuối chặng Go" (lượt 2/2): không response nào chứa `isCorrect`, hộp "Nộp bài?" nêu đúng số câu
 *   chưa trả lời, axe trên màn làm bài; kết quả chỉ có điểm, chờ xác nhận;
 * - bài tập "Kiểm tra cuối chặng Database" (lượt 2/2) với `page.clock`: cảnh báo 5 phút, 1 phút, hết giờ client tự nộp
 *   một lần. Đồng hồ chỉ tua phía trình duyệt; tự nộp phía server có test integration riêng;
 * - giảng viên Hương xác nhận lượt Go từ hàng chờ (axe trên trang xác nhận), Mailpit nhận thư; An thấy nhận xét và
 *   ghi chú theo câu, không thấy đáp án;
 * - giảng viên cấp thêm 1 lượt "Kiểm tra cuối chặng Go" cho Cường (đã hết 2/2 lượt mà chưa đạt), kèm lý do; Cường
 *   thấy "· đã được cấp thêm" và nút "Làm lượt mới" bấm được.
 * Ghi lượt làm bài và nhận xét vào DB seed, nên chạy @serial và seed lại ở beforeAll/afterAll.
 */
import { expect, test, type Browser, type Page } from "@playwright/test";
import { runAxe, waitForPage } from "./support/a11y";
import { browserAs, loginContext, seedPassword, type Role } from "./support/auth";
import { closeDb, ids, query } from "./support/db";
import { messagesTo, waitForNewMessage, type MailSummary } from "./support/mailpit";
import { seedReset } from "./support/seed";

const AN = "an.nguyen@gmail.com";
const AN_NAME = "Nguyễn Hoàng An";
const TEACHER_NAME = "Lê Thu Hương";
const PRACTICE = "Luyện tập: Truy vấn SQL cơ bản";
const GO_TEST = "Kiểm tra cuối chặng Go";
const DB_TEST = "Kiểm tra cuối chặng Database";
const REVIEWED_SUBJECT = `Giảng viên đã xác nhận kết quả: ${GO_TEST} lượt #2`;
const COMMENT = "Lượt này chắc hơn lượt đầu. Ôn lại phần đồng bộ goroutine trước khi học chặng sau.";
const NOTE = "Đọc lại cú pháp khai báo ngắn bên trong hàm.";
const CUONG = "cuong.le@outlook.com";
const CUONG_NAME = "Lê Văn Cường";
const GRANT_REASON = "Mất kết nối khi đang làm bài";

let classId: string;
let practiceId: string;
let goTestId: string;
let dbTestId: string;
/** Lượt 2 của An ở quiz bài tập Go, lấy từ `?result=` sau khi nộp. */
let goAttemptId: string;
let mailSeen: MailSummary[];

const lessonPath = (lessonId: string) => `/learn/classes/${classId}/lessons/${lessonId}`;

/** Trang mới trong context mang phiên của vai trò; `beforeGoto` chạy trước khi mở trang (cài đồng hồ giả). */
async function openAs(
  browser: Browser,
  role: Role,
  path: string,
  beforeGoto?: (page: Page) => Promise<void>,
): Promise<{ page: Page; close: () => Promise<void> }> {
  const context = await browserAs(browser, role);
  const page = await context.newPage();
  await beforeGoto?.(page);
  await page.goto(path);
  return { page, close: () => context.close() };
}

/** Câu đang hiện: fieldset (tên là nội dung câu) chứa các ô chọn đáp án. */
function currentQuestion(page: Page) {
  return page.getByRole("group").filter({ has: page.locator('input[type="radio"], input[type="checkbox"]') });
}

/** Dòng "Câu x/n · …" của câu đang hiện. */
function questionLine(page: Page, no: number, total: number) {
  return page.getByText(new RegExp(`^Câu ${String(no)}/${String(total)} · `));
}

/** Chọn phương án đầu tiên của câu đang hiện và chờ server lưu. */
async function answerCurrent(page: Page): Promise<void> {
  const saved = page.waitForResponse((res) => res.request().method() === "PUT" && /\/answers\/[^/]+$/.test(res.url()));
  await currentQuestion(page).locator("label").first().click();
  expect((await saved).ok()).toBe(true);
}

/** Nút trên bản đồ câu hỏi. */
function mapButton(page: Page, no: number, answered: boolean) {
  return page
    .getByRole("navigation", { name: "Chuyển nhanh đến câu hỏi" })
    .getByRole("button", { name: `Câu ${String(no)}, ${answered ? "đã trả lời" : "chưa trả lời"}`, exact: true });
}

/** Nút "Nộp bài" ở thanh trạng thái (ở câu cuối có thêm nút cùng tên dưới câu hỏi, cùng mở hộp xác nhận). */
function submitButton(page: Page) {
  return page.getByRole("button", { name: "Nộp bài", exact: true }).first();
}

/** Ghi lại body JSON của mọi response API của trang, để kiểm không lộ đáp án. */
function recordApiResponses(page: Page): () => Promise<{ path: string; body: string }[]> {
  const bodies: Promise<{ path: string; body: string }>[] = [];
  page.on("response", (res) => {
    const url = new URL(res.url());
    if (!url.pathname.startsWith("/api/v1/") || !(res.headers()["content-type"] ?? "").includes("json")) return;
    bodies.push(
      res.text().then(
        (body) => ({ path: url.pathname, body }),
        () => ({ path: url.pathname, body: "" }),
      ),
    );
  });
  return () => Promise.all(bodies);
}

/** Trạng thái nộp và xác nhận của một lượt (đọc DB). */
async function attemptState(attemptId: string) {
  const rows = await query<{ review_status: string | null; submitted: boolean; no: number }>(
    "SELECT review_status, submitted_at IS NOT NULL AS submitted, attempt_no AS no FROM quiz_attempts WHERE id = $1",
    [attemptId],
  );
  return rows;
}

test.describe("Quiz luyện tập, bài tập và xác nhận", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });

  test.beforeAll(async () => {
    await seedReset();
    classId = await ids.classId("basic01");
    [practiceId, goTestId, dbTestId] = await Promise.all([
      ids.lessonInClass("basic01", "db-practice"),
      ids.lessonInClass("basic01", "go-quiz"),
      ids.lessonInClass("basic01", "db-test"),
    ]);
    mailSeen = await messagesTo(AN);
  });

  test.afterAll(async () => {
    await closeDb();
    await seedReset();
  });

  test("luyện tập: tải lại trang tiếp tục đúng câu, nộp với 1 câu trống và thấy đáp án", async ({ browser }) => {
    const { page, close } = await openAs(browser, "student", lessonPath(practiceId));
    await expect(page.getByRole("heading", { level: 1, name: PRACTICE })).toBeVisible();
    await page.getByRole("button", { name: "Làm lượt mới" }).click();

    await expect(questionLine(page, 1, 4)).toBeVisible();
    await expect(page.getByText("Không giới hạn thời gian")).toBeVisible();
    await answerCurrent(page);
    await expect(page.getByText(/^Đã lưu \d{2}:\d{2}$/)).toBeVisible();
    await page.getByRole("button", { name: "Câu tiếp" }).click();
    await expect(questionLine(page, 2, 4)).toBeVisible();
    await answerCurrent(page);
    await page.getByRole("button", { name: "Câu tiếp" }).click();
    await expect(questionLine(page, 3, 4)).toBeVisible();
    await expect(page.getByText("2/4 đã trả lời")).toBeVisible();

    // Câu 3 chưa trả lời mà đang xem: tải lại vẫn mở câu 3 cùng hai câu đã lưu.
    await page.reload();
    await expect(questionLine(page, 3, 4)).toBeVisible();
    await expect(mapButton(page, 1, true)).toBeVisible();
    await expect(mapButton(page, 2, true)).toBeVisible();
    await expect(mapButton(page, 3, false)).toHaveAttribute("aria-current", "step");

    await mapButton(page, 4, false).click();
    await expect(questionLine(page, 4, 4)).toBeVisible();
    await answerCurrent(page);
    await submitButton(page).click();
    const dialog = page.getByRole("dialog", { name: "Nộp bài?" });
    await expect(dialog.getByText("1 câu chưa trả lời", { exact: true })).toBeVisible();
    await dialog.getByRole("button", { name: "Nộp bài", exact: true }).click();

    await expect(page.getByText("Đã nộp bài.")).toBeVisible();
    await expect(page).toHaveURL(/\?result=[0-9a-f-]{36}$/);
    await expect(page.getByRole("heading", { level: 2, name: /^Lượt 2 · nộp / })).toBeVisible();
    // Bốn câu, năm phương án đúng (câu DDL có hai), đúng một câu trống.
    await expect(page.getByText(/\(đáp án đúng\)/)).toHaveCount(5);
    await expect(page.getByText(/\(bạn chọn\)/)).toHaveCount(3);
    await expect(page.getByText("Chưa trả lời · 0 điểm", { exact: true })).toHaveCount(1);
    await expect(page.getByText("Giải thích.", { exact: true })).toHaveCount(4);
    await close();
  });

  test("bài tập: không response nào lộ đáp án, hộp Nộp bài? nêu số câu trống; axe trên màn làm bài", async ({
    browser,
  }) => {
    const { page, close } = await openAs(browser, "student", lessonPath(goTestId));
    const responses = recordApiResponses(page);
    await expect(page.getByRole("heading", { level: 1, name: GO_TEST })).toBeVisible();
    await expect(page.getByText("1/2 đã dùng · tính lượt tốt nhất")).toBeVisible();
    await page.getByRole("button", { name: "Làm lượt mới" }).click();

    await expect(questionLine(page, 1, 5)).toBeVisible();
    await expect(
      page.getByText("Lượt 2/2. Hết giờ, hệ thống tự nộp với các câu đã lưu. Rời trang không dừng đồng hồ."),
    ).toBeVisible();
    await expect(page.getByRole("timer", { name: "Thời gian còn lại" })).toHaveText(/^(15:00|14:\d{2})$/);
    await waitForPage(page);
    const { violations } = await runAxe(page);
    expect(violations, JSON.stringify(violations)).toEqual([]);

    await answerCurrent(page);
    await mapButton(page, 3, false).click();
    await answerCurrent(page);
    await mapButton(page, 5, false).click();
    await answerCurrent(page);
    await expect(page.getByText("3/5 đã trả lời")).toBeVisible();

    await submitButton(page).click();
    const dialog = page.getByRole("dialog", { name: "Nộp bài?" });
    await expect(dialog.getByText("2 câu chưa trả lời", { exact: true })).toBeVisible();

    const beforeSubmit = await responses();
    // Đề bài đến từ response của POST …/attempts (client đưa thẳng vào cache), nên kiểm tra nó đã được ghi lại.
    expect(beforeSubmit.some((r) => r.path.endsWith("/attempts") && r.body.includes('"questions"'))).toBe(true);
    expect(beforeSubmit.filter((r) => r.body.includes('"isCorrect"')).map((r) => r.path)).toEqual([]);

    await dialog.getByRole("button", { name: "Nộp bài", exact: true }).click();
    await expect(page.getByText("Đã nộp bài.")).toBeVisible();
    await expect(page).toHaveURL(/\?result=[0-9a-f-]{36}$/);
    goAttemptId = new URL(page.url()).searchParams.get("result") ?? "";

    await expect(page.getByText(/^Chờ giảng viên xác nhận\./)).toBeVisible();
    await expect(page.getByText("Quiz bài tập chỉ hiện điểm và kết quả Đạt, không hiện đáp án.")).toBeVisible();
    await expect(page.getByText(/\(đáp án đúng\)/)).toHaveCount(0);
    // Kết quả bài tập cũng không mang đáp án.
    expect((await responses()).filter((r) => r.body.includes('"isCorrect"')).map((r) => r.path)).toEqual([]);
    expect(await attemptState(goAttemptId)).toEqual([{ review_status: "pending", submitted: true, no: 2 }]);
    await close();
  });

  test("bài tập có giờ: cảnh báo 5 phút và 1 phút, hết giờ client tự nộp một lần", async ({ browser }) => {
    const { page, close } = await openAs(browser, "student", lessonPath(dbTestId), (p) => p.clock.install());
    let submits = 0;
    page.on("request", (req) => {
      if (req.method() === "POST" && /\/me\/attempts\/[^/]+\/submit$/.test(req.url())) submits += 1;
    });
    await expect(page.getByRole("heading", { level: 1, name: DB_TEST })).toBeVisible();
    await page.getByRole("button", { name: "Làm lượt mới" }).click();
    const timer = page.getByRole("timer", { name: "Thời gian còn lại" });
    await expect(timer).toHaveText(/^(10:00|09:\d{2})$/);
    await answerCurrent(page);

    // Sau khi tua không lưu hay tải lại lượt: dữ liệu mới mang giờ server thật và đếm lại từ đó.
    await page.clock.fastForward("05:01");
    await expect(page.getByText("Còn 5 phút. Câu trả lời đã chọn được lưu tự động.")).toBeVisible();
    await expect(timer).toHaveText(/^04:\d{2}$/);

    await page.clock.fastForward("04:00");
    await expect(page.getByText("Còn 1 phút. Câu trả lời đã chọn được lưu tự động.")).toBeVisible();

    await page.clock.fastForward("01:00");
    await expect(page.getByText("Hết giờ: hệ thống đã tự nộp bài với các câu đã lưu.")).toBeVisible();
    await expect(page).toHaveURL(/\?result=[0-9a-f-]{36}$/);
    await expect(page.getByRole("heading", { level: 2, name: /^Lượt 2 · nộp / })).toBeVisible();
    expect(submits).toBe(1);
    const attemptId = new URL(page.url()).searchParams.get("result") ?? "";
    expect(await attemptState(attemptId)).toEqual([{ review_status: "pending", submitted: true, no: 2 }]);
    await close();
  });

  test("giảng viên xác nhận lượt từ hàng chờ, học viên nhận thư; axe trên trang xác nhận", async ({ browser }) => {
    const { page, close } = await openAs(browser, "teacher", "/teach/grading");
    await expect(page.getByRole("heading", { level: 1, name: "Chấm bài" })).toBeVisible();
    await page.getByRole("link", { name: `Xác nhận bài của ${AN_NAME}: ${GO_TEST}`, exact: true }).click();

    await expect(page).toHaveURL(new RegExp(`/teach/grading/quiz/${goAttemptId}`));
    await expect(page.getByRole("heading", { level: 1, name: `${AN_NAME} · ${GO_TEST}` })).toBeVisible();
    await expect(page.getByRole("form", { name: "Xác nhận kết quả" })).toBeVisible();
    // Giảng viên thấy đáp án đúng của từng câu.
    await expect(page.getByText(/\(đáp án đúng\)/).first()).toBeAttached();
    await waitForPage(page);
    const { violations } = await runAxe(page);
    expect(violations, JSON.stringify(violations)).toEqual([]);

    await page.getByRole("textbox", { name: "Ghi chú cho câu 1 (không bắt buộc)", exact: true }).fill(NOTE);
    await page.getByRole("textbox", { name: /^Nhận xét chung/ }).fill(COMMENT);
    await page.getByRole("button", { name: "Xác nhận", exact: true }).click();
    await expect(page.getByText(/^Đã xác nhận kết quả\. Email nhận xét đã gửi cho học viên\./)).toBeVisible();

    expect(await attemptState(goAttemptId)).toEqual([{ review_status: "confirmed", submitted: true, no: 2 }]);
    const mail = await waitForNewMessage(AN, mailSeen);
    expect(mail.Subject).toContain(REVIEWED_SUBJECT);
    mailSeen = [...mailSeen, mail];
    await close();
  });

  test("học viên thấy nhận xét và ghi chú theo câu, không thấy đáp án", async ({ browser }) => {
    const { page, close } = await openAs(browser, "student", `${lessonPath(goTestId)}?result=${goAttemptId}`);
    await expect(page.getByRole("heading", { level: 2, name: /^Lượt 2 · nộp / })).toBeVisible();
    await expect(page.getByRole("heading", { level: 2, name: `Giảng viên đã xác nhận · ${TEACHER_NAME}` })).toBeVisible();
    await expect(page.getByText(COMMENT, { exact: true })).toBeVisible();
    await expect(page.getByText("Câu 1.", { exact: true })).toBeVisible();
    await expect(page.getByText(NOTE, { exact: true })).toBeVisible();
    await expect(page.getByText(/^Chờ giảng viên xác nhận\./)).toHaveCount(0);
    await expect(page.getByText(/\(đáp án đúng\)/)).toHaveCount(0);
    await expect(page.getByText("Giải thích.", { exact: true })).toHaveCount(0);
    await close();
  });

  test("giảng viên cấp thêm 1 lượt cho học viên đã hết lượt, học viên thấy Làm lượt mới", async ({ browser }) => {
    const { page, close } = await openAs(browser, "teacher", `/teach/classes/${classId}?tab=quiz`);
    const card = page.getByRole("region", { name: GO_TEST });
    const row = card.getByRole("row").filter({ hasText: CUONG_NAME });
    await expect(row.getByText(/ · hết lượt$/)).toBeVisible();
    await expect(row.getByText("2/2", { exact: true })).toBeVisible();

    await row.getByRole("button", { name: `Cấp thêm lượt cho ${CUONG_NAME}` }).click();
    const dialog = page.getByRole("dialog", { name: `Cấp thêm lượt cho ${CUONG_NAME}` });
    await expect(dialog.getByText(`${GO_TEST} · đã dùng 2/2 lượt.`)).toBeVisible();
    // Số lượt mặc định là 1; lý do bắt buộc.
    await dialog.getByRole("textbox", { name: /^Lý do/ }).fill(GRANT_REASON);
    await dialog.getByRole("button", { name: "Cấp thêm lượt", exact: true }).click();

    await expect(page.getByText(`Đã cấp thêm 1 lượt cho ${CUONG_NAME}.`)).toBeVisible();
    await expect(dialog).toBeHidden();
    await expect(row.getByText("2/3", { exact: true })).toBeVisible();
    await expect(row.getByText(/ · hết lượt$/)).toHaveCount(0);
    await expect(card.getByText(`+1 lượt cho ${CUONG_NAME}`, { exact: true })).toBeVisible();
    await expect(card.getByText(GRANT_REASON, { exact: true })).toBeVisible();
    expect(
      await query<{ extra_attempts: number; reason: string }>(
        `SELECT g.extra_attempts, g.reason FROM quiz_attempt_grants g
           JOIN class_members m ON m.id = g.class_member_id JOIN users u ON u.id = m.user_id
          WHERE u.email = $1 AND g.lesson_id = $2`,
        [CUONG, goTestId],
      ),
    ).toEqual([{ extra_attempts: 1, reason: GRANT_REASON }]);
    await close();

    // Cường không có storageState riêng: đăng nhập qua API rồi mở trình duyệt với phiên đó.
    const api = await loginContext(CUONG, seedPassword());
    const context = await browser.newContext({ storageState: await api.storageState() });
    await api.dispose();
    const student = await context.newPage();
    await student.goto(lessonPath(goTestId));
    await expect(student.getByRole("heading", { level: 1, name: GO_TEST })).toBeVisible();
    await expect(student.getByText("2/3 đã dùng · đã được cấp thêm · tính lượt tốt nhất")).toBeVisible();
    await expect(student.getByText(/^Bạn đã dùng hết/)).toHaveCount(0);
    const start = student.getByRole("button", { name: "Làm lượt mới" });
    await expect(start).toBeVisible();
    await expect(start).not.toHaveAttribute("aria-disabled", "true");
    await context.close();
  });
});
