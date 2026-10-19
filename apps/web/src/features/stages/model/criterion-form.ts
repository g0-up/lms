import { z } from "zod";
import type { Criterion, Homework } from "./schemas";

export const CRITERION_POINTS_MAX = 100;
export const CRITERION_DESCRIPTION_REQUIRED = "Nhập nội dung tiêu chí.";
export const CRITERION_POINTS_RANGE = "Điểm tối đa của tiêu chí là số nguyên từ 1 đến 100.";

/** "Thêm tiêu chí chấm"/"Sửa tiêu chí chấm" values, with the server's rules and messages. */
export const criterionFormSchema = z.object({
  description: z.string().trim().min(1, CRITERION_DESCRIPTION_REQUIRED),
  maxPoints: z
    .string()
    .trim()
    .refine((s) => /^\d+$/.test(s) && Number(s) >= 1 && Number(s) <= CRITERION_POINTS_MAX, CRITERION_POINTS_RANGE)
    .transform(Number),
});
export type CriterionFormValues = z.input<typeof criterionFormSchema>;
/** Also the body of `POST …/criteria` and `PATCH …/criteria/{cid}`. */
export type CriterionBody = z.output<typeof criterionFormSchema>;

/** The criterion being edited, else a blank one. */
export function criterionFormDefaults(criterion?: Criterion): CriterionFormValues {
  return { description: criterion?.description ?? "", maxPoints: criterion ? String(criterion.maxPoints) : "" };
}

/** "{n} tiêu chí · {max} điểm" under the rubric's heading. */
export function criteriaCountLabel(homework: Pick<Homework, "criteria" | "maxPoints">): string {
  return `${String(homework.criteria.length)} tiêu chí · ${String(homework.maxPoints)} điểm`;
}

/** "{p}% tổng điểm (từ {score}/{max})": the lowest passing score, rounded up, once the rubric has points. */
export function homeworkPassLabel(homework: Pick<Homework, "passPercent" | "maxPoints">): string {
  const { passPercent, maxPoints } = homework;
  const base = `${String(passPercent)}% tổng điểm`;
  if (maxPoints <= 0) return base;
  return `${base} (từ ${String(Math.ceil((passPercent * maxPoints) / 100))}/${String(maxPoints)})`;
}
