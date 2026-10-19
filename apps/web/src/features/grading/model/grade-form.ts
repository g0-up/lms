import { z } from "zod";
import { isApiError } from "@/shared/api/errors";
import { isPassing, percentOf, type Percent } from "@/shared/domain";
import type { Criterion, Grade } from "./schemas";

export const MAX_COMMENT = 5000;
const MAX_SCORE_COMMENT = 1000;

/** Length in code points, as the API counts runes. */
const runeCount = (text: string) => Array.from(text).length;

/** Copy of the grading form's errors, word for word the API's. */
export const GRADE_MESSAGES = {
  missing: (criterion: string) => `Nhập điểm cho tiêu chí "${criterion}".`,
  range: (criterion: string, max: number) => `Điểm tiêu chí "${criterion}" phải từ 0 đến ${String(max)}.`,
  noteTooLong: (criterion: string) => `Nhận xét tiêu chí "${criterion}" tối đa ${String(MAX_SCORE_COMMENT)} ký tự.`,
  commentRequired: "Nhập nhận xét chung cho học viên.",
  commentTooLong: `Nhận xét tối đa ${String(MAX_COMMENT)} ký tự.`,
} as const;

/** Form fields: one row per criterion in rubric order; points stay text until checked. */
export interface GradeFormInput {
  scores: { points: string; comment: string }[];
  comment: string;
}

/** Body of `POST /submissions/{sid}/grades`. */
export interface GradeBody {
  scores: { criterionId: string; points: number; comment: string }[];
  comment: string;
}

/** Fields prefilled from the current grading, blank before the first one. */
export function gradeDefaults(criteria: Criterion[], current: Grade | null): GradeFormInput {
  return {
    scores: criteria.map((c) => {
      const score = current?.scores.find((s) => s.criterionId === c.id);
      return { points: score ? String(score.points) : "", comment: score?.comment ?? "" };
    }),
    comment: current?.comment ?? "",
  };
}

const WHOLE = /^\d+$/;

/** Points of a field, or null when it is blank or not a whole number. */
function pointsOf(raw: string): number | null {
  const value = raw.trim();
  return WHOLE.test(value) ? Number(value) : null;
}

/** The API's checks for the grading form, mapped to the request body. */
export function gradeFormSchema(criteria: Criterion[]) {
  return z
    .object({
      scores: z.array(z.object({ points: z.string(), comment: z.string().trim() })).length(criteria.length),
      comment: z.string().trim(),
    })
    .superRefine((v, ctx) => {
      criteria.forEach((c, i) => {
        const row = v.scores[i];
        const raw = row.points.trim();
        const value = pointsOf(raw);
        if (raw === "") {
          ctx.addIssue({ code: "custom", path: ["scores", i, "points"], message: GRADE_MESSAGES.missing(c.description) });
        } else if (value === null || value > c.maxPoints) {
          ctx.addIssue({
            code: "custom",
            path: ["scores", i, "points"],
            message: GRADE_MESSAGES.range(c.description, c.maxPoints),
          });
        }
        if (runeCount(row.comment) > MAX_SCORE_COMMENT) {
          ctx.addIssue({
            code: "custom",
            path: ["scores", i, "comment"],
            message: GRADE_MESSAGES.noteTooLong(c.description),
          });
        }
      });
      if (v.comment === "") {
        ctx.addIssue({ code: "custom", path: ["comment"], message: GRADE_MESSAGES.commentRequired });
      } else if (runeCount(v.comment) > MAX_COMMENT) {
        ctx.addIssue({ code: "custom", path: ["comment"], message: GRADE_MESSAGES.commentTooLong });
      }
    })
    .transform(
      (v): GradeBody => ({
        scores: criteria.map((c, i) => ({
          criterionId: c.id,
          points: pointsOf(v.scores[i].points) ?? 0,
          comment: v.scores[i].comment,
        })),
        comment: v.comment,
      }),
    );
}

export interface GradeTotal {
  total: number;
  max: number;
  pct: Percent;
  /** Criteria without points yet; the verdict is only meaningful at 0. */
  missing: number;
  passed: boolean;
}

/**
 * Live total of the form as the grader types: entered points clamped to each criterion's range, the share of
 * the maximum, how many criteria are still blank and the verdict by the API's rule.
 */
export function gradeTotal(criteria: Criterion[], points: readonly string[], passPercent: number): GradeTotal {
  let total = 0;
  let missing = 0;
  criteria.forEach((c, i) => {
    const raw = points[i] ?? "";
    if (raw.trim() === "") {
      missing += 1;
      return;
    }
    total += Math.min(c.maxPoints, Math.max(0, pointsOf(raw) ?? 0));
  });
  const max = criteria.reduce((sum, c) => sum + c.maxPoints, 0);
  return { total, max, pct: percentOf(total, max), missing, passed: isPassing(total, max, passPercent) };
}

/** Field of a 422 from the API: the general comment or the points of one criterion. */
export interface GradeFieldErrors {
  comment?: string;
  /** Keyed by criterion id (`scores.<criterionId>` in the API's details). */
  scores: Record<string, string>;
}

const fieldDetailsSchema = z.record(z.string(), z.string());

export function gradeFieldErrors(error: unknown): GradeFieldErrors {
  const out: GradeFieldErrors = { scores: {} };
  if (!isApiError(error) || error.status !== 422) return out;
  const details = fieldDetailsSchema.safeParse(error.details);
  if (!details.success) return out;
  for (const [field, message] of Object.entries(details.data)) {
    if (field === "comment") out.comment = message;
    else if (field.startsWith("scores.")) out.scores[field.slice("scores.".length)] = message;
  }
  return out;
}

/** Success message of a saved grading, before what happens next. */
export function gradedMessage(grade: Pick<Grade, "totalPoints" | "maxPoints" | "result">): string {
  const verdict = grade.result === "passed" ? "Đạt" : "Cần làm lại";
  return `Đã lưu ${String(grade.totalPoints)}/${String(grade.maxPoints)} · ${verdict}. Email kết quả đã gửi cho học viên.`;
}
