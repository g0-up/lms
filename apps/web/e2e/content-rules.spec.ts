/*
 * Luật nội dung (E06–E07): chặn phát hành, xóa và lưu trữ phiên bản.
 * Ánh xạ check(...) của prototype/check-flows.mjs: prototype không có check riêng cho các luật này
 * (chỉ có ở tiêu chí FR-13, FR-15, FR-19 của spec); 'second clone is blocked (one draft max)' đã thuộc E01.
 */
import { expect, test, type APIRequestContext, type Page } from "@playwright/test";
import { apiAs, storageStatePath } from "./support/auth";
import { closeDb, count, ids, one } from "./support/db";
import { seedReset } from "./support/seed";

interface ApiError {
  error: { code: string; message: string; details?: { usedBy?: { courseName: string; versionNo: number }[] } };
}

test.describe("Luật phát hành, xóa và lưu trữ phiên bản", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });
  test.use({ storageState: storageStatePath("admin") });

  let admin: APIRequestContext;

  test.beforeAll(async () => {
    await seedReset();
    admin = await apiAs("admin");
  });

  test.afterAll(async () => {
    await admin.dispose();
    await seedReset();
    await closeDb();
  });

  async function createViaDialog(page: Page, listPath: string, label: string, code: string, name: string) {
    await page.goto(listPath);
    await page.getByRole("button", { name: label }).first().click();
    const dialog = page.getByRole("dialog", { name: label });
    await dialog.getByLabel(label === "Tạo chặng" ? "Mã chặng" : "Mã khóa học").fill(code);
    await dialog.getByLabel(label === "Tạo chặng" ? "Tên chặng" : "Tên khóa học").fill(name);
    await dialog.getByRole("button", { name: label }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
  }

  test("E06 chặn phát hành chặng rỗng và khóa học có chặng chưa phát hành", async ({ page }) => {
    await createViaDialog(page, "/admin/stages", "Tạo chặng", "E2E06", "Chặng E06");
    await expect(page.getByText("Đã tạo chặng với bản nháp v1.")).toBeVisible();
    const sv1 = await ids.stageVersionId("E2E06", 1);

    const publish = page.getByRole("button", { name: "Phát hành v1" });
    await expect(publish).toHaveAttribute("aria-disabled", "true");
    await expect(publish).toHaveAttribute("title", "Cần ít nhất một học liệu");
    // aria-disabled vẫn nhận click (Playwright coi là không enabled nên cần force): chỉ hiện lý do.
    await publish.click({ force: true });
    await expect(page.getByText("Cần ít nhất một học liệu", { exact: true }).last()).toBeVisible();
    await expect(page.getByRole("dialog")).toHaveCount(0);

    // Lỗi dữ liệu nghiệp vụ của API là 422 (cùng quy ước với "Khóa học cần ít nhất một chặng.").
    const refused = await admin.post(`/api/v1/stage-versions/${sv1}/publish`);
    expect(refused.status()).toBe(422);
    expect(((await refused.json()) as ApiError).error).toMatchObject({
      code: "VALIDATION_FAILED",
      message: "Chặng cần ít nhất một học liệu trước khi phát hành.",
    });
    expect((await one<{ status: string }>("SELECT status FROM stage_versions WHERE id = $1", [sv1])).status).toBe("draft");

    // Thêm một học liệu rồi phát hành để có bản published gắn vào khóa học.
    const lesson = await admin.post(`/api/v1/stage-versions/${sv1}/lessons`, {
      data: { title: "Bài đọc E06", type: "markdown", required: true, markdownSource: "# E06" },
    });
    expect(lesson.status(), await lesson.text()).toBeLessThan(300);
    expect((await admin.post(`/api/v1/stage-versions/${sv1}/publish`)).status()).toBe(200);

    await createViaDialog(page, "/admin/courses", "Tạo khóa học", "E2E06C", "Khóa học E06");
    await expect(page.getByText("Chưa phát hành được: Khóa học cần ít nhất một chặng.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Phát hành v1" })).toHaveAttribute("aria-disabled", "true");
    const cv1 = await ids.courseVersionId("E2E06C", 1);

    // Không gắn được bản nháp của chặng; chỉ bản đã phát hành.
    const clone = await admin.post(`/api/v1/stage-versions/${sv1}/clone`);
    expect(clone.status()).toBe(201);
    const sv2 = await ids.stageVersionId("E2E06", 2);
    const attachDraft = await admin.put(`/api/v1/course-versions/${cv1}/stages`, { data: { stageVersionIds: [sv2] } });
    expect(attachDraft.status()).toBe(422);
    expect(((await attachDraft.json()) as ApiError).error.message).toBe("Chỉ gắn được phiên bản chặng đã phát hành.");

    // Gắn v1 đã phát hành rồi lưu trữ nó: bản nháp khóa học bị chặn với danh sách chặng chưa phát hành.
    expect((await admin.put(`/api/v1/course-versions/${cv1}/stages`, { data: { stageVersionIds: [sv1] } })).status()).toBe(200);
    expect((await admin.post(`/api/v1/stage-versions/${sv1}/archive`)).status()).toBe(200);
    await page.reload();
    await expect(page.getByText("Chưa phát hành được: Chặng chưa phát hành: Chặng E06 v1.")).toBeVisible();
    const coursePublish = page.getByRole("button", { name: "Phát hành v1" });
    await expect(coursePublish).toHaveAttribute("title", "Chặng chưa phát hành: Chặng E06 v1.");
    const courseRefused = await admin.post(`/api/v1/course-versions/${cv1}/publish`);
    expect(courseRefused.status()).toBe(422);
    expect(((await courseRefused.json()) as ApiError).error.message).toBe(
      "Phiên bản chặng E2E06 v1 đã lưu trữ, không thể phát hành lại.",
    );
    expect((await one<{ status: string }>("SELECT status FROM course_versions WHERE id = $1", [cv1])).status).toBe("draft");
  });

  test("E07 xóa bản nháp, chặn xóa bản đang dùng, lưu trữ", async ({ page }) => {
    const dbStage = await ids.stageId("DB");
    const svV1 = await ids.stageVersionId("DB", 1);

    // Xóa bản nháp: học liệu bị xóa theo (cascade).
    const clone = await admin.post(`/api/v1/stage-versions/${svV1}/clone`);
    expect(clone.status()).toBe(201);
    const draft = await ids.stageVersionId("DB", 2);
    expect(await count("SELECT count(*) AS n FROM lessons WHERE stage_version_id = $1", [draft])).toBe(3);
    await page.goto(`/admin/stages/${dbStage}?v=${draft}`);
    await page.getByRole("button", { name: "Xóa bản nháp" }).click();
    const delDialog = page.getByRole("dialog", { name: "Xóa Database v2?" });
    await expect(delDialog).toContainText("Bản nháp và 3 học liệu trong đó sẽ bị xóa.");
    await delDialog.getByRole("button", { name: "Xóa phiên bản" }).click();
    await expect(page.getByText("Đã xóa.")).toBeVisible();
    expect(await count("SELECT count(*) AS n FROM stage_versions WHERE id = $1", [draft])).toBe(0);
    expect(await count("SELECT count(*) AS n FROM lessons WHERE stage_version_id = $1", [draft])).toBe(0);
    expect(await count("SELECT count(*) AS n FROM audit_logs WHERE action = 'stage_version.deleted' AND target_id = $1", [draft])).toBe(1);

    // Bản published đang được khóa học tham chiếu: nút xóa bị chặn, API 409 IN_USE kèm usedBy.
    await page.goto(`/admin/stages/${dbStage}?v=${svV1}`);
    const del = page.getByRole("button", { name: "Xóa", exact: true });
    await expect(del).toHaveAttribute("aria-disabled", "true");
    await expect(del).toHaveAttribute("title", "Đang được dùng trong Lập trình cơ bản v1. Hãy lưu trữ thay vì xóa.");
    await del.click({ force: true });
    await expect(page.getByRole("dialog")).toHaveCount(0);
    const inUse = await admin.delete(`/api/v1/stage-versions/${svV1}`);
    expect(inUse.status()).toBe(409);
    const inUseBody = (await inUse.json()) as ApiError;
    expect(inUseBody.error.code).toBe("IN_USE");
    expect(inUseBody.error.details?.usedBy).toEqual([expect.objectContaining({ courseName: "Lập trình cơ bản", versionNo: 1 })]);

    // Khóa học: bản published có lớp gắn cũng không xóa được.
    const cv1 = await ids.courseVersionId("BASIC", 1);
    const courseInUse = await admin.delete(`/api/v1/course-versions/${cv1}`);
    expect(courseInUse.status()).toBe(409);
    expect(((await courseInUse.json()) as ApiError).error.code).toBe("IN_USE");

    // Lưu trữ phiên bản chặng qua UI; khóa học và lớp đang dùng không bị ảnh hưởng.
    await page.getByRole("button", { name: "Lưu trữ" }).click();
    await page.getByRole("dialog", { name: "Lưu trữ v1?" }).getByRole("button", { name: "Lưu trữ" }).click();
    await expect(page.getByText("Đã lưu trữ.")).toBeVisible();
    const archived = await one<{ status: string; archived_at: Date | null }>(
      "SELECT status, archived_at FROM stage_versions WHERE id = $1",
      [svV1],
    );
    expect(archived.status).toBe("archived");
    expect(archived.archived_at).not.toBeNull();
    expect(await count("SELECT count(*) AS n FROM course_version_stages WHERE stage_version_id = $1", [svV1])).toBe(1);

    const courseArchive = await admin.post(`/api/v1/course-versions/${cv1}/archive`);
    expect(courseArchive.status(), await courseArchive.text()).toBe(200);
    expect(await count("SELECT count(*) AS n FROM classes WHERE course_version_id = $1", [cv1])).toBe(3);
    expect(
      await count(
        `SELECT count(*) AS n FROM audit_logs
          WHERE (action = 'stage_version.archived' AND target_id = $1) OR (action = 'course_version.archived' AND target_id = $2)`,
        [svV1, cv1],
      ),
    ).toBe(2);
  });
});
