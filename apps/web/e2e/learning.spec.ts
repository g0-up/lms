/*
 * Học viên học bài (E19–E22): mở, tích, lớp ended/draft, video ký URL, markdown đã lọc.
 * Ánh xạ check(...) của prototype/check-flows.mjs:
 *   'student roadmap renders with an unfinished lesson' → E19
 *   'opening lesson records firstOpenedAt'              → E19
 *   'tick records completedAt'                          → E19
 *   'lesson page shows done state'                      → E19
 *   'course % increases after tick'                     → E19
 *   'untick restores %'                                 → E19
 *   (lớp ended/draft, video, markdown: tiêu chí FR-11, FR-32, Q3, không có check riêng) → E20, E21, E22
 */
import { expect, request, test, type APIRequestContext, type Page } from "@playwright/test";
import { apiAs, storageStatePath } from "./support/auth";
import { closeDb, ids, one, query } from "./support/db";
import { PIXEL_PNG } from "./support/media";
import { activateClass, createClass, endClass, invite, seedReset } from "./support/seed";

const AN = "an.nguyen@gmail.com";
const NOT_OPENED = "Mở học liệu trước khi tích hoàn thành.";
const NOT_IN_COURSE = "Học liệu không thuộc phiên bản khóa học của lớp.";
const VIDEO_GAVE_UP = "Không phát được video. Tải lại trang để thử lại.";

interface ApiError {
  error: { code: string; message: string };
}
interface Roadmap {
  percent: number;
  requiredDone: number;
  readOnly: boolean;
  readOnlyReason?: string;
}
interface Completion {
  completedAt: string | null;
  percent: number;
}
interface VideoLesson {
  content: { type: string; mediaId: string; url: string; expiresAt: string };
}

function completion(student: APIRequestContext, classId: string, lessonId: string, completed: boolean) {
  return student.put(`/api/v1/me/classes/${classId}/lessons/${lessonId}/completion`, { data: { completed } });
}

function progressRow(classCode: string, lessonId: string) {
  return query<{ first_opened_at: Date; completed_at: Date | null }>(
    `SELECT p.first_opened_at, p.completed_at FROM lesson_progress p
       JOIN class_members m ON m.id = p.class_member_id
       JOIN classes c ON c.id = m.class_id
       JOIN users u ON u.id = m.user_id
      WHERE c.code = $1 AND u.email_normalized = $2 AND p.lesson_id = $3`,
    [classCode, AN, lessonId],
  );
}

/** Thanh tiến độ toàn khóa là một ảnh với tên "{pct}% hoàn thành"; thanh đầu tiên trên trang lộ trình. */
function coursePercent(page: Page, pct: number) {
  return page.getByRole("img", { name: `${String(pct)}% hoàn thành` }).first();
}

/** Thẻ lớp ở /learn: khối trong cùng chứa tiêu đề lớp và dòng đếm học liệu. */
function classCard(page: Page, name: string) {
  return page
    .locator("div")
    .filter({ has: page.getByRole("heading", { level: 2, name }) })
    .filter({ hasText: "học liệu bắt buộc" })
    .last();
}

/** Trạng thái phát của thẻ video duy nhất trên trang (null khi trình phát đang thay thẻ). */
function videoState(page: Page) {
  return page.evaluate(() => {
    const v = document.querySelector("video");
    return v ? { src: v.currentSrc, readyState: v.readyState, time: v.currentTime } : null;
  });
}

