/*
 * Kịch bản 7.3 (E01–E05) trên stack thật.
 * Ánh xạ check(...) của prototype/check-flows.mjs:
 *   'stage page shows v1 published'                          → E01 (heading "Học liệu của v1" trước khi nhân bản)
 *   'clone creates draft and navigates'                      → E01
 *   'draft v2 copies 3 lessons with same keys'               → E01
 *   'second clone is blocked (one draft max)'                → E01
 *   'lesson added to draft' / 'published v1 untouched'       → E02
 *   'v2 published' / 'FR-18 warning lists … v1 on stage page'→ E03
 *   'dashboard shows outdated warning'                       → E03
 *   'apply modal explains atomic transaction'                → E04
 *   'course v2 created and published'                        → E04
 *   'course v2 uses Database v2, other stages unchanged'     → E04
 *   'course v1 still published and unchanged'                → E04
 *   'basic01/02/03 remain on course v1'                      → E04
 *   'warning disappears after apply' / 'dashboard warning gone' → E04
 *   'course page lists v1 and v2'                            → E04
 *   (tiêu chí chấp nhận 7.3: lớp nháp đổi phiên bản, lớp active giữ phiên bản) → E05
 *   'reset data' / 'reset restores seed' (chỉ có ở prototype) → afterAll seed-reset
 */
import path from "node:path";
import { expect, test, type APIRequestContext, type Page } from "@playwright/test";
import { apiAs, storageStatePath } from "./support/auth";
import { closeDb, count, ids, one, query } from "./support/db";
import { invite, seedReset } from "./support/seed";
import { kpi } from "./support/ui";

const SAMPLE_MP4 = path.resolve(import.meta.dirname, "../../api/internal/seed/assets/sample.mp4");

