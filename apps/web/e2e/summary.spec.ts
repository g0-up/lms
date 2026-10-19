/*
 * Tổng kết khóa học (FR-80, FR-82, FR-91) trên stack thật. Seed basic01 không có bài nào "Cần làm lại", nên trước tiên An
 * nộp "Bài tập: Đọc CSV và thống kê" của basic01 qua API và giảng viên Hương chấm 0 điểm (dưới ngưỡng đạt).
 * Sau đó Hương mở tab "Tổng kết" của lớp, lọc "Có bài cần làm lại", mở ô Golang basic của An bằng bàn phím,
 * viết nhận xét rồi sửa (thấy "đã sửa 1 lần"); đóng drawer thì focus quay về ô. An mở "Tổng kết khóa học"
 * và thấy nhận xét; lộ trình hiện nhãn "Cần làm lại" cạnh bài tập. Axe trên tab và trang.
 * Số liệu (x/y học viên, x/y học liệu) lấy từ API, không chép từ seed. Ghi bài nộp, điểm và nhận xét vào
 * DB seed, nên chạy @serial và seed lại ở beforeAll/afterAll.
 */
import { expect, test, type APIRequestContext, type Browser, type Page } from "@playwright/test";
import { runAxe, waitForPage } from "./support/a11y";
import { apiAs, browserAs, type Role } from "./support/auth";
import { closeDb, ids } from "./support/db";
import { seedReset } from "./support/seed";

const AN_NAME = "Nguyễn Hoàng An";
const HOMEWORK = "Bài tập: Đọc CSV và thống kê";
const STAGE = "Golang basic";
const REVIEW = "An đọc CSV ổn nhưng chưa xử lý lỗi file không tồn tại.";
const REVIEW_EDIT = `${REVIEW} Làm lại bài tập trước khi sang ReactJS.`;

interface MatrixCell {
  stageVersionId: string;
  rework: boolean;
}
interface ClassMatrix {
  stages: { stageVersionId: string; name: string }[];
  rows: { memberId: string; name: string; cells: MatrixCell[] }[];
}
interface LearnerSummary {
  requiredDone: number;
  requiredTotal: number;
}

let classId: string;
let lessonId: string;
let teacherApi: APIRequestContext;
let studentApi: APIRequestContext;

const tabPath = () => `/teach/classes/${classId}?tab=summary`;
const summaryPath = () => `/learn/classes/${classId}/summary`;

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

/** Ô của An ở chặng Golang basic; tên truy cập của ô bắt đầu bằng "học viên, chặng:". */
function anGoCell(page: Page) {
  return page.getByRole("button", { name: new RegExp(`^${AN_NAME}, ${STAGE}: `) });
}

async function expectNoAxeViolations(page: Page): Promise<void> {
  const { violations } = await runAxe(page);
  expect(violations, JSON.stringify(violations, null, 2)).toEqual([]);
}

