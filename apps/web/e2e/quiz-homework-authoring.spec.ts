/*
 * Soạn quiz và homework (FR-60, FR-70, FR-74) trên stack thật: admin tạo quiz bài tập và homework trong bản nháp
 * chặng, soạn câu hỏi, tiêu chí chấm, đề bài, tải file kèm pdf qua storage; lý do chặn phát hành hiện đúng rồi
 * hết sau khi sửa; giảng viên mở đường dẫn trình soạn bị đưa về /teach.
 * Tạo chặng E2EQH qua API rồi phát hành qua giao diện, nên chạy @serial và seed lại ở afterAll.
 */
import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test";
import { apiAs, storageStatePath } from "./support/auth";
import { clearMultiCorrectAnswers, closeDb, ids, one, query } from "./support/db";
import { createUnit, seedReset } from "./support/seed";

const STAGE = "Chặng soạn quiz và bài tập";
const QUIZ = "Kiểm tra SQL cơ bản";
const HOMEWORK = "Thiết kế lược đồ cửa hàng";
const QUIZ_KIND = "Quiz bài tập (giới hạn lượt và giờ, giảng viên xác nhận)";
const HOMEWORK_KIND = "Bài tập nộp repo hoặc file (giảng viên chấm)";
/** PDF tối giản; server nhận document theo đuôi tên file và ký content type chuẩn của nó. */
const PDF = Buffer.from("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n");
const EDITOR_URL = /\/lessons\/([0-9a-f-]{36})\/edit$/;

const editorPath = (sid: string, vid: string, lessonId: string) =>
  `/admin/stages/${sid}/versions/${vid}/lessons/${lessonId}/edit`;

function publishButton(page: Page): Locator {
  return page.getByRole("button", { name: "Phát hành v1" });
}

/** Khung lý do chặn phát hành ở đầu trang chặng. */
function blockerTitle(page: Page): Locator {
  return page.getByText("Chưa phát hành được v1:");
}

/** Lấy id học liệu từ URL trang soạn mà form "Thêm học liệu" chuyển tới. */
async function lessonIdFromUrl(page: Page): Promise<string> {
  await expect(page).toHaveURL(EDITOR_URL);
  const match = EDITOR_URL.exec(new URL(page.url()).pathname);
  if (!match) throw new Error(`không phải URL trang soạn: ${page.url()}`);
  return match[1];
}

/** Điền nội dung và phương án của dialog câu hỏi; `correct` là số thứ tự (từ 1) các phương án đúng. */
async function fillQuestion(dialog: Locator, body: string, options: string[], correct: number[]) {
  await dialog.getByLabel(/^Nội dung câu hỏi/).fill(body);
  for (const [i, option] of options.entries()) {
    await dialog.getByRole("textbox", { name: `Phương án ${String(i + 1)}`, exact: true }).fill(option);
  }
  for (const n of correct) {
    await dialog.getByRole("checkbox", { name: `Phương án ${String(n)} là đáp án đúng` }).check();
  }
}

