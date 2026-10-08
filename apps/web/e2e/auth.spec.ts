/*
 * Đăng nhập, mật khẩu và phân quyền (E13–E18).
 * Ánh xạ check(...) của prototype/check-flows.mjs:
 *   'temp-password login forces first-login page'  → E13
 *   'short password rejected'                       → E13
 *   'valid new password lands on learn home'        → E13
 *   'expired temp password login refused'           → E14
 *   'disabled account login refused'                → E14
 *   'teacher cannot open admin route'               → E18
 *   (khóa đăng nhập, quên mật khẩu, vô hiệu hóa khi đang có phiên: tiêu chí FR-04, FR-05, FR-06) → E15, E16, E17
 */
import { expect, test, type APIRequestContext, type Page } from "@playwright/test";
import { apiAs, loginContext, saveStorageStates, seedPassword, storageStatePath } from "./support/auth";
import { closeDb, count, expireResetToken, ids, one } from "./support/db";
import { extractResetToken, messageText, messagesTo, waitForNewMessage } from "./support/mailpit";
import { seedReset } from "./support/seed";
import { rowWith } from "./support/ui";

const NEW_PASSWORD = "Mat-khau-moi-E2E-2026";
const INVALID_RESET = "Liên kết đặt lại mật khẩu không hợp lệ hoặc đã hết hạn.";

async function loginUi(page: Page, email: string, password: string) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Mật khẩu").fill(password);
  await page.getByRole("button", { name: "Đăng nhập" }).click();
}

async function resetWithToken(page: Page, token: string, password: string) {
  await page.goto(`/reset-password#token=${token}`);
  await expect(page.getByRole("heading", { name: "Đặt lại mật khẩu" })).toBeVisible();
  // Token chỉ nằm trong fragment và bị xóa khỏi thanh địa chỉ ngay khi đọc.
  await expect(page).toHaveURL(/\/reset-password$/);
  await page.getByLabel(/^Mật khẩu mới/).fill(password);
  await page.getByLabel("Nhập lại mật khẩu mới").fill(password);
  await page.getByRole("button", { name: "Đặt lại mật khẩu" }).click();
}

async function errorCode(res: { json: () => Promise<unknown> }): Promise<string> {
  return ((await res.json()) as { error: { code: string } }).error.code;
}

