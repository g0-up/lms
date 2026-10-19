import type { Grade, MySubmissions, Submission } from "./schemas";

/** Latest graded submission and its current grade; `newerPending` when a later submission waits. */
export interface LatestResult {
  submission: Submission;
  grade: Grade;
  newerPending: boolean;
}

/** Result to show on top of the homework page, or null before the first grading. */
export function latestResult({ submissions }: Pick<MySubmissions, "submissions">): LatestResult | null {
  for (const [index, submission] of submissions.entries()) {
    if (!submission.grade) continue;
    const newerPending = submissions.slice(0, index).some((s) => s.status === "pending");
    return { submission, grade: submission.grade, newerPending };
  }
  return null;
}

/** Repo URL without its scheme, for compact display. */
export function repoLabel(url: string): string {
  return url.replace(/^https:\/\//, "");
}

export function isPast(iso: string, now: Date = new Date()): boolean {
  return new Date(iso).getTime() < now.getTime();
}