test.describe("Soạn quiz và homework", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });
  test.use({ storageState: storageStatePath("admin") });

  let admin: APIRequestContext;
  let stageId: string;
  let draftId: string;
  let quizId: string;
  let homeworkId: string;

  test.beforeAll(async () => {
    await seedReset();
    admin = await apiAs("admin");
    const res = await admin.post("/api/v1/stages", { data: { code: "E2EQH", name: STAGE } });
    expect(res.status(), await res.text()).toBe(201);
    stageId = await ids.stageId("E2EQH");
    draftId = await ids.stageVersionId("E2EQH", 1);
    await createUnit(admin, draftId, "Ôn tập SQL");
  });

  test.afterAll(async () => {
    await admin.dispose();
    await seedReset();
    await closeDb();
  });

  const stagePath = () => `/admin/stages/${stageId}?v=${draftId}`;

  /** Mở dialog "Thêm học liệu" của bài học duy nhất, chọn loại và điền tiêu đề. */
  async function openAddLesson(page: Page, title: string, kind: string): Promise<Locator> {
    await page.goto(stagePath());
    await page.getByRole("button", { name: "Thêm học liệu" }).click();
    const dialog = page.getByRole("dialog", { name: "Thêm học liệu" });
    await dialog.getByLabel(/^Tiêu đề/).fill(title);
    await dialog.getByRole("combobox", { name: "Loại" }).click();
    await page.getByRole("option", { name: kind, exact: true }).click();
    return dialog;
  }

  test("admin tạo quiz bài tập và homework; học liệu thiếu nội dung chặn phát hành", async ({ page }) => {
    const quizDialog = await openAddLesson(page, QUIZ, QUIZ_KIND);
    // Quiz bài tập có giá trị mặc định cho ngưỡng, số lượt và thời gian mỗi lượt.
    await expect(quizDialog.getByLabel(/^Ngưỡng đạt \(%\)/)).toHaveValue("70");
    await expect(quizDialog.getByLabel(/^Số lượt làm bài/)).toHaveValue("2");
    await expect(quizDialog.getByLabel(/^Thời gian mỗi lượt \(phút\)/)).toHaveValue("20");
    await quizDialog.getByLabel(/^Số lượt làm bài/).fill("3");
    await expect(quizDialog.getByRole("checkbox", { name: "Đảo thứ tự câu hỏi và đáp án mỗi lượt" })).toBeChecked();
    await quizDialog.getByRole("button", { name: "Thêm", exact: true }).click();
    quizId = await lessonIdFromUrl(page);
    await expect(page.getByRole("heading", { level: 1, name: QUIZ })).toBeVisible();
    await expect(page.getByRole("button", { name: "Thêm câu hỏi đầu tiên" })).toBeVisible();

    const settings = await one<{
      mode: string;
      pass_percent: number;
      max_attempts: number;
      time_limit_seconds: number;
      shuffle: boolean;
    }>("SELECT mode, pass_percent, max_attempts, time_limit_seconds, shuffle FROM quiz_settings WHERE lesson_id = $1", [
      quizId,
    ]);
    expect(settings).toEqual({
      mode: "assessment",
      pass_percent: 70,
      max_attempts: 3,
      time_limit_seconds: 1200,
      shuffle: true,
    });

    const homeworkDialog = await openAddLesson(page, HOMEWORK, HOMEWORK_KIND);
    await expect(homeworkDialog.getByLabel(/^Ngưỡng đạt \(%\)/)).toHaveValue("70");
    await homeworkDialog.getByRole("button", { name: "Thêm", exact: true }).click();
    homeworkId = await lessonIdFromUrl(page);
    await expect(page.getByRole("heading", { level: 1, name: HOMEWORK })).toBeVisible();

    // Trang chặng: mỗi học liệu thiếu nội dung có badge, nút phát hành khóa và nêu lý do theo thứ tự học liệu.
    await page.goto(stagePath());
    const reasons = [
      `"${QUIZ}" chưa có câu hỏi.`,
      `"${HOMEWORK}" chưa có đề bài.`,
      `"${HOMEWORK}" chưa có tiêu chí chấm.`,
    ];
    await expect(blockerTitle(page)).toBeVisible();
    for (const reason of reasons) await expect(page.getByText(reason, { exact: true })).toBeVisible();
    await expect(publishButton(page)).toHaveAttribute("aria-disabled", "true");
    await expect(publishButton(page)).toHaveAttribute("title", reasons[0]);
    await expect(page.getByText("Chưa đủ để phát hành")).toHaveCount(2);
    await expect(page.getByRole("link", { name: `Soạn câu hỏi: ${QUIZ}` })).toHaveAttribute(
      "href",
      editorPath(stageId, draftId, quizId),
    );
    await expect(page.getByRole("link", { name: `Soạn tiêu chí: ${HOMEWORK}` })).toHaveAttribute(
      "href",
      editorPath(stageId, draftId, homeworkId),
    );
  });

  test("admin soạn hai câu hỏi: một đáp án và nhiều đáp án", async ({ page }) => {
    await page.goto(editorPath(stageId, draftId, quizId));
    await page.getByRole("button", { name: "Thêm câu hỏi đầu tiên" }).click();
    let dialog = page.getByRole("dialog", { name: "Thêm câu hỏi" });
    await fillQuestion(dialog, "Lệnh nào **đọc** dữ liệu từ bảng?", ["SELECT", "DELETE"], []);
    // Chưa đánh dấu đáp án đúng: form không gửi, lỗi gắn với nhóm phương án.
    await dialog.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
    await expect(dialog.getByText("Đánh dấu ít nhất một đáp án đúng.")).toBeVisible();
    await expect(dialog.getByRole("group", { name: /^Phương án/ })).toHaveAccessibleDescription(
      "Đánh dấu ít nhất một đáp án đúng.",
    );
    await dialog.getByRole("checkbox", { name: "Phương án 1 là đáp án đúng" }).check();
    await dialog.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByText("Đã thêm câu hỏi.")).toBeVisible();

    await page.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
    dialog = page.getByRole("dialog", { name: "Thêm câu hỏi" });
    await fillQuestion(
      dialog,
      "Những lệnh nào thay đổi **cấu trúc** bảng? Chọn tất cả đáp án đúng.",
      ["CREATE TABLE", "ALTER TABLE", "SELECT"],
      [1, 2],
    );
    await dialog.getByRole("combobox", { name: "Loại câu" }).click();
    await page.getByRole("option", { name: "Nhiều đáp án", exact: true }).click();
    await dialog.getByRole("spinbutton", { name: /^Điểm/ }).fill("2");
    await dialog.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByRole("heading", { name: "Câu hỏi 2 câu · 3 điểm" })).toBeVisible();

    const questions = await query<{ type: string; points: number; correct: string[] }>(
      `SELECT q.type, q.points, array_agg(o.body ORDER BY o.position) FILTER (WHERE o.is_correct) AS correct
         FROM questions q JOIN question_options o ON o.question_id = q.id
        WHERE q.lesson_id = $1 GROUP BY q.id ORDER BY q.position`,
      [quizId],
    );
    expect(questions).toEqual([
      { type: "single", points: 1, correct: ["SELECT"] },
      { type: "multi", points: 2, correct: ["CREATE TABLE", "ALTER TABLE"] },
    ]);
  });

  test("admin soạn đề bài, hai tiêu chí chấm và đính kèm file pdf", async ({ page }) => {
    await page.goto(editorPath(stageId, draftId, homeworkId));
    await expect(page.getByText("Hạn nộp đặt riêng cho từng lớp trong trang lớp, không đặt ở đây.")).toBeVisible();

    const brief = page.getByRole("textbox", { name: "Đề bài (Markdown)" });
    await brief.click();
    await page.keyboard.type("Thiết kế lược đồ cho cửa hàng bán sách, nộp file SQL tạo bảng.");
    await page.getByRole("button", { name: "Lưu đề bài" }).click();
    await expect(page.getByText("Đã lưu đề bài.")).toBeVisible();

    await page.getByRole("button", { name: "Thêm tiêu chí đầu tiên" }).click();
    let dialog = page.getByRole("dialog", { name: "Thêm tiêu chí chấm" });
    await dialog.getByLabel(/^Tiêu chí/).fill("Lược đồ đạt chuẩn 3NF");
    await dialog.getByLabel(/^Điểm tối đa/).fill("60");
    await dialog.getByRole("button", { name: "Thêm tiêu chí", exact: true }).click();
    await expect(dialog).toBeHidden();
    await page.getByRole("button", { name: "Thêm tiêu chí", exact: true }).click();
    dialog = page.getByRole("dialog", { name: "Thêm tiêu chí chấm" });
    await dialog.getByLabel(/^Tiêu chí/).fill("Khóa ngoại và ràng buộc đúng");
    await dialog.getByLabel(/^Điểm tối đa/).fill("40");
    await dialog.getByRole("button", { name: "Thêm tiêu chí", exact: true }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByRole("heading", { name: "Tiêu chí chấm 2 tiêu chí · 100 điểm" })).toBeVisible();
    await expect(page.getByText("70% tổng điểm (từ 70/100)")).toBeVisible();

    // File kèm đi qua upload ticket và storage thật như video, ảnh.
    await page
      .getByLabel("Chọn file đính kèm")
      .setInputFiles({ name: "de-bai.pdf", mimeType: "application/pdf", buffer: PDF });
    await expect(page.getByText("Đã đính kèm de-bai.pdf.")).toBeVisible({ timeout: 30_000 });
    const chip = page.getByRole("link", { name: /de-bai\.pdf/ });
    await expect(chip).toHaveAttribute("href", /^\/api\/v1\/media\/[0-9a-f-]{36}\/content$/);

    const settings = await one<{ brief_source: string; pass_percent: number }>(
      "SELECT brief_source, pass_percent FROM homework_settings WHERE lesson_id = $1",
      [homeworkId],
    );
    expect(settings.brief_source).toContain("Thiết kế lược đồ cho cửa hàng bán sách");
    expect(settings.pass_percent).toBe(70);
    const criteria = await query<{ description: string; max_points: number }>(
      "SELECT description, max_points FROM rubric_criteria WHERE lesson_id = $1 ORDER BY position",
      [homeworkId],
    );
    expect(criteria).toEqual([
      { description: "Lược đồ đạt chuẩn 3NF", max_points: 60 },
      { description: "Khóa ngoại và ràng buộc đúng", max_points: 40 },
    ]);
    const attachments = await query<{ original_name: string; kind: string; content_type: string; status: string }>(
      `SELECT m.original_name, m.kind, m.content_type, m.status FROM homework_attachments a
         JOIN media_files m ON m.id = a.media_id WHERE a.lesson_id = $1 ORDER BY a.position`,
      [homeworkId],
    );
    expect(attachments).toEqual([
      { original_name: "de-bai.pdf", kind: "document", content_type: "application/pdf", status: "ready" },
    ]);
  });

  test("câu thiếu đáp án đúng chặn phát hành; sửa câu đó rồi phát hành được", async ({ page }) => {
    // API không cho lưu câu không có đáp án đúng, nên dựng trạng thái đó thẳng trong DB để kiểm lớp chặn phát hành.
    expect(await clearMultiCorrectAnswers(quizId)).toBeGreaterThan(0);
    const unanswered = `"${QUIZ}" có 1 câu chưa đánh dấu đáp án đúng.`;
    await page.goto(stagePath());
    await expect(blockerTitle(page)).toBeVisible();
    await expect(page.getByText(unanswered, { exact: true })).toBeVisible();
    await expect(page.getByText(/chưa có (câu hỏi|đề bài|tiêu chí chấm)\.$/)).toHaveCount(0);
    await expect(publishButton(page)).toHaveAttribute("aria-disabled", "true");
    await expect(publishButton(page)).toHaveAttribute("title", unanswered);
    await expect(page.getByText("Chưa đủ để phát hành")).toHaveCount(1);

    await page.getByRole("link", { name: `Soạn câu hỏi: ${QUIZ}` }).click();
    await expect(page).toHaveURL(new RegExp(`/lessons/${quizId}/edit$`));
    await expect(page.getByText(unanswered)).toBeVisible();
    await page.getByRole("button", { name: "Sửa câu 2" }).click();
    const dialog = page.getByRole("dialog", { name: "Sửa câu hỏi" });
    await expect(dialog.getByRole("textbox", { name: "Phương án 2", exact: true })).toHaveValue("ALTER TABLE");
    await dialog.getByRole("checkbox", { name: "Phương án 1 là đáp án đúng" }).check();
    await dialog.getByRole("checkbox", { name: "Phương án 2 là đáp án đúng" }).check();
    await dialog.getByRole("button", { name: "Lưu", exact: true }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByText("Đã lưu câu hỏi.")).toBeVisible();
    await expect(page.getByText(unanswered)).toHaveCount(0);

    await page.goto(stagePath());
    await expect(page.getByText(/^Chưa phát hành được/)).toHaveCount(0);
    await expect(page.getByText("Chưa đủ để phát hành")).toHaveCount(0);
    await expect(publishButton(page)).not.toHaveAttribute("aria-disabled");
    await publishButton(page).click();
    const confirm = page.getByRole("dialog", { name: `Phát hành ${STAGE} v1?` });
    await confirm.getByRole("button", { name: "Phát hành", exact: true }).click();
    await expect(confirm).toBeHidden();
    expect((await one<{ status: string }>("SELECT status FROM stage_versions WHERE id = $1", [draftId])).status).toBe(
      "published",
    );

    // Bản đã phát hành: trang soạn chỉ đọc, không còn nút sửa câu hỏi.
    await page.getByRole("link", { name: `Xem chi tiết: ${QUIZ}` }).click();
    await expect(page.getByRole("heading", { name: "Câu hỏi 2 câu · 3 điểm" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Thêm câu hỏi", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Sửa câu 1" })).toHaveCount(0);
  });

  test("giảng viên mở đường dẫn trình soạn bị đưa về /teach", async ({ browser }) => {
    const context = await browser.newContext({ storageState: storageStatePath("teacher") });
    const page = await context.newPage();
    await page.goto(editorPath(stageId, draftId, quizId));
    await expect(page).toHaveURL(/\/teach$/);
    await expect(page.getByRole("navigation", { name: "Chính" })).toContainText("Lớp của tôi");
    await expect(page.getByRole("heading", { level: 1, name: QUIZ })).toHaveCount(0);
    await context.close();
  });
});
