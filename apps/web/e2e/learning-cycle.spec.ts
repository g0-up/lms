/*
 * Vòng học khép kín trên stack thật (plan Sprint 2 §6):
 * - admin tạo chặng E2ELC qua giao diện: một bài học có quiz luyện tập, quiz bài tập và homework kèm tiêu chí, rồi
 *   phát hành v1;
 * - admin gắn chặng vào khóa học mới, mở lớp do giảng viên Bảo dạy và mời An (qua API, như các spec lớp khác);
 * - Bảo đặt hạn nộp homework ở tab "Bài tập" của lớp;
 * - An làm quiz luyện tập (thấy đáp án), quiz bài tập (chỉ thấy điểm, chờ xác nhận) rồi nộp homework;
 * - Bảo mở "Chấm bài", chấm homework Đạt, được chuyển thẳng sang lượt quiz (nextQueueItem), xác nhận kèm nhận xét rồi
 *   về hàng chờ trống; Mailpit có thư "Bài đã được chấm" và "Giảng viên đã xác nhận kết quả";
 * - An thấy "Hoàn thành chặng" trong "Tổng kết khóa học", ô của An ở tab "Tổng kết" là "Hoàn thành";
 * - Bảo viết nhận xét chặng, An thấy nhận xét đó.
 * nextQueueItem trả mục cũ nhất trong toàn hàng chờ của giảng viên, nên beforeAll xử lý hết hàng chờ seed của Bảo qua
 * API: sau đó hàng chờ chỉ còn hai mục của kịch bản. Ghi chặng, lớp, bài nộp và nhận xét vào DB seed, nên chạy
 * @serial và seed lại ở beforeAll/afterAll.
 */
import { expect, test, type APIRequestContext, type Browser, type Locator, type Page } from "@playwright/test";
import { apiAs, browserAs, emailOf, type Role } from "./support/auth";
import { closeDb, ids, one, query } from "./support/db";
import { messagesTo, waitForSubject, type MailSummary } from "./support/mailpit";
import { activateClass, createClass, invite, seedReset } from "./support/seed";

const AN = "an.nguyen@gmail.com";
const AN_NAME = "Nguyễn Hoàng An";
const TEACHER_NAME = "Phạm Quốc Bảo";
const STAGE_CODE = "E2ELC";
const STAGE = "Chặng vòng học E2E";
const UNIT = "Biến và lệnh go";
const PRACTICE = "Ôn nhanh hằng trong Go";
const ASSESSMENT = "Kiểm tra lệnh go và kiểu số";
const HOMEWORK = "Bài tập: Viết CLI đếm từ";
const HOMEWORK_SHORT = "Viết CLI đếm từ";
const CRITERIA = [
  { description: "Chương trình đếm đúng số từ", maxPoints: 60 },
  { description: "Code rõ ràng, có README", maxPoints: 40 },
] as const;
const REPO = "https://github.com/hoangan-dev/go-wordcount";
const GRADE_COMMENT = "Đếm đúng cả dòng trống, README đủ hướng dẫn chạy.";
const REVIEW_COMMENT = "Hai lượt trả lời đều chắc, nắm được lệnh go cơ bản.";
const STAGE_REVIEW = "An hoàn thành chặng đúng hạn, code gọn. Sang chặng sau ôn thêm xử lý lỗi.";
const APP_PREFIX = "[GoUp LMS] ";
const GRADED_SUBJECT = `${APP_PREFIX}Bài đã được chấm: ${HOMEWORK} · Đạt`;
const REVIEWED_SUBJECT = `${APP_PREFIX}Giảng viên đã xác nhận kết quả: ${ASSESSMENT} lượt #1 · 100% · Đạt`;
const PRACTICE_KIND = "Quiz học liệu (luyện tập, chấm tự động)";
const ASSESSMENT_KIND = "Quiz bài tập (giới hạn lượt và giờ, giảng viên xác nhận)";
const HOMEWORK_KIND = "Bài tập nộp repo hoặc file (giảng viên chấm)";
const EDITOR_URL = /\/lessons\/([0-9a-f-]{36})\/edit$/;
const DAY_MS = 86_400_000;
const VN_OFFSET_MS = 7 * 3_600_000;

