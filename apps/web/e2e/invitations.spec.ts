/*
 * Mời, gửi lại và gỡ học viên (E08–E12) trên stack thật, thư đọc qua Mailpit.
 * Ánh xạ check(...) của prototype/check-flows.mjs:
 *   'inviting existing member is refused'                 → E08
 *   'new email creates invited account with temp password' → E09
 *   'invitation email transitions queued -> sent'         → E09
 *   'bounce address ends as failed'                       → E10
 *   (lớp đã kết thúc / tài khoản nội bộ, gỡ học viên: tiêu chí FR-01, FR-23, không có check riêng) → E11, E12
 */
import { expect, test, type APIRequestContext, type Page } from "@playwright/test";
import { apiAs, storageStatePath } from "./support/auth";
import { closeDb, count, ids, one } from "./support/db";
import { extractTempPassword, messagesTo, messageText, waitForNewMessage } from "./support/mailpit";
import { createEndedClass, seedReset } from "./support/seed";
import { kpi, rowWith } from "./support/ui";

const TEMP_PASSWORD_HOURS = 72;
// Mailer: 3 lần thử, chờ 1 phút rồi 5 phút giữa các lần (không cấu hình được qua env).
const BOUNCE_TIMEOUT = 8 * 60_000;

async function openInvite(page: Page, classId: string, code: string) {
  await page.goto(`/admin/classes/${classId}?tab=students`);
  await page.getByRole("button", { name: "Mời học viên" }).first().click();
  return page.getByRole("dialog", { name: `Mời học viên vào ${code}` });
}

