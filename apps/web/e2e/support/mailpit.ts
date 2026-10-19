import { expect, request } from "@playwright/test";

// Mailpit API (compose: cổng 8025). Luôn lọc theo người nhận để test không đọc nhầm thư của test khác.
const MAILPIT_URL = process.env.E2E_MAILPIT_URL ?? "http://localhost:8025";

export interface MailSummary {
  ID: string;
  Subject: string;
  Created: string;
  To: { Address: string }[];
}

export async function messagesTo(address: string): Promise<MailSummary[]> {
  const ctx = await request.newContext({ baseURL: MAILPIT_URL });
  try {
    const res = await ctx.get("/api/v1/search", { params: { query: `to:"${address}"`, limit: "50" } });
    expect(res.ok(), `Mailpit search HTTP ${String(res.status())}`).toBe(true);
    const body = (await res.json()) as { messages: MailSummary[] | null };
    // Mailpit tìm theo chuỗi con: giữ lại đúng người nhận.
    return (body.messages ?? []).filter((m) => m.To.some((t) => t.Address.toLowerCase() === address.toLowerCase()));
  } finally {
    await ctx.dispose();
  }
}

/**
 * Chờ một thư mới tới địa chỉ, khác mọi thư trong `seen` (chụp trước thao tác gửi). Mailpit giữ thư của các lần
 * chạy trước, nên với địa chỉ cố định (tài khoản seed, email e2e cố định) không thể lấy "thư mới nhất" ngay.
 */
export async function waitForNewMessage(address: string, seen: MailSummary[], timeout = 15_000): Promise<MailSummary> {
  const known = new Set(seen.map((m) => m.ID));
  let fresh: MailSummary | undefined;
  await expect
    .poll(
      async () => {
        fresh = (await messagesTo(address)).find((m) => !known.has(m.ID));
        return fresh !== undefined;
      },
      { timeout, intervals: [250, 500, 1000] },
    )
    .toBe(true);
  if (!fresh) throw new Error(`không có thư mới tới ${address}`);
  return fresh;
}

/**
 * Chờ thư mới (không có trong `seen`) tới địa chỉ có tiêu đề đúng bằng `subject`. Dùng khi một thao tác gửi nhiều thư
 * cho cùng người nhận, hoặc thư khác có thể tới xen giữa, nên "thư mới đầu tiên" không đủ để xác định.
 */
export async function waitForSubject(
  address: string,
  subject: string,
  seen: MailSummary[],
  timeout = 15_000,
): Promise<MailSummary> {
  const known = new Set(seen.map((m) => m.ID));
  let found: MailSummary | undefined;
  await expect
    .poll(
      async () => {
        found = (await messagesTo(address)).find((m) => !known.has(m.ID) && m.Subject === subject);
        return found !== undefined;
      },
      { timeout, intervals: [250, 500, 1000], message: `thư "${subject}" tới ${address}` },
    )
    .toBe(true);
  if (!found) throw new Error(`không có thư "${subject}" tới ${address}`);
  return found;
}

export async function messageText(id: string): Promise<string> {
  const ctx = await request.newContext({ baseURL: MAILPIT_URL });
  try {
    const res = await ctx.get(`/api/v1/message/${id}`);
    expect(res.ok(), `Mailpit message HTTP ${String(res.status())}`).toBe(true);
    const body = (await res.json()) as { Text: string; HTML: string };
    return body.Text || body.HTML;
  } finally {
    await ctx.dispose();
  }
}

/** Mật khẩu tạm trong bản text của thư mời/gửi lại: dòng "Mật khẩu tạm: <giá trị>". */
export function extractTempPassword(text: string): string | undefined {
  return /^Mật khẩu tạm:\s*(\S+)\s*$/m.exec(text)?.[1];
}

export function extractResetToken(text: string): string | undefined {
  return /#token=([A-Za-z0-9_-]+)/.exec(text)?.[1];
}
