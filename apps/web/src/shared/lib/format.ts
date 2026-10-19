const pad = (n: number) => String(n).padStart(2, "0");

const EMPTY = "—";

function parse(iso: string | null | undefined): Date | null {
  if (!iso) return null;
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? null : d;
}

/** dd/mm/yyyy in the browser's time zone; "—" when empty or invalid. */
export function fmtDate(iso: string | null | undefined): string {
  const d = parse(iso);
  if (!d) return EMPTY;
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${String(d.getFullYear())}`;
}

/** dd/mm/yyyy HH:mm in the browser's time zone; "—" when empty or invalid. */
export function fmtDateTime(iso: string | null | undefined): string {
  const d = parse(iso);
  if (!d) return EMPTY;
  return `${fmtDate(iso)} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** Relative Vietnamese time: "Vừa xong", "{n} phút/giờ/ngày trước", then the date after 30 days. */
export function rel(iso: string | null | undefined, now: Date = new Date()): string {
  const d = parse(iso);
  if (!d) return "Chưa có";
  const minutes = (now.getTime() - d.getTime()) / 60_000;
  if (minutes < 1) return "Vừa xong";
  if (minutes < 60) return `${String(Math.round(minutes))} phút trước`;
  if (minutes < 60 * 24) return `${String(Math.round(minutes / 60))} giờ trước`;
  const days = Math.round(minutes / 1440);
  if (days < 30) return `${String(days)} ngày trước`;
  return fmtDate(iso);
}

/** File size as "1,5 MB" (Vietnamese decimal comma) from 1 MiB up, else whole KB (at least "1 KB"). */
export function fmtSize(bytes: number): string {
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1).replace(".", ",")} MB`;
  return `${String(Math.max(1, Math.round(bytes / 1024)))} KB`;
}

/**
 * Time left until a deadline ("còn 3 giờ") or past it ("quá hạn 2 ngày"): minutes under an hour,
 * hours under two days, then days.
 */
export function dueIn(iso: string, now: Date = new Date()): string {
  const ms = new Date(iso).getTime() - now.getTime();
  const hours = Math.abs(ms) / 3_600_000;
  const span =
    hours < 1
      ? `${String(Math.max(1, Math.round(hours * 60)))} phút`
      : hours < 48
        ? `${String(Math.round(hours))} giờ`
        : `${String(Math.round(hours / 24))} ngày`;
  return ms >= 0 ? `còn ${span}` : `quá hạn ${span}`;
}

export function versionLabel(v: { no: number }): string {
  return `v${String(v.no)}`;
}

/** First letters of the last two words, uppercased ("Trần Minh Quân" → "MQ"). */
export function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(-2)
    .map((w) => w.charAt(0))
    .join("")
    .toUpperCase();
}