interface QueueItem {
  kind: "homework" | "quiz";
  itemId: string;
  lesson: { id: string };
}

let admin: APIRequestContext;
let teacherApi: APIRequestContext;
let stageId: string;
let stageVersionId: string;
let practiceId: string;
let assessmentId: string;
let homeworkId: string;
let classId: string;
let attemptId: string;
/** Thư tới An trước khi chấm; mỗi thư kiểm xong được thêm vào. */
let mailSeen: MailSummary[];

const lessonPath = (lessonId: string) => `/learn/classes/${classId}/lessons/${lessonId}`;
const editorPath = (lessonId: string) => `/admin/stages/${stageId}/versions/${stageVersionId}/lessons/${lessonId}/edit`;

/** Trang mới trong context mang phiên của vai trò; đóng context khi xong. */
async function openAs(browser: Browser, role: Role, path: string): Promise<{ page: Page; close: () => Promise<void> }> {
  const context = await browserAs(browser, role);
  const page = await context.newPage();
  await page.goto(path);
  return { page, close: () => context.close() };
}

async function getJson<T>(ctx: APIRequestContext, url: string): Promise<T> {
  const res = await ctx.get(url);
  expect(res.status(), await res.text()).toBe(200);
  return (await res.json()) as T;
}

async function gradingQueue(): Promise<QueueItem[]> {
  return (await getJson<{ items: QueueItem[] }>(teacherApi, "/api/v1/grading/queue")).items;
}

/** Xử lý hết hàng chờ của Bảo qua API: chấm đủ điểm mọi bài nộp, xác nhận mọi lượt quiz bài tập. */
async function drainQueue(): Promise<void> {
  for (const item of await gradingQueue()) {
    let res;
    if (item.kind === "homework") {
      const detail = await getJson<{ lesson: { criteria: { id: string; maxPoints: number }[] } }>(
        teacherApi,
        `/api/v1/submissions/${item.itemId}`,
      );
      res = await teacherApi.post(`/api/v1/submissions/${item.itemId}/grades`, {
        data: {
          scores: detail.lesson.criteria.map((c) => ({ criterionId: c.id, points: c.maxPoints, comment: "" })),
          comment: "Đạt yêu cầu.",
        },
      });
    } else {
      res = await teacherApi.post(`/api/v1/quiz-attempts/${item.itemId}/reviews`, {
        data: { comment: "Đã xem kết quả.", notes: [] },
      });
    }
    expect(res.status(), await res.text()).toBeLessThan(300);
  }
  expect(await gradingQueue()).toEqual([]);
}

/** Ngày `daysAhead` ngày tới theo giờ Việt Nam, 23:59, dạng giá trị của `datetime-local`. */
function vnLocalInput(daysAhead: number): string {
  const vn = new Date(Date.now() + daysAhead * DAY_MS + VN_OFFSET_MS);
  return `${vn.toISOString().slice(0, 10)}T23:59`;
}

/** Lấy id học liệu từ URL trang soạn mà form "Thêm học liệu" chuyển tới. */
async function lessonIdFromUrl(page: Page): Promise<string> {
  await expect(page).toHaveURL(EDITOR_URL);
  const match = EDITOR_URL.exec(new URL(page.url()).pathname);
  if (!match) throw new Error(`không phải URL trang soạn: ${page.url()}`);
  return match[1];
}

