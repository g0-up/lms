/*
 * Bài học (FR-78) trên stack thật: admin soạn bài học trong bản nháp chặng, học viên học theo bài học.
 * Ánh xạ check(...) của prototype/check-flows.mjs:
 *   'bài học is added at the end of the stage'                                         → soạn bài học
 *   'an empty bài học blocks publishing'                                               → soạn bài học
 *   'moving the last học liệu of a bài học down puts it first in the next bài học'     → soạn bài học
 *   'the empty bài học is deleted and publishing unblocks'                             → soạn bài học
 *   'roadmap groups each stage by bài học with progress per bài học'                   → học theo bài học
 *   'lesson breadcrumb shows the bài học' / 'lesson side list groups học liệu by bài học' → học theo bài học
 *   'next stays inside the bài học' / 'next crosses into the next bài học from its last học liệu' → học theo bài học
 * Tạo chặng E2E78 (bài học "Nhập môn" với ba bài đọc) qua API rồi phát hành qua giao diện, nên chạy @serial và
 * seed lại ở afterAll.
 */
import { expect, test, type APIRequestContext, type Page } from "@playwright/test";
import { apiAs, storageStatePath } from "./support/auth";
import { closeDb, ids, one, query } from "./support/db";
import { activateClass, createClass, createUnit, invite, seedReset } from "./support/seed";

const AN = "an.nguyen@gmail.com";
const STAGE = "Chặng bài học E2E";
const emptyUnit = (title: string) => `Bài học "${title}" chưa có học liệu.`;

/** Bài học trong trình soạn chặng: section mang tên theo tiêu đề h3 của nó. */
function unitSection(page: Page, title: string) {
  return page.getByRole("region", { name: title, exact: true });
}

/** Bấm nút chuyển xuống của học liệu rồi chờ API lưu thứ tự mới. */
async function moveDown(page: Page, lessonTitle: string) {
  const saved = page.waitForResponse(
    (res) => res.url().endsWith("/lessons/order") && res.request().method() === "PUT",
  );
  await page.getByRole("button", { name: `Chuyển xuống: ${lessonTitle}` }).click();
  expect((await saved).status()).toBe(200);
}

