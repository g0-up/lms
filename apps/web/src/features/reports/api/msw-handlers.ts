/**
 * MSW handlers for the report and teacher API, answering with the golden fixtures of
 * `test/golden.ts`. Test-only: nothing in the app imports this module.
 */
import { http, HttpResponse } from "msw";
import { api } from "@/shared/test/msw";
import type { TeachClass } from "../model/schemas";
import { errors, fixtures, ids } from "../test/golden";

export function errorResponse(status: number, envelope: { error: { code: string; message: string } }) {
  return HttpResponse.json(envelope, { status });
}

/** Class headers a teacher may open: basic01 and the class of the report golden. */
const teachClasses: Partial<Record<string, TeachClass>> = {
  [ids.teachClass]: fixtures.teachClass,
  [ids.reportClass]: {
    ...fixtures.teachClass,
    id: ids.reportClass,
    code: fixtures.classReport.class.code,
    name: fixtures.classReport.class.name,
  },
};

/** Happy-path report API: any class answers the golden report; dropped rows on request. */
export const reportsHandlers = [
  http.get(api("/classes/:classId/report"), ({ request }) => {
    const includeDropped = new URL(request.url).searchParams.get("includeDropped") === "true";
    return HttpResponse.json(includeDropped ? fixtures.classReportWithDropped : fixtures.classReport);
  }),
  http.get(api("/classes/:classId/report/members/:memberId"), ({ params }) => {
    const row = fixtures.classReportWithDropped.rows.find((r) => r.memberId === params.memberId);
    if (!row) return errorResponse(404, errors.notFound);
    return HttpResponse.json({ ...fixtures.memberReport, member: row });
  }),
];

/** Teacher pages: the classes list and the header of a class they teach (403 otherwise). */
export const teachHandlers = [
  http.get(api("/teach/classes"), () => HttpResponse.json(fixtures.teachClasses)),
  http.get(api("/classes/:classId"), ({ params }) => {
    const found = teachClasses[String(params.classId)];
    return found ? HttpResponse.json(found) : errorResponse(403, errors.notOwnClass);
  }),
];
