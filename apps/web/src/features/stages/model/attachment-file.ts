/** File extensions the media API accepts for `kind: "document"`, for homework attachments and submissions. */
export const DOCUMENT_EXTENSIONS = ["zip", "pdf", "md", "txt", "sql", "png", "jpg", "jpeg"] as const;
/** `accept` of a document file input. */
export const DOCUMENT_ACCEPT = DOCUMENT_EXTENSIONS.map((ext) => `.${ext}`).join(",");
/** Server-side document limit (MAX_DOCUMENT_BYTES, 20 MB by default). */
export const MAX_DOCUMENT_BYTES = 20 * 1024 ** 2;
/** Files a learner may hand in with one homework submission. */
export const MAX_SUBMISSION_FILES = 5;

export const DOCUMENT_TYPE_MESSAGE = `Chỉ nhận file ${DOCUMENT_EXTENSIONS.join(", ")}.`;
export const DOCUMENT_SIZE_MESSAGE = "File vượt dung lượng cho phép (tối đa 20 MB).";

/**
 * Why the file cannot be uploaded as a document, or null. The server decides the type from the
 * extension alone (browsers report `.sql` and `.md` inconsistently), so this does too.
 */
export function documentFileError(file: Pick<File, "name" | "size">): string | null {
  const dot = file.name.lastIndexOf(".");
  const ext = dot > 0 ? file.name.slice(dot + 1).toLowerCase() : "";
  if (!(DOCUMENT_EXTENSIONS as readonly string[]).includes(ext)) return DOCUMENT_TYPE_MESSAGE;
  if (file.size > MAX_DOCUMENT_BYTES) return DOCUMENT_SIZE_MESSAGE;
  return null;
}