test.describe("Bài học: soạn trong chặng và học theo bài học", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });
  test.use({ storageState: storageStatePath("admin") });

  let admin: APIRequestContext;
  let stageId: string;
  let svId: string;

  test.beforeAll(async () => {
    await seedReset();
    admin = await apiAs("admin");
    expect((await admin.post("/api/v1/stages", { data: { code: "E2E78", name: STAGE } })).status()).toBe(201);
    stageId = await ids.stageId("E2E78");
    svId = await ids.stageVersionId("E2E78", 1);
    const unitId = await createUnit(admin, svId, "Nhập môn");
    for (const n of [1, 2, 3]) {
      const res = await admin.post(`/api/v1/stage-versions/${svId}/lessons`, {
        data: {
          unitId,
          title: `Bài đọc ${String(n)}`,
          type: "markdown",
          required: true,
          markdownSource: `Nội dung bài đọc ${String(n)}.`,
        },
      });
      expect(res.status(), await res.text()).toBe(201);
    }
  });

  test.afterAll(async () => {
    await admin.dispose();
    await seedReset();
    await closeDb();
  });

  test("admin thêm bài học, chuyển học liệu qua ranh giới, xóa bài học rỗng rồi phát hành", async ({ page }) => {
    await page.goto(`/admin/stages/${stageId}?v=${svId}`);
    await expect(page.getByRole("heading", { level: 1, name: STAGE })).toBeVisible();
    await expect(unitSection(page, "Nhập môn").getByText(/^Bài đọc \d$/)).toHaveText(["Bài đọc 1", "Bài đọc 2", "Bài đọc 3"]);

    for (const title of ["Nâng cao", "Ôn tập"]) {
      await page.getByRole("button", { name: "Thêm bài học", exact: true }).click();
      const dialog = page.getByRole("dialog", { name: "Thêm bài học" });
      await dialog.getByLabel(/^Tên bài học/).fill(title);
      await dialog.getByRole("button", { name: "Thêm", exact: true }).click();
      await expect(dialog).toBeHidden();
      await expect(unitSection(page, title)).toBeVisible();
    }
    await expect(page.getByRole("heading", { level: 3 })).toHaveText(["Nhập môn", "Nâng cao", "Ôn tập"]);

    // Bài học rỗng chặn phát hành, nêu đúng tên từng bài học.
    const publish = page.getByRole("button", { name: "Phát hành v1" });
    await expect(publish).toHaveAttribute("aria-disabled", "true");
    await expect(publish).toHaveAttribute("title", emptyUnit("Nâng cao"));
    await expect(page.getByText(emptyUnit("Nâng cao"), { exact: true })).toBeVisible();
    await expect(page.getByText(emptyUnit("Ôn tập"), { exact: true })).toBeVisible();

    // Học liệu cuối của bài học đi xuống thành học liệu đầu của bài học sau.
    await moveDown(page, "Bài đọc 3");
    await moveDown(page, "Bài đọc 2");
    await expect(unitSection(page, "Nhập môn").getByText(/^Bài đọc \d$/)).toHaveText(["Bài đọc 1"]);
    await expect(unitSection(page, "Nâng cao").getByText(/^Bài đọc \d$/)).toHaveText(["Bài đọc 2", "Bài đọc 3"]);
    const layout = await query<{ lesson: string; unit: string }>(
      `SELECT l.title AS lesson, u.title AS unit FROM lessons l JOIN stage_units u ON u.id = l.unit_id
        WHERE l.stage_version_id = $1 ORDER BY l.position`,
      [svId],
    );
    expect(layout.map((r) => [r.lesson, r.unit])).toEqual([
      ["Bài đọc 1", "Nhập môn"],
      ["Bài đọc 2", "Nâng cao"],
      ["Bài đọc 3", "Nâng cao"],
    ]);
    await expect(page.getByText(emptyUnit("Nâng cao"), { exact: true })).toHaveCount(0);
    await expect(publish).toHaveAttribute("title", emptyUnit("Ôn tập"));

    await page.getByRole("button", { name: "Xóa bài học: Ôn tập" }).click();
    const confirm = page.getByRole("dialog", { name: 'Xóa bài học "Ôn tập"?' });
    await expect(confirm).toContainText("Bài học trống bị xóa khỏi bản nháp này.");
    await confirm.getByRole("button", { name: "Xóa bài học" }).click();
    await expect(page.getByText("Đã xóa bài học.")).toBeVisible();
    await expect(unitSection(page, "Ôn tập")).toHaveCount(0);
    await expect(page.getByText(/chưa có học liệu\.$/)).toHaveCount(0);

    await expect(publish).not.toHaveAttribute("aria-disabled");
    await publish.click();
    const dialog = page.getByRole("dialog", { name: `Phát hành ${STAGE} v1?` });
    await dialog.getByRole("button", { name: "Phát hành", exact: true }).click();
    await expect(dialog).toBeHidden();
    expect((await one<{ status: string }>("SELECT status FROM stage_versions WHERE id = $1", [svId])).status).toBe(
      "published",
    );
    expect(await query("SELECT id FROM stage_units WHERE stage_version_id = $1", [svId])).toHaveLength(2);
  });

  test("học viên thấy lộ trình theo bài học và sang bài học tiếp", async ({ browser }) => {
    expect((await admin.post("/api/v1/courses", { data: { code: "E2E78C", name: "Khóa học bài học E2E" } })).status()).toBe(
      201,
    );
    const cv = await ids.courseVersionId("E2E78C", 1);
    expect((await admin.put(`/api/v1/course-versions/${cv}/stages`, { data: { stageVersionIds: [svId] } })).status()).toBe(
      200,
    );
    expect((await admin.post(`/api/v1/course-versions/${cv}/publish`)).status()).toBe(200);
    const cls = await createClass(admin, "e2e78", cv);
    await invite(admin, cls.id, AN);
    await activateClass(admin, cls.id);
    const lessonId = async (title: string) =>
      (await one<{ id: string }>("SELECT id FROM lessons WHERE stage_version_id = $1 AND title = $2", [svId, title])).id;
    const unitId = async (title: string) =>
      (await one<{ id: string }>("SELECT id FROM stage_units WHERE stage_version_id = $1 AND title = $2", [svId, title]))
        .id;
    const classHref = `/learn/classes/${cls.id}`;

    const learner = await browser.newContext({ storageState: storageStatePath("student") });
    const page = await learner.newPage();
    await page.goto(classHref);
    await expect(page.getByText("2 bài học · 3 học liệu · 0/3 bắt buộc đã xong")).toBeVisible();
    const first = page.getByRole("region", { name: "Bài 1.1 · Nhập môn" });
    const second = page.getByRole("region", { name: "Bài 1.2 · Nâng cao" });
    await expect(first).toContainText("0/1 bắt buộc");
    await expect(second).toContainText("0/2 bắt buộc");
    await expect(second.getByRole("link", { name: /Bài đọc/ })).toHaveCount(2);

    await first.getByRole("link", { name: /Bài đọc 1/ }).click();
    await expect(page.getByRole("heading", { level: 1, name: "Bài đọc 1" })).toBeVisible();
    const crumbs = page.getByRole("navigation", { name: "Đường dẫn" });
    await expect(crumbs.getByRole("link", { name: "Nhập môn" })).toHaveAttribute(
      "href",
      `${classHref}#unit-${await unitId("Nhập môn")}`,
    );
    const side = page.getByRole("complementary", { name: `Học liệu chặng ${STAGE}` });
    await expect(side.getByRole("heading", { level: 3 })).toHaveText(["Nhập môn", "Nâng cao"]);

    // Học liệu cuối của bài học: nút tiếp sang bài học sau, title là tên bài học đó.
    const cross = page.getByRole("link", { name: "Sang bài học tiếp" });
    await expect(cross).toHaveAttribute("title", "Nâng cao");
    await cross.click();
    await expect(page).toHaveURL(new RegExp(`/lessons/${await lessonId("Bài đọc 2")}$`));
    await expect(page.getByRole("heading", { level: 1, name: "Bài đọc 2" })).toBeVisible();
    await expect(crumbs.getByRole("link", { name: "Nâng cao" })).toBeVisible();
    await expect(
      side.getByRole("region", { name: "Nâng cao" }).getByRole("link", { name: "Bài đọc 2" }),
    ).toHaveAttribute("aria-current", "page");
    // Trong cùng bài học, nút tiếp giữ nhãn thường.
    await expect(page.getByRole("link", { name: "Học liệu tiếp" })).toHaveAttribute(
      "href",
      `${classHref}/lessons/${await lessonId("Bài đọc 3")}`,
    );
    await expect(page.getByRole("link", { name: "Sang bài học tiếp" })).toHaveCount(0);

    // Breadcrumb bài học mở lộ trình tại nhóm của bài học đó.
    await crumbs.getByRole("link", { name: "Nâng cao" }).click();
    await expect(page).toHaveURL(new RegExp(`#unit-${await unitId("Nâng cao")}$`));
    await expect(page.getByRole("region", { name: "Bài 1.2 · Nâng cao" })).toBeInViewport();
    await learner.close();
  });
});