/** Thêm học liệu vào bài học duy nhất của chặng; trả id học liệu. */
async function addLesson(page: Page, title: string, kind: string): Promise<string> {
  await page.goto(`/admin/stages/${stageId}?v=${stageVersionId}`);
  await page.getByRole("button", { name: "Thêm học liệu" }).click();
  const dialog = page.getByRole("dialog", { name: "Thêm học liệu" });
  await dialog.getByLabel(/^Tiêu đề/).fill(title);
  await dialog.getByRole("combobox", { name: "Loại" }).click();
  await page.getByRole("option", { name: kind, exact: true }).click();
  await dialog.getByRole("button", { name: "Thêm", exact: true }).click();
  const id = await lessonIdFromUrl(page);
  await expect(page.getByRole("heading", { level: 1, name: title })).toBeVisible();
  return id;
}

/** Thêm một câu một đáp án trên trang soạn quiz; `correct` là số thứ tự (từ 1) phương án đúng. */
async function addQuestion(page: Page, opener: string, body: string, options: string[], correct: number) {
  await page.getByRole("button", { name: opener, exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Thêm câu hỏi" });
  await dialog.getByLabel(/^Nội dung câu hỏi/).fill(body);
  for (const [i, option] of options.entries()) {
    await dialog.getByRole("textbox", { name: `Phương án ${String(i + 1)}`, exact: true }).fill(option);
  }
  await dialog.getByRole("checkbox", { name: `Phương án ${String(correct)} là đáp án đúng` }).check();
  await dialog.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText("Đã thêm câu hỏi.").first()).toBeVisible();
}

/** Câu đang hiện: fieldset (tên là nội dung câu) chứa các ô chọn đáp án. */
function currentQuestion(page: Page): Locator {
  return page.getByRole("group").filter({ has: page.locator('input[type="radio"], input[type="checkbox"]') });
}

/** Chọn phương án có nội dung khớp `option` ở câu đang hiện và chờ server lưu. */
async function answer(page: Page, option: RegExp): Promise<void> {
  const saved = page.waitForResponse((res) => res.request().method() === "PUT" && /\/answers\/[^/]+$/.test(res.url()));
  await currentQuestion(page).getByText(option).click();
  expect((await saved).ok()).toBe(true);
}

/** Bấm "Nộp bài" ở thanh trạng thái rồi xác nhận trong hộp "Nộp bài?"; trả id lượt từ `?result=`. */
async function submitAttempt(page: Page): Promise<string> {
  await page.getByRole("button", { name: "Nộp bài", exact: true }).first().click();
  const dialog = page.getByRole("dialog", { name: "Nộp bài?" });
  await dialog.getByRole("button", { name: "Nộp bài", exact: true }).click();
  await expect(page.getByText("Đã nộp bài.")).toBeVisible();
  await expect(page).toHaveURL(/\?result=[0-9a-f-]{36}$/);
  return new URL(page.url()).searchParams.get("result") ?? "";
}

/** Ô của An ở chặng của kịch bản trong ma trận "Tổng kết". */
function anCell(page: Page): Locator {
  return page.getByRole("button", { name: new RegExp(`^${AN_NAME}, ${STAGE}: `) });
}

test.describe("Vòng học khép kín", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });

  test.beforeAll(async () => {
    await seedReset();
    admin = await apiAs("admin");
    teacherApi = await apiAs("teacher2");
    await drainQueue();
  });

  test.afterAll(async () => {
    await admin.dispose();
    await teacherApi.dispose();
    await closeDb();
    await seedReset();
  });

  test("admin tạo chặng có quiz luyện tập, quiz bài tập, homework kèm tiêu chí rồi phát hành", async ({ browser }) => {
    const { page, close } = await openAs(browser, "admin", "/admin/stages");
    await page.getByRole("button", { name: "Tạo chặng" }).click();
    const create = page.getByRole("dialog", { name: "Tạo chặng" });
    await create.getByLabel("Mã chặng").fill(STAGE_CODE);
    await create.getByLabel("Tên chặng").fill(STAGE);
    await create.getByRole("button", { name: "Tạo chặng" }).click();
    await expect(page.getByText("Đã tạo chặng với bản nháp v1.")).toBeVisible();
    await expect(page.getByRole("heading", { level: 1, name: STAGE })).toBeVisible();
    stageId = await ids.stageId(STAGE_CODE);
    stageVersionId = await ids.stageVersionId(STAGE_CODE, 1);

    await page.getByRole("button", { name: "Thêm bài học", exact: true }).click();
    const unit = page.getByRole("dialog", { name: "Thêm bài học" });
    await unit.getByLabel(/^Tên bài học/).fill(UNIT);
    await unit.getByRole("button", { name: "Thêm", exact: true }).click();
    await expect(unit).toBeHidden();

    practiceId = await addLesson(page, PRACTICE, PRACTICE_KIND);
    await addQuestion(page, "Thêm câu hỏi đầu tiên", "Từ khóa nào khai báo **hằng** trong Go?", ["const", "var"], 1);

    assessmentId = await addLesson(page, ASSESSMENT, ASSESSMENT_KIND);
    await addQuestion(page, "Thêm câu hỏi đầu tiên", "Lệnh nào biên dịch rồi chạy chương trình Go?", ["go run", "go fmt"], 1);
    await addQuestion(page, "Thêm câu hỏi", "Kiểu nào là số nguyên?", ["int", "string"], 1);

    homeworkId = await addLesson(page, HOMEWORK, HOMEWORK_KIND);
    const brief = page.getByRole("textbox", { name: "Đề bài (Markdown)" });
    await brief.click();
    await page.keyboard.type("Viết CLI đọc file văn bản và in số từ. Nộp repo có README hướng dẫn chạy.");
    await page.getByRole("button", { name: "Lưu đề bài" }).click();
    await expect(page.getByText("Đã lưu đề bài.")).toBeVisible();
    for (const [i, criterion] of CRITERIA.entries()) {
      await page.getByRole("button", { name: i === 0 ? "Thêm tiêu chí đầu tiên" : "Thêm tiêu chí", exact: true }).click();
      const dialog = page.getByRole("dialog", { name: "Thêm tiêu chí chấm" });
      await dialog.getByLabel(/^Tiêu chí/).fill(criterion.description);
      await dialog.getByLabel(/^Điểm tối đa/).fill(String(criterion.maxPoints));
      await dialog.getByRole("button", { name: "Thêm tiêu chí", exact: true }).click();
      await expect(dialog).toBeHidden();
    }
    await expect(page.getByRole("heading", { name: "Tiêu chí chấm 2 tiêu chí · 100 điểm" })).toBeVisible();

    await page.goto(`/admin/stages/${stageId}?v=${stageVersionId}`);
    const publish = page.getByRole("button", { name: "Phát hành v1" });
    await expect(publish).not.toHaveAttribute("aria-disabled");
    await publish.click();
    const confirm = page.getByRole("dialog", { name: `Phát hành ${STAGE} v1?` });
    await confirm.getByRole("button", { name: "Phát hành", exact: true }).click();
    await expect(confirm).toBeHidden();
    await close();

    expect((await one<{ status: string }>("SELECT status FROM stage_versions WHERE id = $1", [stageVersionId])).status).toBe(
      "published",
    );
    // Cả ba học liệu bắt buộc (mặc định của form), theo đúng thứ tự thêm.
    expect(
      await query<{ id: string; type: string; required: boolean; mode: string | null }>(
        `SELECT l.id, l.type, l.required, q.mode FROM lessons l LEFT JOIN quiz_settings q ON q.lesson_id = l.id
          WHERE l.stage_version_id = $1 ORDER BY l.position`,
        [stageVersionId],
      ),
    ).toEqual([
      { id: practiceId, type: "quiz", required: true, mode: "practice" },
      { id: assessmentId, type: "quiz", required: true, mode: "assessment" },
      { id: homeworkId, type: "homework", required: true, mode: null },
    ]);
    // Trang soạn bản đã phát hành chỉ đọc.
    const view = await openAs(browser, "admin", editorPath(assessmentId));
    await expect(view.page.getByRole("heading", { name: "Câu hỏi 2 câu · 2 điểm" })).toBeVisible();
    await expect(view.page.getByRole("button", { name: "Thêm câu hỏi", exact: true })).toHaveCount(0);
    await view.close();
  });

  test("admin gắn chặng vào khóa học mới, mở lớp do Bảo dạy và mời An", async () => {
    const course = await admin.post("/api/v1/courses", { data: { code: "E2ELCC", name: "Khóa học vòng học E2E" } });
    expect(course.status(), await course.text()).toBe(201);
    const courseVersionId = await ids.courseVersionId("E2ELCC", 1);
    const stages = await admin.put(`/api/v1/course-versions/${courseVersionId}/stages`, {
      data: { stageVersionIds: [stageVersionId] },
    });
    expect(stages.status(), await stages.text()).toBe(200);
    const published = await admin.post(`/api/v1/course-versions/${courseVersionId}/publish`);
    expect(published.status(), await published.text()).toBe(200);

    const cls = await createClass(admin, "e2elc", courseVersionId, emailOf("teacher2"));
    classId = cls.id;
    await invite(admin, classId, AN);
    await activateClass(admin, classId);
  });

  test("giảng viên đặt hạn nộp homework ở tab Bài tập của lớp", async ({ browser }) => {
    const { page, close } = await openAs(browser, "teacher2", `/teach/classes/${classId}?tab=homework`);
    const card = page.getByRole("region", { name: HOMEWORK });
    await expect(card.getByText("0/1 đã nộp")).toBeVisible();
    await card.getByRole("button", { name: "Đặt hạn nộp" }).click();
    const dialog = page.getByRole("dialog", { name: `Hạn nộp: ${HOMEWORK_SHORT}` });
    const due = vnLocalInput(7);
    await dialog.getByLabel(/^Hạn nộp/).fill(due);
    await dialog.getByRole("button", { name: "Lưu hạn nộp" }).click();
    await expect(page.getByText("Đã lưu hạn nộp.")).toBeVisible();
    await expect(dialog).toBeHidden();
    const [y, m, d] = due.slice(0, 10).split("-");
    await expect(card.getByText(new RegExp(`^Hạn ${d}/${m}/${y} 23:59 · `))).toBeVisible();
    await expect(card.getByRole("button", { name: "Đổi hạn nộp" })).toBeVisible();
    await close();
  });

  test("học viên làm quiz luyện tập, quiz bài tập và nộp homework", async ({ browser }) => {
    const { page, close } = await openAs(browser, "student", lessonPath(practiceId));
    await expect(page.getByRole("heading", { level: 1, name: PRACTICE })).toBeVisible();
    await page.getByRole("button", { name: "Bắt đầu làm bài" }).click();
    await answer(page, /^const$/);
    await submitAttempt(page);
    // Quiz luyện tập lộ đáp án ngay sau khi nộp.
    await expect(page.getByText(/\(đáp án đúng\)/)).toHaveCount(1);
    await expect(page.getByText(/\(bạn chọn\)/)).toHaveCount(1);

    await page.goto(lessonPath(assessmentId));
    await expect(page.getByRole("heading", { level: 1, name: ASSESSMENT })).toBeVisible();
    await page.getByRole("button", { name: "Bắt đầu làm bài" }).click();
    // Thứ tự câu và phương án bị đảo: mỗi câu chọn phương án đúng của chính nó.
    await answer(page, /^(go run|int)$/);
    await page.getByRole("button", { name: "Câu tiếp" }).click();
    await answer(page, /^(go run|int)$/);
    await expect(page.getByText("2/2 đã trả lời")).toBeVisible();
    attemptId = await submitAttempt(page);
    // Quiz bài tập chỉ hiện điểm, chờ giảng viên xác nhận, không lộ đáp án.
    await expect(page.getByText(/^Chờ giảng viên xác nhận\./)).toBeVisible();
    await expect(page.getByText(/\(đáp án đúng\)/)).toHaveCount(0);

    await page.goto(lessonPath(homeworkId));
    await expect(page.getByRole("heading", { level: 1, name: HOMEWORK })).toBeVisible();
    await page.getByLabel(/^Đường dẫn repo/).fill(REPO);
    await page.getByRole("button", { name: "Nộp bài" }).click();
    await expect(page.getByText("Đã nộp lần #1. Bạn sẽ nhận email khi có kết quả.")).toBeVisible({ timeout: 30_000 });
    await close();

    expect(
      await query<{ review_status: string | null; passed: boolean }>(
        "SELECT review_status, passed FROM quiz_attempts WHERE id = $1",
        [attemptId],
      ),
    ).toEqual([{ review_status: "pending", passed: true }]);
    // Hàng chờ của Bảo chỉ còn hai mục của kịch bản, cũ nhất trước: lượt quiz rồi bài nộp.
    const queue = await gradingQueue();
    expect(queue.map((item) => [item.kind, item.lesson.id])).toEqual([
      ["quiz", assessmentId],
      ["homework", homeworkId],
    ]);
    expect(queue[0].itemId).toBe(attemptId);
    mailSeen = await messagesTo(AN);
  });

  test("giảng viên chấm homework Đạt, được chuyển sang xác nhận quiz rồi về hàng chờ trống", async ({ browser }) => {
    const { page, close } = await openAs(browser, "teacher2", "/teach/grading");
    await expect(page.getByRole("heading", { level: 1, name: "Chấm bài" })).toBeVisible();
    await page.getByRole("link", { name: `Chấm bài của ${AN_NAME}: ${HOMEWORK_SHORT}`, exact: true }).click();
    await expect(page.getByRole("heading", { level: 1, name: `${AN_NAME} · ${HOMEWORK_SHORT}` })).toBeVisible();
    for (const criterion of CRITERIA) {
      await page.getByRole("spinbutton", { name: criterion.description }).fill(String(criterion.maxPoints));
    }
    await page.getByRole("textbox", { name: /Nhận xét chung/ }).fill(GRADE_COMMENT);
    await page.getByRole("button", { name: "Lưu kết quả", exact: true }).click();
    await expect(
      page.getByText("Đã lưu 100/100 · Đạt. Email kết quả đã gửi cho học viên. Chuyển sang bài tiếp theo."),
    ).toBeVisible();

    // nextQueueItem: mục còn lại duy nhất là lượt quiz bài tập của An.
    await expect(page).toHaveURL(new RegExp(`/teach/grading/quiz/${attemptId}$`));
    await expect(page.getByRole("heading", { level: 1, name: `${AN_NAME} · ${ASSESSMENT}` })).toBeVisible();
    const form = page.getByRole("form", { name: "Xác nhận kết quả" });
    await form.getByRole("textbox", { name: /^Nhận xét chung/ }).fill(REVIEW_COMMENT);
    await form.getByRole("button", { name: "Xác nhận", exact: true }).click();
    await expect(
      page.getByText("Đã xác nhận kết quả. Email nhận xét đã gửi cho học viên. Đã xử lý hết hàng chờ."),
    ).toBeVisible();
    await expect(page).toHaveURL(/\/teach\/grading$/);
    await close();

    expect(await gradingQueue()).toEqual([]);
    expect(
      await query<{ status: string }>(
        "SELECT status FROM submissions WHERE lesson_id = $1 AND class_member_id = $2",
        [homeworkId, await ids.memberId("e2elc", AN)],
      ),
    ).toEqual([{ status: "passed" }]);
    expect(
      (await one<{ review_status: string }>("SELECT review_status FROM quiz_attempts WHERE id = $1", [attemptId]))
        .review_status,
    ).toBe("confirmed");

    const graded = await waitForSubject(AN, GRADED_SUBJECT, mailSeen);
    const reviewed = await waitForSubject(AN, REVIEWED_SUBJECT, mailSeen);
    mailSeen = [...mailSeen, graded, reviewed];
  });

  test("học viên thấy Hoàn thành chặng; ô của học viên ở tab Tổng kết là Hoàn thành", async ({ browser }) => {
    // Hoàn thành chỉ đọc từ lesson_progress: quiz luyện tập khi nộp, quiz bài tập khi đạt, homework khi chấm Đạt.
    const done = await query<{ lesson_id: string }>(
      `SELECT lesson_id FROM lesson_progress WHERE class_member_id = $1 AND completed_at IS NOT NULL
        ORDER BY lesson_id`,
      [await ids.memberId("e2elc", AN)],
    );
    expect(done.map((r) => r.lesson_id)).toEqual([practiceId, assessmentId, homeworkId].sort());

    const student = await openAs(browser, "student", `/learn/classes/${classId}/summary`);
    await expect(student.page.getByRole("heading", { level: 1, name: "Tổng kết khóa học" })).toBeVisible();
    await expect(student.page.getByText("3/3 học liệu bắt buộc. Học liệu không bắt buộc")).toBeVisible();
    const card = stageCard(student.page);
    await expect(card.getByText("Hoàn thành chặng", { exact: true })).toBeVisible();
    await expect(card.getByText("3/3 học liệu bắt buộc", { exact: true })).toBeVisible();
    await student.close();

    const teacher = await openAs(browser, "teacher2", `/teach/classes/${classId}?tab=summary`);
    await expect(anCell(teacher.page)).toHaveAccessibleName(
      `${AN_NAME}, ${STAGE}: Hoàn thành, chưa nhận xét. Mở tổng kết chặng`,
    );
    await expect(anCell(teacher.page)).toContainText("Hoàn thành");
    await teacher.close();
  });

  test("giảng viên viết nhận xét chặng, học viên thấy nhận xét", async ({ browser }) => {
    const teacher = await openAs(browser, "teacher2", `/teach/classes/${classId}?tab=summary`);
    await anCell(teacher.page).click();
    const drawer = teacher.page.getByRole("dialog", { name: "Tổng kết chặng" });
    await expect(drawer.getByRole("heading", { name: `${AN_NAME} · ${STAGE}`, exact: true })).toBeVisible();
    const form = drawer.getByRole("form", { name: "Nhận xét tổng kết chặng" });
    await form.getByRole("textbox", { name: "Nhận xét" }).fill(STAGE_REVIEW);
    await form.getByRole("button", { name: "Lưu nhận xét", exact: true }).click();
    await expect(teacher.page.getByText("Đã lưu nhận xét.", { exact: true })).toBeVisible();
    await expect(form.getByText(new RegExp(`^Cập nhật .* bởi ${TEACHER_NAME}$`))).toBeVisible();
    await teacher.page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await expect(anCell(teacher.page)).toHaveAccessibleName(
      `${AN_NAME}, ${STAGE}: Hoàn thành, đã nhận xét. Mở tổng kết chặng`,
    );
    await teacher.close();

    const student = await openAs(browser, "student", `/learn/classes/${classId}/summary`);
    const card = stageCard(student.page);
    await expect(card.getByText(STAGE_REVIEW, { exact: true })).toBeVisible();
    await expect(card.getByText(new RegExp(`^${TEACHER_NAME} · `))).toBeVisible();
    await student.close();
  });
});

/** Thẻ của chặng kịch bản trong "Tổng kết khóa học" (thẻ có h2 "n. Tên chặng"). */
function stageCard(page: Page): Locator {
  return page.locator('[data-slot="card"]').filter({
    has: page.getByRole("heading", { level: 2, name: new RegExp(`^\\d+\\. ${STAGE}$`) }),
  });
}
