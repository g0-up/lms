/**
 * "TG chấm TB" of the grading backlog: whole hours below a day, otherwise days with one decimal and a Vietnamese
 * decimal comma; "—" when the teacher has handled nothing yet. The hour count is rounded before the comparison so
 * 23.6 hours reads "1,0 ngày", never "24 giờ".
 */
export function fmtAvgHours(hours: number | null): string {
  if (hours === null) return "—";
  const rounded = Math.round(hours);
  if (rounded < 24) return `${String(rounded)} giờ`;
  return `${(hours / 24).toFixed(1).replace(".", ",")} ngày`;
}