test.describe("Tổng kết khóa học", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });

  test.beforeAll(async () => {
    await seedReset();
    classId = await ids.classId("basic01");
    lessonId = await ids.lessonInClass("basic01", "go-hw");
    teacherApi = await apiAs("teacher");
    studentApi = await apiAs("student");

    // An nộp bài tập Go, Hương chấm 0 điểm mọi tiêu chí: bài thành "Cần làm lại".
    const submitted = await studentApi.post(`/api/v1/me/classes/${classId}/lessons/${lessonId}/submissions`, {
      data: { repoUrl: "https://github.com/hoangan-dev/go-csv-stats", commitRef: "", mediaIds: [] },
    });
    expect(submitted.status(), await submitted.text()).toBe(201);
    const { submissions } = (await submitted.json()) as { submissions: { id: string; no: number }[] };
    const latest = submissions.reduce((a, b) => (b.no > a.no ? b : a));
    const detail = await getJson<{ lesson: { criteria: { id: string }[] } }>(teacherApi, `/api/v1/submissions/${latest.id}`);
    const graded = await teacherApi.post(`/api/v1/submissions/${latest.id}/grades`, {
      data: {
        scores: detail.lesson.criteria.map((c) => ({ criterionId: c.id, points: 0, comment: "" })),
        comment: "Chưa xử lý lỗi đọc file, làm lại nhé.",
      },
    });
    expect(graded.status(), await graded.text()).toBeLessThan(300);
  });

  test.afterAll(async () => {
    await teacherApi.dispose();
    await studentApi.dispose();
    await closeDb();
    await seedReset();
  });

  test("giảng viên lọc bài cần làm lại, viết rồi sửa nhận xét chặng", async ({ browser }) => {
    const matrix = await getJson<ClassMatrix>(teacherApi, `/api/v1/classes/${classId}/summary`);
    const goStage = matrix.stages.find((s) => s.name === STAGE);
    expect(goStage).toBeDefined();
    const reworkRows = matrix.rows.filter((r) => r.cells.some((c) => c.rework));
    const an = matrix.rows.find((r) => r.name === AN_NAME);
    expect(an?.cells.find((c) => c.stageVersionId === goStage?.stageVersionId)?.rework).toBe(true);

    const { page, close } = await openAs(browser, "teacher", tabPath());
    await waitForPage(page);
    await expect(page.getByRole("link", { name: "Tổng kết", exact: true })).toHaveAttribute("aria-current", "page");
    await expect(page.getByRole("table", { name: /^Tổng kết theo chặng lớp / })).toBeVisible();
    const total = String(matrix.rows.length);
    await expect(page.getByText(`${total}/${total} học viên · Bấm một ô`)).toBeVisible();

    await page.getByRole("checkbox", { name: "Có bài cần làm lại" }).check();
    await page.getByRole("button", { name: "Lọc", exact: true }).click();
    await expect(page).toHaveURL(/[?&]rework=1(&|$)/);
    await expect(page).toHaveURL(/[?&]tab=summary(&|$)/);
    await expect(page.getByText(`${String(reworkRows.length)}/${total} học viên`)).toBeVisible();
    await expect(page.getByRole("rowheader")).toHaveCount(reworkRows.length);
    await expect(anGoCell(page)).toHaveAccessibleName(/có bài cần làm lại, chưa nhận xét\. Mở tổng kết chặng$/);
    await expectNoAxeViolations(page);

    // Mở ô bằng bàn phím.
    await anGoCell(page).focus();
    await page.keyboard.press("Enter");
    const drawer = page.getByRole("dialog", { name: "Tổng kết chặng" });
    await expect(drawer.getByRole("heading", { name: `${AN_NAME} · ${STAGE}`, exact: true })).toBeVisible();
    await expect(drawer.getByText(/^GO · v\d+ · Còn \d+ học liệu bắt buộc$/)).toBeVisible();
    const homeworkRow = drawer.getByRole("listitem").filter({ hasText: HOMEWORK });
    await expect(homeworkRow.getByText("Cần làm lại", { exact: true })).toBeVisible();
    await expectNoAxeViolations(page);

    const form = drawer.getByRole("form", { name: "Nhận xét tổng kết chặng" });
    const field = form.getByRole("textbox", { name: "Nhận xét" });
    await field.fill(REVIEW);
    await form.getByRole("button", { name: "Lưu nhận xét", exact: true }).click();
    await expect(page.getByText("Đã lưu nhận xét.", { exact: true })).toBeVisible();
    await expect(form.getByText(/^Cập nhật .* bởi Lê Thu Hương$/)).toBeVisible();

    await field.fill(REVIEW_EDIT);
    await form.getByRole("button", { name: "Lưu thay đổi", exact: true }).click();
    await expect(page.getByText("Đã lưu thay đổi nhận xét.", { exact: true })).toBeVisible();
    await expect(form.getByText(/^Cập nhật .* bởi Lê Thu Hương · đã sửa 1 lần$/)).toBeVisible();
    await expect(field).toHaveValue(REVIEW_EDIT);

    // Đóng drawer: focus về lại ô vừa mở, ô đã có nhận xét.
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await expect(anGoCell(page)).toBeFocused();
    await expect(anGoCell(page)).toHaveAccessibleName(/đã nhận xét\. Mở tổng kết chặng$/);
    await close();
  });

  test("học viên thấy nhận xét trong trang tổng kết và nhãn trạng thái trên lộ trình", async ({ browser }) => {
    const summary = await getJson<LearnerSummary>(studentApi, `/api/v1/me/classes/${classId}/summary`);

    const { page, close } = await openAs(browser, "student", `/learn/classes/${classId}`);
    await waitForPage(page);
    // Badge trạng thái nằm cạnh link bài tập trong cùng hàng lộ trình; tên link mở đầu bằng nhãn icon loại học liệu.
    const roadmapRow = page.getByRole("link", { name: HOMEWORK }).locator("xpath=..");
    await expect(roadmapRow.getByText("Cần làm lại", { exact: true })).toBeVisible();

    await page.getByRole("link", { name: "Tổng kết khóa học" }).click();
    await expect(page).toHaveURL(summaryPath());
    await waitForPage(page);
    await expect(page.getByRole("heading", { level: 1, name: "Tổng kết khóa học" })).toBeVisible();
    await expect(
      page.getByText(`${String(summary.requiredDone)}/${String(summary.requiredTotal)} học liệu bắt buộc. Học liệu không bắt buộc`),
    ).toBeVisible();

    const goCard = page.locator('[data-slot="card"]').filter({
      has: page.getByRole("heading", { level: 2, name: new RegExp(`^\\d+\\. ${STAGE}$`) }),
    });
    await expect(goCard.getByText("Có bài cần làm lại", { exact: true })).toBeVisible();
    await expect(goCard.getByRole("heading", { level: 3, name: "Còn thiếu" })).toBeVisible();
    await expect(goCard.getByText(REVIEW_EDIT, { exact: true })).toBeVisible();
    await expect(goCard.getByText(/^Lê Thu Hương · /)).toBeVisible();
    await expectNoAxeViolations(page);
    await close();
  });
});
