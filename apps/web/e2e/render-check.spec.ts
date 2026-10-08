/*
 * Render-check: bản React so với prototype tĩnh (project "render", không nằm trong test:e2e).
 * Prototype được phục vụ ở :8090 (`python3 -m http.server`), dùng deep link `#/…?as=<user>`.
 * Mỗi route ở plan.md §8 × 3 viewport (1440×900, 768×1024, 375×812):
 *   1. computed style của topbar, nav active, nút gradient, body, card, input phải trùng prototype → lệch là fail;
 *   2. bố cục web: 0 tràn ngang, 0 chữ bị cắt, 0 chữ < 12px → fail;
 *   3. ảnh theo vùng (topbar, page-head, card đầu, bảng) so với ảnh prototype, ngưỡng 2%, `time` bị mask
 *      → chỉ cảnh báo (annotation "render-diff"), vì dữ liệu và thời gian hai bên không trùng từng pixel.
 * Ảnh toàn trang: e2e/__screenshots__/<route>/<viewport>.png (web) và e2e/__screenshots__/prototype/<route>/<viewport>.png.
 * Ánh xạ check(...) của prototype/check-flows.mjs: không có (check-flows không so giao diện); tiêu chí từ docs/review.md.
 */
import { spawn, type ChildProcess } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { expect, test, type Browser, type Locator, type Page } from "@playwright/test";
import { loginContext, REPO_ROOT, seedPassword, storageStatePath, type Role } from "./support/auth";
import { closeDb, ids } from "./support/db";

const PROTOTYPE_PORT = 8090;
const PROTOTYPE_URL = `http://127.0.0.1:${String(PROTOTYPE_PORT)}/index.html`;
const SHOTS_DIR = path.resolve(import.meta.dirname, "__screenshots__");
const VIEWPORTS = [
  { name: "1440x900", width: 1440, height: 900 },
  { name: "768x1024", width: 768, height: 1024 },
  { name: "375x812", width: 375, height: 812 },
] as const;
type Viewport = (typeof VIEWPORTS)[number];

// Token của prototype (goup.css) mà plan yêu cầu kiểm cố định, ngoài việc so trực tiếp hai bên.
const NAVY_700 = "rgb(35, 60, 101)";
const ACCENT_ORANGE = "rgb(244, 93, 41)";
const SURFACE_50 = "rgb(245, 245, 245)";

/** Phiên của trang web: vai trò seed, hoặc Minh (mật khẩu tạm) cho /first-login. */
type Session = Role | "first-login" | null;

interface SeedIds {
  db: string;
  basic: string;
  basic01: string;
  intro: string;
}

interface RenderRoute {
  name: string;
  path: (id: SeedIds) => string;
  session: Session;
  prototype: string;
  /**
   * Prototype tô gradient nút này nhưng phase-12 chỉ định `Button default` (navy): trên lớp nháp nút "Kích hoạt
   * lớp" đã là gradient, nên hai gradient cùng trang sẽ phạm quy tắc một hành động chính mỗi trang.
   */
  navyPrimary?: string;
}

// Các route của plan.md §8 có trang tương ứng trong prototype (/reset-password không có).
const ROUTES: RenderRoute[] = [
  { name: "login", path: () => "/login", session: null, prototype: "#/login" },
  { name: "forgot", path: () => "/forgot", session: null, prototype: "#/forgot" },
  { name: "first-login", path: () => "/first-login", session: "first-login", prototype: "#/first-login?as=u-minh" },
  { name: "admin", path: () => "/admin", session: "admin", prototype: "#/admin?as=u-admin" },
  { name: "admin-stages", path: () => "/admin/stages", session: "admin", prototype: "#/admin/stages?as=u-admin" },
  { name: "admin-stage", path: (id) => `/admin/stages/${id.db}`, session: "admin", prototype: "#/admin/stages/st-db?as=u-admin" },
  { name: "admin-courses", path: () => "/admin/courses", session: "admin", prototype: "#/admin/courses?as=u-admin" },
  { name: "admin-course", path: (id) => `/admin/courses/${id.basic}`, session: "admin", prototype: "#/admin/courses/co-basic?as=u-admin" },
  { name: "admin-classes", path: () => "/admin/classes", session: "admin", prototype: "#/admin/classes?as=u-admin" },
  {
    name: "admin-class-students",
    path: (id) => `/admin/classes/${id.basic01}?tab=students`,
    session: "admin",
    prototype: "#/admin/classes/cl-basic01?tab=students&as=u-admin",
    navyPrimary: "Mời học viên",
  },
  {
    name: "admin-class-report",
    path: (id) => `/admin/classes/${id.basic01}?tab=report`,
    session: "admin",
    prototype: "#/admin/classes/cl-basic01?tab=report&as=u-admin",
  },
  {
    name: "admin-class-settings",
    path: (id) => `/admin/classes/${id.basic01}?tab=settings`,
    session: "admin",
    prototype: "#/admin/classes/cl-basic01?tab=settings&as=u-admin",
    navyPrimary: "Lưu thay đổi",
  },
  { name: "teach", path: () => "/teach", session: "teacher", prototype: "#/teach?as=u-gv" },
  { name: "teach-class", path: (id) => `/teach/classes/${id.basic01}`, session: "teacher", prototype: "#/teach/classes/cl-basic01?as=u-gv" },
  { name: "learn", path: () => "/learn", session: "student", prototype: "#/learn?as=u-an" },
  { name: "learn-class", path: (id) => `/learn/classes/${id.basic01}`, session: "student", prototype: "#/learn/classes/cl-basic01?as=u-an" },
  {
    name: "lesson",
    path: (id) => `/learn/classes/${id.basic01}/lessons/${id.intro}`,
    session: "student",
    prototype: "#/learn/classes/cl-basic01/lessons/db-intro?as=u-an",
  },
];

