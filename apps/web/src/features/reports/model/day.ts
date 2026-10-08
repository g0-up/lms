import { fmtDate } from "@/shared/lib/format";

/** Calendar day ("2026-10-05") read as local midnight, so no time zone can shift it a day. */
export const fmtDay = (day: string) => fmtDate(`${day}T00:00:00`);
