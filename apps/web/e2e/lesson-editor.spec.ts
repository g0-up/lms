/*
 * Trang soạn học liệu markdown (E33–E42) trên stack thật: những gì jsdom không kiểm được (gõ, phím tắt, dán,
 * kéo thả, upload ảnh qua storage, chặn rời trang, round-trip nội dung seed, axe).
 * Tạo chặng E2EMD (bản nháp v1), nhân bản và phát hành chặng seed, nên chạy @serial và seed lại ở afterAll.
 */
import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test";
import { runAxe, smallHitAreas } from "./support/a11y";
import { apiAs, storageStatePath } from "./support/auth";
import { closeDb, count, ids, one, query } from "./support/db";
import { PIXEL_PNG } from "./support/media";
import { seedReset } from "./support/seed";

const EXTERNAL_IMAGE = "Ảnh trong markdown phải tải lên qua hệ thống.";
/** Ảnh markdown trỏ tới media đã tải lên, với alt cho trước; nhóm 1 là id media. */
const mediaImage = (alt: string) => new RegExp(`!\\[${alt}\\]\\(/api/v1/media/([0-9a-f-]{36})/content\\)`);

/** Trang soạn của một bài markdown đã có. */
const editorPath = (sid: string, vid: string, lessonId: string) => `/admin/stages/${sid}/versions/${vid}/lessons/${lessonId}/edit`;

interface ApiError {
  error: { code: string; message: string };
}

function content(page: Page): Locator {
  return page.getByRole("textbox", { name: "Nội dung bài học" });
}

function toolbar(page: Page): Locator {
  return page.getByRole("toolbar");
}

function saveButton(page: Page): Locator {
  return page.getByRole("button", { name: "Lưu", exact: true });
}

function sourceOf(lessonId: string): Promise<string> {
  return one<{ markdown_source: string }>("SELECT markdown_source FROM lessons WHERE id = $1", [lessonId]).then(
    (row) => row.markdown_source,
  );
}

/**
 * Thêm một đoạn mới ngay sau tiêu đề đầu bài. Không dùng cuối nội dung: Ctrl+End có thể rơi vào ô bảng
 * (editor lồng); cũng không dùng sau đoạn kết thúc bằng chữ đậm vì đoạn mới thừa hưởng định dạng.
 */
async function newParagraph(page: Page) {
  await content(page).locator("h1").first().click();
  await page.keyboard.press("End");
  await page.keyboard.press("Enter");
}

/** Dán vào nội dung bằng ClipboardEvent thật (Lexical nghe `paste` trên contenteditable). */
async function pasteHtml(page: Page, html: string) {
  await content(page).evaluate((el, value) => {
    const data = new DataTransfer();
    data.setData("text/html", value);
    data.setData("text/plain", value.replace(/<[^>]+>/g, ""));
    el.dispatchEvent(new ClipboardEvent("paste", { clipboardData: data, bubbles: true, cancelable: true }));
  }, html);
}

async function pastePng(page: Page, fileName: string) {
  await content(page).evaluate(
    (el, { base64, name }) => {
      const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
      const data = new DataTransfer();
      data.items.add(new File([bytes], name, { type: "image/png" }));
      el.dispatchEvent(new ClipboardEvent("paste", { clipboardData: data, bubbles: true, cancelable: true }));
    },
    { base64: PIXEL_PNG.toString("base64"), name: fileName },
  );
}

async function expectImageLoaded(image: Locator) {
  await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
}