/** Computed style của các phần tử token; selector riêng cho từng bên, cùng thuộc tính. */
interface StyleProbe {
  header: string | null;
  navActive: string | null;
  gradient: string | null;
  gradientCount: number;
  bodyFont: string;
  bodySize: string;
  bodyBg: string;
  cardRadius: string | null;
  cardShadow: string | null;
  inputHeight: string | null;
}

interface Selectors {
  card: string;
  pageHead: string;
}

const WEB: Selectors = { card: '[data-slot="card"]', pageHead: "main .mb-6:has(h1)" };
const PROTO: Selectors = { card: ".card", pageHead: ".page-head" };

function probeStyles(page: Page, sel: Selectors): Promise<StyleProbe> {
  return page.evaluate(({ card }) => {
    const shown = (el: Element) => {
      const r = el.getBoundingClientRect();
      const cs = getComputedStyle(el);
      return r.width > 0 && r.height > 0 && cs.visibility !== "hidden" && cs.display !== "none";
    };
    const first = (selector: string) => [...document.querySelectorAll(selector)].find(shown) ?? null;
    const style = (selector: string) => {
      const el = first(selector);
      return el ? getComputedStyle(el) : null;
    };
    const gradients = [...document.querySelectorAll("button, a")].filter(
      (el) => shown(el) && getComputedStyle(el).backgroundImage.includes("linear-gradient(260deg"),
    );
    const body = getComputedStyle(document.body);
    const cardStyle = style(card);
    // Tailwind ghép shadow với các lớp ring/inset trong suốt bằng 0: bỏ chúng để so với CSS thuần của prototype.
    const shadow = (value: string) =>
      value
        .split(/,(?![^(]*\))/)
        .map((layer) => layer.trim())
        .filter((layer) => !/^rgba\(0, 0, 0, 0\) 0px 0px 0px 0px$/.test(layer))
        .join(", ");
    return {
      header: style("header")?.backgroundColor ?? null,
      navActive: style('nav#nav a[aria-current="page"]')?.borderBottomColor ?? null,
      gradient: gradients[0] ? getComputedStyle(gradients[0]).backgroundImage : null,
      gradientCount: gradients.length,
      bodyFont: body.fontFamily,
      bodySize: body.fontSize,
      bodyBg: body.backgroundColor,
      cardRadius: cardStyle?.borderRadius ?? null,
      cardShadow: cardStyle ? shadow(cardStyle.boxShadow) : null,
      inputHeight:
        style('input:not([type="checkbox"]):not([type="radio"]):not([type="hidden"]):not([type="file"])')?.height ?? null,
    };
  }, sel);
}

/**
 * Lỗi bố cục theo tiêu chí render-check (docs/review.md): tràn ngang, chữ bị cắt, chữ < 12px.
 * Tag GoUp (`Badge`, 11px như `.badge` của prototype và `.un-tag` của goup.css) là ngoại lệ đã ghi trong
 * design-audit: không tính là lỗi nhưng được đếm riêng để báo cáo.
 */