test.describe("Mời học viên", { tag: "@serial" }, () => {
  test.use({ storageState: storageStatePath("admin") });

  let admin: APIRequestContext;
  let basic01: string;
  let basic02: string;

  test.beforeAll(async () => {
    await seedReset();
    admin = await apiAs("admin");
    basic01 = await ids.classId("basic01");
    basic02 = await ids.classId("basic02");
  });

  test.afterAll(async () => {
    await admin.dispose();
    await seedReset();
    await closeDb();
  });

  test("E08 mời trùng email đã chuẩn hóa bị từ chối", async ({ page }) => {
    const members = await count("SELECT count(*) AS n FROM class_members WHERE class_id = $1", [basic01]);
    const dialog = await openInvite(page, basic01, "basic01");
    await dialog.getByLabel("Email").fill("  AN.NGUYEN@gmail.com ");
    await dialog.getByRole("button", { name: "Gửi lời mời" }).click();
    await expect(dialog.getByRole("alert")).toHaveText("Học viên đã có trong lớp.");
    await expect(dialog).toBeVisible();
    expect(await count("SELECT count(*) AS n FROM class_members WHERE class_id = $1", [basic01])).toBe(members);
  });

  test("E09 mời email mới: tài khoản chưa đăng nhập, thư queued rồi sent", async ({ page }) => {
    const email = "e2e+e09@example.com";
    const seen = await messagesTo(email);
    const dialog = await openInvite(page, basic02, "basic02");
    await dialog.getByLabel("Email").fill(email);
    await dialog.getByLabel("Họ tên").fill("Học Viên E09");
    const responsePromise = page.waitForResponse(
      (r) => r.request().method() === "POST" && r.url().endsWith(`/api/v1/classes/${basic02}/invitations`),
    );
    await dialog.getByRole("button", { name: "Gửi lời mời" }).click();
    const response = await responsePromise;
    expect(response.status()).toBe(201);
    const responseText = await response.text();
    await expect(dialog).toBeHidden();
    await expect(page.getByText("Đã tạo tài khoản cho Học Viên E09, lời mời đang được gửi.")).toBeVisible();

    const row = rowWith(page, email);
    await expect(row).toContainText("Chưa đăng nhập");
    // Danh sách tự poll mỗi 5 s khi còn lời mời đang chờ: trạng thái chuyển sang "Đã gửi" trong ≤ 15 s.
    await expect(row).toContainText("Đã gửi", { timeout: 15_000 });
    const outbox = await one<{ status: string; attempts: number }>(
      "SELECT status, attempts FROM email_outbox WHERE to_email = $1",
      [email],
    );
    // attempts chỉ đếm lần gửi thất bại (MarkFailedAttempt), nên gửi thành công ngay lần đầu vẫn là 0.
    expect(outbox).toEqual({ status: "sent", attempts: 0 });

    const mail = await waitForNewMessage(email, seen);
    const password = extractTempPassword(await messageText(mail.ID));
    expect(password?.length ?? 0).toBeGreaterThanOrEqual(12);
    expect(responseText).not.toContain(password ?? "");
    // tempPasswordExpiresAt là mốc thời gian công khai; không có khóa mang mật khẩu hay hash.
    expect(responseText).not.toMatch(/"(temp)?password(_?hash)?"\s*:/i);

    // extract() trả numeric, pg đọc numeric thành chuỗi.
    const user = await one<{ must_change_password: boolean; status: string; hours: string }>(
      `SELECT must_change_password, status,
              extract(epoch FROM temp_password_expires_at - now()) / 3600 AS hours
         FROM users WHERE email_normalized = $1`,
      [email],
    );
    expect(user.must_change_password).toBe(true);
    expect(user.status).toBe("invited");
    expect(Number(user.hours)).toBeGreaterThan(TEMP_PASSWORD_HOURS - 0.1);
    expect(Number(user.hours)).toBeLessThanOrEqual(TEMP_PASSWORD_HOURS);
  });

  test("E10 thư bị từ chối: failed sau 3 lần, gửi lại tạo thư và mật khẩu mới", async ({ page }) => {
    test.setTimeout(BOUNCE_TIMEOUT + 60_000);
    const email = "e2e+bounce-e10@example.com";
    const failedBefore = await count("SELECT count(*) AS n FROM email_outbox WHERE status = 'failed' AND template = 'invite'");

    const dialog = await openInvite(page, basic02, "basic02");
    await dialog.getByLabel("Email").fill(email);
    await dialog.getByLabel("Họ tên").fill("Học Viên E10");
    await dialog.getByRole("button", { name: "Gửi lời mời" }).click();
    await expect(dialog).toBeHidden();

    await expect
      .poll(
        async () =>
          (await one<{ status: string }>("SELECT status FROM email_outbox WHERE to_email = $1", [email])).status,
        { timeout: BOUNCE_TIMEOUT, intervals: [5_000] },
      )
      .toBe("failed");
    const outbox = await one<{ attempts: number; last_error: string | null }>(
      "SELECT attempts, last_error FROM email_outbox WHERE to_email = $1",
      [email],
    );
    expect(outbox.attempts).toBe(3);
    expect(outbox.last_error).toBeTruthy();

    await page.reload();
    const row = rowWith(page, email);
    await expect(row).toContainText("Gửi thất bại");
    await expect(row).toContainText("3 lần thử");

    await page.goto("/admin");
    await expect(page.getByText(`${String(failedBefore + 1)} lời mời gửi thất bại.`)).toBeVisible();
    await expect(kpi(page, "Lời mời thất bại")).toHaveText(String(failedBefore + 1));

    const hashBefore = (await one<{ h: string }>("SELECT password_hash AS h FROM users WHERE email_normalized = $1", [email])).h;
    await page.goto(`/admin/classes/${basic02}?tab=students`);
    await rowWith(page, email).getByRole("button", { name: "Gửi lại" }).click();
    await page.getByRole("dialog", { name: "Gửi lại lời mời cho Học Viên E10?" }).getByRole("button", { name: "Gửi lại" }).click();
    await expect(page.getByText("Lời mời mới đang được gửi.")).toBeVisible();
    expect(await count("SELECT count(*) AS n FROM email_outbox WHERE to_email = $1", [email])).toBe(2);
    expect(await count("SELECT count(*) AS n FROM email_outbox WHERE to_email = $1 AND template = 'resend'", [email])).toBe(1);
    const hashAfter = (await one<{ h: string }>("SELECT password_hash AS h FROM users WHERE email_normalized = $1", [email])).h;
    expect(hashAfter).not.toBe(hashBefore);
  });

  test.describe("Lớp đã kết thúc và gỡ học viên", { tag: "@serial" }, () => {
    test.describe.configure({ mode: "serial" });

    test("E11 không mời vào lớp đã kết thúc hoặc tài khoản nội bộ", async ({ page }) => {
      const ended = await createEndedClass(admin, "e2e11");
      await page.goto(`/admin/classes/${ended.id}?tab=students`);
      const inviteButton = page.getByRole("button", { name: "Mời học viên" }).first();
      await expect(inviteButton).toHaveAttribute("aria-disabled", "true");
      await expect(inviteButton).toHaveAttribute("title", "Không mời được vào lớp đã kết thúc");
      await inviteButton.click({ force: true });
      await expect(page.getByText("Không mời được vào lớp đã kết thúc.")).toBeVisible();
      await expect(page.getByRole("dialog")).toHaveCount(0);
      const refused = await admin.post(`/api/v1/classes/${ended.id}/invitations`, {
        data: { email: "e2e+e11@example.com", fullName: "Học Viên E11" },
      });
      expect(refused.status()).toBe(409);
      expect(((await refused.json()) as { error: { code: string } }).error.code).toBe("INVALID_TRANSITION");

      const dialog = await openInvite(page, basic01, "basic01");
      await dialog.getByLabel("Email").fill("bao.pham@goup.vn");
      await dialog.getByRole("button", { name: "Gửi lời mời" }).click();
      await expect(dialog.getByRole("alert")).toHaveText("Email này thuộc tài khoản nội bộ, không mời làm học viên được.");
    });

    test("E12 gỡ An khỏi basic01: tiến độ còn nguyên, An không còn thấy lớp", async ({ page }) => {
      const memberId = await ids.memberId("basic01", "an.nguyen@gmail.com");
      const progress = await count("SELECT count(*) AS n FROM lesson_progress WHERE class_member_id = $1", [memberId]);
      expect(progress).toBeGreaterThan(0);

      await page.goto(`/admin/classes/${basic01}?tab=students`);
      await rowWith(page, "an.nguyen@gmail.com").getByRole("button", { name: "Gỡ khỏi lớp" }).click();
      await page.getByRole("dialog", { name: "Gỡ Nguyễn Hoàng An khỏi lớp?" }).getByRole("button", { name: "Gỡ khỏi lớp" }).click();
      await expect(page.getByText("Đã gỡ Nguyễn Hoàng An khỏi lớp.")).toBeVisible();
      await expect(rowWith(page, "an.nguyen@gmail.com")).toContainText("Đã rời lớp");

      expect((await one<{ status: string }>("SELECT status FROM class_members WHERE id = $1", [memberId])).status).toBe("dropped");
      expect(await count("SELECT count(*) AS n FROM lesson_progress WHERE class_member_id = $1", [memberId])).toBe(progress);
      expect(await count("SELECT count(*) AS n FROM audit_logs WHERE action = 'class.member_dropped' AND after->>'memberId' = $1", [memberId])).toBe(1);

      const student = await apiAs("student");
      const list = (await (await student.get("/api/v1/me/classes")).json()) as { items: { id: string }[] };
      expect(list.items.map((c) => c.id)).not.toContain(basic01);
      expect((await student.get(`/api/v1/me/classes/${basic01}`)).status()).toBeGreaterThanOrEqual(400);
      await student.dispose();
    });
  });
});
