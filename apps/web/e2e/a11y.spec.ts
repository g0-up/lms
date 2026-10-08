/*
 * Khả năng truy cập, reflow và bàn phím (E26–E32), project "a11y".
 * Ánh xạ check(...) của prototype/check-flows.mjs:
 *   '320px reflow <route>' (10 route)                       → E26
 *   'reduced motion removes transitions'                    → E27
 *   'route change is announced once'                        → E28
 *   'class tabs are a labelled nav, not an ARIA tablist'    → E29
 *   'modal has a resolvable accessible name'                → E30
 *   'Escape closes the modal'                               → E30
 *   'toast has a focusable close button'                    → E31
 *   'close button removes the toast'                        → E31
 *   (skip link, button-name, axe serious/critical trên 10 route: review.md) → E32
 *   (hit area ≥ 44px ở 375px, ≤ 1 nút gradient mỗi route: phase-14 mục token audit) → E26
 */
import { expect, test, type Browser, type Page, type ViewportSize } from "@playwright/test";
import { checkFlowRoutes, runAxe, smallHitAreas, waitForPage, type AppRoute } from "./support/a11y";
import { storageStatePath } from "./support/auth";
import { closeDb, ids } from "./support/db";
import { seedReset } from "./support/seed";

const TOAST_DURATION_MS = 4000;
const VISIBLE_TOASTS = 3;

let routes: AppRoute[];
let stagePath: string;

test.beforeAll(async () => {
  routes = await checkFlowRoutes();
  stagePath = `/admin/stages/${await ids.stageId("DB")}`;
});

test.afterAll(async () => {
  await closeDb();
});

/** Mở route trong context riêng mang phiên của vai trò tương ứng (login: không phiên). */
async function openRoute(browser: Browser, route: AppRoute, viewport?: ViewportSize) {
  const context = await browser.newContext({
    storageState: route.role ? storageStatePath(route.role) : undefined,
    ...(viewport ? { viewport } : {}),
  });
  const page = await context.newPage();
  await page.goto(route.path);
  await waitForPage(page);
  return { page, close: () => context.close() };
}

/** Phần tử đang focus nằm trong dialog đang mở. */
function focusInsideDialog(page: Page): Promise<boolean> {
  return page.evaluate(() => !!document.activeElement?.closest('[role="dialog"]'));
}

/** Toast của sonner (mỗi toast là một `li[data-sonner-toast]`; toast vượt hạn mức vẫn trong DOM với `data-visible="false"`). */
function toasts(page: Page) {
  return page.locator("[data-sonner-toast]");
}

function visibleToasts(page: Page) {
  return page.locator('[data-sonner-toast][data-visible="true"]');
}

/** Số nút/link đang hiển thị mang gradient chính (Button không có data-variant: đọc computed style). */
function gradientCount(page: Page): Promise<number> {
  return page.evaluate(
    () =>
      [...document.querySelectorAll("button, a")].filter(
        (el) => el.checkVisibility() && getComputedStyle(el).backgroundImage.includes("linear-gradient(260deg"),
      ).length,
  );
}

test.describe("Reflow và axe trên 10 route", () => {
  test("E26 reflow 320px không cuộn ngang", async ({ browser }) => {
    for (const route of routes) {
      const { page, close } = await openRoute(browser, route, { width: 320, height: 812 });
      const size = await page.evaluate(() => ({ inner: window.innerWidth, scroll: document.documentElement.scrollWidth }));
      expect.soft(size.inner, `${route.name}: innerWidth`).toBe(320);
      expect.soft(size.scroll, `${route.name}: scrollWidth`).toBeLessThanOrEqual(320);
      await close();
    }
  });

  test("E26 hit area ≥ 44px ở 375px và tối đa một nút gradient", async ({ browser }) => {
    for (const route of routes) {
      const { page, close } = await openRoute(browser, route, { width: 375, height: 812 });
      expect.soft(await smallHitAreas(page), `${route.name}: vùng chạm < 44px`).toEqual([]);
      expect.soft(await gradientCount(page), `${route.name}: số nút gradient`).toBeLessThanOrEqual(1);
      await close();
    }
  });

  test("E32 axe không có vi phạm serious/critical (kể cả button-name)", async ({ browser }) => {
    for (const route of routes) {
      const { page, close } = await openRoute(browser, route);
      const { violations, inputBorder } = await runAxe(page);
      expect.soft(violations, `${route.name}: ${JSON.stringify(violations)}`).toEqual([]);
      // Ngoại lệ đã ghi nhận (plan.md §11): viền input gray-300 dưới 3:1, liệt kê riêng chứ không ẩn.
      for (const finding of inputBorder) {
        test.info().annotations.push({
          type: "axe-input-border",
          description: `${route.name}: ${finding.id} × ${String(finding.targets.length)} (${finding.targets.slice(0, 3).join(", ")})`,
        });
      }
      await close();
    }
  });
});

