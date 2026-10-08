/** Image types the media API accepts for `kind: "image"`. */
export const IMAGE_CONTENT_TYPES = ["image/png", "image/jpeg", "image/webp", "image/gif"] as const;
/** Server-side image limit (MAX_IMAGE_BYTES, 10 MB by default). */
export const MAX_IMAGE_BYTES = 10 * 1024 ** 2;

export const IMAGE_TYPE_MESSAGE = "Chỉ hỗ trợ ảnh PNG, JPEG, WebP hoặc GIF.";
export const IMAGE_SIZE_MESSAGE = "File vượt dung lượng cho phép (tối đa 10 MB).";
/** Alt text used when a file name has nothing left once the extension is dropped. */
export const DEFAULT_IMAGE_ALT = "Ảnh";

/** Why the file cannot be uploaded as a lesson image, or null. */
export function imageFileError(file: Pick<File, "type" | "size">): string | null {
  if (!(IMAGE_CONTENT_TYPES as readonly string[]).includes(file.type)) return IMAGE_TYPE_MESSAGE;
  if (file.size > MAX_IMAGE_BYTES) return IMAGE_SIZE_MESSAGE;
  return null;
}

/** Alt text from a file name: drops only the last extension ("so-do.v2.png" → "so-do.v2"). */
export function altFromFileName(name: string): string {
  const dot = name.lastIndexOf(".");
  const base = (dot > 0 ? name.slice(0, dot) : dot === 0 ? "" : name).trim();
  return base === "" ? DEFAULT_IMAGE_ALT : base;
}

/** The only image src the API keeps in lesson markdown. */
export function mediaContentUrl(mediaId: string): string {
  return `/api/v1/media/${mediaId}/content`;
}