test.describe("Xác thực và phân quyền", { tag: "@serial" }, () => {
  test.describe.configure({ mode: "serial" });

  test.beforeAll(async () => {
    await seedReset();
  });

  test.afterAll(async () => {
    await seedReset();
    await closeDb();
  });

  test("E13 đăng nhập lần đầu bằng mật khẩu tạm", async ({ page }) => {
    const email = "minh.bui@gmail.com";
    await loginUi(page, email, seedPassword());
    await expect(page).toHaveURL(/\/first-login$/);
    await expect(page.getByRole("heading", { name: "Đặt mật khẩu của bạn" })).toBeVisible();

    // Phiên của tài khoản chưa đổi mật khẩu chỉ dùng được cho đổi mật khẩu.
    const blocked = await page.request.get("/api/v1/me/classes");
    expect(blocked.status()).toBe(403);
    expect(await errorCode(blocked)).toBe("PASSWORD_CHANGE_REQUIRED");

    const fill = async (password: string) => {
      await page.getByLabel(/^Mật khẩu mới/).fill(password);
      await page.getByLabel("Nhập lại mật khẩu mới").fill(password);
      await page.getByRole("button", { name: "Lưu mật khẩu" }).click();
    };
    await fill("short");
    await expect(page.getByText("Mật khẩu mới cần tối thiểu 8 ký tự.")).toBeVisible();
    await fill(seedPassword());
    await expect(page.getByRole("alert")).toHaveText("Mật khẩu mới phải khác mật khẩu tạm.");
    await fill(NEW_PASSWORD);
    await expect(page.getByText("Đã lưu mật khẩu. Chào mừng bạn vào lớp.")).toBeVisible();
    await expect(page).toHaveURL(/\/learn$/);

    const user = await one<{ must_change_password: boolean; status: string; temp_password_expires_at: Date | null }>(
      "SELECT must_change_password, status, temp_password_expires_at FROM users WHERE email_normalized = $1",
      [email],
    );
    expect(user).toEqual({ must_change_password: false, status: "active", temp_password_expires_at: null });
  });

  test("E14 mật khẩu tạm hết hạn và tài khoản vô hiệu hóa bị từ chối", async ({ page }) => {
    await loginUi(page, "dung.pham@gmail.com", seedPassword());
    await expect(page.getByRole("alert")).toHaveText("Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên.");
    await expect(page).toHaveURL(/\/login$/);
    await loginUi(page, "thao.vo@gmail.com", seedPassword());
    await expect(page.getByRole("alert")).toHaveText("Tài khoản đã bị vô hiệu hóa. Vui lòng liên hệ quản trị viên.");
    await expect(page).toHaveURL(/\/login$/);
  });

  test("E15 sai mật khẩu 5 lần khóa email đó, email khác vẫn đăng nhập được", async ({ page, request }) => {
    const email = "phong.dang@gmail.com";
    for (let i = 0; i < 5; i++) {
      const wrong = await request.post("/api/v1/auth/login", {
        data: { email, password: `sai-mat-khau-${String(i)}` },
        headers: { "X-Requested-With": "fetch" },
      });
      expect(wrong.status()).toBe(401);
    }
    const locked = await request.post("/api/v1/auth/login", {
      data: { email, password: seedPassword() },
      headers: { "X-Requested-With": "fetch" },
    });
    expect(locked.status()).toBe(429);
    expect(await errorCode(locked)).toBe("TOO_MANY_ATTEMPTS");
    expect(Number(locked.headers()["retry-after"])).toBeGreaterThan(0);

    await loginUi(page, email, seedPassword());
    await expect(page.getByRole("alert")).toHaveText("Bạn đã nhập sai quá nhiều lần. Thử lại sau 15 phút.");

    // Khóa theo email, không theo IP: cùng máy, email khác vẫn vào được.
    const other = await loginContext("linh.do@gmail.com", seedPassword());
    await other.dispose();
  });

  test("E16 quên mật khẩu: cùng thông báo, đặt lại qua Mailpit, token dùng một lần và hết hạn", async ({ page }) => {
    const email = "nhan.ngo@gmail.com";
    const ghost = "e2e+e16-khong-ton-tai@example.com";
    const notice = "Nếu email tồn tại trong hệ thống, chúng tôi đã gửi đường dẫn đặt lại mật khẩu. Vui lòng kiểm tra hộp thư.";
    const requestReset = async (address: string) => {
      await page.goto("/forgot");
      await page.getByLabel("Email").fill(address);
      await page.getByRole("button", { name: "Gửi đường dẫn" }).click();
      await expect(page.getByRole("status").filter({ hasText: notice })).toBeVisible();
    };

    const seen = await messagesTo(email);
    await requestReset(ghost);
    await requestReset(email);
    const mail = await waitForNewMessage(email, seen);
    const text = await messageText(mail.ID);
    expect(text).toMatch(/\/reset-password#token=/);
    const token = extractResetToken(text);
    if (!token) throw new Error("thư không có token");
    // Thư của email thật đã tới; email không tồn tại không nhận gì.
    expect(await messagesTo(ghost)).toHaveLength(0);

    await resetWithToken(page, token, NEW_PASSWORD);
    await expect(page.getByText("Đã đặt lại mật khẩu. Hãy đăng nhập bằng mật khẩu mới.")).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
    const fresh = await loginContext(email, NEW_PASSWORD);
    await fresh.dispose();

    await resetWithToken(page, token, `${NEW_PASSWORD}-2`);
    await expect(page.getByRole("alert")).toHaveText(INVALID_RESET);

    await requestReset(email);
    const newest = await waitForNewMessage(email, [...seen, mail]);
    const second = extractResetToken(await messageText(newest.ID));
    if (!second) throw new Error("thư thứ hai không có token");
    expect(await expireResetToken(email)).toBe(1);
    await resetWithToken(page, second, `${NEW_PASSWORD}-3`);
    await expect(page.getByRole("alert")).toHaveText(INVALID_RESET);
    test.info().annotations.push({
      type: "not-covered",
      description: "429 của /auth/forgot-password: APP_ENV=e2e tắt rate limiter theo IP, kiểm ở hardening H4.",
    });
  });

  test("E18 guard vai trò: chuyển về trang chủ của vai trò, API trả 403", async ({ browser }) => {
    const teacherCtx = await browser.newContext({ storageState: storageStatePath("teacher") });
    const teacherPage = await teacherCtx.newPage();
    await teacherPage.goto("/admin");
    await expect(teacherPage).toHaveURL(/\/teach$/);
    const dashboard = await teacherPage.request.get("/api/v1/dashboard");
    expect(dashboard.status()).toBe(403);
    expect(await errorCode(dashboard)).toBe("FORBIDDEN");
    await teacherCtx.close();

    const studentCtx = await browser.newContext({ storageState: storageStatePath("student") });
    const studentPage = await studentCtx.newPage();
    await studentPage.goto("/teach");
    await expect(studentPage).toHaveURL(/\/learn$/);
    await studentPage.goto("/admin/stages");
    await expect(studentPage).toHaveURL(/\/learn$/);
    await studentCtx.close();
  });

  test("E17 vô hiệu hóa An khi An đang có phiên", async ({ browser }) => {
    const anEmail = "an.nguyen@gmail.com";
    const anId = await ids.userId(anEmail);
    const basic01 = await ids.classId("basic01");
    const memberId = await ids.memberId("basic01", anEmail);
    const progress = await count("SELECT count(*) AS n FROM lesson_progress WHERE class_member_id = $1", [memberId]);

    const an: APIRequestContext = await loginContext(anEmail, seedPassword());
    expect((await an.get("/api/v1/me/classes")).status()).toBe(200);

    const adminCtx = await browser.newContext({ storageState: storageStatePath("admin") });
    const adminPage = await adminCtx.newPage();
    await adminPage.goto(`/admin/classes/${basic01}?tab=students`);
    await rowWith(adminPage, anEmail).getByRole("button", { name: "Vô hiệu hóa" }).click();
    await adminPage
      .getByRole("dialog", { name: `Vô hiệu hóa tài khoản ${anEmail}?` })
      .getByRole("button", { name: "Vô hiệu hóa" })
      .click();
    await expect(adminPage.getByText("Đã vô hiệu hóa tài khoản.")).toBeVisible();

    expect((await an.get("/api/v1/me/classes")).status()).toBe(401);
    await an.dispose();
    expect(await count("SELECT count(*) AS n FROM sessions WHERE user_id = $1", [anId])).toBe(0);
    expect(await count("SELECT count(*) AS n FROM lesson_progress WHERE class_member_id = $1", [memberId])).toBe(progress);
    expect(await count("SELECT count(*) AS n FROM audit_logs WHERE action = 'user.disabled' AND target_id = $1", [anId])).toBe(1);

    await rowWith(adminPage, anEmail).getByRole("button", { name: "Kích hoạt lại" }).click();
    await expect(adminPage.getByText("Đã kích hoạt lại tài khoản.")).toBeVisible();
    await adminCtx.close();
    const again = await loginContext(anEmail, seedPassword());
    await again.dispose();
    // Phiên lưu sẵn của học viên đã bị hủy cùng lúc vô hiệu hóa: đăng nhập lại cho các spec sau.
    await saveStorageStates();
    const student = await apiAs("student");
    expect((await student.get("/api/v1/me/classes")).status()).toBe(200);
    await student.dispose();
  });
});