function layoutDefects(page: Page): Promise<{ defects: string[]; smallTags: number }> {
  return page.evaluate(() => {
    const vw = window.innerWidth;
    const defects: string[] = [];
    let smallTags = 0;
    const describe = (el: Element) => {
      const text = (el as HTMLElement).innerText.trim().replace(/\s+/g, " ").slice(0, 40);
      const cls = [...el.classList].slice(0, 2).join(".");
      return `${el.tagName.toLowerCase()}${cls ? `.${cls}` : ""} "${text}"`;
    };
    const visible = (el: Element) => {
      const cs = getComputedStyle(el);
      if (cs.display === "none" || cs.visibility === "hidden" || Number(cs.opacity) === 0) return false;
      const r = el.getBoundingClientRect();
      return r.width > 0 && r.height > 0 && r.right > 0 && r.left < vw;
    };
    // Nội dung chỉ dành cho trình đọc màn hình (sr-only: khung 1×1 bị clip) không tính.
    const srOnly = (el: Element) => {
      const r = el.getBoundingClientRect();
      return r.width <= 1 && r.height <= 1;
    };
    const ownText = (el: Element) => [...el.childNodes].some((n) => n.nodeType === Node.TEXT_NODE && n.textContent?.trim());

    const overflow = document.documentElement.scrollWidth - vw;
    if (overflow > 1) defects.push(`tràn ngang ${String(overflow)}px`);

    for (const el of document.body.querySelectorAll("*")) {
      if (["SCRIPT", "STYLE", "NOSCRIPT", "TEMPLATE"].includes(el.tagName)) continue;
      if (!ownText(el) || !visible(el) || srOnly(el)) continue;
      const cs = getComputedStyle(el);
      const size = parseFloat(cs.fontSize);
      if (size < 12) {
        if (el.closest('[data-slot="badge"]')) smallTags++;
        else defects.push(`chữ ${String(size)}px: ${describe(el)}`);
      }
      const clipX = ["hidden", "clip"].includes(cs.overflowX) && el.scrollWidth > el.clientWidth + 1 && cs.textOverflow !== "ellipsis";
      const clipY =
        ["hidden", "clip"].includes(cs.overflowY) &&
        el.scrollHeight > el.clientHeight + 1 &&
        cs.getPropertyValue("-webkit-line-clamp") === "none";
      if (clipX || clipY) defects.push(`chữ bị cắt: ${describe(el)}`);
    }
    return { defects, smallTags };
  });
}

async function webContext(browser: Browser, session: Session, viewport: Viewport) {
  const size = { width: viewport.width, height: viewport.height };
  if (session === "first-login") {
    // Minh còn mật khẩu tạm: đăng nhập qua API rồi dùng cookie đó (phiên chỉ dùng được cho đổi mật khẩu).
    const api = await loginContext("minh.bui@gmail.com", seedPassword());
    const storageState = await api.storageState();
    await api.dispose();
    return browser.newContext({ storageState, viewport: size });
  }
  return browser.newContext({ storageState: session ? storageStatePath(session) : undefined, viewport: size });
}

async function settle(page: Page) {
  await expect(page.locator("h1").first()).toBeVisible();
  await expect(page.locator('[data-slot="skeleton"], [aria-busy="true"]')).toHaveCount(0);
  await page.waitForLoadState("networkidle");
  await page.evaluate(() => document.fonts.ready.then(() => undefined));
  await page.mouse.move(0, 0);
}

async function regionShot(page: Page, locator: Locator): Promise<Buffer | null> {
  const target = locator.first();
  if (!(await target.isVisible())) return null;
  return target.screenshot({ animations: "disabled", caret: "hide", mask: [page.locator("time")] });
}

function save(file: string, data: Buffer) {
  mkdirSync(path.dirname(file), { recursive: true });
  writeFileSync(file, data);
}

let server: ChildProcess | undefined;
let seedIds: SeedIds;

test.beforeAll(async ({ request }) => {
  const [db, basic, basic01, intro] = await Promise.all([
    ids.stageId("DB"),
    ids.courseId("BASIC"),
    ids.classId("basic01"),
    ids.lessonInClass("basic01", "db-intro"),
  ]);
  seedIds = { db, basic, basic01, intro };
  server = spawn("python3", ["-m", "http.server", String(PROTOTYPE_PORT), "--bind", "127.0.0.1", "--directory", path.join(REPO_ROOT, "prototype")], {
    stdio: "ignore",
  });
  await expect
    .poll(async () => (await request.get(PROTOTYPE_URL).catch(() => null))?.status() ?? 0, { timeout: 10_000 })
    .toBe(200);
});

test.afterAll(async () => {
  server?.kill("SIGTERM");
  await closeDb();
});

test.describe.configure({ timeout: 180_000 });

