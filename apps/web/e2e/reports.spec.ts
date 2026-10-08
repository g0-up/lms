/*
 * Báo cáo tiến độ lớp (E23–E25) trên dữ liệu seed, chỉ đọc.
 * Ánh xạ check(...) của prototype/check-flows.mjs:
 *   'report filter below 30% narrows rows'                → E23
 *   'report sort changes order'                           → E23
 *   'notlogged filter shows only never-logged students'   → E23
 *   'inactive>10 days filter'                             → E23 (bản thật dùng inactive=7 theo kế hoạch)
 *   'filter form writes query to hash'                    → E23 (search params thay cho hash)
 *   'teacher sees class report'                           → E25
 *   'draft class shows no inactivity warning'             → E25
 */
import { expect, test, type Page } from "@playwright/test";
import { apiAs, storageStatePath } from "./support/auth";
import { closeDb, ids } from "./support/db";

const AN = "Nguyễn Hoàng An";
const CUONG = "Lê Văn Cường";
const DUNG = "Phạm Minh Dũng";
const HA = "Hoàng Thu Hà";
const KHANG = "Vũ Đức Khang";
const PHONG = "Đặng Hải Phong";
const THAO = "Võ Phương Thảo";
const ACTIVE_BASIC01 = [AN, "Trần Thị Bích", CUONG, DUNG, HA, KHANG, PHONG];

/** Mỗi hàng báo cáo là một nút "Xem chi tiết {tên}"; trả tên theo thứ tự hiển thị. */
function rows(page: Page) {
  return page.getByRole("button", { name: /^Xem chi tiết / });
}

async function rowNames(page: Page): Promise<string[]> {
  await expect(page.locator("table[aria-busy]")).toHaveCount(0);
  const labels = await rows(page).evaluateAll((els) => els.map((el) => el.getAttribute("aria-label") ?? ""));
  return labels.map((label) => label.replace(/^Xem chi tiết /, ""));
}

async function expectRows(page: Page, names: string[]) {
  await expect(rows(page)).toHaveCount(names.length);
  expect((await rowNames(page)).sort()).toEqual([...names].sort());
}

test.afterAll(async () => {
  await closeDb();
});

