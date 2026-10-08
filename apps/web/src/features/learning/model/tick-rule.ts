import type { ReadOnlyReason } from "./schemas";

export interface TickState {
  readOnlyReason?: ReadOnlyReason | null;
  /** The lesson has been opened at least once (`firstOpenedAt` recorded). */
  opened: boolean;
}

/**
 * Why the student cannot tick or untick a lesson, or `null` when they can. The class state wins
 * over "not opened yet". Display only: the server decides, and its error is shown when it disagrees.
 */
export function tickBlocker({ readOnlyReason, opened }: TickState): string | null {
  if (readOnlyReason === "ended") return "Lớp đã kết thúc";
  if (readOnlyReason === "draft") return "Lớp chưa bắt đầu";
  if (!opened) return "Mở học liệu trước khi tích";
  return null;
}
