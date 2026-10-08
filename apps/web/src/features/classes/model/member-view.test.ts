import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import { LAST_ERROR_MAX, accountState, hasQueuedInvite, inviteCell, nextPoll, sortMembers } from "./member-view";

const now = new Date("2026-10-05T10:00:00Z");

describe("accountState", () => {
  it.each([
    [{ accountStatus: "disabled", tempPasswordExpiresAt: undefined }, "disabled"],
    [{ accountStatus: "active", tempPasswordExpiresAt: undefined }, "active"],
    [{ accountStatus: "invited", tempPasswordExpiresAt: "2026-10-08T10:00:00Z" }, "invited"],
    [{ accountStatus: "invited", tempPasswordExpiresAt: "2026-10-05T09:59:00Z" }, "invited-expired"],
    [{ accountStatus: "invited", tempPasswordExpiresAt: undefined }, "invited"],
  ] as const)("%o is %s", (member, expected) => {
    expect(accountState(member, now)).toBe(expected);
  });
});

describe("inviteCell", () => {
  it("shows only a dash when the member has no invitation", () => {
    expect(inviteCell({ inviteStatus: undefined }, now)).toEqual({
      dot: null,
      label: "—",
      secondary: null,
      secondaryTitle: null,
    });
    expect(inviteCell({ inviteStatus: null }, now).label).toBe("—");
  });

  it("calls a delivered notice to an existing account a notification", () => {
    const cell = inviteCell({ inviteStatus: "sent", inviteKind: "added", invitedAt: "2026-10-05T08:00:00Z" }, now);
    expect(cell).toMatchObject({ dot: "sent", label: "Đã gửi thông báo", secondary: "2 giờ trước" });
  });

  it.each([
    ["queued", "invite", "Đang chờ gửi"],
    ["sent", "invite", "Đã gửi"],
    ["sent", "resend", "Đã gửi"],
    ["queued", "added", "Đang chờ gửi"],
  ] as const)("labels %s (%s) as %s", (inviteStatus, inviteKind, label) => {
    expect(inviteCell({ inviteStatus, inviteKind, invitedAt: "2026-10-05T09:30:00Z" }, now).label).toBe(label);
  });

  it("shows the mail error and the attempts of a failed invitation", () => {
    const khang = fixtures.members.items.find((m) => m.inviteStatus === "failed");
    expect(khang && inviteCell(khang, now)).toMatchObject({
      dot: "failed",
      label: "Gửi thất bại",
      secondary: "Mailbox không tồn tại (550 5.1.1) · 3 lần thử",
      secondaryTitle: null,
    });
  });

  it("shortens a long mail error and keeps the full text for the title", () => {
    const error = "x".repeat(300);
    const cell = inviteCell({ inviteStatus: "failed", inviteAttempts: 2, inviteLastError: error }, now);
    expect(cell.secondary).toBe(`${"x".repeat(LAST_ERROR_MAX - 1)}… · 2 lần thử`);
    expect(cell.secondaryTitle).toBe(`${error} · 2 lần thử`);
  });
});

describe("sortMembers", () => {
  it("puts members who left last, then orders by Vietnamese name", () => {
    const sorted = sortMembers(fixtures.roster("2026-10-08T10:00:00Z").items).map((m) => m.fullName);
    expect(sorted).toEqual([
      "Bùi Quang Minh",
      "Học Viên Mới",
      "Trần Thị Bích",
      "Võ Phương Thảo",
      "Vũ Đức Khang",
      "Nguyễn Hoàng An",
    ]);
  });
});

describe("hasQueuedInvite", () => {
  it("is true while an active member waits for the mail worker", () => {
    expect(hasQueuedInvite(fixtures.members.items)).toBe(true);
  });

  it("ignores members who left", () => {
    expect(hasQueuedInvite([{ inviteStatus: "queued", memberStatus: "dropped" }])).toBe(false);
    expect(hasQueuedInvite([{ inviteStatus: "sent", memberStatus: "active" }])).toBe(false);
  });
});

describe("nextPoll", () => {
  const queued = [{ id: "m1", inviteStatus: "queued", memberStatus: "active" }] as const;

  it("polls every 5 seconds while an invitation is queued", () => {
    expect(nextPoll(null, queued, 1_000)).toEqual({ window: { key: "m1", since: 1_000 }, interval: 5_000 });
  });

  it("stops after 2 minutes for the same queued invitations", () => {
    const window = { key: "m1", since: 0 };
    expect(nextPoll(window, queued, 119_999).interval).toBe(5_000);
    expect(nextPoll(window, queued, 120_000)).toEqual({ window, interval: false });
  });

  it("opens a new window when another invitation is queued", () => {
    const more = [...queued, { id: "m2", inviteStatus: "queued", memberStatus: "active" }] as const;
    expect(nextPoll({ key: "m1", since: 0 }, more, 200_000)).toEqual({
      window: { key: "m1,m2", since: 200_000 },
      interval: 5_000,
    });
  });

  it("stops once nothing is queued", () => {
    expect(nextPoll({ key: "m1", since: 0 }, [{ id: "m1", inviteStatus: "sent", memberStatus: "active" }], 10)).toEqual({
      window: null,
      interval: false,
    });
    expect(nextPoll(null, undefined, 10).interval).toBe(false);
  });
});
