/**
 * Deadlines are entered and shown in Vietnam time (UTC+7 all year, no daylight saving), whatever the
 * browser's own zone, and sent to the API as RFC 3339 with the `+07:00` offset.
 */
const OFFSET_MS = 7 * 3_600_000;
const LOCAL_INPUT = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/;

/** `datetime-local` value ("2026-10-05T15:00") of an API timestamp, in Vietnam time; "" for none. */
export function toLocalInput(iso: string | null): string {
  if (!iso) return "";
  const ms = new Date(iso).getTime();
  if (Number.isNaN(ms)) return "";
  return new Date(ms + OFFSET_MS).toISOString().slice(0, 16);
}

/** RFC 3339 timestamp of a `datetime-local` value read as Vietnam time, or null when it is not a real time. */
export function fromLocalInput(value: string): string | null {
  const match = LOCAL_INPUT.exec(value.trim());
  if (!match) return null;
  const [, year, month, day, hour, minute] = match.map(Number);
  const utc = Date.UTC(year, month - 1, day, hour, minute);
  const back = new Date(utc);
  // Date.UTC rolls 30/02 over to 02/03: such a value was never a date.
  if (back.getUTCFullYear() !== year || back.getUTCMonth() !== month - 1 || back.getUTCDate() !== day) return null;
  if (hour > 23 || minute > 59) return null;
  return `${value.trim()}:00+07:00`;
}

/** A deadline already behind us: not-submitted members show "Quá hạn". */
export function isPastDeadline(iso: string, now: Date = new Date()): boolean {
  return new Date(iso).getTime() < now.getTime();
}
