import { z } from "zod";
import { isApiError } from "@/shared/api/errors";

/** Accepted file types of a homework submission, as the API's document uploads. */
export const SUBMISSION_FILE_EXTS = ["zip", "pdf", "md", "txt", "sql", "png", "jpg", "jpeg"] as const;
export const MAX_FILE_MB = 20;
const MAX_FILE_BYTES = MAX_FILE_MB * 1_048_576;
const MAX_REPO_URL = 500;
const MAX_COMMIT_REF = 100;

/** `accept` of the file input. */
export const SUBMISSION_ACCEPT = SUBMISSION_FILE_EXTS.map((ext) => `.${ext}`).join(",");

// Same rule as the API: https on GitHub, GitLab or Bitbucket with owner and repo; the host is
// case-insensitive, the scheme is not.
const REPO_URL = /^https:\/\/(www\.)?(github\.com|gitlab\.com|bitbucket\.org)\/[^/\s]+\/[^/\s]+(\/\S*)?$/i;
const COMMIT_REF = /^[A-Za-z0-9._/-]+$/;

export const SUBMISSION_MESSAGES = {
  noContent: "Nộp đường dẫn repo hoặc ít nhất một file.",
  repoUrl: "Đường dẫn repo phải là https trên GitHub, GitLab hoặc Bitbucket, ví dụ https://github.com/ten-ban/ten-repo.",
  commitNeedsRepo: "Commit hoặc tag chỉ đi kèm đường dẫn repo.",
  commitRef: "Commit hoặc tag tối đa 100 ký tự, chỉ gồm chữ, số và . _ / -.",
  tooManyFiles: (max: number) => `Tối đa ${String(max)} file mỗi lần nộp.`,
  badType: (name: string) => `File ${name} không đúng định dạng. Chấp nhận: ${SUBMISSION_FILE_EXTS.join(", ")}.`,
  tooBig: (name: string) => `File ${name} lớn hơn ${String(MAX_FILE_MB)} MB.`,
} as const;

export function isValidRepoUrl(value: string): boolean {
  return value.startsWith("https://") && value.length <= MAX_REPO_URL && REPO_URL.test(value);
}

function extensionOf(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot < 0 ? "" : name.slice(dot + 1).toLowerCase();
}

/** First reason a picked file cannot be submitted, or null. */
export function fileProblem(file: Pick<File, "name" | "size">): string | null {
  if (!(SUBMISSION_FILE_EXTS as readonly string[]).includes(extensionOf(file.name))) {
    return SUBMISSION_MESSAGES.badType(file.name);
  }
  if (file.size > MAX_FILE_BYTES) return SUBMISSION_MESSAGES.tooBig(file.name);
  return null;
}

/** Form schema of a submission; `maxFiles` comes from the API (`MySubmissions.maxFiles`). */
export function submissionFormSchema(maxFiles: number) {
  return z
    .object({
      repoUrl: z.string().trim(),
      commitRef: z.string().trim(),
      files: z.array(z.instanceof(File)),
    })
    .superRefine((v, ctx) => {
      if (v.repoUrl === "" && v.files.length === 0) {
        ctx.addIssue({ code: "custom", path: ["repoUrl"], message: SUBMISSION_MESSAGES.noContent });
      } else if (v.repoUrl !== "" && !isValidRepoUrl(v.repoUrl)) {
        ctx.addIssue({ code: "custom", path: ["repoUrl"], message: SUBMISSION_MESSAGES.repoUrl });
      }
      if (v.commitRef !== "" && v.repoUrl === "") {
        ctx.addIssue({ code: "custom", path: ["commitRef"], message: SUBMISSION_MESSAGES.commitNeedsRepo });
      } else if (v.commitRef !== "" && (v.commitRef.length > MAX_COMMIT_REF || !COMMIT_REF.test(v.commitRef))) {
        ctx.addIssue({ code: "custom", path: ["commitRef"], message: SUBMISSION_MESSAGES.commitRef });
      }
      if (v.files.length > maxFiles) {
        ctx.addIssue({ code: "custom", path: ["files"], message: SUBMISSION_MESSAGES.tooManyFiles(maxFiles) });
        return;
      }
      const problem = v.files.map(fileProblem).find((p) => p !== null);
      if (problem) ctx.addIssue({ code: "custom", path: ["files"], message: problem });
    });
}

export type SubmissionFormInput = z.input<ReturnType<typeof submissionFormSchema>>;
export type SubmissionFormValues = z.output<ReturnType<typeof submissionFormSchema>>;

/** Body of `POST …/submissions` once the files are uploaded. */
export interface SubmitBody {
  repoUrl: string;
  commitRef: string;
  mediaIds: string[];
}

export function toSubmitBody(values: Pick<SubmissionFormValues, "repoUrl" | "commitRef">, mediaIds: string[]): SubmitBody {
  return { repoUrl: values.repoUrl, commitRef: values.commitRef, mediaIds };
}

/** Success toast: number, late marker, and which pending submission it replaced. */
export function submittedMessage({ no, isLate, replaced }: { no: number; isLate: boolean; replaced: number | null }) {
  const late = isLate ? " (nộp muộn)" : "";
  const swap = replaced === null ? "" : `, thay lần #${String(replaced)} đang chờ chấm`;
  return `Đã nộp lần #${String(no)}${late}${swap}. Bạn sẽ nhận email khi có kết quả.`;
}

const SUBMISSION_FIELDS = ["repoUrl", "commitRef", "mediaIds"] as const;
export type SubmissionField = (typeof SUBMISSION_FIELDS)[number];

const fieldDetailsSchema = z.record(z.string(), z.string());

/** Per-field messages of a 422 from the API (`details` keyed by JSON field), when it sent any. */
export function submissionFieldErrors(error: unknown): Partial<Record<SubmissionField, string>> {
  if (!isApiError(error) || error.status !== 422) return {};
  const details = fieldDetailsSchema.safeParse(error.details);
  if (!details.success) return {};
  const out: Partial<Record<SubmissionField, string>> = {};
  for (const field of SUBMISSION_FIELDS) {
    const message = details.data[field];
    if (message) out[field] = message;
  }
  return out;
}