test.describe("Báo cáo lớp của admin", () => {
  test.use({ storageState: storageStatePath("admin") });

  let reportPath: string;
  test.beforeAll(async () => {
    reportPath = `/admin/classes/${await ids.classId("basic01")}?tab=report`;
  });

  test("E23 lọc, sắp xếp, học viên đã rời lớp, form ghi search params", async ({ page }) => {
    const selfReported = page.getByText("Tiến độ do học viên tự xác nhận", { exact: true });

    await page.goto(reportPath);
    await expectRows(page, ACTIVE_BASIC01);
    await expect(selfReported).toBeVisible();
    await expect(page.getByText(`${String(ACTIVE_BASIC01.length)}/${String(ACTIVE_BASIC01.length)} học viên`)).toBeVisible();

    await page.goto(`${reportPath}&below=30`);
    await expectRows(page, [DUNG, HA, KHANG, PHONG]);
    await expect(selfReported).toBeVisible();

    await page.goto(`${reportPath}&sort=pct-desc`);
    await expect(rows(page)).toHaveCount(ACTIVE_BASIC01.length);
    const desc = await rowNames(page);
    expect(desc[0]).toBe(CUONG);
    await page.goto(`${reportPath}&sort=pct`);
    await expect(rows(page)).toHaveCount(ACTIVE_BASIC01.length);
    const asc = await rowNames(page);
    expect(asc[0]).toBe(DUNG);
    // Hai chiều đảo nhau; học viên bằng % (Hà, Phong cùng 29%) giữ thứ tự tên ở cả hai chiều.
    expect(asc.at(-1)).toBe(CUONG);
    expect(desc.at(-1)).toBe(DUNG);

    // Bấm tiêu đề cột đổi chiều sắp xếp và ghi vào URL.
    await page.getByRole("button", { name: "Toàn khóa" }).click();
    await expect(page).toHaveURL(/[?&]sort=pct-desc/);
    await expect(page.getByRole("columnheader", { name: "Toàn khóa" })).toHaveAttribute("aria-sort", "descending");
    expect((await rowNames(page))[0]).toBe(CUONG);

    await page.goto(`${reportPath}&notlogged=1`);
    await expectRows(page, [DUNG]);

    await page.goto(`${reportPath}&inactive=7`);
    await expectRows(page, [DUNG, KHANG]);

    await page.goto(reportPath);
    await expect(page.getByText(THAO)).toHaveCount(0);
    await page.getByLabel("Hiện học viên đã rời lớp").check();
    await page.getByRole("button", { name: "Lọc", exact: true }).click();
    await expect(page).toHaveURL(/[?&]dropped=1/);
    await expectRows(page, [...ACTIVE_BASIC01, THAO]);
    await expect(page.getByRole("button", { name: `Xem chi tiết ${THAO}` })).toContainText("Đã rời lớp");
    // Học viên đã rời lớp hiện ra nhưng không tính vào tổng.
    await expect(page.getByText(`${String(ACTIVE_BASIC01.length)}/${String(ACTIVE_BASIC01.length)} học viên`)).toBeVisible();

    await page.goto(reportPath);
    await page.getByLabel("% toàn khóa dưới").fill("50");
    await page.getByRole("button", { name: "Lọc", exact: true }).click();
    await expect(page).toHaveURL(/[?&]tab=report/);
    await expect(page).toHaveURL(/[?&]below=50/);
    await expectRows(page, [DUNG, HA, KHANG, PHONG]);
    await page.getByRole("link", { name: "Xóa bộ lọc" }).first().click();
    await expect(page).not.toHaveURL(/below=/);
    await expectRows(page, ACTIVE_BASIC01);
    await expect(page.getByLabel("% toàn khóa dưới")).toHaveValue("");
    await expect(selfReported).toBeVisible();
  });

  test("E24 drilldown một học viên trong sheet", async ({ page }) => {
    await page.goto(reportPath);
    const row = page.getByRole("button", { name: `Xem chi tiết ${AN}` });
    await row.click();
    const sheet = page.getByRole("dialog", { name: "Chi tiết học viên" });
    await expect(sheet).toBeVisible();
    await expect(sheet.getByRole("heading", { name: AN })).toBeVisible();
    await expect(sheet).toContainText("an.nguyen@gmail.com · basic01 · hoạt động cuối");
    await expect(sheet.getByRole("img", { name: "71% hoàn thành" }).first()).toBeVisible();
    await expect(sheet.getByRole("heading", { level: 2, name: /^1\. Database/ })).toBeVisible();
    await expect(sheet.getByRole("heading", { level: 2, name: /^5\. / })).toBeVisible();
    await expect(sheet.getByText(/^Mở lần đầu /).first()).toBeVisible();
    await expect(sheet.getByText("Chưa mở").first()).toBeVisible();
    await expect(sheet.getByText(/^Tích /).first()).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(sheet).toBeHidden();
    await expect(row).toBeFocused();
  });
});

test.describe("Phạm vi giảng viên", () => {
  test.use({ storageState: storageStatePath("teacher") });

  test("E25 giảng viên chỉ thấy lớp mình, lớp nháp không có cảnh báo không hoạt động", async ({ page }) => {
    const basic01 = await ids.classId("basic01");
    const basic02 = await ids.classId("basic02");

    await page.goto("/teach");
    const card01 = page.getByRole("link", { name: /basic01/ });
    const card03 = page.getByRole("link", { name: /basic03/ });
    await expect(card01).toBeVisible();
    await expect(card03).toBeVisible();
    await expect(page.getByRole("link", { name: /basic02/ })).toHaveCount(0);
    await expect(card01).toContainText(/[1-9]\d* lâu không hoạt động/);
    await expect(card03).not.toContainText("lâu không hoạt động");

    await card01.click();
    await expect(page).toHaveURL(new RegExp(`/teach/classes/${basic01}$`));
    await expect(page.getByRole("table", { name: "Tiến độ học viên lớp basic01" })).toBeVisible();
    await expect(rows(page)).toHaveCount(ACTIVE_BASIC01.length);
    await expect(page.getByRole("img", { name: /% hoàn thành$/ }).first()).toBeVisible();

    // Lớp của giảng viên khác: API 403, trang báo lỗi rồi về /teach.
    const teacher = await apiAs("teacher");
    expect((await teacher.get(`/api/v1/classes/${basic02}/report`)).status()).toBe(403);
    expect((await teacher.get(`/api/v1/classes/${basic02}`)).status()).toBe(403);
    await teacher.dispose();
    await page.goto(`/teach/classes/${basic02}`);
    await expect(page.getByText("Bạn chỉ xem được lớp mình phụ trách.")).toBeVisible();
    await expect(page).toHaveURL(/\/teach$/);
  });
});