test.describe("Kịch bản 7.3: nhân bản, sửa, phát hành, áp dụng, đổi phiên bản lớp", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });
  test.use({ storageState: storageStatePath("admin") });

  let admin: APIRequestContext;
  let student: APIRequestContext;
  let dbStageId: string;
  let svV1: string;
  let svV2: string;
  let basic01: string;
  let basic03: string;
  let anPercentBefore: number;

  test.beforeAll(async () => {
    await seedReset();
    admin = await apiAs("admin");
    student = await apiAs("student");
    dbStageId = await ids.stageId("DB");
    svV1 = await ids.stageVersionId("DB", 1);
    basic01 = await ids.classId("basic01");
    basic03 = await ids.classId("basic03");
    const roadmap = await student.get(`/api/v1/me/classes/${basic01}`);
    expect(roadmap.status()).toBe(200);
    anPercentBefore = ((await roadmap.json()) as { percent: number }).percent;
  });

  test.afterAll(async () => {
    await admin.dispose();
    await student.dispose();
    await seedReset();
    await closeDb();
  });

  async function openStage(page: Page, vid?: string) {
    await page.goto(`/admin/stages/${dbStageId}${vid ? `?v=${vid}` : ""}`);
    await expect(page.getByRole("heading", { level: 1, name: "Database" })).toBeVisible();
  }

  test("E01 nhân bản v1 thành bản nháp v2", async ({ page }) => {
    await openStage(page);
    await expect(page.getByText("Học liệu của v1")).toBeVisible();
    await page.getByRole("button", { name: "Nhân bản thành bản nháp" }).click();

    await expect(page).toHaveURL(/\?v=[0-9a-f-]{36}$/);
    await expect(page.getByText("Học liệu của v2")).toBeVisible();
    svV2 = new URL(page.url()).searchParams.get("v") ?? "";
    expect(svV2).not.toBe(svV1);

    const drafts = await query<{ id: string; version_no: number }>(
      "SELECT id, version_no FROM stage_versions WHERE stage_id = $1 AND status = 'draft'",
      [dbStageId],
    );
    expect(drafts).toEqual([{ id: svV2, version_no: 2 }]);
    const lessons = (vid: string) =>
      query<{ lesson_key: string; video_media_id: string | null }>(
        "SELECT lesson_key, video_media_id FROM lessons WHERE stage_version_id = $1 ORDER BY position",
        [vid],
      );
    const [v1Lessons, v2Lessons] = await Promise.all([lessons(svV1), lessons(svV2)]);
    expect(v2Lessons).toHaveLength(3);
    // Cùng lesson_key và cùng media_files (không sao chép file video).
    expect(v2Lessons).toEqual(v1Lessons);
    expect(v2Lessons[0]?.video_media_id).toBeTruthy();

    // Chỉ một bản nháp: bản nháp không có nút nhân bản, v1 chuyển sang liên kết mở bản nháp.
    await expect(page.getByRole("button", { name: /Nhân bản/ })).toHaveCount(0);
    await openStage(page, svV1);
    await expect(page.getByRole("link", { name: "Mở bản nháp v2" })).toBeVisible();
    await expect(page.getByRole("button", { name: /Nhân bản/ })).toHaveCount(0);
  });

  test("E02 thêm học liệu video vào bản nháp; bản phát hành bất biến", async ({ page }) => {
    await openStage(page, svV2);
    await page.getByRole("button", { name: "Thêm học liệu" }).click();
    const dialog = page.getByRole("dialog", { name: "Thêm học liệu" });
    await dialog.getByLabel("Tiêu đề").fill("JOIN và subquery");
    await expect(dialog.getByRole("combobox", { name: "Loại" })).toHaveText("Video");
    await dialog.locator('input[type="file"]').setInputFiles(SAMPLE_MP4);
    const submit = dialog.getByRole("button", { name: "Thêm", exact: true });
    await expect(submit).toBeEnabled({ timeout: 30_000 });
    await expect(dialog.getByText("sample.mp4")).toBeVisible();
    // Thời lượng tự điền từ metadata của file; ghi đè bằng giá trị của kịch bản.
    await dialog.getByLabel("Thời lượng").fill("12:40");
    await expect(dialog.getByRole("checkbox", { name: "Bắt buộc (tính vào %)" })).toBeChecked();
    await submit.click();
    await expect(dialog).toBeHidden();
    await expect(page.getByText("JOIN và subquery")).toBeVisible();

    const added = await one<{ n: string; title: string; type: string; required: boolean; duration_seconds: number }>(
      `SELECT (SELECT count(*) FROM lessons WHERE stage_version_id = $1) AS n, title, type, required, duration_seconds
         FROM lessons WHERE stage_version_id = $1 ORDER BY position DESC LIMIT 1`,
      [svV2],
    );
    expect(added).toMatchObject({ n: "4", title: "JOIN và subquery", type: "video", required: true, duration_seconds: 760 });
    expect(await count("SELECT count(*) AS n FROM lessons WHERE stage_version_id = $1", [svV1])).toBe(3);

    const v1Lesson = await one<{ id: string }>(
      "SELECT id FROM lessons WHERE stage_version_id = $1 AND lesson_key = 'db-table'",
      [svV1],
    );
    const patch = await admin.patch(`/api/v1/stage-versions/${svV1}/lessons/${v1Lesson.id}`, {
      data: { title: "Sửa lén bản đã phát hành" },
    });
    expect(patch.status()).toBe(409);
    expect(((await patch.json()) as { error: { code: string } }).error.code).toBe("VERSION_IMMUTABLE");
  });

  test("E03 phát hành v2; cảnh báo khóa học dùng bản cũ", async ({ page }) => {
    await openStage(page, svV2);
    await page.getByRole("button", { name: "Phát hành v2" }).click();
    const dialog = page.getByRole("dialog", { name: "Phát hành Database v2?" });
    await dialog.getByRole("button", { name: "Phát hành", exact: true }).click();
    await expect(dialog).toBeHidden();

    const card = page.locator('[data-slot="card"]').filter({ hasText: "Học liệu của v2" });
    await expect(card.getByText("Đã phát hành", { exact: true })).toBeVisible();
    const row = await one<{ status: string; published_at: Date | null }>(
      "SELECT status, published_at FROM stage_versions WHERE id = $1",
      [svV2],
    );
    expect(row.status).toBe("published");
    expect(row.published_at).not.toBeNull();

    const outdated = page.locator('[data-slot="card"]').filter({ hasText: "Khóa học đang dùng phiên bản cũ của chặng này" });
    await expect(outdated.getByRole("link", { name: "Lập trình cơ bản v1" })).toBeVisible();

    await page.goto("/admin");
    await expect(page.getByText("vẫn dùng Database v1 trong khi v2 đã phát hành.")).toBeVisible();
    await expect(kpi(page, "Khóa học dùng chặng cũ")).toHaveText("1");
    expect(
      await count("SELECT count(*) AS n FROM audit_logs WHERE action = 'stage_version.published' AND target_id = $1", [svV2]),
    ).toBe(1);
  });

  test("E04 áp dụng v2 cho Lập trình cơ bản trong một thao tác", async ({ page }) => {
    await page.goto("/admin");
    await page.getByRole("link", { name: "Xem và áp dụng" }).click();
    await expect(page).toHaveURL(new RegExp(`/admin/stages/${dbStageId}\\?v=${svV2}$`));

    await page.getByRole("button", { name: "Áp dụng v2" }).click();
    const dialog = page.getByRole("dialog", { name: "Áp dụng Database v2 cho Lập trình cơ bản" });
    await expect(dialog).toContainText(/Nhân bản Lập trình cơ bản v1 thành v2[\s\S]*Phát hành v2/);
    await dialog.getByRole("button", { name: "Tạo và phát hành Lập trình cơ bản v2" }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByText("Đã phát hành Lập trình cơ bản v2 dùng v2.")).toBeVisible();

    const versions = await query<{ id: string; version_no: number; status: string }>(
      `SELECT cv.id, cv.version_no, cv.status FROM course_versions cv JOIN courses c ON c.id = cv.course_id
        WHERE c.code = 'BASIC' ORDER BY cv.version_no`,
    );
    expect(versions.map((v) => [v.version_no, v.status])).toEqual([
      [1, "published"],
      [2, "published"],
    ]);
    const stagesOf = async (cvId: string) =>
      (
        await query<{ stage_version_id: string }>(
          "SELECT stage_version_id FROM course_version_stages WHERE course_version_id = $1 ORDER BY position",
          [cvId],
        )
      ).map((r) => r.stage_version_id);
    const cv1 = versions.at(0);
    const cv2 = versions.at(1);
    if (!cv1 || !cv2) throw new Error("thiếu phiên bản khóa học");
    const [s1, s2] = await Promise.all([stagesOf(cv1.id), stagesOf(cv2.id)]);
    expect(s2).toContain(svV2);
    expect(s2).not.toContain(svV1);
    expect(s2).toHaveLength(s1.length);
    expect(s1).toContain(svV1);
    expect(s2.filter((id) => id !== svV2)).toEqual(s1.filter((id) => id !== svV1));
    const classVersions = await query<{ code: string; course_version_id: string }>(
      "SELECT code, course_version_id FROM classes ORDER BY code",
    );
    expect(classVersions.every((c) => c.course_version_id === cv1.id)).toBe(true);

    await expect(page.getByText("Khóa học đang dùng phiên bản cũ của chặng này")).toHaveCount(0);
    await expect(page.getByText("Mọi khóa học đã phát hành đều dùng phiên bản mới nhất của chặng này.")).toBeVisible();

    // Ba bản ghi audit của cùng một giao dịch có cùng `at`.
    const audits = await query<{ action: string; at: Date }>(
      `SELECT action, at FROM audit_logs
        WHERE action IN ('course.stage_version_applied', 'course_version.cloned', 'course_version.published')
          AND at > (SELECT published_at FROM stage_versions WHERE id = $1)`,
      [svV2],
    );
    expect(audits.map((a) => a.action).sort()).toEqual([
      "course.stage_version_applied",
      "course_version.cloned",
      "course_version.published",
    ]);
    expect(new Set(audits.map((a) => a.at.toISOString())).size).toBe(1);

    await page.goto("/admin");
    await expect(page.getByText(/vẫn dùng Database v1/)).toHaveCount(0);
    await expect(kpi(page, "Khóa học dùng chặng cũ")).toHaveText("0");

    await page.goto(`/admin/courses/${await ids.courseId("BASIC")}`);
    await expect(page.getByRole("link", { name: "v1", exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "v2", exact: true })).toBeVisible();
  });

  test("E05 lớp nháp đổi sang v2; lớp đang chạy giữ v1", async ({ page, browser }) => {
    const cv2 = await ids.courseVersionId("BASIC", 2);
    // Bản nháp v3 để chứng minh select chỉ liệt kê bản đã phát hành.
    const clone = await admin.post(`/api/v1/course-versions/${cv2}/clone`);
    expect(clone.status(), await clone.text()).toBe(201);
    // Seed: An không thuộc basic03; thêm An để kiểm phía học viên của lớp nháp sau khi kích hoạt.
    await invite(admin, basic03, "an.nguyen@gmail.com");

    await page.goto(`/admin/classes/${basic03}?tab=settings`);
    const select = page.getByRole("combobox", { name: "Phiên bản khóa học" });
    await select.click();
    const options = page.getByRole("option");
    await expect(options).toHaveText(["Lập trình cơ bản v2", "Lập trình cơ bản v1"]);
    await page.getByRole("option", { name: "Lập trình cơ bản v2" }).click();
    await page.getByRole("button", { name: "Lưu thay đổi" }).click();
    await expect(page.getByText("Đã lưu cài đặt lớp.")).toBeVisible();
    expect((await one<{ id: string }>("SELECT course_version_id AS id FROM classes WHERE id = $1", [basic03])).id).toBe(cv2);

    await page.getByRole("button", { name: "Kích hoạt lớp" }).click();
    await page.getByRole("dialog", { name: "Kích hoạt lớp basic03?" }).getByRole("button", { name: "Kích hoạt" }).click();
    await expect.poll(async () => (await one<{ status: string }>("SELECT status FROM classes WHERE id = $1", [basic03])).status).toBe(
      "active",
    );

    const learner = await browser.newContext({ storageState: storageStatePath("student") });
    const lp = await learner.newPage();
    await lp.goto(`/learn/classes/${basic01}`);
    const dbSection01 = lp.locator('[data-slot="card"]').filter({ has: lp.getByRole("heading", { name: "Database" }) });
    await expect(dbSection01).toContainText("3 học liệu");
    const roadmap01 = (await (await student.get(`/api/v1/me/classes/${basic01}`)).json()) as { percent: number };
    expect(roadmap01.percent).toBe(anPercentBefore);
    await expect(lp.getByText(`${String(anPercentBefore)}%`).first()).toBeVisible();

    await lp.goto(`/learn/classes/${basic03}`);
    const dbSection03 = lp.locator('[data-slot="card"]').filter({ has: lp.getByRole("heading", { name: "Database" }) });
    await expect(dbSection03).toContainText("4 học liệu");
    await expect(dbSection03.getByText("JOIN và subquery")).toBeVisible();
    await learner.close();

    const refused = await admin.patch(`/api/v1/classes/${basic01}`, { data: { courseVersionId: cv2 } });
    expect(refused.status()).toBe(409);
    const body = (await refused.json()) as { error: { code: string; message: string } };
    expect(body.error).toMatchObject({ code: "INVALID_TRANSITION", message: "Chỉ đổi phiên bản khi lớp còn nháp." });
  });
});