test.describe("Bàn phím, announcer, dialog", () => {
  test.use({ storageState: storageStatePath("admin") });

  test("E27 reduced motion tắt transition và animation", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(stagePath);
    const primary = page.getByRole("button", { name: "Nhân bản thành bản nháp" });
    await expect(primary).toBeVisible();
    expect(await primary.evaluate((el) => getComputedStyle(el).transitionDuration)).toBe("0s");

    await page.getByRole("button", { name: "Lưu trữ", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "Lưu trữ v1?" });
    await expect(dialog).toBeVisible();
    expect(await dialog.evaluate((el) => getComputedStyle(el).animationName)).toBe("none");
    expect(await dialog.evaluate((el) => getComputedStyle(el).animationDuration)).toBe("0s");
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });

  test("E28 điều hướng được thông báo một lần, focus về h1", async ({ page }) => {
    await page.goto("/admin");
    await waitForPage(page);
    const announcer = page.locator('p[aria-live="polite"]');
    await expect(announcer).toHaveText("");

    await page.getByRole("navigation", { name: "Chính" }).getByRole("link", { name: "Chặng", exact: true }).click();
    await expect(page).toHaveURL(/\/admin\/stages$/);
    await expect(announcer).toHaveText("Chặng", { useInnerText: false });
    expect(await announcer.textContent()).toBe("Chặng");
    // Đúng một vùng aria-live mang tiêu đề trang; khung ứng dụng không tự là live region.
    expect(
      await page.locator('[aria-live="polite"], [aria-live="assertive"]').filter({ hasText: "Chặng" }).count(),
    ).toBe(1);
    await expect(page).toHaveTitle("Chặng · GoUp LMS");
    await expect(page.locator("#root")).not.toHaveAttribute("aria-live");
    await expect(page.locator("main h1")).toBeFocused();
  });

  test("E29 tab lớp là nav có nhãn, không phải tablist", async ({ page }) => {
    await page.goto(`/admin/classes/${await ids.classId("basic01")}?tab=report`);
    await waitForPage(page);
    await expect(page.locator('[role="tablist"]')).toHaveCount(0);
    const tabs = page.getByRole("navigation", { name: "Mục của lớp" });
    await expect(tabs.locator('a[aria-current="page"]')).toHaveCount(1);
    await expect(tabs.locator('a[aria-current="page"]')).toHaveText("Tiến độ");
  });

  test("E30 dialog có tên, bẫy focus, Escape đóng và trả focus", async ({ page }) => {
    await page.goto(stagePath);
    const opener = page.getByRole("button", { name: "Lưu trữ", exact: true });
    await opener.click();
    const dialog = page.getByRole("dialog", { name: "Lưu trữ v1?" });
    await expect(dialog).toBeVisible();
    const heading = await dialog.evaluate((el) => {
      const id = el.getAttribute("aria-labelledby");
      return id ? (document.getElementById(id)?.textContent ?? "") : "";
    });
    expect(heading).toBe("Lưu trữ v1?");

    expect(await focusInsideDialog(page)).toBe(true);
    // Dialog có 3 phần tử focus được (Đóng, Hủy, Lưu trữ): Tab và Shift+Tab vòng quanh mà không thoát ra.
    for (let i = 0; i < 5; i++) {
      await page.keyboard.press("Tab");
      expect(await focusInsideDialog(page), `Tab lần ${String(i + 1)}`).toBe(true);
    }
    for (let i = 0; i < 5; i++) {
      await page.keyboard.press("Shift+Tab");
      expect(await focusInsideDialog(page), `Shift+Tab lần ${String(i + 1)}`).toBe(true);
    }

    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(opener).toBeFocused();
  });

  test("E32 skip link là điểm focus đầu tiên và đưa focus vào main", async ({ page }) => {
    await page.goto("/admin");
    await waitForPage(page);
    await page.keyboard.press("Tab");
    const skip = page.getByRole("link", { name: "Bỏ qua điều hướng" });
    await expect(skip).toBeFocused();
    await expect(skip).toBeVisible();
    await page.keyboard.press("Enter");
    await expect(page.locator("main#main")).toBeFocused();
  });
});

test.describe("Toast", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });
  test.use({ storageState: storageStatePath("admin") });

  test.afterAll(async () => {
    // Nhân bản DB v1 tạo bản nháp v2: trả seed về như cũ cho các spec sau.
    await seedReset();
  });

  test("E31 tối đa 3 toast, tự tắt sau 4 s", async ({ page }) => {
    await page.goto(stagePath);
    const remove = page.getByRole("button", { name: "Xóa", exact: true });
    await expect(remove).toHaveAttribute("aria-disabled", "true");
    for (let i = 0; i < 5; i++) await remove.click({ force: true });
    await expect(toasts(page)).toHaveCount(5);
    await expect(visibleToasts(page)).toHaveCount(VISIBLE_TOASTS);
    const blocker = visibleToasts(page).filter({ hasText: "Đang được dùng trong" }).first();
    await expect(blocker).toBeVisible();

    // Chuột rời khỏi vùng toast (hover tạm dừng bộ đếm) rồi đo thời gian toast mới nhất tự tắt.
    await page.mouse.move(0, 0);
    await page.waitForTimeout(TOAST_DURATION_MS + 1500);
    await expect(toasts(page)).toHaveCount(0, { timeout: 2000 });

    const started = Date.now();
    await remove.click({ force: true });
    await page.mouse.move(0, 0);
    await expect(toasts(page)).toHaveCount(1);
    await expect(toasts(page)).toHaveCount(0, { timeout: TOAST_DURATION_MS + 3000 });
    const elapsed = Date.now() - started;
    expect(elapsed).toBeGreaterThanOrEqual(TOAST_DURATION_MS - 500);
    expect(elapsed).toBeLessThan(TOAST_DURATION_MS + 3000);
  });

  test("E31 toast sau thao tác thành công có nút đóng focus được", async ({ page }) => {
    await page.goto(stagePath);
    await page.getByRole("button", { name: "Nhân bản thành bản nháp" }).click();
    const toast = toasts(page).filter({ hasText: "Đã tạo bản nháp mới." });
    await expect(toast).toBeVisible();
    const close = toast.getByRole("button", { name: "Đóng thông báo" });
    await close.focus();
    await expect(close).toBeFocused();
    await close.click();
    await expect(toast).toHaveCount(0);
  });
});