test.describe("Trang soạn học liệu markdown", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });
  test.use({ storageState: storageStatePath("admin") });

  let admin: APIRequestContext;
  let stageId: string;
  let draftId: string;
  /** Bài markdown E33 tạo; các kịch bản sau sửa tiếp bài này. */
  let lessonId: string;

  test.beforeAll(async () => {
    await seedReset();
    admin = await apiAs("admin");
    const res = await admin.post("/api/v1/stages", { data: { code: "E2EMD", name: "Chặng soạn markdown" } });
    expect(res.status(), await res.text()).toBe(201);
    stageId = await ids.stageId("E2EMD");
    draftId = await ids.stageVersionId("E2EMD", 1);
  });

  test.afterAll(async () => {
    await admin.dispose();
    await seedReset();
    await closeDb();
  });

  async function openEditor(page: Page, path = editorPath(stageId, draftId, lessonId)) {
    await page.goto(path);
    await expect(content(page)).toBeVisible();
  }

  /** Lưu rồi chờ quay về trang chặng. */
  async function save(page: Page, toast: string) {
    await saveButton(page).click();
    await expect(page.getByText(toast)).toBeVisible();
    await expect(page).toHaveURL(/\/admin\/stages\/[0-9a-f-]{36}\?v=[0-9a-f-]{36}$/);
  }

  test("E33 thêm bài markdown qua trang soạn", async ({ page }) => {
    await page.goto(`/admin/stages/${stageId}?v=${draftId}`);
    await page.getByRole("button", { name: "Thêm học liệu" }).click();
    const dialog = page.getByRole("dialog", { name: "Thêm học liệu" });
    await dialog.getByLabel("Tiêu đề").fill("Bài đọc E33");
    await dialog.getByRole("combobox", { name: "Loại" }).click();
    await page.getByRole("option", { name: "Markdown" }).click();
    await dialog.getByRole("button", { name: "Tiếp tục soạn" }).click();

    await expect(page).toHaveURL(new RegExp(`/versions/${draftId}/lessons/new$`));
    await expect(page.getByRole("heading", { level: 1, name: "Thêm học liệu markdown" })).toBeVisible();
    await expect(page.getByLabel(/^Tiêu đề/)).toHaveValue("Bài đọc E33");

    await content(page).click();
    await page.keyboard.type("# Tiêu đề");
    await page.keyboard.press("Enter");
    await page.keyboard.type("Đoạn có chữ ");
    await toolbar(page).getByLabel("Đậm", { exact: true }).click();
    await page.keyboard.type("đậm");
    await toolbar(page).getByLabel("Bỏ đậm", { exact: true }).click();
    await page.keyboard.press("Enter");
    await toolbar(page).getByLabel("Danh sách dấu chấm", { exact: true }).click();
    await page.keyboard.type("mục một");
    await page.keyboard.press("Enter");
    await page.keyboard.press("Enter");

    await page.keyboard.type("Xem ");
    await toolbar(page).getByLabel("Chèn liên kết", { exact: true }).click();
    // MDXEditor không gắn label "Địa chỉ" với ô nhập (input thiếu id="link-url"), nên tìm theo name.
    await page.locator('input[name="url"]').fill("https://example.com");
    await page.getByLabel("Chữ hiển thị").fill("ví dụ");
    await page.getByRole("button", { name: "Lưu liên kết" }).click();
    await expect(content(page).getByRole("link", { name: "ví dụ" })).toBeVisible();

    await newParagraph(page);
    await toolbar(page).getByLabel("Chèn khối mã", { exact: true }).click();
    await content(page).getByRole("combobox", { name: "Ngôn ngữ", exact: true }).click();
    await page.getByRole("option", { name: "Go", exact: true }).click();
    await content(page).locator(".cm-content").click();
    await page.keyboard.type("fmt.Println(1)");

    await newParagraph(page);
    await toolbar(page).getByLabel("Chèn bảng", { exact: true }).click();
    await expect(content(page).locator("table")).toBeVisible();

    await save(page, "Đã thêm học liệu.");
    const row = await one<{ id: string; markdown_source: string; required: boolean }>(
      "SELECT id, markdown_source, required FROM lessons WHERE stage_version_id = $1 AND title = 'Bài đọc E33'",
      [draftId],
    );
    lessonId = row.id;
    expect(row.required).toBe(true);
    expect(row.markdown_source).toContain("# Tiêu đề");
    expect(row.markdown_source).toContain("**đậm**");
    expect(row.markdown_source).toMatch(/^[*-] mục một$/m);
    expect(row.markdown_source).toContain("[ví dụ](https://example.com)");
    expect(row.markdown_source).toContain("```go\nfmt.Println(1)\n```");
    expect(row.markdown_source).toContain("|");
  });

  test("E34 chặn định dạng ngoài GFM khi gõ và dán", async ({ page }) => {
    await openEditor(page);
    await newParagraph(page);
    // Đối chứng dương: phím tắt tới được editor. Bật/tắt định dạng trên con trỏ rồi gõ, vì Lexical giữ
    // anchor cũ khi Shift+← ngay sau khi gõ nhanh nên bôi đen lệch.
    await page.keyboard.type("plain ");
    await page.keyboard.press("ControlOrMeta+B");
    await page.keyboard.type("bold");
    await page.keyboard.press("ControlOrMeta+B");
    await page.keyboard.type(" ");
    await page.keyboard.press("ControlOrMeta+U");
    await page.keyboard.type("under");
    await page.keyboard.press("ControlOrMeta+U");
    await page.keyboard.press("Enter");

    await pasteHtml(
      page,
      '<u>a</u><sup>2</sup><mark>m</mark><span style="color:red">s</span><h5>h</h5>' +
        '<a href="javascript:alert(1)">x</a><img src="https://evil.example/x.png">',
    );
    await save(page, "Đã lưu học liệu.");

    const source = await sourceOf(lessonId);
    expect(source).toContain("plain **bold** under");
    for (const pasted of ["a", "2", "m", "s", "x"]) expect(source).toContain(pasted);
    expect(source).toMatch(/^#### h$/m);
    for (const refused of ["<u>", "<sup>", "==", "style", "javascript:", "evil.example", "#####"]) {
      expect(source).not.toContain(refused);
    }
  });

  test("E35 chèn ảnh qua dialog bắt buộc alt", async ({ page }) => {
    await openEditor(page);
    await newParagraph(page);
    await toolbar(page).getByLabel("Chèn ảnh", { exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "Chèn ảnh" });
    await dialog.getByLabel(/File ảnh/).setInputFiles({ name: "so-do-lop.png", mimeType: "image/png", buffer: PIXEL_PNG });
    const alt = dialog.getByLabel(/Mô tả ảnh/);
    await expect(alt).toHaveValue("so-do-lop");
    await alt.fill("");
    await dialog.getByRole("button", { name: "Chèn ảnh" }).click();
    await expect(dialog.getByText("Nhập mô tả ảnh.")).toBeVisible();
    await alt.fill("Sơ đồ lớp");
    await dialog.getByRole("button", { name: "Chèn ảnh" }).click();
    await expect(dialog).toBeHidden({ timeout: 30_000 });

    const image = content(page).getByRole("img", { name: "Sơ đồ lớp" });
    await expectImageLoaded(image);
    await save(page, "Đã lưu học liệu.");

    const source = await sourceOf(lessonId);
    const match = mediaImage("Sơ đồ lớp").exec(source);
    expect(match, source).not.toBeNull();
    expect(
      await count("SELECT count(*) AS n FROM media_files WHERE id = $1 AND kind = 'image' AND status = 'ready'", [
        match?.[1],
      ]),
    ).toBe(1);
  });

  test("E36 dán ảnh lấy alt theo tên file; kéo thả URL ngoài không chèn ảnh", async ({ page, browserName }) => {
    test.skip(browserName === "webkit", "ClipboardEvent/DragEvent tổng hợp chỉ kiểm trên chromium");
    await openEditor(page);
    await newParagraph(page);
    await pastePng(page, "so-do-dan.png");
    await expectImageLoaded(content(page).getByRole("img", { name: "so-do-dan" }));

    const images = await content(page).getByRole("img").count();
    // Sự kiện tổng hợp không kích hoạt hành động mặc định của trình duyệt; kiểm phần editor tự xử lý.
    await content(page).evaluate((el) => {
      const data = new DataTransfer();
      data.setData("text/uri-list", "https://evil.example/drop.png");
      data.setData("text/plain", "https://evil.example/drop.png");
      el.dispatchEvent(new DragEvent("drop", { dataTransfer: data, bubbles: true, cancelable: true }));
    });
    await expect(content(page).getByRole("img")).toHaveCount(images);

    await save(page, "Đã lưu học liệu.");
    const source = await sourceOf(lessonId);
    expect(source).toMatch(mediaImage("so-do-dan"));
    expect(source).not.toContain("evil.example");
  });

  test("E36b nút Lưu khóa khi ảnh dán còn đang tải", async ({ page, browserName }) => {
    test.skip(browserName === "webkit", "ClipboardEvent tổng hợp chỉ kiểm trên chromium");
    let release!: () => void;
    const released = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.route("http://localhost:9000/**", async (route) => {
      if (route.request().method() === "PUT") await released;
      await route.continue();
    });
    await openEditor(page);
    await newParagraph(page);
    await pastePng(page, "dang-tai.png");

    await expect(page.getByRole("button", { name: "Đang tải ảnh…" })).toBeDisabled();
    release();
    await expect(saveButton(page)).toBeEnabled({ timeout: 30_000 });
    await expectImageLoaded(content(page).getByRole("img", { name: "dang-tai" }));
    await save(page, "Đã lưu học liệu.");
    expect(await sourceOf(lessonId)).toMatch(mediaImage("dang-tai"));
  });

  test("E37 xem trước bằng HTML của server; quay lại soạn còn nguyên và hoàn tác được", async ({ page }) => {
    await openEditor(page);
    await page.getByRole("tab", { name: "Xem trước" }).click();
    const preview = page.getByRole("tabpanel", { name: "Xem trước" });
    await expect(preview.getByRole("heading", { level: 1, name: "Tiêu đề" })).toBeVisible();
    await expect(preview.locator("table")).toBeVisible();
    await expect(preview.locator("pre code.language-go")).toContainText("fmt.Println(1)");
    const link = preview.getByRole("link", { name: "ví dụ" });
    // Xem trước dùng cùng bộ lọc với trang học: link mở tab mới mang rel="noopener noreferrer".
    await expect(link).toHaveAttribute("target", "_blank");
    await expect(link).toHaveAttribute("rel", "noopener noreferrer");
    await expectImageLoaded(preview.getByRole("img", { name: "Sơ đồ lớp" }));
    // Stack E2E không dùng CSP production: ảnh trên staging/homelab kiểm tay sau khi đổi img-src.

    await page.getByRole("tab", { name: "Soạn thảo" }).click();
    await expect(content(page).getByRole("heading", { level: 1, name: "Tiêu đề" })).toBeVisible();
    await newParagraph(page);
    await page.keyboard.type("hoàn tác");
    await expect(content(page)).toContainText("hoàn tác");
    await toolbar(page).getByLabel(/^Hoàn tác/).click();
    await expect(content(page)).not.toContainText("hoàn tác");
  });

  test("E38 ảnh ngoài bị chặn ở server; giảng viên không gọi được xem trước", async ({ page }) => {
    const patch = await admin.patch(`/api/v1/stage-versions/${draftId}/lessons/${lessonId}`, {
      data: { markdownSource: "![a](https://x.example/a.png)" },
    });
    expect(patch.status()).toBe(422);
    expect(((await patch.json()) as ApiError).error.message).toBe(EXTERNAL_IMAGE);

    // Chế độ Mã nguồn không qua guard của editor: server là lớp chặn cuối, trang giữ nguyên công soạn.
    const before = await sourceOf(lessonId);
    await openEditor(page);
    await toolbar(page).getByRole("radio", { name: "Mã nguồn" }).click();
    const source = page.locator(".cm-content").last();
    await source.click();
    await page.keyboard.press("ControlOrMeta+End");
    await page.keyboard.type("\n\n![ngoài](https://x.example/a.png)");
    await saveButton(page).click();
    await expect(page.getByRole("alert").filter({ hasText: EXTERNAL_IMAGE })).toBeVisible();
    await expect(page).toHaveURL(/\/lessons\/[0-9a-f-]{36}\/edit$/);
    expect(await sourceOf(lessonId)).toBe(before);

    const teacher = await apiAs("teacher");
    const preview = await teacher.post("/api/v1/stages/markdown-preview", { data: { markdownSource: "# a" } });
    expect(preview.status()).toBe(403);
    await teacher.dispose();
  });

  test("E39 hỏi trước khi rời trang còn thay đổi chưa lưu", async ({ page }) => {
    await openEditor(page);
    await newParagraph(page);
    await page.keyboard.type("chưa lưu");
    const editUrl = page.url();

    await page.getByRole("navigation", { name: "Đường dẫn" }).getByRole("link", { name: "Chặng" }).click();
    const confirm = page.getByRole("dialog", { name: "Rời trang?" });
    await expect(confirm).toBeVisible();
    await confirm.getByRole("button", { name: "Ở lại" }).click();
    await expect(confirm).toBeHidden();
    expect(page.url()).toBe(editUrl);
    await expect(content(page)).toContainText("chưa lưu");

    let seen = "";
    page.once("dialog", (dialog) => {
      seen = dialog.type();
      void dialog.dismiss();
    });
    // Hủy beforeunload nên reload không bao giờ xong; bỏ qua promise của nó.
    const reload = page.reload().catch(() => undefined);
    await expect.poll(() => seen).toBe("beforeunload");
    expect(page.url()).toBe(editUrl);
    await expect(content(page)).toContainText("chưa lưu");
    void reload;

    await page.getByRole("navigation", { name: "Đường dẫn" }).getByRole("link", { name: "Chặng" }).click();
    await page.getByRole("dialog", { name: "Rời trang?" }).getByRole("button", { name: "Rời trang" }).click();
    await expect(page).toHaveURL(/\/admin\/stages$/);
    expect(await sourceOf(lessonId)).not.toContain("chưa lưu");
  });

  test("E40 nội dung seed giữ nguyên byte khi chỉ sửa tiêu đề; chuẩn hoá của editor không đổi HTML", async ({ page }) => {
    test.setTimeout(180_000);
    const seeded = await query<{ code: string; stage_id: string; v1: string }>(
      `SELECT DISTINCT s.code, s.id AS stage_id, sv.id AS v1 FROM lessons l
         JOIN stage_versions sv ON sv.id = l.stage_version_id JOIN stages s ON s.id = sv.stage_id
        WHERE l.type = 'markdown' AND sv.status = 'published' AND s.code <> 'E2EMD' ORDER BY s.code`,
    );
    expect(seeded.map((s) => s.code)).toEqual(["DB", "DS", "GO", "RE", "WEB"]);

    type Lesson = { id: string; lesson_key: string; markdown_source: string; markdown_html: string | null };
    const lessonsOf = (vid: string) =>
      query<Lesson>(
        "SELECT id, lesson_key, markdown_source, markdown_html FROM lessons WHERE stage_version_id = $1 AND type = 'markdown' ORDER BY position",
        [vid],
      );
    const drafts: { stageId: string; v1: string; v2: string }[] = [];
    for (const stage of seeded) {
      const res = await admin.post(`/api/v1/stage-versions/${stage.v1}/clone`);
      expect(res.status(), await res.text()).toBe(201);
      drafts.push({ stageId: stage.stage_id, v1: stage.v1, v2: ((await res.json()) as { id: string }).id });
    }

    let opened = 0;
    for (const { stageId: sid, v2 } of drafts) {
      for (const lesson of await lessonsOf(v2)) {
        await page.goto(editorPath(sid, v2, lesson.id));
        await expect(content(page)).toBeVisible();
        const title = page.getByLabel(/^Tiêu đề/);
        await title.press("End");
        await title.press("x");
        await title.press("Backspace");
        await save(page, "Đã lưu học liệu.");
        opened += 1;
      }
    }
    expect(opened).toBe(7);
    for (const { v1, v2 } of drafts) {
      const [before, after] = await Promise.all([lessonsOf(v1), lessonsOf(v2)]);
      expect(after.map((l) => [l.lesson_key, l.markdown_source])).toEqual(before.map((l) => [l.lesson_key, l.markdown_source]));
    }

    // Một bài sửa nội dung thật (thêm rồi xóa một khoảng trắng): source chuẩn hoá phải render ra cùng HTML.
    const touched = drafts[0];
    const [first] = await lessonsOf(touched.v2);
    await page.goto(editorPath(touched.stageId, touched.v2, first.id));
    await content(page).click();
    await page.keyboard.press("ControlOrMeta+Home");
    await page.keyboard.press("End");
    await page.keyboard.type(" ");
    await page.keyboard.press("Backspace");
    await save(page, "Đã lưu học liệu.");
    const normalized = await sourceOf(first.id);
    const render = async (markdownSource: string) => {
      const res = await admin.post("/api/v1/stages/markdown-preview", { data: { markdownSource } });
      expect(res.status(), await res.text()).toBe(200);
      return ((await res.json()) as { html: string }).html;
    };
    expect(await render(normalized)).toBe(await render(first.markdown_source));

    for (const { v1, v2 } of drafts) {
      const res = await admin.post(`/api/v1/stage-versions/${v2}/publish`);
      expect(res.status(), await res.text()).toBe(200);
      const [before, after] = await Promise.all([lessonsOf(v1), lessonsOf(v2)]);
      expect(after.map((l) => [l.lesson_key, l.markdown_html])).toEqual(before.map((l) => [l.lesson_key, l.markdown_html]));
    }
  });

  test("E41 phiên bản đã phát hành mở ở chế độ chỉ đọc", async ({ page }) => {
    const db = await ids.stageId("DB");
    const v1 = await ids.stageVersionId("DB", 1);
    const lesson = await one<{ id: string }>(
      "SELECT id FROM lessons WHERE stage_version_id = $1 AND lesson_key = 'db-table'",
      [v1],
    );
    await page.goto(editorPath(db, v1, lesson.id));
    await expect(
      page.getByText("Phiên bản đã phát hành hoặc lưu trữ, không thể sửa. Nhân bản thành bản nháp để chỉnh."),
    ).toBeVisible();
    await expect(content(page)).toHaveCount(0);
    await expect(saveButton(page)).toHaveCount(0);
  });

  test("E42 trang soạn học liệu không có vi phạm axe serious/critical", async ({ page }) => {
    await openEditor(page);
    await expect(toolbar(page)).toBeVisible();
    const { violations, inputBorder } = await runAxe(page);
    expect(violations, JSON.stringify(violations)).toEqual([]);
    for (const finding of inputBorder) {
      test.info().annotations.push({ type: "axe-input-border", description: `${finding.id} × ${String(finding.targets.length)}` });
    }

    // Toolbar Radix: phím mũi tên chuyển focus giữa các nút (nút bị tắt như "Làm lại" khi chưa có lịch sử thì bỏ qua).
    await toolbar(page).getByLabel("Đậm", { exact: true }).focus();
    await page.keyboard.press("ArrowRight");
    await expect(toolbar(page).getByLabel("Nghiêng", { exact: true })).toBeFocused();

    // Thứ tự Tab: Lưu → tiêu đề → bắt buộc → tab list → toolbar → nội dung.
    await saveButton(page).focus();
    const order: string[] = [];
    for (let i = 0; i < 12; i += 1) {
      await page.keyboard.press("Tab");
      order.push(
        await page.evaluate(() => {
          const el = document.activeElement;
          if (!el) return "";
          if (el.closest('[role="toolbar"]')) return "toolbar";
          if (el.getAttribute("contenteditable") === "true") return "content";
          return el.getAttribute("role") ?? el.tagName.toLowerCase();
        }),
      );
    }
    const firsts = ["input", "checkbox", "tab", "toolbar", "content"].map((stop) => order.indexOf(stop));
    expect(firsts, order.join(" → ")).toEqual([...firsts].sort((a, b) => a - b));
    expect(firsts.every((i) => i >= 0), order.join(" → ")).toBe(true);

    await page.setViewportSize({ width: 375, height: 812 });
    await expect(toolbar(page)).toBeVisible();
    expect(await smallHitAreas(page)).toEqual([]);
  });
});
