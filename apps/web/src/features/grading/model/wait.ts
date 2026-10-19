import { GRADING_SLA_DAYS } from "@/shared/domain";

const HOUR_MS = 3_600_000;

/** Whole hours since `iso`, never negative (the API's `waitedHours`). */
export function hoursSince(iso: string, now: Date = new Date()): number {
  return Math.max(0, Math.floor((now.getTime() - new Date(iso).getTime()) / HOUR_MS));
}

/** How long an item has waited: "5 giờ" under a day (at least 1 giờ), else "2 ngày 3 giờ". */
export function fmtWait(hours: number): string {
  if (hours < 24) return `${String(Math.max(1, hours))} giờ`;
  return `${String(Math.floor(hours / 24))} ngày ${String(hours % 24)} giờ`;
}

/**
 * A pending submission past the grading SLA, from the API's whole `waitingDays` (the server's clock): three full
 * days of waiting is 72 hours or more.
 */
export function isOverdueWaiting(waitingDays: number | null): boolean {
  return waitingDays !== null && waitingDays >= GRADING_SLA_DAYS;
}
