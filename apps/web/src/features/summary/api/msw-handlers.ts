/**
 * MSW handlers for the summary API, answering with the golden fixtures of `test/golden.ts`. Any member and stage of
 * any class answer the drawer golden; saving a review returns the next revision of the current one.
 * Test-only: nothing in the app imports this module.
 */
import { http, HttpResponse } from "msw";
import { api, apiError } from "@/shared/test/msw";
import { reviewProblem } from "../model/review-form";
import type { MemberStage, StageReview } from "../model/schemas";
import { fixtures } from "../test/golden";

const memberStagePath = "/classes/:classId/members/:memberId/stages/:stageVersionId";

/** The review the API stores for `body` on top of `current`, as Hương writes it now. */
export function nextReview(current: StageReview | null, body: string): StageReview {
  const now = new Date().toISOString();
  return {
    body: body.trim(),
    authorName: "Lê Thu Hương",
    createdAt: current?.createdAt ?? now,
    updatedAt: now,
    revision: (current?.revision ?? 0) + 1,
  };
}

/**
 * Handlers over a mutable drawer: a saved review shows on the next read of the drawer and flags the matrix cell
 * of that stage. Starts from `memberStage` (the reviewed golden by default).
 */
export function summaryHandlers({ memberStage = fixtures.memberStage }: { memberStage?: MemberStage } = {}) {
  let current = memberStage;
  let reviewed = new Set<string>();
  return [
    http.get(api("/me/classes/:classId/summary"), () => HttpResponse.json(fixtures.learner)),
    http.get(api("/classes/:classId/summary"), () =>
      HttpResponse.json({
        ...fixtures.matrix,
        rows: fixtures.matrix.rows.map((row) => ({
          ...row,
          cells: row.cells.map((c) =>
            reviewed.has(`${row.memberId}/${c.stageVersionId}`) ? { ...c, hasReview: true } : c,
          ),
        })),
      }),
    ),
    http.get(api(`${memberStagePath}/summary`), () => HttpResponse.json(current)),
    http.put(api(`${memberStagePath}/review`), async ({ request, params }) => {
      const { body } = (await request.json()) as { body: string };
      const problem = reviewProblem(body);
      if (problem) return apiError(422, "VALIDATION_FAILED", problem);
      const review = nextReview(current.summary.review, body);
      current = { ...current, summary: { ...current.summary, review } };
      reviewed = new Set(reviewed).add(`${String(params.memberId)}/${String(params.stageVersionId)}`);
      return HttpResponse.json(review);
    }),
  ];
}