test.describe("Học viên học bài", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });
  test.use({ storageState: storageStatePath("student") });

  let admin: APIRequestContext;
  let student: APIRequestContext;
  let basic01: string;

  test.beforeAll(async () => {
    await seedReset();
    admin = await apiAs("admin");
    student = await apiAs("student");
    basic01 = await ids.classId("basic01");
  });

  test.afterAll(async () => {
    await admin.dispose();
    await student.dispose();
    await seedReset();
    await closeDb();
  });

  test("E19 mở, tích, bỏ tích học liệu; tích lặp lại không đổi mốc; chặn tích sai luật", async ({ page }) => {
    const lessonId = await ids.lessonInClass("basic01", "web-html");
    const before = (await (await student.get(`/api/v1/me/classes/${basic01}`)).json()) as Roadmap;
    expect(await progressRow("basic01", lessonId)).toHaveLength(0);

    await page.goto(`/learn/classes/${basic01}`);
    await expect(coursePercent(page, before.percent)).toBeVisible();
    const box = page.getByRole("checkbox", { name: "Đã học xong: HTML ngữ nghĩa" });
    await expect(box).toBeDisabled();
    await expect(page.locator("label", { has: box })).toHaveAttribute("title", "Mở học liệu trước khi tích");

    // API cũng từ chối tích học liệu chưa mở.
    const early = await completion(student, basic01, lessonId, true);
    expect(early.status()).toBe(409);
    expect(((await early.json()) as ApiError).error.message).toBe(NOT_OPENED);

    await page.getByRole("link", { name: /HTML ngữ nghĩa/ }).click();
    // Bài markdown mở đầu bằng chính tiêu đề của nó: chờ đủ cả tiêu đề trang lẫn nội dung.
    await expect(page.getByRole("heading", { level: 1, name: "HTML ngữ nghĩa" })).toHaveCount(2);
    await expect.poll(async () => (await progressRow("basic01", lessonId)).length).toBe(1);

    await page.getByRole("button", { name: "Đã học xong" }).click();
    await expect(page.getByText("Đã ghi nhận hoàn thành.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Bỏ tích" })).toBeVisible();
    await expect(page.getByText(/^Đã tích /)).toBeVisible();
    const done = (await progressRow("basic01", lessonId)).at(0);
    expect(done?.completed_at).toBeTruthy();

    // Tích lần hai cùng trạng thái: idempotent, giữ nguyên completed_at.
    const again = await completion(student, basic01, lessonId, true);
    expect(again.status()).toBe(200);
    const againBody = (await again.json()) as Completion;
    expect(Date.parse(againBody.completedAt ?? "")).toBe(done?.completed_at?.getTime());
    expect(againBody.percent).toBeGreaterThan(before.percent);

    await page.getByRole("link", { name: "basic01" }).click();
    await expect(coursePercent(page, againBody.percent)).toBeVisible();
    await expect(box).toBeChecked();
    await box.click();
    await expect(page.getByText("Đã bỏ tích.")).toBeVisible();
    await expect(coursePercent(page, before.percent)).toBeVisible();
    await expect(box).not.toBeChecked();
    const undone = (await progressRow("basic01", lessonId)).at(0);
    expect(undone?.completed_at).toBeNull();

    // Học liệu của phiên bản chặng khác (bản nháp nhân bản, không thuộc khóa học của lớp).
    expect((await admin.post(`/api/v1/stage-versions/${await ids.stageVersionId("WEB", 1)}/clone`)).status()).toBe(201);
    const foreign = (
      await one<{ id: string }>(
        "SELECT id FROM lessons WHERE stage_version_id = $1 AND lesson_key = 'web-html'",
        [await ids.stageVersionId("WEB", 2)],
      )
    ).id;
    const outside = await completion(student, basic01, foreign, true);
    expect(outside.status()).toBe(404);
    expect(((await outside.json()) as ApiError).error.message).toBe(NOT_IN_COURSE);
    // Trang học báo lỗi bằng toast rồi đưa về lộ trình của lớp.
    await page.goto(`/learn/classes/${basic01}/lessons/${foreign}`);
    await expect(page.getByText(NOT_IN_COURSE)).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/learn/classes/${basic01}$`));
  });

  test("E20 lớp ended chỉ xem, lớp draft chưa bắt đầu", async ({ page }) => {
    const ended = await createClass(admin, "e2e20");
    await invite(admin, ended.id, AN);
    await activateClass(admin, ended.id);
    const opened = await ids.lessonInClass("e2e20", "db-table");
    const unopened = await ids.lessonInClass("e2e20", "db-index");
    expect((await student.get(`/api/v1/me/classes/${ended.id}/lessons/${opened}`)).status()).toBe(200);
    await endClass(admin, ended.id);

    await page.goto(`/learn/classes/${ended.id}`);
    await expect(
      page.getByText("Lớp đã kết thúc: bạn vẫn xem được học liệu nhưng không tích hoàn thành được nữa."),
    ).toBeVisible();
    const box = page.getByRole("checkbox", { name: "Đã học xong: Đọc thêm: chỉ mục" });
    await expect(box).toBeDisabled();
    await expect(page.locator("label", { has: box })).toHaveAttribute("title", "Lớp đã kết thúc");

    // Nội dung vẫn mở được; nút tích chỉ giải thích lý do.
    await page.goto(`/learn/classes/${ended.id}/lessons/${opened}`);
    // Tiêu đề trang và tiêu đề đầu của bài markdown trùng nhau: thấy cả hai nghĩa là nội dung đã hiện.
    await expect(page.getByRole("heading", { level: 1, name: "Thiết kế bảng và khóa" })).toHaveCount(2);
    const tick = page.getByRole("button", { name: "Đã học xong" });
    await expect(tick).toHaveAttribute("aria-disabled", "true");
    await expect(tick).toHaveAttribute("title", "Lớp đã kết thúc");
    for (const lessonId of [opened, unopened]) {
      const refused = await completion(student, ended.id, lessonId, true);
      expect(refused.status()).toBe(409);
      expect(((await refused.json()) as ApiError).error.code).toBe("INVALID_TRANSITION");
    }

    // basic03 là lớp nháp của seed; An được mời vào để xem phía học viên.
    const basic03 = await ids.classId("basic03");
    await invite(admin, basic03, AN);
    const roadmapRes = await student.get(`/api/v1/me/classes/${basic03}`);
    expect(roadmapRes.status()).toBe(200);
    expect((await roadmapRes.json()) as Roadmap).toMatchObject({ readOnly: true, readOnlyReason: "draft" });
    const draftLesson = await student.get(`/api/v1/me/classes/${basic03}/lessons/${await ids.lessonInClass("basic03", "db-intro")}`);
    expect(draftLesson.status()).toBe(409);
    expect(((await draftLesson.json()) as ApiError).error.code).toBe("INVALID_TRANSITION");

    await page.goto(`/learn/classes/${basic03}`);
    await expect(page.getByText("Lớp chưa bắt đầu.", { exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: /Giới thiệu SQL/ })).toHaveCount(0);
    await page.goto("/learn");
    const card = classCard(page, "Lập trình cơ bản – khóa 3");
    await expect(card).toContainText("Bạn sẽ vào học được khi lớp kích hoạt.");
    await expect(card.getByRole("link", { name: "Học tiếp" })).toHaveCount(0);
    await expect(classCard(page, "Lập trình cơ bản – khóa 1").getByRole("link", { name: "Học tiếp" })).toBeVisible();
  });

  test.describe("E21 video ký URL", () => {
    let lessonId: string;
    let lessonPath: string;
    let storagePattern: string;
    let mediaId: string;

    test.beforeAll(async () => {
      lessonId = await ids.lessonInClass("basic01", "web-css");
      lessonPath = `/learn/classes/${basic01}/lessons/${lessonId}`;
      const lesson = (await (await student.get(`/api/v1/me/classes/${basic01}/lessons/${lessonId}`)).json()) as VideoLesson;
      expect(lesson.content.type).toBe("video");
      mediaId = lesson.content.mediaId;
      const signed = new URL(lesson.content.url);
      storagePattern = `${signed.origin}${signed.pathname.split("/").slice(0, 2).join("/")}/**`;
    });

    async function waitForVideo(page: Page) {
      await expect.poll(async () => (await videoState(page))?.readyState ?? 0, { timeout: 15_000 }).toBeGreaterThanOrEqual(1);
    }

    test("API ký URL ≥ 2 giờ, object không chữ ký bị từ chối", async () => {
      const res = await student.get(`/api/v1/media/${mediaId}/url`);
      expect(res.status()).toBe(200);
      const body = (await res.json()) as { url: string; expiresAt: string };
      expect(Object.keys(body).sort()).toEqual(["expiresAt", "url"]);
      const url = new URL(body.url);
      expect(Number(url.searchParams.get("X-Amz-Expires"))).toBeGreaterThanOrEqual(7200);
      expect(Date.parse(body.expiresAt) - Date.now()).toBeGreaterThan(7190_000);

      const raw = await request.newContext();
      expect((await raw.get(body.url, { headers: { Range: "bytes=0-15" } })).status()).toBeLessThan(300);
      expect((await raw.get(`${url.origin}${url.pathname}`)).status()).toBe(403);
      await raw.dispose();
    });

    test("storage trả 403 một lần: ký lại qua /url, phát và tua được", async ({ page }) => {
      await page.route(storagePattern, (route) => route.fulfill({ status: 403, body: "AccessDenied" }), { times: 1 });
      const resigned = page.waitForResponse((r) => r.url().endsWith(`/api/v1/media/${mediaId}/url`));
      await page.goto(lessonPath);
      expect((await resigned).status()).toBe(200);
      await waitForVideo(page);
      await page.evaluate(async () => {
        const v = document.querySelector("video");
        if (!v) throw new Error("không có video");
        const seeked = new Promise((resolve) => {
          v.addEventListener("seeked", resolve, { once: true });
        });
        v.currentTime = 1;
        await seeked;
      });
      expect((await videoState(page))?.time).toBeCloseTo(1, 0);
    });

    test("lỗi giữa chừng rồi lần tải lại cũng lỗi: vẫn khôi phục vị trí ±1 s", async ({ page }) => {
      await page.goto(lessonPath);
      await waitForVideo(page);
      await page.evaluate(async () => {
        const v = document.querySelector("video");
        if (!v) throw new Error("không có video");
        const seeked = new Promise((resolve) => {
          v.addEventListener("seeked", resolve, { once: true });
        });
        v.currentTime = 1.6;
        await seeked;
        // Đánh dấu thẻ cũ: URL ký lại trong cùng giây trùng hệt URL cũ nên so src không phân biệt được.
        Object.assign(v, { e2eOld: true });
      });

      let resigns = 0;
      page.on("response", (r) => {
        if (r.url().endsWith(`/api/v1/media/${mediaId}/url`)) resigns += 1;
      });
      await page.route(storagePattern, (route) => route.fulfill({ status: 403, body: "AccessDenied" }), { times: 1 });
      // URL hết hạn chỉ hiện ra như một MediaError không có mã HTTP: phát sự kiện error giống trình duyệt.
      await page.evaluate(() => document.querySelector("video")?.dispatchEvent(new Event("error")));

      await expect.poll(() => resigns, { timeout: 15_000 }).toBe(2);
      await waitForVideo(page);
      const state = await videoState(page);
      expect(await page.evaluate(() => "e2eOld" in (document.querySelector("video") ?? {}))).toBe(false);
      expect(state?.time ?? 0).toBeGreaterThanOrEqual(0.6);
      expect(state?.time ?? 0).toBeLessThanOrEqual(2.6);
      await expect(page.getByText(VIDEO_GAVE_UP)).toHaveCount(0);
    });

    test("storage luôn trả 403: ký lại tối đa 2 lần rồi báo lỗi", async ({ page }) => {
      let resigns = 0;
      page.on("response", (r) => {
        if (r.url().endsWith(`/api/v1/media/${mediaId}/url`)) resigns += 1;
      });
      await page.route(storagePattern, (route) => route.fulfill({ status: 403, body: "AccessDenied" }));
      await page.goto(lessonPath);
      await expect(page.getByRole("alert").filter({ hasText: VIDEO_GAVE_UP })).toBeVisible({ timeout: 15_000 });
      await expect(page.locator("video")).toHaveCount(0);
      expect(resigns).toBe(2);
    });
  });

  test("E22 markdown của học liệu được lọc, ảnh nội bộ tải qua media", async ({ page }) => {
    // Ảnh upload qua luồng media thật: xin URL upload, PUT lên storage, xác nhận.
    const ticketRes = await admin.post("/api/v1/media/uploads", {
      data: { kind: "image", fileName: "so-do-e22.png", contentType: "image/png", sizeBytes: PIXEL_PNG.length },
    });
    expect(ticketRes.status(), await ticketRes.text()).toBe(201);
    const ticket = (await ticketRes.json()) as { mediaId: string; uploadUrl: string };
    const storage = await request.newContext();
    const put = await storage.put(ticket.uploadUrl, { data: PIXEL_PNG, headers: { "Content-Type": "image/png" } });
    expect(put.status()).toBe(200);
    await storage.dispose();
    const done = await admin.post(`/api/v1/media/uploads/${ticket.mediaId}/complete`);
    expect(done.status(), await done.text()).toBe(200);
    const imageSrc = `/api/v1/media/${ticket.mediaId}/content`;

    expect((await admin.post("/api/v1/stages", { data: { code: "E2E22", name: "Chặng E22" } })).status()).toBe(201);
    const sv = await ids.stageVersionId("E2E22", 1);
    const markdownSource = [
      "# Nội dung E22",
      "",
      "<script>window.__e22 = 'script'</script>",
      "",
      '<img src="x" onerror="window.__e22 = \'onerror\'">',
      "",
      "[Liên kết độc](javascript:window.__e22='href')",
      "",
      "[Tài liệu ngoài](https://example.com/tai-lieu)",
      "",
      `![Sơ đồ E22](${imageSrc})`,
    ].join("\n");
    const lessonRes = await admin.post(`/api/v1/stage-versions/${sv}/lessons`, {
      data: { title: "Bài đọc E22", type: "markdown", required: true, markdownSource },
    });
    expect(lessonRes.status(), await lessonRes.text()).toBe(201);
    expect((await admin.post(`/api/v1/stage-versions/${sv}/publish`)).status()).toBe(200);
    expect((await admin.post("/api/v1/courses", { data: { code: "E2E22C", name: "Khóa học E22" } })).status()).toBe(201);
    const cv = await ids.courseVersionId("E2E22C", 1);
    expect((await admin.put(`/api/v1/course-versions/${cv}/stages`, { data: { stageVersionIds: [sv] } })).status()).toBe(200);
    expect((await admin.post(`/api/v1/course-versions/${cv}/publish`)).status()).toBe(200);
    const cls = await createClass(admin, "e2e22", cv);
    await invite(admin, cls.id, AN);
    await activateClass(admin, cls.id);
    const lessonId = (await one<{ id: string }>("SELECT id FROM lessons WHERE stage_version_id = $1", [sv])).id;

    const api = (await (await student.get(`/api/v1/me/classes/${cls.id}/lessons/${lessonId}`)).json()) as {
      content: { html: string };
    };
    expect(api.content.html).not.toMatch(/<script|onerror|javascript:/i);

    await page.goto(`/learn/classes/${cls.id}/lessons/${lessonId}`);
    const body = page.locator("div", { has: page.getByRole("heading", { name: "Nội dung E22" }) }).last();
    await expect(body).toBeVisible();
    await expect(body.locator("script")).toHaveCount(0);
    await expect(body.locator("[onerror]")).toHaveCount(0);
    await expect(body.locator('a[href^="javascript:" i]')).toHaveCount(0);
    const external = body.getByRole("link", { name: "Tài liệu ngoài" });
    await expect(external).toHaveAttribute("href", "https://example.com/tai-lieu");
    const rel = (await external.getAttribute("rel")) ?? "";
    expect(rel.split(/\s+/)).toEqual(expect.arrayContaining(["noopener", "noreferrer"]));
    const image = body.getByRole("img", { name: "Sơ đồ E22" });
    await expect(image).toHaveAttribute("src", imageSrc);
    await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
    expect(await page.evaluate(() => (window as { __e22?: unknown }).__e22)).toBeUndefined();

    const content = await student.get(imageSrc, { maxRedirects: 0 });
    expect(content.status()).toBe(302);
    expect(content.headers().location).toContain("X-Amz-Signature=");
  });
});