for (const route of ROUTES) {
  test(`render ${route.name}`, async ({ browser }) => {
    for (const viewport of VIEWPORTS) {
      const at = `${route.name} @${viewport.name}`;
      const webCtx = await webContext(browser, route.session, viewport);
      const protoCtx = await browser.newContext({ viewport: { width: viewport.width, height: viewport.height } });
      const web = await webCtx.newPage();
      const proto = await protoCtx.newPage();
      await web.goto(route.path(seedIds));
      await proto.goto(`${PROTOTYPE_URL}${route.prototype}`);
      await settle(web);
      await settle(proto);

      const [w, p] = await Promise.all([probeStyles(web, WEB), probeStyles(proto, PROTO)]);
      expect.soft(w.header, `${at}: màu topbar`).toBe(p.header);
      if (p.header) expect.soft(w.header, `${at}: topbar navy-700`).toBe(NAVY_700);
      expect.soft(w.navActive, `${at}: viền nav active`).toBe(p.navActive);
      if (p.navActive) expect.soft(w.navActive, `${at}: nav active cam`).toBe(ACCENT_ORANGE);
      if (p.gradient && route.navyPrimary) {
        expect.soft(w.gradient, `${at}: "${route.navyPrimary}" là Button default`).toBeNull();
        test.info().annotations.push({ type: "navy-primary", description: `${at}: "${route.navyPrimary}" navy theo phase-12` });
      } else if (p.gradient) {
        expect.soft(w.gradient, `${at}: nút gradient`).toBe(p.gradient);
      }
      expect.soft(w.gradientCount, `${at}: tối đa 1 nút gradient`).toBeLessThanOrEqual(1);
      expect.soft(w.bodyFont.startsWith('"Inter Tight"'), `${at}: font body ${w.bodyFont}`).toBe(true);
      expect.soft(w.bodyFont, `${at}: font body`).toBe(p.bodyFont);
      expect.soft(w.bodySize, `${at}: cỡ chữ body`).toBe("15px");
      expect.soft(w.bodySize, `${at}: cỡ chữ body`).toBe(p.bodySize);
      expect.soft(w.bodyBg, `${at}: nền body`).toBe(SURFACE_50);
      expect.soft(w.bodyBg, `${at}: nền body`).toBe(p.bodyBg);
      if (w.cardRadius !== null && p.cardRadius !== null) {
        expect.soft(w.cardRadius, `${at}: radius card`).toBe(p.cardRadius);
        expect.soft(w.cardShadow, `${at}: bóng card`).toBe(p.cardShadow);
        expect.soft(w.cardShadow ?? "", `${at}: bóng card 0 0 7px`).toContain("0px 0px 7px");
      }
      if (w.inputHeight !== null && p.inputHeight !== null) {
        expect.soft(w.inputHeight, `${at}: chiều cao input`).toBe(p.inputHeight);
      }

      const layout = await layoutDefects(web);
      expect.soft(layout.defects, `${at}: lỗi bố cục`).toEqual([]);
      if (layout.smallTags > 0) {
        test.info().annotations.push({ type: "tag-11px", description: `${at}: ${String(layout.smallTags)} tag 11px` });
      }

      save(path.join(SHOTS_DIR, route.name, `${viewport.name}.png`), await web.screenshot({ fullPage: true, animations: "disabled", mask: [web.locator("time")] }));
      save(
        path.join(SHOTS_DIR, "prototype", route.name, `${viewport.name}.png`),
        await proto.screenshot({ fullPage: true, animations: "disabled", mask: [proto.locator("time")] }),
      );

      const regions: [string, Locator, Locator][] = [
        ["topbar", web.locator("header"), proto.locator("header.topbar")],
        ["page-head", web.locator(WEB.pageHead).last(), proto.locator(PROTO.pageHead)],
        ["card", web.locator(WEB.card), proto.locator(PROTO.card)],
        ["table", web.locator("table"), proto.locator("table")],
      ];
      for (const [region, webRegion, protoRegion] of regions) {
        const [webShot, protoShot] = await Promise.all([regionShot(web, webRegion), regionShot(proto, protoRegion)]);
        if (!webShot || !protoShot) continue;
        // Ảnh prototype làm baseline, rồi so ảnh web với nó; vượt 2% chỉ ghi cảnh báo.
        const snapshot = ["prototype", route.name, `${viewport.name}-${region}.png`];
        save(path.join(SHOTS_DIR, ...snapshot), protoShot);
        try {
          expect(webShot).toMatchSnapshot(snapshot, { maxDiffPixelRatio: 0.02 });
        } catch (err) {
          // Dòng nói rõ lệch kích thước hay tỉ lệ pixel; message của Playwright mở đầu bằng một dòng chung chung.
          const lines = (err instanceof Error ? err.message : String(err)).split("\n");
          const reason = lines.find((l) => /Expected an image|pixels \(ratio/.test(l)) ?? lines.find((l) => l.trim()) ?? "";
          test.info().annotations.push({ type: "render-diff", description: `${at} ${region}: ${reason.trim()}` });
        }
      }

      await webCtx.close();
      await protoCtx.close();
    }
  });
}
