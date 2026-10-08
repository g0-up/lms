import AxeBuilder from "@axe-core/playwright";
import { expect, type Page } from "@playwright/test";
import type { Role } from "./auth";
import { ids } from "./db";

export interface AppRoute {
  /** Tên ngắn dùng trong tên test và đường dẫn ảnh. */
  name: string;
  path: string;
  role: Role | null;
  /** Route tương ứng của prototype (hash) cho render-check. */
  prototype: string;
}

/** 10 route của `prototype/check-flows.mjs` (mục 320px reflow / axe), ánh xạ sang id thật của seed. */
export async function checkFlowRoutes(): Promise<AppRoute[]> {
  const [db, basic, basic01, intro] = await Promise.all([
    ids.stageId("DB"),
    ids.courseId("BASIC"),
    ids.classId("basic01"),
    ids.lessonInClass("basic01", "db-intro"),
  ]);
  return [
    { name: "login", path: "/login", role: null, prototype: "#/login" },
    { name: "admin", path: "/admin", role: "admin", prototype: "#/admin?as=u-admin" },
    { name: "admin-stage", path: `/admin/stages/${db}`, role: "admin", prototype: "#/admin/stages/st-db?as=u-admin" },
    { name: "admin-course", path: `/admin/courses/${basic}`, role: "admin", prototype: "#/admin/courses/co-basic?as=u-admin" },
    { name: "admin-class", path: `/admin/classes/${basic01}`, role: "admin", prototype: "#/admin/classes/cl-basic01?as=u-admin" },
    {
      name: "admin-class-report",
      path: `/admin/classes/${basic01}?tab=report`,
      role: "admin",
      prototype: "#/admin/classes/cl-basic01?tab=report&as=u-admin",
    },
    { name: "teach", path: "/teach", role: "teacher", prototype: "#/teach?as=u-gv" },
    { name: "learn", path: "/learn", role: "student", prototype: "#/learn?as=u-an" },
    { name: "learn-class", path: `/learn/classes/${basic01}`, role: "student", prototype: "#/learn/classes/cl-basic01?as=u-an" },
    {
      name: "lesson",
      path: `/learn/classes/${basic01}/lessons/${intro}`,
      role: "student",
      prototype: "#/learn/classes/cl-basic01/lessons/db-intro?as=u-an",
    },
  ];
}

/** Chờ trang dữ liệu hiển thị xong: có h1, không còn skeleton, mạng rảnh. */
export async function waitForPage(page: Page): Promise<void> {
  await expect(page.locator("h1").first()).toBeVisible();
  await expect(page.locator('[data-slot="skeleton"]')).toHaveCount(0);
  await page.waitForLoadState("networkidle");
}

export interface AxeFinding {
  id: string;
  impact: string;
  targets: string[];
}

/** Vi phạm mức serious/critical; vi phạm tương phản do viền input gray-300 được tách riêng (ngoại lệ đã ghi nhận). */
export async function runAxe(page: Page): Promise<{ violations: AxeFinding[]; inputBorder: AxeFinding[] }> {
  const result = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  const serious = result.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  const violations: AxeFinding[] = [];
  const inputBorder: AxeFinding[] = [];
  for (const v of serious) {
    const finding = { id: v.id, impact: v.impact ?? "", targets: v.nodes.map((n) => n.target.join(" ")) };
    const onlyInputBorders =
      v.id === "color-contrast" && v.nodes.every((n) => /input|textarea|select/i.test(n.target.join(" ")));
    (onlyInputBorders ? inputBorder : violations).push(finding);
  }
  return { violations, inputBorder };
}

/**
 * Phần tử tương tác hiển thị có vùng chạm < 44px (cao < 44 và diện tích < 44×44), trừ link trong đoạn văn.
 * Phần tử sr-only (1×1) như skip link lúc chưa focus không phải đích chạm. Checkbox 18px nằm trong `label`
 * (shared/ui/checkbox): label bấm được là vùng chạm, nên đo label.
 */
export function smallHitAreas(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const MIN = 44;
    return [...document.querySelectorAll<HTMLElement>('button, a[href], input, select, [role="button"]')]
      .filter((el) => !el.closest("p") || el.tagName !== "A")
      .flatMap((el) => {
        const own = el.getBoundingClientRect();
        if (!el.checkVisibility({ visibilityProperty: true }) || own.width <= 1 || own.height <= 1) return [];
        const r = (el.closest("label") ?? el).getBoundingClientRect();
        if (r.height >= MIN || r.width * r.height >= MIN * MIN) return [];
        const name = (el.getAttribute("aria-label") ?? el.textContent).trim().slice(0, 40);
        return [`${el.tagName.toLowerCase()} "${name}" ${String(Math.round(r.width))}×${String(Math.round(r.height))}`];
      });
  });
}
