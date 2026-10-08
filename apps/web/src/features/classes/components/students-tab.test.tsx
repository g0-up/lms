import { act, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { toast } from "sonner";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { api, server } from "@/shared/test/msw";
import { errorResponse, seedDb } from "../api/msw-handlers";
import { ALREADY_ACTIVATED, RESEND_RATE_LIMITED } from "../hooks/use-member-mutations";
import { ENDED_CLASS, INVALID_EMAIL, NAME_REQUIRED } from "../model/invite-form";
import { POLL_EVERY_MS } from "../model/member-view";
import { errors, ids } from "../test/golden";
import { classPath, gate, polyfillRadix, renderClasses } from "../test/render-classes";

polyfillRadix();

beforeAll(async () => {
  await import("../pages/class-detail-page");
});

afterEach(() => {
  vi.useRealTimers();
  // Sonner keeps its toasts across renders: a toast must not leak into the next test.
  toast.dismiss();
});

const membersTable = () => screen.findByRole("table", { name: "Học viên lớp basic01" });

function memberRow(name: string): HTMLElement {
  const row = screen.getByText(name, { selector: "span" }).closest("tr");
  if (!row) throw new Error(`no row for ${name}`);
  return row;
}

type User = ReturnType<typeof renderClasses>["user"];

async function openInvite(user: User) {
  // The card head's button; an empty list offers a second one.
  const [button] = await screen.findAllByRole("button", { name: "Mời học viên" });
  await user.click(button);
  return screen.findByRole("dialog", { name: /^Mời học viên vào/ });
}

async function invite(user: User, email: string, fullName = "") {
  const dialog = await openInvite(user);
  await user.type(within(dialog).getByLabelText(/^Email/), email);
  if (fullName) await user.type(within(dialog).getByLabelText(/^Họ tên/), fullName);
  await user.click(within(dialog).getByRole("button", { name: "Gửi lời mời" }));
  return dialog;
}

async function confirmIn(user: User, title: string, confirm: string) {
  const dialog = await screen.findByRole("dialog", { name: title });
  await user.click(within(dialog).getByRole("button", { name: confirm }));
}

describe("students tab", () => {
  it("shows each member's account, invitation, progress and the actions that apply", async () => {
    renderClasses(classPath(ids.active));
    const table = await membersTable();

    // Active members by name, the one who left last.
    const names = within(table)
      .getAllByRole("row")
      .slice(1)
      .map((row) => row.querySelector("td span")?.textContent);
    expect(names).toEqual([
      "Bùi Quang Minh",
      "Học Viên Mới",
      "Trần Thị Bích",
      "Võ Phương Thảo",
      "Vũ Đức Khang",
      "Nguyễn Hoàng An",
    ]);

    const minh = memberRow("Bùi Quang Minh");
    expect(within(minh).getByText("Chưa đăng nhập")).toBeInTheDocument();
    expect(within(minh).getByText("Đã gửi thông báo")).toBeInTheDocument();
    expect(within(minh).getByRole("button", { name: "Gửi lại" })).toBeInTheDocument();

    const newcomer = memberRow("Học Viên Mới");
    expect(within(newcomer).getByText("Mật khẩu tạm hết hạn")).toBeInTheDocument();
    expect(within(newcomer).getByText("Đang chờ gửi")).toBeInTheDocument();

    const bich = memberRow("Trần Thị Bích");
    expect(within(bich).getByText("Đã kích hoạt")).toBeInTheDocument();
    expect(await within(bich).findByRole("img", { name: "50% hoàn thành" })).toBeInTheDocument();
    expect(within(bich).queryByRole("button", { name: "Gửi lại" })).not.toBeInTheDocument();
    expect(within(bich).getByRole("button", { name: "Vô hiệu hóa" })).toBeInTheDocument();
    expect(within(bich).getByRole("button", { name: "Gỡ khỏi lớp" })).toBeInTheDocument();

    const thao = memberRow("Võ Phương Thảo");
    expect(within(thao).getAllByText("Vô hiệu hóa")).toHaveLength(1);
    expect(within(thao).getByText("—")).toBeInTheDocument();
    expect(within(thao).getByRole("button", { name: "Kích hoạt lại" })).toBeInTheDocument();

    const khang = memberRow("Vũ Đức Khang");
    expect(within(khang).getByText("Gửi thất bại")).toBeInTheDocument();
    expect(within(khang).getByText("Mailbox không tồn tại (550 5.1.1) · 3 lần thử")).toBeInTheDocument();
    expect(within(khang).getByRole("img", { name: "14% hoàn thành" })).toBeInTheDocument();

    const an = memberRow("Nguyễn Hoàng An");
    // A dropped row is tinted, never faded, so its text keeps the 4.5:1 contrast.
    expect(an).toHaveClass("bg-surface-100");
    expect(an).not.toHaveClass("opacity-60");
    expect(within(an).getByText("Đã rời lớp")).toBeInTheDocument();
    expect(within(an).queryByRole("button")).not.toBeInTheDocument();
    expect(within(an).queryByRole("img")).not.toBeInTheDocument();
  });

  it("shows a loading state, then offers a retry when the members cannot be loaded", async () => {
    const hold = gate();
    const { user } = renderClasses(classPath(ids.active), {
      overrides: [
        http.get(
          api("/classes/:classId/members"),
          async () => {
            await hold.wait();
            return HttpResponse.json({ error: { code: "INTERNAL", message: "Lỗi máy chủ." } }, { status: 500 });
          },
          { once: true },
        ),
      ],
    });

    expect(await screen.findByText("Đang tải danh sách học viên")).toBeInTheDocument();
    hold.open();
    expect(await screen.findByRole("alert")).toHaveTextContent("Không tải được danh sách học viên.");
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await membersTable()).toBeInTheDocument();
  });

  it("invites from the empty state of a class without members", async () => {
    const { user } = renderClasses(classPath(ids.draft));

    expect(await screen.findByText("Chưa có học viên")).toBeInTheDocument();
    const buttons = screen.getAllByRole("button", { name: "Mời học viên" });
    expect(buttons).toHaveLength(2);
    await user.click(buttons[1]);
    const dialog = await screen.findByRole("dialog", { name: "Mời học viên vào basic04" });
    expect(within(dialog).getByText(/Mật khẩu tạm không bao giờ hiển thị/)).toBeInTheDocument();
  });

  it("refuses invitations into an ended class", async () => {
    const db = seedDb();
    db.classes = db.classes.map((c) => (c.id === ids.active ? { ...c, status: "ended" } : c));
    const { user } = renderClasses(classPath(ids.active), { db });
    await membersTable();

    const button = screen.getByRole("button", { name: "Mời học viên" });
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).toHaveAttribute("title", "Không mời được vào lớp đã kết thúc");
    await user.click(button);
    expect(await screen.findByText(ENDED_CLASS)).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("creates the account of a new email, normalized, and lists the queued invitation", async () => {
    const bodies: unknown[] = [];
    const { user } = renderClasses(classPath(ids.active));
    await membersTable();
    const record = http.post(api("/classes/:classId/invitations"), async ({ request }) => {
      bodies.push(await request.clone().json());
      return undefined;
    });
    server.use(record);

    await invite(user, "  Ngoc.Lan@Gmail.com ", "Đỗ Ngọc Lan");

    expect(await screen.findByText("Đã tạo tài khoản cho Đỗ Ngọc Lan, lời mời đang được gửi.")).toBeInTheDocument();
    expect(bodies).toEqual([{ email: "ngoc.lan@gmail.com", fullName: "Đỗ Ngọc Lan" }]);
    await waitFor(() => { expect(screen.queryByRole("dialog")).not.toBeInTheDocument(); });
    const lan = await screen.findByText("Đỗ Ngọc Lan", { selector: "span" });
    const row = lan.closest("tr") as HTMLElement;
    expect(within(row).getByText("Đang chờ gửi")).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "Mục của lớp" })).getByRole("link", { name: /^Học viên/ })).toHaveTextContent("6");
  });

  it("adds an existing account without a name", async () => {
    const { user } = renderClasses(classPath(ids.draft));
    await screen.findByText("Chưa có học viên");

    await invite(user, "bich.tran@gmail.com");

    expect(
      await screen.findByText("Trần Thị Bích đã có tài khoản: đã thêm vào lớp và gửi thông báo."),
    ).toBeInTheDocument();
    expect(await screen.findByText("Trần Thị Bích", { selector: "span" })).toBeInTheDocument();
  });

  it("checks the email before sending", async () => {
    const { user } = renderClasses(classPath(ids.active));
    await membersTable();

    const dialog = await invite(user, "khong-phai-email");
    expect(await within(dialog).findByText(INVALID_EMAIL)).toBeInTheDocument();
  });

  it.each([
    ["a new email without a name", 422, errors.nameRequired, NAME_REQUIRED],
    ["a malformed email", 422, { error: { code: "VALIDATION_FAILED", message: "Email không hợp lệ." } }, INVALID_EMAIL],
    ["an internal account", 403, errors.internalEmail, "Email này thuộc tài khoản nội bộ, không mời làm học viên được."],
    ["a disabled account", 409, errors.accountDisabled, "Tài khoản đã bị vô hiệu hóa. Kích hoạt lại trước khi mời."],
    ["an active member", 409, errors.alreadyMember, "Học viên đã có trong lớp."],
    ["too many invitations", 429, errors.rateLimited, errors.rateLimited.error.message],
  ])("explains a refused invitation for %s", async (_case, status, envelope, copy) => {
    const { user } = renderClasses(classPath(ids.active), {
      overrides: [http.post(api("/classes/:classId/invitations"), () => errorResponse(status, envelope))],
    });
    await membersTable();

    const dialog = await invite(user, "ai.do@gmail.com");
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(copy);
    expect(screen.getByRole("dialog", { name: "Mời học viên vào basic01" })).toBeInTheDocument();
  });

  it("reloads the class when it ended while inviting", async () => {
    const { user, db } = renderClasses(classPath(ids.active));
    await membersTable();
    db.classes = db.classes.map((c) => (c.id === ids.active ? { ...c, status: "ended" } : c));

    const dialog = await invite(user, "ai.do@gmail.com", "Ai Đó");
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(ENDED_CLASS);
    await user.click(within(dialog).getByRole("button", { name: "Hủy" }));
    await waitFor(() => {
      expect(screen.getAllByRole("button", { name: "Mời học viên" })[0]).toHaveAttribute("aria-disabled", "true");
    });
    expect(screen.queryByRole("button", { name: "Kết thúc lớp" })).not.toBeInTheDocument();
  });

  it("resends an invitation after confirmation", async () => {
    const { user } = renderClasses(classPath(ids.active));
    await membersTable();

    await user.click(within(memberRow("Bùi Quang Minh")).getByRole("button", { name: "Gửi lại" }));
    await confirmIn(user, "Gửi lại lời mời cho Bùi Quang Minh?", "Gửi lại");

    expect(await screen.findByText("Lời mời mới đang được gửi.")).toBeInTheDocument();
    await waitFor(() => { expect(within(memberRow("Bùi Quang Minh")).getByText("Đang chờ gửi")).toBeInTheDocument(); });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it.each([
    ["the student already changed the password", 409, errors.alreadyActivated, ALREADY_ACTIVATED],
    ["too many resends", 429, errors.rateLimited, RESEND_RATE_LIMITED],
  ])("explains a refused resend when %s", async (_case, status, envelope, copy) => {
    const { user } = renderClasses(classPath(ids.active), {
      overrides: [http.post(api("/classes/:classId/members/:memberId/resend"), () => errorResponse(status, envelope))],
    });
    await membersTable();

    await user.click(within(memberRow("Bùi Quang Minh")).getByRole("button", { name: "Gửi lại" }));
    await confirmIn(user, "Gửi lại lời mời cho Bùi Quang Minh?", "Gửi lại");

    expect(await screen.findByText(copy)).toBeInTheDocument();
    await waitFor(() => { expect(screen.queryByRole("dialog")).not.toBeInTheDocument(); });
  });

  it("removes a member, who can then rejoin the class", async () => {
    const { user } = renderClasses(classPath(ids.active));
    await membersTable();

    await user.click(within(memberRow("Trần Thị Bích")).getByRole("button", { name: "Gỡ khỏi lớp" }));
    await confirmIn(user, "Gỡ Trần Thị Bích khỏi lớp?", "Gỡ khỏi lớp");

    expect(await screen.findByText("Đã gỡ Trần Thị Bích khỏi lớp.")).toBeInTheDocument();
    await waitFor(() => { expect(within(memberRow("Trần Thị Bích")).getByText("Đã rời lớp")).toBeInTheDocument(); });
    const studentsTab = within(screen.getByRole("navigation", { name: "Mục của lớp" })).getByRole("link", { name: /^Học viên/ });
    await waitFor(() => { expect(studentsTab).toHaveTextContent("4"); });

    await invite(user, "bich.tran@gmail.com");
    expect(
      await screen.findByText("Trần Thị Bích đã có tài khoản: đã thêm vào lớp và gửi thông báo."),
    ).toBeInTheDocument();
    await waitFor(() => { expect(within(memberRow("Trần Thị Bích")).queryByText("Đã rời lớp")).not.toBeInTheDocument(); });
    await waitFor(() => { expect(studentsTab).toHaveTextContent("5"); });
  });

  it("disables an account after confirmation and re-enables one directly", async () => {
    const { user } = renderClasses(classPath(ids.active));
    await membersTable();

    await user.click(within(memberRow("Trần Thị Bích")).getByRole("button", { name: "Vô hiệu hóa" }));
    await confirmIn(user, "Vô hiệu hóa tài khoản bich.tran@gmail.com?", "Vô hiệu hóa");
    expect(await screen.findByText("Đã vô hiệu hóa tài khoản.")).toBeInTheDocument();
    await waitFor(() => {
      expect(within(memberRow("Trần Thị Bích")).getByRole("button", { name: "Kích hoạt lại" })).toBeInTheDocument();
    });

    await user.click(within(memberRow("Võ Phương Thảo")).getByRole("button", { name: "Kích hoạt lại" }));
    expect(await screen.findByText("Đã kích hoạt lại tài khoản.")).toBeInTheDocument();
    await waitFor(() => { expect(within(memberRow("Võ Phương Thảo")).getByText("Đã kích hoạt")).toBeInTheDocument(); });
  });

  it("polls a queued invitation until the mail worker sent it", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { db } = renderClasses(classPath(ids.active));
    await membersTable();
    expect(within(memberRow("Học Viên Mới")).getByText("Đang chờ gửi")).toBeInTheDocument();

    db.members[ids.active] = (db.members[ids.active] ?? []).map((m) =>
      m.id === ids.newcomer ? { ...m, inviteStatus: "sent" } : m,
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_EVERY_MS);
    });

    await waitFor(() => { expect(within(memberRow("Học Viên Mới")).getByText("Đã gửi")).toBeInTheDocument(); });
  });
});
