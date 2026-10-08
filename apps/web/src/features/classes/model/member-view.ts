import { STATUS_VI } from "@/shared/domain";
import { rel } from "@/shared/lib/format";
import type { Member } from "./schemas";

export type AccountState = "disabled" | "invited-expired" | "invited" | "active";

export const ACCOUNT_LABEL: Record<AccountState, string> = {
  disabled: "Vô hiệu hóa",
  "invited-expired": "Mật khẩu tạm hết hạn",
  invited: "Chưa đăng nhập",
  active: "Đã kích hoạt",
};

/** Account column: a temporary password past its expiry is called out apart from a plain invite. */
export function accountState(
  member: Pick<Member, "accountStatus" | "tempPasswordExpiresAt">,
  now: Date = new Date(),
): AccountState {
  if (member.accountStatus === "disabled") return "disabled";
  if (member.accountStatus === "active") return "active";
  const expiresAt = member.tempPasswordExpiresAt ? new Date(member.tempPasswordExpiresAt) : null;
  return expiresAt && expiresAt.getTime() < now.getTime() ? "invited-expired" : "invited";
}

/** Longest mail-system error shown inline; the full text stays in the cell's `title`. */
export const LAST_ERROR_MAX = 120;

export interface InviteCell {
  /** Null when the member has no invitation: the cell shows "—" only. */
  dot: "queued" | "sent" | "failed" | null;
  label: string;
  secondary: string | null;
  /** Full secondary text when it was shortened. */
  secondaryTitle: string | null;
}

function shorten(text: string, max: number): string {
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
}

/** Invite column: delivery state, then the mail error and attempts on failure, else when it was sent. */
export function inviteCell(
  member: Pick<Member, "inviteStatus" | "inviteKind" | "inviteAttempts" | "inviteLastError" | "invitedAt">,
  now: Date = new Date(),
): InviteCell {
  const status = member.inviteStatus;
  if (!status) return { dot: null, label: "—", secondary: null, secondaryTitle: null };
  const label = status === "sent" && member.inviteKind === "added" ? "Đã gửi thông báo" : STATUS_VI[status];
  if (status !== "failed") return { dot: status, label, secondary: rel(member.invitedAt, now), secondaryTitle: null };
  const attempts = `${String(member.inviteAttempts ?? 0)} lần thử`;
  const error = member.inviteLastError?.trim() ?? "";
  const full = error ? `${error} · ${attempts}` : attempts;
  const short = error ? `${shorten(error, LAST_ERROR_MAX)} · ${attempts}` : attempts;
  return { dot: "failed", label, secondary: short, secondaryTitle: short === full ? null : full };
}

/** Members still in the class first, those who left last; then by name in Vietnamese order. */
export function sortMembers<T extends Pick<Member, "memberStatus" | "fullName">>(members: readonly T[]): T[] {
  return [...members].sort((a, b) => {
    const dropped = Number(a.memberStatus === "dropped") - Number(b.memberStatus === "dropped");
    return dropped !== 0 ? dropped : a.fullName.localeCompare(b.fullName, "vi");
  });
}

/** Whether a queued invitation is still waiting for the mail worker (the list polls until none is). */
export function hasQueuedInvite(members: readonly Pick<Member, "inviteStatus" | "memberStatus">[]): boolean {
  return members.some((m) => m.memberStatus !== "dropped" && m.inviteStatus === "queued");
}

export const POLL_EVERY_MS = 5_000;
export const POLL_FOR_MS = 2 * 60_000;

export interface PollWindow {
  /** Queued member ids the window was opened for. */
  key: string;
  since: number;
}

/**
 * Polling of the member list while invitations are queued: every 5 s, for at most 2 minutes per
 * set of queued invitations (a new invitation opens a new window). Returns the next window and
 * the interval, `false` to stop.
 */
export function nextPoll(
  window: PollWindow | null,
  members: readonly Pick<Member, "id" | "inviteStatus" | "memberStatus">[] | undefined,
  now: number,
): { window: PollWindow | null; interval: number | false } {
  if (!members || !hasQueuedInvite(members)) return { window: null, interval: false };
  const key = members
    .filter((m) => m.memberStatus !== "dropped" && m.inviteStatus === "queued")
    .map((m) => m.id)
    .sort()
    .join(",");
  const open = window?.key === key ? window : { key, since: now };
  return { window: open, interval: now - open.since < POLL_FOR_MS ? POLL_EVERY_MS : false };
}
