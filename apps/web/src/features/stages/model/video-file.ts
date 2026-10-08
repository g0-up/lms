/** The only video type the API accepts. */
export const VIDEO_CONTENT_TYPE = "video/mp4";
/** Server-side upload limit (2 GB); the API does not send it in its error details. */
export const MAX_VIDEO_BYTES = 2 * 1024 ** 3;

export const VIDEO_TYPE_MESSAGE = "Định dạng không hỗ trợ.";
export const VIDEO_SIZE_MESSAGE = "File vượt dung lượng cho phép.";
export const UPLOAD_FAILED_MESSAGE = "Tải file lên thất bại. Thử lại.";

/** Why the file cannot be uploaded, or null. */
export function videoFileError(file: Pick<File, "type" | "size">): string | null {
  if (file.type !== VIDEO_CONTENT_TYPE) return VIDEO_TYPE_MESSAGE;
  if (file.size > MAX_VIDEO_BYTES) return VIDEO_SIZE_MESSAGE;
  return null;
}

const UNITS = ["B", "KB", "MB", "GB"] as const;

/** Human size with one decimal above bytes: 1536 → "1,5 KB". */
export function fmtBytes(bytes: number): string {
  let value = Math.max(0, bytes);
  let unit = 0;
  while (value >= 1024 && unit < UNITS.length - 1) {
    value /= 1024;
    unit += 1;
  }
  const text = unit === 0 ? String(Math.round(value)) : value.toFixed(1).replace(".", ",");
  return `${text} ${UNITS[unit]}`;
}
